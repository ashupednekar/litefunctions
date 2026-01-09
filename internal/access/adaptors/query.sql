-- name: AddProjectOwner :exec
INSERT INTO user_project_access (
    user_id,
    project_id,
    role
) VALUES (
    $1,
    $2,
    'owner'
)
ON CONFLICT (user_id, project_id)
DO UPDATE SET
    role = 'owner',
    updated_at = now();

-- name: ListProjectsForUser :many
SELECT
    p.*,
    upa.role::text as role
FROM projects p
JOIN user_project_access upa
  ON upa.project_id = p.id
WHERE upa.user_id = $1
ORDER BY p.created_at DESC;

-- name: GetUserProjectRole :one
SELECT role::text
FROM user_project_access
WHERE user_id = $1
  AND project_id = $2;

-- name: HasManagerAccess :one
SELECT 1
FROM user_project_access
WHERE user_id = $1
  AND project_id = $2
  AND role IN ('manager', 'owner');

-- name: HasOwnerAccess :one
SELECT 1
FROM user_project_access
WHERE user_id = $1
  AND project_id = $2
  AND role = 'owner';

-- name: UpdateUserProjectRole :exec
UPDATE user_project_access
SET role = $3::text::project_role,
    updated_at = now()
WHERE user_id = $1
  AND project_id = $2;

-- name: RevokeProjectAccess :exec
DELETE FROM user_project_access
WHERE project_id = $1
  AND user_id = $2
  AND role != 'owner';
  
-- name: CreateProjectInvite :one
INSERT INTO project_invites (
    project_id,
    invite_code,
    created_by,
    expires_at
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING invite_code, expires_at;

-- name: GetValidInviteByCode :one
SELECT
    id,
    project_id,
    created_by
FROM project_invites
WHERE invite_code = $1
  AND used_at IS NULL
  AND expires_at > now()
FOR UPDATE;

-- name: AddViewerToProject :exec
INSERT INTO user_project_access (
    user_id,
    project_id,
    role
) VALUES (
    $1,
    $2,
    'viewer'
)
ON CONFLICT (user_id, project_id) DO NOTHING;

-- name: MarkInviteUsed :exec
UPDATE project_invites
SET used_at = now()
WHERE id = $1;

-- name: ListUsersForProject :many
SELECT
    u.id        AS user_id,
    u.name      AS user_name,
    upa.role::text AS role
FROM user_project_access upa
JOIN users u
  ON u.id = upa.user_id
WHERE upa.project_id = $1
ORDER BY
    CASE upa.role
        WHEN 'owner' THEN 1
        WHEN 'manager' THEN 2
        WHEN 'viewer' THEN 3
    END,
    u.name;
