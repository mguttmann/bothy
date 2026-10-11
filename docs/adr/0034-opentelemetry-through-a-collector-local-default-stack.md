---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team F (operations)
consulted: Team A (architecture), Team C (security), Team D (agent platform)
informed: all teams
---

# OpenTelemetry through a collector, local default stack

## Context and Problem Statement

B11 specifies OpenTelemetry and requires export only through a collector, without content.
Instance operators use different backends (Grafana, New Relic, Datadog, Azure Monitor, Sentry) or
none. A single instance on a Mac should still have dashboards and alerts right away. How does
telemetry flow, and what is the default?

## Decision Drivers

* Backend change without code
* Privacy enforced independently of the backend (no content, pseudonyms)
* Telemetry never blocks the message path
* a single instance gets dashboards, SLOs and alerts without an external account
* several backends at the same time are possible (for example metrics in Grafana, traces also in
  Sentry)

## Considered Options

* SDKs export to a collector; backends as overlays of the collector; local default stack of
  Prometheus, Loki, Tempo, Grafana and Alertmanager as a Compose profile
* SDKs export directly to the respective backend
* Classic without OpenTelemetry: Prometheus scrape of `/metrics` and structured logs

## Decision Outcome

Chosen option: "Collector with overlays and local default stack", because only the collector
offers one place where privacy filters, sampling and temporality apply equally to every
backend.

1. Base [infra/otel/collector.yaml](../../infra/otel/collector.yaml): OTLP receiving,
   `memory_limiter`, content filter, allowlist with HMAC pseudonyms, tail sampling, `batch`,
   distribution through `forward` connectors.
2. Overlays under `infra/otel/exporters/` attach their own pipelines; two can be combined.
3. Local default: Compose profile `observability` and overlay `local`. Metrics go via OTLP to
   Prometheus (cumulative), logs to Loki, traces to Tempo.
4. The status page and the in-app statistics do not depend on the stack.

### Consequences

* Good, because a backend change is one line in `.env` and one secret file.
* Good, because privacy is checked once (canary test against the real collector).
* Good, because the server knows only one export path (OTLP to the collector).
* Bad, because the collector is another service whose failure costs telemetry (not
  operations).
* Bad, because tail sampling needs all spans of a trace in the same collector; several
  collectors need load balancing by trace ID.
* Neutral, because the local stack serves one instance and has no high availability.

### Confirmation

* `otelcol-contrib validate` of the base alone and with each overlay in the gate (on 2026-10-10
  locally green with 0.162.0, also three pairs).
* Canary test through the real collector (on 2026-10-10 locally: no content in the export,
  email and bearer value as HMAC).
* Naming test: metrics in dashboards, rules and SLOs are in the catalogue
  ([signals.md](../observability/signals.md)).
* Grafana provisioning and `promtool check` in the portability test.

## Pros and Cons of the Options

### Collector with overlays

* Good, because filters, sampling and temporality are in one place.
* Good, because vendor exporters (Datadog, Azure) are needed only in the collector.
* Bad, because lists are replaced on merging; overlays must use their own pipelines
  (solved with `forward` connectors).

### Direct export from the SDKs

* Good, because there is one service less.
* Bad, because every process would need credentials of every backend.
* Bad, because privacy filters would have to be built the same way in every language (Go,
  Python, TypeScript).

### Classic without OpenTelemetry

* Good, because a Prometheus scrape is simple and widespread.
* Bad, because traces across app, server, adapter and runtime are missing.
* Bad, because B11 requires OpenTelemetry.

## More Information

* Rules: [17-observability.md](../spec/17-observability.md); backends:
  [backends.md](../observability/backends.md); collector: [collector.md](../observability/collector.md).
* Prometheus as an OTLP target: https://prometheus.io/docs/guides/opentelemetry/ (retrieved
  2026-10-10).
* Grafana stack instead of alternatives: Prometheus, Loki, Tempo and Grafana accept OTLP directly
  and are available as images for amd64 and arm64 (Docker Hub query on 2026-10-10). Other
  complete stacks were not compared; a switch would only be a new overlay.
