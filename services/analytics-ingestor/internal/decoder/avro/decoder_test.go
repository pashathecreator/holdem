package avro

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/linkedin/goavro/v2"

	"github.com/pashathecreator/holdem/services/analytics-ingestor/internal/model"
)

const (
	handStartedSchema = `{
	  "type": "record",
	  "name": "HandStarted",
	  "namespace": "holdem.engine.v1",
	  "fields": [
	    {"name": "event_id", "type": "string"},
	    {"name": "event_version", "type": "int"},
	    {"name": "hand_id", "type": "string"},
	    {"name": "table_id", "type": "string"},
	    {"name": "sequence_number", "type": "int"},
	    {"name": "players", "type": {"type": "array", "items": "string"}},
	    {"name": "player_count", "type": "int"},
	    {"name": "button", "type": "int"},
	    {"name": "betting_structure", "type": "string"},
	    {"name": "small_blind", "type": "long"},
	    {"name": "big_blind", "type": "long"},
	    {"name": "occurred_at", "type": "long", "logicalType": "timestamp-millis"}
	  ]
	}`
	playerActedSchema = `{
	  "type": "record",
	  "name": "PlayerActed",
	  "namespace": "holdem.engine.v1",
	  "fields": [
	    {"name": "event_id", "type": "string"},
	    {"name": "event_version", "type": "int"},
	    {"name": "hand_id", "type": "string"},
	    {"name": "table_id", "type": "string"},
	    {"name": "sequence_number", "type": "int"},
	    {"name": "player_id", "type": "string"},
	    {"name": "street", "type": "string"},
	    {"name": "player_position", "type": "int"},
	    {"name": "action_type", "type": {"type": "enum", "name": "ActionType", "symbols": ["FOLD", "CHECK", "CALL", "RAISE", "ALL_IN"]}},
	    {"name": "current_bet", "type": "long"},
	    {"name": "player_current_bet", "type": "long"},
	    {"name": "amount", "type": "long"},
	    {"name": "occurred_at", "type": "long", "logicalType": "timestamp-millis"}
	  ]
	}`
	handEndedSchema = `{
	  "type": "record",
	  "name": "HandEnded",
	  "namespace": "holdem.engine.v1",
	  "fields": [
	    {"name": "event_id", "type": "string"},
	    {"name": "event_version", "type": "int"},
	    {"name": "hand_id", "type": "string"},
	    {"name": "table_id", "type": "string"},
	    {"name": "sequence_number", "type": "int"},
	    {"name": "player_count", "type": "int"},
	    {"name": "button", "type": "int"},
	    {"name": "small_blind", "type": "long"},
	    {"name": "big_blind", "type": "long"},
	    {"name": "showdown", "type": "boolean"},
	    {"name": "gross_pot", "type": "long"},
	    {"name": "net_pot", "type": "long"},
	    {"name": "winners", "type": {"type": "array", "items": {"type": "record", "name": "Winner", "fields": [{"name": "player_id", "type": "string"}, {"name": "amount", "type": "long"}]}}},
	    {"name": "rake", "type": "long"},
	    {"name": "board", "type": {"type": "array", "items": "string"}},
	    {"name": "occurred_at", "type": "long", "logicalType": "timestamp-millis"}
	  ]
	}`
)

func TestParseConfluentMessage(t *testing.T) {
	value := wrapConfluentMessage(42, []byte{1, 2, 3})

	schemaID, payload, err := parseConfluentMessage(value)
	if err != nil {
		t.Fatalf("parseConfluentMessage() error = %v", err)
	}

	if schemaID != 42 {
		t.Fatalf("schemaID = %d, want 42", schemaID)
	}
	if got := fmt.Sprintf("%v", payload); got != "[1 2 3]" {
		t.Fatalf("payload = %s, want [1 2 3]", got)
	}
}

