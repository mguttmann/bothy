# M1-68: Server core metrics per catalogue

Status: ready | Depends on: M1-04, M1-50, M1-66 | Milestone: M1

## Goal

After this ticket the server core emits the metrics of `docs/observability/signals.md` for REST,
authorization, rate limits, the gateway, catch-up, delivery, fan-out lag, the `event_seq` lock,
jobs and the database pool, with the fixed bucket bounds of the catalogue and only closed
attribute sets. A Go test fails on any instrument that differs from the catalogue or carries an
open or person attribute, and a new gate `make signal-check` fails when a dashboard, alert rule or
SLO uses a metric or label outside the catalogue or an SLO threshold that is not a bucket bound.
Without this the dashboards, alerts and SLOs shipped in `infra/observability/` measure nothing.

## Read first

- `docs/PLAN.md`: section "Way of working per ticket"; section "M1 Foundation", rows M1-04, M1-10,
  M1-14, M1-16, M1-18, M1-19, M1-50, M1-65, M1-66, M1-68, M1-69, M7-09.
- `docs/spec/17-observability.md` (all sections, especially "Basic rules" 3, 4, 8, 9,
  "Tests that hold these rules" items 2 to 4, R-OB-06).
- `docs/observability/signals.md` (all sections; tables 4.1, 4.2, 4.4, 4.5, 4.6, 4.7).
- `docs/observability/slos.md`, `docs/observability/alerting.md`, `docs/observability/dashboards.md`,
  `docs/observability/privacy.md` "No monitoring of individual people".
- `infra/observability/slo/*.yaml`, `infra/observability/prometheus/rules/*.yaml`,
  `infra/observability/grafana/dashboards/*.json`, `infra/otel/collector.yaml`
  (`transform/scrub_metrics`).
- `docs/spec/02-architecture.md`: "Layers within a module" (app layer imports no third-party
  library and no `platform`), "Gateway", "Event path", "Background jobs (River)", "Deadlines",
  "Code layout".
- `docs/spec/04-sync-protocol.md`: "Close codes", "`resync_required`"; `docs/spec/05-actions.md`:
  "Rate-limit classes", "Decision"; `docs/spec/03-data-model.md` table `messages` (`origin`,
  `kind` checks).
- ADRs: `docs/adr/0034-*`, `0035-*`, `0008-*`.
- Code you build on: `internal/platform/otel` (M1-10, meter provider, `bothy.telemetry.dropped`),
  `internal/platform/httpserver` (M1-18), `internal/platform/ratelimit` (M1-19),
  `internal/platform/jobs` (M1-14), `internal/platform/pg` (M1-12), `internal/sync` (M1-16),
  `internal/authz` (M1-28), `internal/messaging` (M1-56), `internal/gateway` (M1-64, M1-65,
  M1-66), `internal/audit` (M1-50), `api/routes` and `api/operations.yaml` (M1-17).
- `tools/README.md` (rules for new tools), `tools/pyproject.toml`, `Makefile`,
  `docs/quality/gates.md`, `docs/quality/toolchain.md`, `CONTRIBUTING.md`.

## Fixed requirements that apply

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour change has a test that was red before the change (record the red
   run in the report), then green, then clean up.
7. No en dash or em dash (U+2013, U+2014) in any file you write.
8. White label: only the examples Alex, Sam, Robin, `example.com` and the spaces Support, Marketing,
   Engineering, Research; no paths under `/Users`.
9. Limits only through named keys from `docs/operations/configuration.md`.
10. The catalogue is the only list of names, units and attributes; what is not in it is not sent
    (signals.md, header; 17 "Signals").
11. Metric attributes come only from closed sets; never IDs, user input, error texts or raw paths;
    `http.route` is the template or `unmatched` (signals.md "Cardinality"; 17 rule 9).
12. Metrics never carry attributes that identify a person
    (17 rule 8; privacy.md "No monitoring of individual people").
