#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PID_FILE="$SCRIPT_DIR/.local-port-forward.pid"
LOG_FILE="$SCRIPT_DIR/.local-port-forward.log"
CERT_FILE="${CERT_FILE:-/tmp/litefunctions-local.pem}"
KEY_FILE="${KEY_FILE:-/tmp/litefunctions-local-key.pem}"
export KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"

NAMESPACE="${NAMESPACE:-litefunctions}"
RELEASE="${RELEASE:-litefunctions}"
ISTIO_NAMESPACE="${ISTIO_NAMESPACE:-istio-system}"
TLS_SECRET="${TLS_SECRET:-litefunctions-gateway-tls}"
GATEWAY_API_CRD_FILE="${TMPDIR:-/tmp}/litefunctions-gateway-api-standard-install.yaml"
GATEWAY_API_CRD_URL="https://github.com/kubernetes-sigs/gateway-api/releases/latest/download/standard-install.yaml"
HTTPS_PORT="${HTTPS_PORT:-443}"
CHART_SOURCE="oci://registry-1.docker.io/ashupednekar535/litefunctions"
ARGOCD_CHART_VERSION="${ARGOCD_CHART_VERSION:-0.2.0}"
ARGOCD_HELM_VERSION="${ARGOCD_HELM_VERSION:-10.10.5}"
USE_HEAD_CHART=true
ACTION=default
ACTION_SET=false
INSTALL_ARGO=false
FRESH_CLUSTER=false

for arg in "$@"; do
  case "$arg" in
    --head)
      USE_HEAD_CHART=true
      ;;
    --argo)
      INSTALL_ARGO=true
      ;;
    --fresh-cluster)
      FRESH_CLUSTER=true
      ;;
    -h|--help)
      ACTION=help
      ACTION_SET=true
      ;;
    -* )
      echo "unknown option: $arg" >&2
      echo "usage: sh install.sh [--head] [--argo] [--fresh-cluster] [mkcert|start|stop|restart|status]" >&2
      exit 1
      ;;
    *)
      if [ "$ACTION_SET" = true ]; then
        echo "unexpected argument: $arg" >&2
        echo "usage: sh install.sh [--head] [--argo] [--fresh-cluster] [mkcert|start|stop|restart|status]" >&2
        exit 1
      fi
      ACTION=$arg
      ACTION_SET=true
      ;;
  esac
done

PORTAL_HOST="${PORTAL_HOST:-litefunctions.portal}"
GITEA_HOST="${GITEA_HOST:-litefunctions.gitea}"
GRAFANA_HOST="${GRAFANA_HOST:-litefunctions.grafana}"
ARGO_NODE_PORT="${ARGO_NODE_PORT:-30080}"

is_running() {
  if [ ! -f "$PID_FILE" ]; then
    return 1
  fi

  pid=$(cat "$PID_FILE")
  if [ -z "$pid" ]; then
    return 1
  fi

  ps -p "$pid" -o pid= >/dev/null 2>&1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "$1 not found" >&2
    exit 1
  }
}

