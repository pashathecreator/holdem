package avro

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/linkedin/goavro/v2"
	"github.com/riferrei/srclient"

	"github.com/pashathecreator/holdem/services/analytics-ingestor/internal/model"
)

const confluentMagicByte = 0x00

type MessageMeta struct {
	Topic     string
	Partition int
	Offset    int64
}

type Decoder struct {
	lookup func(schemaID int) (string, error)

	mu     sync.RWMutex
	codecs map[int]*goavro.Codec
}

func NewDecoder(schemaRegistryURL string) *Decoder {
	registry := srclient.CreateSchemaRegistryClient(schemaRegistryURL)

	return &Decoder{
		lookup: func(schemaID int) (string, error) {
			schema, err := registry.GetSchema(schemaID)
			if err != nil {
				return "", err
			}
			return schema.Schema(), nil
		},
		codecs: make(map[int]*goavro.Codec),
	}
}

func newDecoderWithLookup(lookup func(schemaID int) (string, error)) *Decoder {
	return &Decoder{
		lookup: lookup,
		codecs: make(map[int]*goavro.Codec),
	}
}

func (d *Decoder) Decode(ctx context.Context, value []byte, meta MessageMeta) (model.DecodedEvent, error) {
	schemaID, payload, err := parseConfluentMessage(value)
	if err != nil {
		return model.DecodedEvent{}, err
	}

	codec, err := d.codecForSchema(schemaID)
	if err != nil {
		return model.DecodedEvent{}, err
	}

	native, _, err := codec.NativeFromBinary(payload)
	if err != nil {
		return model.DecodedEvent{}, fmt.Errorf("decode avro payload: %w", err)
	}

	record, ok := native.(map[string]interface{})
	if !ok {
		return model.DecodedEvent{}, fmt.Errorf("unexpected avro payload type %T", native)
	}

	return decodeRecord(ctx, record, meta)
}

func parseConfluentMessage(value []byte) (int, []byte, error) {
	if len(value) < 5 {
		return 0, nil, fmt.Errorf("confluent message too short: %d", len(value))
	}
	if value[0] != confluentMagicByte {
		return 0, nil, fmt.Errorf("unexpected confluent magic byte: %d", value[0])
	}

	schemaID := int(binary.BigEndian.Uint32(value[1:5]))
	return schemaID, value[5:], nil
}

func (d *Decoder) codecForSchema(schemaID int) (*goavro.Codec, error) {
	d.mu.RLock()
	codec, ok := d.codecs[schemaID]
	d.mu.RUnlock()
	if ok {
		return codec, nil
	}

	schema, err := d.lookup(schemaID)
	if err != nil {
		return nil, fmt.Errorf("get schema %d: %w", schemaID, err)
	}

	codec, err = goavro.NewCodec(schema)
	if err != nil {
		return nil, fmt.Errorf("compile schema %d: %w", schemaID, err)
	}

	d.mu.Lock()
	d.codecs[schemaID] = codec
	d.mu.Unlock()

	return codec, nil
}

func decodeRecord(_ context.Context, record map[string]interface{}, meta MessageMeta) (model.DecodedEvent, error) {
	payloadJSON, err := marshalJSON(record)
	if err != nil {
		return model.DecodedEvent{}, fmt.Errorf("marshal payload json: %w", err)
	}

	raw := model.RawEvent{
		EventID:        requireString(record, "event_id"),
		EventVersion:   requireInt(record, "event_version"),
		EventType:      meta.Topic,
		HandID:         requireString(record, "hand_id"),
		TableID:        requireString(record, "table_id"),
		SequenceNumber: requireInt(record, "sequence_number"),
		OccurredAt:     requireTimestamp(record, "occurred_at"),
		KafkaTopic:     meta.Topic,
		KafkaPartition: meta.Partition,
		KafkaOffset:    meta.Offset,
		PayloadJSON:    payloadJSON,
		IngestedAt:     time.Now().UTC(),
	}

	event := model.DecodedEvent{RawEvent: raw}

	switch meta.Topic {
	case model.EventTypeHandStarted:
		return event, nil
	case model.EventTypeHandActed:
		event.HandAction = &model.HandAction{
			EventID:          raw.EventID,
			HandID:           raw.HandID,
			TableID:          raw.TableID,
			SequenceNumber:   raw.SequenceNumber,
			PlayerID:         requireString(record, "player_id"),
			Street:           requireString(record, "street"),
			PlayerPosition:   requireInt(record, "player_position"),
			ActionType:       requireString(record, "action_type"),
			CurrentBet:       requireInt64(record, "current_bet"),
			PlayerCurrentBet: requireInt64(record, "player_current_bet"),
			Amount:           requireInt64(record, "amount"),
			OccurredAt:       raw.OccurredAt,
		}
		return event, nil
	case model.EventTypeHandEnded:
		winnersJSON, err := marshalJSON(record["winners"])
		if err != nil {
			return model.DecodedEvent{}, fmt.Errorf("marshal winners json: %w", err)
		}

		event.HandSummary = &model.HandSummary{
			EventID:     raw.EventID,
			HandID:      raw.HandID,
			TableID:     raw.TableID,
			PlayerCount: requireInt(record, "player_count"),
			Button:      requireInt(record, "button"),
			SmallBlind:  requireInt64(record, "small_blind"),
			BigBlind:    requireInt64(record, "big_blind"),
			Showdown:    requireBool(record, "showdown"),
			GrossPot:    requireInt64(record, "gross_pot"),
			NetPot:      requireInt64(record, "net_pot"),
			Rake:        requireInt64(record, "rake"),
			Board:       requireStringArray(record, "board"),
			WinnersJSON: winnersJSON,
			OccurredAt:  raw.OccurredAt,
		}
		return event, nil
	default:
		return model.DecodedEvent{}, fmt.Errorf("unsupported topic %q", meta.Topic)
	}
}

func marshalJSON(value interface{}) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func requireString(record map[string]interface{}, key string) string {
	value, ok := record[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func requireInt(record map[string]interface{}, key string) int {
	return int(requireInt64(record, key))
}

func requireInt64(record map[string]interface{}, key string) int64 {
	value, ok := record[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case uint32:
		return int64(typed)
	case uint64:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func requireBool(record map[string]interface{}, key string) bool {
	value, ok := record[key]
	if !ok || value == nil {
		return false
	}
	typed, ok := value.(bool)
	if !ok {
		return false
	}
	return typed
}

func requireTimestamp(record map[string]interface{}, key string) time.Time {
	return time.UnixMilli(requireInt64(record, key)).UTC()
}

func requireStringArray(record map[string]interface{}, key string) []string {
	value, ok := record[key]
	if !ok || value == nil {
		return nil
	}

	items, ok := value.([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case string:
			result = append(result, typed)
		case []byte:
			result = append(result, string(typed))
		default:
			result = append(result, fmt.Sprint(typed))
		}
	}
	return result
}
