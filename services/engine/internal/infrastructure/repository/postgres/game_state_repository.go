package postgres

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	"github.com/pashathecreator/holdem/services/engine/internal/infrastructure/repository"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type GameStateRepo struct {
	pool *pgxpool.Pool
}

func NewGameStateRepo(pool *pgxpool.Pool) *GameStateRepo {
	return &GameStateRepo{pool: pool}
}

func (r *GameStateRepo) Save(ctx context.Context, state *domain.GameState) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("game state repo: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := upsertGameState(ctx, tx, state); err != nil {
		return err
	}
	if err := upsertPlayers(ctx, tx, state); err != nil {
		return err
	}
	if err := upsertBoard(ctx, tx, state); err != nil {
		return err
	}
	if err := upsertPots(ctx, tx, state); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *GameStateRepo) FindByID(ctx context.Context, id domain.HandID) (*domain.GameState, error) {
	state, err := findGameState(ctx, r.pool, id)
	if err != nil {
		return nil, err
	}

	players, err := findPlayers(ctx, r.pool, id)
	if err != nil {
		return nil, err
	}
	state.Players = players

	board, err := findBoard(ctx, r.pool, id)
	if err != nil {
		return nil, err
	}
	state.Board = board

	pots, err := findPots(ctx, r.pool, id)
	if err != nil {
		return nil, err
	}
	state.Pots = pots

	return state, nil
}