ensure_cluster() {
  if [ "$FRESH_CLUSTER" = true ]; then
    case "$(uname -s)" in
      Darwin)
        require_command colima
        colima delete --force
        ;;
      Linux)
        require_command sudo
        if [ -x /usr/local/bin/k3s-uninstall.sh ]; then
          sudo /usr/local/bin/k3s-uninstall.sh
        fi
        ;;
      *)
        echo "Fresh cluster setup supports macOS and Linux" >&2
        exit 1
        ;;
    esac
  fi

  if command -v kubectl >/dev/null 2>&1 && kubectl get nodes >/dev/null 2>&1; then
    return 0
  fi

  case "$(uname -s)" in
    Darwin)
      require_command kubectl
      require_command colima
      colima start --memory "${COLIMA_MEMORY:-6}" --kubernetes \
        --k3s-arg=--disable=traefik --k3s-arg=--disable=servicelb
      kubectl get nodes
      return 0
      ;;
    Linux)
      if [ -x /usr/local/bin/k3s-uninstall.sh ]; then
        require_command sudo
        if command -v systemctl >/dev/null 2>&1; then
          sudo systemctl start k3s || true
        elif command -v service >/dev/null 2>&1; then
          sudo service k3s start || true
        fi
        attempt=0
        until kubectl get nodes >/dev/null 2>&1; do
          attempt=$((attempt + 1))
          if [ "$attempt" -ge 30 ]; then
            echo "Existing k3s cluster is unavailable; tried to start it and will not reinstall it automatically" >&2
            exit 1
          fi
          sleep 2
        done
        kubectl get nodes
        return 0
      fi
      ;;
    *)
      echo "Automatic cluster setup supports Linux; configure a working kubectl context first" >&2
      exit 1
      ;;
  esac

  require_command sudo
  require_command curl
  installer=$(mktemp)
  curl -sfL https://get.k3s.io -o "$installer"
  sudo env INSTALL_K3S_EXEC="--disable=traefik --disable=servicelb" sh "$installer"
  rm -f "$installer"

  mkdir -p "$HOME/.kube"
  sudo install -o "$(id -u)" -g "$(id -g)" -m 600 \
    /etc/rancher/k3s/k3s.yaml "$KUBECONFIG"
  attempt=0
  until kubectl get nodes >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 30 ]; then
      echo "k3s did not become available" >&2
      exit 1
    fi
    sleep 2
  done
  kubectl get nodes
}

ensure_istio() {
  control_plane=$(kubectl -n "$ISTIO_NAMESPACE" get deployment istiod --ignore-not-found -o name)
  if [ -n "$control_plane" ]; then
    return 0
  fi

  if command -v istioctl >/dev/null 2>&1; then
    istioctl_bin=$(command -v istioctl)
  elif [ -x "${HOME}/.istioctl/bin/istioctl" ]; then
    istioctl_bin="${HOME}/.istioctl/bin/istioctl"
  else
    require_command curl
    echo "Downloading istioctl"
    export ISTIO_VERSION=1.31.3 # TODO: find a way to fetch latest 
    curl -fsSL https://istio.io/downloadIstioctl | sh - 
    istioctl_bin="${HOME}/.istioctl/bin/istioctl"
    if [ ! -x "$istioctl_bin" ]; then
      echo "istioctl download failed" >&2
      exit 1
    fi
  fi

  echo "Installing Istio in $ISTIO_NAMESPACE"
  "$istioctl_bin" install --kubeconfig "$KUBECONFIG" --set profile=default \
    --set "namespace=$ISTIO_NAMESPACE" \
    --set "values.global.istioNamespace=$ISTIO_NAMESPACE" \
    --set "values.pilot.resources.requests.memory=${ISTIOD_MEMORY_REQUEST:-256Mi}" \
    --skip-confirmation
  kubectl -n "$ISTIO_NAMESPACE" get deployment istiod
}

mkcert_setup() {
  if [ "$(id -u)" -eq 0 ]; then
    echo "Run this script as your normal user; mkcert -install will request sudo when needed" >&2
    exit 1
  fi
  require_command mkcert
  require_command kubectl
  if [ ! -r "$KUBECONFIG" ]; then
    echo "Kubernetes config not readable: $KUBECONFIG" >&2
    exit 1
  fi
  ensure_istio

  kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

  cert_crd=$(kubectl get crd certificates.cert-manager.io --ignore-not-found -o name)
  if [ -n "$cert_crd" ]; then
    kubectl -n "$NAMESPACE" delete certificate "$TLS_SECRET" --ignore-not-found=true
  fi
  mkcert -install
  for file in "$CERT_FILE" "$KEY_FILE"; do
    if [ -e "$file" ] && [ ! -w "$file" ]; then
      sudo chown "$(id -u):$(id -g)" "$file"
    fi
  done
  cert_hosts="$PORTAL_HOST $GITEA_HOST $GRAFANA_HOST"
  # Intentional word splitting: mkcert accepts each hostname as a separate argument.
  mkcert -cert-file "$CERT_FILE" -key-file "$KEY_FILE" $cert_hosts
  kubectl -n "$NAMESPACE" create secret tls "$TLS_SECRET" \
    --cert="$CERT_FILE" \
    --key="$KEY_FILE" \
    --dry-run=client -o yaml | kubectl apply -f -

  echo
  echo "add these to /etc/hosts if not already present:"
  echo "127.0.0.1 $PORTAL_HOST"
  echo "127.0.0.1 $GITEA_HOST"
  echo "127.0.0.1 $GRAFANA_HOST"
}

