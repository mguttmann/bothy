# infra/helm

Optional and later (B12). No chart yet.

## When

A Helm chart is created when an instance is to run on Kubernetes and the Compose stack is not
enough (several `api` processes, a managed platform of the instance operator). Until then Compose
is the only supported way.

## What the chart must adopt

| Topic | Requirement from the specification |
|---|---|
| Roles | separate deployments for `api` and `worker` from the same image (B1) |
| Migration | job `migrate` before the rollout; the rollout waits for success ([16-operations.md](../../docs/spec/16-operations.md) 10) |
| Secrets | secret driver of the cluster or files from Kubernetes secrets, per pod only the ones it needs |
| Database | managed service; no PostgreSQL in the chart |
| Files | S3 driver |
| Telemetry | collector as its own deployment with the same files from `infra/otel`; several collectors need load balancing by trace ID for tail sampling |
| Edge | ingress with TLS; header identity only with a sidecar in the same pod |
| Fan-out | every `api` pod reads the event log itself (R-SY-15); no session affinity needed except for WebSocket connections |
| Health | liveness `/healthz`, readiness `/readyz` on the internal port |

The chart is versioned like every component ([18-release.md](../../docs/spec/18-release.md)).
