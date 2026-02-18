---
title: "Architecture"
description: "How LiteFunctions routes traffic and executes functions."
summary: "Control plane, data plane, and runtime responsibilities."
date: 2026-02-19T00:00:00+00:00
lastmod: 2026-02-19T00:00:00+00:00
draft: false
weight: 300
toc: true
---

LiteFunctions runs entirely inside your cluster. The control plane coordinates deployments and runtime orchestration, while the data plane handles request and event execution.

## High-Level Flow

1. **Ingress** receives HTTP traffic and forwards it to the LiteFunctions ingestor.
2. **Ingestor** dispatches requests or publishes events (via NATS) for async processing.
3. **Operator** reconciles function definitions and manages runtime pods.
4. **Runtime** executes the function, returning responses or emitting events.

## Architecture Diagram

![LiteFunctions architecture](../../images/architecture.png)

If you don’t see the diagram, ensure the image exists at `docs/static/images/architecture.png` and refresh.

## Runtime Modes

- **Dynamic runtime**: Shared server per project, starts on-demand and reuses warm processes.
- **Binary runtime**: Dedicated server per function with stricter isolation.
