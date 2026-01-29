-- name: CreateUser :exec
INSERT INTO users (id, name, display_name, icon)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO NOTHING;

-- name: GetUserByName :one
SELECT * FROM users WHERE name = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpdateUser :exec
UPDATE users
SET display_name = $2,
    icon = $3
WHERE id = $1;

-- name: CreateCredential :exec
INSERT INTO credentials (
    id,
    user_id,
    public_key,
    attestation_type,
    aaguid,
    sign_count,
    transports,
    flags
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO NOTHING;

-- name: GetCredentialsForUser :many
SELECT *
FROM credentials
WHERE user_id = $1
ORDER BY created_at ASC;

-- name: GetCredentialByID :one
SELECT *
FROM credentials
WHERE id = $1;

-- name: UpdateCredential :exec
UPDATE credentials
SET public_key = $2,
    attestation_type = $3,
    aaguid = $4,
    sign_count = $5,
    transports = $6,
    flags = $7,
    updated_at = now()
WHERE id = $1;

-- name: SaveSession :exec
INSERT INTO webauthn_sessions (
    session_id,
    user_name,
    challenge,
    user_id,
    allowed_credentials,
    expires_at,
    rp_id,
    cred_params,
    extensions,
    user_verification,
    mediation
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (session_id) DO UPDATE SET
    user_name = EXCLUDED.user_name,
    challenge = EXCLUDED.challenge,
    user_id = EXCLUDED.user_id,
    allowed_credentials = EXCLUDED.allowed_credentials,
    expires_at = EXCLUDED.expires_at,
    rp_id = EXCLUDED.rp_id,
    cred_params = EXCLUDED.cred_params,
    extensions = EXCLUDED.extensions,
    user_verification = EXCLUDED.user_verification,
    mediation = EXCLUDED.mediation;

-- name: GetSession :one
SELECT *
FROM webauthn_sessions
WHERE session_id = $1;

-- name: DeleteSession :exec
DELETE FROM webauthn_sessions
WHERE session_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM webauthn_sessions
WHERE expires_at < now();

-- User Sessions (for persistent authentication)
-- name: CreateUserSession :exec
INSERT INTO user_sessions (session_id, user_id, expires_at, user_agent, ip_address)
VALUES ($1, $2, $3, $4, $5);

-- name: GetUserSession :one
SELECT * FROM user_sessions WHERE session_id = $1 AND expires_at > now();

-- name: DeleteUserSession :exec
DELETE FROM user_sessions WHERE session_id = $1;

-- name: DeleteExpiredUserSessions :exec
DELETE FROM user_sessions WHERE expires_at < now();

-- name: DeleteUserSessionsByUserID :exec
DELETE FROM user_sessions WHERE user_id = $1;
