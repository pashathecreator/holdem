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
ORDER BY (hand_id, sequence_number);
