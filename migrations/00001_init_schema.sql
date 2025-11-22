-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id BYTEA PRIMARY KEY,             -- WebAuthn user ID (raw bytes)
    name TEXT NOT NULL UNIQUE,        -- WebAuthnName()
    display_name TEXT NOT NULL,       -- WebAuthnDisplayName()
    icon TEXT                         -- avatar URL
);

CREATE TABLE credentials (
    id BYTEA PRIMARY KEY,             -- CredentialID (raw bytes)
    user_id BYTEA NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    public_key BYTEA NOT NULL,        -- Credential.PublicKey
    attestation_type TEXT,            -- credential.AttestationType
    aaguid BYTEA,                     -- credential.AAGUID
    sign_count BIGINT NOT NULL,       -- credential.Authenticator.SignCount
    transports TEXT[],                -- credential.Transports (string array)
    flags INTEGER NOT NULL DEFAULT 0, -- authenticator flags bitmask
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE webauthn_sessions (
    session_id TEXT PRIMARY KEY,       -- base64 token
    user_name TEXT NOT NULL,           -- "username" (string key)
    challenge BYTEA NOT NULL,
    user_id BYTEA,
    allowed_credentials BYTEA[],
    expires_at TIMESTAMPTZ NOT NULL,
    rp_id TEXT,
    cred_params JSONB,
    extensions JSONB,
    user_verification TEXT,
    mediation TEXT
);

CREATE INDEX idx_credentials_user_id ON credentials(user_id);
CREATE INDEX idx_sessions_user_name ON webauthn_sessions(user_name);
CREATE INDEX idx_sessions_expires ON webauthn_sessions(expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS webauthn_sessions;
DROP TABLE IF EXISTS credentials;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
