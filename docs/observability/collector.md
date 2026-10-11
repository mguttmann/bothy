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

```
otelcol-contrib validate --config=infra/otel/collector.yaml --config=infra/otel/exporters/local.yaml
```

For the check outside the container, the paths under `/run/bothy/secrets` must exist
or point to test files; the key `telemetry_pseudonym_key` must be at least 32 bytes long.
