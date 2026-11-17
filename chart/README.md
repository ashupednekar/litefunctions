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
sh chart/install.sh
```

The helper uses your current Kubernetes context if it is reachable. Otherwise,
on macOS it starts Colima with Kubernetes and Traefik disabled; on Linux it
installs k3s. It checks `kubectl get nodes`, then installs Istio,
configures locally trusted TLS with mkcert, installs the chart, and starts an
HTTPS port-forward. Add the printed host entries to `/etc/hosts`.
Run the helper as your normal user; it requests sudo for certificate trust and
the default HTTPS port 443. Use `HTTPS_PORT=8443` to choose another local port
without requiring sudo for the port-forward.
`KUBECONFIG` can select a different kubeconfig file.
Use the same overrides for subsequent commands:

```sh
sh chart/install.sh status
sh chart/install.sh stop
sh chart/install.sh start
```

If you need a local cluster on macOS, Colima can run Kubernetes directly:

```sh
brew install colima
sh chart/install.sh --fresh-cluster
```

The helper disables Colima's bundled Traefik to avoid port conflicts with Istio.
Each install fetches and applies the latest Gateway API Standard CRDs before
installing the chart.
Pass `--argo` to install Argo CD and apply a `litefunctions` Application that
tracks the published Artifact Hub OCI chart (`ARGOCD_CHART_VERSION`, default
`0.2.0`). Argo CD itself is installed from the official `argo/argo-cd` Helm
chart (`ARGOCD_HELM_VERSION`, default `10.10.5`) with its server exposed on
NodePort 30080. Run `./chart/install.sh argo` to open
`http://127.0.0.1:30080`. The regular install path continues to install the
local `chart/` directory by default. `--head` also selects the local chart
explicitly.

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
  enabled: true
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

Object storage is enabled by default so Olly can persist logs and metrics.
Set `rustfs.enabled=false` to opt out and override the Olly storage settings.
Replace previous `minio.*` overrides with RustFS settings; existing MinIO data
is not migrated automatically.
RustFS serves the S3 API at `/` and includes its console on service port 9001.
The defaults expose the S3 API at `lws.ashudev.in` and the console at
`litefunctions.rustfs`; override `gatewayApi.hosts.rustfs` and
`gatewayApi.hosts.rustfsConsole` to change them.
Create the `content` bucket with an S3 client after installation; the RustFS chart
does not support MinIO's `defaultBuckets` setting. TLS uses the shared Gateway
certificate configured under `gatewayApi.tls`.

#### Olly (Observability)

Olly bundles Grafana Alloy, Loki, Mimir, Prometheus, Grafana and Tempo. Logs and
metrics are enabled by default; traces are opt-in. Loki and Mimir persist their
data in the bundled RustFS object store, so `rustfs.enabled=true` is the default.
A post-install Job creates the required buckets.

```yaml
olly:
  logs:
    enabled: true
  metrics:
    enabled: true
  traces:
    enabled: false
  grafana:
    enabled: true
  alloy:
    enabled: true
```

Loki and Mimir use the bundled RustFS endpoint
(`litefunctions-rustfs-svc:9000`) and the `litefunctions-rustfs-secret`
credentials. If you replace RustFS with an external object store, override the
`loki`, `mimir-distributed`, and `tempo` storage and `extraEnvFrom` values
directly.

Prometheus scrapes the cluster and remote-writes to Mimir. Grafana reads logs
from Loki and metrics from Mimir through its Prometheus-compatible API. The
default Grafana host is `litefunctions.grafana`; set `gatewayApi.hosts.grafana`
to change it, or set it to an empty list to skip the HTTPRoute.

Grafana Alloy tails logs from function runtime pods (`app=runtime`, applied by
the operator) and from the ingestor (`app=litefunctions-ingestor`). It forwards
those logs to Loki and adds `namespace`, `pod`, `container`, `lang`, `project`
and `function` labels. Change `alloy.alloy.configMap.content` to broaden or
narrow collection.

Metrics are scraped by Prometheus using the chart's default Kubernetes jobs
(API server, nodes, cAdvisor, annotated pods/services and endpoints) and
remote-written to Mimir. Mimir runs with replication factor 1 because the chart
deploys a single ingester; both Mimir and Loki persist blocks in RustFS.

To enable traces, set `olly.traces.enabled=true`. Tempo also writes to RustFS.
Add a Tempo datasource to `grafana.datasources` pointing at
`http://litefunctions-tempo:3200` and optionally link it to the existing `loki`
and `prom` UIDs.

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
  enabled: true
  db:
    secret: litefunctions-pguser-litefunctions
  admin:
    # Allows the internal litefunctions-system user to sign in with a password
    # instead of a passkey. It has owner access to every project.
    enabled: true
    password: "lfadmin@123" # change before exposing the portal
  # Reverse proxy CIDRs allowed to supply X-Forwarded-* headers. Empty never
  # trusts them. Set this to the ingress/gateway CIDR to preserve client IPs.
  trustedProxies: []
```

The admin password is stored only in Helm's Secret and the portal environment;
it is never written to the database. Disable `ui.admin.enabled` if the system
user should not be able to sign in.

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
chart. `chart/install.sh` fetches and applies the latest Gateway API Standard CRDs.
No NGINX controller is required.

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
    secretName: litefunctions-tls
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
kubectl -n istio-system port-forward svc/istio-ingressgateway 18080:80 18443:443
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
