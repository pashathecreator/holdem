package clickhouse

import (
	"context"
	"fmt"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/pashathecreator/holdem/services/analytics-ingestor/internal/config"
	"github.com/pashathecreator/holdem/services/analytics-ingestor/internal/model"
)

type Storage struct {
	conn clickhouse.Conn
}

func New(ctx context.Context, cfg config.ClickHouseConfig) (*Storage, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.Addr},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("open clickhouse connection: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}

	s := &Storage{conn: conn}
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Storage) InsertEvent(ctx context.Context, event model.DecodedEvent) error {
	if err := s.insertRawEvent(ctx, event.RawEvent); err != nil {
		return err
	}
	if event.HandAction != nil {
		if err := s.insertHandAction(ctx, *event.HandAction); err != nil {
			return err
		}
	}
	if event.HandSummary != nil {
		if err := s.insertHandSummary(ctx, *event.HandSummary); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) Close() error {
	return s.conn.Close()
}

func (s *Storage) ensureSchema(ctx context.Context) error {
	for _, ddl := range []string{rawEventsDDL, handActionsDDL, handSummariesDDL} {
		if err := s.conn.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("ensure clickhouse schema: %w", err)
		}
	}
	return nil
}

const rawEventsDDL = `
CREATE TABLE IF NOT EXISTS analytics.raw_events
(
    event_id String,
    event_version Int32,
    event_type String,
    hand_id String,
    table_id String,
    sequence_number Int32,
    occurred_at DateTime64(3, 'UTC'),
    kafka_topic String,
    kafka_partition Int32,
    kafka_offset Int64,
    payload_json String,
    ingested_at DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY (hand_id, sequence_number, kafka_partition, kafka_offset)
`

const handActionsDDL = `
CREATE TABLE IF NOT EXISTS analytics.hand_actions
(
    event_id String,
    hand_id String,
    table_id String,
    sequence_number Int32,
    player_id String,
    street String,
    player_position Int32,
    action_type String,
    current_bet Int64,
    player_current_bet Int64,
    amount Int64,
    occurred_at DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY (hand_id, sequence_number)
`

const handSummariesDDL = `
CREATE TABLE IF NOT EXISTS analytics.hand_summaries
(
    event_id String,
    hand_id String,
    table_id String,
    player_count Int32,
    button Int32,
    small_blind Int64,
    big_blind Int64,
    showdown Bool,
    gross_pot Int64,
    net_pot Int64,
    rake Int64,
    board Array(String),
    winners_json String,
    occurred_at DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY (hand_id)
`

func (s *Storage) insertRawEvent(ctx context.Context, event model.RawEvent) error {
	const query = `
		INSERT INTO analytics.raw_events (
			event_id,
			event_version,
			event_type,
			hand_id,
			table_id,
			sequence_number,
			occurred_at,
			kafka_topic,
			kafka_partition,
			kafka_offset,
			payload_json,
			ingested_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	if err := s.conn.Exec(
		ctx,
		query,
		event.EventID,
		event.EventVersion,
		event.EventType,
		event.HandID,
		event.TableID,
		event.SequenceNumber,
		event.OccurredAt,
		event.KafkaTopic,
		event.KafkaPartition,
		event.KafkaOffset,
		event.PayloadJSON,
		event.IngestedAt,
	); err != nil {
		return fmt.Errorf("insert raw_events row: %w", err)
	}

	return nil
}

func (s *Storage) insertHandAction(ctx context.Context, action model.HandAction) error {
	const query = `
		INSERT INTO analytics.hand_actions (
			event_id,
			hand_id,
			table_id,
			sequence_number,
			player_id,
			street,
			player_position,
			action_type,
			current_bet,
			player_current_bet,
			amount,
			occurred_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	if err := s.conn.Exec(
		ctx,
		query,
		action.EventID,
		action.HandID,
		action.TableID,
		action.SequenceNumber,
		action.PlayerID,
		action.Street,
		action.PlayerPosition,
		action.ActionType,
		action.CurrentBet,
		action.PlayerCurrentBet,
		action.Amount,
		action.OccurredAt,
	); err != nil {
		return fmt.Errorf("insert hand_actions row: %w", err)
	}

	return nil
}

func (s *Storage) insertHandSummary(ctx context.Context, summary model.HandSummary) error {
	const query = `
		INSERT INTO analytics.hand_summaries (
			event_id,
			hand_id,
			table_id,
			player_count,
			button,
			small_blind,
			big_blind,
			showdown,
			gross_pot,
			net_pot,
			rake,
			board,
			winners_json,
			occurred_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	if err := s.conn.Exec(
		ctx,
		query,
		summary.EventID,
		summary.HandID,
		summary.TableID,
		summary.PlayerCount,
		summary.Button,
		summary.SmallBlind,
		summary.BigBlind,
		summary.Showdown,
		summary.GrossPot,
		summary.NetPot,
		summary.Rake,
		summary.Board,
		summary.WinnersJSON,
		summary.OccurredAt,
	); err != nil {
		return fmt.Errorf("insert hand_summaries row: %w", err)
	}

	return nil
}
