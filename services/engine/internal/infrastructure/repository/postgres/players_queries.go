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

func replacePlayers(ctx context.Context, tx pgx.Tx, state *domain.GameState) error {
	deleteQuery, deleteArgs, err := psql.
		Delete("game_players").
		Where(sq.Eq{"hand_id": string(state.ID)}).
		ToSql()
	if err != nil {
		return fmt.Errorf("game state repo: build delete players query: %w", err)
	}

	if _, err := tx.Exec(ctx, deleteQuery, deleteArgs...); err != nil {
		return fmt.Errorf("game state repo: delete players: %w", err)
	}

	for _, player := range state.Players {
		query, args, err := psql.
			Insert("game_players").
			Columns(
				"hand_id",
				"player_id",
				"stack",
				"status",
				"current_bet",
				"position",
				"hole_card_1",
				"hole_card_2",
			).
			Values(
				string(state.ID),
				string(player.ID),
				player.Stack,
				playerStatusToString(player.Status),
				player.CurrentBet,
				player.Position,
				cardToString(player.HoleCards[0]),
				cardToString(player.HoleCards[1]),
			).
			ToSql()
		if err != nil {
			return fmt.Errorf("game state repo: build insert player query: %w", err)
		}

		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("game state repo: insert player: %w", err)
		}
	}

	return nil
}

func loadPlayers(ctx context.Context, pool *pgxpool.Pool, handID domain.HandID) ([]*domain.Player, error) {
	query, args, err := psql.
		Select(
			"player_id",
			"stack",
			"status",
			"current_bet",
			"position",
			"hole_card_1",
			"hole_card_2",
		).
		From("game_players").
		Where(sq.Eq{"hand_id": string(handID)}).
		OrderBy("position ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("game state repo: build load players query: %w", err)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("game state repo: load players: %w", err)
	}
	defer rows.Close()

	players := make([]*domain.Player, 0)

	for rows.Next() {
		var (
			playerID   string
			stack      int
			status     string
			currentBet int
			position   int
			card1      string
			card2      string
		)

		if err := rows.Scan(
			&playerID,
			&stack,
			&status,
			&currentBet,
			&position,
			&card1,
			&card2,
		); err != nil {
			return nil, fmt.Errorf("game state repo: scan player: %w", err)
		}

		holeCard1, err := repository.ParseCard(card1)
		if err != nil {
			return nil, fmt.Errorf("game state repo: parse hole card 1: %w", err)
		}

		holeCard2, err := repository.ParseCard(card2)
		if err != nil {
			return nil, fmt.Errorf("game state repo: parse hole card 2: %w", err)
		}

		players = append(players, &domain.Player{
			ID:         domain.PlayerID(playerID),
			Stack:      stack,
			HoleCards:  [2]domain.Card{holeCard1, holeCard2},
			Status:     playerStatusFromString(status),
			CurrentBet: currentBet,
			Position:   position,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("game state repo: iterate players: %w", err)
	}

	return players, nil
}