DROP TABLE IF EXISTS wallet.wallet_chain_state;

ALTER TABLE wallet.wallet_withdrawal_requests
    DROP COLUMN IF EXISTS confirmed_at,
    DROP COLUMN IF EXISTS submitted_at,
    DROP COLUMN IF EXISTS nonce;

ALTER TABLE wallet.wallet_deposits
    DROP COLUMN IF EXISTS sweep_failure_reason,
    DROP COLUMN IF EXISTS swept_at,
    DROP COLUMN IF EXISTS sweep_submitted_at,
    DROP COLUMN IF EXISTS sweep_status,
    DROP COLUMN IF EXISTS sweep_tx_hash,
    DROP COLUMN IF EXISTS confirmations,
    DROP COLUMN IF EXISTS raw_amount_wei,
    DROP COLUMN IF EXISTS to_address,
    DROP COLUMN IF EXISTS from_address;

ALTER TABLE wallet.wallet_deposit_addresses
    DROP COLUMN IF EXISTS last_observed_block,
    DROP COLUMN IF EXISTS last_sweep_at,
    DROP COLUMN IF EXISTS last_sweep_tx_hash;
