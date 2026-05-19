package application

import (
	"context"
	"testing"
	"time"

	"github.com/pashathecreator/holdem/services/wallet-service/internal/domain"
)

type walletRepoStub struct {
	address         *domain.DepositAddress
	createdTransfer *domain.FaucetTransfer
}

func (s *walletRepoStub) ProvisionUser(context.Context, *domain.Account, *domain.DepositAddress) error {
	return nil
}
func (s *walletRepoStub) GetBalance(context.Context, string) (*domain.Balance, error) {
	return nil, nil
}
func (s *walletRepoStub) GetDepositAddress(context.Context, string) (*domain.DepositAddress, error) {
	return s.address, nil
}
func (s *walletRepoStub) ListLinkedAddresses(context.Context, string) ([]*domain.LinkedAddress, error) {
	return nil, nil
}
func (s *walletRepoStub) CreateLinkChallenge(context.Context, *domain.LinkChallenge) error {
	return nil
}
func (s *walletRepoStub) FindLinkChallenge(context.Context, string, string, string) (*domain.LinkChallenge, error) {
	return nil, nil
}
func (s *walletRepoStub) ConfirmLinkChallenge(context.Context, string, *domain.LinkedAddress) error {
	return nil
}
func (s *walletRepoStub) ListDeposits(context.Context, string) ([]*domain.Deposit, error) {
	return nil, nil
}
func (s *walletRepoStub) CreateConfirmedDeposit(context.Context, *domain.Deposit, *domain.LedgerEntry) error {
	return nil
}
func (s *walletRepoStub) ListWithdrawals(context.Context, string) ([]*domain.WithdrawalRequest, error) {
	return nil, nil
}
func (s *walletRepoStub) CreateWithdrawal(context.Context, *domain.WithdrawalRequest, *domain.LedgerEntry) error {
	return nil
}
func (s *walletRepoStub) ListFaucetTransfers(context.Context) ([]*domain.FaucetTransfer, error) {
	return nil, nil
}
func (s *walletRepoStub) CreateFaucetTransfer(_ context.Context, transfer *domain.FaucetTransfer) error {
	s.createdTransfer = transfer
	return nil
}
func (s *walletRepoStub) ApplyLedgerEntry(context.Context, *domain.LedgerEntry) error { return nil }

type walletCryptoStub struct{}

func (walletCryptoStub) GenerateDepositAccount() (string, string, error)           { return "", "", nil }
func (walletCryptoStub) RecoverPersonalSignAddress(string, string) (string, error) { return "", nil }

func TestCreateAdminFaucetTransferCreatesPendingRequest(t *testing.T) {
	repo := &walletRepoStub{
		address: &domain.DepositAddress{UserID: "user-1", Address: "0xabc", CreatedAt: time.Now()},
	}
	service := NewService(repo, "sepolia", walletCryptoStub{})

	transfer, err := service.CreateAdminFaucetTransfer(context.Background(), "admin-1", "user-1", 42)
	if err != nil {
		t.Fatalf("CreateAdminFaucetTransfer() error = %v", err)
	}
	if transfer.Status != domain.FaucetTransferStatusPending {
		t.Fatalf("transfer.Status = %q, want pending", transfer.Status)
	}
	if transfer.TargetAddress != "0xabc" {
		t.Fatalf("transfer.TargetAddress = %q, want 0xabc", transfer.TargetAddress)
	}
	if repo.createdTransfer == nil || repo.createdTransfer.AdminUserID != "admin-1" {
		t.Fatalf("created transfer admin user was not saved")
	}
}
