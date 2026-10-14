# M6-03: Content Security Policy with a headless test

Status: ready | Depends on: M1-73, M6-02 | Milestone: M6

## Goal

After this ticket every console response carries the strict Content Security Policy of 19 5 and the remaining headers of SEC-HDR-3, and a headless browser test proves that the console runs under it with zero violations (SEC-T05): `default-src 'none'`, scripts only from the own origin plus `'wasm-unsafe-eval'` for the renderer, styles from the own origin and a nonce that the server generates per response and hands to the Flutter loader, images, fonts, workers and the manifest from the own origin, `connect-src 'self'` (also for the WebSocket of the terminal stream, which the test proves), `base-uri 'none'`, `form-action 'self'`, `frame-ancestors 'none'`, `object-src 'none'`, `require-trusted-types-for 'script'` with exactly the Trusted Types policy names that the test observes from the Flutter loader and engine (the list in the repository follows the test, not an assumption), and `report-to csp` with a reporting endpoint of the instance that accepts violation reports with a rate limit and logs no content. `Cross-Origin-Opener-Policy: same-origin` and `Cross-Origin-Embedder-Policy: require-corp` make the console cross-origin isolated. CanvasKit and the fallback fonts come from the own origin (`canvasKitBaseUrl`, `fontFallbackBaseUrl`). A new make target `console-e2e` starts the Compose smoke stack with the console, runs the headless test in Chromium with the CSP enforced and in Firefox and WebKit as a smoke test, fails on any violation, on any request to a foreign origin and on a failing WebSocket, and runs in a CI job that feeds `ci-ok` when the console or the server changes. Why: Flutter Web is not readily compatible with Trusted Types, nonces and COEP (PLAN 12 risks); the console reads everything except DMs, so XSS and clickjacking must be blocked by policy and proven by a test (T-AC-01, T-AC-02).

## Read first

