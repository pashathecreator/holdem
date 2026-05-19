package domain

import (
	"errors"
	"time"
)

const (
	EntryKindDepositConfirmed       = "deposit_confirmed"
	EntryKindTableBuyInDebit        = "table_buy_in_debit"
	EntryKindTableCashout           = "table_cashout_credit"
	EntryKindWithdrawalDebit        = "withdrawal_debit"
	EntryKindWithdrawalCompensation = "withdrawal_compensation_credit"

	DepositStatusPending   = "pending"
	DepositStatusConfirmed = "confirmed"

	SweepStatusPending   = "pending"
	SweepStatusSubmitted = "submitted"
	SweepStatusConfirmed = "confirmed"
	SweepStatusFailed    = "failed"

	WithdrawalStatusPending   = "pending"
	WithdrawalStatusSubmitted = "submitted"
	WithdrawalStatusConfirmed = "confirmed"
	WithdrawalStatusFailed    = "failed"

	FaucetTransferStatusPending   = "pending"
	FaucetTransferStatusSubmitted = "submitted"
	FaucetTransferStatusConfirmed = "confirmed"
	FaucetTransferStatusFailed    = "failed"
)

type Account struct {
	UserID        string
	Email         string
	CreatedAt     time.Time
	ProvisionedAt time.Time
}

type Balance struct {
	UserID        string
	AvailableGwei int64
	UpdatedAt     time.Time
}

type LedgerEntry struct {
	ID             string
	UserID         string
	EntryKind      string
	AmountGwei     int64
	ReferenceID    string
	IdempotencyKey string
	CreatedAt      time.Time
}

type DepositAddress struct {
	UserID            string
	Address           string
	PrivateKeyHex     string
	Chain             string
	CreatedAt         time.Time
	LastSweepTxHash   *string
	LastSweepAt       *time.Time
	LastObservedBlock *int64
}

type LinkChallenge struct {
	ID         string
	UserID     string
	Address    string
	Challenge  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}

type LinkedAddress struct {
	ID         string
	UserID     string
	Address    string
	CreatedAt  time.Time
	VerifiedAt time.Time
}

type Deposit struct {
	ID                 string
	UserID             string
	Address            string
	FromAddress        string
	ToAddress          string
	AmountGwei         int64
	RawAmountWei       string
	Status             string
	TxHash             string
	ObservedBlock      *int64
	ConfirmedBlock     *int64
	Confirmations      int64
	SweepTxHash        *string
	SweepStatus        string
	SweepSubmittedAt   *time.Time
	SweptAt            *time.Time
	SweepFailureReason *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type WithdrawalRequest struct {
	ID            string
	UserID        string
	Address       string
	AmountGwei    int64
	Status        string
	TxHash        *string
	Nonce         *int64
	SubmittedAt   *time.Time
	ConfirmedAt   *time.Time
	FailureReason *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type FaucetTransfer struct {
	ID            string
	AdminUserID   string
	TargetUserID  string
	TargetAddress string
	AmountGwei    int64
	Status        string
	TxHash        *string
	Nonce         *int64
	SubmittedAt   *time.Time
	ConfirmedAt   *time.Time
	FailureReason *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ChainState struct {
	Name        string
	BlockNumber int64
	UpdatedAt   time.Time
}

var (
	ErrInvalidInput          = errors.New("invalid input")
	ErrAccountNotFound       = errors.New("wallet account not found")
	ErrInsufficientFunds     = errors.New("insufficient wallet balance")
	ErrAddressAlreadyLinked  = errors.New("address already linked")
	ErrAddressNotLinked      = errors.New("address not linked")
	ErrChallengeNotFound     = errors.New("link challenge not found")
	ErrChallengeExpired      = errors.New("link challenge expired")
	ErrInvalidChallengeProof = errors.New("invalid challenge proof")
)
