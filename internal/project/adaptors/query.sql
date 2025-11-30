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

-- name: ListProjectsForUser :many
SELECT p.*
FROM projects p
JOIN user_projects up ON up.project_id = p.id
WHERE up.user_id = $1
ORDER BY p.created_at DESC;

-- name: AddUserToProject :exec
INSERT INTO user_projects (user_id, project_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, project_id) DO UPDATE
SET role = EXCLUDED.role;

-- name: RemoveUserFromProject :exec
DELETE FROM user_projects
WHERE user_id = $1 AND project_id = $2;

-- name: ListProjectMembers :many
SELECT u.*, up.role, up.created_at
FROM user_projects up
JOIN users u ON u.id = up.user_id
WHERE up.project_id = $1
ORDER BY u.name;

