package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenManagerIssueAndParseAccessToken(t *testing.T) {
	privateKeyPath := writePrivateKey(t)

	manager, err := NewTokenManager("holdem-auth", "test-key", privateKeyPath, 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewTokenManager() error = %v", err)
	}

	token, err := manager.IssueAccessToken("user-1", true)
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	claims, err := manager.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}
	if claims.Sub != "user-1" {
		t.Fatalf("claims.Sub = %q, want user-1", claims.Sub)
	}
	if claims.Iss != "holdem-auth" {
		t.Fatalf("claims.Iss = %q, want holdem-auth", claims.Iss)
	}
	if !claims.IsAdmin {
		t.Fatalf("claims.IsAdmin = false, want true")
	}
}

func TestTokenManagerRefreshTokensAreHashedDeterministically(t *testing.T) {
	privateKeyPath := writePrivateKey(t)

	manager, err := NewTokenManager("holdem-auth", "test-key", privateKeyPath, 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewTokenManager() error = %v", err)
	}

	token, hash, expiresAt, err := manager.NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken() error = %v", err)
	}
	if token == "" || hash == "" {
		t.Fatalf("token = %q hash = %q, want non-empty values", token, hash)
	}
	if !expiresAt.After(time.Now()) {
		t.Fatalf("expiresAt = %v, want future time", expiresAt)
	}
	if manager.HashRefreshToken(token) != hash {
		t.Fatalf("HashRefreshToken() mismatch")
	}
}

func writePrivateKey(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "jwt_private.pem")
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
