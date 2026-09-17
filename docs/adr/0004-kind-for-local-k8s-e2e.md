# kind for local Kubernetes e2e

The dataset is tiny and rebuilds from a fixture on boot, so e2e does not need a PVC or a shared cluster. kind (`helm/kind-action` in CI, `scripts/e2e-kind.sh` locally) loads a locally built image, applies `deploy/k8s` kustomize, and curls `/healthz`, list, FTS search, vector search, and resource-by-id. That is enough to prove the container, probes, and HTTP contract without a cloud account. A full managed-cluster suite can wait until there is a production deploy target.