13. Histograms use the fixed bounds of signals.md 4.7, and every SLO threshold is one of these
    bounds (R-OB-06).
14. Telemetry never blocks or changes the result of a request (17 rule 4).
15. Tests named in 17 "Tests that hold these rules" items 3 and 4 (cardinality, names).

## Scope

Go (module `services/server`):

- `internal/platform/otel/metrics/`: the single Go declaration of every instrument the server
  registers (`Spec`: name, kind, unit, description, attributes with closed value sets, buckets),
  `Check(specs) []error` for the structural rules, SDK views (explicit bucket bounds per
  histogram, attribute allow-filters per instrument, including the ones created by otelhttp and
  otelpgx), and typed recording wrappers. Unknown attribute values map to `other` in production
  and fail the test in test builds (a strict switch the test helpers turn on).
- Recording, in the layer where the fact is known (02 "Layers within a module", rule 5): app-layer code records
  through a small outbound port in `internal/<module>/app/metrics.go` with a no-op default for
  unit tests; the implementation lives in `internal/<module>/adapters/otelmetrics/metrics.go`;
  platform packages record in `internal/platform/<pkg>/metrics.go`. Wire everything in
  `internal/wire`. Register new adapter packages in `architecture/modules.yaml` so `make arch`
  stays green.
- Instruments (catalogue names; sets per the catalogue unless stated):

