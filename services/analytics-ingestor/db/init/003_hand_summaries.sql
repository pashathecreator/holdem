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
ORDER BY (hand_id);