install() {
  require_command curl
  ensure_cluster
  require_command kubectl
  require_command helm
  mkcert_setup

  gateway_api_crd_tmp="$GATEWAY_API_CRD_FILE.tmp"
  curl --fail --location "$GATEWAY_API_CRD_URL" --output "$gateway_api_crd_tmp"
  mv "$gateway_api_crd_tmp" "$GATEWAY_API_CRD_FILE"
  kubectl apply --server-side --force-conflicts \
    -f "$GATEWAY_API_CRD_FILE"
  kubectl wait --for=condition=Established --timeout=2m \
    crd/gatewayclasses.gateway.networking.k8s.io \
    crd/gateways.gateway.networking.k8s.io \
    crd/httproutes.gateway.networking.k8s.io

  if [ "$INSTALL_ARGO" = true ]; then
    kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -
    helm repo add argo https://argoproj.github.io/argo-helm --force-update
    helm repo update argo
    helm upgrade --install argocd argo/argo-cd \
      --version "$ARGOCD_HELM_VERSION" \
      --namespace argocd --create-namespace --take-ownership --wait --timeout 10m \
      --hide-notes \
      --set configs.params.server.insecure=true \
      --set server.service.type=NodePort \
      --set "server.service.nodePortHttp=$ARGO_NODE_PORT"
    register_helm_repo() {
      repo_name=$1
      repo_url=$2
      oci=${3:-false}
      kubectl -n argocd create secret generic "helm-repo-$repo_name" \
        --from-literal=type=helm \
        --from-literal=name="$repo_name" \
        --from-literal=url="$repo_url" \
        --from-literal=enableOCI="$oci" \
        --dry-run=client -o yaml | kubectl apply -f -
      kubectl -n argocd label secret "helm-repo-$repo_name" \
        argocd.argoproj.io/secret-type=repository --overwrite
    }
    register_helm_repo nats https://nats-io.github.io/k8s/helm/charts
    register_helm_repo valkey https://valkey.io/valkey-helm/
    register_helm_repo pgo registry-1.docker.io/ashupednekar535 true
    register_helm_repo litefunctions registry-1.docker.io/ashupednekar535 true
    register_helm_repo rustfs https://charts.rustfs.com/
    register_helm_repo gitea https://dl.gitea.com/charts/
    register_helm_repo zot http://zotregistry.dev/helm-charts
    register_helm_repo cert-manager quay.io/jetstack/charts true
    kubectl rollout status deployment/argocd-server -n argocd --timeout=5m
    kubectl rollout status statefulset/argocd-application-controller -n argocd --timeout=5m
    admin_password=$(kubectl -n argocd get secret argocd-initial-admin-secret \
      -o jsonpath='{.data.password}' | base64 --decode)
    if [ -z "$admin_password" ]; then
      echo "Argo CD initial admin password is missing from argocd-initial-admin-secret" >&2
      exit 1
    fi
    echo "Argo CD: http://localhost:$ARGO_NODE_PORT"
    echo "Argo CD username: admin"
    echo "Argo CD password: $admin_password"
    kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: litefunctions
  namespace: argocd
spec:
  project: default
  source:
    repoURL: registry-1.docker.io/ashupednekar535
    chart: litefunctions
    targetRevision: ${ARGOCD_CHART_VERSION}
    helm:
      parameters:
        - name: gatewayApi.tls.certManager.enabled
          value: "false"
  destination:
    server: https://kubernetes.default.svc
    namespace: ${NAMESPACE}
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
EOF
    echo "Waiting for LiteFunctions Application to sync in Argo CD..."
    kubectl wait --for=jsonpath='{.status.sync.status}'=Synced \
      application/litefunctions -n argocd --timeout=10m
    echo "LiteFunctions Application synced."
    echo "Run ./chart/install.sh start after the LiteFunctions Application is Healthy to start the LiteFunctions port-forward."
    exit 0
  else
    if [ "$USE_HEAD_CHART" = true ]; then
      chart_source=$SCRIPT_DIR
      dependency_status=$(helm dependency list "$chart_source")
      if printf '%s\n' "$dependency_status" | awk 'NR > 1 && NF && $NF != "ok" { needed = 1 } END { exit !needed }'; then
        helm dependency update "$chart_source"
      fi
    else
      chart_source=$CHART_SOURCE
    fi
    helm upgrade --install "$RELEASE" "$chart_source" --namespace "$NAMESPACE" \
      --create-namespace --debug \
      --set gatewayApi.tls.certManager.enabled=false
  fi
  if [ "$INSTALL_ARGO" != true ]; then
    helm ls -A
  fi

  start
  echo "Checking https://$PORTAL_HOST:$HTTPS_PORT/"
  attempt=0
  until curl --noproxy '*' --fail --show-error --silent --max-time 15 \
    --cacert "$(mkcert -CAROOT)/rootCA.pem" \
    --resolve "$PORTAL_HOST:$HTTPS_PORT:127.0.0.1" \
    -o /dev/null "https://$PORTAL_HOST:$HTTPS_PORT/"; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 12 ]; then
      echo "Portal HTTPS check failed" >&2
      exit 1
    fi
    sleep 5
  done
  echo "Portal HTTPS check passed"
  kubectl -n "$NAMESPACE" rollout restart deployment/litefunctions-portal
}

