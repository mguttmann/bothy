# M4-31: Client metrics and crash reports (ADR 0057)

Status: ready | Depends on: M4-18, M1-69 | Milestone: M4

## Goal

After this ticket the apps report the fixed client metrics of 15 17 without content, and the server accepts them: in the server module `stats` the operation `submitClientMetrics` (action `client_metrics.submit`, `rl.write`) checks `privacy.client_metrics` of the person (`403 feature_disabled` when off), discards one by one every entry with a name outside the closed list of `docs/observability/signals.md` 4.6a or a `kind` that does not match its type, accepts the rest with `202`, normalizes the attributes (`bothy.client.platform`, `density`, `theme`, `app_major`; the close code for reconnects), checks histogram bounds against the catalogue, and passes the values to the OpenTelemetry pipeline of M1-10 as `bothy.client.*` with a proven path for pre-aggregated histograms (the open point of ADR 0057); the canary test of M1-69 covers the new operation. In the desktop app a `ClientMetrics` collector measures cold start, thread open, send until local display, frames and frames over budget, catch-up duration and events, resyncs, reconnects by close code, outbox depth, batches them and sends at most every `client.metrics.flush_interval` while `privacy.client_metrics` is on. Crash reports use `sentry_flutter` only when the build configuration names a target, with every content scrubbed (`sendDefaultPii` off, `beforeSend` and `beforeBreadcrumb` remove texts, file names, URLs with parameters or IDs, titles; no screenshots, no view hierarchy, no session replay). A gate check fails when a Dart package depends on an OpenTelemetry or analytics SDK. Why: the performance targets of the apps must be visible in operation (15 19) without a single message text, name or ID leaving a device (B11, 17, ADR 0057).

## Read first

