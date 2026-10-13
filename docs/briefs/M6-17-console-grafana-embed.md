# M6-17: Optional Grafana embedding in the console

Status: ready | Depends on: M6-16 | Milestone: M6

## Goal

After this ticket an instance operator can embed one Grafana dashboard in the console by setting `console.grafana_embed_url` (25 9.2, ADR 0069 item 4, R-VZ-12); without a value nothing changes: no embedding, no `frame-src`, the CSP stays exactly as M6-03 built it. With a value, the console's CSP gains `frame-src` with exactly the origin of that URL (SEC-VZ-2, 19 5 rule 7), and only owners see a page "Operations (Grafana)" with the dashboard in an iframe with `sandbox="allow-scripts allow-same-origin"` and `referrerpolicy="no-referrer"`; `frame-ancestors 'none'` and `X-Frame-Options: DENY` of the console itself stay. `bothy config check` rejects a value whose origin equals the console's origin (an iframe of the same origin would lift the sandbox) and warns when the value is on another site (the auth proxy cookie would not be sent). `docs/operations/` gets an example of the auth proxy in front of Grafana with the headers 25 9.2 requires (`Content-Security-Policy: frame-ancestors <console origin>`, `Cross-Origin-Resource-Policy: cross-origin`, `Cross-Origin-Embedder-Policy: require-corp`, only owners logged in). VZ-T12 and SEC-T19 for the console pass with a test proxy, and SEC-T05 passes with an embedded dashboard without a violation. Why: some operators want their operations dashboard next to the administration; the embedding is the one place where the console loosens its CSP, so it must be optional, exact and proven (T-VZ-01: clickjacking through the embedding).

## Read first

- `docs/spec/25-visualization.md`: 9.2 (all conditions 1 to 6), 12 (VZ-T12), 14 (R-VZ-12), 15 (`console.grafana_embed_url`).
- `docs/spec/19-security.md`: 3 (SEC-HDR-3), 5 (rule 7), 11b (SEC-VZ-2), 14 (SEC-T05, SEC-T19).
- `docs/adr/0069-*.md` (item 4), `docs/security/threat-model.md` 6.8 (T-VZ-01).
- `docs/operations/configuration.md` (row `console.grafana_embed_url`, status `planned (M6)`), `docs/spec/16-operations.md` (where `bothy config check` is described).
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-17-<n>` rows) and the rows of M6-03, M6-16.
- Briefs (merged names win, D-W-100): `M6-03-console-csp.md` (policy builder, `console-e2e`, the headless runner), `M6-16-viz-in-apps.md` (console "Live" navigation, the "Open in Grafana" rule), `M6-06-stats-views.md` (owner-only members of `getInstance`), `M1-07-config-schema-loader.md` (`bothy config check`).
- Code on the base branch: `services/server/internal/platform/httpserver` (CSP), `services/server/internal/instance`, `services/server/cmd` (config check), `apps/console`, `tools/console-e2e`, `infra/compose/`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; the Grafana example uses `https://grafana.example.com`; no company, host or personal names.
9. The embedding is controlled only by `console.grafana_embed_url`; code comments name the key, never a value.
10. Specification first: the new member of `InstanceInfo` goes into `docs/api/openapi.yaml` first; the configuration reference row becomes active with this ticket. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Without `console.grafana_embed_url`: no `frame-src`, no embedding page, no change to the CSP of M6-03 (VZ-T12).
12. With a value: `frame-src` with exactly its origin and nothing else; the iframe with `sandbox="allow-scripts allow-same-origin"` and `referrerpolicy="no-referrer"`; the console's own `frame-ancestors 'none'` and `X-Frame-Options: DENY` unchanged (25 9.2 item 5, SEC-VZ-2).
13. `bothy config check` fails for a value with the console's origin and warns for a value on another site (25 9.2 item 4).
14. Only owners see the page, labelled "Operations (Grafana)"; the product passes no identity and no token to Grafana (25 9.2 items 2 and 6).
15. SEC-T05 passes with the embedded test dashboard: zero violations, no request to an origin other than the console and the embed origin (the latter only from the iframe).

## Scope

