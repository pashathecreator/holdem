ALTER TABLE wallet.wallet_deposit_addresses
    ADD COLUMN IF NOT EXISTS last_sweep_tx_hash TEXT,
    ADD COLUMN IF NOT EXISTS last_sweep_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_observed_block BIGINT;

ALTER TABLE wallet.wallet_deposits
    ADD COLUMN IF NOT EXISTS from_address TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS to_address TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS raw_amount_wei TEXT NOT NULL DEFAULT '0',
    ADD COLUMN IF NOT EXISTS confirmations BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS sweep_tx_hash TEXT,
    ADD COLUMN IF NOT EXISTS sweep_status TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sweep_submitted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS swept_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS sweep_failure_reason TEXT;

ALTER TABLE wallet.wallet_withdrawal_requests
    ADD COLUMN IF NOT EXISTS nonce BIGINT,
    ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS confirmed_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS wallet.wallet_chain_state (
    name TEXT PRIMARY KEY,
    block_number BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