- `docs/PLAN.md`: section 3; section 7 rows M1-10, M1-61, M1-68, M1-69 (what exists on the server); section 10 rows M4-18, M4-31, M4-34.
- `docs/spec/15-client-architecture.md`: 16 "Crash reports", 17 "Telemetry in the client", 19 (the targets the metrics serve), 21 (R-CL-10), 22 (`client.metrics.flush_interval`).
- `docs/observability/signals.md`: 4.6a "App metrics" (names, types, units, attributes, server names), 4.7 (histogram boundaries "Durations up to seconds"); the rules of the catalogue the signal check of M1-68 enforces.
- `docs/spec/17-observability.md`: rules 2, 3, 8 and 9 (export only through the Collector, no content, no person attributes), 10 (allow-list and cardinality test).
- `docs/adr/0057-client-telemetry-through-a-server-endpoint.md` (decision, confirmation, the open histogram path).
- `docs/spec/01-product.md` 11 (`privacy.client_metrics`, `privacy.crash_reports`); `docs/spec/20-privacy.md` (telemetry and crash reports as processing); `docs/spec/19-security.md` (logging rules).
- `docs/spec/02-architecture.md` 4.6 (module `stats`) and the module rules of section 4 (ports, `wire`, registration).
- `docs/spec/05-actions.md` (`client_metrics.submit`: human, no step-up, no audit) and `docs/spec/06-api.md` 5 (`feature_disabled`).
- `docs/api/openapi.yaml`: `submitClientMetrics` (`x-idempotency-exempt: telemetry`), `ClientMetricsBatch`, `ClientMetric`, the bucket schema.
- `docs/DECISIONS.md`: D-W-100, D-W-114 (this ticket depends on M1-69); section 10.
- Briefs: `docs/briefs/M1-10-telemetry-foundation.md` (meter provider, exporter to the Collector), `docs/briefs/M1-17-api-server-stubs.md` (generated handler interface), `docs/briefs/M1-61-sync-personal-data.md` (reading a person's setting), `docs/briefs/M1-68-core-metrics.md` (instrument registration, the catalogue test, `make signal-check`), `docs/briefs/M1-69-canary-test.md` (the scenario every new operation joins), `docs/briefs/M4-18-desktop-shell.md` (bootstrap, build defines), `docs/briefs/M4-13-sync-connection.md`, `docs/briefs/M4-14-sync-catch-up.md` and `docs/briefs/M4-15-sync-outbox.md` (where the measurements come from).
- Code on the base branch: `services/server/internal/stats/`, `services/server/internal/platform/otel/`, the generated server interface, the canary test package `services/server/test/canary`.

This ticket adds the server implementation of the existing operation `submitClientMetrics`; it touches no table, no event and no configuration key, and the action `client_metrics.submit` already exists in the catalogue.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

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

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. Limits only through named keys: `client.metrics.flush_interval` from `AppConfig`; the batch size limit from the schema (`maxItems`); no literal in code.
10. Specification first: 15 16 lets the crash target come from "the build configuration or the instance settings", but no instance setting key exists; this ticket uses the build configuration only and states that in 15 16 (Design note 5). ADR 0057 leaves the histogram path open; this ticket decides it with a test (Design note 3) and records it in the ADR's "More Information".

Product requirements implemented here:

11. The client sends only the names of signals.md 4.6a with their types; attributes only platform, app major version, density, theme and, for reconnects, the close code; no content, no IDs of threads, channels or people (15 17, 17 rules 3 and 9).
12. The server accepts only those names and kinds, discards each other entry one by one and answers `202` for the rest; it attaches no principal and no device; it writes no table (ADR 0057).
13. With `privacy.client_metrics` off the server answers `403 feature_disabled` and the client sends nothing (01 11).
14. The client sends at most once per `client.metrics.flush_interval`, batched (15 17).
15. Histograms use the boundaries "Durations up to seconds" of signals.md 4.7; other boundaries are discarded by the server.
16. Crash reports only with a configured target; no content, no screenshots, no view hierarchy, no session replay; the person can turn them off (`privacy.crash_reports`) (15 16).
17. No Dart package depends on an OpenTelemetry or analytics SDK; `sentry_flutter` is the only exception (ADR 0057 "Confirmation").

## Scope

Server, `services/server/`:

- `internal/stats/clientmetrics/` (or the layout the module rules of 02 4 give `stats`): the use case `SubmitClientMetrics` (setting check, per-entry filter, attribute normalization, bound check), the instruments registered with the catalogue mechanism of M1-68, and the HTTP handler bound to the generated interface; registration in the module register.
- Tests: unit tests of the filter and normalization, a handler test with `202`, `403 feature_disabled` and `400` for a body that violates the schema, the histogram path test (Design note 3), and the operation in the canary scenario of `test/canary` with canary values in every free field the batch allows (`app_version`).
- `services/server/internal/stats/README.md` (the endpoint and the histogram path).

App, `apps/client/`:

- `lib/telemetry/client_metrics.dart` (collector, Design note 1), `lib/telemetry/metrics_sender.dart` (batching, flush interval, `privacy.client_metrics`), `lib/telemetry/measurements.dart` (hooks into the app and the engine: cold start until the sidebar shows from the cache, thread open until the first messages are visible, send until the local display, frame timings, catch-up from the engine, reconnects from `ConnectionState`, outbox depth from M4-15), `lib/telemetry/crash_reports.dart` (Sentry setup and scrubbing, Design note 4).
- Tests under `test/telemetry/` (Tests).

Gate:

- `tools/ci/dart_sdk_denylist.py` with `tools/ci/tests/test_dart_sdk_denylist.py` (Design note 6), called from `make licenses` or `make lint` for Dart; `tools/pyproject.toml` and the `bandit` list only if the new file needs to be listed there.

Change:

- `apps/client/pubspec.yaml` (`sentry_flutter` at the version 15 16 names or newer with the same platform support), `apps/client/README.md`.
- `docs/spec/15-client-architecture.md` 16 (crash target from the build configuration only), 17 (the histogram path in one sentence); `docs/adr/0057-client-telemetry-through-a-server-endpoint.md` "More Information" (the decided histogram path); `docs/observability/signals.md` only if a server name or attribute had to be corrected, with the signal check green.
- `docs/DECISIONS.md` section 10: rows `D-M4-31-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Push display metrics (`client.push_display.duration`, `client.push.placeholder`): the mobile apps and the Notification Service Extension measure them (M5); the server accepts them already.
- Performance targets as tests (M4-34) and the terminal targets (M4-38).
- An instance setting for the crash target (needs a core contract change; Specification first).
- Dashboards for the client metrics (operations, M7).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-31-client-metrics`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `build(gates): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server unless a section below names a container. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH` (and Go and Docker where a section names them); without them the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout" ("Places that carry the product name"); the app shows `branding.product_name` from the instance, never a literal.
- No permission logic in the client (15 1 principle 1): controls follow server responses, snapshots and events, never an own derivation from grants.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M4 by D-W-114). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- Server tests follow the server's rules (`make test` with Go, the database harness only where a test needs it; this endpoint writes no table, so unit and handler tests need no database except what reading the person's setting needs).
- Never send a test metric or crash report to a real service; the Sentry transport is replaced by a recording transport in tests.

## Design notes

1. **Collector.** `ClientMetrics` keeps per metric name a histogram with the bounds of signals.md 4.7 (counts per bound plus overflow), counters and the gauge; `record(name, value, {int? closeCode})` rejects names outside the list at compile time through an enum generated or written from the 4.6a table, with a test that compares the enum with the table (walk up to the repository root). Attributes come from the build (platform, app version major) and the current theme and density. Nothing else is recorded.

2. **Sender.** Every `client.metrics.flush_interval` (and at app shutdown), if `privacy.client_metrics` is on and the batch is not empty: send `submitClientMetrics` with the batch (at most `maxItems` entries; split otherwise), not through the outbox (telemetry is not user data; the operation is idempotency-exempt), drop the batch on any error except a network error (keep one batch for the next interval, never more), stop after `403 feature_disabled` until the setting changes.

