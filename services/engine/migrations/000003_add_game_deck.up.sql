CREATE TABLE game_deck (
    hand_id  TEXT NOT NULL REFERENCES game_states(id) ON DELETE CASCADE,
    position INT NOT NULL,
    card     TEXT NOT NULL,
    PRIMARY KEY (hand_id, position)
);

CREATE INDEX idx_game_deck_hand_id ON game_deck(hand_id);
