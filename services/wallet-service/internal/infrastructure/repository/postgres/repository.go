package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pashathecreator/holdem/services/wallet-service/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
	sql  sq.StatementBuilderType
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
		sql:  sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *Repository) ProvisionUser(ctx context.Context, account *domain.Account, depositAddress *domain.DepositAddress) error {
	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		accountQuery, accountArgs, err := r.sql.
			Insert("wallet.wallet_accounts").
			Columns("user_id", "email", "created_at", "provisioned_at").
			Values(account.UserID, account.Email, account.CreatedAt, account.ProvisionedAt).
			Suffix("ON CONFLICT (user_id) DO NOTHING").
			ToSql()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, accountQuery, accountArgs...); err != nil {
			return fmt.Errorf("insert wallet account: %w", err)
		}

		balanceQuery, balanceArgs, err := r.sql.
			Insert("wallet.wallet_balances").
			Columns("user_id", "available_gwei", "updated_at").
			Values(account.UserID, 0, account.ProvisionedAt).
			Suffix("ON CONFLICT (user_id) DO NOTHING").
			ToSql()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, balanceQuery, balanceArgs...); err != nil {
			return fmt.Errorf("insert wallet balance: %w", err)
		}

		addressQuery, addressArgs, err := r.sql.
			Insert("wallet.wallet_deposit_addresses").
			Columns("user_id", "address", "private_key_hex", "chain", "created_at").
			Values(depositAddress.UserID, depositAddress.Address, depositAddress.PrivateKeyHex, depositAddress.Chain, depositAddress.CreatedAt).
			Suffix("ON CONFLICT (user_id) DO NOTHING").
			ToSql()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, addressQuery, addressArgs...); err != nil {
			return fmt.Errorf("insert deposit address: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetBalance(ctx context.Context, userID string) (*domain.Balance, error) {
	query, args, err := r.sql.
		Select("user_id", "available_gwei", "updated_at").
		From("wallet.wallet_balances").
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return nil, err
	}
	var balance domain.Balance
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&balance.UserID, &balance.AvailableGwei, &balance.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, fmt.Errorf("find balance: %w", err)
	}
	return &balance, nil
}

func (r *Repository) GetDepositAddress(ctx context.Context, userID string) (*domain.DepositAddress, error) {
	query, args, err := r.sql.
		Select("user_id", "address", "private_key_hex", "chain", "created_at", "last_sweep_tx_hash", "last_sweep_at", "last_observed_block").
		From("wallet.wallet_deposit_addresses").
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return nil, err
	}
	var deposit domain.DepositAddress
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&deposit.UserID, &deposit.Address, &deposit.PrivateKeyHex, &deposit.Chain, &deposit.CreatedAt, &deposit.LastSweepTxHash, &deposit.LastSweepAt, &deposit.LastObservedBlock); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, fmt.Errorf("find deposit address: %w", err)
	}
	return &deposit, nil
}

func (r *Repository) ListLinkedAddresses(ctx context.Context, userID string) ([]*domain.LinkedAddress, error) {
	query, args, err := r.sql.
		Select("id", "user_id", "address", "created_at", "verified_at").
		From("wallet.wallet_linked_addresses").
		Where(sq.Eq{"user_id": userID}).
		OrderBy("created_at ASC").
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list linked addresses: %w", err)
	}
	defer rows.Close()

	result := make([]*domain.LinkedAddress, 0)
	for rows.Next() {
		item := &domain.LinkedAddress{}
		if err := rows.Scan(&item.ID, &item.UserID, &item.Address, &item.CreatedAt, &item.VerifiedAt); err != nil {
			return nil, fmt.Errorf("scan linked address: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) CreateLinkChallenge(ctx context.Context, challenge *domain.LinkChallenge) error {
	query, args, err := r.sql.
		Insert("wallet.wallet_link_challenges").
		Columns("id", "user_id", "address", "challenge", "created_at", "expires_at").
		Values(challenge.ID, challenge.UserID, challenge.Address, challenge.Challenge, challenge.CreatedAt, challenge.ExpiresAt).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert link challenge: %w", err)
	}
	return nil
}

func (r *Repository) FindLinkChallenge(ctx context.Context, userID, challengeID, address string) (*domain.LinkChallenge, error) {
	query, args, err := r.sql.
		Select("id", "user_id", "address", "challenge", "created_at", "expires_at", "consumed_at").
		From("wallet.wallet_link_challenges").
		Where(sq.Eq{"id": challengeID, "user_id": userID, "address": strings.ToLower(address)}).
		ToSql()
	if err != nil {
		return nil, err
	}
	item := &domain.LinkChallenge{}
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&item.ID, &item.UserID, &item.Address, &item.Challenge, &item.CreatedAt, &item.ExpiresAt, &item.ConsumedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrChallengeNotFound
		}
		return nil, fmt.Errorf("find link challenge: %w", err)
	}
	if item.ConsumedAt != nil {
		return nil, domain.ErrChallengeNotFound
	}
	return item, nil
}

