#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v kind >/dev/null 2>&1; then
  if command -v brew >/dev/null 2>&1; then
    echo "kind not found; installing with brew..."
    brew install kind
  else
    echo "kind is required for e2e. Install with: brew install kind" >&2
    echo "See https://kind.sigs.k8s.io/docs/user/quick-start/" >&2
    exit 2
  fi
fi

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is required. Install with: brew install kubectl" >&2
  exit 2
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required to build and load the e2e image" >&2
  exit 2
fi

CLUSTER="${KIND_CLUSTER:-klimatsearch-e2e}"
if ! kind get clusters | grep -qx "$CLUSTER"; then
  kind create cluster --name "$CLUSTER"
fi

docker build -t klimatsearch:e2e .
kind load docker-image klimatsearch:e2e --name "$CLUSTER"
kubectl apply -k deploy/k8s
kubectl rollout status deployment/klimatsearch --timeout=180s

kubectl port-forward svc/klimatsearch 18080:8080 >/tmp/klimatsearch-pf.log 2>&1 &
PF=$!
cleanup() {
  kill "$PF" 2>/dev/null || true
}
trap cleanup EXIT

for i in $(seq 1 30); do
  if curl -sf http://127.0.0.1:18080/healthz >/dev/null; then
    break
  fi
  sleep 1
done

curl -sf http://127.0.0.1:18080/healthz | grep -q ok
BODY=""
for i in $(seq 1 30); do
  BODY=$(curl -sf "http://127.0.0.1:18080/api/resources?lang=sv" || true)
  if echo "$BODY" | python3 -c 'import json,sys; d=json.load(sys.stdin); raise SystemExit(0 if len(d.get("resources") or [])>=1 else 1)'; then
    break
  fi
  sleep 1
done
python3 - "$BODY" <<'PY'
import json, sys
d = json.loads(sys.argv[1])
assert len(d.get("resources") or []) >= 1, d
print(d["resources"][0]["id"])
PY
ID=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["resources"][0]["id"])' "$BODY")
curl -sf "http://127.0.0.1:18080/api/search?q=betong&vector=false&lang=sv" >/dev/null
curl -sf "http://127.0.0.1:18080/api/search?q=betong&vector=true&lang=sv" >/dev/null
curl -sf "http://127.0.0.1:18080/api/resources/${ID}" >/dev/null
echo "e2e-kind: ok"
