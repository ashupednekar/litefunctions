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

## Building container images

Docker Engine with Buildx or Docker Desktop is required. Images support Linux
on AMD64 and ARM64, including Linux containers on Intel and Apple Silicon Macs.

Build and load a component image for your Docker host's architecture without
pushing to a registry:

```bash
make -C portal docker-build
make -C ingestor docker-build
make -C operator docker-build
```

Publish both architectures under one tag (log in to the registry first):

```bash
make -C portal docker-buildx IMG=your-user/litefunctions-portal:dev
make -C ingestor docker-buildx IMG=your-user/litefunctions-ingestor:dev
make -C operator docker-buildx IMG=your-user/litefunctions-operator:dev
make runtime-python-buildx runtime-ts-buildx runtime-lua-buildx
```

`make build-multiarch` publishes all six images and syncs the Helm artifacts.
GitHub Actions uses the same Makefile targets. `PLATFORMS` defaults to
`linux/amd64,linux/arm64` and can be overridden on the command line.
The existing `make build` and `make runtime-python`, `make runtime-ts`, and
`make runtime-lua` targets still build and publish for the Docker host only.

Multi-platform targets create and reuse a `docker-container` builder named
`litefunctions-builder`; set `BUILDER` to use another builder. These targets push
directly to the registry and do not load the result into the local image store.
Build errors stop Make and return a failure status.

Go binaries are cross-compiled on the builder's native architecture. Building
Python, TypeScript, and Lua for another architecture requires emulation or a
builder with native nodes for both architectures. Docker Desktop includes
emulation; GitHub Actions configures QEMU automatically. With standalone Docker
on Linux, configure emulation once if it is not already available:

```bash
docker run --privileged --rm tonistiigi/binfmt --install amd64,arm64
```

## Upcoming Features

Planned next:

- Package directories for functions, so each function can include helpers, assets,
  tests, and its own dependency manifest. See the proposed layout below.
- Authorization policy engine for endpoints (authz policies).
- Richer authn/authz integration across HTTP/WS/SSE surfaces.
- More granular routing and traffic controls.
- Enhanced observability (structured traces/metrics per function).
- Runtime hardening and policy guardrails for multi-tenant environments.

### Proposed function package layout

Today, a function is a single file such as `functions/python/orders.py`.
The proposed model makes the function directory the unit of discovery, build,
and deployment:

```text
functions/
  python/
    orders/
      function.py          # entry point: exports handle
      helpers.py           # imported by function.py
      requirements.txt     # dependencies for this function
      tests/
        test_orders.py
  ts/
    notifications/
      function.ts          # entry point: exports handle
      formatter.ts
      package.json         # dependencies for this function
```

The directory name is the function name; its parent selects the language.
Each supported language would use `function.<extension>` as its entry point
(`.py`, `.ts`, `.lua`, `.go`, or `.rs`). Files inside the directory belong to
that function and should travel together when it is synced or built. A change
to any file in a package should update that function. Dependency manifests
should be resolved in the package's build or runtime context so functions can
use different dependencies without sharing an implicit project-wide set.

This is a design proposal, **not yet supported by the platform**. Existing
single-file functions continue to use the current layout until package
discovery, Portal editing, runtime loading, and build workflows are updated.

## How does it work?
![LiteFunctions architecture](https://github.com/user-attachments/assets/95c952b3-2f7b-48e9-ad92-36ba0cd1a1c6)
Refer blog: TODO

