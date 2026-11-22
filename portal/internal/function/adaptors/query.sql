-- name: CreateFunction :one
INSERT INTO functions (project_id, name, language, path, is_async, is_ui, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
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

-- name: UpdateFunctionIsAsync :one
UPDATE functions
SET is_async = $2
WHERE id = $1
RETURNING *;

-- name: UpdateFunctionIsUI :one
UPDATE functions
SET is_ui = $2
WHERE id = $1
RETURNING *;


-- name: DeleteFunction :exec
DELETE FROM functions
WHERE id = $1;

-- name: LatestFunctionForProject :one
SELECT *
FROM functions
WHERE project_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: ListFunctionsSearchPaged :many
SELECT *
FROM functions
WHERE project_id = $1
  AND (
        $2::text = '' OR
        name ILIKE '%' || $2::text || '%'
      )
ORDER BY language ASC, created_at DESC
LIMIT $3 OFFSET $4;
