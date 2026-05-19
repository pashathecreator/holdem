package auth

import (
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
)

var ErrUnauthenticated = errors.New("authentication required")

type Validator struct {
	issuer  string
	jwksURL string
	client  *http.Client

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

type Claims struct {
	UserID  string
	IsAdmin bool
}

type claims struct {
	Iss     string `json:"iss"`
	Sub     string `json:"sub"`
	IsAdmin bool   `json:"is_admin"`
	Exp     int64  `json:"exp"`
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

func NewValidator(issuer, jwksURL string) *Validator {
	return &Validator{
		issuer:  issuer,
		jwksURL: jwksURL,
		client:  &http.Client{Timeout: 5 * time.Second},
		keys:    make(map[string]*rsa.PublicKey),
	}
}

func (v *Validator) AuthenticateRequest(r *http.Request) (*Claims, error) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		return nil, ErrUnauthenticated
	}
	return v.verify(token)
}

func (v *Validator) verify(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrUnauthenticated
	}

	headerData, err := decodeSegment(parts[0])
	if err != nil {
		return nil, ErrUnauthenticated
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerData, &header); err != nil || header.Alg != "RS256" || header.Kid == "" {
		return nil, ErrUnauthenticated
	}

	claimsData, err := decodeSegment(parts[1])
	if err != nil {
		return nil, ErrUnauthenticated
	}
	var parsed claims
	if err := json.Unmarshal(claimsData, &parsed); err != nil {
		return nil, ErrUnauthenticated
	}
	if parsed.Iss != v.issuer || parsed.Sub == "" || parsed.Exp < time.Now().UTC().Unix() {
		return nil, ErrUnauthenticated
	}

	key, err := v.keyFor(header.Kid)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	signature, err := decodeSegment(parts[2])
	if err != nil {
		return nil, ErrUnauthenticated
	}
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], signature); err != nil {
		return nil, ErrUnauthenticated
	}
	return &Claims{
		UserID:  parsed.Sub,
		IsAdmin: parsed.IsAdmin,
	}, nil
}

func (v *Validator) keyFor(kid string) (*rsa.PublicKey, error) {
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

func (v *Validator) refreshKeys() error {
	resp, err := v.client.Get(v.jwksURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks status %d", resp.StatusCode)
	}

	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
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
	v.keys = keys
	v.fetchedAt = time.Now().UTC()
	v.mu.Unlock()
	return nil
}

func publicKeyFromJWK(key jwk) (*rsa.PublicKey, error) {
	modulusBytes, err := decodeSegment(key.N)
	if err != nil {
		return nil, err
	}
	exponentBytes, err := decodeSegment(key.E)
	if err != nil {
		return nil, err
	}
	exponent := 0
	for _, b := range exponentBytes {
		exponent = exponent<<8 + int(b)
	}
	if exponent == 0 {
		return nil, fmt.Errorf("invalid exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(modulusBytes), E: exponent}, nil
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
