package domain

import "testing"

func TestNormalizedButtonSeatFallsBackToFirstOccupiedSeat(t *testing.T) {
	table := &Table{
		Button: 4,
		Seats: []Seat{
			{Index: 0},
			{Index: 1, PlayerID: "p1", Stack: 1000},
			{Index: 2},
			{Index: 3, PlayerID: "p2", Stack: 1000},
		},
	}

	if got := table.NormalizedButtonSeat(); got != 1 {
		t.Fatalf("NormalizedButtonSeat() = %d, want 1", got)
	}
}

func TestAdvanceButtonSkipsEmptySeats(t *testing.T) {
	table := &Table{
		Button: 1,
		Seats: []Seat{
			{Index: 0, PlayerID: "p1", Stack: 1000},
			{Index: 1, PlayerID: "p2", Stack: 1000},
			{Index: 2},
			{Index: 3, PlayerID: "p3", Stack: 1000},
		},
	}

	table.AdvanceButton()
	if table.Button != 3 {
		t.Fatalf("AdvanceButton() button = %d, want 3", table.Button)
	}

	table.AdvanceButton()
	if table.Button != 0 {
		t.Fatalf("AdvanceButton() second button = %d, want 0", table.Button)
	}
}
