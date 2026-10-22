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

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.24+)
- Helm CLI (v3.0+)
- kubectl CLI
- Cluster admin access (for CRD installation)
- Gateway API controller and GatewayClass (default: Istio)

### Installation

For local development on macOS with a working Kubernetes context:

```sh
brew install kubectl helm mkcert
sh chart/local.sh install
```

The helper uses your current Kubernetes context if it is reachable. Otherwise,
on macOS it starts Colima with Kubernetes and Traefik disabled; on Linux it
installs k3s. It checks `kubectl get nodes`, then installs Istio,
configures locally trusted TLS with mkcert, installs the chart, and starts an
HTTPS port-forward. Add the printed host entries to `/etc/hosts`.
Run the helper as your normal user; it requests sudo for certificate trust and
the default HTTPS port 443. Use `HTTPS_PORT=8443` to choose another local port.
`KUBECONFIG` can select a different kubeconfig file.
Use the same overrides for subsequent commands:

```sh
sh chart/local.sh status
sh chart/local.sh stop
sh chart/local.sh start
```

If you need a local cluster on macOS, Colima can run Kubernetes directly:

```sh
brew install colima
sh chart/local.sh install
```

The helper disables Colima's bundled Traefik to avoid port conflicts with Istio.

```bash
# Install from OCI registry
helm install litefunctions oci://registry-1.docker.io/ashupednekar535/litefunctions

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
  giteaSetup: ashupednekar535/litefunctions-gitea-setup:latest
  operator: ashupednekar535/litefunctions-operator:latest
  portal: ashupednekar535/litefunctions-portal:latest
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

#### RustFS (Object Storage)

```yaml
rustfs:
  enabled: false
  replicaCount: 1
  mode:
    standalone:
      enabled: true
    distributed:
      enabled: false
  secret:
    rustfs:
      access_key: ashudev
      secret_key: minioadmin
  storageclass:
    name: ""
    dataStorageSize: 15Gi
    logStorageSize: 256Mi
  ingress:
    enabled: false
  gatewayApi:
    enabled: false
```

Enable object storage with `rustfs.enabled=true`. Replace previous `minio.*`
overrides with RustFS settings; existing MinIO data is not migrated automatically.
RustFS serves the S3 API at `/` and includes its console on service port 9001.
To expose the S3 API and console, set `gatewayApi.hosts.rustfs` and
`gatewayApi.hosts.rustfsConsole`, respectively.
Create the `content` bucket with an S3 client after installation; the RustFS chart
does not support MinIO's `defaultBuckets` setting. TLS uses the shared Gateway
certificate configured under `gatewayApi.tls`.

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

### Gateway API Configuration

Install an implementation of Gateway API and its GatewayClass before installing
LiteFunctions. The default class is `istio`; Istio is managed separately from this
chart. Gateway API CRDs are bundled in `chart/crds`. No NGINX controller is required.

```yaml
gatewayApi:
  enabled: true
  gatewayClassName: istio
  hosts:
    portal: [litefunctions.example.com]
    gitea: [git.example.com]
    rustfs: [s3.example.com]
    rustfsConsole: [storage.example.com]
  http:
    port: 80
  tls:
    enabled: true
    secretName: litefunctions-gateway-tls
    certManager:
      enabled: true
      selfSigned:
        enabled: false
      letsencrypt:
        enabled: true
        email: admin@example.com
```

The shared Gateway routes `/` to the portal and `/lambda` to the ingestor.
Gitea and RustFS routes are created only when those components are enabled.
RustFS uses separate hostnames for its S3 API and console. All enabled route
hostnames are included in the Gateway listeners and certificate DNS names.
An empty portal host list defaults to `litefunctions.portal` and `ui.domain`;
an empty Gitea host list defaults to `litefunctions.gitea` and `gitea.local`.
An empty RustFS host list disables exposure of that endpoint.

TLS defaults to a self-signed certificate. To supply an existing TLS Secret,
set `gatewayApi.tls.certManager.enabled=false`. To use Let's Encrypt, disable
self-signed issuance, provide public DNS names, and expose HTTP port 80.
The bundled cert-manager has Gateway API support enabled; an externally managed
cert-manager must enable it too. See the
[cert-manager HTTP-01 Gateway API solver documentation](https://cert-manager.io/v1.20-docs/configuration/acme/http01/).

### Migrating from Ingress

`ui.ingress`, `gitea.ingress`, `gitea.gitea.ingress`, and `rustfs.ingress` are
deprecated. Enabling them now fails with migration instructions. Move their hosts
to `gatewayApi.hosts`, and move TLS configuration to `gatewayApi.tls`. Keep the
subchart ingress flags disabled. Use the shared Gateway rather than enabling
`rustfs.gatewayApi` separately. NGINX annotations and ingress class settings
can be removed from existing values files. Helm removes previously managed
Ingress resources on upgrade; verify Gateway routing and update DNS before
retiring your old controller. Plan the traffic cutover to avoid downtime.

Set `gatewayApi.enabled=false` to omit the Gateway, routes, and certificate
resources. With Istio and the default listener ports, local access can use:

```bash
kubectl -n litefunctions port-forward svc/litefunctions-gateway-istio 18080:8080 18443:443
curl -H 'Host: litefunctions.portal' http://127.0.0.1:18080/
```

For another GatewayClass, use the service created by that implementation.

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
- [ ] Configure Gateway API routing with TLS
- [ ] Enable monitoring and alerting
- [ ] Review and update security contexts
- [ ] Configure horizontal pod autoscaling
- [ ] Set up log aggregation
- [ ] Configure persistent volumes appropriately
- [ ] Review and update secrets management
- [ ] Test disaster recovery procedures
- [ ] Enable and configure network policies
