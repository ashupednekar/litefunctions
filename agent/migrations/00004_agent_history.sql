-- +goose Up
-- +goose StatementBegin

-------------------------------------------------------------------------------
-- AGENT SESSIONS (one conversation with the agent)
-------------------------------------------------------------------------------
CREATE TABLE agent_sessions (
    id TEXT PRIMARY KEY,                       -- OpenCode session ID
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id BYTEA REFERENCES users(id) ON DELETE SET NULL,
    title TEXT NOT NULL DEFAULT '',
    directory TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_agent_sessions_project ON agent_sessions(project_id, updated_at DESC);

-------------------------------------------------------------------------------
-- AGENT CHATS (one prompt + reply turn inside a session)
-------------------------------------------------------------------------------
CREATE TABLE agent_chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'completed', 'failed', 'canceled')),
    prompt TEXT NOT NULL,
    reply TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    tool_calls INTEGER NOT NULL DEFAULT 0,
    functions JSONB NOT NULL DEFAULT '[]'::jsonb,
    latest_function_id UUID,
    latest_function_name TEXT NOT NULL DEFAULT '',
    last_synced_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX idx_agent_chats_session ON agent_chats(session_id, started_at DESC);
CREATE INDEX idx_agent_chats_project ON agent_chats(project_id, started_at DESC);

-------------------------------------------------------------------------------
-- AGENT MESSAGES (raw transcript for a session)
-------------------------------------------------------------------------------
CREATE TABLE agent_messages (
    session_id TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    id TEXT NOT NULL,
    chat_id UUID REFERENCES agent_chats(id) ON DELETE SET NULL,
    role TEXT NOT NULL,
    text TEXT NOT NULL DEFAULT '',
    parts JSONB NOT NULL DEFAULT '[]'::jsonb,
    model TEXT,
    provider TEXT,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, id)
);

CREATE INDEX idx_agent_messages_session ON agent_messages(session_id, created_at);

-------------------------------------------------------------------------------
-- AGENT TOOL CALLS (MCP/tool activity, grouped by chat)
-------------------------------------------------------------------------------
CREATE TABLE agent_tool_calls (
    session_id TEXT NOT NULL,
    id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    chat_id UUID REFERENCES agent_chats(id) ON DELETE SET NULL,
    tool TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    input JSONB,
    output TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, id),
    FOREIGN KEY (session_id, message_id)
        REFERENCES agent_messages(session_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_agent_tool_calls_chat ON agent_tool_calls(chat_id, created_at);
CREATE INDEX idx_agent_tool_calls_session ON agent_tool_calls(session_id, created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS agent_tool_calls;
DROP TABLE IF EXISTS agent_messages;
DROP TABLE IF EXISTS agent_chats;
DROP TABLE IF EXISTS agent_sessions;
-- +goose StatementEnd
