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
GATEWAY_NAMESPACE="${GATEWAY_NAMESPACE:-$NAMESPACE}"
SERVICE="${SERVICE:-litefunctions-gateway-istio}"
TLS_SECRET="${TLS_SECRET:-litefunctions-gateway-tls}"
HTTPS_PORT="${HTTPS_PORT:-443}"
CHART_SOURCE="oci://registry-1.docker.io/ashupednekar535/litefunctions"
USE_HEAD_CHART=true
ACTION=status
ACTION_SET=false

for arg in "$@"; do
  case "$arg" in
    --head)
      USE_HEAD_CHART=true
      ;;
    -h|--help)
      ACTION=help
      ACTION_SET=true
      ;;
    -* )
      echo "unknown option: $arg" >&2
      echo "usage: sh local.sh [--head] [install|mkcert|start|stop|restart|status]" >&2
      exit 1
      ;;
    *)
      if [ "$ACTION_SET" = true ]; then
        echo "unexpected argument: $arg" >&2
        echo "usage: sh local.sh [--head] [install|mkcert|start|stop|restart|status]" >&2
        exit 1
      fi
      ACTION=$arg
      ACTION_SET=true
      ;;
  esac
done

PORTAL_HOST="${PORTAL_HOST:-litefunctions.portal}"
GITEA_HOST="${GITEA_HOST:-litefunctions.gitea}"

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
      ;;
    *)
      echo "Automatic cluster setup supports Linux; configure a working kubectl context first" >&2
      exit 1
      ;;
  esac

  require_command sudo
  require_command curl
  if [ -x /usr/local/bin/k3s-uninstall.sh ]; then
    sudo /usr/local/bin/k3s-uninstall.sh
  fi
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
  mkcert -cert-file "$CERT_FILE" -key-file "$KEY_FILE" "$PORTAL_HOST" "$GITEA_HOST"
  kubectl -n "$NAMESPACE" create secret tls "$TLS_SECRET" \
    --cert="$CERT_FILE" \
    --key="$KEY_FILE" \
    --dry-run=client -o yaml | kubectl apply -f -

  echo
  echo "add these to /etc/hosts if not already present:"
  echo "127.0.0.1 $PORTAL_HOST"
  echo "127.0.0.1 $GITEA_HOST"
}

install() {
  require_command helm
  require_command curl
  ensure_cluster
  helm ls -A
  mkcert_setup

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
  require_command sudo
  require_command curl
  require_command kubectl
  kubectl_bin=$(command -v kubectl)
  if is_running; then
    if forward_ready; then
      echo "port-forward already running with pid $(cat "$PID_FILE")"
      status
      return 0
    fi
    echo "stale port-forward pid $(cat "$PID_FILE"); restarting"
    sudo kill "$(cat "$PID_FILE")" 2>/dev/null || true
  fi

  rm -f "$PID_FILE"

  sudo -v
  if [ -e "$LOG_FILE" ] && [ ! -w "$LOG_FILE" ]; then
    sudo chown "$(id -u):$(id -g)" "$LOG_FILE"
  fi
  nohup sudo -n "$kubectl_bin" --kubeconfig "$KUBECONFIG" -n "$GATEWAY_NAMESPACE" \
    port-forward "svc/$SERVICE" "$HTTPS_PORT:443" >"$LOG_FILE" 2>&1 &
  pid=$!
  echo "$pid" >"$PID_FILE"

  attempt=0
  until forward_ready; do
    attempt=$((attempt + 1))
    if ! is_running || [ "$attempt" -ge 30 ]; then
      echo "failed to reach port-forward at https://127.0.0.1:$HTTPS_PORT" >&2
      echo "log:" >&2
      sed -n '1,120p' "$LOG_FILE" >&2 || true
      sudo kill "$pid" 2>/dev/null || true
      rm -f "$PID_FILE"
      exit 1
    fi
    sleep 1
  done

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
  sudo kill "$pid"
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
  echo
  echo "mkcert:"
  echo "sh local.sh mkcert"
  echo
  echo "install:"
  echo "sh local.sh install"
  echo
  echo "start:"
  echo "sh local.sh start"
  echo
  echo "browser:"
  echo "https://$PORTAL_HOST"
  echo "https://$GITEA_HOST"
  echo
  echo "https test:"
  echo "curl --noproxy '*' -k -H 'Host: $PORTAL_HOST' https://127.0.0.1:$HTTPS_PORT/"
  echo "curl --noproxy '*' -k -H 'Host: $GITEA_HOST' https://127.0.0.1:$HTTPS_PORT/"
}

case "$ACTION" in
  help)
    echo "usage: sh local.sh [--head] [install|mkcert|start|stop|restart|status]"
    echo "  --head  install the chart from this checkout; default installs the published Artifact Hub chart"
    ;;
  install)
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
  *)
    echo "unknown command: $ACTION" >&2
    echo "usage: sh local.sh [--head] [install|mkcert|start|stop|restart|status]" >&2
    exit 1
    ;;
esac
