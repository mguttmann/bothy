# M6-12: `bothy_viz`: six views

Status: ready | Depends on: M6-11 | Milestone: M6

## Goal

After this ticket `packages/bothy_viz` contains the six views of 25 6, built only from the models, ports and basic forms of M6-11: the live topology (lanes per space, run cards, rates of the last minute; 25 6.1, screen D12), the backend view (configured and reported billing mode per agent, supported modes, switches, denials by I2 and I3, costs per mode, rate-limit state; 25 6.2, ruling T8), delivery and sync live (deliveries and their share within the threshold, reconnects, catch-ups, `resync_required` by reason, slow consumer closes, push acceptance, connections of one node; 25 6.3), the executive page (tiles for usage, runs and error rate, costs per mode, the SLO traffic light "own measurement"; 25 6.4), the replayable daily timeline (rates as areas, runs as bars per channel and agent, play, pause, jump and the speeds of `viz.timeline_speeds`; 25 6.5) and TV mode (full screen, dark, one font step larger, rotation every `viz.tv_rotation_interval`, costs only with `viz.tv_show_costs`, ended by a key or a click; 25 6.6, screen D13). Every view has a switch "As table" with the same values (25 7.4), follows the reduced motion context, and is pinned by goldens with every fixture in light and dark, `comfortable`, `compact` and `touch`, text scaling 1 and 2, de and en (VZ-T11). A performance test of the live topology with `stats.live_max_runs` runs meets `viz.perf.frame_build_p95`. The views know no transport; the apps and the console plug in the sources in M6-16. Why: the same views must run in the desktop app, the mobile app and the console (R-CL-13, ADR 0069), against fixtures before the server measures (P-21).

## Read first

