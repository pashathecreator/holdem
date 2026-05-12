package postgres

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

func upsertGameState(ctx context.Context, tx pgx.Tx, state *domain.GameState) error {
	query, args, err := psql.
		Insert("game_states").
		Columns(
			"id",
			"table_id",
			"street",
			"current_bet",
			"active_player",
			"button",
			"event_sequence",
			"betting_structure",
			"small_blind",
			"big_blind",
			"small_bet",
			"big_bet",
			"max_raises_per_street",
			"raises_this_street",
			"updated_at",
		).
		Values(
			string(state.ID),
			string(state.TableID),
			streetToString(state.Street),
			state.CurrentBet,
			state.ActivePlayer,
			state.Button,
			state.EventSequence,
			bettingStructureToString(state.Structure),
			state.SmallBlind,
			state.BigBlind,
			state.SmallBet,
			state.BigBet,
			state.MaxRaisesPerStreet,
			state.RaisesThisStreet,
			sq.Expr("NOW()"),
		).
		Suffix(`ON CONFLICT (id) DO UPDATE SET
			street = EXCLUDED.street,
			current_bet = EXCLUDED.current_bet,
			active_player = EXCLUDED.active_player,
			button = EXCLUDED.button,
			event_sequence = EXCLUDED.event_sequence,
			betting_structure = EXCLUDED.betting_structure,
			small_blind = EXCLUDED.small_blind,
			big_blind = EXCLUDED.big_blind,
			small_bet = EXCLUDED.small_bet,
			big_bet = EXCLUDED.big_bet,
			max_raises_per_street = EXCLUDED.max_raises_per_street,
			raises_this_street = EXCLUDED.raises_this_street,
			updated_at = NOW()`).
		ToSql()
	if err != nil {
		return fmt.Errorf("game state repo: build upsert state query: %w", err)
	}

	if _, err := tx.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("game state repo: upsert state: %w", err)
	}

	return nil
}

func findGameState(ctx context.Context, pool *pgxpool.Pool, id domain.HandID) (*domain.GameState, error) {
	query, args, err := psql.
		Select(
			"id",
			"table_id",
			"street",
			"current_bet",
			"active_player",
			"button",
			"event_sequence",
			"betting_structure",
			"small_blind",
			"big_blind",
			"small_bet",
			"big_bet",
			"max_raises_per_street",
			"raises_this_street",
		).
		From("game_states").
		Where(sq.Eq{"id": string(id)}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("game state repo: build find state query: %w", err)
	}

	var (
		handID             string
		tableID            string
		street             string
		currentBet         int
		activePlayer       int
		button             int
		eventSequence      int
		bettingStructure   string
		smallBlind         int
		bigBlind           int
		smallBet           int
		bigBet             int
		maxRaisesPerStreet int
		raisesThisStreet   int
	)

	row := pool.QueryRow(ctx, query, args...)
	if err := row.Scan(
		&handID,
		&tableID,
		&street,
		&currentBet,
		&activePlayer,
		&button,
		&eventSequence,
		&bettingStructure,
		&smallBlind,
		&bigBlind,
		&smallBet,
		&bigBet,
		&maxRaisesPerStreet,
		&raisesThisStreet,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrGameNotFound
		}
		return nil, fmt.Errorf("game state repo: find state: %w", err)
	}

	return &domain.GameState{
		ID:            domain.HandID(handID),
		TableID:       domain.TableID(tableID),
		Street:        streetFromString(street),
		CurrentBet:    currentBet,
		ActivePlayer:  activePlayer,
		Button:        button,
		EventSequence: eventSequence,
		BettingConfig: domain.BettingConfig{
			Structure:          bettingStructureFromString(bettingStructure),
			SmallBlind:         smallBlind,
			BigBlind:           bigBlind,
			SmallBet:           smallBet,
			BigBet:             bigBet,
			MaxRaisesPerStreet: maxRaisesPerStreet,
		},
		RaisesThisStreet: raisesThisStreet,
		Players:          make([]*domain.Player, 0),
		Board:            make([]domain.Card, 0),
		Pots:             make([]domain.Pot, 0),
	}, nil
}
