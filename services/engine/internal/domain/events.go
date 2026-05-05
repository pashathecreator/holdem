package domain

import "time"

type HandStartedEvent struct {
	HandID     HandID
	TableID    TableID
	Players    []PlayerID
	Button     int
	SmallBlind int
	BigBlind   int
	OccurredAt time.Time
}

type PlayerActedEvent struct {
	HandID     HandID
	TableID    TableID
	PlayerID   PlayerID
	Action     Action
	OccurredAt time.Time
}

type HandEndedEvent struct {
	HandID     HandID
	TableID    TableID
	Winners    map[PlayerID]int
	Rake       int
	Board      []Card
	OccurredAt time.Time
}