func (r *Repository) ConfirmLinkChallenge(ctx context.Context, challengeID string, linked *domain.LinkedAddress) error {
	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		now := time.Now().UTC()
		updateQuery, updateArgs, err := r.sql.
			Update("wallet.wallet_link_challenges").
			Set("consumed_at", now).
			Where(sq.Eq{"id": challengeID}).
			Where("consumed_at IS NULL").
			ToSql()
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, updateQuery, updateArgs...)
		if err != nil {
			return fmt.Errorf("consume link challenge: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrChallengeNotFound
		}

		insertQuery, insertArgs, err := r.sql.
			Insert("wallet.wallet_linked_addresses").
			Columns("id", "user_id", "address", "created_at", "verified_at").
			Values(linked.ID, linked.UserID, linked.Address, linked.CreatedAt, linked.VerifiedAt).
			ToSql()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, insertQuery, insertArgs...); err != nil {
			if isUniqueViolation(err) {
				return domain.ErrAddressAlreadyLinked
			}
			return fmt.Errorf("insert linked address: %w", err)
		}
		return nil
	})
}

func (r *Repository) ListDeposits(ctx context.Context, userID string) ([]*domain.Deposit, error) {
	query, args, err := r.sql.
		Select(depositColumns...).
		From("wallet.wallet_deposits").
		Where(sq.Eq{"user_id": userID}).
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list deposits: %w", err)
	}
	defer rows.Close()

	var result []*domain.Deposit
	for rows.Next() {
		item := &domain.Deposit{}
		if err := rows.Scan(&item.ID, &item.UserID, &item.Address, &item.FromAddress, &item.ToAddress, &item.AmountGwei, &item.RawAmountWei, &item.Status, &item.TxHash, &item.ObservedBlock, &item.ConfirmedBlock, &item.Confirmations, &item.SweepTxHash, &item.SweepStatus, &item.SweepSubmittedAt, &item.SweptAt, &item.SweepFailureReason, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan deposit: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) CreateConfirmedDeposit(ctx context.Context, deposit *domain.Deposit, entry *domain.LedgerEntry) error {
	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := r.applyLedgerEntryTx(ctx, tx, entry); err != nil {
			return err
		}
		query, args, err := r.sql.
			Insert("wallet.wallet_deposits").
			Columns("id", "user_id", "address", "from_address", "to_address", "amount_gwei", "raw_amount_wei", "status", "tx_hash", "observed_block", "confirmed_block", "confirmations", "sweep_tx_hash", "sweep_status", "sweep_submitted_at", "swept_at", "sweep_failure_reason", "created_at", "updated_at").
			Values(deposit.ID, deposit.UserID, deposit.Address, deposit.FromAddress, deposit.ToAddress, deposit.AmountGwei, deposit.RawAmountWei, deposit.Status, deposit.TxHash, deposit.ObservedBlock, deposit.ConfirmedBlock, deposit.Confirmations, deposit.SweepTxHash, deposit.SweepStatus, deposit.SweepSubmittedAt, deposit.SweptAt, deposit.SweepFailureReason, deposit.CreatedAt, deposit.UpdatedAt).
			Suffix("ON CONFLICT (tx_hash) DO NOTHING").
			ToSql()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("insert deposit: %w", err)
		}
		return nil
	})
}

func (r *Repository) ListWithdrawals(ctx context.Context, userID string) ([]*domain.WithdrawalRequest, error) {
	query, args, err := r.sql.
		Select("id", "user_id", "address", "amount_gwei", "status", "tx_hash", "nonce", "submitted_at", "confirmed_at", "failure_reason", "created_at", "updated_at").
		From("wallet.wallet_withdrawal_requests").
		Where(sq.Eq{"user_id": userID}).
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}
	defer rows.Close()

	var result []*domain.WithdrawalRequest
	for rows.Next() {
		item := &domain.WithdrawalRequest{}
		if err := rows.Scan(&item.ID, &item.UserID, &item.Address, &item.AmountGwei, &item.Status, &item.TxHash, &item.Nonce, &item.SubmittedAt, &item.ConfirmedAt, &item.FailureReason, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan withdrawal: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) CreateWithdrawal(ctx context.Context, withdrawal *domain.WithdrawalRequest, entry *domain.LedgerEntry) error {
	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := r.applyLedgerEntryTx(ctx, tx, entry); err != nil {
			return err
		}
		query, args, err := r.sql.
			Insert("wallet.wallet_withdrawal_requests").
			Columns("id", "user_id", "address", "amount_gwei", "status", "tx_hash", "nonce", "submitted_at", "confirmed_at", "failure_reason", "created_at", "updated_at").
			Values(withdrawal.ID, withdrawal.UserID, withdrawal.Address, withdrawal.AmountGwei, withdrawal.Status, withdrawal.TxHash, withdrawal.Nonce, withdrawal.SubmittedAt, withdrawal.ConfirmedAt, withdrawal.FailureReason, withdrawal.CreatedAt, withdrawal.UpdatedAt).
			ToSql()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("insert withdrawal: %w", err)
		}
		return nil
	})
}

