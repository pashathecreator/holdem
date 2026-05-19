package chain

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestGenerateDepositAccountReturnsEthereumAddress(t *testing.T) {
	client := &Client{}

	privateKeyHex, address, err := client.GenerateDepositAccount()
	if err != nil {
		t.Fatalf("GenerateDepositAccount() error = %v", err)
	}
	if len(privateKeyHex) != 64 {
		t.Fatalf("private key length = %d, want 64", len(privateKeyHex))
	}
	if len(address) != 42 || address[:2] != "0x" {
		t.Fatalf("address = %q, want ethereum hex address", address)
	}
}

func TestRecoverPersonalSignAddress(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	message := "Holdem wallet link\nuser_id: user-1\naddress: 0x1234\nchallenge_id: challenge-1\nexpires_at: 2026-01-01T00:00:00Z\nnetwork: sepolia"
	hash := accounts.TextHash([]byte(message))
	signature, err := crypto.Sign(hash, privateKey)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	client := &Client{}
	recovered, err := client.RecoverPersonalSignAddress(message, "0x"+hex.EncodeToString(signature))
	if err != nil {
		t.Fatalf("RecoverPersonalSignAddress() error = %v", err)
	}

	expected := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	if recovered != expected {
		t.Fatalf("recovered = %q, want %q", recovered, expected)
	}
}
