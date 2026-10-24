# Module `stats`

Status: skeleton (M1-05). The port package exists; interfaces and use cases come with the tickets of the milestone.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapter applies.

## Purpose

Statistics for the console and apps: usage per space, runs, tool calls, costs, error rates; own measurement (in-memory counters, flush, retention), SLO traffic light, live frame `stats.live`, timeline.

## Code

- `internal/stats` (`doc.go`): port as the root package, so far only with a package comment. Interfaces, DTOs and In-Tx events come with the ticket that implements them.
- `domain/`, `app/` and `adapters/<kind>/` are created with the first use case. `tools/archgen` generates their components from `architecture/modules.yaml`; a package outside these layers fails `make arch`.

## Port (root package `internal/stats`)

- Inbound: `Stats` (queries, timeline, traffic light, backend view), `Live` (base frame per interval, filter per receiver).
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: kernel `measure.Recorder`.

## Dependencies

May use the ports of these modules: `chat`, `agents`, `messaging`, `runs`, `billing`, `files`, `apps`, `notify`.

Reason: reads metrics through ports.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`stats_minutes`.

Only this module writes and reads these tables via SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`stats.read`, `client_metrics.submit`.

## Events and WebSocket ops

- Durable: none.
- Ephemeral: `stats.live`.
- WebSocket ops (client to server): `stats.subscribe`, `stats.unsubscribe` (through the shared WebSocket hub).

## Notes for the build

- Metrics without content; instance view only for the owner, space view with `manage` (05, `stats.read`).
- Distinction from telemetry: `stats` shows domain figures in the console, operational metrics run through OpenTelemetry (17-observability.md).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [01-product.md](../../../../docs/spec/01-product.md), [17-observability.md](../../../../docs/spec/17-observability.md)

## Client metrics (M4-31)

`submitClientMetrics` (`POST /api/v1/client-metrics`, action `client_metrics.submit`, `rl.write`)
takes the app metrics of docs/observability/signals.md 4.6a (ADR 0057):

- `clientmetrics.go` (port): `ClientMetrics` with `Submit` and the batch types.
- `domain/`: the closed list of names and kinds, the per-entry filter (unknown name, mismatched
  `kind`, bucket bounds outside 4.7, negative counts, a negative counter or a non-finite value
  discard only that entry) and the attribute normalization (`platform`, `density`, `theme`,
  `app_major` from `app_version` or `other`; `close_code` only for `client.reconnects`, `other`
  outside the codes of 04 6.4).
- `app/`: `SubmitClientMetrics` reads `privacy.client_metrics` of the person (`403
  feature_disabled` when off), filters, records and answers `202`; it attaches no principal or
  device and writes no table.
- `adapters/httpapi`: the handler of the generated interface; `adapters/otelmetrics`: the sink over
  the instruments `bothy.client.*` and `bothy.client_metrics.discarded`.

Histogram path (docs/DECISIONS.md D-M4-31-1): the buckets go to `metrics.AggregatedSet` in
`internal/platform/otel/metrics`, an `sdkmetric.Producer` on the OTLP metric reader, which exports
them as cumulative OTLP histogram data points with the bounds of 4.7. Count and bucket counts are
exact; the sum is an upper bound (each value at its bucket's upper bound), minimum and maximum are
absent. The app clamps a value above the top bound into the top bucket.