- `docs/spec/25-visualization.md`: 1 (rules 1 to 5; anti-goals), 4 (rules 2 to 5: missing cost fields, "information only", no people), 5.2 (frame fields), 6 (all six views with their data sources and the traffic light table), 7.3 and 7.4, 9.1 (link "Open in Grafana" only with `telemetry.dashboard_url`, delivered by the app, not the package), 12 (VZ-T11), 15 (`viz.*`).
- `docs/spec/01-product.md`: 5.1 D12 and D13, 5.3 M10 (mobile tabs: topology, backend, delivery, executive, timeline; no TV mode on mobile), 5.2 A7 (the statistics view of M6-06 uses the tiles of the executive page), 8 (rows "Visualizations", "Motion", "Font size").
- `docs/spec/10-backend-policy.md` 3 and 4 (configured and reported mode, I5: no credentials, no subscription names), `docs/spec/09-agents.md` 4.8 (`rate_limit` states).
- `docs/design/components.md` 11 (forms), 2 (run states), 10 (badge); `docs/observability/admin-statistics.md` 2 (labels "Estimate" and "information only").
- `docs/api/openapi.yaml`: `StatsLiveFrame`, `StatsTimeline`, `StatsSloReport`, `StatsBackendReport`, `StatsSeries`, `CostSeries` and the operations that return them.
- `docs/DECISIONS.md`: rulings T7, T8, D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-12-<n>` rows).
- Briefs (merged names win, D-W-100): `M6-11-viz-package.md` (models, ports, marks, fixtures, harness, golden matrix), `M4-04-design-system-theme.md` (theme, density, motion), `M4-06-l10n.md` (formatting of numbers, currencies, times).
- Code on the base branch: `packages/bothy_viz` (M6-11), `packages/design_system`, `packages/l10n`, `packages/domain` (app configuration table).

This ticket touches no API operation, no table, no action, no event and no server configuration key.

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
8. White label: examples, fixtures and test data use `example.com` and the people Alex, Sam and Robin and the spaces and agents of 07 10; no company, host or personal names. Interface texts come from `packages/l10n`, never as literals in widgets.
9. Intervals and limits only through named configuration values (`viz.*` from the app configuration table of `packages/domain`, server values from `docs/operations/configuration.md`); code and docs name the key, never the number.
10. Specification first: a change to behaviour or configuration updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Each view draws exactly the data 25 6 names for it, from the models of M6-11; nothing is computed that the server did not deliver except presentation (sorting for display, formatting, the traffic light mapping of M6-11).
12. No values per person, no triggering person, no content; tools only by name; the backend view names no runtime of an instance, no subscription and no credential (25 1 rules 2 and 3, 6.2, ruling T8, I5).
13. Missing cost fields mean "not shown"; costs in mode `personal` carry "information only"; estimates carry "estimate" (25 4 rule 2, admin-statistics.md 2).
14. Every view has a table view with the same values and column headers; states carry icon and text; live values are not announced continuously; reduced motion removes transitions and makes the timeline jump (25 7.4).
15. TV mode: only for hosts that offer it (desktop app and console, never mobile), full screen dark, density `comfortable` with the font one step larger, no controls, rotation every `viz.tv_rotation_interval`, costs only with `viz.tv_show_costs`, any key press or click ends it (25 6.6).
16. The live topology with `stats.live_max_runs` runs builds a frame within `viz.perf.frame_build_p95` at p95 (PLAN row).

## Scope

Create in `packages/bothy_viz/`:

- `lib/src/views/live_topology_view.dart`, `backend_view.dart`, `delivery_view.dart`, `executive_view.dart`, `timeline_view.dart`, `tv_mode.dart`, and a `VizViewKind` enum with a registry that the hosts use for navigation and tabs (Design note 1).
- `lib/src/views/table/`: one table view per view (Design note 3).
- `lib/src/views/timeline/`: the playback controller over the fake or real clock (Design note 5).
- Fixtures that the views need and M6-11 did not add (for example a timeline over a day boundary, a backend report with I2 and I3 denials and every rate-limit state), with the check of M6-11 green.
- Harness: register the six views with their fixtures (M6-11 Design note 7).
- Tests under `test/` and the performance test (Tests, Design note 6).

Change:

- `packages/bothy_viz/README.md` (views, what each reads, how hosts embed them), `packages/l10n` ARB files (all view texts in de and en, with the terms of 01 9), `docs/spec/25-visualization.md` 6 only where the build proves a rule unbuildable (spec first, with a reason), root `CHANGELOG.md` under 'Unreleased', `docs/DECISIONS.md` section 10 rows `D-M6-12-1` to `D-M6-12-6`.

## Out of scope

- The sources in the apps and the console, navigation entries, the link "Open in Grafana", the mobile foreground rule and TV mode wiring in the hosts (M6-16).
- The A7 statistics screen of the administration (M6-06), which reuses the executive tiles.
- Server queries and the live stream (M6-13 to M6-15).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-12-viz-views`; one writer per worktree.
- Conventional Commits with the package as scope (for example `feat(bothy-viz): ...`, `feat(l10n): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fixtures replace the server. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH`; without it the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping. The performance test needs macOS; on another host `make apps-platform` prints its SKIP line and the `apps-macos` CI job is the evidence; the report names where it ran.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **View contract** (25 6, 7.1; Ruling for docs/DECISIONS.md). Each view is a widget `XxxView({required LiveFrameSource? live, required StatsQuerySource queries, required VizHostOptions options, required void Function(String threadId) onOpenThread})`. `VizHostOptions` carries what only the host knows: `formFactor` (`desktop`, `mobile`, `web`), `allowTvMode`, `showCostsInTv` (from `viz.tv_show_costs`), `scope` (instance or a space ID), and an optional `onOpenGrafana` callback (the host passes it only when `telemetry.dashboard_url` is set, 25 9.1). The registry lists for each `VizViewKind` its title key, icon, whether it needs instance scope (delivery and sync live do; a space scope shows "only for the owner" instead of the view, without calling the source) and whether it appears on mobile (all but TV mode). The views never decide permissions; an error event of the source (`forbidden`, `not_found`, `feature_disabled`, `rate_limited`) shows its text from `l10n` and stops polling until the host restarts the source.

2. **Data per view** (25 6). Live topology and delivery read `LiveFrameSource`; delivery adds history from `getStatsTimeline` for the selected metrics; the backend view reads `getStatsBackends` and updates the agent rows from the frame's `agents`; the executive page reads `getUsageStats`, `getRunStats`, `getErrorStats`, `getCostStats` and `getStatsSlo` (a space manager has no traffic light; the tile shows "only for the owner" when the source answers `403`); the timeline reads `getStatsTimeline` once and plays locally. The `offline` fixture state shows the last frame with the "stale" marker (`viz.stale_after`, M6-11 port).

3. **Table views** (25 7.4). A switch "As table" in every view header; the table shows the same values with column headers from `l10n`, sortable by the visible columns, readable by screen readers as a data table (`Semantics` with table roles where Flutter offers them, otherwise row and cell labels). The table never shows more than the view (no hidden columns with extra data).

4. **TV mode** (25 6.6; D13). A route-less overlay widget `TvMode(views: [...], options)`: dark theme regardless of the setting, density `comfortable` with the font one step larger (a text scale step from the tokens, not a literal factor), no controls, rotation every `viz.tv_rotation_interval` with a transition from `motion.*` (none in `reduced`), costs only with `showCostsInTv`; any key event or pointer down ends it through a callback. With `allowTvMode == false` the registry hides it and a direct construction asserts in debug builds.

5. **Timeline playback** (25 6.5). A controller with play, pause, jump to a minute and speed from `viz.timeline_speeds`, driven by an injected ticker so tests advance it deterministically; day boundaries come from the server response (`stats.time_zone` is applied server-side); `runs_truncated` shows a marker; reduced motion jumps between minutes without interpolation.

6. **Performance** (25 7.3; PLAN row; P-07). An integration test in `packages/bothy_viz/harness/integration_test/topology_perf_test.dart` renders the live topology with the `load` fixture (exactly `stats.live_max_runs` runs) and replays frames every `stats.live_interval` of the fixture clock for at least 200 frames, collects `FrameTiming.buildDuration` and fails when its p95 exceeds `viz.perf.frame_build_p95`. It runs with `flutter drive --profile` on macOS through `platform-tests.json` of the harness (`apps-platform`, M4-01) and as part of `make perf` where the Makefile reaches Dart members (if the `perf` target has no Dart step yet, add one that runs this test on macOS and prints a SKIP line elsewhere, with a test in `tools/ci/tests`). Optimise with repaint boundaries per lane and by rebuilding only changed cards, not by dropping content.

7. **Rulings for `docs/DECISIONS.md`** (section 10): `D-M6-12-1` the view contract and host options (note 1); `D-M6-12-2` instance-only views in space scope (note 1); `D-M6-12-3` data per view and the "only for the owner" tile (note 2); `D-M6-12-4` table views (note 3); `D-M6-12-5` TV mode as an overlay with assertions (note 4); `D-M6-12-6` the performance test and its place (note 6).

## Tests (write them first)

Dart, `packages/bothy_viz/test/`:

- `views_golden_test.dart`: every view with every fixture state that applies, in light and dark, `comfortable`, `compact` and `touch`, text scaling 1 and 2, de and en (VZ-T11 goldens), within the size rule of M6-11 Design note 8.
- `views_semantics_test.dart`: each view and its table view have labels; the table has the same values as the view (compare the model values rendered in both); guidelines pass; a traffic light change is announced once.
- `views_reduced_motion_test.dart`: no running animation in the `reduced` context; the timeline jumps.
- `views_no_cost_test.dart`: the `no_cost_permission` fixtures show no cost cell, no zero and no currency anywhere (search the rendered text); `personal` shows "information only".
- `views_no_people_test.dart`: the rendered text of every view with every fixture contains no principal ID of a person and no message text (the fixtures carry canary values in fields the views must not show, if the schema has such fields; otherwise assert by schema).
- `backend_view_test.dart`: configured next to reported mode, a mismatch marker, I2 and I3 denials, every `rate_limit` state with its text; no runtime name appears.
- `timeline_test.dart`: play, pause, jump and every speed of `viz.timeline_speeds` on the injected ticker; `runs_truncated` marker.
- `tv_mode_test.dart`: rotation every `viz.tv_rotation_interval`, costs hidden by default and shown with `showCostsInTv`, a key press and a click end it, hidden when `allowTvMode` is false.
- `space_scope_test.dart`: delivery and sync live in space scope show "only for the owner" and never call the source.
- `errors_test.dart`: each error event of the source shows its text and stops.

Integration: `harness/integration_test/topology_perf_test.dart` (Design note 6), listed in the harness `platform-tests.json` for `macos`.

Commands that must pass, from the repository root:

- `cd packages/bothy_viz && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS: `make apps-platform` (runs the performance test); elsewhere it prints its SKIP line
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make api-checks`
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] The six views exist, each with a table view, and read only the sources and models of M6-11.
- [ ] Goldens of every view with every fixture are green in the full matrix (VZ-T11); widget tests for semantics, table view and reduced motion pass.
- [ ] No costs without cost fields in the fixture; "information only" and "estimate" labels are shown; no person and no content appears.
- [ ] TV mode rotates, hides costs by default and ends on a key or click; it is never offered on mobile.
- [ ] The live topology with `stats.live_max_runs` runs meets `viz.perf.frame_build_p95` in the profile run on macOS.
- [ ] README, ARB files, CHANGELOG and the rows `D-M6-12-1` to `D-M6-12-6` are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public Dart symbol; comments explain why.
- `packages/bothy_viz/README.md` updated in English.
- Rulings of Design note 7 recorded in `docs/DECISIONS.md` section 10.
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
4. The measured p95 of the frame build with the machine it ran on, the size of the golden files.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
