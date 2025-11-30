-- FUNCTION + ENDPOINT QUERIES

-------------------------------------------------------------------------------
-- FUNCTIONS
-------------------------------------------------------------------------------

-- name: CreateFunction :one
INSERT INTO functions (project_id, name, language, path, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetFunctionByID :one
SELECT *
FROM functions
WHERE id = $1;

-- name: ListFunctionsForProject :many
SELECT *
FROM functions
WHERE project_id = $1
ORDER BY created_at DESC;

-- name: UpdateFunctionPath :one
UPDATE functions
SET path = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteFunction :exec
DELETE FROM functions
WHERE id = $1;

-------------------------------------------------------------------------------
-- ENDPOINTS
-------------------------------------------------------------------------------

-- name: CreateEndpoint :one
INSERT INTO endpoints (project_id, name, method, scope, function_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetEndpointByID :one
SELECT *
FROM endpoints
WHERE id = $1;

-- name: ListEndpointsForProject :many
SELECT *
FROM endpoints
WHERE project_id = $1
ORDER BY created_at DESC;

-- name: ListEndpointsForFunction :many
SELECT *
FROM endpoints
WHERE function_id = $1;