3. **Histogram path on the server.** Decide between OTLP data points built from the client's buckets and one measurement per bucket count; the test sends known buckets and reads the exported data from an in-memory exporter of M1-10, then compares counts per bound and the sum with the input (the sum may be bounded by the bucket bounds if the client sends none; document what is exact and what is not). Server-side checks: the bounds must equal 4.7's first row exactly (otherwise the entry is discarded); counts are non-negative; a counter value is non-negative; a gauge value is finite. Discarded entries are counted by an instrument the catalogue already has or one this ticket adds to signals.md with the signal check green. Ruling for docs/DECISIONS.md and the ADR note.

4. **Crash reports.** `SentryFlutter.init` only when the build define `crash_dsn` is set and `privacy.crash_reports` is on (default `true` when a target exists, 01 11); options: `sendDefaultPii = false`, `attachScreenshot = false`, `attachViewHierarchy = false`, no replay, `beforeSend` and `beforeBreadcrumb` that drop message texts, titles, file names, query strings and path segments that look like UUIDs, and HTTP bodies; release and environment from the build; a hashed instance ID as a tag. A test runs the scrubber over an event and breadcrumbs that contain canary values and asserts none survive.

5. **Canary on the client.** `test/telemetry/canary_test.dart` drives the collector, the sender (with a recording API adapter) and the crash scrubber while the app shows a thread with canary message text, a canary channel name and a canary file name, then asserts that no canary value appears in any sent batch, any crash event, any breadcrumb or any log line (the "canary test of the client paths" of PLAN M4-31).

6. **SDK deny list.** `dart_sdk_denylist.py` reads the root `pubspec.lock` and fails when it contains a package whose name matches the deny patterns (`opentelemetry*`, `firebase_analytics`, `amplitude*`, `mixpanel*`, `segment*`, `posthog*`, `datadog*`, `newrelic*`, `appcenter*`; the list with a comment per entry) unless it is `sentry_flutter` or one of its own dependencies; tests with a fake lock file.

7. **Rulings for docs/DECISIONS.md** (`D-M4-31-<n>`): the histogram path (note 3); the sender's error handling (note 2); the crash target from the build only (note 4); the deny list (note 6).

## Tests (write them first)

Server (Go, `make test`):

- Filter: unknown name, wrong `kind`, wrong bounds, negative counts are each discarded and the rest is accepted with `202`; `close_code` kept only for `client.reconnects` and mapped to `other` outside the codes of 04 6.4; `app_major` from `app_version` and `other` outside `^[0-9]{1,2}$`.
- Setting: with `privacy.client_metrics` off the answer is `403 feature_disabled`.
- Histogram path test (Design note 3) against the in-memory exporter.
- No principal or device attribute on any exported data point (the catalogue test of M1-68 extended to the new instruments).
- Canary: the operation is in the canary scenario and no canary value appears in logs, traces or metrics.

App (`flutter test`):

- `client_metrics_test.dart`: only the listed names; bounds of 4.7; enum equals the 4.6a table.
- `metrics_sender_test.dart`: one send per interval at most; nothing when the setting is off; stop after `403`; split above `maxItems`.
- `canary_test.dart` (Design note 5): no canary value in batches, crash events, breadcrumbs or logs; no URL with parameters in a report.
- `crash_reports_test.dart`: no init without a target or with the switch off.

Gate (pytest): `test_dart_sdk_denylist.py` with a fake lock file that contains a denied package and one that contains only `sentry_flutter`.

Commands that must pass, from the repository root:

- `make test` (Go and Dart), `make signal-check`
- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`, `make osv`, `make contract-test`
- `make check`

## Acceptance criteria

- [ ] The canary test of the client paths is green: no message text, name, file name or URL with parameters in batches, crash reports or logs.
- [ ] An entry with an unknown name or a mismatched `kind` is discarded on its own, the rest of the batch is accepted with `202` (signals.md 4.6a); `403 feature_disabled` when the setting is off.
- [ ] The histogram path into the Collector is decided and tested; ADR 0057 and 15 17 say which.
- [ ] Crash reports only with a configured target and scrubbed; the SDK deny list runs in the gates.
- [ ] `make signal-check` and the canary test of M1-69 are green with the new operation; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every exported Go identifier and every public Dart symbol; comments explain why and cite 15, 17, signals.md or ADR 0057.
- `services/server/internal/stats/README.md` and `apps/client/README.md` (English) as in Scope.
- 15 16 and 17 and ADR 0057 updated; rulings of Design note 7 in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The histogram path chosen with the numbers of its test, the list of measurement points in the app with what starts and stops each one, and whether the crash path was checked against a real Sentry endpoint (it must not be; state "recording transport only").
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
