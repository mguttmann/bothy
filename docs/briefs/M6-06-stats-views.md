# M6-06: Statistics views

Status: ready | Depends on: M4-26, M6-02, M6-05, M6-11 | Milestone: M6

## Goal

After this ticket the screen A7 "Statistics" of 01 5.2 exists in `packages/admin_ui` and appears in the administration of the desktop app and at full width in the console (01 5.5): a time range selector (presets and a custom range within `stats.max_range`), a space filter (all spaces for the owner, the managed spaces for a space manager), metric tiles (runs with the change against the previous range, costs in mode `organization` with "Estimate" and in mode `personal` labelled "information only", open questions waiting for people), runs per day stacked by result, the p50 and p95 duration per agent (as `getRunStats` delivers them; an instance-wide percentile cannot be derived from per-agent values, so the screen shows none), costs per agent and per space, failure reasons, tool calls by tool name, error rates, and usage per space with active people shown as "fewer than `privacy.stats_min_group_size`" where the server withholds the value (`below_min_group`, M6-05). All numbers come from the five operations of M6-05; the screen computes nothing beyond formatting and the comparison with the previous range (two calls). Forms come from the basic marks of `bothy_viz` (M6-11), every chart has a table view. A link "Open telemetry dashboard" appears only when the server delivers `telemetry.dashboard_url`, which `getInstance` now carries for owners (spec first), and opens in the system browser on the desktop and in a new tab with `noopener` in the console. Why: owners and space managers need usage, runs, costs and errors at a glance, without performance monitoring of people (admin-statistics.md 3, T-OW-01).

## Read first

- `docs/spec/01-product.md`: 5.2 A7 (wireframe), 5.5 (console: full width), 8 (rows "Visualizations", "Color").
- `docs/observability/admin-statistics.md` (all; section 2 key figures and their grouping, section 3 privacy, section 4 permissions, section 6 telemetry link).
- `docs/spec/17-observability.md` 8 and 12 (`telemetry.dashboard_url`, `stats.max_range`).
- `docs/spec/25-visualization.md` 1 rules 2 and 3, 4 rule 2, 7.4, 9.1 (the link opens the system browser, never a WebView).
- `docs/api/openapi.yaml`: `getUsageStats`, `getRunStats`, `getToolCallStats`, `getErrorStats`, `getCostStats` and their schemas including `below_min_group`; `getInstance` and `InstanceInfo`.
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-06-<n>` rows) and the rows `D-M6-05-<n>`, `D-M6-11-<n>`.
- Briefs (merged names win, D-W-100): `M6-05-stats-server.md` (metrics per operation, grouping keys, `below_min_group`, the cache), `M6-11-viz-package.md` (marks and models; its `StatsQuerySource` serves the six views, not A7), `M6-02-console-app.md` (console routes and HTTP), the brief of M4-26 (the desktop administration navigation and `admin_ui` structure), `M1-46-instance-settings.md` (`getInstance`).
- Code on the base branch: `packages/admin_ui`, `packages/bothy_viz`, `apps/client/lib` (administration), `apps/console`, `services/server/internal/instance`.

This ticket touches no table, no action and no event.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names. Interface texts come from `packages/l10n`.
9. Limits only through named configuration keys (`stats.max_range` and `privacy.stats_min_group_size` are the server's; the screen shows what the server answers); code comments name the key, never the number.
10. Specification first: the new member of `InstanceInfo` goes into `docs/api/openapi.yaml` first. The core contract is not edited by this ticket.

Product requirements implemented here:

11. A7 with tiles, runs per day by result, costs per agent and per space, failure reasons, tool calls, error rates and usage, in `admin_ui`, shown in the desktop administration and in the console (01 5.2, 5.5).
12. Numbers only from the statistics operations; no grouping by person anywhere; "fewer than `privacy.stats_min_group_size`" for withheld values (admin-statistics.md 3).
13. Costs: `organization` with "Estimate" where `is_estimate`, `personal` always "information only", never summed together (admin-statistics.md 2).
14. The space filter offers what the server accepts for the caller; a `403` or `404` shows its reason; no client derivation of who may see what (15 1 principle 1).
15. The link to the telemetry backend appears only when `getInstance` delivers `telemetry_dashboard_url` (owners only) and opens outside the app; never a WebView or iframe (25 9.1).
16. Every chart has a table view; states carry icon and text (25 7.4).

## Scope

Create:

- `packages/admin_ui/lib/src/stats/`: `StatsScreen` (A7), its controller over the port `AdminStatsSource` of `admin_ui` with one typed call per operation of M6-05 (`getUsageStats`, `getRunStats`, `getToolCallStats`, `getErrorStats`, `getCostStats`; two range queries for the comparison), so `bothy_viz` and its `StatsQuerySource`, which serves the six views, stay unchanged, tile and chart sections composed of the marks of M6-11, the table views, the range and space selectors (Design notes 1 to 3).
- Tests (Tests).

Change:

- `docs/api/openapi.yaml`: `InstanceInfo` gains the optional `telemetry_dashboard_url` (description: present only for owners, from `telemetry.dashboard_url`); `make api-lint`, `make api-checks`, `make codegen-check`.
- `services/server/internal/instance`: `getInstance` sets the member for owners only (an owner check through `authorize` of `stats.read` on the instance in filter mode, so no audit and no error), from the configuration key `telemetry.dashboard_url`, which enters the schema with this ticket (`make config-reference`, `make config-doc-check`); handler test.
- `apps/client/lib` desktop administration navigation: the entry "Statistics" opening A7; `apps/console` route `stats` hosting A7 at full width; the desktop implementation of `AdminStatsSource` over the `api_client` with DPoP, the console implementation with the cookie client (the live source comes with M6-16).
- `packages/l10n` ARB files; `packages/admin_ui/README.md`; `services/server/internal/instance/README.md`; root and server `CHANGELOG.md` under `[Unreleased]`; `docs/DECISIONS.md` section 10 rows `D-M6-06-1` to `D-M6-06-4`.

## Out of scope

- The six live and timeline views and their hosting (M6-12, M6-16); CSV export of statistics (no operation yet, M6-05 "Out of scope").
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-06-stats-views`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(admin-ui): ...`, `feat(instance): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL only through `pgtest`; client tests use the fixtures of `bothy_viz` and the fake server of M4-17. Temporary directories only via `mktemp -d`, removed at the end. Stop only processes you started, by PID.
- Never read or print secrets.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Data** (admin-statistics.md 2; M6-05 Design note 2). The controller calls, for the selected range and space: `getRunStats` (tile runs; the p50 and p95 duration per agent as a bar section with its table; runs per day by result; failure reasons), `getCostStats` grouped by agent and by space (tiles and bars, one series per mode), `getToolCallStats`, `getErrorStats`, `getUsageStats` (usage section, `active_people` with `below_min_group`); for the comparison tile, the same run query for the previous range of equal length. "Waiting for people" comes from the runs in state `waiting_input` the caller can see (use the run list operation of M2 with the state filter, not a statistic; if no such filter exists, leave the tile out and record it). Granularity follows the range (hour up to a day, day up to a few months, week above; document the cut-offs as constants with a reason, they are presentation, not limits).

