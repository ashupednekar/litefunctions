package adaptors

import "context"

const listAllProjects = `-- name: ListAllProjects :many
SELECT
    id, name, description, created_by, created_at,
    'owner'::text AS role
FROM projects
ORDER BY created_at DESC
`

// ListProjectsForUserOrAdmin returns the projects visible to a normal user, or
// every project for the internal system administrator.
func (q *Queries) ListProjectsForUserOrAdmin(ctx context.Context, userID []byte, isAdmin bool) ([]ListProjectsForUserRow, error) {
	if !isAdmin {
		return q.ListProjectsForUser(ctx, userID)
	}

	rows, err := q.db.Query(ctx, listAllProjects)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ListProjectsForUserRow
	for rows.Next() {
		var i ListProjectsForUserRow
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Description,
			&i.CreatedBy,
			&i.CreatedAt,
			&i.Role,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
