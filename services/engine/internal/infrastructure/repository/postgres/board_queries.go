package postgres

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	"github.com/pashathecreator/holdem/services/engine/internal/infrastructure/repository"
)

func replaceBoard(ctx context.Context, tx pgx.Tx, state *domain.GameState) error {
	deleteQuery, deleteArgs, err := psql.
		Delete("game_board").
		Where(sq.Eq{"hand_id": string(state.ID)}).
		ToSql()
	if err != nil {
		return fmt.Errorf("game state repo: build delete board query: %w", err)
	}

	if _, err := tx.Exec(ctx, deleteQuery, deleteArgs...); err != nil {
		return fmt.Errorf("game state repo: delete board: %w", err)
	}

	for position, card := range state.Board {
		query, args, err := psql.
			Insert("game_board").
			Columns("hand_id", "position", "card").
			Values(string(state.ID), position, cardToString(card)).
			ToSql()
		if err != nil {
			return fmt.Errorf("game state repo: build insert board query: %w", err)
		}

		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("game state repo: insert board: %w", err)
		}
	}

	return nil
}

func loadBoard(ctx context.Context, pool *pgxpool.Pool, handID domain.HandID) ([]domain.Card, error) {
	query, args, err := psql.
		Select("card").
		From("game_board").
		Where(sq.Eq{"hand_id": string(handID)}).
		OrderBy("position ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("game state repo: build load board query: %w", err)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("game state repo: load board: %w", err)
	}
	defer rows.Close()

	board := make([]domain.Card, 0)

	for rows.Next() {
		var cardString string

		if err := rows.Scan(&cardString); err != nil {
			return nil, fmt.Errorf("game state repo: scan board card: %w", err)
		}

		card, err := repository.ParseCard(cardString)
		if err != nil {
			return nil, fmt.Errorf("game state repo: parse board card: %w", err)
		}

		board = append(board, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("game state repo: iterate board: %w", err)
	}

	return board, nil
}