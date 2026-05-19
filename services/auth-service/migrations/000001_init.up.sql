CREATE SCHEMA IF NOT EXISTS auth;

CREATE TABLE auth.users (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE auth.password_credentials (
    user_id       TEXT PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE auth.refresh_sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auth_refresh_sessions_user_id ON auth.refresh_sessions(user_id);
CREATE INDEX idx_auth_refresh_sessions_expires_at ON auth.refresh_sessions(expires_at);