- `docs/spec/19-security.md`: 3 (SEC-HDR-1 to SEC-HDR-3), 4 (SEC-CORS-3), 5 (the policy and rules 1 to 7), 11a (SEC-TERM-7: the terminal under the same CSP), 11b (SEC-VZ-2: `frame-src` only with `console.grafana_embed_url`, which M6-17 adds), 13 (P0: "CSP test of the console"), 14 (SEC-T03, SEC-T05, SEC-T19).
- `docs/spec/15-client-architecture.md`: 2 (web: current Chromium, Firefox, Safari; Wasm only in Chromium), 23.1 (console terminal transport on the same origin).
- `docs/adr/0028-*.md`, `docs/adr/0070-*.md` (item 2: the same CSP for the terminal).
- Flutter documentation on web initialization (`canvasKitBaseUrl`, `fontFallbackBaseUrl`, `nonce`; https://docs.flutter.dev/platform-integration/web/initialization) and on web renderers.
- `docs/PLAN.md` 12 (Risks: Trusted Types, nonce and COEP; Red team T-AC-01, T-AC-02: XSS, CSRF, clickjacking, session theft).
- `docs/spec/actions.yaml` and 05 (row `console.csp_report`, R-AZ-33), `docs/api/openapi.yaml` (the reporting operation is added by this ticket, Design note 4).
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-03-<n>` rows) and the rows `D-M6-02-<n>`.
- Briefs (merged names win, D-W-100): `M6-02-console-app.md` (serving under `/console/`, `index.html`, the loader, the headers already set, `console.assets_dir`), `M6-01-console-session.md` (cookie and CSRF header for the test login), `M1-73-compose-smoke.md` (the Compose smoke stack and overlays), `M1-18-http-chain.md` (security headers, rate limits), `M4-01-workspace-arch.md` (CI jobs and their rules in `ci_ok.py`).
- Code on the base branch: `apps/console`, `services/server/internal/platform/httpserver` (console serving), `infra/compose/`, `.github/workflows/ci.yml`, `tools/ci/`.

This ticket touches no table and no event. It adds one operation (spec first) for the action `console.csp_report`, which the core contract already has.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits only through named configuration keys; the rate limit of the report endpoint uses the class of the action (`rl.public_auth`); code comments name the key, never the number.
10. Specification first: the reporting operation goes into `docs/api/openapi.yaml` first; 19 5 gets the observed Trusted Types names and the WebSocket rule. The core contract is not edited by this ticket.

Product requirements implemented here:

11. The CSP of 19 5 exactly, on `index.html` and every console asset response; a fresh nonce per response; no `'unsafe-inline'` for scripts, no `'unsafe-eval'` (19 5 rules 2 and 3).
12. CanvasKit, the renderer's Wasm and the fallback fonts load from the own origin; the console makes no request to a foreign origin (19 5 rule 1; SEC-T05).
13. Trusted Types are required; the allowed policy names are exactly those the test observes, listed in one place in the repository (19 5 rule 4).
14. The WebSocket to the own origin works under `connect-src 'self'`; if a browser of 15 2 needs `wss://<host>`, the test proves it and the policy adds exactly that (19 5 rule 5).
15. Violation reports go to an endpoint of the instance with a rate limit and no content in the log (19 5 rule 6).
16. `frame-ancestors 'none'` and `X-Frame-Options: DENY`; COOP `same-origin` and COEP `require-corp`; `frame-src` is never set by this ticket (M6-17 adds it only with `console.grafana_embed_url`).
17. SEC-T05 runs in CI on every change of the console or the server.

## Scope

Create:

- `services/server/internal/platform/httpserver/csp.go` (or within the console serving of M6-02): the policy builder (directives from 19 5, the nonce per response from `crypto/rand`, the Trusted Types names from one Go constant list with a reference to the test that verifies them), the injection of the nonce into `index.html` (the loader's `nonce` option and the `nonce` attribute of the bootstrap script tag; Design note 2), and the headers COEP and `Reporting-Endpoints: csp="/api/v1/csp-reports"` (Design note 3).
- Handler of the reporting operation (`submitCspReport`, `POST /api/v1/csp-reports`, action `console.csp_report`, rate class `rl.public_auth`, body `application/reports+json` or `application/csp-report` limited by `http.max_json_body_bytes`, the cap of the HTTP chain for JSON bodies): parses only the fields it logs (`effectiveDirective`, `blockedURL` reduced to its origin, `disposition`), counts a metric `bothy.console.csp_violations` (spec first in `docs/observability/signals.md`), answers `204`; it never logs the document URL path, the sample or a source line (Design note 4).
- `infra/compose/test/compose.console-e2e.yaml` (an overlay of the M1-73 stack under `infra/compose/test`, which the installation bundle excludes): the TLS sidecar, the published port, the public URL and relying party settings, the seed job and, later, the host service container of M6-18 (Design note 7).
- `tools/console-e2e/` (Python with Playwright from the pinned `tools/` environment, or Dart with `puppeteer` if the tools environment cannot take Playwright; choose and record): the runner that starts the Compose smoke stack with the console overlay on loopback ports, logs in through the console form with a seeded local account and the virtual WebAuthn authenticator of the browser (CDP `WebAuthn.addVirtualAuthenticator`) for the step-up, opens every navigation entry, and asserts the conditions of Design note 5; `tools/console-e2e/README.md` and its tests (Design note 5).
- Root Makefile target `console-e2e` (not part of `gates`), the row `console-e2e` in the table "Make targets created by tickets" of the brief index.
- CI job `console-e2e` on `ubuntu-24.04` with Docker, the pinned Playwright browsers, `permissions: contents: read`, selected when the `apps` or `server` component changes, feeding `ci-ok` through the rule mechanism of M4-01 (Design note 6).

Change:

- `apps/console/web/index.html` and the loader configuration (M6-02): `canvasKitBaseUrl` and `fontFallbackBaseUrl` on the own origin, the `nonce` placeholder that the server replaces; the build includes the fallback fonts the console needs (the web build otherwise loads them from a foreign origin, 19 5 rule 1).
- `docs/api/openapi.yaml`: the operation `submitCspReport` (no security scheme, `x-authz-action: console.csp_report`, `x-rate-limit-class: rl.public_auth`); `make api-lint`, `make api-checks`, `make codegen-check`; `services/server/api/operations.yaml` enables it.
- `docs/spec/19-security.md` 5 (the observed Trusted Types names, the WebSocket finding, the reporting endpoint path), `docs/observability/signals.md` (the metric), `docs/quality/gates.md` (the target and the CI job), `apps/console/README.md`, root and server `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M6-03-1` to `D-M6-03-7`.

## Out of scope

- `frame-src` and the Grafana embedding (M6-17); the terminal stream itself (M6-18 runs SEC-T05 with an open terminal using this target).
- `patrol` web tests of the administration flows (M6-04 adds them to `console-e2e`).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-03-console-csp`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(httpserver): ...`, `test(console): ...`, `ci(console): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules); the stack publishes its ports only on `127.0.0.1` with free ports chosen by the runner.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. The browsers make no request outside the loopback (the test fails if they do); package and browser downloads only for the pinned versions.
- Docker and the pinned Playwright browsers must be available for `make console-e2e`; the ticket must run it once on the build host or stop with `Verdict: BLOCKED <tool> missing`; the CI job is the second evidence.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never add `'unsafe-inline'` or `'unsafe-eval'` to make a test pass.
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Policy builder** (19 5; Ruling for docs/DECISIONS.md). One function builds the header from fixed directives and the per-response nonce; the Trusted Types names come from one constant list. The policy is set on every response under `/console/` (HTML and assets), not on `/api/` responses (they have SEC-HDR-2). COOP `same-origin` and COEP `require-corp` on the same responses; every console asset is same-origin, so CORP is not needed for them, but set `Cross-Origin-Resource-Policy: same-origin` on them as well so a future cross-origin embed cannot load them.

2. **Nonce** (19 5 rule 2). The server reads `index.html` once at start, keeps it in memory with a placeholder, and per request substitutes a fresh nonce (128 bits, base64) into the bootstrap script tag and the loader option, and the same value into the `style-src` directive. `index.html` stays `Cache-Control: no-store` (M6-02), so a nonce is never reused.

3. **Trusted Types and WebSocket** (19 5 rules 4 and 5). Start with `trusted-types` set to the names the Flutter documentation or source suggests; run the headless test with the CSP enforced; collect every `trusted-types` violation and the policy names the engine tries to create; set the constant list to exactly the observed names; repeat until zero violations. Do the same for the WebSocket: the test opens a WebSocket to the own origin from the console's context (the gateway path with a cookie, which the server rejects or accepts; the test only checks that the browser did not block it by policy). Record the browser versions with the result.

4. **Reports** (19 5 rule 6; R-AZ-33; Ruling for docs/DECISIONS.md). The endpoint accepts the Reporting API format and the legacy `report-uri` format, unauthenticated (browsers send reports without cookies in many cases), limited by `rl.public_auth` per client IP and by `http.max_json_body_bytes`; it never stores reports; it logs at Info with `csp.directive`, the disposition and the blocked origin only, and counts the metric. Abuse cannot do more than raise a counter within the rate limit.

5. **Headless test** (SEC-T05; Ruling for docs/DECISIONS.md). For Chromium with the CSP enforced: collect `securitypolicyviolation` events through an init script and the browser console, collect every request URL; open the login, log in with the console form and the seeded password and TOTP, do the step-up with the browser's virtual authenticator (`userVerification` on), open every navigation entry, wait for the first frame of each; assert zero violations, zero requests to an origin other than the console origin, the WebSocket not blocked, and that `crossOriginIsolated` is true in the page. For Firefox and WebKit: the same flow as a smoke test, failing on violations; if a browser cannot run the renderer under COEP or Trusted Types, record the exact finding and fail (a supported browser of 15 2 must work). A negative test injects an inline script into a test copy of `index.html` served by the sidecar of note 7 for one test path (the sidecar rewrites `index.html` of that path only in the test overlay; the server image stays the release image) and asserts that the browser blocks it and that a report reaches the endpoint.

6. **CI** (M4-01; ADR 0039). The job installs the pinned Flutter (from the root `pubspec.yaml`), Go, `uv` and the Playwright browsers pinned in the tools lock, runs `make console-e2e`, uploads the violation log as an artifact only on failure (no secrets in it), and feeds `ci-ok` for changes of the `apps` or `server` component.

7. **Topology of the test stack** (M1-73 D1; SEC-TLS-1; Ruling for docs/DECISIONS.md). A browser on the host cannot use a published port of `api` directly: the server sees that traffic from the bridge gateway, not from loopback, and SEC-TLS-1 refuses plaintext from it, and widening `server.trusted_proxies` for a test would weaken the rule under test (M1-73 D1). The overlay therefore adds a TLS sidecar (a pinned `caddy` image by digest, or the reverse proxy the Compose stack already pins) with `network_mode: service:api`: it terminates TLS with a certificate for `localhost` and `api` from a test CA generated per run (containers on the Compose network, such as the host service of M6-18, use the name `api`), and forwards to `127.0.0.1` of the shared namespace, which the server treats as loopback. The `api` service publishes the sidecar's port on `127.0.0.1` with a free port chosen by the runner; `BOTHY_SERVER__PUBLIC_URL` (the key `server.public_url`) is `https://localhost:<port>`, `auth.webauthn_rp_id` is `localhost` and `auth.webauthn_origins` that origin, so the console's `Origin` check and the passkeys match what the browser sends; the browsers trust the test CA (Playwright context option or a profile with the CA). The seed job creates the owner with a local account, its password and TOTP secret known to the test, and the browser's virtual authenticator registers the passkey through the console after the first login (Chromium); Firefox and WebKit, which have no virtual authenticator in Playwright, do the step-up with password and TOTP. M5-06, M5-11 and M5-13 use the same pattern for devices.

8. **Rulings for `docs/DECISIONS.md`** (`D-M6-03-<n>`): the policy builder and where it applies (note 1); the nonce in a cached template (note 2); observed Trusted Types names (note 3); the WebSocket result (note 3); the report endpoint behaviour (note 4); the headless test and browsers (note 5); the CI job and its trigger (note 6); the topology with the TLS sidecar (note 7).

## Tests (write them first)

Go: `TestCSPHeader` (every directive of 19 5, a different nonce on two requests, the nonce equal in header and body, no `frame-src`, no `'unsafe-inline'` for scripts), `TestConsoleIsolationHeaders` (COOP, COEP, CORP, `X-Frame-Options`, `Permissions-Policy`), `TestCspReport` (both formats accepted with `204`, oversize rejected, rate limited after the class limit, the log line carries only directive, disposition and blocked origin; a canary in the document URL and the sample never appears in the log capture), SEC-T03 rows for console responses in the header test of M1-18 if it holds a list.

Headless (`tools/console-e2e/tests/`): the flow of Design note 5 in Chromium with zero violations (SEC-T05), the foreign origin check, the WebSocket check, `crossOriginIsolated`; Firefox and WebKit smoke; the negative inline script test with a report received.

Runner and CI tests: teardown after failure; the job is pinned, read-only and feeds `ci-ok`.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/platform/... ./internal/instance/... ./internal/wire/...`
- `make console-e2e` on the build host
- `make codegen-check`, `make api-lint`, `make api-checks`, `make signal-check`, `make runners-check`, `make lint`
- `make check`

## Acceptance criteria

- [ ] SEC-T05: zero violations, no foreign origin, WebSocket works, in Chromium with the CSP enforced; Firefox and WebKit pass the smoke test.
- [ ] The list of Trusted Types names is verified by the test and recorded in 19 5.
- [ ] CanvasKit and fonts come from the own origin; the nonce is fresh per response; COOP and COEP make the console cross-origin isolated.
- [ ] The report endpoint is in `openapi.yaml`, rate limited and logs no content.
- [ ] `console-e2e` exists, is in the make target table of the brief index, and runs in CI feeding `ci-ok`.
- [ ] 19 5, signals.md, gates.md, README, CHANGELOGs and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public symbol; comments explain why.
- `apps/console/README.md` and `tools/console-e2e/README.md` written or updated in English.
- Rulings of Design note 8 recorded in `docs/DECISIONS.md` section 10.
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
4. The observed Trusted Types names, the browser versions, the WebSocket finding.
5. What is not proven (for example Safari itself, where WebKit in Playwright stands in for it).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
