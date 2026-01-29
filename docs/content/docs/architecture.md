---
title: "Architecture"
description: "Understanding the LiteFunctions system architecture"
summary: "Learn about the components, data flow, and design principles of LiteFunctions"
date: 2026-01-30T00:00:00+05:30
lastmod: 2026-01-30T00:00:00+05:30
draft: false
weight: 300
toc: true
seo:
  title: "LiteFunctions Architecture"
  description: "Detailed architecture overview of LiteFunctions serverless platform, including components, data flow, and interactions."
  canonical: "" # custom canonical URL (optional)
  noindex: false
---

## Overview

LiteFunctions is built on Kubernetes and follows a microservices architecture. Each component is independently deployable and communicates via well-defined interfaces.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Kubernetes Cluster                       │
│                                                                   │
│  ┌─────────────┐      ┌─────────────┐      ┌─────────────┐      │
│  │   Portal    │───── │    Gitea    │───── │   Actions   │      │
│  │   (Web UI)  │      │  (Git Host) │      │   (CI/CD)   │      │
│  └─────────────┘      └─────────────┘      └─────────────┘      │
│         │                 │                     │                │
│         └─────────────────┼─────────────────────┘                │
│                           │                                      │
│                  ┌────────▼────────┐                             │
│                  │     Operator    │                             │
│                  │  (K8s Control)  │                             │
│                  └────────┬────────┘                             │
│                           │                                      │
│         ┌─────────────────┼─────────────────┐                  │
│         │                 │                 │                  │
│  ┌──────▼──────┐   ┌─────▼─────┐   ┌──────▼──────┐             │
│  │   Ingestor  │   │    NATS   │   │    Valkey   │             │
│  │ (Request)   │   │  (Events) │   │   (Cache)   │             │
│  └─────────────┘   └───────────┘   └─────────────┘             │
│         │                                                       │
│         └───────────────┬──────────────────┐                    │
│                         │                  │                    │
│  ┌──────────┐   ┌──────▼──────┐   ┌──────▼──────┐              │
│  │ Function │   │  Function   │   │  Function   │              │
│  │  Runtime │   │   Runtime   │   │   Runtime   │              │
│  │    Go    │   │   Python    │   │    Rust     │              │
│  └──────────┘   └─────────────┘   └─────────────┘              │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘

┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│ PostgreSQL  │      │    Zot      │      │   MinIO     │
│ (Database)  │      │ (Registry)  │      │  (Storage)  │
└─────────────┘      └─────────────┘      └─────────────┘
```

## Components

### Operator

The Kubernetes operator is the heart of LiteFunctions. It manages the lifecycle of function deployments.

**Responsibilities:**

- Manages Custom Resources (Functions, Triggers)
- Deploys function pods based on configuration
- Handles scaling and auto-scaling
- Manages secrets and configurations
- Reconciles desired state with actual state

**Key Features:**

- Built with Kubebuilder
- Uses Kubernetes Custom Resource Definitions
- Implements controller pattern
- Supports webhook validation

### Ingestor

The ingestor handles incoming function execution requests and routes them to the appropriate runtime.

**Responsibilities:**

- Receives HTTP/gRPC requests
- Parses and validates function calls
- Routes requests to NATS topics
- Handles authentication and authorization
- Manages request/response transformation

**Communication:**

- Listens on NATS subjects
- Communicates with function runtimes via NATS JetStream
- Caches responses in Valkey for performance

### Portal

The web UI provides a user-friendly interface for managing the entire platform.

**Features:**

- Function creation and management
- Build and deployment monitoring
- Log viewing and debugging
- Metrics and analytics
- User and permission management

### Runtimes

Each runtime is a containerized environment optimized for executing functions in a specific language.

**Go Runtime:**

- Pre-compiled execution
- Low latency
- Standard library included
- Hot-reload support for development

**Python Runtime:**

- Common packages pre-installed
- Supports async functions
- Virtual environment isolation
- Dependency management

**Rust Runtime:**

- Maximum performance
- Memory safety
- Zero-cost abstractions
- Compiled functions

### Supporting Services

#### NATS (Message Broker)

NATS JetStream provides reliable message delivery and event streaming.

**Use Cases:**

- Function invocation messaging
- Event streaming
- Request-response patterns
- Pub/Sub for triggers

#### Valkey (Redis)

Valkey provides caching and state management.

**Use Cases:**

- Response caching
- Session state
- Rate limiting
- Leader election

#### PostgreSQL

The primary database for persistent data.

**Stored Data:**

- Function configurations
- User information
- Execution logs
- Metrics and analytics

#### Gitea

Integrated Git server for version control.

**Features:**

- Private repositories
- Webhooks for CI/CD
- Integrated authentication
- Pull request workflows

#### Actions

CI/CD pipeline for automated builds and deployments.

**Workflow:**

1. Detect new commits
2. Build function container
3. Push to Zot registry
4. Update Kubernetes deployment
5. Report status

#### Zot

Private container registry for storing function images.

**Features:**

- Push/pull support
- Authentication
- Garbage collection
- Storage optimization

## Data Flow

### Function Deployment Flow

1. **User** commits function code to Gitea
2. **Actions** detects the commit
3. **Actions** builds the function container
4. **Actions** pushes image to Zot
5. **Operator** receives notification
6. **Operator** creates/update function deployment
7. **Runtime pods** start with new image

### Function Execution Flow

1. **Client** sends execution request to Ingestor
2. **Ingestor** validates and authenticates
3. **Ingestor** publishes request to NATS
4. **Runtime** subscribes to NATS subject
5. **Runtime** executes function
6. **Runtime** publishes response to NATS
7. **Ingestor** returns response to client

### Event-Triggered Flow

1. **External Event** (e.g., HTTP, timer) occurs
2. **Trigger Controller** detects event
3. **Trigger Controller** publishes to NATS
4. **Runtime** executes function
5. **Runtime** publishes result
6. **Optional**: Results stored in database

## Design Principles

### Kubernetes Native

All components follow Kubernetes best practices and use standard APIs:

- Custom Resources for function definitions
- Controllers for reconciliation
- Services for networking
- ConfigMaps and Secrets for configuration

### Loose Coupling

Components communicate via well-defined interfaces:

- NATS for asynchronous messaging
- REST/gRPC APIs for synchronous calls
- Shared database for state

### Scalability

Designed for horizontal scaling:

- Stateless services where possible
- Shared nothing architecture
- Load balancing through Kubernetes
- Auto-scaling based on metrics

### Observability

Built-in monitoring and tracing:

- Structured logging
- Metrics collection
- Distributed tracing support
- Health checks and readiness probes

## Security

### Authentication & Authorization

- JWT-based authentication
- Role-based access control
- Service-to-service authentication
- User permissions per function

### Network Security

- Network policies for pod isolation
- TLS for all communications
- Secrets management via Kubernetes

### Supply Chain Security

- Signed container images
- Vulnerability scanning
- Immutable infrastructure
- Least privilege execution

## High Availability

- Multiple replicas for stateless components
- NATS clustering for message reliability
- PostgreSQL high availability via PGO
- Graceful rolling updates
- Pod disruption budgets
