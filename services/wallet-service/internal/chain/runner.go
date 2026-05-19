package chain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ethereum/go-ethereum/core/types"

	"github.com/pashathecreator/holdem/services/wallet-service/internal/domain"
)

const chainStateScanCursor = "deposit_scan_cursor"

type Repository interface {
	ListDepositAddresses(ctx context.Context) ([]*domain.DepositAddress, error)
	GetChainState(ctx context.Context, name string) (*domain.ChainState, error)
	SaveChainState(ctx context.Context, state *domain.ChainState) error
	UpsertObservedDeposit(ctx context.Context, deposit *domain.Deposit) error
	ListPendingDeposits(ctx context.Context) ([]*domain.Deposit, error)
	ConfirmDeposit(ctx context.Context, txHash string, confirmedBlock, confirmations int64, entry *domain.LedgerEntry) error
	ListSweepCandidates(ctx context.Context) ([]*domain.Deposit, error)
	MarkDepositSweepSubmitted(ctx context.Context, txHash, sweepTxHash string, submittedAt time.Time) error
	ListSubmittedSweeps(ctx context.Context) ([]*domain.Deposit, error)
	MarkDepositSweepConfirmed(ctx context.Context, txHash string, sweptAt time.Time) error
	MarkDepositSweepFailed(ctx context.Context, txHash, reason string, failedAt time.Time) error
	ListPendingWithdrawals(ctx context.Context) ([]*domain.WithdrawalRequest, error)
	MarkWithdrawalSubmitted(ctx context.Context, withdrawalID, txHash string, nonce int64, submittedAt time.Time) error
	ListSubmittedWithdrawals(ctx context.Context) ([]*domain.WithdrawalRequest, error)
	MarkWithdrawalConfirmed(ctx context.Context, withdrawalID string, confirmedAt time.Time) error
	MarkWithdrawalFailed(ctx context.Context, withdrawalID, reason string, compensationEntry *domain.LedgerEntry, failedAt time.Time) error
	ListPendingFaucetTransfers(ctx context.Context) ([]*domain.FaucetTransfer, error)
	MarkFaucetTransferSubmitted(ctx context.Context, transferID, txHash string, nonce int64, submittedAt time.Time) error
	ListSubmittedFaucetTransfers(ctx context.Context) ([]*domain.FaucetTransfer, error)
	MarkFaucetTransferConfirmed(ctx context.Context, transferID string, confirmedAt time.Time) error
	MarkFaucetTransferFailed(ctx context.Context, transferID, reason string, failedAt time.Time) error
	GetDepositAddress(ctx context.Context, userID string) (*domain.DepositAddress, error)
}

type RunnerConfig struct {
	Confirmations       int64
	StartBlock          uint64
	ScanInterval        time.Duration
	ReceiptPollInterval time.Duration
	HotWalletAddress    string
}

type Runner struct {
	repo   Repository
	client *Client
	cfg    RunnerConfig
	logger *zap.Logger
}

func NewRunner(repo Repository, client *Client, cfg RunnerConfig, logger *zap.Logger) *Runner {
	return &Runner{
		repo:   repo,
		client: client,
		cfg:    cfg,
		logger: logger,
	}
}

func (r *Runner) Run(ctx context.Context) {
	if r == nil || r.client == nil || !r.client.Enabled() {
		<-ctx.Done()
		return
	}

	scanTicker := time.NewTicker(r.cfg.ScanInterval)
	receiptTicker := time.NewTicker(r.cfg.ReceiptPollInterval)
	defer scanTicker.Stop()
	defer receiptTicker.Stop()

	r.safeRun(ctx, r.scanDepositsAndSubmit)
	r.safeRun(ctx, r.processReceipts)

	for {
		select {
		case <-ctx.Done():
			return
		case <-scanTicker.C:
			r.safeRun(ctx, r.scanDepositsAndSubmit)
		case <-receiptTicker.C:
			r.safeRun(ctx, r.processReceipts)
		}
	}
}

func (r *Runner) safeRun(ctx context.Context, fn func(context.Context) error) {
	if err := fn(ctx); err != nil && ctx.Err() == nil {
		r.logger.Error("wallet chain worker cycle failed", zap.Error(err))
	}
}

