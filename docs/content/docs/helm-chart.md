---
title: "Helm Chart"
description: "Deploying LiteFunctions using Helm"
summary: "Learn how to install, configure, and manage LiteFunctions with Helm"
date: 2026-01-30T00:00:00+05:30
lastmod: 2026-01-30T00:00:00+05:30
draft: false
weight: 400
toc: true
seo:
  title: "LiteFunctions Helm Chart"
  description: "Complete guide to deploying LiteFunctions using Helm charts, including installation, configuration, and customization."
  canonical: "" # custom canonical URL (optional)
  noindex: false
---

The LiteFunctions Helm chart simplifies deployment of the entire platform to Kubernetes. It includes all necessary components and dependencies.

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.24+)
- Helm CLI (v3.0+)
- kubectl CLI
- Cluster admin access (for CRD installation)

### Installation

```bash
# Install from OCI registry
helm install litefunctions oci://ghcr.io/ashupednekar/charts/litefunctions

# Or install from local directory
helm install litefunctions ./chart
```

### Verify Installation

```bash
# Wait for all pods to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=litefunctions --timeout=300s

# Check all resources
kubectl get all,crd -l app.kubernetes.io/name=litefunctions

# Get the release status
helm status litefunctions
```

## Configuration

### Default Values

The chart comes with sensible defaults, but you can customize it by creating a `values.yaml` file:

```yaml
redis:
  enabled: true

operator:
  registry: localhost:30050
  vcs_user: ashudev

ingestor:
  nats_url: litefunctions-nats:4222

database:
  enabled: true
  postgresVersion: 16
  pgBouncerReplicas: 1
  instanceReplicas: 2
  instanceVolumeSize: 4Gi
```

### Install with Custom Values

```bash
helm install litefunctions ./chart -f values.yaml
```

## Configuration Options

### Core Components

#### Images

```yaml
images:
  ingestor: ashupednekar535/litefunctions-ingestor:latest
  operator: ashupednekar535/litefunctions-operator:latest
```

#### Operator Settings

```yaml
operator:
  registry: localhost:30050 # Container registry for function images
  vcs_user: ashudev # VCS user for Git operations
  # Additional settings default in operator manager
```

#### Ingestor Settings

```yaml
ingestor:
  nats_url: litefunctions-nats:4222 # NATS connection URL
```

### Dependencies

#### NATS (Message Broker)

```yaml
nats:
  enabled: true
  natsBox:
    enabled: false
  config:
    cluster:
      enabled: true
      replicas: 3
    jetstream:
      enabled: true
      fileStore:
        enabled: true
        pvc:
          enabled: true
          size: 4Gi
```

#### Redis/Valkey (Cache)

```yaml
redis:
  enabled: true

valkey-cluster:
  enabled: true
  global:
    security:
      allowInsecureImages: true
  auth:
    enabled: true
  password: valkeyadmin
```

#### PostgreSQL (Database)

```yaml
database:
  enabled: true
  postgresVersion: 16
  pgBouncerReplicas: 1
  instanceReplicas: 2
  instanceVolumeSize: 4Gi
  initialSchemas:
    - gitea
    - lwsportal
```

#### MinIO (Object Storage)

```yaml
minio:
  enabled: false
  global:
    security:
      allowInsecureImages: true
  image:
    repository: bitnamilegacy/minio
  auth:
    rootUser: ashudev
    rootPassword: minioadmin
  mode: standalone
  defaultBuckets: content
  persistence:
    enabled: true
    size: 15Gi
  ingress:
    enabled: true
    ingressClassName: nginx
    hostname: lws.ashudev.in
    path: /content
    pathType: Prefix
    tls: true
    annotations:
      cert-manager.io/cluster-issuer: letsencrypt
```

#### Gitea (Git Server)

```yaml
gitea:
  enabled: true
  service:
    http:
      type: NodePort
      nodePort: 30080
    ssh:
      type: NodePort
      nodePort: 30022
  postgresql-ha:
    enabled: false
  postgresql:
    enabled: false
  valkey-cluster:
    enabled: false
  gitea:
    admin:
      username: ashudev
      password: gitadmin
      email: gitea@ashudev.in
    config:
      actions:
        enabled: true
      packages:
        enabled: true
      server:
        ROOT_URL: "http://litefunctions-gitea-http.litefunctions.svc.cluster.local:3000/"
        DOMAIN: "litefunctions-gitea-http.litefunctions.svc.cluster.local"
      registry:
        enabled: false
      database:
        DB_TYPE: postgres
        NAME: litefunctions
        SCHEMA: gitea
        SSL_MODE: require
        HOST: "litefunctions-pgbouncer.litefunctions.svc"
    additionalConfigFromEnvs:
      - name: GITEA__DATABASE__HOST
        valueFrom:
          secretKeyRef:
            key: host
            name: litefunctions-pguser-litefunctions
      - name: GITEA__DATABASE__PASSWD
        valueFrom:
          secretKeyRef:
            key: password
            name: litefunctions-pguser-litefunctions
      - name: GITEA__DATABASE__NAME
        valueFrom:
          secretKeyRef:
            key: dbname
            name: litefunctions-pguser-litefunctions
      - name: GITEA__DATABASE__USER
        valueFrom:
          secretKeyRef:
            key: user
            name: litefunctions-pguser-litefunctions
    ingress:
      enabled: false
      tls: []
      hosts:
        - host: git.ashudev.in
          paths:
            - path: /
      className: nginx
      annotations:
        cert-manager.io/cluster-issuer: letsencrypt
```

