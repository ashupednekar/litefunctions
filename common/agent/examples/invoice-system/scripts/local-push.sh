#!/usr/bin/env bash
# Deploy the invoices function package to the local LiteFunctions cluster.
#
#   make local-push PROJECT=one
#
# This mirrors the component `local-push` workflow: push the package to the
# project repo in Gitea, apply the Function CR, and wait for the shared python
# runtime to become ready again.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

PROJECT="${PROJECT:-one}"
FUNCTION="${FUNCTION:-invoices}"
NAMESPACE="${NAMESPACE:-litefunctions}"
GITEA_URL="${GITEA_URL:-http://localhost:30080}"
GIT_OWNER="${GIT_OWNER:-ashudev}"
GIT_TOKEN_SECRET="${GIT_TOKEN_SECRET:-litefunctions-admin-token}"
INGESTOR_URL="${INGESTOR_URL:-http://localhost:32130}"
RUNTIME_DEPLOYMENT="${RUNTIME_DEPLOYMENT:-litefunctions-runtime-python-${PROJECT}}"

if [[ -z "${GIT_TOKEN:-}" ]]; then
  GIT_TOKEN="$(kubectl -n "$NAMESPACE" get secret "$GIT_TOKEN_SECRET" -o jsonpath='{.data.token}' | base64 -d)"
fi

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

HOST="${GITEA_URL#http://}"
HOST="${HOST#https://}"
REPO_URL="http://${GIT_OWNER}:${GIT_TOKEN}@${HOST}/${GIT_OWNER}/${PROJECT}.git"

echo "==> cloning ${GIT_OWNER}/${PROJECT}"
git clone -q "$REPO_URL" "$WORKDIR/repo"

mkdir -p "$WORKDIR/repo/functions/python"
rm -rf "$WORKDIR/repo/functions/python/$FUNCTION"
cp -r "$ROOT/functions/python/$FUNCTION" "$WORKDIR/repo/functions/python/$FUNCTION"
find "$WORKDIR/repo/functions" -type d -name __pycache__ -prune -exec rm -rf {} +

cd "$WORKDIR/repo"
if [[ -n "$(git status --porcelain)" ]]; then
  git config user.email "agent@litefunctions.local"
  git config user.name "litefunctions agent"
  git add functions
  git commit -q -m "chore(${FUNCTION}): sync from common/agent/examples/invoice-system"
  git push -q origin HEAD:main
  echo "==> pushed $(git rev-parse --short HEAD)"
  CHANGED=1
else
  echo "==> package already up to date"
  CHANGED=0
fi
cd "$ROOT"

echo "==> applying Function ${FUNCTION} (project ${PROJECT})"
sed "s/REPLACE_ME/${PROJECT}/" "$ROOT/deploy/function.yaml" | kubectl apply -f -

if [[ "$CHANGED" == "1" ]]; then
  echo "==> restarting ${RUNTIME_DEPLOYMENT}"
  kubectl -n "$NAMESPACE" rollout restart "deploy/${RUNTIME_DEPLOYMENT}" >/dev/null
fi
kubectl -n "$NAMESPACE" rollout status "deploy/${RUNTIME_DEPLOYMENT}" --timeout=180s

echo "==> waiting for runtime to serve ${FUNCTION}"
for _ in $(seq 1 30); do
  if curl -fsS --max-time 5 "${INGESTOR_URL}/lambda/${PROJECT}/${FUNCTION}" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

curl -fsS --max-time 10 "${INGESTOR_URL}/lambda/${PROJECT}/${FUNCTION}"
echo
echo "==> ${INGESTOR_URL}/lambda/${PROJECT}/${FUNCTION}"
