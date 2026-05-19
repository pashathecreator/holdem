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

	"github.com/pashathecreator/holdem/services/auth-service/internal/domain"
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

func (r *Repository) CreateUser(ctx context.Context, user *domain.User, passwordHash string) error {
	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		userQuery, userArgs, err := r.sql.
			Insert("auth.users").
			Columns("id", "email", "is_admin", "created_at").
			Values(user.ID, user.Email, user.IsAdmin, user.CreatedAt).
			ToSql()
		if err != nil {
			return fmt.Errorf("build insert user query: %w", err)
		}
		if _, err := tx.Exec(ctx, userQuery, userArgs...); err != nil {
			if isUniqueViolation(err) {
				return domain.ErrEmailTaken
			}
			return fmt.Errorf("insert user: %w", err)
		}

		passwordQuery, passwordArgs, err := r.sql.
			Insert("auth.password_credentials").
			Columns("user_id", "password_hash").
			Values(user.ID, passwordHash).
			ToSql()
		if err != nil {
			return fmt.Errorf("build insert password query: %w", err)
		}
		if _, err := tx.Exec(ctx, passwordQuery, passwordArgs...); err != nil {
			return fmt.Errorf("insert password credentials: %w", err)
		}

		return nil
	})
}

func (r *Repository) FindUserWithPasswordByEmail(ctx context.Context, email string) (*domain.UserWithPassword, error) {
	query, args, err := r.sql.
		Select("u.id", "u.email", "u.is_admin", "u.created_at", "pc.password_hash").
		From("auth.users u").
		Join("auth.password_credentials pc ON pc.user_id = u.id").
		Where(sq.Eq{"u.email": strings.ToLower(email)}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build find user by email query: %w", err)
	}

	var user domain.User
	var passwordHash string
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&user.ID, &user.Email, &user.IsAdmin, &user.CreatedAt, &passwordHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}

	return &domain.UserWithPassword{User: &user, PasswordHash: passwordHash}, nil
}

func (r *Repository) FindUserByID(ctx context.Context, userID string) (*domain.User, error) {
	query, args, err := r.sql.
		Select("id", "email", "is_admin", "created_at").
		From("auth.users").
		Where(sq.Eq{"id": userID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build find user query: %w", err)
	}

	var user domain.User
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&user.ID, &user.Email, &user.IsAdmin, &user.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}
	return &user, nil
}

func (r *Repository) SearchUsersByEmail(ctx context.Context, query string, limit int) ([]*domain.User, error) {
	sqlQuery, args, err := r.sql.
		Select("id", "email", "is_admin", "created_at").
		From("auth.users").
		Where("email LIKE ?", "%"+strings.ToLower(query)+"%").
		OrderBy("created_at DESC").
		Limit(uint64(limit)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build search users query: %w", err)
	}

	rows, err := r.pool.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()

	users := make([]*domain.User, 0)
	for rows.Next() {
		item := &domain.User{}
		if err := rows.Scan(&item.ID, &item.Email, &item.IsAdmin, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, item)
	}
	return users, rows.Err()
}

func (r *Repository) CreateRefreshSession(ctx context.Context, session *domain.RefreshSession) error {
	query, args, err := r.sql.
		Insert("auth.refresh_sessions").
		Columns("id", "user_id", "token_hash", "expires_at").
		Values(session.ID, session.UserID, session.TokenHash, session.ExpiresAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert refresh session query: %w", err)
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert refresh session: %w", err)
	}
	return nil
}

func (r *Repository) FindRefreshSessionByTokenHash(ctx context.Context, tokenHash string) (*domain.RefreshSession, error) {
	query, args, err := r.sql.
		Select("id", "user_id", "token_hash", "expires_at", "revoked_at").
		From("auth.refresh_sessions").
		Where(sq.Eq{"token_hash": tokenHash}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build find refresh session query: %w", err)
	}

	var session domain.RefreshSession
	var revokedAt *time.Time
	if err := r.pool.QueryRow(ctx, query, args...).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.ExpiresAt,
		&revokedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("find refresh session: %w", err)
	}
	session.RevokedAt = revokedAt
	return &session, nil
}

func (r *Repository) RotateRefreshSession(ctx context.Context, oldSessionID string, newSession *domain.RefreshSession) error {
	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		now := time.Now().UTC()
		updateQuery, updateArgs, err := r.sql.
			Update("auth.refresh_sessions").
			Set("revoked_at", now).
			Where(sq.Eq{"id": oldSessionID}).
			Where("revoked_at IS NULL").
			ToSql()
		if err != nil {
			return fmt.Errorf("build revoke refresh session query: %w", err)
		}
		if _, err := tx.Exec(ctx, updateQuery, updateArgs...); err != nil {
			return fmt.Errorf("revoke refresh session: %w", err)
		}

		insertQuery, insertArgs, err := r.sql.
			Insert("auth.refresh_sessions").
			Columns("id", "user_id", "token_hash", "expires_at").
			Values(newSession.ID, newSession.UserID, newSession.TokenHash, newSession.ExpiresAt).
			ToSql()
		if err != nil {
			return fmt.Errorf("build insert rotated session query: %w", err)
		}
		if _, err := tx.Exec(ctx, insertQuery, insertArgs...); err != nil {
			return fmt.Errorf("insert rotated refresh session: %w", err)
		}

		return nil
	})
}

func (r *Repository) RevokeRefreshSession(ctx context.Context, tokenHash string) error {
	query, args, err := r.sql.
		Update("auth.refresh_sessions").
		Set("revoked_at", time.Now().UTC()).
		Where(sq.Eq{"token_hash": tokenHash}).
		Where("revoked_at IS NULL").
		ToSql()
	if err != nil {
		return fmt.Errorf("build revoke refresh session query: %w", err)
	}
	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidRefreshToken
	}
	return nil
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