start() {
  require_command curl
  require_command kubectl
  ensure_cluster
  gateway_info=$(kubectl -n "$NAMESPACE" get gateways.gateway.networking.k8s.io \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.gatewayClassName}{"\n"}{end}')
  if [ -z "$gateway_info" ]; then
    echo "No Gateway found in namespace $NAMESPACE; install the LiteFunctions chart first" >&2
    exit 1
  fi
  set -- $gateway_info
  GATEWAY_SERVICE="$1-$2"
  attempt=0
  until kubectl -n "$NAMESPACE" get service "$GATEWAY_SERVICE" >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 60 ]; then
      echo "Istio has not created service $NAMESPACE/$GATEWAY_SERVICE for the Gateway" >&2
      kubectl -n "$NAMESPACE" get gateway "$1" -o yaml >&2 || true
      exit 1
    fi
    sleep 2
  done
  kubectl -n "$NAMESPACE" wait --for=condition=Available \
    "deployment/$GATEWAY_SERVICE" --timeout=5m
  kubectl_bin=$(command -v kubectl)
  if is_running; then
    echo "stopping existing port-forward pid $(cat "$PID_FILE") before restart"
    stop
  fi

  rm -f "$PID_FILE"

  if [ "$HTTPS_PORT" -lt 1024 ]; then
    require_command sudo
    sudo -v
  fi
  if [ -e "$LOG_FILE" ] && [ ! -w "$LOG_FILE" ]; then
    sudo chown "$(id -u):$(id -g)" "$LOG_FILE"
  fi
  if [ "$HTTPS_PORT" -lt 1024 ]; then
    nohup sudo -n "$kubectl_bin" --kubeconfig "$KUBECONFIG" -n "$NAMESPACE" \
      port-forward "svc/$GATEWAY_SERVICE" "$HTTPS_PORT:443" >"$LOG_FILE" 2>&1 &
  else
    nohup "$kubectl_bin" --kubeconfig "$KUBECONFIG" -n "$NAMESPACE" \
      port-forward "svc/$GATEWAY_SERVICE" "$HTTPS_PORT:443" >"$LOG_FILE" 2>&1 &
  fi
  pid=$!
  echo "$pid" >"$PID_FILE"

  attempt=0
  until forward_ready; do
    attempt=$((attempt + 1))
    if ! is_running || [ "$attempt" -ge 30 ]; then
      echo "failed to reach port-forward at https://127.0.0.1:$HTTPS_PORT" >&2
      echo "log:" >&2
      sed -n '1,120p' "$LOG_FILE" >&2 || true
      kill "$pid" 2>/dev/null || sudo kill "$pid" 2>/dev/null || true
      rm -f "$PID_FILE"
      exit 1
    fi
    sleep 1
  done

  check_https() {
    host=$1
    if curl --noproxy '*' --insecure --silent --show-error --fail --output /dev/null \
      --connect-timeout 2 --max-time 10 \
      --resolve "$host:$HTTPS_PORT:127.0.0.1" \
      "https://$host:$HTTPS_PORT/"; then
      echo "HTTPS check passed: $host"
    else
      echo "HTTPS check failed: $host" >&2
      return 1
    fi
  }

  check_https "$PORTAL_HOST"
  if kubectl -n "$NAMESPACE" get httproutes.gateway.networking.k8s.io \
    -o jsonpath='{range .items[*]}{.spec.hostnames[*]}{"\n"}{end}' | grep -F -q "$GITEA_HOST"; then
    check_https "$GITEA_HOST"
  fi
  if kubectl -n "$NAMESPACE" get httproutes.gateway.networking.k8s.io \
    -o jsonpath='{range .items[*]}{.spec.hostnames[*]}{"\n"}{end}' | grep -F -q "$GRAFANA_HOST"; then
    check_https "$GRAFANA_HOST"
  fi
  status
}

