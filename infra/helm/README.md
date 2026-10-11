# infra/helm

Optional und später (B12). Noch kein Chart.

## Wann

Ein Helm-Chart entsteht, wenn eine Instanz auf Kubernetes laufen soll und der Compose-Stack nicht
reicht (mehrere `api`-Prozesse, verwaltete Plattform des Betreibers). Bis dahin ist Compose der
einzige unterstützte Weg.

## Was das Chart übernehmen muss

| Thema | Vorgabe aus der Spezifikation |
|---|---|
| Rollen | getrennte Deployments für `api` und `worker` aus demselben Image (B1) |
| Migration | Job `migrate` vor dem Rollout; Rollout wartet auf Erfolg ([16-betrieb.md](../../docs/spec/16-betrieb.md) 10) |
| Geheimnisse | Secret-Treiber des Clusters oder Dateien aus Kubernetes-Secrets, je Pod nur die nötigen |
| Datenbank | verwalteter Dienst; kein PostgreSQL im Chart |
| Dateien | S3-Treiber |
| Telemetrie | Collector als eigenes Deployment mit denselben Dateien aus `infra/otel`; mehrere Collector brauchen Lastverteilung nach Trace-ID für das Tail-Sampling |
| Rand | Ingress mit TLS; Header-Identität nur mit Sidecar im selben Pod |
| Fan-out | jeder `api`-Pod liest das Ereignis-Log selbst (R-SY-15); keine Sitzungsbindung nötig außer für WebSocket-Verbindungen |
| Gesundheit | Liveness `/healthz`, Readiness `/readyz` am internen Port |

Das Chart wird wie jede Komponente versioniert ([18-release.md](../../docs/spec/18-release.md)).
