---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team E (apps and design), Team F (operations and observability)
informed: Team C (privacy)
---

# Client telemetry through a server endpoint instead of a client OTel SDK

## Context and Problem Statement

The instance operator should see whether the apps start quickly, whether catch-up and sending work
and whether push texts appear in time. Telemetry leaves only through an OpenTelemetry Collector
and never contains content (B11, ADR 0034). There is no official OpenTelemetry SDK for Dart; a
community SDK has been donated, its status is open [Q1, verified]. The Collector runs in the
internal network of the instance. How do metrics get from the apps into the telemetry?

## Decision Drivers

- Export only through the Collector, no direct path to a vendor (B11, 17 rule 2).
- No content, no IDs of threads, channels or people; metrics without person attributes
  ([17-observability.md](../spec/17-observability.md), rules 3, 8 and 9).
- The Collector stays internal; no open ingestion endpoint on the internet.
- Every ingestion goes through `authorize` and a rate limit (B7).
- The person can turn it off (`privacy.client_metrics`).

## Considered Options

- A fixed, small list of metrics, batched to a dedicated server endpoint; the server checks
  them and passes them on to the Collector
- OpenTelemetry SDK in the app, export via OTLP directly to a reachable Collector
- Third-party mobile analytics or RUM SDK
- No metrics from the apps, only server telemetry and crash reports

## Decision Outcome

Chosen option: "Fixed metrics to a server endpoint", because it keeps B11 without opening the
Collector, and because the server can check every ingestion. Normative in
[15-client-architecture.md](../spec/15-client-architecture.md), section 17 (R-CL-10), and
[02-architecture.md](../spec/02-architecture.md), section 4.6 (module `stats`).

1. The app measures only the metrics from 15, section 17: cold start, opening a thread, sending
   until local display, frames over budget, catch-up, reconnects by close code, depth of the
   outbox, push until display. Attributes are only platform, app version, density and theme.
2. It sends at most every `client.metrics.flush_interval` to `POST /api/v1/client-metrics`
   (action `client_metrics.submit`, class `rl.write`). If `privacy.client_metrics` is off, the
   server responds `403 feature_disabled`.
3. The server accepts only names and attributes from the fixed list, discards everything else and
   attaches no principal and no device. It writes no table; the values go to the Collector as their
   own time series with fixed bounds (17, R-OB-06).
4. The apps send no spans. They set `traceparent` on every request, so that server traces and
   crash reports carry the same trace ID; the server decides the sampling (17, section 9).
5. Crash reports are a separate path (Sentry only with a configured target, without content;
   15, section 16).

### Consequences

- Good, because the Collector stays internal and apps only talk to the server, which they reach
  anyway.
- Good, because privacy applies at the source (fixed list) and in the server (allow-list) before
  the Collector filters.
- Good, because ingestions are authenticated, throttled and can be turned off.
- Bad, because there are no client traces; a slow screen shows only in the metric, not in the
  flow.
- Bad, because pre-aggregated histograms from many clients cannot be mapped directly to
  measurements of a Go SDK instrument; the build ticket must prove how they reach the Collector
  without distortion (open, see More Information).
- Bad, because the server maintains another ingestion interface with schema and list; new
  metrics need a change in the app, the server and the signal catalogue.

### Confirmation

- Build ticket M4-31 in [PLAN.md](../PLAN.md): canary test of the client paths; report without
  message texts and without URLs with parameters.
- Test: an unknown metric name or an unknown attribute is discarded and counted, never
  passed on; with `privacy.client_metrics` off, the server responds `403 feature_disabled`.
- Allow-list and cardinality test from [17-observability.md](../spec/17-observability.md),
  section 10: every metric passed on is in the signal catalogue, without IDs as attributes.
- New check in the gate, built with M4-31: no Dart package depends on an OpenTelemetry or
  analytics SDK (deny list of package names against `pubspec.lock`; `sentry_flutter` for
  crash reports is exempt).

## Pros and Cons of the Options

### Server endpoint with a fixed list

- Good, because authorization, throttling and filtering come without new infrastructure.
- Bad, because only metrics, no spans.

### OpenTelemetry SDK in the app

- Good, because standard instrumentation and spans from the app would be possible.
- Bad, because there is no official Dart SDK [Q1, verified] and the Collector would have to be
  reachable for devices on the internet and authenticated.

### Third-party analytics SDK

- Good, because of ready-made analyses.
- Bad, because data goes to third parties (an additional processor, 20) and B11 is
  bypassed.

### No client metrics

- Good, because there is nothing to build.
- Bad, because the performance targets of the apps (15, section 19) would stay invisible in
  operation and push problems would only show up through complaints.

## More Information

- Source (retrieved 2026-10-10): Q1 OpenTelemetry Blog, "Dart and Flutter",
  https://opentelemetry.io/blog/2026/dart-flutter-opentelemetry/ [verified, as cited in 15 and 17].
- Decided by build ticket M4-31 (docs/DECISIONS.md D-M4-31-1): the pre-aggregated histograms
  reach the Collector as own OTLP data points. An `AggregatedSet` in `platform/otel/metrics` is an
  `sdkmetric.Producer` attached to the OTLP metric reader; it keeps one cumulative histogram per
  name and attribute set with the bounds of signals.md 4.7 and adds the client's bucket counts
  unchanged. One measurement per bucket count was rejected: it would put every value at one point
  of its bucket through the SDK's own aggregation and could not keep the counts exact. Exact are the
  count and the count of every bucket; the sum is an upper bound (every value at its bucket's upper
  bound, the clients send no sum); minimum and maximum are absent. `TestAggregated_HistogramPath`
  sends known buckets (13 values up to 0.1 s, 3 up to 0.25 s, 2 up to 10 s) and reads count 18,
  the counts per bound and the sum 22.05 from an in-memory reader.
- Related: ADR 0021 (audit separate from telemetry), ADR 0027 (Flutter), ADR 0034 (Collector).