#### Actions (CI/CD)

```yaml
actions:
  enabled: true
  giteaRootURL: http://litefunctions-gitea-http:3000
  existingSecret: litefunctions-registration-token
  existingSecretKey: token
```

#### Zot (Container Registry)

```yaml
zot:
  service:
    nodePort: 30050
```

#### Web UI

```yaml
ui:
  enabled: false
  server:
    db:
      secret: litefunctions-pguser-litefunctions
```

## Advanced Usage

### Upgrading

```bash
# Check for updates
helm repo update
helm search repo litefunctions

# Upgrade to latest version
helm upgrade litefunctions litefunctions/litefunctions

# Upgrade with custom values
helm upgrade litefunctions ./chart -f values.yaml
```

### Rolling Back

```bash
# View release history
helm history litefunctions

# Rollback to previous version
helm rollback litefunctions

# Rollback to specific revision
helm rollback litefunctions <revision>
```

### Uninstalling

```bash
# Delete the release
helm uninstall litefunctions

# Delete all remaining resources
kubectl delete namespace litefunctions
```

## Resource Management

### Setting Resource Limits

```yaml
operator:
  resources:
    limits:
      cpu: 1000m
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 256Mi

ingestor:
  resources:
    limits:
      cpu: 500m
      memory: 256Mi
    requests:
      cpu: 100m
      memory: 128Mi
```

### Node Selectors

```yaml
operator:
  nodeSelector:
    kubernetes.io/os: linux
    node-type: worker

ingestor:
  nodeSelector:
    kubernetes.io/os: linux
```

## Networking

### Service Types

```yaml
operator:
  service:
    type: ClusterIP

ingestor:
  service:
    type: LoadBalancer
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: nlb
```

### Ingress Configuration

```yaml
ui:
  enabled: true
  ingress:
    enabled: true
    className: nginx
    hosts:
      - host: litefunctions.example.com
        paths:
          - path: /
            pathType: Prefix
    tls:
      - secretName: litefunctions-tls
        hosts:
          - litefunctions.example.com
```

## Monitoring and Logging

### Enable Logging

```yaml
operator:
  logLevel: info

ingestor:
  logLevel: debug
```

### Enable Metrics

```yaml
operator:
  metrics:
    enabled: true
    service:
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8443"
        prometheus.io/path: "/metrics"
```

## Troubleshooting

### Check Pod Status

```bash
# List all pods
kubectl get pods -n litefunctions

# Get pod details
kubectl describe pod <pod-name> -n litefunctions

# View logs
kubectl logs <pod-name> -n litefunctions
```

### Check Operator Status

```bash
# View operator logs
kubectl logs -l app.kubernetes.io/name=operator -n litefunctions

# Check operator status
kubectl get crd
kubectl get functions -n litefunctions
```

### Check Dependencies

```bash
# Check NATS
kubectl get pods -l app.kubernetes.io/name=nats -n litefunctions

# Check PostgreSQL
kubectl get pods -l app.kubernetes.io/name=postgres -n litefunctions

# Check Gitea
kubectl get pods -l app.kubernetes.io/name=gitea -n litefunctions
```

### Common Issues

**Pods not starting:**

```bash
# Check events
kubectl get events -n litefunctions --sort-by='.lastTimestamp'

# Check resource limits
kubectl describe pod <pod-name> -n litefunctions | grep -A 5 "Limits"
```

**CRDs not installed:**

```bash
# Verify CRDs are installed
kubectl get crd | grep litefunctions

# Manually install CRDs
kubectl apply -f chart/crds/
```

## Production Checklist

- [ ] Configure resource limits and requests
- [ ] Enable PodDisruptionBudgets
- [ ] Set up backups for PostgreSQL
- [ ] Configure ingress with TLS
- [ ] Enable monitoring and alerting
- [ ] Review and update security contexts
- [ ] Configure horizontal pod autoscaling
- [ ] Set up log aggregation
- [ ] Configure persistent volumes appropriately
- [ ] Review and update secrets management
- [ ] Test disaster recovery procedures
- [ ] Enable and configure network policies
