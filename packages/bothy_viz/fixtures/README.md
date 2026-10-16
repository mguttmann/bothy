# Fixtures of `bothy_viz`

JSON fixtures per schema and state for the views of
[25-visualisierung.md](../../../docs/spec/25-visualisierung.md) (section 7.2). Tests, the
harness and the fake server read them instead of a server.

- Name: `<schema>.<state>.json`, or `<schema>.<query>.<state>.json` when one schema serves
  several queries. The part before the first dot is the name of the schema in
  [openapi.yaml](../../../docs/api/openapi.yaml).
- States: `empty`, `normal`, `load`, `truncated`, `no_cost_permission`, `offline`, where they
  apply. `StatsLiveFrame.load.json` has exactly the default of `stats.live_max_runs` runs;
  `StatsLiveFrame.truncated.json` has `runs_truncated = true`; `no_cost_permission` lacks every
  cost member, as the server filters them without `cost.read.aggregate`.
- Content: no message text and no titles; only IDs, tool names, numbers and the test world of
  07 10. Names of agents and spaces are not part of the schemas; the harness maps the IDs to
  Support, Engineering, support-agent and dev-runner.

Checks: `make api-checks` (rule `viz-fixtures`, `tools/api-checks/viz_fixtures.py`) validates
every file against its schema with its references and fails on an unknown schema, an invalid
file, a file missing from the table below and a listed schema without a file. The Dart test
`test/fixtures_decode_test.dart` decodes every file with the generated serializers of
`api_client`.

## Fixtures

| Fixture | Schema | State | Read by |
|---|---|---|---|
| `StatsLiveFrame.normal.json` | `StatsLiveFrame` | normal | live topology, delivery, backend (live part), lane and run card marks |
| `StatsLiveFrame.empty.json` | `StatsLiveFrame` | empty | live topology, delivery |
| `StatsLiveFrame.load.json` | `StatsLiveFrame` | load | live topology (performance), harness |
| `StatsLiveFrame.truncated.json` | `StatsLiveFrame` | truncated | live topology ("more runs than shown") |
| `StatsLiveFrame.no_cost_permission.json` | `StatsLiveFrame` | no_cost_permission | live topology, run card mark |
| `StatsLiveFrame.offline.json` | `StatsLiveFrame` | offline | live topology, backend (presence offline, space scope) |
| `StatsTimeline.normal.json` | `StatsTimeline` | normal (with the delivery metrics) | timeline, delivery history, area and bar marks |
| `StatsTimeline.empty.json` | `StatsTimeline` | empty | timeline |
| `StatsTimeline.load.json` | `StatsTimeline` | load | timeline (a full day) |
| `StatsTimeline.truncated.json` | `StatsTimeline` | truncated | timeline |
| `StatsTimeline.day_boundary.json` | `StatsTimeline` | day_boundary (a period over midnight) | timeline |
| `StatsTimeline.with_costs.json` | `StatsTimeline` | with_costs (`cost` buckets, as with `cost.read.aggregate`) | timeline (no cost series) |
| `StatsSloReport.normal.json` | `StatsSloReport` | normal (all four states) | executive page, traffic light mark |
| `StatsSloReport.empty.json` | `StatsSloReport` | empty | executive page |
| `StatsBackendReport.normal.json` | `StatsBackendReport` | normal | backend view |
| `StatsBackendReport.no_cost_permission.json` | `StatsBackendReport` | no_cost_permission | backend view |
| `StatsBackendReport.empty.json` | `StatsBackendReport` | empty | backend view |
| `StatsBackendReport.rate_limits.json` | `StatsBackendReport` | rate_limits (every `rate_limit` state, I2 and I3 denials, a mismatch) | backend view |
| `StatsSeries.usage.normal.json` | `StatsSeries` | normal (`getUsageStats`) | executive page, metric tile mark |
| `StatsSeries.runs.normal.json` | `StatsSeries` | normal (`getRunStats`) | executive page |
| `StatsSeries.errors.normal.json` | `StatsSeries` | normal (`getErrorStats`) | executive page |
| `StatsSeries.empty.json` | `StatsSeries` | empty | executive page |
| `CostSeries.normal.json` | `CostSeries` | normal | executive page, cost tiles |
| `CostSeries.empty.json` | `CostSeries` | empty | executive page |
