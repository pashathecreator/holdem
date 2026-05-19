package grpc

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/metadata"
)

const userIDMetadataKey = "x-user-id"

var (
	ErrUnauthenticated = errors.New("authentication required")
	ErrInvalidToken    = errors.New("invalid token")
)

type JWTValidator struct {
	issuer  string
	jwksURL string
	client  *http.Client

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

type Authenticator struct {
	validator   *JWTValidator
	allowLegacy bool
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type tokenClaims struct {
	Iss string `json:"iss"`
	Sub string `json:"sub"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

func NewJWTValidator(issuer, jwksURL string) *JWTValidator {
	return &JWTValidator{
		issuer:  issuer,
		jwksURL: jwksURL,
		client:  &http.Client{Timeout: 5 * time.Second},
		keys:    make(map[string]*rsa.PublicKey),
	}
}

func NewAuthenticator(validator *JWTValidator, allowLegacy bool) *Authenticator {
	return &Authenticator{validator: validator, allowLegacy: allowLegacy}
}

func (a *Authenticator) OptionalUserID(ctx context.Context) (string, error) {
	if a == nil {
		return "", nil
	}
	if a.validator == nil {
		if a.allowLegacy {
			return userIDFromContext(ctx), nil
		}
		return "", nil
	}
	return a.validator.AuthenticateMetadata(ctx, false, a.allowLegacy)
}

func (a *Authenticator) RequiredUserID(ctx context.Context) (string, error) {
	if a == nil {
		return "", ErrUnauthenticated
	}
	if a.validator == nil {
		if userID := userIDFromContext(ctx); userID != "" && a.allowLegacy {
			return userID, nil
		}
		return "", ErrUnauthenticated
	}
	return a.validator.AuthenticateMetadata(ctx, true, a.allowLegacy)
}

func (v *JWTValidator) AuthenticateHTTPRequest(r *http.Request, required, allowLegacy bool) (string, error) {
	if token := bearerToken(r.Header.Get("Authorization")); token != "" {
		claims, err := v.verifyToken(token)
		if err != nil {
			return "", err
		}
		return claims.Sub, nil
	}

	if allowLegacy {
		if userID := strings.TrimSpace(r.Header.Get("X-User-Id")); userID != "" {
			return userID, nil
		}
		if userID := strings.TrimSpace(r.Header.Get(userIDMetadataKey)); userID != "" {
			return userID, nil
		}
	}

	if required {
		return "", ErrUnauthenticated
	}
	return "", nil
}

func (v *JWTValidator) AuthenticateMetadata(ctx context.Context, required, allowLegacy bool) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if values := md.Get("authorization"); len(values) > 0 {
			if token := bearerToken(values[0]); token != "" {
				claims, err := v.verifyToken(token)
				if err != nil {
					return "", err
				}
				return claims.Sub, nil
			}
		}
	}

	if allowLegacy {
		if userID := userIDFromContext(ctx); userID != "" {
			return userID, nil
		}
	}

	if required {
		return "", ErrUnauthenticated
	}
	return "", nil
}

func InjectUserID(r *http.Request, userID string) *http.Request {
	if userID == "" {
		return r
	}

	r.Header.Set(userIDMetadataKey, userID)
	ctx := metadata.NewIncomingContext(r.Context(), metadata.Pairs(userIDMetadataKey, userID))
	return r.WithContext(ctx)
}

func userIDFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(userIDMetadataKey)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (v *JWTValidator) verifyToken(token string) (*tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	headerData, err := decodeSegment(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerData, &header); err != nil || header.Alg != "RS256" || header.Kid == "" {
		return nil, ErrInvalidToken
	}

	claimsData, err := decodeSegment(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims tokenClaims
	if err := json.Unmarshal(claimsData, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Iss != v.issuer || claims.Sub == "" || claims.Exp < time.Now().UTC().Unix() {
		return nil, ErrInvalidToken
	}

	key, err := v.keyFor(header.Kid)
	if err != nil {
		return nil, ErrInvalidToken
	}

	signature, err := decodeSegment(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], signature); err != nil {
		return nil, ErrInvalidToken
	}

	return &claims, nil
}

func (v *JWTValidator) keyFor(kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	needsRefresh := time.Since(v.fetchedAt) > 5*time.Minute
	v.mu.RUnlock()
	if ok && !needsRefresh {
		return key, nil
	}

	if err := v.refreshKeys(); err != nil {
		if ok {
			return key, nil
		}
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok = v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("unknown key id")
	}
	return key, nil
}

func (v *JWTValidator) refreshKeys() error {
	resp, err := v.client.Get(v.jwksURL)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks returned status %d", resp.StatusCode)
	}

	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, key := range doc.Keys {
		if key.Kty != "RSA" || key.Alg != "RS256" || key.Kid == "" {
			continue
		}
		publicKey, err := publicKeyFromJWK(key)
		if err != nil {
			return err
		}
		keys[key.Kid] = publicKey
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	v.keys = keys
	v.fetchedAt = time.Now()
	return nil
}

func publicKeyFromJWK(key jwk) (*rsa.PublicKey, error) {
	modulusBytes, err := decodeSegment(key.N)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	exponentBytes, err := decodeSegment(key.E)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}

	exponent := 0
	for _, b := range exponentBytes {
		exponent = exponent<<8 + int(b)
	}
	if exponent == 0 {
		return nil, fmt.Errorf("invalid exponent")
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulusBytes),
		E: exponent,
	}, nil
}

func bearerToken(header string) string {
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

func decodeSegment(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}
