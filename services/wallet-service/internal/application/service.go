package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/pashathecreator/holdem/services/wallet-service/internal/domain"
)

const linkChallengeTTL = 10 * time.Minute

type Repository interface {
	ProvisionUser(ctx context.Context, account *domain.Account, depositAddress *domain.DepositAddress) error
	GetBalance(ctx context.Context, userID string) (*domain.Balance, error)
	GetDepositAddress(ctx context.Context, userID string) (*domain.DepositAddress, error)
	ListLinkedAddresses(ctx context.Context, userID string) ([]*domain.LinkedAddress, error)
	CreateLinkChallenge(ctx context.Context, challenge *domain.LinkChallenge) error
	FindLinkChallenge(ctx context.Context, userID, challengeID, address string) (*domain.LinkChallenge, error)
	ConfirmLinkChallenge(ctx context.Context, challengeID string, linked *domain.LinkedAddress) error
	ListDeposits(ctx context.Context, userID string) ([]*domain.Deposit, error)
	CreateConfirmedDeposit(ctx context.Context, deposit *domain.Deposit, entry *domain.LedgerEntry) error
	ListWithdrawals(ctx context.Context, userID string) ([]*domain.WithdrawalRequest, error)
	CreateWithdrawal(ctx context.Context, withdrawal *domain.WithdrawalRequest, entry *domain.LedgerEntry) error
	ListFaucetTransfers(ctx context.Context) ([]*domain.FaucetTransfer, error)
	CreateFaucetTransfer(ctx context.Context, transfer *domain.FaucetTransfer) error
	ApplyLedgerEntry(ctx context.Context, entry *domain.LedgerEntry) error
}

type WalletCrypto interface {
	GenerateDepositAccount() (string, string, error)
	RecoverPersonalSignAddress(message, signature string) (string, error)
}

type Service struct {
	repo   Repository
	chain  string
	crypto WalletCrypto
}

func NewService(repo Repository, chain string, crypto WalletCrypto) *Service {
	return &Service{repo: repo, chain: chain, crypto: crypto}
}

func (s *Service) ProvisionUser(ctx context.Context, userID, email string) error {
	userID = strings.TrimSpace(userID)
	email = strings.ToLower(strings.TrimSpace(email))
	if userID == "" || email == "" {
		return domain.ErrInvalidInput
	}

	privateKeyHex, address, err := s.crypto.GenerateDepositAccount()
	if err != nil {
		return fmt.Errorf("generate deposit address: %w", err)
	}
	now := time.Now().UTC()
	return s.repo.ProvisionUser(ctx, &domain.Account{
		UserID:        userID,
		Email:         email,
		CreatedAt:     now,
		ProvisionedAt: now,
	}, &domain.DepositAddress{
		UserID:        userID,
		Address:       address,
		PrivateKeyHex: privateKeyHex,
		Chain:         s.chain,
		CreatedAt:     now,
	})
}

func (s *Service) Balance(ctx context.Context, userID string) (*domain.Balance, error) {
	return s.repo.GetBalance(ctx, strings.TrimSpace(userID))
}

func (s *Service) DepositAddress(ctx context.Context, userID string) (*domain.DepositAddress, error) {
	return s.repo.GetDepositAddress(ctx, strings.TrimSpace(userID))
}

func (s *Service) LinkedAddresses(ctx context.Context, userID string) ([]*domain.LinkedAddress, error) {
	return s.repo.ListLinkedAddresses(ctx, strings.TrimSpace(userID))
}

func (s *Service) CreateLinkChallenge(ctx context.Context, userID, address string) (*domain.LinkChallenge, error) {
	userID = strings.TrimSpace(userID)
	address = normalizeAddress(address)
	if userID == "" || !isEthereumAddress(address) {
		return nil, domain.ErrInvalidInput
	}

	now := time.Now().UTC()
	challengeID := newID("challenge")
	challenge := &domain.LinkChallenge{
		ID:        challengeID,
		UserID:    userID,
		Address:   address,
		Challenge: fmt.Sprintf("Holdem wallet link\nuser_id: %s\naddress: %s\nchallenge_id: %s\nexpires_at: %s\nnetwork: %s", userID, address, challengeID, now.Add(linkChallengeTTL).Format(time.RFC3339), s.chain),
		CreatedAt: now,
		ExpiresAt: now.Add(linkChallengeTTL),
	}
	if err := s.repo.CreateLinkChallenge(ctx, challenge); err != nil {
		return nil, err
	}
	return challenge, nil
}

func (s *Service) ConfirmLink(ctx context.Context, userID, challengeID, address, signature string) (*domain.LinkedAddress, error) {
	userID = strings.TrimSpace(userID)
	challengeID = strings.TrimSpace(challengeID)
	address = normalizeAddress(address)
	signature = strings.TrimSpace(signature)
	if userID == "" || challengeID == "" || !isEthereumAddress(address) || signature == "" {
		return nil, domain.ErrInvalidInput
	}

	challenge, err := s.repo.FindLinkChallenge(ctx, userID, challengeID, address)
	if err != nil {
		return nil, err
	}
	if challenge.ExpiresAt.Before(time.Now().UTC()) {
		return nil, domain.ErrChallengeExpired
	}
	recoveredAddress, err := s.crypto.RecoverPersonalSignAddress(challenge.Challenge, signature)
	if err != nil {
		return nil, domain.ErrInvalidChallengeProof
	}
	if normalizeAddress(recoveredAddress) != address {
		return nil, domain.ErrInvalidChallengeProof
	}

	now := time.Now().UTC()
	linked := &domain.LinkedAddress{
		ID:         newID("linked"),
		UserID:     userID,
		Address:    address,
		CreatedAt:  now,
		VerifiedAt: now,
	}
	if err := s.repo.ConfirmLinkChallenge(ctx, challenge.ID, linked); err != nil {
		return nil, err
	}
	return linked, nil
}

