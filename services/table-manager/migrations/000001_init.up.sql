CREATE TABLE tables (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    seat_count      INT NOT NULL,
    status          TEXT NOT NULL,
    small_blind     BIGINT NOT NULL,
    big_blind       BIGINT NOT NULL,
    button          INT NOT NULL DEFAULT 0,
    active_hand_id  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE table_seats (
    table_id     TEXT NOT NULL REFERENCES tables(id) ON DELETE CASCADE,
    seat_index   INT NOT NULL,
    player_id    TEXT,
    stack        BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (table_id, seat_index)
);

CREATE INDEX idx_tables_status ON tables(status);
