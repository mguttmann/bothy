# M7-25: Grafana panel plugin "Bothy Live"

Status: ready | Depends on: M1-68, M7-24 | Milestone: M7

## Goal

After this ticket operations sees the live views of 25 in its own Grafana (level b, 25 8): the panel plugin `bothy-live-panel` in `plugins/grafana-bothy-live`, scaffolded with a pinned `@grafana/create-plugin`, TypeScript and React, shows the five views of 25 8.2 from metrics only (live topology as lanes per agent, backend view, delivery and sync, executive page with the SLO traffic light from the recording rules, timeline as playback over the dashboard period) and documents TV mode as Grafana's kiosk mode, verified against the documentation of the pinned Grafana version. The queries live in `queries.yaml`; a script generates `src/queries.gen.ts`, and a changed `queries.yaml` without regeneration fails. The naming test of 17 10 item 4 checks every metric in `queries.yaml` against the signal catalogue. The theme follows Grafana's light or dark theme and takes its colors and fonts only from `design/build/ts/tokens.gen.ts` (M7-24); a lint rule rejects color literals in the plugin's code. A seventh dashboard `bothy-live.json` uses the panel. The component `grafana-bothy-live` gets its `VERSION` and `CHANGELOG.md` (18 2), and the npm dependencies pass the license gate. The plugin never talks to the Bothy server and carries no IDs (17 rule 9). Why: level b brings the views to operations without giving Grafana any access to the product; it must use the same design language and the same metric names as everything else.

## Read first

- `docs/spec/25-visualization.md`: 1 (levels), 6 (the views of level a), 8 (all), 10, 12 (VZ-T13 belongs to M7-26), 14, 15.
- `docs/spec/17-observability.md` 2 (rule 9: no IDs on metrics), 7 (dashboards), 10 (item 4 name test); `docs/observability/signals.md` (the metrics of 25 8.2), `docs/observability/slos.md` (recording rules), `docs/observability/dashboards.md`.
- `docs/spec/18-release.md` 2 (component `grafana-bothy-live`), `docs/quality/licenses.md` (npm), `docs/quality/gates.md` (component gates for `plugins/`), `tools/ci/components.py` (release component and gate component of `plugins/`).
- `infra/observability/grafana/` (dashboards, provisioning, datasources), `infra/compose/compose.yaml` (Grafana version pin `BOTHY_GRAFANA_VERSION`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125), rulings T7, T8, T11; section 10 (format of `D-M7-25-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-68-core-metrics.md` (signal registry and the catalogue the name test reads), `M7-24-tokens-typescript.md`, `M6-12-viz-views.md` (the views of level a this plugin mirrors from metrics).

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: sample data and screenshots use agent names like "Scout" and "Lotse" and no channel, person or company names.
9. Every npm dependency is locked (`package-lock.json`), `@grafana/create-plugin` and the Grafana version are pinned; no CDN.
10. Specification first: a change to the views or queries updates 25 8 in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): the plugin stays unsigned until V1 (ruling T11); no Grafana Cloud account.
12. Never tag, never publish the plugin (D-W-125); the release path is M7-26.

Product requirements implemented here:

13. Only metrics of the catalogue and the recording rules of `slos.md`; no metric with an ID label; the name test fails on an unknown metric.
14. Colors only from `tokens.gen.ts`; a lint rule (ESLint with a rule against color literals in `src/`, or an equivalent check) fails on a hex, `rgb()` or named color outside the generated file.
15. `src/queries.gen.ts` is generated from `queries.yaml`; a check fails on a difference.
16. TV mode is documented with the kiosk URL form confirmed in the documentation of the pinned Grafana version (25 8.2 marks it as assumed); the plugin itself adds no full-screen mode.
17. The plugin's own tests (unit tests of each view with fixed data frames) run in the component gate of `plugins/`.

## Scope

- `plugins/grafana-bothy-live/` scaffolded with the pinned create-plugin: `src/` (panel, five views, theme adapter), `queries.yaml`, `scripts/gen-queries` (generator), `src/queries.gen.ts`, tests, `README.md`, `CHANGELOG.md`, `VERSION`, `package.json` and lock file, a `Makefile` with the component targets the gates expect (test, lint, typecheck, `generate-check`).
- `infra/observability/grafana/dashboards/bothy-live.json` using the panel; the dashboard provisioning lists it.
- The name test of 17 10 item 4 extended to `queries.yaml` (in `make ops-check` or the plugin gate; record where).
- `tools/ci/components.py`: the release component `grafana-bothy-live` and its path in the gate component `plugins` if missing.
- Docs: `docs/spec/25-visualization.md` 8 as built (kiosk form verified), `docs/observability/dashboards.md` (the seventh dashboard), `plugins/README.md`, root `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-25-<n>`.

## Out of scope

- Provisioning in the Compose stack, VZ-T13 and the release ZIP (M7-26); signing (V3-17, ruling T11).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-25-grafana-plugin`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(grafana-bothy-live): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). A local Grafana for manual checks binds only `127.0.0.1`.
- No network calls except package downloads of the locked npm dependencies and pinned tools.
- Install npm dependencies with `npm ci` in the plugin folder only; never copy a `node_modules` folder from elsewhere.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Scaffold and pin** (25 8.1; Ruling for docs/DECISIONS.md). Run the pinned `@grafana/create-plugin` once, keep its generated configuration, and record the version; the plugin's `grafanaDependency` matches the pinned Grafana of the Compose stack.

2. **Views from metrics** (25 8.2). Each view is a function from data frames to a rendering with tests on fixed frames; lanes per agent use the attribute values the catalogue allows; the executive page reads the three recording rules; the timeline uses the dashboard time range.

3. **Rulings for `docs/DECISIONS.md`** (`D-M7-25-<n>`): scaffold and versions (note 1), the color lint rule, where the name test runs, the verified kiosk form.

## Tests (write them first)

- Unit tests per view with fixed frames (rendering, empty data, theme switch).
- `generate-check`: a changed `queries.yaml` without regeneration fails.
- Name test: every metric of `queries.yaml` is in the catalogue; an unknown name fails.
- Color lint: a color literal in `src/` fails.
- License gate for npm green.

Commands that must pass, from the repository root:

- `make -C plugins/grafana-bothy-live test lint typecheck generate-check` (or the targets the component contract names)
- `make ops-check`, `make licenses`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] Unit tests of the views are green; the naming test checks every metric in `queries.yaml` against the catalogue.
- [ ] A changed `queries.yaml` without regeneration fails; colors only from `tokens.gen.ts` (lint).
- [ ] The license gate for npm is green; the kiosk path of TV mode is verified against the documentation of the pinned Grafana version.
- [ ] The dashboard `bothy-live` exists; 25, `dashboards.md`, READMEs, CHANGELOGs and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every exported symbol (TSDoc: purpose, parameters, return value, errors); comments explain why.
- `plugins/grafana-bothy-live/README.md` written in English.
- Rulings of Design note 3 recorded in `docs/DECISIONS.md` section 10.
- CHANGELOG entries under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. Screenshots of each view in light and dark from a local Grafana with fixture data.
5. What is not proven (Grafana Cloud, a signed plugin).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
