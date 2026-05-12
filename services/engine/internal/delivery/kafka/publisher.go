package kafka

import (
	"context"
	"encoding/binary"
	"fmt"
	"slices"
	"time"

	"github.com/linkedin/goavro/v2"
	"github.com/riferrei/srclient"
	kafka "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/pashathecreator/holdem/services/engine/internal/delivery/kafka/avroschema"
	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	"github.com/pashathecreator/holdem/services/engine/internal/metrics"
	"github.com/pashathecreator/holdem/services/engine/internal/telemetry"
)

const (
	topicHandStarted = "hand.started"
	topicPlayerActed = "hand.acted"
	topicHandEnded   = "hand.ended"
)

type Publisher struct {
	writer         *kafka.Writer
	schemaRegistry *srclient.SchemaRegistryClient
	codecs         map[string]*goavro.Codec
	schemaIDs      map[string]int
}

func NewPublisher(brokers []string, schemaRegistryURL string) (*Publisher, error) {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireOne,
	}

	registry := srclient.CreateSchemaRegistryClient(schemaRegistryURL)

	p := &Publisher{
		writer:         writer,
		schemaRegistry: registry,
		codecs:         make(map[string]*goavro.Codec),
		schemaIDs:      make(map[string]int),
	}

	if err := p.registerSchemas(); err != nil {
		return nil, fmt.Errorf("kafka publisher: register schemas: %w", err)
	}

	return p, nil
}

func (p *Publisher) registerSchemas() error {
	schemas := publisherSchemas()

	for topic, schema := range schemas {
		subject := topic + "-value"

		registered, err := p.schemaRegistry.CreateSchema(subject, schema, srclient.Avro)
		if err != nil {
			return fmt.Errorf("register schema for %s: %w", topic, err)
		}

		codec, err := goavro.NewCodec(schema)
		if err != nil {
			return fmt.Errorf("create codec for %s: %w", topic, err)
		}

		p.codecs[topic] = codec
		p.schemaIDs[topic] = registered.ID()
	}

	return nil
}

func (p *Publisher) PublishHandStarted(ctx context.Context, event domain.HandStartedEvent) error {
	ctx, span := telemetry.Tracer().Start(ctx, "kafka.PublishHandStarted")
	defer span.End()

	players := make([]interface{}, len(event.Players))
	for i, id := range event.Players {
		players[i] = string(id)
	}

	payload := map[string]interface{}{
		"event_id":          event.EventID,
		"event_version":     event.EventVersion,
		"hand_id":           string(event.HandID),
		"table_id":          string(event.TableID),
		"sequence_number":   event.SequenceNumber,
		"players":           players,
		"player_count":      event.PlayerCount,
		"button":            event.Button,
		"betting_structure": event.BettingStructure,
		"small_blind":       int64(event.SmallBlind),
		"big_blind":         int64(event.BigBlind),
		"occurred_at":       event.OccurredAt.UnixMilli(),
	}

	return p.publish(ctx, topicHandStarted, string(event.HandID), payload)
}

func (p *Publisher) PublishPlayerActed(ctx context.Context, event domain.PlayerActedEvent) error {
	ctx, span := telemetry.Tracer().Start(ctx, "kafka.PublishPlayerActed")
	defer span.End()

	payload := map[string]interface{}{
		"event_id":           event.EventID,
		"event_version":      event.EventVersion,
		"hand_id":            string(event.HandID),
		"table_id":           string(event.TableID),
		"sequence_number":    event.SequenceNumber,
		"player_id":          string(event.PlayerID),
		"street":             event.Street,
		"player_position":    event.PlayerPosition,
		"action_type":        avroActionType(event.Action.Type),
		"current_bet":        int64(event.CurrentBet),
		"player_current_bet": int64(event.PlayerCurrentBet),
		"amount":             int64(event.Action.Amount),
		"occurred_at":        event.OccurredAt.UnixMilli(),
	}

	return p.publish(ctx, topicPlayerActed, string(event.HandID), payload)
}

