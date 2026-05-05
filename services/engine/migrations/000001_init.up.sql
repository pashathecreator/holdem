CREATE TABLE game_states (
    id                      TEXT PRIMARY KEY,
    table_id                TEXT NOT NULL,
    street                  TEXT NOT NULL,
    current_bet             BIGINT NOT NULL DEFAULT 0,
    active_player           INT NOT NULL DEFAULT 0,
    button                  INT NOT NULL DEFAULT 0,

    betting_structure       TEXT NOT NULL DEFAULT 'fixed_limit',
    small_blind             BIGINT NOT NULL,
    big_blind               BIGINT NOT NULL,
    small_bet               BIGINT NOT NULL,
    big_bet                 BIGINT NOT NULL,
    max_raises_per_street   INT NOT NULL DEFAULT 4,
    raises_this_street      INT NOT NULL DEFAULT 0,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE game_players (
    hand_id      TEXT NOT NULL REFERENCES game_states(id) ON DELETE CASCADE,
    player_id    TEXT NOT NULL,
    stack        BIGINT NOT NULL DEFAULT 0,
    status       TEXT NOT NULL,
    current_bet  BIGINT NOT NULL DEFAULT 0,
    position     INT NOT NULL,
    hole_card_1  TEXT NOT NULL,
    hole_card_2  TEXT NOT NULL,
    PRIMARY KEY (hand_id, player_id)
);

CREATE TABLE game_board (
    hand_id  TEXT NOT NULL REFERENCES game_states(id) ON DELETE CASCADE,
    position INT NOT NULL,
    card     TEXT NOT NULL,
    PRIMARY KEY (hand_id, position)
);

CREATE TABLE game_pots (
    hand_id  TEXT NOT NULL REFERENCES game_states(id) ON DELETE CASCADE,
    position INT NOT NULL,
    amount   BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (hand_id, position)
);

CREATE TABLE game_pot_eligible (
    hand_id      TEXT NOT NULL REFERENCES game_states(id) ON DELETE CASCADE,
    pot_position INT NOT NULL,
    player_id    TEXT NOT NULL,
    PRIMARY KEY (hand_id, pot_position, player_id)
);

CREATE INDEX idx_game_states_table_id ON game_states(table_id);
CREATE INDEX idx_game_players_hand_id ON game_players(hand_id);
CREATE INDEX idx_game_pots_hand_id ON game_pots(hand_id);