| Instrument | Recorded in | Rule |
|---|---|---|
| `http.server.request.duration` | `platform/httpserver` (otelhttp) | attributes exactly `http.request.method`, `http.route`, `http.response.status_code`, `error.type`; `http.route` = `/api/v1` plus the path template of each enabled operation, `/api/v1/gateway`, or `unmatched`; drop every other key with a view (otelhttp adds keys such as `server.address` whose values come from the client) |
| `bothy.authz.decisions` | `authz` | requests and ops only; checks in filter mode (`authorize.Filter(ctx)`, implied by the gateway's `Delivery(ctx)`) are not counted (signals.md) |
| `bothy.ratelimit.rejected` | `platform/ratelimit` | class from the generated catalogue |
| `bothy.gateway.connections.active` | `gateway` | +1 at `ready`/`resumed`, -1 when such a connection closes; kinds `user`, `agent`, `app` |
| `bothy.gateway.connect.duration` | `gateway` | upgrade to `ready`/`resumed`, or to the close before it; `client_error` and `server_error` exactly as signals.md 4.1 defines them; kind `anonymous` before identification |
| `bothy.gateway.catchup.duration` | `gateway` | `ready`/`resumed` to `catchup.complete` (`complete`) or `resync_required` |
| `bothy.gateway.resync` | `gateway` | reasons of 04 "`resync_required`" |
| `bothy.gateway.close` | `gateway` | every close in both directions; codes of 04 "Close codes" as strings, any other code `other` |
| `bothy.gateway.outbound.dropped` | `gateway` | `slow_consumer`: number of frames discarded at a `4014` close, live buffer included |
| `bothy.message.created` | `messaging` | after commit only; `origin` and `kind` from the `messages` checks in 03 |
| `bothy.event.appended` | `sync` phase hook | after commit, per durable type |
| `bothy.db.event_seq.lock_wait` | `sync` phase hook | duration of the `UPDATE sequence_counters` statement for `event_seq` |
| `bothy.event.fanout.lag` | `gateway` (fed by the tail) | 0 when the hub has distributed every event the tail knows of; otherwise now minus the time the oldest undistributed event became known (NOTIFY or poll) |
| `bothy.delivery.duration` | `gateway` writer | `occurred_at` to successful write, phase `live` only; durable and ephemeral path (M1-66) |
| `bothy.job.duration` | `platform/jobs` | kinds from the job registry; outcome `completed`, `failed`, `cancelled`, `snoozed` |
| `bothy.job.queue` | `platform/jobs`, role `worker` | observable gauge: jobs per kind in states `available`, `scheduled`, `retryable` |
| `db.client.operation.duration` | `platform/pg` | from otelpgx if its installed version emits it with closed `db.operation.name` and `db.collection.name`; otherwise an own pgx query tracer in `platform/pg` |
| `bothy.db.pool.connections` | `platform/pg` | observable gauge from the pool statistics: `idle`, `acquired`, `max` |
| `bothy.telemetry.dropped` | `platform/otel` | already from M1-10; move its declaration into the registry |
| `bothy.audit.verify.failures`, `bothy.audit.verify.last_success.timestamp` | `audit` (job `audit.chain_verify`) | recorded by M1-50; this ticket only moves their declaration into the registry |

Python gate:

- `tools/signalcheck/signalcheck.py` with tests in `tools/signalcheck/tests/test_signalcheck.py`
  and fixtures under `tools/signalcheck/tests/fixtures/`; standard library plus PyYAML only;
  `SPDX-License-Identifier: Apache-2.0` in every file.
- `Makefile`: target `signal-check` (`$(UVRUN) python tools/signalcheck/signalcheck.py`) added to
  `gates`; add `tools/signalcheck` to the bandit directory list of `lint`.
- `tools/pyproject.toml`: add `signalcheck` to `testpaths`, `pythonpath`, coverage `source` and
  mypy `files`.
- Rows in `tools/README.md`, `docs/quality/toolchain.md` and `docs/quality/gates.md`.

Docs:

- `docs/observability/signals.md` already carries the definition of `bothy.event.fanout.lag`, the
  `client_error` list, `bothy.authz.decisions` without filters, the view rule for library
  instruments (2, item 5), the outcome values of `bothy.job.duration` and the GenAI package path.
  Change only the Source column of `db.client.operation.duration` (verified otelpgx version) and
  what the build proves wrong. Move the declarations of
  `bothy.authz.errors` (M1-18, M1-25), `bothy.egress.blocked` (M1-22) and
  `bothy.secrets.refresh.failures` (M1-09) into the registry like `bothy.telemetry.dropped`.
- Module READMEs of every module that records metrics (English, like the existing files),
  `services/server/CHANGELOG.md` under "Unreleased", `docs/DECISIONS.md` (only decisions beyond the specification).

## Out of scope

- Run, agent, push, file, search and backup metrics (M2, M3, M7); their catalogue rows stay
  planned and are not registered.
- The canary test across all outputs (M1-69); load tests (M1-72).
- Collector allowlist against the catalogue, SLO table against the Sloth files, alert thresholds
  against configuration values: M1-04 (`ops-check`). Do not duplicate them.
- New dashboards, alerts or SLOs; changing SLO objectives.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-68-core-metrics`.
- Conventional Commits with scope (`feat(gateway): ...`, `feat(platform): ...`,
  `build(tools): ...`) and DCO sign-off.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except the docs and changelog entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp`, removed at the
  end. Tests bind `127.0.0.1` and port 0 only; PostgreSQL through `pgtest`. Tests read metrics with
  an in-process SDK reader, never from a running collector.
- Never read or print secrets. No network calls except package downloads for pinned tools.

## Design notes

Go registry and checks

- Attribute value sets are either literal lists or functions over generated, closed sources
  (action ids, rate classes, event types, route templates from `api/routes`, close codes, job
  kinds registered at startup, table names from the module register). `other` is implicitly part
  of every set. A set without a source is an open attribute and fails `Check`.
- Person and ID attributes fail `Check`: keys ending in `.id` or `_id`, `bothy.principal.pseudo_id`,
  `bothy.correlation_id`, `bothy.event.seq`, `user.*`, `enduser.*`, `client.address`,
  `session.id`.
- Bucket rows (signals.md 4.7): durations up to seconds, long durations, sizes. Every histogram
  gets the row the catalogue names; the view sets `ExplicitBucketBoundaries` so third-party
  histograms (otelhttp, otelpgx) follow it too.
- Recording never blocks: synchronous instruments only add; observable gauges read cheap state
  (pool statistics, hub counters) except `bothy.job.queue`, whose callback runs one grouped count
  query in `platform/jobs` (the owner of the River tables) with `database.statement_timeout`.
- Catalogue test: the Go test reads `docs/observability/signals.md` (found by walking up to the
  directory that contains it), parses tables 4.1 to 4.7 and compares every registered `Spec`:
  name present, kind (Histogram, Counter, UpDownCounter, Gauge), unit, attribute keys, listed
  values where the catalogue lists them, and bucket bounds.

`signalcheck` (Python)

1. Catalogue integrity: every row of 4.1 to 4.6 parses; the Prometheus column equals the
   translation rule of signals.md "Names and units" item 4 (dots to `_`, unit suffix `s` ->
   `_seconds`, `By` -> `_bytes`, counters `_total`, units in braces dropped); each histogram has
   exactly one row in 4.7 and 4.7 names only histograms; no attribute column names a person or ID
   attribute (deny list as in Go).
2. Names: every metric identifier in the `expr` fields of the dashboards (recursively), in `expr`
   of the rule files and in `error_query`/`total_query` of the Sloth files is a catalogue
   Prometheus name (histograms also with `_bucket`, `_count`, `_sum`), a recording rule defined
   by `record:` in the rule files, or `up`, `ALERTS`, `ALERTS_FOR_STATE`.
3. Labels: inside the selector braces of a catalogue metric, every label name is an attribute of
   that metric (translated), a resource attribute of signals.md "Resource attributes"
   (translated), `job`, `instance`, or `le` on `_bucket` series.
4. Thresholds: every `le` matcher on a `_bucket` series in the Sloth files, rule files and
   dashboards is a bound of that histogram's 4.7 row (numeric comparison) or `+Inf`.
5. PromQL lexing without a PromQL library: replace `{{.window}}` and Grafana variables (`$name`,
   `${name}`, `$__rate_interval`) by a duration; skip string literals except as label values;
   identifiers followed by `(` are functions or aggregations; identifiers inside `by (...)`,
   `without (...)`, `on (...)`, `ignoring (...)`, `group_left (...)`, `group_right (...)` are
   labels; skip keywords (`and`, `or`, `unless`, `bool`, `offset`, `inf`, `nan`), numbers and
   durations; the rest are metric names.
6. PromQL syntax of the dashboards: every dashboard `expr` (after the replacements of rule 5)
   becomes the `expr` of a recording rule in a temporary rules file under `tmp_path` or
   `mktemp -d`, checked with the pinned `promtool check rules` from `tools/bin/tools.lock.json`
   (M1-04); a parse error is a finding with the dashboard file and panel. Reason: rule 5 only
   lexes names, and a broken query would show an empty panel without any gate noticing.
7. Exit code 0 with a one-line summary, 1 with one line per finding (file, location, rule).

Fixes to existing files: if the real dashboards, rules or Sloth files fail a rule, fix the file
or, when the file is right, the catalogue, in this change, each with a line in the report.

No rulings of its own: metric ports (02 "Layers within a module" rule 5), the fan-out lag, the
`client_error` set, the scope of `bothy.authz.decisions`, the views for library instruments and the
job outcomes are in the specification. Record in `docs/DECISIONS.md` only what the build had to
decide beyond them.

## Tests (write them first)

Go (`internal/platform/otel/metrics` and the recording packages, SDK `ManualReader`):

- `TestCatalogue_RegistryMatchesSignalsDoc`, `TestCatalogue_M1InstrumentSet` (exactly the
  instruments of the table above that this build records).
- `TestCheck_OpenAttributeFails`, `TestCheck_PersonAttributeFails`,
  `TestCheck_HistogramWithoutBucketsFails` (fixture specs).
- `TestViews_BucketBounds` for every histogram including `http.server.request.duration`.
- `TestHTTPMetric_RouteTemplateAndUnmatched`, `TestHTTPMetric_OnlyAllowedAttributes` (a request
  with an unusual `Host` header leaves no `server.address`).
- `TestRecord_UnknownValueStrictFailsInTests`, `TestRecord_UnknownValueMapsToOther`.
- `TestTelemetry_ExportFailureDoesNotAffectRequest` (exporter that errors; request result
  unchanged).
- Gateway: `TestGatewayMetrics_ConnectOutcomes` (ok, client_error for `4016`, server_error),
  `TestGatewayMetrics_CloseCodeForeignCodeIsOther`, `TestGatewayMetrics_CatchupAndResync`,
  `TestGatewayMetrics_SlowConsumerDroppedFrames`, `TestGatewayMetrics_ConnectionsActiveUpDown`,
  `TestGatewayMetrics_DeliveryDurationLiveOnly`.
- `TestFanoutLag_ZeroWhenIdle`, `TestFanoutLag_GrowsWhenHubStuck` (test clock).
- `TestSyncMetrics_EventAppendedAfterCommitOnly` (rolled-back transaction counts nothing),
  `TestSyncMetrics_LockWaitRecorded`.
- `TestMessagingMetrics_CreatedAfterCommit`, `TestAuthzMetrics_DeliveryChecksNotCounted`,
  `TestRateLimitMetrics_Rejected`, `TestJobsMetrics_DurationAndQueue`,
  `TestPoolMetrics_States`, `TestDBClientMetric_ClosedAttributes`.

Python (`tools/signalcheck/tests/test_signalcheck.py`):

- the real repository passes;
- fixtures that must fail: a dashboard query with a PromQL syntax error, unknown metric in a dashboard, unknown label on a catalogue metric,
  Sloth `le` that is not a bound, Prometheus column that breaks the translation rule, histogram
  without a 4.7 row, person attribute in the catalogue, undefined recording rule;
- fixtures that must pass: recording rules, `{{.window}}`, Grafana variables, `by (...)` lists,
  `histogram_quantile` over `_bucket`.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/... ./api/...
make signal-check
make test
make check
```

## Acceptance criteria

- [ ] Every instrument in the table above is registered from one Go declaration and matches
      signals.md in name, kind, unit, attributes and buckets.
- [ ] The cardinality test fails on an open attribute and on a person or ID attribute (fixture
      tests red first).
- [ ] Exported data points carry only catalogue keys and values; `http.route` is a template or
      `unmatched`.
- [ ] Every metric and label used in dashboards, rules and SLOs is in the catalogue; every SLO
      threshold is a bucket bound (`make signal-check` green, fixtures red).
- [ ] `bothy.event.fanout.lag` is 0 when idle and grows when the hub is stuck.
- [ ] No metric carries a person attribute; telemetry failure never changes a response.
- [ ] signals.md, module READMEs, tools docs and gates docs updated; `make check` green.

## Definition of done

- Code and tests as above; doc comment on every public symbol (purpose, parameters, return value,
  errors); comments explain why.
- signals.md corrected as listed; READMEs, `tools/README.md`, `docs/quality/toolchain.md`,
  `docs/quality/gates.md`, `services/server/CHANGELOG.md` under "Unreleased"; rulings in
  `docs/DECISIONS.md`. Write an ADR only if you take an architecture decision beyond this brief.
- `make check` green.
- Independent review (`make review` plus a code review over the diff) and the red team gate
  (`docs/quality/red-team-gate.md`) over the diff, verdict recorded in the report.
- Merge locally into the base branch only if review and red team are green and `make check` is
  green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then
commits, files with line counts, test results with counts (including the red runs), the verified
otelpgx behaviour, every fix to dashboards, rules or the catalogue, what is not proven, and
follow-ups that could not be done in scope with the reason. Then write `{{DONE_PATH}}` with
`exit=0 sha=<sha>` or `exit=1 <reason>`.
