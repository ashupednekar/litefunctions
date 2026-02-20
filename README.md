[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/litefunctions)](https://artifacthub.io/packages/search?repo=litefunctions)
[![Build And Push Images](https://github.com/ashupednekar/litefunctions/actions/workflows/build-and-push-images.yaml/badge.svg)](https://github.com/ashupednekar/litefunctions/actions/workflows/build-and-push-images.yaml)
[![Deploy hugo site to pages](https://github.com/ashupednekar/litefunctions/actions/workflows/docs.yaml/badge.svg)](https://github.com/ashupednekar/litefunctions/actions/workflows/docs.yaml)

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

### What does it do?
https://github.com/user-attachments/assets/20171ef9-876d-4519-9f72-5a6141682a35

1. User calls an endpoint from the Portal or an external client.
2. Ingestor receives the request and asks Operator to ensure the function/runtime is active.
3. Operator reconciles Function CRDs and ensures deployment/service readiness.
4. For sync execution, Ingestor proxies HTTP to runtime services.
5. For async execution, Ingestor publishes execution events over NATS subjects.
6. Runtime workers consume events, execute function code, and publish responses/results.
7. Dynamic language runtimes (Python/TS/Lua) refresh function code from VCS using hook events instead of per-function image builds.
8. Portal tracks runs, status, and endpoint behavior via APIs and live updates.

### Install with Helm (OCI)

```bash
helm install litefunctions oci://registry-1.docker.io/ashupednekar535/litefunctions
```

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

## How does it work?
![LiteFunctions architecture](https://github.com/user-attachments/assets/95c952b3-2f7b-48e9-ad92-36ba0cd1a1c6)
Refer blog: TODO