func TestDecoderDecodeStartedActedEnded(t *testing.T) {
	decoder := newDecoderWithLookup(func(schemaID int) (string, error) {
		schema, ok := map[int]string{
			1: handStartedSchema,
			2: playerActedSchema,
			3: handEndedSchema,
		}[schemaID]
		if !ok {
			return "", fmt.Errorf("unknown schema id %d", schemaID)
		}
		return schema, nil
	})
	occurredAt := time.UnixMilli(1715512345678).UTC()

	started := mustEncodeAvro(t, handStartedSchema, map[string]interface{}{
		"event_id":          "hand-1:1",
		"event_version":     int32(1),
		"hand_id":           "hand-1",
		"table_id":          "table-1",
		"sequence_number":   int32(1),
		"players":           []interface{}{"p1", "p2"},
		"player_count":      int32(2),
		"button":            int32(0),
		"betting_structure": "no_limit",
		"small_blind":       int64(50),
		"big_blind":         int64(100),
		"occurred_at":       occurredAt.UnixMilli(),
	})
	startedEvent, err := decoder.Decode(context.Background(), wrapConfluentMessage(1, started), MessageMeta{
		Topic:     model.EventTypeHandStarted,
		Partition: 0,
		Offset:    11,
	})
	if err != nil {
		t.Fatalf("Decode(started) error = %v", err)
	}
	if startedEvent.RawEvent.EventType != model.EventTypeHandStarted {
		t.Fatalf("started event_type = %q", startedEvent.RawEvent.EventType)
	}
	if startedEvent.RawEvent.HandID != "hand-1" {
		t.Fatalf("started hand_id = %q", startedEvent.RawEvent.HandID)
	}
	if startedEvent.HandAction != nil || startedEvent.HandSummary != nil {
		t.Fatalf("started event produced unexpected derived rows")
	}

	acted := mustEncodeAvro(t, playerActedSchema, map[string]interface{}{
		"event_id":           "hand-1:2",
		"event_version":      int32(1),
		"hand_id":            "hand-1",
		"table_id":           "table-1",
		"sequence_number":    int32(2),
		"player_id":          "p1",
		"street":             "preflop",
		"player_position":    int32(0),
		"action_type":        "CALL",
		"current_bet":        int64(100),
		"player_current_bet": int64(100),
		"amount":             int64(100),
		"occurred_at":        occurredAt.UnixMilli(),
	})
	actedEvent, err := decoder.Decode(context.Background(), wrapConfluentMessage(2, acted), MessageMeta{
		Topic:     model.EventTypeHandActed,
		Partition: 1,
		Offset:    12,
	})
	if err != nil {
		t.Fatalf("Decode(acted) error = %v", err)
	}
	if actedEvent.HandAction == nil {
		t.Fatalf("acted event did not produce hand_action")
	}
	if actedEvent.HandAction.ActionType != "CALL" {
		t.Fatalf("action_type = %q, want CALL", actedEvent.HandAction.ActionType)
	}
	if actedEvent.RawEvent.KafkaPartition != 1 || actedEvent.RawEvent.KafkaOffset != 12 {
		t.Fatalf("raw kafka metadata = (%d, %d)", actedEvent.RawEvent.KafkaPartition, actedEvent.RawEvent.KafkaOffset)
	}

	ended := mustEncodeAvro(t, handEndedSchema, map[string]interface{}{
		"event_id":        "hand-1:3",
		"event_version":   int32(1),
		"hand_id":         "hand-1",
		"table_id":        "table-1",
		"sequence_number": int32(3),
		"player_count":    int32(2),
		"button":          int32(0),
		"small_blind":     int64(50),
		"big_blind":       int64(100),
		"showdown":        true,
		"gross_pot":       int64(200),
		"net_pot":         int64(190),
		"winners": []interface{}{
			map[string]interface{}{"player_id": "p1", "amount": int64(190)},
		},
		"rake":        int64(10),
		"board":       []interface{}{"As", "Kd", "Qc", "Jh", "Ts"},
		"occurred_at": occurredAt.UnixMilli(),
	})
	endedEvent, err := decoder.Decode(context.Background(), wrapConfluentMessage(3, ended), MessageMeta{
		Topic:     model.EventTypeHandEnded,
		Partition: 2,
		Offset:    13,
	})
	if err != nil {
		t.Fatalf("Decode(ended) error = %v", err)
	}
	if endedEvent.HandSummary == nil {
		t.Fatalf("ended event did not produce hand_summary")
	}
	if !endedEvent.HandSummary.Showdown {
		t.Fatalf("summary showdown = false, want true")
	}
	if len(endedEvent.HandSummary.Board) != 5 {
		t.Fatalf("board len = %d, want 5", len(endedEvent.HandSummary.Board))
	}
	if !strings.Contains(endedEvent.HandSummary.WinnersJSON, `"player_id":"p1"`) {
		t.Fatalf("winners json = %s", endedEvent.HandSummary.WinnersJSON)
	}
}

func mustEncodeAvro(t *testing.T, schema string, payload map[string]interface{}) []byte {
	t.Helper()

	codec, err := goavro.NewCodec(schema)
	if err != nil {
		t.Fatalf("NewCodec() error = %v", err)
	}

	binaryValue, err := codec.BinaryFromNative(nil, payload)
	if err != nil {
		t.Fatalf("BinaryFromNative() error = %v", err)
	}

	return binaryValue
}

func wrapConfluentMessage(schemaID int, payload []byte) []byte {
	msg := make([]byte, 5+len(payload))
	msg[0] = confluentMagicByte
	binary.BigEndian.PutUint32(msg[1:5], uint32(schemaID))
	copy(msg[5:], payload)
	return msg
}