forward_ready() {
  curl --noproxy '*' --insecure --silent --output /dev/null \
    --connect-timeout 1 --max-time 2 \
    --resolve "$PORTAL_HOST:$HTTPS_PORT:127.0.0.1" \
    "https://$PORTAL_HOST:$HTTPS_PORT/"
}

stop() {
  if ! is_running; then
    echo "port-forward is not running"
    rm -f "$PID_FILE"
    return 0
  fi

  pid=$(cat "$PID_FILE")
  kill "$pid" 2>/dev/null || sudo kill "$pid"
  rm -f "$PID_FILE"
  echo "stopped port-forward pid $pid"
}

status() {
  if is_running; then
    pid=$(cat "$PID_FILE")
    echo "port-forward pid: $pid"
  else
    echo "port-forward is not running"
  fi

  echo "log: $LOG_FILE"
  echo
  echo "hosts:"
  echo "127.0.0.1 $PORTAL_HOST"
  echo "127.0.0.1 $GITEA_HOST"
  echo "127.0.0.1 $GRAFANA_HOST"
  echo
  echo "mkcert:"
  echo "sh install.sh mkcert"
  echo
  echo "install:"
  echo "sh install.sh"
  echo
  echo "start:"
  echo "sh install.sh start"
  echo
  echo "browser:"
  echo "https://$PORTAL_HOST"
  echo "https://$GITEA_HOST"
  echo "https://$GRAFANA_HOST"
}

open_argo() {
  if ! kubectl -n argocd get service argocd-server >/dev/null 2>&1; then
    echo "Argo CD is not installed; run ./chart/install.sh --argo first" >&2
    exit 1
  fi
  argo_url="http://localhost:$ARGO_NODE_PORT"
  case "$(uname -s)" in
    Darwin) open "$argo_url" ;;
    Linux) xdg-open "$argo_url" ;;
    *) echo "$argo_url" ;;
  esac
}

case "$ACTION" in
  help)
    echo "usage: sh install.sh [--head] [--argo] [--fresh-cluster] [mkcert|start|stop|restart|status|argo]"
    echo "  --head  install the chart from this checkout; default installs the published Artifact Hub chart"
    echo "  --argo  install Argo CD in the cluster"
    echo "  --fresh-cluster  recreate the local Colima or k3s cluster before installing"
    ;;
  default)
    install
    ;;
  mkcert)
    mkcert_setup
    ;;
  start)
    start
    ;;
  stop)
    stop
    ;;
  restart)
    stop
    start
    ;;
  status)
    status
    ;;
  argo)
    open_argo
    ;;
  *)
    echo "unknown command: $ACTION" >&2
    echo "usage: sh install.sh [--head] [--argo] [--fresh-cluster] [mkcert|start|stop|restart|status]" >&2
    exit 1
    ;;
esac
