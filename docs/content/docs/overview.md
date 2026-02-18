---
title: "Overview"
description: "What LiteFunctions is and how it fits into your stack."
summary: "A high-level tour of the platform and its core building blocks."
date: 2026-02-19T00:00:00+00:00
lastmod: 2026-02-19T00:00:00+00:00
draft: false
weight: 100
toc: true
---

LiteFunctions is a Kubernetes-native functions platform designed to run entirely inside your infrastructure. It supports both synchronous HTTP functions and asynchronous event-driven workloads, with multiple language runtimes under a unified execution model.

## What You Get

- **Cluster-native control plane**: The control plane lives inside your Kubernetes cluster, so you own and operate it end-to-end.
- **Sync + async execution**: Run request/response handlers and background jobs in the same platform.
- **Multi-language runtimes**: Go, Rust, Python, TypeScript, and Lua share the same execution model.
- **Git-driven operations**: Functions can be updated via repository workflows and runtime hooks.

## Core Components

- **Ingestor**: Routes HTTP requests and events into the functions runtime.
- **Operator**: Orchestrates function deployments and lifecycle operations.
- **Runtime**: Executes functions with language-specific adapters.
- **Messaging + storage**: NATS for eventing, Redis/Valkey for caching, Postgres for persistence.

## Where To Start

1. Read the **Quickstart** to install LiteFunctions with Helm.
2. Review the **Architecture** page to understand the request and event flow.
3. Use the **Helm Chart** reference to customize deployment settings.
