CREATE TABLE IF NOT EXISTS wallet.wallet_faucet_transfers (
    id TEXT PRIMARY KEY,
    admin_user_id TEXT NOT NULL,
    target_user_id TEXT NOT NULL REFERENCES wallet.wallet_accounts(user_id) ON DELETE CASCADE,
    target_address TEXT NOT NULL,
    amount_gwei BIGINT NOT NULL,
    status TEXT NOT NULL,
    tx_hash TEXT,
    nonce BIGINT,
    submitted_at TIMESTAMPTZ,
    confirmed_at TIMESTAMPTZ,
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_wallet_faucet_transfers_status
    ON wallet.wallet_faucet_transfers(status, updated_at);
