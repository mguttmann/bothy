# M7-26: Plugin in the Compose stack, portability and release

Status: ready | Depends on: M7-14, M7-25 | Milestone: M7

## Goal

After this ticket the plugin reaches every instance with the observability profile and every release (25 8.3, 18 2): the Compose stack provisions the plugin build unsigned into `/var/lib/grafana/plugins/bothy-live-panel` and allows exactly this ID with `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=bothy-live-panel` (ruling T11, SEC-VZ-5), the provisioning loads the dashboard `bothy-live`, and a gate check rejects any other ID, a placeholder, a wildcard or a list in that setting. The portability test (M7-14) proves VZ-T13: the plugin is loaded, Grafana's log names only `bothy-live-panel` as unsigned, and the dashboard `bothy-live` loads without errors. The release of the component `grafana-bothy-live` (tag `grafana-bothy-live/v<version>`) ships the plugin as a ZIP with an SBOM, a checksum and a signature of the files (18 6), proven by the trial release (`make release-trial COMPONENT=grafana-bothy-live`, D-W-127). Why: exit criterion 7 of M7 names VZ-T13; T-VZ-05 (supply chain, the unsigned plugin allowed only by its ID) needs a check that holds the exact ID.

## Read first

- `docs/spec/25-visualization.md` 8.3 (distribution items 1 to 4), 12 (VZ-T13), 14; `docs/spec/19-security.md` (SEC-VZ-5); `docs/security/threat-model.md` (T-VZ-05).
- `docs/spec/18-release.md` 2 (component `grafana-bothy-live`), 6; `docs/spec/16-operations.md` 11.1 (portability steps).
- `infra/compose/compose.yaml` (service `grafana`, profile `observability`), `infra/observability/grafana/provisioning/`, `.github/workflows/portability.yml`, `.github/workflows/release.yml`.
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125), the trial release (D-W-127), ruling T11; section 10 (format of `D-M7-26-<n>` rows).
- Briefs (merged names win, D-W-100): `M7-25-grafana-plugin.md` (plugin build), `M7-14-portability-test.md` (`tests/portability`, `make portability-trial`), `M7-10-server-release-chain.md` (trial release, signing of files), `M1-04-ops-config-checks.md` (`make ops-check`).

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
8. White label: no names beyond the plugin ID and the dashboard name.
9. The Grafana image stays pinned; the plugin build in the stack comes from the release ZIP of the same version, verified by checksum.
10. Specification first: a change to the distribution updates 25 8.3 in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125); the plugin stays unsigned (ruling T11).
12. Never tag, never create a release, never publish (D-W-125).

Product requirements implemented here:

13. The allow setting holds exactly `bothy-live-panel`; the gate check (in `make ops-check` or a dedicated check of `make check`) fails on any other value, a placeholder, `*`, or a list with more than this ID.
14. VZ-T13 in the portability test: plugin loaded, the log names only `bothy-live-panel` as unsigned, dashboard `bothy-live` without errors (checked through Grafana's HTTP API on loopback).
15. The release ZIP carries an SBOM and appears in the signed `SHA256SUMS`; a missing SBOM aborts.

## Scope

- `infra/compose/compose.yaml` and `.env.example`: the plugin volume or download step, the allow setting, the dashboard provisioning (only in the profile `observability`).
- `infra/observability/grafana/provisioning/`: the plugin and the dashboard.
- The gate check of requirement 13 with tests (`tools/ci/` or `tools/ops/`).
- `tests/portability/` and `.github/workflows/portability.yml`: VZ-T13 as a step of the run with the profile `observability`.
- `plugins/grafana-bothy-live/Makefile`: `release-build` (ZIP); `release-trial` support; `.github/workflows/release.yml` for the component.
- Docs: `docs/spec/25-visualization.md` 8.3 as built, `docs/operations/` (enable the panel), `plugins/grafana-bothy-live/README.md`, CHANGELOGs under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-26-<n>`.

## Out of scope

- Signing and catalogue publication (V3-17, ruling T11).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-26-plugin-release`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(compose): ...`, `ci(release): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). The stack publishes only on `127.0.0.1`.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **How the build reaches the stack** (Ruling for docs/DECISIONS.md). The bundle of the product does not contain plugin code; the stack fetches the ZIP of the pinned plugin version from the release download address with checksum verification (like `install.sh` fetches the bundle), or the bundle carries the ZIP; choose one, record why, and keep the bundle rules of 18 2.1.

2. **Rulings for `docs/DECISIONS.md`** (`D-M7-26-<n>`): the delivery path (note 1), the gate check, the VZ-T13 check through the Grafana API.

## Tests (write them first)

- Gate check tests: exactly the ID passes; another ID, a placeholder, `*`, a list and an empty value fail.
- Portability (through `make portability-trial` with the profile `observability`): VZ-T13.
- Trial release of the component: ZIP, SBOM, checksum and signature verify; a missing SBOM aborts.

Commands that must pass, from the repository root:

- the gate check tests (`uv run --project tools --locked pytest -q tools/...`)
- `make portability-trial`
- `make release-trial COMPONENT=grafana-bothy-live`
- `make ops-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] VZ-T13 in the portability test is green.
- [ ] A gate check rejects any other ID, placeholder or list in the allow setting (SEC-VZ-5).
- [ ] Trial release with SBOM and checksum is verified.
- [ ] 25, the guide, READMEs, CHANGELOGs and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- Rulings of Design note 2 recorded in `docs/DECISIONS.md` section 10.
- CHANGELOG entries under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`, of the portability trial and of the plugin trial release.
3. Review and red team verdicts and the findings fixed.
4. The Grafana log lines about unsigned plugins from the test.
5. What is not proven (a real release of the component).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
