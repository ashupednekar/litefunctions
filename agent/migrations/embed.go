package migrations

import "embed"

// Files are the agent-owned schema migrations. They are merged with the portal
// migrations at migrate time so the agent module owns its tables.
//
//go:embed *.sql
var Files embed.FS
