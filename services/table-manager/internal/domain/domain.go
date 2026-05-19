package domain

import "errors"

type TableStatus string

const (
	TableStatusIdle   TableStatus = "idle"
	TableStatusInHand TableStatus = "in_hand"
)

type Table struct {
	ID           string
	Name         string
	SeatCount    int
	Status       TableStatus
	SmallBlind   int64
	BigBlind     int64
	Button       int
	ActiveHandID string
	Seats        []Seat
}

type Seat struct {
	Index    int
	PlayerID string
	Stack    int64
}

func (s Seat) Occupied() bool {
	return s.PlayerID != ""
}

func (t *Table) SeatedPlayers() []Seat {
	result := make([]Seat, 0, len(t.Seats))
	for _, seat := range t.Seats {
		if seat.Occupied() {
			result = append(result, seat)
		}
	}
	return result
}

func (t *Table) PlayablePlayers() []Seat {
	result := make([]Seat, 0, len(t.Seats))
	for _, seat := range t.Seats {
		if seat.Occupied() && seat.Stack > 0 {
			result = append(result, seat)
		}
	}
	return result
}

func (t *Table) SeatByPlayerID(playerID string) *Seat {
	for i := range t.Seats {
		if t.Seats[i].PlayerID == playerID {
			return &t.Seats[i]
		}
	}
	return nil
}

func (t *Table) SeatByIndex(index int) *Seat {
	for i := range t.Seats {
		if t.Seats[i].Index == index {
			return &t.Seats[i]
		}
	}
	return nil
}

func (t *Table) CanAutoStart() bool {
	return t.Status == TableStatusIdle && len(t.PlayablePlayers()) >= 2
}

func (t *Table) NormalizedButtonSeat() int {
	if len(t.PlayablePlayers()) == 0 {
		return 0
	}
	if seat := t.SeatByIndex(t.Button); seat != nil && seat.Occupied() && seat.Stack > 0 {
		return seat.Index
	}
	for _, seat := range t.Seats {
		if seat.Occupied() && seat.Stack > 0 {
			return seat.Index
		}
	}
	return 0
}

func (t *Table) AdvanceButton() {
	if len(t.PlayablePlayers()) == 0 {
		t.Button = 0
		return
	}
	start := t.NormalizedButtonSeat()
	for offset := 1; offset <= len(t.Seats); offset++ {
		idx := (start + offset) % len(t.Seats)
		if seat := t.SeatByIndex(idx); seat != nil && seat.Occupied() && seat.Stack > 0 {
			t.Button = idx
			return
		}
	}
	t.Button = start
}

var (
	ErrTableNotFound         = errors.New("table not found")
	ErrSeatOutOfRange        = errors.New("seat out of range")
	ErrSeatOccupied          = errors.New("seat already occupied")
	ErrPlayerAlreadySeated   = errors.New("player already seated")
	ErrPlayerNotSeated       = errors.New("player not seated")
	ErrActiveHandRequired    = errors.New("active hand required")
	ErrLeaveDuringActiveHand = errors.New("cannot leave during active hand")
	ErrSpectatorCannotAct    = errors.New("spectator cannot act")
	ErrInvalidTableConfig    = errors.New("invalid table config")
	ErrInsufficientFunds     = errors.New("insufficient wallet balance")
)
