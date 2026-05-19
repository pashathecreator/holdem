package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/pashathecreator/holdem/services/wallet-service/internal/domain"
)

func (r *Repository) ListDepositAddresses(ctx context.Context) ([]*domain.DepositAddress, error) {
	query, args, err := r.sql.
		Select("user_id", "address", "private_key_hex", "chain", "created_at", "last_sweep_tx_hash", "last_sweep_at", "last_observed_block").
		From("wallet.wallet_deposit_addresses").
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list deposit addresses: %w", err)
	}
	defer rows.Close()

	var result []*domain.DepositAddress
	for rows.Next() {
		item := &domain.DepositAddress{}
		if err := rows.Scan(&item.UserID, &item.Address, &item.PrivateKeyHex, &item.Chain, &item.CreatedAt, &item.LastSweepTxHash, &item.LastSweepAt, &item.LastObservedBlock); err != nil {
			return nil, fmt.Errorf("scan deposit address: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) GetChainState(ctx context.Context, name string) (*domain.ChainState, error) {
	query, args, err := r.sql.
		Select("name", "block_number", "updated_at").
		From("wallet.wallet_chain_state").
		Where(sq.Eq{"name": name}).
		ToSql()
	if err != nil {
		return nil, err
	}
	item := &domain.ChainState{}
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&item.Name, &item.BlockNumber, &item.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get chain state: %w", err)
	}
	return item, nil
}

func (r *Repository) SaveChainState(ctx context.Context, state *domain.ChainState) error {
	query, args, err := r.sql.
		Insert("wallet.wallet_chain_state").
		Columns("name", "block_number", "updated_at").
		Values(state.Name, state.BlockNumber, state.UpdatedAt).
		Suffix("ON CONFLICT (name) DO UPDATE SET block_number = EXCLUDED.block_number, updated_at = EXCLUDED.updated_at").
		ToSql()
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("save chain state: %w", err)
	}
	return nil
}

func (r *Repository) UpsertObservedDeposit(ctx context.Context, deposit *domain.Deposit) error {
	query, args, err := r.sql.
		Insert("wallet.wallet_deposits").
		Columns("id", "user_id", "address", "from_address", "to_address", "amount_gwei", "raw_amount_wei", "status", "tx_hash", "observed_block", "confirmed_block", "confirmations", "sweep_tx_hash", "sweep_status", "sweep_submitted_at", "swept_at", "sweep_failure_reason", "created_at", "updated_at").
		Values(deposit.ID, deposit.UserID, deposit.Address, deposit.FromAddress, deposit.ToAddress, deposit.AmountGwei, deposit.RawAmountWei, deposit.Status, deposit.TxHash, deposit.ObservedBlock, deposit.ConfirmedBlock, deposit.Confirmations, deposit.SweepTxHash, deposit.SweepStatus, deposit.SweepSubmittedAt, deposit.SweptAt, deposit.SweepFailureReason, deposit.CreatedAt, deposit.UpdatedAt).
		Suffix("ON CONFLICT (tx_hash) DO UPDATE SET observed_block = EXCLUDED.observed_block, confirmations = EXCLUDED.confirmations, updated_at = EXCLUDED.updated_at, from_address = EXCLUDED.from_address, to_address = EXCLUDED.to_address, raw_amount_wei = EXCLUDED.raw_amount_wei").
		ToSql()
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert observed deposit: %w", err)
	}

	updateAddressQuery, updateAddressArgs, err := r.sql.
		Update("wallet.wallet_deposit_addresses").
		Set("last_observed_block", deposit.ObservedBlock).
		Where(sq.Eq{"user_id": deposit.UserID}).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, updateAddressQuery, updateAddressArgs...); err != nil {
		return fmt.Errorf("update deposit address observed block: %w", err)
	}
	return nil
}

func (r *Repository) ListPendingDeposits(ctx context.Context) ([]*domain.Deposit, error) {
	return r.listDepositsByStatus(ctx, domain.DepositStatusPending)
}

func (r *Repository) ConfirmDeposit(ctx context.Context, txHash string, confirmedBlock, confirmations int64, entry *domain.LedgerEntry) error {
	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		updateQuery, updateArgs, err := r.sql.
			Update("wallet.wallet_deposits").
			Set("status", domain.DepositStatusConfirmed).
			Set("confirmed_block", confirmedBlock).
			Set("confirmations", confirmations).
			Set("updated_at", entry.CreatedAt).
			Where(sq.Eq{"tx_hash": txHash}).
			Where("status <> ?", domain.DepositStatusConfirmed).
			ToSql()
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, updateQuery, updateArgs...)
		if err != nil {
			return fmt.Errorf("confirm deposit: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		return r.applyLedgerEntryTx(ctx, tx, entry)
	})
}

