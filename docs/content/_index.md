---
title: "Litefunctions"
description: "Self-hosted, Kubernetes-native functions platform."
eyebrow: "SELF-HOSTED · KUBERNETES-NATIVE"
hero_title: "Ship functions.<br><span>Not infrastructure.</span>"
hero_sub: "Write in Go, Rust, Python, TypeScript or Lua. Deploy from Git, route traffic at the edge, and keep every secret, stream and schema inside your own cluster."
---

{{< features >}}

## One workspace for the whole function lifecycle

From the first function to production traffic, Litefunctions keeps the pieces in one place. The portal surfaces the runtime, build, data and access controls that usually live in five different dashboards.

- **Functions** — write in five languages with a package-aware editor, Vim/telescope navigation and per-function compute config.
- **Data** — S3-compatible object buckets, SQL table definitions and a sqlc query workspace.
- **Plan** — describe a use case and review the generated functions, tables and routes before anything ships.
- **Audit** — every privileged action lands in an immutable log, available from a Postman-style bottom console.

{{< showcase >}}

## More from the portal

{{< screenshots >}}

## Quickstart

Clone the repository and run the installer:

```bash
git clone https://github.com/ashupednekar/litefunctions.git
cd litefunctions
./chart/install.sh
```

Add the portal and Gitea hostnames to `/etc/hosts`:

```bash
echo "127.0.0.1 litefunctions.portal gitea" | sudo tee -a /etc/hosts
```

Then visit [https://litefunctions.portal](https://litefunctions.portal). Register a passkey, create a project and deploy your first function.

## Built for self-hosting

Litefunctions runs inside Kubernetes and keeps the control plane in your own infrastructure. Bring your own ingress, database and object storage; the operator reconciles functions, endpoints and streams from Git.
