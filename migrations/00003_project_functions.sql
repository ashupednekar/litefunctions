-- +goose Up

CREATE TYPE project_role AS ENUM ('owner', 'manager', 'viewer');

-------------------------------------------------------------------------------
-- PROJECTS
-------------------------------------------------------------------------------
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_by BYTEA NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_projects_created_by ON projects(created_by);

-------------------------------------------------------------------------------
-- USER ↔ PROJECT ACCESS
-------------------------------------------------------------------------------
CREATE TABLE user_project_access (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id BYTEA NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role project_role NOT NULL DEFAULT 'viewer',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_user_project_access UNIQUE (user_id, project_id)
);

CREATE INDEX idx_user_project_access_user ON user_project_access(user_id);
CREATE INDEX idx_user_project_access_project ON user_project_access(project_id);

-------------------------------------------------------------------------------
-- PROJECT INVITES
-------------------------------------------------------------------------------
CREATE TABLE project_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    invite_code TEXT NOT NULL,
    created_by BYTEA NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_project_invite_code UNIQUE (invite_code),
    CONSTRAINT chk_invite_expiry CHECK (expires_at > created_at)
);

CREATE INDEX idx_project_invites_project ON project_invites(project_id);
CREATE INDEX idx_project_invites_code ON project_invites(invite_code);

-------------------------------------------------------------------------------
-- FUNCTIONS (METADATA ONLY)
-------------------------------------------------------------------------------
CREATE TABLE functions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    language TEXT NOT NULL,                    -- e.g., "go", "js", "rust"
    path TEXT NOT NULL,                        -- repo path to function entrypoint
    created_by BYTEA NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- A function name must be unique *within a project*
    UNIQUE (project_id, name)
);

CREATE INDEX idx_functions_project_id ON functions(project_id);
CREATE INDEX idx_functions_created_by ON functions(created_by);

-------------------------------------------------------------------------------
-- ENDPOINTS → FUNCTION MAPPING
-------------------------------------------------------------------------------
CREATE TABLE endpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    method TEXT NOT NULL,                      -- GET/POST/PUT/DELETE
    scope TEXT NOT NULL CHECK (scope IN ('public', 'authn')),
    function_id UUID NOT NULL REFERENCES functions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- endpoint name + method must be unique within a project
    UNIQUE (project_id, name, method)
);

CREATE INDEX idx_endpoints_project_id ON endpoints(project_id);
CREATE INDEX idx_endpoints_function_id ON endpoints(function_id);

-- +goose Down
DROP TABLE IF EXISTS endpoints;
DROP TABLE IF EXISTS functions;
DROP TABLE IF EXISTS project_invites;
DROP TABLE IF EXISTS user_project_access;
DROP TABLE IF EXISTS projects;
DROP TYPE IF EXISTS project_role;
