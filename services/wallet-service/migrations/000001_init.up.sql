CREATE SCHEMA IF NOT EXISTS wallet;

CREATE TABLE IF NOT EXISTS wallet.wallet_accounts (
    user_id TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    provisioned_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS wallet.wallet_balances (
    user_id TEXT PRIMARY KEY REFERENCES wallet.wallet_accounts(user_id) ON DELETE CASCADE,
    available_gwei BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS wallet.wallet_ledger_entries (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES wallet.wallet_accounts(user_id) ON DELETE CASCADE,
    entry_kind TEXT NOT NULL,
    amount_gwei BIGINT NOT NULL,
    reference_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS wallet.wallet_deposit_addresses (
    user_id TEXT PRIMARY KEY REFERENCES wallet.wallet_accounts(user_id) ON DELETE CASCADE,
    address TEXT NOT NULL UNIQUE,
    private_key_hex TEXT NOT NULL,
    chain TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS wallet.wallet_link_challenges (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES wallet.wallet_accounts(user_id) ON DELETE CASCADE,
    address TEXT NOT NULL,
    challenge TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS wallet.wallet_linked_addresses (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES wallet.wallet_accounts(user_id) ON DELETE CASCADE,
    address TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    verified_at TIMESTAMPTZ NOT NULL,
    UNIQUE(user_id, address)
);

CREATE TABLE IF NOT EXISTS wallet.wallet_deposits (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES wallet.wallet_accounts(user_id) ON DELETE CASCADE,
    address TEXT NOT NULL,
    amount_gwei BIGINT NOT NULL,
    status TEXT NOT NULL,
    tx_hash TEXT NOT NULL,
    observed_block BIGINT,
    confirmed_block BIGINT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE(tx_hash)
);

CREATE TABLE IF NOT EXISTS wallet.wallet_withdrawal_requests (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES wallet.wallet_accounts(user_id) ON DELETE CASCADE,
    address TEXT NOT NULL,
    amount_gwei BIGINT NOT NULL,
    status TEXT NOT NULL,
    tx_hash TEXT,
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

