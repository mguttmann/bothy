# M6-11: `bothy_viz`: primitives, models, fixtures and harness

Status: ready | Depends on: M1-04, M4-02, M4-04, M4-06, M4-08, M6-10 | Milestone: M6

## Goal

After this ticket `packages/bothy_viz` is the foundation of every visualization of level a (25 1, 7): view models built only from the generated `api_client` types (`StatsLiveFrame`, `StatsTimeline`, `StatsSloReport`, `StatsBackendReport`, `StatsSeries`, `CostSeries`), the two ports through which the apps and the console feed the views (`LiveFrameSource`, a stream of live frames; `StatsQuerySource`, the REST queries), the basic forms (lane, run card, area, bar, traffic light, metric tile) as `CustomPainter`s with semantics and goldens, JSON fixtures per view and state that the gate checks against the OpenAPI schemas (VZ-T11: a schema change without an adjusted fixture fails), and a harness app for desktop and web that shows the basic forms and, from M6-12 on, every view from the fixtures without a server, with switches for theme, density, text scaling, language and time-lapse. On the server side the generator `tools/observability/sli_bounds.py` reads the Sloth files and writes `services/server/internal/stats/domain/slibounds.gen.go` (thresholds for `good`, targets for the traffic light) and `packages/bothy_viz/lib/src/slo/slo_catalog.g.dart` (names and display, no thresholds); every SLO gets the label `bothy_sli_metric` with its `sli.*` metric, the Prometheus rules are regenerated, and two gate checks hold both: the generated files equal a fresh generation, and every SLO carries exactly one valid label and every `sli.*` metric maps to exactly one SLO (VZ-T03). Why: the visualization track starts before the server measures (P-21); views built on fixtures and generated types stay true to the API, and one source for SLO thresholds keeps Prometheus and the own measurement from drifting (R-VZ-03).

## Read first

