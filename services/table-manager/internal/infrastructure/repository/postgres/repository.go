package postgres

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pashathecreator/holdem/services/table-manager/internal/domain"
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

func (r *Repository) CreateTable(ctx context.Context, table *domain.Table) error {
	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		query, args, err := r.sql.
			Insert("tables").
			Columns("id", "name", "seat_count", "status", "small_blind", "big_blind", "button", "active_hand_id").
			Values(table.ID, table.Name, table.SeatCount, string(table.Status), table.SmallBlind, table.BigBlind, table.Button, nullableString(table.ActiveHandID)).
			ToSql()
		if err != nil {
			return fmt.Errorf("build insert table query: %w", err)
		}
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("insert table: %w", err)
		}

		for _, seat := range table.Seats {
			if err := insertSeat(ctx, tx, r.sql, table.ID, seat); err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *Repository) SaveTable(ctx context.Context, table *domain.Table) error {
	_, err := r.FindTable(ctx, table.ID)
	if err != nil {
		return err
	}

	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		query, args, err := r.sql.
			Update("tables").
			Set("name", table.Name).
			Set("seat_count", table.SeatCount).
			Set("status", string(table.Status)).
			Set("small_blind", table.SmallBlind).
			Set("big_blind", table.BigBlind).
			Set("button", table.Button).
			Set("active_hand_id", nullableString(table.ActiveHandID)).
			Where(sq.Eq{"id": table.ID}).
			ToSql()
		if err != nil {
			return fmt.Errorf("build update table query: %w", err)
		}
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("update table: %w", err)
		}

		deleteQuery, deleteArgs, err := r.sql.Delete("table_seats").Where(sq.Eq{"table_id": table.ID}).ToSql()
		if err != nil {
			return fmt.Errorf("build delete seats query: %w", err)
		}
		if _, err := tx.Exec(ctx, deleteQuery, deleteArgs...); err != nil {
			return fmt.Errorf("delete seats: %w", err)
		}

		for _, seat := range table.Seats {
			if err := insertSeat(ctx, tx, r.sql, table.ID, seat); err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *Repository) FindTable(ctx context.Context, tableID string) (*domain.Table, error) {
	query, args, err := r.sql.
		Select("id", "name", "seat_count", "status", "small_blind", "big_blind", "button", "active_hand_id").
		From("tables").
		Where(sq.Eq{"id": tableID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build find table query: %w", err)
	}

	var table domain.Table
	var status string
	var activeHandID *string
	if err := r.pool.QueryRow(ctx, query, args...).Scan(
		&table.ID,
		&table.Name,
		&table.SeatCount,
		&status,
		&table.SmallBlind,
		&table.BigBlind,
		&table.Button,
		&activeHandID,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTableNotFound
		}
		return nil, fmt.Errorf("find table: %w", err)
	}
	table.Status = domain.TableStatus(status)
	if activeHandID != nil {
		table.ActiveHandID = *activeHandID
	}

	seats, err := r.loadSeats(ctx, tableID)
	if err != nil {
		return nil, err
	}
	table.Seats = seats

	return &table, nil
}

func (r *Repository) ListTables(ctx context.Context) ([]*domain.Table, error) {
	query, args, err := r.sql.
		Select("id", "name", "seat_count", "status", "small_blind", "big_blind", "button", "active_hand_id").
		From("tables").
		OrderBy("created_at ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list tables query: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	var tables []*domain.Table
	for rows.Next() {
		var table domain.Table
		var status string
		var activeHandID *string
		if err := rows.Scan(&table.ID, &table.Name, &table.SeatCount, &status, &table.SmallBlind, &table.BigBlind, &table.Button, &activeHandID); err != nil {
			return nil, fmt.Errorf("scan table: %w", err)
		}
		table.Status = domain.TableStatus(status)
		if activeHandID != nil {
			table.ActiveHandID = *activeHandID
		}
		seats, err := r.loadSeats(ctx, table.ID)
		if err != nil {
			return nil, err
		}
		table.Seats = seats
		tables = append(tables, &table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tables: %w", err)
	}
	return tables, nil
}

func (r *Repository) loadSeats(ctx context.Context, tableID string) ([]domain.Seat, error) {
	query, args, err := r.sql.
		Select("seat_index", "player_id", "stack").
		From("table_seats").
		Where(sq.Eq{"table_id": tableID}).
		OrderBy("seat_index ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build load seats query: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load seats: %w", err)
	}
	defer rows.Close()

	seats := make([]domain.Seat, 0)
	for rows.Next() {
		var seat domain.Seat
		var playerID *string
		if err := rows.Scan(&seat.Index, &playerID, &seat.Stack); err != nil {
			return nil, fmt.Errorf("scan seat: %w", err)
		}
		if playerID != nil {
			seat.PlayerID = *playerID
		}
		seats = append(seats, seat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate seats: %w", err)
	}
	return seats, nil
}

func insertSeat(ctx context.Context, tx pgx.Tx, sql sq.StatementBuilderType, tableID string, seat domain.Seat) error {
	query, args, err := sql.
		Insert("table_seats").
		Columns("table_id", "seat_index", "player_id", "stack").
		Values(tableID, seat.Index, nullableString(seat.PlayerID), seat.Stack).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert seat query: %w", err)
	}
	if _, err := tx.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert seat: %w", err)
	}
	return nil
}

func nullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func withTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
