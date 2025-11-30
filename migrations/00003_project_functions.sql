-- +goose Up
-- +goose StatementBegin

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
-- USER ↔ PROJECT (MANY-TO-MANY)
-------------------------------------------------------------------------------
CREATE TABLE user_projects (
    user_id BYTEA NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role TEXT,                                 -- optional: owner/member/viewer etc.
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, project_id)
);

CREATE INDEX idx_user_projects_project_id ON user_projects(project_id);

-------------------------------------------------------------------------------
-- FUNCTIONS (METADATA ONLY)
-------------------------------------------------------------------------------
CREATE TABLE functions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    language TEXT NOT NULL,                    -- e.g., "python", "js", "rust"
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

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS endpoints;
DROP TABLE IF EXISTS functions;
DROP TABLE IF EXISTS user_projects;
DROP TABLE IF EXISTS projects;

-- +goose StatementEnd