func (r *Runner) scanDepositsAndSubmit(ctx context.Context) error {
	currentBlock, err := r.client.CurrentBlock(ctx)
	if err != nil {
		return fmt.Errorf("load current block: %w", err)
	}

	addresses, err := r.repo.ListDepositAddresses(ctx)
	if err != nil {
		return fmt.Errorf("list deposit addresses: %w", err)
	}
	state, err := r.repo.GetChainState(ctx, chainStateScanCursor)
	if err != nil {
		return fmt.Errorf("load chain state: %w", err)
	}

	if state == nil {
		initial := r.initialBlock(currentBlock)
		return r.repo.SaveChainState(ctx, &domain.ChainState{
			Name:        chainStateScanCursor,
			BlockNumber: int64(initial),
			UpdatedAt:   time.Now().UTC(),
		})
	}

	addressMap := make(map[string]string, len(addresses))
	for _, item := range addresses {
		addressMap[strings.ToLower(item.Address)] = item.UserID
	}

	startBlock := uint64(state.BlockNumber + 1)
	if startBlock <= currentBlock && len(addressMap) > 0 {
		observed, err := r.client.ScanDeposits(ctx, startBlock, currentBlock, addressMap)
		if err != nil {
			return fmt.Errorf("scan deposits: %w", err)
		}
		for _, transfer := range observed {
			now := time.Now().UTC()
			observedBlock := transfer.BlockNumber
			deposit := &domain.Deposit{
				ID:            newID("deposit"),
				UserID:        transfer.UserID,
				Address:       strings.ToLower(transfer.ToAddress),
				FromAddress:   strings.ToLower(transfer.FromAddress),
				ToAddress:     strings.ToLower(transfer.ToAddress),
				AmountGwei:    transfer.ValueGwei,
				RawAmountWei:  transfer.RawValueWei,
				Status:        domain.DepositStatusPending,
				TxHash:        transfer.TxHash,
				ObservedBlock: &observedBlock,
				Confirmations: 0,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if err := r.repo.UpsertObservedDeposit(ctx, deposit); err != nil {
				return fmt.Errorf("upsert observed deposit: %w", err)
			}
		}
	}

	if err := r.repo.SaveChainState(ctx, &domain.ChainState{
		Name:        chainStateScanCursor,
		BlockNumber: int64(currentBlock),
		UpdatedAt:   time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("save chain state: %w", err)
	}

	if err := r.confirmPendingDeposits(ctx, int64(currentBlock)); err != nil {
		return err
	}
	if err := r.submitSweeps(ctx); err != nil {
		return err
	}
	if err := r.submitWithdrawals(ctx); err != nil {
		return err
	}
	if err := r.submitFaucetTransfers(ctx); err != nil {
		return err
	}
	return nil
}

func (r *Runner) processReceipts(ctx context.Context) error {
	if err := r.confirmSweepReceipts(ctx); err != nil {
		return err
	}
	if err := r.confirmWithdrawalReceipts(ctx); err != nil {
		return err
	}
	if err := r.confirmFaucetTransferReceipts(ctx); err != nil {
		return err
	}
	return nil
}

func (r *Runner) confirmPendingDeposits(ctx context.Context, currentBlock int64) error {
	deposits, err := r.repo.ListPendingDeposits(ctx)
	if err != nil {
		return fmt.Errorf("list pending deposits: %w", err)
	}
	for _, deposit := range deposits {
		if deposit.ObservedBlock == nil {
			continue
		}
		confirmations := currentBlock - *deposit.ObservedBlock + 1
		if confirmations < r.cfg.Confirmations {
			continue
		}
		confirmedBlock := currentBlock
		if err := r.repo.ConfirmDeposit(ctx, deposit.TxHash, confirmedBlock, confirmations, &domain.LedgerEntry{
			ID:             newID("ledger"),
			UserID:         deposit.UserID,
			EntryKind:      domain.EntryKindDepositConfirmed,
			AmountGwei:     deposit.AmountGwei,
			ReferenceID:    deposit.TxHash,
			IdempotencyKey: "deposit:" + deposit.TxHash,
			CreatedAt:      time.Now().UTC(),
		}); err != nil {
			return fmt.Errorf("confirm deposit %s: %w", deposit.TxHash, err)
		}
	}
	return nil
}

func (r *Runner) submitSweeps(ctx context.Context) error {
	deposits, err := r.repo.ListSweepCandidates(ctx)
	if err != nil {
		return fmt.Errorf("list sweep candidates: %w", err)
	}
	for _, deposit := range deposits {
		depositAddress, err := r.repo.GetDepositAddress(ctx, deposit.UserID)
		if err != nil {
			return fmt.Errorf("load deposit address for sweep: %w", err)
		}
		txHash, err := r.client.SubmitSweep(ctx, depositAddress.PrivateKeyHex, r.cfg.HotWalletAddress)
		if err != nil {
			_ = r.repo.MarkDepositSweepFailed(ctx, deposit.TxHash, err.Error(), time.Now().UTC())
			continue
		}
		if err := r.repo.MarkDepositSweepSubmitted(ctx, deposit.TxHash, txHash, time.Now().UTC()); err != nil {
			return fmt.Errorf("mark sweep submitted: %w", err)
		}
	}
	return nil
}

func (r *Runner) confirmSweepReceipts(ctx context.Context) error {
	deposits, err := r.repo.ListSubmittedSweeps(ctx)
	if err != nil {
		return fmt.Errorf("list submitted sweeps: %w", err)
	}
	for _, deposit := range deposits {
		if deposit.SweepTxHash == nil {
			continue
		}
		receipt, err := r.client.TransactionReceipt(ctx, *deposit.SweepTxHash)
		if err != nil {
			return fmt.Errorf("load sweep receipt: %w", err)
		}
		if receipt == nil {
			continue
		}
		if receipt.Status == types.ReceiptStatusSuccessful {
			if err := r.repo.MarkDepositSweepConfirmed(ctx, deposit.TxHash, time.Now().UTC()); err != nil {
				return fmt.Errorf("mark sweep confirmed: %w", err)
			}
			continue
		}
		if err := r.repo.MarkDepositSweepFailed(ctx, deposit.TxHash, "sweep transaction failed", time.Now().UTC()); err != nil {
			return fmt.Errorf("mark sweep failed: %w", err)
		}
	}
	return nil
}

func (r *Runner) submitWithdrawals(ctx context.Context) error {
	withdrawals, err := r.repo.ListPendingWithdrawals(ctx)
	if err != nil {
		return fmt.Errorf("list pending withdrawals: %w", err)
	}
	for _, withdrawal := range withdrawals {
		txHash, nonce, err := r.client.SubmitWithdrawal(ctx, withdrawal.Address, withdrawal.AmountGwei)
		if err != nil {
			r.logger.Warn("submit withdrawal failed", zap.String("withdrawal_id", withdrawal.ID), zap.Error(err))
			continue
		}
		if err := r.repo.MarkWithdrawalSubmitted(ctx, withdrawal.ID, txHash, nonce, time.Now().UTC()); err != nil {
			return fmt.Errorf("mark withdrawal submitted: %w", err)
		}
	}
	return nil
}

func (r *Runner) confirmWithdrawalReceipts(ctx context.Context) error {
	withdrawals, err := r.repo.ListSubmittedWithdrawals(ctx)
	if err != nil {
		return fmt.Errorf("list submitted withdrawals: %w", err)
	}
	for _, withdrawal := range withdrawals {
		if withdrawal.TxHash == nil {
			continue
		}
		receipt, err := r.client.TransactionReceipt(ctx, *withdrawal.TxHash)
		if err != nil {
			return fmt.Errorf("load withdrawal receipt: %w", err)
		}
		if receipt == nil {
			continue
		}
		if receipt.Status == types.ReceiptStatusSuccessful {
			if err := r.repo.MarkWithdrawalConfirmed(ctx, withdrawal.ID, time.Now().UTC()); err != nil {
				return fmt.Errorf("mark withdrawal confirmed: %w", err)
			}
			continue
		}
		if err := r.repo.MarkWithdrawalFailed(ctx, withdrawal.ID, "withdrawal transaction failed", &domain.LedgerEntry{
			ID:             newID("ledger"),
			UserID:         withdrawal.UserID,
			EntryKind:      domain.EntryKindWithdrawalCompensation,
			AmountGwei:     withdrawal.AmountGwei,
			ReferenceID:    withdrawal.ID,
			IdempotencyKey: "withdrawal-compensation:" + withdrawal.ID,
			CreatedAt:      time.Now().UTC(),
		}, time.Now().UTC()); err != nil {
			return fmt.Errorf("mark withdrawal failed: %w", err)
		}
	}
	return nil
}

func (r *Runner) submitFaucetTransfers(ctx context.Context) error {
	transfers, err := r.repo.ListPendingFaucetTransfers(ctx)
	if err != nil {
		return fmt.Errorf("list pending faucet transfers: %w", err)
	}
	for _, transfer := range transfers {
		txHash, nonce, err := r.client.SubmitWithdrawal(ctx, transfer.TargetAddress, transfer.AmountGwei)
		if err != nil {
			r.logger.Warn("submit faucet transfer failed", zap.String("faucet_transfer_id", transfer.ID), zap.Error(err))
			continue
		}
		if err := r.repo.MarkFaucetTransferSubmitted(ctx, transfer.ID, txHash, nonce, time.Now().UTC()); err != nil {
			return fmt.Errorf("mark faucet transfer submitted: %w", err)
		}
	}
	return nil
}

func (r *Runner) confirmFaucetTransferReceipts(ctx context.Context) error {
	transfers, err := r.repo.ListSubmittedFaucetTransfers(ctx)
	if err != nil {
		return fmt.Errorf("list submitted faucet transfers: %w", err)
	}
	for _, transfer := range transfers {
		if transfer.TxHash == nil {
			continue
		}
		receipt, err := r.client.TransactionReceipt(ctx, *transfer.TxHash)
		if err != nil {
			return fmt.Errorf("load faucet transfer receipt: %w", err)
		}
		if receipt == nil {
			continue
		}
		if receipt.Status == types.ReceiptStatusSuccessful {
			if err := r.repo.MarkFaucetTransferConfirmed(ctx, transfer.ID, time.Now().UTC()); err != nil {
				return fmt.Errorf("mark faucet transfer confirmed: %w", err)
			}
			continue
		}
		if err := r.repo.MarkFaucetTransferFailed(ctx, transfer.ID, "faucet transfer transaction failed", time.Now().UTC()); err != nil {
			return fmt.Errorf("mark faucet transfer failed: %w", err)
		}
	}
	return nil
}

func (r *Runner) initialBlock(currentBlock uint64) uint64 {
	if r.cfg.StartBlock > 0 {
		return r.cfg.StartBlock - 1
	}
	if currentBlock <= uint64(r.cfg.Confirmations) {
		return 0
	}
	return currentBlock - uint64(r.cfg.Confirmations)
}

func newID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
