---
title: "Introduction"
description: "Get started with LiteFunctions - a Kubernetes-native serverless functions platform"
summary: "Learn what LiteFunctions is and how to get started"
date: 2026-01-30T00:00:00+05:30
lastmod: 2026-01-30T00:00:00+05:30
draft: false
weight: 100
toc: true
seo:
  title: "Introduction to LiteFunctions"
  description: "Learn about LiteFunctions, a Kubernetes-native serverless functions platform supporting Go, Python, and Rust runtimes."
  canonical: "" # custom canonical URL (optional)
  noindex: false
---

## What is LiteFunctions?

LiteFunctions is a serverless functions platform built on Kubernetes. It allows you to write functions in Go, Python, or Rust, deploy them via Git, and execute them in a fully managed environment.

## How It Works

1. **Write Your Function**: Create a function in your preferred language (Go, Python, or Rust)
2. **Push to Git**: Commit and push your code to the integrated Gitea Git server
3. **Automatic Build**: The CI/CD pipeline builds and deploys your function automatically
4. **Execute**: Trigger your function via HTTP, NATS messages, or other event sources

## Prerequisites

- Kubernetes cluster (v1.24+)
- Helm CLI
- kubectl CLI

## Quick Install

```bash
# Clone the repository
git clone https://github.com/ashupednekar/litefunctions.git
cd litefunctions

# Install using Helm
helm install litefunctions ./chart

# Wait for all pods to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=litefunctions --timeout=300s
```

## Your First Function

### Create a Function

```bash
# Create a new function repository
# (This will be available through the web UI or CLI)
```

### Write Your Code

**Python Example:**

```python
def handler(event):
    return {
        "message": f"Hello, {event.get('name', 'World')}!"
    }
```

### Deploy

```bash
# Push to your Git repository
git add .
git commit -m "Add hello function"
git push
```

Your function will be automatically built and deployed!

## Next Steps

- Learn about the [Architecture](docs/architecture)
- Understand the [Monorepo Structure](docs/monorepo-structure)
- Explore [Helm Chart Configuration](docs/helm-chart)