func (r *Repository) ListSweepCandidates(ctx context.Context) ([]*domain.Deposit, error) {
	query, args, err := r.sql.
		Select(depositColumns...).
		From("wallet.wallet_deposits").
		Where(sq.Eq{"status": domain.DepositStatusConfirmed}).
		Where("(sweep_status = '' OR sweep_status IS NULL OR sweep_status = ?)", domain.SweepStatusFailed).
		OrderBy("updated_at ASC").
		ToSql()
	if err != nil {
		return nil, err
	}
	return r.scanDeposits(ctx, query, args...)
}

func (r *Repository) MarkDepositSweepSubmitted(ctx context.Context, txHash, sweepTxHash string, submittedAt time.Time) error {
	query, args, err := r.sql.
		Update("wallet.wallet_deposits").
		Set("sweep_tx_hash", sweepTxHash).
		Set("sweep_status", domain.SweepStatusSubmitted).
		Set("sweep_submitted_at", submittedAt).
		Set("updated_at", submittedAt).
		Where(sq.Eq{"tx_hash": txHash}).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark deposit sweep submitted: %w", err)
	}
	return nil
}

func (r *Repository) ListSubmittedSweeps(ctx context.Context) ([]*domain.Deposit, error) {
	query, args, err := r.sql.
		Select(depositColumns...).
		From("wallet.wallet_deposits").
		Where(sq.Eq{"sweep_status": domain.SweepStatusSubmitted}).
		OrderBy("updated_at ASC").
		ToSql()
	if err != nil {
		return nil, err
	}
	return r.scanDeposits(ctx, query, args...)
}

func (r *Repository) MarkDepositSweepConfirmed(ctx context.Context, txHash string, sweptAt time.Time) error {
	query, args, err := r.sql.
		Update("wallet.wallet_deposits").
		Set("sweep_status", domain.SweepStatusConfirmed).
		Set("swept_at", sweptAt).
		Set("updated_at", sweptAt).
		Where(sq.Eq{"tx_hash": txHash}).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark deposit sweep confirmed: %w", err)
	}
	return nil
}

func (r *Repository) MarkDepositSweepFailed(ctx context.Context, txHash, reason string, failedAt time.Time) error {
	query, args, err := r.sql.
		Update("wallet.wallet_deposits").
		Set("sweep_status", domain.SweepStatusFailed).
		Set("sweep_failure_reason", reason).
		Set("updated_at", failedAt).
		Where(sq.Eq{"tx_hash": txHash}).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark deposit sweep failed: %w", err)
	}
	return nil
}

func (r *Repository) ListPendingWithdrawals(ctx context.Context) ([]*domain.WithdrawalRequest, error) {
	return r.listWithdrawalsByStatus(ctx, domain.WithdrawalStatusPending)
}

func (r *Repository) ListPendingFaucetTransfers(ctx context.Context) ([]*domain.FaucetTransfer, error) {
	return r.listFaucetTransfersByStatus(ctx, domain.FaucetTransferStatusPending)
}

func (r *Repository) MarkFaucetTransferSubmitted(ctx context.Context, transferID, txHash string, nonce int64, submittedAt time.Time) error {
	query, args, err := r.sql.
		Update("wallet.wallet_faucet_transfers").
		Set("status", domain.FaucetTransferStatusSubmitted).
		Set("tx_hash", txHash).
		Set("nonce", nonce).
		Set("submitted_at", submittedAt).
		Set("updated_at", submittedAt).
		Where(sq.Eq{"id": transferID}).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark faucet transfer submitted: %w", err)
	}
	return nil
}

func (r *Repository) ListSubmittedFaucetTransfers(ctx context.Context) ([]*domain.FaucetTransfer, error) {
	return r.listFaucetTransfersByStatus(ctx, domain.FaucetTransferStatusSubmitted)
}

func (r *Repository) MarkFaucetTransferConfirmed(ctx context.Context, transferID string, confirmedAt time.Time) error {
	query, args, err := r.sql.
		Update("wallet.wallet_faucet_transfers").
		Set("status", domain.FaucetTransferStatusConfirmed).
		Set("confirmed_at", confirmedAt).
		Set("updated_at", confirmedAt).
		Where(sq.Eq{"id": transferID}).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark faucet transfer confirmed: %w", err)
	}
	return nil
}

func (r *Repository) MarkFaucetTransferFailed(ctx context.Context, transferID, reason string, failedAt time.Time) error {
	query, args, err := r.sql.
		Update("wallet.wallet_faucet_transfers").
		Set("status", domain.FaucetTransferStatusFailed).
		Set("failure_reason", reason).
		Set("updated_at", failedAt).
		Where(sq.Eq{"id": transferID}).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark faucet transfer failed: %w", err)
	}
	return nil
}

