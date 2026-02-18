# [LiteFunctions](https://ashupednekar.github.io/litefunctions/)

Kubernetes-native functions platform with sync and async execution, Git-driven updates, and first-class multi-language runtimes.

## Why LiteFunctions

Bottom line:

- No vendor lock-in.
- No language lock-in.

LiteFunctions is built around first principles:

- Keep the control plane in your cluster.
- Keep source-of-truth in your repo.
- Keep runtime choices open.

## Architecture

### High-level diagram

![LiteFunctions architecture](https://github.com/user-attachments/assets/95c952b3-2f7b-48e9-ad92-36ba0cd1a1c6)

### Request/data flow

![LiteFunctions flow](https://github.com/user-attachments/assets/c6c23289-675a-454a-bcb3-a1407086a846)

### Diagram walkthrough

The diagram represents a Kubernetes-native control and data path:

1. User calls an endpoint from the Portal or an external client.
2. Ingestor receives the request and asks Operator to ensure the function/runtime is active.
3. Operator reconciles Function CRDs and ensures deployment/service readiness.
4. For sync execution, Ingestor proxies HTTP to runtime services.
5. For async execution, Ingestor publishes execution events over NATS subjects.
6. Runtime workers consume events, execute function code, and publish responses/results.
7. Dynamic language runtimes (Python/TS/Lua) refresh function code from VCS using hook events instead of per-function image builds.
8. Portal tracks runs, status, and endpoint behavior via APIs and live updates.

## Who It Is For

- Teams running Kubernetes who want function-style deployment without moving to a managed FaaS vendor.
- Platform engineers who need predictable control over network, storage, auth, and CI/CD.
- Product teams that want one platform for multiple languages and execution styles.
- Self-hosters and enterprises that care about portability, sovereignty, and auditability.

## Features

Current platform features:

- Kubernetes-native function lifecycle via CRDs and operator reconciliation.
- Multi-language runtimes: Go, Rust, Python, TypeScript (Bun), Lua.
- Sync HTTP execution and async pub/sub execution.
- Dynamic runtime refresh for Python/TS/Lua via VCS hook events.
- Git-integrated workflow support (Gitea/GitHub workflow templates).
- Auto endpoint provisioning and endpoint management from Portal.
- Build/action visibility in Portal with live status updates.
- Runtime activation + keep-warm/deprovision lifecycle management.

## Upcoming Features

Planned next:

- Authorization policy engine for endpoints (authz policies).
- Richer authn/authz integration across HTTP/WS/SSE surfaces.
- More granular routing and traffic controls.
- Enhanced observability (structured traces/metrics per function).
- Runtime hardening and policy guardrails for multi-tenant environments.

## Usage

### Install with Helm (OCI)

```bash
helm install litefunctions oci://ghcr.io/ashupednekar/charts/litefunctions
```

### Basic flow

1. Create/connect a project repository.
2. Add function files under language directories (for example `functions/go`, `functions/python`, `functions/ts`, `functions/lua`, `functions/rust`).
3. Push changes.
4. Use Portal to invoke and monitor endpoints.

## Demo

Demo assets (placeholders for now):

- Screenshot 1: Portal dashboard walkthrough (TBD)
- Screenshot 2: Endpoint test + live action status (TBD)
- Video 1: End-to-end create -> push -> invoke flow (TBD)
- Video 2: Dynamic runtime hook refresh for Python/TS/Lua (TBD)

## License

MIT
