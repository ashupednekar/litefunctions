# AGENTS.md — common module (developer guide)

This file is developer guidance for coding agents working on the `common` Go
module. It is **not** the runtime prompt for the LiteFunctions build agent.
The runtime prompt lives in `chart/files/AGENTS.md` and is mounted into the
OpenCode pod by the Helm chart. The MCP server lives in `../mcp` and has its own
`README.md`.

Keep this module dependency-light: it must not import portal, operator,
ingestor, or mcp.

## Layout

- `crd.go` — gRPC helper that asks the operator to create a Function CR.
- `constants.go` — language extensions used by samples and sync.
- `manifest.go` — repo-local function metadata shared by MCP and portal.
- `sample/` — starter function packages for every supported runtime.
- `vcs/` — vendor clients (Gitea/GitHub) and an in-memory git repo helper.
- `proto/` — generated operator gRPC bindings.
- `events/` — event payload types.
- `agent/` — OpenCode client and the project-aware agent service.

## agent package

`agent` is the clean interface between the portal and an OpenCode server.

```go
svc := agent.NewService(agent.Config{
    BaseURL:  "http://litefunctions-opencode:4096",
    Username: "opencode",
    Password: os.Getenv("OPENCODE_SERVER_PASSWORD"),
})

result, err := svc.Prompt(ctx, agent.PromptRequest{
    Project:   "orders",
    ProjectID: projectID,
    UserName:  "ashu",
    Text:      "Build a health endpoint",
})
```

`Service.Prompt` creates a session when `SessionID` is empty, prepends project
context, calls the OpenCode HTTP API, and returns the assistant text. It does
not duplicate the build instructions: those come from the mounted
`AGENTS.md`. `Client` exposes lower-level routes (`CreateSession`,
`SendMessage`, `ListMessages`, `DeleteSession`).

The client talks to both OpenCode API generations:

- legacy: `POST /session`, `POST /session/{id}/message`
- current: `POST /api/session`, `POST /api/session/{id}/prompt`, then wait and
  read `GET /api/session/{id}/message`

Environment variables: `OPENCODE_URL`, `OPENCODE_USERNAME`,
`OPENCODE_PASSWORD`, `OPENCODE_MODEL`, `OPENCODE_AGENT`,
`OPENCODE_TIMEOUT_SECONDS`.

## manifest.go

`manifest.go` defines `.litefunctions/functions.json`, the metadata file the MCP
server writes alongside function files. It exists because the repository is the
source of truth but the portal also needs to know things the file tree cannot
express:

- whether a function is async
- which endpoints to create and with what method/scope
- the authoritative path for the entrypoint

Shape:

```json
{
  "version": 1,
  "functions": [
    {
      "name": "create-order",
      "language": "go",
      "path": "functions/go/create-order/handle.go",
      "is_async": false,
      "endpoints": [
        { "name": "/orders/create-order", "method": "POST", "scope": "public" }
      ]
    }
  ]
}
```

`FunctionsManifest.Upsert` replaces an entry matched by language+name or path,
`Remove` deletes one, and `Marshal` writes stable JSON. The portal reads this
file during `SyncRepoFunctionsToDb`: it walks the repo for packages, then merges
manifest entries into the functions and endpoints tables. Functions created
through the portal (not the agent) are not in the manifest and keep their database
async flag during sync.

## How agents write to a project

The OpenCode container connects to the LiteFunctions MCP server. The MCP server
uses this module's `vcs`, `sample`, and `crd` helpers. The portal does not write
agent-generated code itself. After an agent turn it calls the normal repo sync
path, which turns files and the manifest into portal function and endpoint rows.

Rules for agent-authored functions:

1. Keep `functions/<language>/<name>/handle.<ext>` as the entrypoint.
2. Use the sample package for the language so manifests and helpers are present.
3. Mark async workers in the manifest; they are NATS-backed functions, not plain
   HTTP request/response handlers.
4. WebSocket/SSE functions are normal functions exposed through the ingestor's
   `/lambda/ws/...` and `/lambda/sse/...` routes.
5. Databases, persistent streams, object storage, traffic policies, and scheduled
   jobs are not supported yet. Say so instead of inventing APIs.

## Conventions

- Go 1.26, standard library first.
- Do not add comments that only restate the code.
- Prefer small interfaces and context-aware methods.
- `common` must remain importable by every service, including tests.