func (p *Publisher) PublishHandEnded(ctx context.Context, event domain.HandEndedEvent) error {
	ctx, span := telemetry.Tracer().Start(ctx, "kafka.PublishHandEnded")
	defer span.End()

	winnerIDs := make([]string, 0, len(event.Winners))
	for playerID := range event.Winners {
		winnerIDs = append(winnerIDs, string(playerID))
	}
	slices.Sort(winnerIDs)

	winners := make([]interface{}, 0, len(event.Winners))
	for _, playerID := range winnerIDs {
		winners = append(winners, map[string]interface{}{
			"player_id": playerID,
			"amount":    int64(event.Winners[domain.PlayerID(playerID)]),
		})
	}

	board := make([]interface{}, len(event.Board))
	for i, card := range event.Board {
		board[i] = card.String()
	}

	payload := map[string]interface{}{
		"event_id":        event.EventID,
		"event_version":   event.EventVersion,
		"hand_id":         string(event.HandID),
		"table_id":        string(event.TableID),
		"sequence_number": event.SequenceNumber,
		"player_count":    event.PlayerCount,
		"button":          event.Button,
		"small_blind":     int64(event.SmallBlind),
		"big_blind":       int64(event.BigBlind),
		"showdown":        event.Showdown,
		"gross_pot":       int64(event.GrossPot),
		"net_pot":         int64(event.NetPot),
		"winners":         winners,
		"rake":            int64(event.Rake),
		"board":           board,
		"occurred_at":     event.OccurredAt.UnixMilli(),
	}

	return p.publish(ctx, topicHandEnded, string(event.HandID), payload)
}

func (p *Publisher) publish(ctx context.Context, topic, key string, payload map[string]interface{}) error {
	codec, ok := p.codecs[topic]
	if !ok {
		return fmt.Errorf("kafka publisher: no codec for topic %s", topic)
	}

	schemaID, ok := p.schemaIDs[topic]
	if !ok {
		return fmt.Errorf("kafka publisher: no schema id for topic %s", topic)
	}

	avroBytes, err := codec.BinaryFromNative(nil, payload)
	if err != nil {
		metrics.KafkaPublishTotal.WithLabelValues(topic, "error").Inc()
		return fmt.Errorf("kafka publisher: encode avro: %w", err)
	}

	headers := p.extractTraceHeaders(ctx)
	msg := p.wrapWithSchemaID(schemaID, avroBytes)

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Topic:   topic,
		Key:     []byte(key),
		Value:   msg,
		Headers: headers,
		Time:    time.Now(),
	})
	if err != nil {
		metrics.KafkaPublishTotal.WithLabelValues(topic, "error").Inc()
		return fmt.Errorf("kafka publisher: write message to %s: %w", topic, err)
	}

	metrics.KafkaPublishTotal.WithLabelValues(topic, "ok").Inc()
	return nil
}

func (p *Publisher) extractTraceHeaders(ctx context.Context) []kafka.Header {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	headers := make([]kafka.Header, 0, len(carrier))
	for k, v := range carrier {
		headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
	}
	return headers
}

func (p *Publisher) wrapWithSchemaID(schemaID int, data []byte) []byte {
	msg := make([]byte, 5+len(data))
	msg[0] = 0x00
	binary.BigEndian.PutUint32(msg[1:5], uint32(schemaID))
	copy(msg[5:], data)
	return msg
}

func (p *Publisher) Close() error {
	return p.writer.Close()
}

func publisherSchemas() map[string]string {
	return map[string]string{
		topicHandStarted: avroschema.HandStarted(),
		topicPlayerActed: avroschema.PlayerActed(),
		topicHandEnded:   avroschema.HandEnded(),
	}
}

func avroActionType(actionType domain.ActionType) string {
	switch actionType {
	case domain.ActionFold:
		return "FOLD"
	case domain.ActionCheck:
		return "CHECK"
	case domain.ActionCall:
		return "CALL"
	case domain.ActionRaise:
		return "RAISE"
	case domain.ActionAllIn:
		return "ALL_IN"
	default:
		return "FOLD"
	}
}
