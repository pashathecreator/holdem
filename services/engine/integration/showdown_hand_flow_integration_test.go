//go:build integration

package integration_test

import (
	"strings"
	"testing"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"
)

func TestE2E_HandFlow_ShowdownCompletesHand(t *testing.T) {
	t.Parallel()
	skipIfDockerUnavailable(t)

	shuffledDeck := orderedDeck([]domain.Card{
		domain.NewCard(domain.Ace, domain.Spades),
		domain.NewCard(domain.Ace, domain.Hearts),
		domain.NewCard(domain.King, domain.Diamonds),
		domain.NewCard(domain.Queen, domain.Diamonds),
		domain.NewCard(domain.Ace, domain.Clubs),
		domain.NewCard(domain.Two, domain.Clubs),
		domain.NewCard(domain.Three, domain.Hearts),
		domain.NewCard(domain.Seven, domain.Spades),
		domain.NewCard(domain.Nine, domain.Diamonds),
	})
	h := newIntegrationHarness(t, deterministicShuffle(shuffledDeck))

	startResp, err := h.client.StartHand(h.ctx, &enginev1.StartHandRequest{
		TableId: "table-showdown",
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

	actions := []*enginev1.Action{
		{PlayerId: "p1", Type: enginev1.ActionType_ACTION_TYPE_CALL},
		{PlayerId: "p2", Type: enginev1.ActionType_ACTION_TYPE_CHECK},
		{PlayerId: "p2", Type: enginev1.ActionType_ACTION_TYPE_CHECK},
		{PlayerId: "p1", Type: enginev1.ActionType_ACTION_TYPE_CHECK},
		{PlayerId: "p2", Type: enginev1.ActionType_ACTION_TYPE_CHECK},
		{PlayerId: "p1", Type: enginev1.ActionType_ACTION_TYPE_CHECK},
		{PlayerId: "p2", Type: enginev1.ActionType_ACTION_TYPE_CHECK},
		{PlayerId: "p1", Type: enginev1.ActionType_ACTION_TYPE_CHECK},
	}

	var lastState *enginev1.GameState
	for i, action := range actions {
		resp, err := h.client.ApplyAction(h.ctx, &enginev1.ApplyActionRequest{
			HandId: handID,
			Action: action,
		})
		if err != nil {
			t.Fatalf("apply action %d: %v", i, err)
		}
		lastState = resp.State
	}

	if lastState == nil {
		t.Fatal("expected final state after showdown flow")
	}
	if lastState.Street != enginev1.Street_STREET_SHOWDOWN {
		t.Fatalf("expected showdown state, got %v", lastState.Street)
	}
	if len(lastState.Board) != 5 {
		t.Fatalf("expected 5 board cards, got %d", len(lastState.Board))
	}

	expectedBoard := []string{"A♣", "2♣", "3♥", "7♠", "9♦"}
	gotBoard := make([]string, len(lastState.Board))
	for i, card := range lastState.Board {
		gotBoard[i] = card.Value
	}
	if strings.Join(gotBoard, ",") != strings.Join(expectedBoard, ",") {
		t.Fatalf("unexpected board: got %v want %v", gotBoard, expectedBoard)
	}

	stacks := stateStacks(lastState)
	if stacks["p1"] != 102 || stacks["p2"] != 98 {
		t.Fatalf("unexpected final stacks: %+v", stacks)
	}

	stateResp, err := h.client.GetGameState(h.ctx, &enginev1.GetGameStateRequest{HandId: handID})
	if err != nil {
		t.Fatalf("get game state: %v", err)
	}
	if len(stateResp.State.Board) != 5 {
		t.Fatalf("expected persisted board of 5 cards, got %d", len(stateResp.State.Board))
	}

	storedState, err := h.repo.FindByID(h.ctx, domain.HandID(handID))
	if err != nil {
		t.Fatalf("repo find state: %v", err)
	}
	if storedState.EventSequence != 10 {
		t.Fatalf("expected event sequence 10, got %d", storedState.EventSequence)
	}
	if storedState.Deck == nil {
		t.Fatal("expected persisted deck to be restored")
	}
	if remaining := storedState.Deck.Remaining(); remaining != 43 {
		t.Fatalf("expected 43 remaining deck cards, got %d", remaining)
	}

	if got := h.mustReadTopicMessages(t, "hand.started", 1); got < 1 {
		t.Fatalf("expected hand.started messages, got %d", got)
	}
	if got := h.mustReadTopicMessages(t, "hand.acted", len(actions)); got < len(actions) {
		t.Fatalf("expected at least %d hand.acted messages, got %d", len(actions), got)
	}
	if got := h.mustReadTopicMessages(t, "hand.ended", 1); got < 1 {
		t.Fatalf("expected hand.ended messages, got %d", got)
	}
}
