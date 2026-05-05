package postgres

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	"github.com/pashathecreator/holdem/services/engine/internal/telemetry"
)

type TracedGameStateRepo struct {
	repo *GameStateRepo
}

func NewTracedGameStateRepo(repo *GameStateRepo) *TracedGameStateRepo {
	return &TracedGameStateRepo{repo: repo}
}

func (r *TracedGameStateRepo) Save(ctx context.Context, state *domain.GameState) error {
	ctx, span := telemetry.Tracer().Start(ctx, "postgres.GameStateRepo.Save")
	defer span.End()

	span.SetAttributes(
		attribute.String("hand_id", string(state.ID)),
		attribute.String("table_id", string(state.TableID)),
		attribute.String("street", streetToString(state.Street)),
	)

	if err := r.repo.Save(ctx, state); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (r *TracedGameStateRepo) FindByID(ctx context.Context, id domain.HandID) (*domain.GameState, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "postgres.GameStateRepo.FindByID")
	defer span.End()

	span.SetAttributes(attribute.String("hand_id", string(id)))

	state, err := r.repo.FindByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return state, nil
}

func (r *TracedGameStateRepo) Delete(ctx context.Context, id domain.HandID) error {
	ctx, span := telemetry.Tracer().Start(ctx, "postgres.GameStateRepo.Delete")
	defer span.End()

	span.SetAttributes(attribute.String("hand_id", string(id)))

	if err := r.repo.Delete(ctx, id); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}