2. **Presentation** (25 7.4; 10 12). Tiles are `MetricTileMark`, bars and stacked bars are `BarMark`, series colors from `color.viz.series.*` in the fixed order, states with icon and text, every section with "As table". Costs show the currency of the response; `personal` and `organization` are separate series and separate tiles with their labels. A withheld value shows "fewer than <n>" with `n` from the instance limits if the client has `privacy.stats_min_group_size` there, otherwise the text without the number (M6-05 Design note 4).

3. **Scope selector** (admin-statistics.md 4). The owner sees "All spaces" and every space; a space manager sees the spaces where the server answers (the space list of the person's structure, then the first query decides: `403` or `404` removes the entry with a note). The console shows the same.

4. **Telemetry link** (17 8; 25 9.1; Ruling for docs/DECISIONS.md). Present only when `InstanceInfo.telemetry_dashboard_url` is set; desktop opens it with the system browser (`url_launcher` external mode); console with a new tab, `rel=noopener noreferrer`. The server sends the member only to owners, because the URL names internal infrastructure.

5. **Rulings for `docs/DECISIONS.md`** (`D-M6-06-<n>`): data per section and the granularity cut-offs (note 1); presentation of withheld values and costs (note 2); the scope selector (note 3); the telemetry URL for owners only (note 4).

## Tests (write them first)

- Widget and golden (`packages/admin_ui/test/stats/`, fixtures of `bothy_viz` and fake sources): A7 for the owner and for a space manager in light and dark, text scaling 1 and 2, de and en; `below_min_group` shows "fewer than"; `personal` costs show "information only" and are not summed with `organization`; table views carry the same values; the telemetry link appears only with the member; a `403` on a space removes it with the note.
- Server (`pgtest`): `TestGetInstance_TelemetryURL` (present for the owner when configured, absent for Sam and when empty).
- Integration: the desktop administration opens A7 against the fake server; the console route `stats` renders A7 (console widget test with the cookie client fake).

Commands that must pass, from the repository root:

- `cd packages/admin_ui && flutter analyze --fatal-infos && flutter test --coverage`
- `cd apps/console && flutter test --coverage` and `cd apps/client && flutter test --coverage`
- `cd services/server && go test -race -count=1 ./internal/instance/... ./internal/wire/...`
- `make codegen-check`, `make api-lint`, `make api-checks`, `make config-doc-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Widget tests of A7 pass in desktop and console; no grouping by person; withheld values show "fewer than".
- [ ] The link to the telemetry backend appears only with `telemetry.dashboard_url` (delivered to owners) and never as a WebView or iframe.
- [ ] `openapi.yaml`, the configuration reference, READMEs, ARB files, CHANGELOGs and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart and Go symbol; comments explain why.
- `packages/admin_ui/README.md` and the instance README updated in English.
- Rulings of Design note 5 recorded in `docs/DECISIONS.md` section 10.
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
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