func (r *Repository) MarkWithdrawalSubmitted(ctx context.Context, withdrawalID, txHash string, nonce int64, submittedAt time.Time) error {
	query, args, err := r.sql.
		Update("wallet.wallet_withdrawal_requests").
		Set("status", domain.WithdrawalStatusSubmitted).
		Set("tx_hash", txHash).
		Set("nonce", nonce).
		Set("submitted_at", submittedAt).
		Set("updated_at", submittedAt).
		Where(sq.Eq{"id": withdrawalID}).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark withdrawal submitted: %w", err)
	}
	return nil
}

func (r *Repository) ListSubmittedWithdrawals(ctx context.Context) ([]*domain.WithdrawalRequest, error) {
	return r.listWithdrawalsByStatus(ctx, domain.WithdrawalStatusSubmitted)
}

func (r *Repository) MarkWithdrawalConfirmed(ctx context.Context, withdrawalID string, confirmedAt time.Time) error {
	query, args, err := r.sql.
		Update("wallet.wallet_withdrawal_requests").
		Set("status", domain.WithdrawalStatusConfirmed).
		Set("confirmed_at", confirmedAt).
		Set("updated_at", confirmedAt).
		Where(sq.Eq{"id": withdrawalID}).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark withdrawal confirmed: %w", err)
	}
	return nil
}

func (r *Repository) MarkWithdrawalFailed(ctx context.Context, withdrawalID, reason string, compensationEntry *domain.LedgerEntry, failedAt time.Time) error {
	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		updateQuery, updateArgs, err := r.sql.
			Update("wallet.wallet_withdrawal_requests").
			Set("status", domain.WithdrawalStatusFailed).
			Set("failure_reason", reason).
			Set("updated_at", failedAt).
			Where(sq.Eq{"id": withdrawalID}).
			Where("status <> ?", domain.WithdrawalStatusFailed).
			ToSql()
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, updateQuery, updateArgs...)
		if err != nil {
			return fmt.Errorf("mark withdrawal failed: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		return r.applyLedgerEntryTx(ctx, tx, compensationEntry)
	})
}

var depositColumns = []string{
	"id", "user_id", "address", "from_address", "to_address", "amount_gwei", "raw_amount_wei", "status", "tx_hash", "observed_block", "confirmed_block", "confirmations", "sweep_tx_hash", "sweep_status", "sweep_submitted_at", "swept_at", "sweep_failure_reason", "created_at", "updated_at",
}

func (r *Repository) listDepositsByStatus(ctx context.Context, status string) ([]*domain.Deposit, error) {
	query, args, err := r.sql.
		Select(depositColumns...).
		From("wallet.wallet_deposits").
		Where(sq.Eq{"status": status}).
		OrderBy("updated_at ASC").
		ToSql()
	if err != nil {
		return nil, err
	}
	return r.scanDeposits(ctx, query, args...)
}

func (r *Repository) scanDeposits(ctx context.Context, query string, args ...interface{}) ([]*domain.Deposit, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.Deposit
	for rows.Next() {
		item := &domain.Deposit{}
		if err := rows.Scan(&item.ID, &item.UserID, &item.Address, &item.FromAddress, &item.ToAddress, &item.AmountGwei, &item.RawAmountWei, &item.Status, &item.TxHash, &item.ObservedBlock, &item.ConfirmedBlock, &item.Confirmations, &item.SweepTxHash, &item.SweepStatus, &item.SweepSubmittedAt, &item.SweptAt, &item.SweepFailureReason, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) listWithdrawalsByStatus(ctx context.Context, status string) ([]*domain.WithdrawalRequest, error) {
	query, args, err := r.sql.
		Select("id", "user_id", "address", "amount_gwei", "status", "tx_hash", "nonce", "submitted_at", "confirmed_at", "failure_reason", "created_at", "updated_at").
		From("wallet.wallet_withdrawal_requests").
		Where(sq.Eq{"status": status}).
		OrderBy("updated_at ASC").
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.WithdrawalRequest
	for rows.Next() {
		item := &domain.WithdrawalRequest{}
		if err := rows.Scan(&item.ID, &item.UserID, &item.Address, &item.AmountGwei, &item.Status, &item.TxHash, &item.Nonce, &item.SubmittedAt, &item.ConfirmedAt, &item.FailureReason, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) listFaucetTransfersByStatus(ctx context.Context, status string) ([]*domain.FaucetTransfer, error) {
	query, args, err := r.sql.
		Select("id", "admin_user_id", "target_user_id", "target_address", "amount_gwei", "status", "tx_hash", "nonce", "submitted_at", "confirmed_at", "failure_reason", "created_at", "updated_at").
		From("wallet.wallet_faucet_transfers").
		Where(sq.Eq{"status": status}).
		OrderBy("updated_at ASC").
		ToSql()
	if err != nil {
		return nil, err
	}
	return r.scanFaucetTransfers(ctx, query, args...)
}
