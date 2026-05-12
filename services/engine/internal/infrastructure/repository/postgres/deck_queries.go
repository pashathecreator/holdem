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

func replaceDeck(ctx context.Context, tx pgx.Tx, state *domain.GameState) error {
	deleteQuery, deleteArgs, err := psql.
		Delete("game_deck").
		Where(sq.Eq{"hand_id": string(state.ID)}).
		ToSql()
	if err != nil {
		return fmt.Errorf("game state repo: build delete deck query: %w", err)
	}

	if _, err := tx.Exec(ctx, deleteQuery, deleteArgs...); err != nil {
		return fmt.Errorf("game state repo: delete deck: %w", err)
	}

	if state.Deck == nil {
		return nil
	}

	for position, card := range state.Deck.Cards() {
		query, args, err := psql.
			Insert("game_deck").
			Columns("hand_id", "position", "card").
			Values(string(state.ID), position, cardToString(card)).
			ToSql()
		if err != nil {
			return fmt.Errorf("game state repo: build insert deck query: %w", err)
		}

		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("game state repo: insert deck: %w", err)
		}
	}

	return nil
}

func loadDeck(ctx context.Context, pool *pgxpool.Pool, handID domain.HandID) (*domain.Deck, error) {
	query, args, err := psql.
		Select("card").
		From("game_deck").
		Where(sq.Eq{"hand_id": string(handID)}).
		OrderBy("position ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("game state repo: build load deck query: %w", err)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("game state repo: load deck: %w", err)
	}
	defer rows.Close()

	cards := make([]domain.Card, 0)

	for rows.Next() {
		var cardString string

		if err := rows.Scan(&cardString); err != nil {
			return nil, fmt.Errorf("game state repo: scan deck card: %w", err)
		}

		card, err := repository.ParseCard(cardString)
		if err != nil {
			return nil, fmt.Errorf("game state repo: parse deck card: %w", err)
		}

		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("game state repo: iterate deck: %w", err)
	}

	return domain.NewDeckFromCards(cards), nil
}
