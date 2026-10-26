# infra

Everything that describes an installation of the product, without the values of a concrete
instance. Instance values (host names, accounts, secrets) belong in the private repository of the
instance operator (ADR 0002).

| Folder | Content | Chapter |
|---|---|---|
| `compose/` | Installation with Docker Compose in one command; `smoke.sh` for the portability test | [16 Operations](../docs/spec/16-operations.md) |
| `host/` | Installation packages of the host service `bothy-host` (`.deb`, `.rpm`, `.pkg`, MSI) and their service definitions | [16 Operations](../docs/spec/16-operations.md) 4.4, [18 Release](../docs/spec/18-release.md) |
| `otel/` | OpenTelemetry Collector with exporters per backend | [17 Observability](../docs/spec/17-observability.md) |
| `observability/` | Dashboards, SLO rules, alert routes | [17 Observability](../docs/spec/17-observability.md) |
| `tofu/` | OpenTofu modules per target (later) | [16 Operations](../docs/spec/16-operations.md) |
| `helm/` | Helm chart (optional, later) | [16 Operations](../docs/spec/16-operations.md) |

Gates: Trivy checks Dockerfiles, Kubernetes, Helm and Terraform files for
misconfiguration as soon as they exist (`make trivy`). The portability test
(`.github/workflows/portability.yml`) starts the whole stack from the published
images on a fresh Linux machine.