func (s *Service) Deposits(ctx context.Context, userID string) ([]*domain.Deposit, error) {
	return s.repo.ListDeposits(ctx, strings.TrimSpace(userID))
}

func (s *Service) Withdrawals(ctx context.Context, userID string) ([]*domain.WithdrawalRequest, error) {
	return s.repo.ListWithdrawals(ctx, strings.TrimSpace(userID))
}

func (s *Service) RequestWithdrawal(ctx context.Context, userID, address string, amountGwei int64) (*domain.WithdrawalRequest, error) {
	userID = strings.TrimSpace(userID)
	address = normalizeAddress(address)
	if userID == "" || !isEthereumAddress(address) || amountGwei <= 0 {
		return nil, domain.ErrInvalidInput
	}

	linkedAddresses, err := s.repo.ListLinkedAddresses(ctx, userID)
	if err != nil {
		return nil, err
	}
	found := false
	for _, linked := range linkedAddresses {
		if normalizeAddress(linked.Address) == address {
			found = true
			break
		}
	}
	if !found {
		return nil, domain.ErrAddressNotLinked
	}

	now := time.Now().UTC()
	withdrawal := &domain.WithdrawalRequest{
		ID:         newID("withdrawal"),
		UserID:     userID,
		Address:    address,
		AmountGwei: amountGwei,
		Status:     domain.WithdrawalStatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	entry := &domain.LedgerEntry{
		ID:             newID("ledger"),
		UserID:         userID,
		EntryKind:      domain.EntryKindWithdrawalDebit,
		AmountGwei:     -amountGwei,
		ReferenceID:    withdrawal.ID,
		IdempotencyKey: "withdrawal:" + withdrawal.ID,
		CreatedAt:      now,
	}
	if err := s.repo.CreateWithdrawal(ctx, withdrawal, entry); err != nil {
		return nil, err
	}
	return withdrawal, nil
}

func (s *Service) CreateAdminFaucetTransfer(ctx context.Context, adminUserID, userID string, amountGwei int64) (*domain.FaucetTransfer, error) {
	adminUserID = strings.TrimSpace(adminUserID)
	userID = strings.TrimSpace(userID)
	if adminUserID == "" || userID == "" || amountGwei <= 0 {
		return nil, domain.ErrInvalidInput
	}

	address, err := s.repo.GetDepositAddress(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	transfer := &domain.FaucetTransfer{
		ID:            newID("faucet"),
		AdminUserID:   adminUserID,
		TargetUserID:  userID,
		TargetAddress: address.Address,
		AmountGwei:    amountGwei,
		Status:        domain.FaucetTransferStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repo.CreateFaucetTransfer(ctx, transfer); err != nil {
		return nil, err
	}
	return transfer, nil
}

func (s *Service) FaucetTransfers(ctx context.Context) ([]*domain.FaucetTransfer, error) {
	return s.repo.ListFaucetTransfers(ctx)
}

func (s *Service) DebitForJoin(ctx context.Context, userID, tableID string, amountGwei int64, idempotencyKey string) error {
	userID = strings.TrimSpace(userID)
	tableID = strings.TrimSpace(tableID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if userID == "" || tableID == "" || idempotencyKey == "" || amountGwei <= 0 {
		return domain.ErrInvalidInput
	}
	return s.repo.ApplyLedgerEntry(ctx, &domain.LedgerEntry{
		ID:             newID("ledger"),
		UserID:         userID,
		EntryKind:      domain.EntryKindTableBuyInDebit,
		AmountGwei:     -amountGwei,
		ReferenceID:    tableID,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now().UTC(),
	})
}

func (s *Service) CreditForCashout(ctx context.Context, userID, tableID string, amountGwei int64, idempotencyKey string) error {
	userID = strings.TrimSpace(userID)
	tableID = strings.TrimSpace(tableID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if userID == "" || tableID == "" || idempotencyKey == "" || amountGwei < 0 {
		return domain.ErrInvalidInput
	}
	if amountGwei == 0 {
		return nil
	}
	return s.repo.ApplyLedgerEntry(ctx, &domain.LedgerEntry{
		ID:             newID("ledger"),
		UserID:         userID,
		EntryKind:      domain.EntryKindTableCashout,
		AmountGwei:     amountGwei,
		ReferenceID:    tableID,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now().UTC(),
	})
}

func normalizeAddress(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

func isEthereumAddress(address string) bool {
	if len(address) != 42 || !strings.HasPrefix(address, "0x") {
		return false
	}
	for _, r := range address[2:] {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func newID(prefix string) string {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + strings.ToLower(base64.RawURLEncoding.EncodeToString(raw))
}
