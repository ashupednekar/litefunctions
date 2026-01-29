-- PROJECT QUERIES

-- name: CreateProject :one
INSERT INTO projects (name, description, created_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetProjectByID :one
SELECT *
FROM projects
WHERE id = $1;

-- name: GetProjectByName :one
SELECT *
FROM projects
WHERE name = $1;
