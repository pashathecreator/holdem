package security

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"
)

type TokenManager struct {
	issuer     string
	keyID      string
	accessTTL  time.Duration
	refreshTTL time.Duration
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

type Claims struct {
	Iss     string `json:"iss"`
	Sub     string `json:"sub"`
	IsAdmin bool   `json:"is_admin"`
	Iat     int64  `json:"iat"`
	Exp     int64  `json:"exp"`
}

type JWKResponse struct {
	Keys []JWK `json:"keys"`
}

type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func NewTokenManager(issuer, keyID, privateKeyPath string, accessTTL, refreshTTL time.Duration) (*TokenManager, error) {
	privateKey, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return nil, err
	}

	return &TokenManager{
		issuer:     issuer,
		keyID:      keyID,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
	}, nil
}

func (m *TokenManager) IssueAccessToken(userID string, isAdmin bool) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		Iss:     m.issuer,
		Sub:     userID,
		IsAdmin: isAdmin,
		Iat:     now.Unix(),
		Exp:     now.Add(m.accessTTL).Unix(),
	}

	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
		"kid": m.keyID,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	signingInput := encodeSegment(headerJSON) + "." + encodeSegment(claimsJSON)
	hash := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, m.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return signingInput + "." + encodeSegment(signature), nil
}

func (m *TokenManager) ParseAccessToken(token string) (*Claims, error) {
	return VerifyTokenWithKey(token, m.issuer, m.publicKey)
}

func (m *TokenManager) NewRefreshToken() (string, string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", time.Time{}, fmt.Errorf("generate refresh token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	return token, base64.RawURLEncoding.EncodeToString(hash[:]), time.Now().UTC().Add(m.refreshTTL), nil
}

func (m *TokenManager) HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func (m *TokenManager) RefreshTTL() time.Duration {
	return m.refreshTTL
}

func (m *TokenManager) JWKS() JWKResponse {
	return JWKResponse{
		Keys: []JWK{{
			Kty: "RSA",
			Use: "sig",
			Kid: m.keyID,
			Alg: "RS256",
			N:   encodeBigInt(m.publicKey.N),
			E:   encodeBigInt(big.NewInt(int64(m.publicKey.E))),
		}},
	}
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("decode private key PEM")
	}

	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		privateKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
		return privateKey, nil
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse RSA private key: %w", err)
	}
	return privateKey, nil
}

func VerifyTokenWithKey(token, issuer string, publicKey *rsa.PublicKey) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("token must have 3 segments")
	}

	headerData, err := decodeSegment(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerData, &header); err != nil {
		return nil, fmt.Errorf("unmarshal header: %w", err)
	}
	if header.Alg != "RS256" {
		return nil, fmt.Errorf("unsupported alg")
	}

	claimsData, err := decodeSegment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}
	var claims Claims
	if err := json.Unmarshal(claimsData, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}
	if claims.Iss != issuer {
		return nil, fmt.Errorf("invalid issuer")
	}
	if claims.Exp < time.Now().UTC().Unix() {
		return nil, fmt.Errorf("token expired")
	}

	signature, err := decodeSegment(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash[:], signature); err != nil {
		return nil, fmt.Errorf("verify token: %w", err)
	}
	return &claims, nil
}

func encodeSegment(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeSegment(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}

func encodeBigInt(value *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(value.Bytes())
}
