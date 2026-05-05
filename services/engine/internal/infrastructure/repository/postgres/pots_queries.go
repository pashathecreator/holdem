package postgres

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
)

func replacePots(ctx context.Context, tx pgx.Tx, state *domain.GameState) error {
	if err := deletePots(ctx, tx, state.ID); err != nil {
		return err
	}

	for position, pot := range state.Pots {
		if err := insertPot(ctx, tx, state.ID, position, pot); err != nil {
			return err
		}
	}

	return nil
}

func deletePots(ctx context.Context, tx pgx.Tx, handID domain.HandID) error {
	deleteEligibleQuery, deleteEligibleArgs, err := psql.
		Delete("game_pot_eligible").
		Where(sq.Eq{"hand_id": string(handID)}).
		ToSql()
	if err != nil {
		return fmt.Errorf("game state repo: build delete pot eligible query: %w", err)
	}

	if _, err := tx.Exec(ctx, deleteEligibleQuery, deleteEligibleArgs...); err != nil {
		return fmt.Errorf("game state repo: delete pot eligible: %w", err)
	}

	deletePotsQuery, deletePotsArgs, err := psql.
		Delete("game_pots").
		Where(sq.Eq{"hand_id": string(handID)}).
		ToSql()
	if err != nil {
		return fmt.Errorf("game state repo: build delete pots query: %w", err)
	}

	if _, err := tx.Exec(ctx, deletePotsQuery, deletePotsArgs...); err != nil {
		return fmt.Errorf("game state repo: delete pots: %w", err)
	}

	return nil
}

func insertPot(ctx context.Context, tx pgx.Tx, handID domain.HandID, position int, pot domain.Pot) error {
	query, args, err := psql.
		Insert("game_pots").
		Columns("hand_id", "position", "amount").
		Values(string(handID), position, pot.Amount).
		ToSql()
	if err != nil {
		return fmt.Errorf("game state repo: build insert pot query: %w", err)
	}

	if _, err := tx.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("game state repo: insert pot: %w", err)
	}

	for _, playerID := range pot.Eligible {
		query, args, err := psql.
			Insert("game_pot_eligible").
			Columns("hand_id", "pot_position", "player_id").
			Values(string(handID), position, string(playerID)).
			ToSql()
		if err != nil {
			return fmt.Errorf("game state repo: build insert pot eligible query: %w", err)
		}

		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("game state repo: insert pot eligible: %w", err)
		}
	}

	return nil
}

func loadPots(ctx context.Context, pool *pgxpool.Pool, handID domain.HandID) ([]domain.Pot, error) {
	query, args, err := psql.
		Select("position", "amount").
		From("game_pots").
		Where(sq.Eq{"hand_id": string(handID)}).
		OrderBy("position ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("game state repo: build load pots query: %w", err)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("game state repo: load pots: %w", err)
	}
	defer rows.Close()

	pots := make([]domain.Pot, 0)

	for rows.Next() {
		var (
			position int
			amount   int
		)

		if err := rows.Scan(&position, &amount); err != nil {
			return nil, fmt.Errorf("game state repo: scan pot: %w", err)
		}

		eligible, err := loadPotEligible(ctx, pool, handID, position)
		if err != nil {
			return nil, err
		}

		pots = append(pots, domain.Pot{
			Amount:   amount,
			Eligible: eligible,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("game state repo: iterate pots: %w", err)
	}

	return pots, nil
}

func loadPotEligible(ctx context.Context, pool *pgxpool.Pool, handID domain.HandID, potPosition int) ([]domain.PlayerID, error) {
	query, args, err := psql.
		Select("player_id").
		From("game_pot_eligible").
		Where(sq.Eq{
			"hand_id":      string(handID),
			"pot_position": potPosition,
		}).
		OrderBy("player_id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("game state repo: build load pot eligible query: %w", err)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("game state repo: load pot eligible: %w", err)
	}
	defer rows.Close()

	eligible := make([]domain.PlayerID, 0)

	for rows.Next() {
		var playerID string

		if err := rows.Scan(&playerID); err != nil {
			return nil, fmt.Errorf("game state repo: scan pot eligible: %w", err)
		}

		eligible = append(eligible, domain.PlayerID(playerID))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("game state repo: iterate pot eligible: %w", err)
	}

	return eligible, nil
}