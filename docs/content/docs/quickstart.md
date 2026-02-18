---
title: "Quickstart"
description: "Install LiteFunctions and verify the platform."
summary: "Deploy LiteFunctions with Helm and validate the control plane."
date: 2026-02-19T00:00:00+00:00
lastmod: 2026-02-19T00:00:00+00:00
draft: false
weight: 200
toc: true
---

This quickstart installs LiteFunctions into your Kubernetes cluster using Helm and verifies that all core components are running.

## Prerequisites

- Kubernetes v1.24+
- Helm v3+
- `kubectl` configured for your target cluster

## Install

```bash
helm install litefunctions oci://ghcr.io/ashupednekar/charts/litefunctions
```

## Verify

```bash
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=litefunctions --timeout=300s
kubectl get all,crd -l app.kubernetes.io/name=litefunctions
helm status litefunctions
```

## Next

- Move to the **Architecture** page to understand the request and event flow.
- See **Helm Chart** for configuration options and production settings.
