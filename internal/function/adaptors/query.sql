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
ORDER BY language ASC, created_at DESC;

-- name: UpdateFunctionPath :one
UPDATE functions
SET path = $2
WHERE id = $1
RETURNING *;



-- name: DeleteFunction :exec
DELETE FROM functions
WHERE id = $1;