func (r *Repository) ListFaucetTransfers(ctx context.Context) ([]*domain.FaucetTransfer, error) {
	query, args, err := r.sql.
		Select("id", "admin_user_id", "target_user_id", "target_address", "amount_gwei", "status", "tx_hash", "nonce", "submitted_at", "confirmed_at", "failure_reason", "created_at", "updated_at").
		From("wallet.wallet_faucet_transfers").
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, err
	}
	return r.scanFaucetTransfers(ctx, query, args...)
}

func (r *Repository) CreateFaucetTransfer(ctx context.Context, transfer *domain.FaucetTransfer) error {
	query, args, err := r.sql.
		Insert("wallet.wallet_faucet_transfers").
		Columns("id", "admin_user_id", "target_user_id", "target_address", "amount_gwei", "status", "tx_hash", "nonce", "submitted_at", "confirmed_at", "failure_reason", "created_at", "updated_at").
		Values(transfer.ID, transfer.AdminUserID, transfer.TargetUserID, transfer.TargetAddress, transfer.AmountGwei, transfer.Status, transfer.TxHash, transfer.Nonce, transfer.SubmittedAt, transfer.ConfirmedAt, transfer.FailureReason, transfer.CreatedAt, transfer.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert faucet transfer: %w", err)
	}
	return nil
}

func (r *Repository) ApplyLedgerEntry(ctx context.Context, entry *domain.LedgerEntry) error {
	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		return r.applyLedgerEntryTx(ctx, tx, entry)
	})
}

func (r *Repository) applyLedgerEntryTx(ctx context.Context, tx pgx.Tx, entry *domain.LedgerEntry) error {
	existsQuery, existsArgs, err := r.sql.
		Select("id").
		From("wallet.wallet_ledger_entries").
		Where(sq.Eq{"idempotency_key": entry.IdempotencyKey}).
		ToSql()
	if err != nil {
		return err
	}
	var existing string
	if err := tx.QueryRow(ctx, existsQuery, existsArgs...).Scan(&existing); err == nil {
		return nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("find existing ledger entry: %w", err)
	}

	var balance int64
	selectQuery, selectArgs, err := r.sql.
		Select("available_gwei").
		From("wallet.wallet_balances").
		Where(sq.Eq{"user_id": entry.UserID}).
		Suffix("FOR UPDATE").
		ToSql()
	if err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, selectQuery, selectArgs...).Scan(&balance); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrAccountNotFound
		}
		return fmt.Errorf("lock wallet balance: %w", err)
	}

	newBalance := balance + entry.AmountGwei
	if newBalance < 0 {
		return domain.ErrInsufficientFunds
	}

	insertQuery, insertArgs, err := r.sql.
		Insert("wallet.wallet_ledger_entries").
		Columns("id", "user_id", "entry_kind", "amount_gwei", "reference_id", "idempotency_key", "created_at").
		Values(entry.ID, entry.UserID, entry.EntryKind, entry.AmountGwei, entry.ReferenceID, entry.IdempotencyKey, entry.CreatedAt).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, insertQuery, insertArgs...); err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("insert ledger entry: %w", err)
	}

	updateQuery, updateArgs, err := r.sql.
		Update("wallet.wallet_balances").
		Set("available_gwei", newBalance).
		Set("updated_at", entry.CreatedAt).
		Where(sq.Eq{"user_id": entry.UserID}).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, updateQuery, updateArgs...); err != nil {
		return fmt.Errorf("update wallet balance: %w", err)
	}
	return nil
}

func (r *Repository) scanFaucetTransfers(ctx context.Context, query string, args ...interface{}) ([]*domain.FaucetTransfer, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*domain.FaucetTransfer, 0)
	for rows.Next() {
		item := &domain.FaucetTransfer{}
		if err := rows.Scan(&item.ID, &item.AdminUserID, &item.TargetUserID, &item.TargetAddress, &item.AmountGwei, &item.Status, &item.TxHash, &item.Nonce, &item.SubmittedAt, &item.ConfirmedAt, &item.FailureReason, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func withTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
