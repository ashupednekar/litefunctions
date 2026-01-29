---
title: "LiteFunctions"
description: "A Kubernetes-native serverless functions platform"
lead: "Deploy and manage serverless functions across multiple runtimes on Kubernetes with ease"
date: 2026-01-30T00:00:00+05:30
lastmod: 2026-01-30T00:00:00+05:30
draft: false
seo:
  title: "LiteFunctions - Kubernetes Serverless Platform"
  description: "LiteFunctions is a Kubernetes-native serverless functions platform supporting Go, Python, and Rust runtimes. Deploy functions using GitOps with built-in CI/CD."
  canonical: "" # custom canonical URL (optional)
  noindex: false
---

LiteFunctions is a powerful serverless functions platform built on Kubernetes. Write your functions in Go, Python, or Rust, push them to your Git repository, and let LiteFunctions handle the rest.

> This is a stop gap documentation to see what it'd look like, might be slop. Do not bother reading yet

## Key Features

- **Multi-Runtime Support**: Go, Python, and Rust runtimes out of the box
- **GitOps Workflow**: Push code to Git, deploy automatically via CI/CD
- **Kubernetes Native**: Built on standard Kubernetes primitives
- **Event-Driven**: Built on NATS JetStream for reliable message delivery
- **Integrated Registry**: Private container registry included (Zot)
- **Full Stack**: Includes Gitea for Git hosting and Actions for CI/CD

## Architecture

LiteFunctions consists of several components working together:

- **Operator**: Kubernetes operator that manages function deployments
- **Ingestor**: Service that processes function execution requests
- **Portal**: Web UI for managing and monitoring functions
- **Runtimes**: Pre-built container images for executing functions

## Quick Start

```bash
helm install litefunctions ./chart
```

Learn more in the [Documentation](docs/)
