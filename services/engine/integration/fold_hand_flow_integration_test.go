//go:build integration

package integration_test

import (
	"testing"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"
)

func TestE2E_HandFlow_FoldCompletesHand(t *testing.T) {
	t.Parallel()
	skipIfDockerUnavailable(t)

	h := newIntegrationHarness(t, deterministicShuffle(allCards()))

	startResp, err := h.client.StartHand(h.ctx, &enginev1.StartHandRequest{
		TableId: "table-fold",
		Players: []*enginev1.Player{
			{Id: "p1", Stack: 100, Position: 0},
			{Id: "p2", Stack: 100, Position: 1},
		},
		Button:     0,
		SmallBlind: 1,
		BigBlind:   2,
	})
	if err != nil {
		t.Fatalf("start hand: %v", err)
	}

	handID := startResp.State.Id
	if handID == "" {
		t.Fatal("start hand: empty hand id")
	}

	actionResp, err := h.client.ApplyAction(h.ctx, &enginev1.ApplyActionRequest{
		HandId: handID,
		Action: &enginev1.Action{
			PlayerId: "p1",
			Type:     enginev1.ActionType_ACTION_TYPE_FOLD,
		},
	})
	if err != nil {
		t.Fatalf("apply fold: %v", err)
	}

	if actionResp.State.Street != enginev1.Street_STREET_SHOWDOWN {
		t.Fatalf("expected finished hand at showdown state, got %v", actionResp.State.Street)
	}
	if len(actionResp.State.Board) != 0 {
		t.Fatalf("expected no board cards after preflop fold, got %d", len(actionResp.State.Board))
	}

	stacks := stateStacks(actionResp.State)
	if stacks["p1"] != 99 || stacks["p2"] != 101 {
		t.Fatalf("unexpected final stacks: %+v", stacks)
	}

	stateResp, err := h.client.GetGameState(h.ctx, &enginev1.GetGameStateRequest{HandId: handID})
	if err != nil {
		t.Fatalf("get game state: %v", err)
	}
	if stateResp.State.Street != enginev1.Street_STREET_SHOWDOWN {
		t.Fatalf("expected persisted finished state, got %v", stateResp.State.Street)
	}

	if got := h.mustReadTopicMessages(t, "hand.started", 1); got < 1 {
		t.Fatalf("expected hand.started messages, got %d", got)
	}
	if got := h.mustReadTopicMessages(t, "hand.acted", 1); got < 1 {
		t.Fatalf("expected hand.acted messages, got %d", got)
	}
	if got := h.mustReadTopicMessages(t, "hand.ended", 1); got < 1 {
		t.Fatalf("expected hand.ended messages, got %d", got)
	}

	storedState, err := h.repo.FindByID(h.ctx, domain.HandID(handID))
	if err != nil {
		t.Fatalf("repo find state: %v", err)
	}
	if storedState.EventSequence != 3 {
		t.Fatalf("expected event sequence 3, got %d", storedState.EventSequence)
	}
}