- `docs/spec/25-visualization.md`: 1 (rules 1 to 5, anti-goals), 2, 3.2 (the closed metric list), 3.3 "Bounds from the Sloth files" (all of it), 4 (rule 5: no permission logic in the client), 5.2 (the live frame fields), 6 (the six views, to see what the forms must draw), 7 "Package `packages/bothy_viz`" (7.1 structure table, 7.2 contract, 7.3 tests, 7.4 accessibility), 10 (shared design language), 12 (VZ-T03, VZ-T11), 15 (`viz.*` values with place `App`).
- `docs/spec/15-client-architecture.md`: 3 and 3.1 (rule 6: `bothy_viz` without `sync_engine` and `local_store`; the port `LiveFrameSource`), 20 (golden matrix, `alchemist`), 21 (R-CL-13).
- `docs/design/components.md` 11 "Visualizations" (lane, run card, area and bar, traffic light, metric tile, tokens per form), 2 (live box colors and run states), 3 (tool chip), 10 (backend policy badge); `docs/design/README.md` (principles, ban list).
- `docs/observability/slos.md` 1 and 2 (the column "Metric of the own measurement" is the requirement for the labels), `docs/observability/signals.md` (no change expected), `infra/observability/slo/*.yaml`, `infra/observability/prometheus/rules/slo-*.yaml`, `tools/opscheck/opscheck.py` (checks `sloth`, `slo-docs`) and its README.
- `docs/api/openapi.yaml`: schemas `StatsLiveFrame` and everything it references, `StatsTimeline`, `StatsSloReport`, `StatsBackendReport`, `StatsSeries`, `CostSeries`, `StatsMetric`; operations `getStatsLive`, `getStatsTimeline`, `getStatsSlo`, `getStatsBackends`, `getUsageStats`, `getRunStats`, `getErrorStats`, `getCostStats`. `docs/api/asyncapi.yaml`: message `event.stats.live`.
- `docs/spec/03-data-model.md` (core contract, read only): `stats_minutes.metric` (the `CHECK` list the label values must be in).
- ADRs 0030, 0069; `docs/DECISIONS.md`: rulings T7, T8, D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-11-<n>` rows).
- Briefs (merged names win, D-W-100): `M4-01-workspace-arch.md` (members, `tools/arch-check/rules.yaml`, Dart gates, `platform-tests.json`), `M4-02-dart-api-client.md` (generated types, the client factory, tolerant decoding), `M4-04-design-system-theme.md` (theme builder, token extensions, the golden setup with `alchemist`), `M4-06-l10n.md` (`AppLocalizations`, number, date and currency formatting), `M4-08-domain.md` (the app configuration table with the `viz.*` values), `M6-10-viz-cvd-check.md` (the series roles).
- Code on the base branch: `packages/bothy_viz` (skeleton of M4-01), `packages/design_system`, `packages/l10n`, `packages/domain`, `packages/api_client`, `tools/arch-check/rules.yaml`, `tools/opscheck/`, `tools/api-checks/`, `services/server/internal/stats/` (skeleton).

This ticket touches no API operation, no table, no action and no event. It adds no server configuration key.

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
8. White label: examples, fixtures and test data use `example.com` and the people Alex, Sam and Robin, the spaces and agents of 07 10 (Support, Engineering, support-agent, dev-runner); no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. Limits, intervals and thresholds only through named configuration values: server values from `docs/operations/configuration.md`, app values (`viz.*`) from the app configuration table of `packages/domain` (M4-08); code and docs name the key, never the number.
10. Specification first: a change to behaviour, API or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. `bothy_viz` depends only on `design_system`, `l10n`, `domain` and `api_client`; never on `sync_engine`, `local_store`, a WebView or an iframe (15 3.1 rule 6, SEC-VZ-1); the architecture test holds it.
12. All statistics types come from the generated `api_client`; there is no hand-written type for statistics data (25 7.2 item 1); view models are pure Dart classes derived from those types.
13. No charting library: the basic forms are `CustomPainter`s with semantics; colors, fonts, spacing and motion come only from the tokens of `design_system` (25 7.1, 10; R-VZ-10).
14. No permission logic: a missing cost field means "not shown", never "zero" (25 4 rule 5, 1 rule 3); no content is ever drawn (run cards show agent, state, tool name, badge and cost only).
15. Every fixture is checked against its OpenAPI schema in the gate; a schema change without adjusted fixtures fails (25 7.2 item 2, VZ-T11).
16. SLO thresholds and targets exist only in the Sloth files; the server and the app files are generated from them and checked (25 3.3, R-VZ-03, VZ-T03).
17. Accessibility: every form has a semantics label, states carry icon and text, never color alone; live values are not announced continuously (25 7.4).

## Scope

Create in `packages/bothy_viz/`:

- `lib/bothy_viz.dart` (exports) and `lib/src/model/`: view models (`LiveTopologyModel`, `BackendModel`, `DeliveryModel`, `ExecutiveModel`, `TimelineModel`) with factories from the generated types, plus `RunCardModel`, `LaneModel`, `SeriesModel`, `TrafficLightModel` (Design note 2).
- `lib/src/sources/`: `LiveFrameSource` (`Stream<LiveFrameEvent>` with frame, stale and error events; `start({String? spaceId})`, `stop()`), `StatsQuerySource` (one method per query operation with typed parameters), `FixtureLiveFrameSource` and `FixtureStatsQuerySource` for tests and the harness, with a controllable clock (Design note 3).
- `lib/src/marks/`: `LaneMark`, `RunCardMark`, `AreaMark`, `BarMark`, `TrafficLightMark`, `MetricTileMark`, each a widget with a `CustomPainter` and a `Semantics` wrapper (Design note 4).
- `lib/src/slo/slo_catalog.g.dart` (generated, Design note 6) and `lib/src/slo/slo_catalog.dart` (lookup of display names through `l10n`).
- `fixtures/`: JSON fixtures per view and state, named `<schema>.<state>.json` with the states `empty`, `normal`, `load`, `truncated`, `no_cost_permission`, `offline` where they apply (Design note 5), and `fixtures/README.md` (which view reads which fixture).
- `harness/`: a Flutter app (`harness/lib/main.dart`, its own `pubspec.yaml` as a workspace member with `publish_to: none`) for macOS, Windows and web that renders every mark (and every view once M6-12 registers them) from the fixtures with switches for theme, density, text scaling, language and a time-lapse of the fake clock (Design note 7).
- Tests under `test/` (unit, widget, golden; Tests).

Create elsewhere:

- `tools/observability/sli_bounds.py` (standard library plus PyYAML from the pinned `tools/` environment), `tools/observability/README.md`, `tools/observability/tests/test_sli_bounds.py` (Design note 6).
- `services/server/internal/stats/domain/slibounds.gen.go` (generated; package doc and `DO NOT EDIT` header).
- `tools/api-checks`: a check `viz-fixtures` that validates every fixture against its schema in `docs/api/openapi.yaml` (Design note 5), with tests.

Change:

- `infra/observability/slo/*.yaml`: the label `bothy_sli_metric` on every SLO (values from `docs/observability/slos.md` 2); `infra/observability/prometheus/rules/slo-*.yaml` regenerated with the pinned `sloth` (the `sloth` check of `make ops-check` holds it).
- `tools/opscheck/opscheck.py`: checks `sli-bounds` (generated files equal a fresh run of `sli_bounds.py`) and `sli-labels` (Design note 6), registered with the other checks of `make ops-check`, with tests.
- Root `pubspec.yaml` workspace list and `tools/arch-check/rules.yaml`: the harness member with the same allowed dependencies as `bothy_viz` plus `flutter` (Design note 7).
- `tools/ci/components.py`: `tools/observability/` and `infra/observability/slo/` select the components whose gates run these checks (with a case each in its tests).
- `docs/spec/25-visualization.md` 7.1 and 7.2 (fixture names and states, the check `viz-fixtures`), `docs/observability/slos.md` 2 (the labels are set; the sentence "until then this column is the requirement" becomes "the label in the Sloth file"), `packages/bothy_viz/README.md`, `tools/opscheck/README.md`, `tools/api-checks/README.md`, `docs/quality/gates.md` (the new checks).
- Root `CHANGELOG.md` and `services/server/CHANGELOG.md` under 'Unreleased'; `docs/DECISIONS.md` section 10 rows `D-M6-11-1` to `D-M6-11-7`.

## Out of scope

- The six views and their table views (M6-12); the server's own measurement and queries (M6-13 to M6-15); the implementations of `LiveFrameSource` and `StatsQuerySource` in the apps and the console (M6-16).
- The Grafana panel plugin and the TypeScript token output (M7).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-11-viz-package`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(bothy-viz): ...`, `feat(observability): ...`, `build(gates): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fixtures replace the server. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH`; without it the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping. `sloth` is not taken from `PATH`: `make ops-check` fetches the pinned version through `tools/bin/fetch_tools.py` from `tools/bin/tools.lock.json`; run the regeneration of the rules with that binary.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Layout and dependencies** (15 3.1 rule 6; 25 7.1). `bothy_viz` imports `package:design_system`, `package:l10n`, `package:domain` and `package:api_client` only; `make arch` (M4-01) already forbids `sync_engine` and `local_store`; add a test of this ticket that greps the package for `webview`, `HtmlElementView` and `iframe` and fails on a hit (the package part of VZ-T16; M6-16 adds the app part).

2. **View models** (25 5.2, 6; Ruling for docs/DECISIONS.md). Factories take the generated types and the app configuration values (`viz.stale_after` and so on) and return immutable models; they never call the network. Rules that the models encode: runs grouped into lanes by space and channel in the order the frame gives; `runs_truncated` becomes a visible "more runs than shown" marker; a run card shows agent, state (the run state roles of the live box, components.md 2), current step `kind` and `name` (tool chip), billing mode badge (reported mode, otherwise configured mode, components.md 10), "Approval waiting" when `approval_pending`, and cost only for runs in mode `organization` and only if the cost members are present (`cost_is_estimate` gives the "estimate" label; 25 6.1, 01 D12); costs in mode `personal` appear only in cost tiles and backend rows per mode with the label "information only" (10 12); a missing member never becomes 0. Rates map metric names of 25 3.2 to labelled series through `l10n`. The traffic light model computes color and text from the server's `StatsSloReport` values (the server computes remaining budget; the model only maps the `StatsSloState` value to role, icon and text: `green` "on target", `amber` "budget tight", `red` "target missed", `insufficient_data` "not enough data").

3. **Ports** (15 3.1 rule 6; 25 5.1, 7.1). `LiveFrameSource` emits `LiveFrameEvent.frame(model)`, `LiveFrameEvent.stale()` when no frame arrived for `viz.stale_after` (computed by the source with an injected clock, so all implementations behave the same; provide a mixin or base class that the apps reuse), and `LiveFrameEvent.error(code)` for `forbidden`, `not_found`, `feature_disabled` and `rate_limited` (25 11). `StatsQuerySource` exposes typed calls for the query operations and returns the generated types or an `ApiProblem` of M4-02. The fixture implementations read the files of `fixtures/`, can replay a sequence of frames on the fake clock and can simulate the error codes.

4. **Basic forms** (components.md 11; 25 7.1, 7.4; R-VZ-10). Each mark is a widget that takes its model and the theme from `design_system`; colors only from `color.viz.*`, the run state roles and the state roles; series in the fixed order `color.viz.series.1` to `.8` with the wrap that M6-10 checks; fonts and spacing from the tokens; motion only from `motion.*` (no animation in the `reduced` context). Each mark has a `Semantics` node with a label built from the model (for example "dev-runner, running, tool Bash, organization, 0.62 USD estimate") and, for the traffic light, a polite live region that announces only a state change. Hit testing: a run card reports a tap with the thread ID to a callback (the apps open the deep link, the console shows "Open in the app"); the package itself never navigates.

5. **Fixtures and their check** (25 7.2; VZ-T11; Ruling for docs/DECISIONS.md). One fixture file per schema and state, for example `StatsLiveFrame.normal.json`, `StatsLiveFrame.truncated.json`, `StatsLiveFrame.no_cost_permission.json`, `StatsTimeline.load.json`, `StatsSloReport.normal.json` (all four traffic light states), `StatsBackendReport.normal.json`, `StatsSeries.usage.normal.json`, `CostSeries.normal.json`; the file name before the first dot names the schema. The check `viz-fixtures` in `tools/api-checks` resolves each schema in `docs/api/openapi.yaml` (with its references) and validates the fixture with the JSON Schema validator already pinned in `tools/` (add `jsonschema` with a license check only if none is pinned); it fails on an unknown schema name, an invalid fixture and a schema without at least one fixture among those listed in `fixtures/README.md`. A Dart test decodes every fixture with the generated `api_client` serializers and fails on a decoding error, so both sides of the contract are held. Fixtures contain no content (no message text, no titles), only IDs, names of agents and spaces from the test world, tool names and numbers; the `load` frame has exactly `stats.live_max_runs` runs at the default of 25 15, and the `truncated` frame `runs_truncated = true`.

6. **Generator `sli_bounds.py`** (25 3.3; slos.md 2; VZ-T03; Ruling for docs/DECISIONS.md). Reads every `infra/observability/slo/*.yaml`, per SLO: `service/name`, the label `bothy_sli_metric` (set on the SLO under `labels:`), `objective`, the threshold (the value of `le` in `error_query` or `total_query` when present, as seconds) and the period (the file's `slo_period` if set, otherwise the Sloth default of 30 days, slos.md 1 item 3). Writes `slibounds.gen.go` (a sorted Go map from metric name to `{SLO, Objective float64, Threshold time.Duration (zero when none), Period time.Duration}`, gofmt-clean) and `slo_catalog.g.dart` (a sorted const list of `{slo, metric, l10nKey}` without thresholds), both with a `DO NOT EDIT` header naming the generator. Modes `--write` and `--check` (check compares with a fresh run in a temporary folder and prints a diff). Label rules, checked by `sli-labels` and by the generator itself: every SLO has exactly one label `bothy_sli_metric`; its value is one of the `sli.*` values of the `CHECK` of `stats_minutes.metric` (read the list from `docs/spec/03-data-model.md` with a narrow parser and a test that the parser finds all six); every `sli.*` value maps to exactly one SLO; an unknown metric, a missing label, two SLOs with the same metric and an `sli.*` metric without an SLO each fail with a message. After setting the labels run the pinned `sloth generate` for every file (the command in the file header) so the `sloth` check stays green; the labels reach the generated rules, which is intended.

7. **Harness** (25 7.1 row `harness/`). A small app with a left list of marks (and later views), a toolbar with theme (light, dark), density (`comfortable`, `compact`, `touch`), text scaling (1, 1.5, 2), language (de, en), fixture state and a time-lapse slider that drives the fake clock of the fixture sources; it starts without any server and without network. It is a workspace member under `packages/bothy_viz/harness` with the same allowed dependencies as `bothy_viz`; `make arch` must accept it (add it to `rules.yaml`). A widget test pumps the harness and switches every toggle once.

8. **Goldens and tests** (15 20; 25 7.3). Every mark with every relevant fixture in light and dark, `comfortable`, `compact` and `touch`, text scaling 1 and 2, de and en, through the `alchemist` setup of M4-04 (CI goldens with the Ahem font on all hosts; platform goldens only on macOS). Keep the matrix within `repo.max_file_kb` per file; if the total size of the goldens becomes large, report it and keep only the CI goldens for the full matrix with platform goldens for one representative per mark.

9. **Rulings for `docs/DECISIONS.md`** (section 10): `D-M6-11-1` view model rules, missing cost is not zero (note 2); `D-M6-11-2` the stale computation in the port contract (note 3); `D-M6-11-3` mark semantics and the live region rule (note 4); `D-M6-11-4` fixture naming, states and the check `viz-fixtures` (note 5); `D-M6-11-5` the generator, its outputs and label rules (note 6); `D-M6-11-6` the harness as a workspace member (note 7); `D-M6-11-7` the golden matrix size rule (note 8).

## Tests (write them first)

Dart, `packages/bothy_viz/test/` (`flutter test`):

- `model_test.dart`: lanes from `StatsLiveFrame.normal.json`; `runs_truncated` marker; a frame without cost members gives cards without cost (not zero); a run in mode `personal` shows no cost on its card, while a cost tile of mode `personal` carries "information only"; traffic light states from `StatsSloReport.normal.json` map to role, icon and text.
- `sources_test.dart`: the fixture live source emits frames on the fake clock and `stale` after `viz.stale_after` without a frame; every error code becomes an error event; a new `start(spaceId)` replaces the old stream.
- `fixtures_decode_test.dart`: every file of `fixtures/` decodes with the generated serializers.
- `marks_golden_test.dart`: goldens of every mark with the matrix of Design note 8.
- `marks_semantics_test.dart`: each mark has a label; the traffic light announces a state change once and nothing on an unchanged frame; tap on a run card reports the thread ID; guidelines `textContrastGuideline`, `androidTapTargetGuideline`, `iOSTapTargetGuideline`, `labeledTapTargetGuideline` pass.
- `no_webview_test.dart`: no import or use of WebView, `HtmlElementView` or iframe in `lib/`, `harness/` or `test/`.
- `harness_test.dart`: the harness starts without a server and every toggle changes the rendered tree.

Python (pytest, run from `tools/`):

- `observability/tests/test_sli_bounds.py` (VZ-T03): the real files give no difference; a fixture tree with an SLO without the label, with an unknown metric, two SLOs with the same metric, an `sli.*` metric without an SLO each fail; thresholds parse from `le="0.5"` and from an absent `le`; the period default; outputs are deterministic and sorted.
- `api-checks` tests for `viz-fixtures`: a valid fixture passes; a fixture with a missing required member, with a member of the wrong type and with an unknown schema name fail; a schema change in a temporary copy of `openapi.yaml` (required member added) makes the real fixture fail.
- `opscheck` tests for `sli-bounds` and `sli-labels`.

Go: a test in `services/server/internal/stats/domain` that the generated map has the six `sli.*` metrics and that every threshold is either zero or positive.

Commands that must pass, from the repository root:

- `cd packages/bothy_viz && flutter analyze --fatal-infos && flutter test --coverage`
- `cd tools && uv run --locked --quiet pytest observability/tests api-checks opscheck/tests`
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make api-checks`, `make ops-check`, `make codegen-check`
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] VZ-T03 is green: generator and checks hold the labels and the generated Go and Dart files; the Prometheus rules are regenerated with the labels.
- [ ] VZ-T11 is green: every fixture is valid against its schema and decodes with the generated client; a schema change without an adjusted fixture fails the gate.
- [ ] Goldens of every basic form are green in the full matrix; semantics and guidelines pass.
- [ ] `bothy_viz` imports neither `sync_engine` nor `local_store` and contains no WebView or iframe; the harness starts without a server.
- [ ] 25 7.1 and 7.2, slos.md 2, the READMEs, gates.md, the CHANGELOGs and the rows `D-M6-11-1` to `D-M6-11-7` are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public Dart symbol (`public_member_api_docs`) and docstrings on every public Python symbol (ruff `D`); comments explain why.
- READMEs of `packages/bothy_viz`, `fixtures/`, `tools/observability`, `tools/opscheck` and `tools/api-checks` written or updated in English.
- Rulings of Design note 9 recorded in `docs/DECISIONS.md` section 10.
- Root and server `CHANGELOG.md` entries under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The label value per SLO, the size of the golden files, where the platform goldens ran.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
