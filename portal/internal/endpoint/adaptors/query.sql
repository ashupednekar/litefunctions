-- name: CreateEndpoint :one
INSERT INTO endpoints (project_id, name, method, scope, function_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListEndpointsForProject :many
SELECT e.*, f.name as function_name, f.is_async
FROM endpoints e
JOIN functions f ON e.function_id = f.id
WHERE e.project_id = $1
ORDER BY e.name ASC, e.method ASC;

-- name: GetEndpointByID :one
SELECT *
FROM endpoints
WHERE id = $1;

-- name: UpdateEndpoint :one
UPDATE endpoints
SET name = $2, method = $3, scope = $4, function_id = $5
WHERE id = $1
RETURNING *;

-- name: UpdateEndpointMethodScope :one
UPDATE endpoints
SET method = $2, scope = $3
WHERE id = $1
RETURNING *;

-- name: DeleteEndpoint :exec
DELETE FROM endpoints
WHERE id = $1;

-- name: ListEndpointsSearch :many
SELECT e.*, f.name as function_name, f.is_async
FROM endpoints e
JOIN functions f ON e.function_id = f.id
WHERE e.project_id = $1
  AND (
        $2::text = '' OR
        e.name ILIKE '%' || $2::text || '%'
      )
ORDER BY e.name ASC, e.method ASC
LIMIT $3 OFFSET $4;
