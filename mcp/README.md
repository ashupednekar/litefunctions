# LiteFunctions MCP server

Streamable HTTP MCP server used by the bundled OpenCode agent. It reuses the
`common` module for git access, function samples, and operator registration.

## Tools

| Tool | Purpose |
| --- | --- |
| `list_project_files` | Inspect a project repository |
| `read_project_file` | Read a file |
| `write_project_file` | Write and push a file |
| `list_functions` | List functions from `.litefunctions/functions.json` |
| `create_function` | Add a sync HTTP, WebSocket/SSE, or async function |
| `update_function` | Change code, files, async flag, or endpoints |
| `delete_function` | Remove a function package |
| `create_endpoint` | Add an endpoint entry for a function |

Every write tool takes a `project`. Function metadata is stored in
`.litefunctions/functions.json`; the portal's normal repo sync merges it into
the functions and endpoints tables after an agent turn.

## Environment

- `MCP_LISTEN_ADDR` (default `:8090`)
- `MCP_PATH` (default `/mcp`)
- `MCP_TOKEN` (optional bearer token)
- `OPERATOR_URL` (default `litefunctions-operator:50051`)
- `POD_NAMESPACE` (default `litefunctions`)
- `VCS_VENDOR`, `VCS_BASE_URL`, `VCS_USER`, `VCS_TOKEN`, `VCS_AUTH_MODE`

## Run

```bash
go run .
```

The MCP endpoint expects JSON-RPC 2.0 over HTTP POST. It supports
`initialize`, `ping`, `tools/list`, and `tools/call`.