Server (paths under `services/server/` unless they start with `docs/`; every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`):

- The configuration key `console.grafana_embed_url` enters the schema (URL kind, `https` only, empty allowed; `make config-reference`, `make config-doc-check`).
- The CSP builder of M6-03: adds `frame-src <origin>` only when the key is set (Design note 1).
- `bothy config check`: the same-origin rejection and the cross-site warning (Design note 2).
- `docs/api/openapi.yaml`: `InstanceInfo` gains the optional `grafana_embed_url`, present only for owners and only when set (the same filter as `telemetry_dashboard_url` of M6-06); `make api-lint`, `make api-checks`, `make codegen-check`; the instance handler fills it.

Client:

- `apps/console/lib/ops/grafana_page.dart`: the page "Operations (Grafana)" with the iframe through `HtmlElementView` and `package:web` (allowed in the console only; the static check of M6-16 covers `apps/client`, not `apps/console`), shown in the navigation only when `grafana_embed_url` is present (Design note 3).

Tests and docs:

- `infra/compose/console-e2e-grafana.yaml` (an overlay for `console-e2e`): a test proxy container that serves a static test dashboard page on its own origin with the headers of 25 9.2 item 3 and logs the requests (Design note 4).
- `tools/console-e2e`: the SEC-T19 and VZ-T12 suites (Design note 4).
- `docs/operations/grafana-embedding.md`: the example auth proxy (Grafana `[auth.proxy]` with a sender list, `allow_embedding = true`, the proxy headers, an own subdomain on the same site, only owners), linked from `mkdocs.yml` and the operations index.
- `docs/spec/25-visualization.md` 9.2 (the `getInstance` member, the console page), `docs/spec/19-security.md` 5 rule 7 only if the build sharpens it, `apps/console/README.md`, root and server `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M6-17-1` to `D-M6-17-4`.

## Out of scope

- Embedding in the apps (never, 25 9.1, SEC-VZ-1); the Grafana panel plugin (M7).
- Running or configuring a real Grafana in the product's Compose stack.
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-17-console-grafana-embed`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(httpserver): ...`, `feat(console): ...`, `docs(operations): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules); the stack and the test proxy publish ports only on `127.0.0.1`; the two origins in the test are two loopback host names (for example `console.localhost` and `grafana.localhost` resolved by the browser's host resolver rules), never external names.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Docker and the pinned browsers must be available for `make console-e2e`; the ticket must run it once on the build host or stop with `Verdict: BLOCKED <tool> missing`.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never add more than the one origin to `frame-src`.
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **CSP** (19 5 rule 7; SEC-VZ-2; Ruling for docs/DECISIONS.md). The origin is derived once at start from the configured URL (scheme, host, port; default port dropped); the builder appends `frame-src <origin>` to the policy of every console response; no wildcard, no path, no second origin. Without the key the builder output is byte-identical to M6-03 (test).

2. **Config check** (25 9.2 item 4; VZ-T12; Ruling for docs/DECISIONS.md). Same origin as `server.public_url`: error naming both keys. Same site (registrable domain via the public suffix list the toolchain already pins, or, if none is pinned, the last two labels with an explicit limitation in the message and the docs) but another origin: fine. Another site: warning that the auth proxy cookie will not be sent in the iframe. Not `https`: error.

3. **Page** (25 9.2 item 6). Visible only when `getInstance` returns `grafana_embed_url` (owners); title "Operations (Grafana)" with the note that the page follows the permissions of Grafana, not of the instance; the iframe fills the content area; a link opens the dashboard in a new tab with `noopener`.

4. **Tests with a test proxy** (SEC-T19, SEC-T05). The overlay runs a small static server (for example the `caddy` image pinned by digest, or a Python `http.server` in the tools image) on the embed origin that answers with `frame-ancestors <console origin>`, `Cross-Origin-Resource-Policy: cross-origin`, `Cross-Origin-Embedder-Policy: require-corp`. Suites: without the key the CSP has no `frame-src` and the navigation has no page; with the key the CSP has exactly one `frame-src` origin, the page loads the test dashboard inside the cross-origin isolated console with zero violations; a non-owner session sees no page and `getInstance` has no member; a second test origin that the CSP does not allow is blocked (negative test); the config check fails for the console origin.

5. **Rulings for `docs/DECISIONS.md`** (`D-M6-17-<n>`): one exact `frame-src` origin (note 1); the same-site rule and its limitation (note 2); the owner-only member and page (note 3); the test proxy instead of a real Grafana (note 4).

## Tests (write them first)

- Go: `TestCSP_FrameSrcOnlyWithKey` (byte-identical without the key; exactly one origin with it), `TestConfigCheck_GrafanaEmbed` (same origin error, other site warning, `http` error), `TestGetInstance_GrafanaEmbedOwnerOnly`.
- Dart (`apps/console/test/ops/`): the page appears only with the member; iframe attributes `sandbox` and `referrerpolicy` as required.
- Headless (`make console-e2e`): the SEC-T19 and VZ-T12 suites of Design note 4, and SEC-T05 with the embedded test dashboard.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/platform/... ./internal/instance/... ./cmd/... ./internal/wire/...`
- `cd apps/console && flutter analyze --fatal-infos && flutter test --coverage`
- `make console-e2e` on the build host
- `make codegen-check`, `make api-lint`, `make api-checks`, `make config-doc-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] VZ-T12: without a value no `frame-src`; with a value exactly this origin; with the console origin `bothy config check` fails.
- [ ] SEC-T19 for the console with a test proxy; SEC-T05 with an embedded dashboard without a violation.
- [ ] Only owners see the embedding, labelled "Operations (Grafana)".
- [ ] The example auth proxy is in `docs/operations/`; 25 9.2, the configuration reference, READMEs, CHANGELOGs and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public symbol; comments explain why.
- `docs/operations/grafana-embedding.md` written in English and linked; `apps/console/README.md` updated.
- Rulings of Design note 5 recorded in `docs/DECISIONS.md` section 10.
- Root and server `CHANGELOG.md` entries under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make console-e2e`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (a real Grafana behind a real auth proxy).
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
