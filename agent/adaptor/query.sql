-- name: UpsertAgentSession :one
INSERT INTO agent_sessions (id, project_id, user_id, title, directory, model, provider)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    directory = EXCLUDED.directory,
    model = EXCLUDED.model,
    provider = EXCLUDED.provider,
    updated_at = now()
RETURNING *;

-- name: GetAgentSession :one
SELECT *
FROM agent_sessions
WHERE id = $1;

-- name: ListAgentSessionsForProject :many
SELECT *
FROM agent_sessions
WHERE project_id = $1
  AND (sqlc.arg(user_id)::bytea IS NULL OR user_id = sqlc.arg(user_id))
ORDER BY updated_at DESC
LIMIT sqlc.arg(row_limit);

-- name: TouchAgentSession :exec
UPDATE agent_sessions
SET updated_at = now()
WHERE id = $1;

-- name: CreateAgentChat :one
INSERT INTO agent_chats (session_id, project_id, prompt)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetAgentChat :one
SELECT *
FROM agent_chats
WHERE id = $1;

-- name: ListAgentChatsForSession :many
SELECT *
FROM agent_chats
WHERE session_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: FinishAgentChat :one
UPDATE agent_chats
SET status = $2,
    reply = $3,
    error = $4,
    tool_calls = $5,
    finished_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateAgentChatSync :one
UPDATE agent_chats
SET functions = $2,
    latest_function_id = $3,
    latest_function_name = $4,
    last_synced_at = now()
WHERE id = $1
RETURNING *;

-- name: UpsertAgentMessage :one
INSERT INTO agent_messages (session_id, id, chat_id, role, text, parts, model, provider, error)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (session_id, id) DO UPDATE SET
    text = EXCLUDED.text,
    parts = EXCLUDED.parts,
    model = EXCLUDED.model,
    provider = EXCLUDED.provider,
    error = EXCLUDED.error
RETURNING *;

-- name: ListAgentMessages :many
SELECT *
FROM agent_messages
WHERE session_id = $1
ORDER BY created_at ASC, id ASC;

-- name: UpsertAgentToolCall :one
INSERT INTO agent_tool_calls (
    session_id, id, message_id, chat_id, tool, status, title, input, output,
    error, started_at, completed_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (session_id, id) DO UPDATE SET
    status = EXCLUDED.status,
    title = EXCLUDED.title,
    input = EXCLUDED.input,
    output = EXCLUDED.output,
    error = EXCLUDED.error,
    completed_at = EXCLUDED.completed_at
RETURNING *;

-- name: ListAgentToolCallsForChat :many
SELECT *
FROM agent_tool_calls
WHERE chat_id = $1
ORDER BY created_at ASC, id ASC;

-- name: ListAgentToolCallsForSession :many
SELECT *
FROM agent_tool_calls
WHERE session_id = $1
ORDER BY created_at ASC, id ASC;
