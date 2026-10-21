# OpenTelemetry Collector

Status: draft M0. Explains [infra/otel/collector.yaml](../../infra/otel/collector.yaml) and the
overlays under [infra/otel/exporters/](../../infra/otel/exporters/). Decision: ADR 0034.

![Telemetry pipeline](diagrams/telemetry-pipeline.svg)

## 1. Version and distribution

`otel/opentelemetry-collector-contrib`, version pinned in `BOTHY_OTELCOL_VERSION`. The
configuration was checked with `otelcol-contrib` 0.162.0 (`otelcol-contrib validate`, base alone,
with each overlay and with three combinations of two overlays). In this version the
components are named `otlp_grpc`, `otlp_http`, `cumulative_to_delta`, `azure_monitor` (output of
`otelcol-contrib components`) [verified].

## 2. Structure of the base

| Part | Component | Purpose |
|---|---|---|
| Receiving | `otlp` (gRPC `:4317`, HTTP `:4318`) | Server, `bothy-backupd`, adapters |
| Receiving | `postgresql` | Database metrics with the role `bothy_monitor`; deadlocks additionally enabled |
| Protection | `memory_limiter` | Rejects under memory pressure instead of crashing; the server then discards and counts |
| Labeling | `resource/instance` | Sets `deployment.environment.name` if the source does not set it |
| Privacy | `transform/scrub` | Deletes `url.query`, `url.full`, client addresses, content attributes of the GenAI conventions, `exception.message` |
| Privacy | `redaction/allowlist` | Spans and logs: only keys from [signals.md](signals.md) 5.3 and the conventions; HMAC for suspicious values |
| Privacy | `transform/scrub_metrics` | Metrics: delete person attributes of third-party sources |
| Sampling | `tail_sampling` | Keeps all traces with an error, all slower than `BOTHY_TRACE_SLOW_THRESHOLD_MS`, and `BOTHY_TRACE_SAMPLE_PERCENT` of the rest |
| Batching | `batch` | Fewer exports |
| Distribution | `forward/traces`, `forward/metrics`, `forward/logs` | End of the inbound pipelines; overlays attach to them |
| Default target | `nop` | Without an overlay, data is discarded |
| Own metrics | Prometheus reader on `:8888` | Prometheus scrapes them (alert `BothyTelemetryExportFailing`) |

Metrics do not pass through the allowlist: Bothy's own attributes are bounded in the code
([signals.md](signals.md) 2), and the processor is only alpha for metrics. For metrics of
third-party sources, `transform/scrub_metrics` deletes person attributes (`user.*`, `enduser.*`,
`session.id`, `organization.id`, `client.address`) on the resource and the data point.

## 3. Overlays

An overlay is a second `--config` file. The collector merges maps and replaces lists.
Therefore every overlay adds its own named pipelines (for example `traces/newrelic`) and changes
no list of the base. Exception: an overlay with an extension must list `service.extensions` in
full; this applies only to `azure-monitor`. Two overlays with extensions at the same time therefore
do not work; all other pairs work.

| Overlay | Target | Signals | Special feature |
|---|---|---|---|
| `none` | none | none | Default |
| `local` | Prometheus, Loki, Tempo in the Compose profile `observability` | all | Metrics cumulative to the OTLP receiver of Prometheus |
| `newrelic` | New Relic OTLP | all | Delta temporality, attributes truncated |
| `grafana` | Grafana Cloud OTLP gateway | all | Whole `Authorization` header from a file |
| `datadog` | Datadog | all | Datadog connector for APM statistics |
| `azure-monitor` | Azure Monitor (native OTLP ingestion) | all | Entra authentication, delta temporality |
| `sentry` | Sentry OTLP | traces, logs | No metrics |

Evidence and limits per backend: [backends.md](backends.md).

Compose selects up to two overlays with `BOTHY_OTEL_EXPORTER` and `BOTHY_OTEL_EXPORTER_2`.

