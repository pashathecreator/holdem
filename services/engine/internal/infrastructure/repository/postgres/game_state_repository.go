package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
)

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
	defer rollback(ctx, tx)

	if err := upsertGameState(ctx, tx, state); err != nil {
		return err
	}
	if err := replacePlayers(ctx, tx, state); err != nil {
		return err
	}
	if err := replaceBoard(ctx, tx, state); err != nil {
		return err
	}
	if err := replacePots(ctx, tx, state); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("game state repo: commit tx: %w", err)
	}

	return nil
}

func (r *GameStateRepo) FindByID(ctx context.Context, id domain.HandID) (*domain.GameState, error) {
	state, err := findGameState(ctx, r.pool, id)
	if err != nil {
		return nil, err
	}

	state.Players, err = loadPlayers(ctx, r.pool, id)
	if err != nil {
		return nil, err
	}

	state.Board, err = loadBoard(ctx, r.pool, id)
	if err != nil {
		return nil, err
	}

	state.Pots, err = loadPots(ctx, r.pool, id)
	if err != nil {
		return nil, err
	}

	return state, nil
}

func (r *GameStateRepo) Delete(ctx context.Context, id domain.HandID) error {
	query, args, err := psql.
		Delete("game_states").
		Where("id = ?", string(id)).
		ToSql()
	if err != nil {
		return fmt.Errorf("game state repo: build delete query: %w", err)
	}

	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("game state repo: delete: %w", err)
	}

	return nil
}