# Module `stats`

Status: client metrics (M4-31), own measurement (M6-13), statistics queries (M6-05) and the live
stream `stats.live` with `getStatsLive` (M6-15) built; the timeline and the traffic light come
with M6-14.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapter applies.

## Purpose

Statistics for the console and apps: usage per space, runs, tool calls, costs, error rates; own measurement (in-memory counters, flush, retention), SLO traffic light, live frame `stats.live`, timeline.

## Code

- `internal/stats` (`doc.go`): port as the root package: `ClientMetrics` (M4-31), `Statistics`
  (M6-05), `Live` with its DTOs and `Register` (M6-15).
- `domain/`, `app/` and `adapters/<kind>/` are created with the first use case. `tools/archgen` generates their components from `architecture/modules.yaml`; a package outside these layers fails `make arch`.

## Port (root package `internal/stats`)

- Inbound: `Stats` (queries, timeline, traffic light, backend view), `Live` (base frame per interval, filter per receiver), `SLIWindows` (M7-09: the sums of one `sli.*` metric per dimension over a window of minute buckets, the query `SLITotals` of `adapters/postgres`, for the checks of the status page).
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

## Own measurement (M6-13)

The server measures what level a of the visualizations needs itself
([25-visualization.md](../../../../docs/spec/25-visualization.md) section 3, rulings
`D-M6-13-1` to `D-M6-13-8` in [docs/decisions.d/M6-13.md](../../../../docs/decisions.d/M6-13.md)):

- Port `internal/kernel/measure`: `Recorder.Record(metric, key, value)`, the 18 metrics of 25 3.2
  with their closed dimension lists and scopes, `Validate`, and the values `One`, `Timed`,
  `Amount`, `Outcome` and `SLI`. Every measuring point calls it next to its OpenTelemetry
  instrument, domain counters after the commit of their unit of work.
- `domain/`: `IsGood` (an SLI event is good up to and including the threshold of
  `slibounds.gen.go`), the counters of one event, minute and day truncation in UTC, the longest
  SLO period and the retention cutoff.
- `app/`: `Measurement` (counters per minute and key in memory under one mutex, at most
  `stats.flush_queue_max` keys, the oldest minute dropped first), `Flush` (every
  `stats.flush_interval` and once at the shutdown, chunks of one additive upsert each, merge back
  on an error, missing day partitions created on demand) and `Retention` (the use case of
  `stats.retention`). Loss is counted in `bothy.stats.flush.dropped` with `queue_full`,
  `db_error` or `invalid_key`; each flush in `bothy.stats.flush.duration`.
- `adapters/postgres`: the sqlc queries of `stats_minutes` (upsert, partition functions);
  `adapters/jobs`: `stats.retention` on the queue `maintenance`, hourly and at the start;
  `adapters/otelmetrics`: the two flush instruments.
- `internal/wire`: the start step `stats_measure` builds it in every role before any module and
  registers the stop step `stats_flush`; `bothy config check` refuses a `retention.stats_minutes`
  shorter than the longest SLO period.

`stats_minutes` (migration `00075`) is partitioned per UTC day (`stats_minutes_YYYYMMDD`);
`bothy_app` may select, insert and update it, never delete; partitions are created and dropped only
through `stats_minutes_ensure_partition` and `stats_minutes_drop_expired`. The table holds no
person and no content: a key is minute, metric, channel, agent, dimension code and currency.

## Statistics queries (M6-05)

`stats.Statistics` (`statistics.go`) is the port of `getUsageStats`, `getRunStats`,
`getToolCallStats`, `getErrorStats` and `getCostStats`; `app.Statistics` (`app/queries.go`)
implements it and `adapters/httpapi/statistics.go` serves it. Each use case authorizes `stats.read`
(`getCostStats`: `cost.read.aggregate`) on the instance or the space, builds the channel set from
`Scope(message.read)` and `chat.StatsSource`, calls the `StatsSource` ports of `messaging`,
`chat`, `runs`, `billing`, `files`, `apps` and `notify`, merges their aggregates into buckets
(`domain/statistics.go`: bucket arithmetic in `stats.time_zone`, range limits, minimum group,
merge with the defence in depth) and caches the result (`app/cache.go`, key with the rights
version and the channel set, `stats.cache_ttl`, `stats.cache_max_entries`). Values without a
channel (webhook, push, scans) are read only for an owner on the instance; no figure is grouped
by person, and `active_people` below `privacy.stats_min_group_size` is withheld and named in
`below_min_group`. The wiring is `wire/module_stats.go`, step `statistics` in the roles `api` and
`all`. Rulings: `docs/decisions.d/M6-05.md`.

## Live stream (M6-15)

`stats.live` and `getStatsLive` ([25-visualization.md](../../../../docs/spec/25-visualization.md)
section 5, rulings `D-M6-15-1` to `D-M6-15-8` in
[docs/decisions.d/M6-15.md](../../../../docs/decisions.d/M6-15.md)):

- Port (`live.go`): `Live` (`Check`, `Acquire`, `Release`, `Current`, `BindSink`), `LiveQuery`,
  `LiveFrame`, `LiveRound` (one base frame with `For`, the filter per receiver), `LiveSink` (the
  gateway), `LiveConnectionCounter` (the hub) and `Register` (the event `stats.live` and the ops
  `stats.subscribe`, `stats.unsubscribe`).
- `domain/live.go`: the base frame (runs per channel, agents, channel places, rate cells per
  channel and without a channel, SLI cells, connections), `FilterLive` (channel set and space,
  costs, instance-only members, `stats.live_max_runs`) and the size cut below
  `sync.event_payload_max_bytes` with `runs_truncated`.
- `app/live.go`: `Live`, the builder of the node: `Check` (`stats.live_enabled`, `stats.read`,
  people only), slots up to `stats.live_max_subscribers` with the gauge
  `bothy.stats.live.subscribers`, `Run` (a ticker of `stats.live_interval` only while a
  subscription or a poll lease exists), `Tick` (one build, handed to the sink), `Current` (the
  base frame of the current interval, built once in a single flight on an idle node, filtered for
  the caller). Sources: `runs.LiveSource`, `chat.ChannelPlaces`, `agents.Facts`,
  `agents.Presence` and `stats_minutes` (last full and current minute, the SLI window).
- `adapters/postgres`: the queries `LiveRates` and `LiveSLI`; `adapters/httpapi/live.go`:
  `getStatsLive`, which writes the filtered frame's bytes as its body; `adapters/otelmetrics/live.go`:
  the gauge.
- `internal/wire`: the stats step builds `Live` in the roles `api` and `all`, runs its loop with
  the stop step `stats_live` and the test seam `Deps.StatsLive`; the gateway mount binds the
  gateway as sink and the hub as connection source.

The frame carries IDs, states, billing modes, step kind and name, the approval flag and costs only:
no person, no triggering person, no message, title or step detail (SEC-VZ-3, VZ-T07).