func (r *GameStateRepo) Delete(ctx context.Context, id domain.HandID) error {
	query, args, err := psql.
		Delete("game_states").
		Where(sq.Eq{"id": string(id)}).
		ToSql()
	if err != nil {
		return fmt.Errorf("game state repo: build delete query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("game state repo: delete: %w", err)
	}
	return nil
}

func upsertGameState(ctx context.Context, tx pgx.Tx, state *domain.GameState) error {
	query, args, err := psql.
		Insert("game_states").
		Columns("id", "table_id", "street", "current_bet", "active_player", "button", "small_blind", "big_blind", "updated_at").
		Values(
			string(state.ID),
			string(state.TableID),
			streetToString(state.Street),
			state.CurrentBet,
			state.ActivePlayer,
			state.Button,
			state.SmallBlind,
			state.BigBlind,
			sq.Expr("NOW()"),
		).
		Suffix(`ON CONFLICT (id) DO UPDATE SET
			street        = EXCLUDED.street,
			current_bet   = EXCLUDED.current_bet,
			active_player = EXCLUDED.active_player,
			button        = EXCLUDED.button,
			updated_at    = NOW()`).
		ToSql()
	if err != nil {
		return fmt.Errorf("game state repo: build upsert state query: %w", err)
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("game state repo: upsert state: %w", err)
	}
	return nil
}

func upsertPlayers(ctx context.Context, tx pgx.Tx, state *domain.GameState) error {
	for _, p := range state.Players {
		query, args, err := psql.
			Insert("game_players").
			Columns("hand_id", "player_id", "stack", "status", "current_bet", "position", "hole_card_1", "hole_card_2").
			Values(
				string(state.ID),
				string(p.ID),
				p.Stack,
				statusToString(p.Status),
				p.CurrentBet,
				p.Position,
				p.HoleCards[0].String(),
				p.HoleCards[1].String(),
			).
			Suffix(`ON CONFLICT (hand_id, player_id) DO UPDATE SET
				stack       = EXCLUDED.stack,
				status      = EXCLUDED.status,
				current_bet = EXCLUDED.current_bet`).
			ToSql()
		if err != nil {
			return fmt.Errorf("game state repo: build upsert player query: %w", err)
		}

		_, err = tx.Exec(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("game state repo: upsert player %s: %w", p.ID, err)
		}
	}
	return nil
}

func upsertBoard(ctx context.Context, tx pgx.Tx, state *domain.GameState) error {
	for i, card := range state.Board {
		query, args, err := psql.
			Insert("game_board").
			Columns("hand_id", "position", "card").
			Values(string(state.ID), i, card.String()).
			Suffix("ON CONFLICT (hand_id, position) DO NOTHING").
			ToSql()
		if err != nil {
			return fmt.Errorf("game state repo: build upsert board query: %w", err)
		}

		_, err = tx.Exec(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("game state repo: upsert board card %d: %w", i, err)
		}
	}
	return nil
}

func upsertPots(ctx context.Context, tx pgx.Tx, state *domain.GameState) error {
	deleteQuery, deleteArgs, err := psql.
		Delete("game_pots").
		Where(sq.Eq{"hand_id": string(state.ID)}).
		ToSql()
	if err != nil {
		return fmt.Errorf("game state repo: build delete pots query: %w", err)
	}

	_, err = tx.Exec(ctx, deleteQuery, deleteArgs...)
	if err != nil {
		return fmt.Errorf("game state repo: delete pots: %w", err)
	}

	for i, pot := range state.Pots {
		potQuery, potArgs, err := psql.
			Insert("game_pots").
			Columns("hand_id", "position", "amount").
			Values(string(state.ID), i, pot.Amount).
			ToSql()
		if err != nil {
			return fmt.Errorf("game state repo: build insert pot query: %w", err)
		}

		_, err = tx.Exec(ctx, potQuery, potArgs...)
		if err != nil {
			return fmt.Errorf("game state repo: insert pot %d: %w", i, err)
		}

		for _, playerID := range pot.Eligible {
			eligibleQuery, eligibleArgs, err := psql.
				Insert("game_pot_eligible").
				Columns("hand_id", "pot_position", "player_id").
				Values(string(state.ID), i, string(playerID)).
				ToSql()
			if err != nil {
				return fmt.Errorf("game state repo: build insert pot eligible query: %w", err)
			}

			_, err = tx.Exec(ctx, eligibleQuery, eligibleArgs...)
			if err != nil {
				return fmt.Errorf("game state repo: insert pot eligible: %w", err)
			}
		}
	}
	return nil
}

func findGameState(ctx context.Context, pool *pgxpool.Pool, id domain.HandID) (*domain.GameState, error) {
	query, args, err := psql.
		Select("id", "table_id", "street", "current_bet", "active_player", "button", "small_blind", "big_blind").
		From("game_states").
		Where(sq.Eq{"id": string(id)}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("game state repo: build find state query: %w", err)
	}

	var (
		handID, tableID, street string
		currentBet, sb, bb      int
		activePlayer, button    int
	)

	row := pool.QueryRow(ctx, query, args...)
	if err := row.Scan(&handID, &tableID, &street, &currentBet, &activePlayer, &button, &sb, &bb); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrGameNotFound
		}
		return nil, fmt.Errorf("game state repo: find state: %w", err)
	}

	return &domain.GameState{
		ID:           domain.HandID(handID),
		TableID:      domain.TableID(tableID),
		Street:       streetFromString(street),
		CurrentBet:   currentBet,
		ActivePlayer: activePlayer,
		Button:       button,
		SmallBlind:   sb,
		BigBlind:     bb,
		Board:        make([]domain.Card, 0),
		Pots:         make([]domain.Pot, 0),
	}, nil
}

func findPlayers(ctx context.Context, pool *pgxpool.Pool, id domain.HandID) ([]*domain.Player, error) {
	query, args, err := psql.
		Select("player_id", "stack", "status", "current_bet", "position", "hole_card_1", "hole_card_2").
		From("game_players").
		Where(sq.Eq{"hand_id": string(id)}).
		OrderBy("position").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("game state repo: build find players query: %w", err)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("game state repo: find players: %w", err)
	}
	defer rows.Close()

	var players []*domain.Player
	for rows.Next() {
		var (
			playerID, status, card1, card2 string
			stack, currentBet              int
			position                       int
		)
		if err := rows.Scan(&playerID, &stack, &status, &currentBet, &position, &card1, &card2); err != nil {
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
			Status:     statusFromString(status),
			CurrentBet: currentBet,
			Position:   position,
			HoleCards:  [2]domain.Card{holeCard1, holeCard2},
		})
	}

	return players, rows.Err()
}

