package application

import (
	"context"
	"fmt"
	"time"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	"github.com/pashathecreator/holdem/services/engine/internal/metrics"
)

type finishHandGameRepository interface {
	Save(ctx context.Context, state *domain.GameState) error
}

type finishHandEventPublisher interface {
	PublishHandEnded(ctx context.Context, event domain.HandEndedEvent) error
}

type handEvaluator interface {
	Best(playerID domain.PlayerID, hole [2]domain.Card, board []domain.Card) domain.HandResult
	Compare(a, b domain.HandResult) int
}

type FinishHand struct {
	repo       finishHandGameRepository
	publisher  finishHandEventPublisher
	evaluator  handEvaluator
	rakeConfig domain.RakeConfig
}

func NewFinishHand(
	repo finishHandGameRepository,
	publisher finishHandEventPublisher,
	evaluator handEvaluator,
	rakeConfig domain.RakeConfig,
) *FinishHand {
	return &FinishHand{
		repo:       repo,
		publisher:  publisher,
		evaluator:  evaluator,
		rakeConfig: rakeConfig,
	}
}

func (uc *FinishHand) Execute(ctx context.Context, state *domain.GameState) error {
	domain.CollectBets(state)

	rake := domain.CalculateRake(state.Pots, state.Street, uc.rakeConfig)
	state.Pots = domain.ApplyRake(state.Pots, rake)

	winners := uc.determineWinners(state)

	if state.Street == domain.StreetShowdown {
		metrics.ShowdownsTotal.Inc()
	}

	state.Street = domain.StreetShowdown

	metrics.HandsEnded.Inc()
	metrics.ActiveGames.Dec()
	metrics.PotSize.Observe(float64(domain.Total(state.Pots)))
	metrics.RakeCollected.Add(float64(rake))

	if err := uc.repo.Save(ctx, state); err != nil {
		return fmt.Errorf("finish hand: save state: %w", err)
	}

	if err := uc.publisher.PublishHandEnded(ctx, domain.HandEndedEvent{
		HandID:     state.ID,
		TableID:    state.TableID,
		Winners:    winners,
		Rake:       rake,
		Board:      state.Board,
		OccurredAt: time.Now(),
	}); err != nil {
		return fmt.Errorf("finish hand: publish event: %w", err)
	}

	return nil
}

func (uc *FinishHand) determineWinners(state *domain.GameState) map[domain.PlayerID]int {
	activePlayers := state.ActivePlayers()

	if len(activePlayers) == 1 {
		return singleWinner(state, activePlayers[0].ID)
	}

	return uc.showdown(state, activePlayers)
}

func singleWinner(state *domain.GameState, playerID domain.PlayerID) map[domain.PlayerID]int {
	total := domain.Total(state.Pots)
	result := map[domain.PlayerID]int{playerID: total}
	award(state, result)
	return result
}

func (uc *FinishHand) showdown(state *domain.GameState, activePlayers []*domain.Player) map[domain.PlayerID]int {
	results := make(map[domain.PlayerID]domain.HandResult, len(activePlayers))
	for _, p := range activePlayers {
		results[p.ID] = uc.evaluator.Best(p.ID, p.HoleCards, state.Board)
	}

	winnersByPot := make([][]domain.PlayerID, len(state.Pots))
	for i, pot := range state.Pots {
		winnersByPot[i] = bestHandsInPot(pot.Eligible, results, uc.evaluator)
	}

	winners := winnersToMap(state.Pots, winnersByPot)
	award(state, winners)
	return winners
}

func bestHandsInPot(
	eligible []domain.PlayerID,
	results map[domain.PlayerID]domain.HandResult,
	evaluator handEvaluator,
) []domain.PlayerID {
	var best []domain.PlayerID

	for _, id := range eligible {
		result, ok := results[id]
		if !ok {
			continue
		}

		if len(best) == 0 {
			best = []domain.PlayerID{id}
			continue
		}

		cmp := evaluator.Compare(result, results[best[0]])
		switch {
		case cmp > 0:
			best = []domain.PlayerID{id}
		case cmp == 0:
			best = append(best, id)
		}
	}

	return best
}

func award(state *domain.GameState, winners map[domain.PlayerID]int) {
	for _, p := range state.Players {
		if amount, ok := winners[p.ID]; ok {
			p.Stack += amount
		}
	}
}

func winnersToMap(pots []domain.Pot, winnersByPot [][]domain.PlayerID) map[domain.PlayerID]int {
	result := make(map[domain.PlayerID]int)

	for i, pot := range pots {
		if i >= len(winnersByPot) {
			break
		}
		winners := winnersByPot[i]
		if len(winners) == 0 {
			continue
		}

		share := pot.Amount / len(winners)
		remainder := pot.Amount % len(winners)

		for j, id := range winners {
			result[id] += share
			if j < remainder {
				result[id]++
			}
		}
	}

	return result
}