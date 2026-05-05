package application

import (
	"context"
	"fmt"
	"time"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	"github.com/pashathecreator/holdem/services/engine/internal/metrics"
)

type applyActionGameRepository interface {
	FindByID(ctx context.Context, id domain.HandID) (*domain.GameState, error)
	Save(ctx context.Context, state *domain.GameState) error
}

type applyActionEventPublisher interface {
	PublishPlayerActed(ctx context.Context, event domain.PlayerActedEvent) error
}

type applyActionFinishHand interface {
	Execute(ctx context.Context, state *domain.GameState) error
}

type ApplyActionInput struct {
	HandID domain.HandID
	Action domain.Action
}

type ApplyAction struct {
	repo       applyActionGameRepository
	publisher  applyActionEventPublisher
	finishHand applyActionFinishHand
}

func NewApplyAction(
	repo applyActionGameRepository,
	publisher applyActionEventPublisher,
	finishHand applyActionFinishHand,
) *ApplyAction {
	return &ApplyAction{repo: repo, publisher: publisher, finishHand: finishHand}
}

func (uc *ApplyAction) Execute(ctx context.Context, input ApplyActionInput) (*domain.GameState, error) {
	state, err := uc.repo.FindByID(ctx, input.HandID)
	if err != nil {
		return nil, fmt.Errorf("apply action: find game: %w", err)
	}

	if err := domain.ValidateAction(state, input.Action); err != nil {
		return nil, fmt.Errorf("apply action: validate: %w", err)
	}

	domain.ApplyAction(state, input.Action)

	metrics.ActionsTotal.WithLabelValues(input.Action.Type.String()).Inc()

	if err := uc.publisher.PublishPlayerActed(ctx, domain.PlayerActedEvent{
		HandID:     state.ID,
		TableID:    state.TableID,
		PlayerID:   input.Action.PlayerID,
		Action:     input.Action,
		OccurredAt: time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("apply action: publish event: %w", err)
	}

	if len(state.PlayersWhoCanAct()) <= 1 {
		if err := uc.finishHand.Execute(ctx, state); err != nil {
			return nil, fmt.Errorf("apply action: finish hand: %w", err)
		}
		return state, nil
	}

	if domain.IsBettingRoundOver(state) {
		if err := advanceStreet(state); err != nil {
			return nil, err
		}

		if state.Street == domain.StreetShowdown {
			if err := uc.finishHand.Execute(ctx, state); err != nil {
				return nil, fmt.Errorf("apply action: finish hand: %w", err)
			}
			return state, nil
		}
	} else {
		state.ActivePlayer = domain.NextActivePlayer(state)
	}

	if err := uc.repo.Save(ctx, state); err != nil {
		return nil, fmt.Errorf("apply action: save state: %w", err)
	}

	return state, nil
}

func advanceStreet(state *domain.GameState) error {
	domain.CollectBets(state)
	state.CurrentBet = 0
	state.RaisesThisStreet = 0
	state.ActivePlayer = domain.FirstActiveAfterButton(state)

	switch state.Street {
	case domain.StreetPreflop:
		cards, err := state.Deck.Deal(3)
		if err != nil {
			return fmt.Errorf("advance street: deal flop: %w", err)
		}
		state.Board = append(state.Board, cards...)
		state.Street = domain.StreetFlop

	case domain.StreetFlop:
		cards, err := state.Deck.Deal(1)
		if err != nil {
			return fmt.Errorf("advance street: deal turn: %w", err)
		}
		state.Board = append(state.Board, cards...)
		state.Street = domain.StreetTurn

	case domain.StreetTurn:
		cards, err := state.Deck.Deal(1)
		if err != nil {
			return fmt.Errorf("advance street: deal river: %w", err)
		}
		state.Board = append(state.Board, cards...)
		state.Street = domain.StreetRiver

	case domain.StreetRiver:
		state.Street = domain.StreetShowdown
	}

	return nil
}