The test overlay `file` (`infra/compose/test/otel/file.yaml`) exists only for the Compose smoke
test (M1-73): it writes the three signals as OTLP JSON lines into a volume of the test overlay,
shortens `tail_sampling.decision_wait` and `batch.timeout` to one second, and is never part of an
installation or of the release bundle. `tools/opscheck` validates it with the base alone.

## 4. Secrets

The collector reads all keys with `${file:/run/bothy/secrets/<name>}` from its
subdirectory of the volume `secrets`. Environment variables carry only endpoints and IDs.

## 5. Limits and operations

1. Tail sampling needs all spans of a trace in the same collector. With several collectors
   (cloud, several hosts) a load balancer by trace ID is needed in front of them (`load_balancing`
   exporter) [derived].
2. APM statistics at Datadog are based on the sample, because sampling runs before the
   overlay.
3. If the collector fails, the server discards once its buffer is full; the message path stays
   unaffected. The alert `BothyServerTelemetryMissing` reports this.
4. Adapters on the same host send to `127.0.0.1:BOTHY_OTLP_PUBLISH_PORT`; adapters on other
   hosts run their own collector with the same files (base plus overlay).

## 6. Check

`make ops-check` checks the configuration in the gate
([tools/opscheck](../../tools/opscheck/README.md)) with `otelcol-contrib` in the version of the
image (check `tool-versions` keeps both equal):

1. Check `collector`: `otelcol-contrib validate` for the base alone, the base with each overlay
   and the base with every ordered pair of two overlays except `none`. Both orders count,
   because Compose sets `BOTHY_OTEL_EXPORTER` before `BOTHY_OTEL_EXPORTER_2` and lists are
   replaced when merging.
2. Check `collector-allowlist`: the `bothy.*` keys of `redaction/allowlist` equal the table in
   [signals.md](signals.md) 5.3, and the allowlist works: `allow_all_keys` is false, every trace
   and log pipeline with a receiver runs it, and no overlay redefines the processor or a
   pipeline of the base.

Before any `validate`, a reference to a config provider other than `env`, `file` and `yaml`
(`${https:...}`, `${s3:...}`), also one inside another reference, is a finding: the collector
resolves it while it validates, and most of these providers fetch over the network (providers
such as `s3` and `googlesecretmanager`:
[confmap/provider](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/443a0a020c3a61e5309ef3cedc83ea360d98a5fc/confmap/provider),
retrieved 2026-10-12). Their clients may bypass the closed proxy of the validators: a sandbox run in
the red team saw `${https:...}` connect past it (ruling M1-04/29). The check runs on a work copy:
instead of `/run/bothy/secrets` it reads placeholder files (`telemetry_pseudonym_key` needs at least
32 bytes), and the `datadog` exporter gets a fixed host name, so `validate` does not wait for host
name detection. It sets only the variables of `COLLECTOR_ENV` in `tools/opscheck/opscheck.py`, the
ones the overlays use without a default (Azure Monitor, Grafana and Sentry endpoints point at
`example.com`, the Azure IDs are zero UUIDs). A new overlay that reads a variable without a default
(`${env:NAME}` instead of `${env:NAME:-<default>}`) needs `NAME` in `COLLECTOR_ENV`; otherwise the
value is empty and, for an endpoint, `validate` fails (with the `otlphttp` exporter: "at least one
endpoint must be specified"). The metadata query for GCP, which the exporter starts anyway, points
through `GCE_METADATA_HOST` to a closed port of `127.0.0.1`, like the proxy of all validators
([tools/opscheck](../../tools/opscheck/README.md)); the GCP metadata client honours that variable
and uses a transport of its own without a proxy
([metadata.go](https://github.com/googleapis/google-cloud-go/blob/f06f6b35e4b7ae8b48a3f9d1715229fe81cb2ebd/compute/metadata/metadata.go#L71-L86),
retrieved 2026-10-12; ruling M1-04/22). Check only the collector:

```
uv run --project tools --locked python tools/opscheck/opscheck.py --only collector collector-allowlist
```
