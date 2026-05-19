package grpc

import (
	"testing"

	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"

	"github.com/pashathecreator/holdem/services/table-manager/internal/domain"
	tablemanagerv1 "github.com/pashathecreator/holdem/services/table-manager/pkg/gen/go/table_manager/v1"
)

func TestDomainTableToProtoViewerSeesOnlyOwnHoleCards(t *testing.T) {
	table := &domain.Table{
		ID:           "table-1",
		Name:         "Main",
		SeatCount:    2,
		Status:       domain.TableStatusInHand,
		Button:       1,
		ActiveHandID: "hand-1",
		SmallBlind:   50,
		BigBlind:     100,
		Seats: []domain.Seat{
			{Index: 0, PlayerID: "p1", Stack: 1000},
			{Index: 1, PlayerID: "p2", Stack: 900},
		},
	}
	state := &enginev1.GameState{
		Id:           "hand-1",
		Street:       enginev1.Street_STREET_TURN,
		CurrentBet:   200,
		ActivePlayer: 1,
		Button:       1,
		SmallBlind:   50,
		BigBlind:     100,
		Board: []*enginev1.Card{
			{Value: "A♠"},
			{Value: "K♦"},
			{Value: "Q♣"},
		},
		Pots: []*enginev1.Pot{
			{Amount: 300, Eligible: []string{"p1", "p2"}},
		},
		Players: []*enginev1.Player{
			{
				Id:         "p1",
				Stack:      1000,
				CurrentBet: 100,
				Status:     enginev1.PlayerStatus_PLAYER_STATUS_ACTIVE,
				HoleCards: []*enginev1.Card{
					{Value: "2♣"},
					{Value: "3♣"},
				},
			},
			{
				Id:         "p2",
				Stack:      900,
				CurrentBet: 200,
				Status:     enginev1.PlayerStatus_PLAYER_STATUS_ALL_IN,
				HoleCards: []*enginev1.Card{
					{Value: "A♥"},
					{Value: "A♦"},
				},
			},
		},
	}

	view := domainTableToProto(table, state, "p1")
	if view.Hand == nil || view.Hand.HandId != "hand-1" {
		t.Fatalf("view.Hand = %+v, want hand-1", view.Hand)
	}
	if len(view.Hand.Board) != 3 || len(view.Hand.Pots) != 1 {
		t.Fatalf("hand view = %+v, want board and pots", view.Hand)
	}

	seat0 := view.Seats[0]
	seat1 := view.Seats[1]
	if !seat0.IsViewer {
		t.Fatalf("seat0.IsViewer = false, want true")
	}
	if len(seat0.HoleCards) != 2 {
		t.Fatalf("seat0 hole cards = %d, want 2", len(seat0.HoleCards))
	}
	if seat0.PlayerStatus != tablemanagerv1.PlayerStatus_PLAYER_STATUS_ACTIVE {
		t.Fatalf("seat0.PlayerStatus = %v, want active", seat0.PlayerStatus)
	}
	if len(seat1.HoleCards) != 0 {
		t.Fatalf("seat1 hole cards = %d, want 0", len(seat1.HoleCards))
	}
	if seat1.PlayerStatus != tablemanagerv1.PlayerStatus_PLAYER_STATUS_ALL_IN {
		t.Fatalf("seat1.PlayerStatus = %v, want all-in", seat1.PlayerStatus)
	}
}

func TestDomainTableToProtoObserverSeesNoHoleCards(t *testing.T) {
	table := &domain.Table{
		ID:        "table-1",
		Name:      "Main",
		SeatCount: 2,
		Status:    domain.TableStatusInHand,
		Seats: []domain.Seat{
			{Index: 0, PlayerID: "p1", Stack: 1000},
			{Index: 1, PlayerID: "p2", Stack: 1000},
		},
	}
	state := &enginev1.GameState{
		Id: "hand-1",
		Players: []*enginev1.Player{
			{
				Id:        "p1",
				HoleCards: []*enginev1.Card{{Value: "2♣"}, {Value: "3♣"}},
			},
			{
				Id:        "p2",
				HoleCards: []*enginev1.Card{{Value: "A♥"}, {Value: "A♦"}},
			},
		},
	}

	view := domainTableToProto(table, state, "observer")
	for i, seat := range view.Seats {
		if seat.IsViewer {
			t.Fatalf("seat[%d].IsViewer = true, want false", i)
		}
		if len(seat.HoleCards) != 0 {
			t.Fatalf("seat[%d].HoleCards = %d, want 0", i, len(seat.HoleCards))
		}
	}
}
