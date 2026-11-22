-- +goose Up
-- +goose StatementBegin

-------------------------------------------------------------------------------
-- AGENT SESSIONS (one conversation with the agent)
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS agent_sessions (
    id TEXT PRIMARY KEY,                       -- OpenCode session ID
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
                                                -- LiteFunctions project owning the session
    user_id BYTEA REFERENCES users(id) ON DELETE SET NULL,
                                                -- portal user who started the session
    title TEXT NOT NULL DEFAULT '',             -- display title (usually "<project> · <user>")
    directory TEXT NOT NULL DEFAULT '',         -- workspace directory passed to OpenCode
    model TEXT NOT NULL DEFAULT '',             -- model used for the session
    provider TEXT NOT NULL DEFAULT '',          -- model provider used for the session
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_sessions_project ON agent_sessions(project_id, updated_at DESC);
                                                -- latest sessions for a project

-------------------------------------------------------------------------------
-- AGENT CHATS (one prompt + reply turn inside a session)
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS agent_chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- one prompt/response turn
    session_id TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
                                                -- parent conversation
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
                                                -- project snapshot for access checks
    status TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'completed', 'failed', 'canceled')),
                                                -- current run state
    prompt TEXT NOT NULL,                       -- user prompt as submitted
    reply TEXT NOT NULL DEFAULT '',             -- final assistant text
    error TEXT NOT NULL DEFAULT '',             -- failure reason when status is failed
    tool_calls INTEGER NOT NULL DEFAULT 0,      -- number of tool calls observed
    functions JSONB NOT NULL DEFAULT '[]'::jsonb,
                                                -- function list snapshot after the last sync
    latest_function_id UUID,                    -- newest function created/updated
    latest_function_name TEXT NOT NULL DEFAULT '',
    last_synced_at TIMESTAMPTZ,                 -- last project sync for this turn
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ                     -- set when the turn completes or fails
);

CREATE INDEX IF NOT EXISTS idx_agent_chats_session ON agent_chats(session_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_chats_project ON agent_chats(project_id, started_at DESC);

-------------------------------------------------------------------------------
-- AGENT MESSAGES (raw transcript for a session)
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS agent_messages (
    session_id TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
                                                -- parent conversation
    id TEXT NOT NULL,                           -- OpenCode message ID
    chat_id UUID REFERENCES agent_chats(id) ON DELETE SET NULL,
                                                -- turn that produced this message
    role TEXT NOT NULL,                         -- user / assistant
    text TEXT NOT NULL DEFAULT '',              -- concatenated text parts
    parts JSONB NOT NULL DEFAULT '[]'::jsonb,   -- full OpenCode parts payload
    model TEXT,                                 -- model that produced the message
    provider TEXT,                              -- provider that produced the message
    error TEXT,                                 -- message-level error, if any
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, id)                -- IDs are unique per session
);

CREATE INDEX IF NOT EXISTS idx_agent_messages_session ON agent_messages(session_id, created_at);

-------------------------------------------------------------------------------
-- AGENT TOOL CALLS (MCP/tool activity, grouped by chat)
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS agent_tool_calls (
    session_id TEXT NOT NULL,                   -- parent conversation
    id TEXT NOT NULL,                           -- OpenCode tool part ID
    message_id TEXT NOT NULL,                   -- assistant message containing the call
    chat_id UUID REFERENCES agent_chats(id) ON DELETE SET NULL,
                                                -- turn that produced the call
    tool TEXT NOT NULL,                         -- MCP tool name, e.g. litefunctions_create_function
    status TEXT NOT NULL DEFAULT '',            -- running / completed / error
    title TEXT NOT NULL DEFAULT '',             -- short display title from OpenCode
    input JSONB,                                -- tool arguments
    output TEXT NOT NULL DEFAULT '',            -- tool result text
    error TEXT NOT NULL DEFAULT '',             -- tool failure text
    started_at TIMESTAMPTZ,                     -- OpenCode start timestamp
    completed_at TIMESTAMPTZ,                   -- OpenCode completion timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, id),
    FOREIGN KEY (session_id, message_id)
        REFERENCES agent_messages(session_id, id) ON DELETE CASCADE
                                                -- tool calls disappear with their message
);

CREATE INDEX IF NOT EXISTS idx_agent_tool_calls_chat ON agent_tool_calls(chat_id, created_at);
CREATE INDEX IF NOT EXISTS idx_agent_tool_calls_session ON agent_tool_calls(session_id, created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS agent_tool_calls;
DROP TABLE IF EXISTS agent_messages;
DROP TABLE IF EXISTS agent_chats;
DROP TABLE IF EXISTS agent_sessions;
-- +goose StatementEnd
