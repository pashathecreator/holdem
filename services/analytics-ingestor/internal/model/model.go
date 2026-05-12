package model

import "time"

const (
	EventTypeHandStarted = "hand.started"
	EventTypeHandActed   = "hand.acted"
	EventTypeHandEnded   = "hand.ended"
)

type RawEvent struct {
	EventID        string
	EventVersion   int
	EventType      string
	HandID         string
	TableID        string
	SequenceNumber int
	OccurredAt     time.Time
	KafkaTopic     string
	KafkaPartition int
	KafkaOffset    int64
	PayloadJSON    string
	IngestedAt     time.Time
}

type HandAction struct {
	EventID          string
	HandID           string
	TableID          string
	SequenceNumber   int
	PlayerID         string
	Street           string
	PlayerPosition   int
	ActionType       string
	CurrentBet       int64
	PlayerCurrentBet int64
	Amount           int64
	OccurredAt       time.Time
}

type HandSummary struct {
	EventID     string
	HandID      string
	TableID     string
	PlayerCount int
	Button      int
	SmallBlind  int64
	BigBlind    int64
	Showdown    bool
	GrossPot    int64
	NetPot      int64
	Rake        int64
	Board       []string
	WinnersJSON string
	OccurredAt  time.Time
}

type DecodedEvent struct {
	RawEvent    RawEvent
	HandAction  *HandAction
	HandSummary *HandSummary
}
