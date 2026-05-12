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
ORDER BY (hand_id, sequence_number, kafka_partition, kafka_offset);
