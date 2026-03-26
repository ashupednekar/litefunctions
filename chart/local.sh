#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PID_FILE="$SCRIPT_DIR/.local-port-forward.pid"
LOG_FILE="$SCRIPT_DIR/.local-port-forward.log"
CERT_FILE="${CERT_FILE:-/tmp/litefunctions-local.pem}"
KEY_FILE="${KEY_FILE:-/tmp/litefunctions-local-key.pem}"

NAMESPACE="${NAMESPACE:-default}"
SERVICE="${SERVICE:-litefunctions-gateway-istio}"
TLS_SECRET="${TLS_SECRET:-litefunctions-gateway-tls}"
HTTPS_PORT="${HTTPS_PORT:-443}"

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

  kill -0 "$pid" 2>/dev/null
}

mkcert_setup() {
  command -v mkcert >/dev/null 2>&1 || {
    echo "mkcert not found"
    exit 1
  }

  command -v kubectl >/dev/null 2>&1 || {
    echo "kubectl not found"
    exit 1
  }

  kubectl -n "$NAMESPACE" delete certificate "$TLS_SECRET" --ignore-not-found=true
  mkcert -install
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

start() {
  if is_running; then
    echo "port-forward already running with pid $(cat "$PID_FILE")"
    status
    return 0
  fi

  rm -f "$PID_FILE"

  nohup kubectl -n "$NAMESPACE" port-forward "svc/$SERVICE" "$HTTPS_PORT:443" >"$LOG_FILE" 2>&1 &
  pid=$!
  echo "$pid" >"$PID_FILE"

  sleep 2

  if ! kill -0 "$pid" 2>/dev/null; then
    echo "failed to start port-forward"
    echo "log:"
    sed -n '1,120p' "$LOG_FILE" || true
    rm -f "$PID_FILE"
    exit 1
  fi

  status
}

stop() {
  if ! is_running; then
    echo "port-forward is not running"
    rm -f "$PID_FILE"
    return 0
  fi

  pid=$(cat "$PID_FILE")
  kill "$pid"
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

case "${1:-status}" in
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
    echo "usage: sh local.sh [mkcert|start|stop|restart|status]"
    exit 1
    ;;
esac
