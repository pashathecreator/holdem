package domain

import "time"

type HandStartedEvent struct {
	EventID          string
	EventVersion     int
	HandID           HandID
	TableID          TableID
	SequenceNumber   int
	Players          []PlayerID
	PlayerCount      int
	Button           int
	BettingStructure string
	SmallBlind       int
	BigBlind         int
	OccurredAt       time.Time
}

type PlayerActedEvent struct {
	EventID          string
	EventVersion     int
	HandID           HandID
	TableID          TableID
	SequenceNumber   int
	PlayerID         PlayerID
	Street           string
	PlayerPosition   int
	Action           Action
	CurrentBet       int
	PlayerCurrentBet int
	OccurredAt       time.Time
}

type HandEndedEvent struct {
	EventID        string
	EventVersion   int
	HandID         HandID
	TableID        TableID
	SequenceNumber int
	PlayerCount    int
	Button         int
	SmallBlind     int
	BigBlind       int
	Showdown       bool
	GrossPot       int
	NetPot         int
	Winners        map[PlayerID]int
	Rake           int
	Board          []Card
	OccurredAt     time.Time
}

func (s Street) EventValue() string {
	switch s {
	case StreetPreflop:
		return "preflop"
	case StreetFlop:
		return "flop"
	case StreetTurn:
		return "turn"
	case StreetRiver:
		return "river"
	case StreetShowdown:
		return "showdown"
	default:
		return "unknown"
	}
}

func (s BettingStructure) EventValue() string {
	switch s {
	case BettingFixedLimit:
		return "fixed_limit"
	default:
		return "unknown"
	}
}
