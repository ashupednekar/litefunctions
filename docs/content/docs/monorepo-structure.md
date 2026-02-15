---
title: "Monorepo Structure"
description: "Understanding the LiteFunctions monorepo organization"
summary: "Learn about the directory structure and components of the LiteFunctions project"
date: 2026-01-30T00:00:00+05:30
lastmod: 2026-01-30T00:00:00+05:30
draft: false
weight: 200
toc: true
seo:
  title: "LiteFunctions Monorepo Structure"
  description: "Detailed overview of the LiteFunctions monorepo directory structure and component organization."
  canonical: "" # custom canonical URL (optional)
  noindex: false
---

LiteFunctions is organized as a monorepo containing all components of the platform. This structure makes development, testing, and deployment more cohesive and manageable.

## Directory Overview

```
litefunctions/
├── operator/          # Kubernetes operator
├── ingestor/          # Function execution ingestor
├── portal/            # Web UI
├── runtimes/          # Language runtimes
│   ├── go/
│   ├── python/
│   └── rust/
├── chart/             # Helm chart for deployment
├── build/             # Build configurations
├── docs/              # Documentation
└── README.md
```

## Components

### Operator

The operator is the core component that manages function deployments on Kubernetes.

**Location:** `operator/`

**Key Contents:**

- `api/` - Custom Resource Definitions (CRDs) and types
- `cmd/` - Main operator binary entry point
- `config/` - Kubernetes manifests and Kustomize configurations
- `internal/` - Internal business logic
- `Makefile` - Build and deployment targets

**Technology:** Go with Kubebuilder

### Ingestor

The ingestor receives function execution requests and routes them to the appropriate runtime.

**Location:** `ingestor/`

**Key Contents:**

- `cmd/` - Ingestor binary entry point
- `pkg/` - Ingestor business logic
- `go.mod` - Go module dependencies

**Technology:** Go

### Portal

The web UI provides a user-friendly interface for managing functions, viewing logs, and monitoring deployments.

**Location:** `portal/`

**Technology:** [litewebservices-portal](https://github.com/ashupednekar/litewebservices-portal)

### Runtimes

Runtimes are containerized environments for executing functions in different languages.

**Location:** `runtimes/`

#### Go Runtime

**Location:** `runtimes/go/`

Contains the Go execution environment and standard library for running Go functions.

#### Python Runtime

**Location:** `runtimes/python/`

Contains the Python execution environment with common packages for Python functions.

#### Rust Runtime

**Location:** `runtimes/rust/`

Contains the Rust execution environment with the standard library for running Rust functions.

### Build

Build configurations for creating container images and deploying components.

**Location:** `build/`

**Contents:**

- `ingestor/Dockerfile` - Build configuration for ingestor
- `runtimes/Dockerfile.go` - Go runtime container
- `runtimes/Dockerfile.python` - Python runtime container
- `runtimes/Dockerfile.rust` - Rust runtime container
- `runtimes/Dockerfile.rust.base` - Base Rust runtime image

### Chart

Helm chart for deploying the entire LiteFunctions stack to Kubernetes.

**Location:** `chart/`

**Key Contents:**

- `Chart.yaml` - Helm chart metadata and dependencies
- `values.yaml` - Configuration values for all components
- `templates/` - Kubernetes templates
- `crds/` - Custom Resource Definitions

**Dependencies:**

- NATS - Message streaming
- Valkey (Redis) - Caching and state
- PostgreSQL - Database (via PGO)
- Gitea - Git server
- Actions - CI/CD
- Zot - Container registry
- MinIO - Object storage (optional)

### Docs

Documentation for the LiteFunctions platform.

**Location:** `docs/`

Built with Hugo for LiteFunctions documentation.

## Development Workflow

### Building Components

```bash
# Build operator
cd operator
make build

# Build ingestor
cd ingestor
go build -o bin/ingestor ./cmd/main.go

# Build runtimes
# Use Dockerfiles in build/runtimes/
```

### Running Locally

```bash
# Deploy to local Kubernetes cluster
cd chart
helm install litefunctions .

# Install CRDs
cd operator
make install

# Deploy operator
make deploy
```

### Testing

```bash
# Run operator tests
cd operator
make test

# Run ingestor tests
cd ingestor
go test ./...
```

## Contributing

When contributing to LiteFunctions, follow these guidelines:

1. **Component Isolation**: Changes should be focused on specific components
2. **API Changes**: Update CRDs and regenerate code when modifying APIs
3. **Runtime Changes**: Test runtime changes with sample functions
4. **Documentation**: Update relevant documentation alongside code changes