func findBoard(ctx context.Context, pool *pgxpool.Pool, id domain.HandID) ([]domain.Card, error) {
	query, args, err := psql.
		Select("card").
		From("game_board").
		Where(sq.Eq{"hand_id": string(id)}).
		OrderBy("position").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("game state repo: build find board query: %w", err)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("game state repo: find board: %w", err)
	}
	defer rows.Close()

	var board []domain.Card
	for rows.Next() {
		var cardStr string
		if err := rows.Scan(&cardStr); err != nil {
			return nil, fmt.Errorf("game state repo: scan board card: %w", err)
		}
		card, err := repository.ParseCard(cardStr)
		if err != nil {
			return nil, fmt.Errorf("game state repo: parse board card: %w", err)
		}
		board = append(board, card)
	}

	return board, rows.Err()
}

func findPots(ctx context.Context, pool *pgxpool.Pool, id domain.HandID) ([]domain.Pot, error) {
	query, args, err := psql.
		Select("p.position", "p.amount", "e.player_id").
		From("game_pots p").
		LeftJoin("game_pot_eligible e ON e.hand_id = p.hand_id AND e.pot_position = p.position").
		Where(sq.Eq{"p.hand_id": string(id)}).
		OrderBy("p.position").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("game state repo: build find pots query: %w", err)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("game state repo: find pots: %w", err)
	}
	defer rows.Close()

	potMap := make(map[int]*domain.Pot)
	potOrder := make([]int, 0)

	for rows.Next() {
		var (
			position int
			amount   int
			playerID *string
		)
		if err := rows.Scan(&position, &amount, &playerID); err != nil {
			return nil, fmt.Errorf("game state repo: scan pot: %w", err)
		}

		if _, exists := potMap[position]; !exists {
			potMap[position] = &domain.Pot{Amount: amount, Eligible: make([]domain.PlayerID, 0)}
			potOrder = append(potOrder, position)
		}

		if playerID != nil {
			potMap[position].Eligible = append(potMap[position].Eligible, domain.PlayerID(*playerID))
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("game state repo: iterate pots: %w", err)
	}

	pots := make([]domain.Pot, len(potOrder))
	for i, pos := range potOrder {
		pots[i] = *potMap[pos]
	}

	return pots, nil
}

func streetToString(s domain.Street) string {
	switch s {
	case domain.StreetPreflop:
		return "preflop"
	case domain.StreetFlop:
		return "flop"
	case domain.StreetTurn:
		return "turn"
	case domain.StreetRiver:
		return "river"
	case domain.StreetShowdown:
		return "showdown"
	default:
		return "preflop"
	}
}

func streetFromString(s string) domain.Street {
	switch s {
	case "preflop":
		return domain.StreetPreflop
	case "flop":
		return domain.StreetFlop
	case "turn":
		return domain.StreetTurn
	case "river":
		return domain.StreetRiver
	case "showdown":
		return domain.StreetShowdown
	default:
		return domain.StreetPreflop
	}
}

func statusToString(s domain.PlayerStatus) string {
	switch s {
	case domain.PlayerStatusActive:
		return "active"
	case domain.PlayerStatusFolded:
		return "folded"
	case domain.PlayerStatusAllIn:
		return "all_in"
	case domain.PlayerStatusSittingOut:
		return "sitting_out"
	default:
		return "active"
	}
}

func statusFromString(s string) domain.PlayerStatus {
	switch s {
	case "active":
		return domain.PlayerStatusActive
	case "folded":
		return domain.PlayerStatusFolded
	case "all_in":
		return domain.PlayerStatusAllIn
	case "sitting_out":
		return domain.PlayerStatusSittingOut
	default:
		return domain.PlayerStatusActive
	}
}