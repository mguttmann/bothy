# M6-02: Console app and serving by the server (ADR 0028)

Status: ready | Depends on: M4-07, M4-09, M6-01 | Milestone: M6

## Goal

After this ticket `apps/console` is a Flutter Web app for administration, statistics and the terminal only (ADR 0028, ADR 0070; 01 5.5), built with the JavaScript build, and the server serves it from its own image under `/console/` on the same origin as the API. The console has the shell of 01 5.5 (navigation on the left: Live, Statistics, Hosts and terminal, People, Groups and grants, Devices, Agents and costs, Apps and webhooks, Audit, Import, Instance and branding, Status; each entry a placeholder until its ticket fills it), the login following E2 of 01 5.6 against the console form of M6-01 (provider buttons from `getDiscovery`, OIDC with a redirect to `/console/login/callback` in the same tab, passkey through WebAuthn in the browser, password with the one-time code), the step-up that 08 7.5 requires directly after login (passkey, otherwise a new OIDC login or password with TOTP), E7 after logout or expiry, and no device screens (no E3, E4, E5, no device pairing). Its HTTP layer uses the generated `api_client` with the session cookie, the CSRF header of M6-01 on every state-changing call and no DPoP; it holds no token. The console never depends on `sync_engine` or `local_store` (architecture test), has no route for channels, threads or messages (route test), turns on semantics at startup (ADR 0028 "Confirmation"), and shows and checks its version, which always equals the API version of the server that serves it. Why: administration must be reachable without an app (B5), with the same packages as the desktop (`design_system`, `api_client`, `auth`, `admin_ui`) and without chat in the browser (ADR 0028).

## Read first

- `docs/adr/0028-web-target-only-for-the-admin-and-statistics-console.md` (all; "Confirmation": architecture test, route test, semantics test), `docs/adr/0070-terminal-in-the-web-console.md` (item 5).
- `docs/spec/01-product.md`: 5.5 "Web console" (navigation and the note "No chat in the browser"), 5.6 (E2 and E7 in the console; "The web console uses only E2 and E7: it has no device confirmation, but requires a step-up after the login"), 8 (screen readers in the console: NVDA and VoiceOver), 9 (languages), 12 (branding).
- `docs/spec/15-client-architecture.md`: 2 (row Web: current Chromium, Firefox and Safari; JavaScript build), 3 and 3.1 (rule 3: `apps/console` without `sync_engine` and `local_store`), 8 (console step-up by WebAuthn passkey), 9 (row Web console: cookie session, no tokens in the browser), 10 (row Web: REST, fetch on opening), 11 (paths; `/admin/...`), 20.
- `docs/spec/08-identity.md`: 6.2 (console step-up methods), 7.5 (BFF session).
- `docs/spec/02-architecture.md`: 3.1 (row "Web console": static files), 6 "Code layout" (places with the product name).
- `docs/spec/19-security.md`: 3 (SEC-HDR-1 to SEC-HDR-3; the CSP itself comes with M6-03).
- `docs/api/openapi.yaml`: `getDiscovery`, the console forms of the login operations and the operations M6-01 added (`getConsoleSession`, `stepUpWithPassword`, `beginStepUpOidcReauth`, `completeStepUpOidcReauth`, `endViewAs`), `createStepUpChallenge`, `completeStepUpWithPasskey`, `logout`, `getInstance`.
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120), D-W-114, D-W-115; section 10 (format of `D-M6-02-<n>` rows) and the rows `D-M6-01-<n>`.
- Briefs (merged names win, D-W-100): `M6-01-console-session.md` (cookie, CSRF header `X-Console-Request`, console login forms, pre-step-up gate, operations), `M4-01-workspace-arch.md` (members, `rules.yaml`, platform folders: "M6 for web"), `M4-02-dart-api-client.md` (client factory), `M4-04-design-system-theme.md`, `M4-06-l10n.md`, `M4-07-core-components.md`, `M4-09-auth-dpop.md` (`LoginService` and its ports; the console uses the parts without DPoP), the brief of M4-18 (the E2 and E7 flows and texts of the app; the console builds its own screens in `apps/console` from `design_system` components, because one app never imports another, 15 3.1 rule 2), `M1-23-server-image.md` (the image build and `make image-test`), `M1-18-http-chain.md` (security headers, unmatched handler).
- Code on the base branch: `apps/console` (skeleton of M4-01), `packages/*`, `services/server/internal/platform/httpserver`, `services/server/Dockerfile`, `infra/compose/`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`; the console shows `branding.product_name` and the logo from the instance, never a literal.
9. Limits only through named configuration keys; a new server key (Design note 4) goes into `docs/operations/configuration.md` and its chapter first; code comments name the key, never the number.
10. Specification first: a change to behaviour, API or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Flutter Web only for administration, statistics and terminal; no chat, no route for channels, threads or messages; links into threads open the app through the deep link (01 5.5; ADR 0028).
12. No `sync_engine`, no `local_store`, no gateway connection, no offline storage, no push in the console (15 3.1 rule 3; ADR 0028).
13. No token in the browser: the console authenticates only with the cookie of M6-01 and sends the CSRF header on every state-changing request (08 7.5).
14. Login E2 and E7 only; step-up directly after login before any administration screen (08 7.5).
15. The console is served by the server under `/console/` from the server image; its version equals the API version of that server.
16. Semantics are turned on at startup (ADR 0028), the language of the semantics tree follows the app language, and the console works in the current versions of Chromium, Firefox and Safari (15 2).

## Scope

Create:

- `apps/console/web/` with `flutter create --platforms=web` (no service worker for offline use: set the build so the Flutter service worker is not registered; Design note 3), `index.html` with the loader configuration that M6-03 extends (asset URLs on the own origin).
- `apps/console/lib/`: `main.dart` (semantics on, theme from `design_system`, router), `shell/` (navigation of 01 5.5 with placeholder pages), `login/` (E2 and E7 in the console form, the OIDC callback route, the passkey adapter over WebAuthn through `package:web` interop, the step-up screen), `session/` (the console session state from `getConsoleSession`, logout, expiry handling with the clearing cookie), `http/` (the `api_client` factory for the web: cookie by the browser, `X-Console-Request: 1` on state-changing requests, no DPoP), `version/` (Design note 5) (Design notes 1 and 2).
- `services/server/internal/platform/httpserver/console.go` (or a package `internal/platform/consoleassets`): serving the console build under `/console/` with the SPA fallback to `index.html` for unknown paths below `/console/` (never for `/api/`), the right content types, `Cache-Control` (hashed assets long, `index.html` no-store), and the headers SEC-HDR-1 and SEC-HDR-3 except the CSP, which M6-03 adds (Design note 4).
- Tests (Tests).

Change:

- `services/server/Dockerfile.dockerignore` and `docs/spec/18-release.md` 7 rule 7 (spec first): the build context also admits `apps/console`, every member `pubspec.yaml` of the pub workspace (the root resolution needs them all), the `lib/` of the packages the console needs, the root `pubspec.yaml`, `pubspec.lock` and `analysis_options.yaml`, without tests and test data; the Flutter stage runs on `$BUILDPLATFORM` (18 7 rule 2), because its output is platform independent; the Flutter builder image is pinned by digest like every base image (18 7 rule 5, SEC-SC-4).
- `services/server/Dockerfile`: a build stage that builds the console (`flutter build web --release` with the pinned Flutter) and copies the output into the image at the path the server serves; `make image` and `make image-test` stay green and the image test asserts that `/console/` answers with `index.html` (Design note 4).
- `services/server` configuration: `console.assets_dir` (path of the built console; empty disables the console with `404` under `/console/`), first in `docs/operations/configuration.md` and the chapter that owns the console serving (02 3.1 or 16), then in the schema (`make config-reference`, `make config-doc-check`).
- `tools/arch-check/rules.yaml` only if the console's members need an entry M4-01 did not foresee; `make arch` must fail for an import of `sync_engine` or `local_store` in `apps/console` (M4-01 already holds it; add a test case for the real console if none exists).
- `packages/l10n` ARB files (console navigation and login texts).
- `apps/console/README.md` (build, serve, test), `docs/spec/15-client-architecture.md` 11 (console paths under `/console/`), `docs/spec/02-architecture.md` 3.1 (row "Web console": served by the api process under `/console/`), root and server `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M6-02-1` to `D-M6-02-7`.

## Out of scope

- The CSP, nonce, Trusted Types, COOP and COEP and the headless browser test (M6-03).
- Administration screens (M6-04, M6-07 to M6-09), statistics (M6-06), live views (M6-16), terminal (M6-18).
- Chat in the browser (never, ADR 0028).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-02-console-app`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(console): ...`, `feat(httpserver): ...`, `build(server): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL only through `pgtest`; browser tests in a headless Chromium on the loopback. Temporary directories only via `mktemp -d`, removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages; the console loads nothing from a foreign origin (fonts and CanvasKit are bundled; M6-03 enforces it).
- The Flutter SDK at the pinned version must be on `PATH`; without it the gates fail by design and the ticket stops with `Verdict: BLOCKED <tool> missing`.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Routes** (01 5.5; ADR 0028; Ruling for docs/DECISIONS.md). `/console/login`, `/console/login/callback`, `/console/step-up`, `/console/logged-out`, and below `/console/` one route per navigation entry (`live`, `stats`, `hosts`, `people`, `groups`, `devices`, `agents`, `apps`, `audit`, `import`, `instance`, `status`). A route test enumerates the router's configuration and fails on any path containing `channel`, `thread`, `message`, `space` (content) or `/t/`, `/c/`, `/s/`; a link to a thread renders "Open in the app" with the deep link `<scheme>://t/{id}` from the instance's branding (the scheme comes from `getDiscovery` if it carries one; otherwise the console shows the thread ID with a copy action and says to open it in the app; record which).

2. **Session and HTTP** (08 7.5; M6-01). After a reload the console calls `getConsoleSession`: `401` goes to `/console/login`; a session without `step_up_at` goes to `/console/step-up`. The `api_client` factory for the web uses the browser's fetch (cookies of the same origin are sent automatically), adds `X-Console-Request: 1` to every method except `GET`, `HEAD`, `OPTIONS`, and never adds `Authorization` or `DPoP`. A `401` leads to E7 or to the login by the problem type of the response (`unauthenticated` or a revoked or expired session to E7 with the reason, `step_up_required` to the step-up screen); the browser cannot read `Set-Cookie`, so the console never branches on the cookie. The passkey adapter calls `navigator.credentials.get` with `userVerification: required` for the step-up options of `createStepUpChallenge` (M1-40 format) and for the passkey login; the OIDC adapter navigates the tab to `authorization_url` and handles `/console/login/callback`, keeping the `client_verifier` only in memory of the tab (a reload during the login starts over).

3. **Web build** (15 2; Ruling for docs/DECISIONS.md). JavaScript build with the CanvasKit renderer files bundled into the build output (no CDN); the Flutter service worker is not used (no offline cache of an administration surface; a stale cached console could mismatch the API version); `SemanticsBinding.instance.ensureSemantics()` at startup; the HTML `lang` attribute follows the app language.

4. **Serving** (02 3.1; SEC-HDR-1, SEC-HDR-3; Ruling for docs/DECISIONS.md). The api process serves files from `console.assets_dir` under `/console/` through the HTTP chain (rate limits, security headers, unmatched handling): exact files with their content type, everything else below `/console/` falls back to `index.html`; `/console` without the slash redirects to `/console/`; paths with `..` or encoded separators are rejected before the file system. `index.html` gets `Cache-Control: no-store`; hashed assets get a long `max-age` with `immutable`; other files `no-cache`. Headers now: SEC-HDR-1, `Cross-Origin-Opener-Policy: same-origin`, `X-Frame-Options: DENY`, `Permissions-Policy` of SEC-HDR-3; COEP and the CSP come with M6-03 (record that SEC-HDR-3 is complete only after M6-03). The Dockerfile builds the console in its own stage and copies `build/web` to the path the image's default configuration names in `console.assets_dir`.

5. **Version** (PLAN row M6-02; Ruling for docs/DECISIONS.md). The build writes `version.json` with the `spec_version` of `docs/api/openapi.yaml` that its `api_client` was generated from (and the server version from `services/server/VERSION` for display); the console reads it at start and compares the `spec_version` with `Discovery.spec_version` of `getDiscovery` (the members `Discovery` and `InstanceInfo` really carry); a mismatch (a cached old `index.html` despite `no-store`, a proxy) shows "The console was updated, reload" and blocks actions. A Go test asserts that the served `version.json` carries the `spec_version` the server reports in `getDiscovery`.

6. **Accessibility** (01 8; ADR 0028). Semantics on, focus order per column, visible focus ring, keyboard navigation of the navigation rail; the login and step-up screens follow the onboarding rules of 01 5.6 (errors name the cause and the next step).

7. **Rulings for `docs/DECISIONS.md`** (`D-M6-02-<n>`): routes and the route test (note 1); cookie-only HTTP with the CSRF header and the passkey and OIDC adapters (note 2); no service worker (note 3); serving under `/console/` with caching rules and `console.assets_dir` (note 4); headers completed by M6-03 (note 4); the version check (note 5); the deep link fallback (note 1).

## Tests (write them first)

Dart (`apps/console/test/`, `flutter test`, plus `flutter test --platform chrome` for the web-only adapters):

- `routes_test.dart`: no route for channels, threads or messages (ADR 0028 "Confirmation"); every navigation entry has a route.
- `semantics_test.dart`: semantics are enabled after `main()` (ADR 0028 "Confirmation").
- `http_test.dart`: state-changing requests carry `X-Console-Request: 1`; no request carries `Authorization` or `DPoP`; `401` leads to E7.
- `login_test.dart`: E2 shows only the methods from a fake `getDiscovery`; OIDC navigates and completes on the callback; passkey with a fake `navigator.credentials`; password with the one-time code; after login the step-up screen comes first; E7 shows the reason; goldens of E2, step-up and E7 in light and dark, text scaling 1 and 2, de and en.
- `version_test.dart`: equal versions pass; a mismatch blocks actions with the reload notice.

Architecture: `make arch` fails on a test fixture where `apps/console` imports `sync_engine` or `local_store` (the case of M4-01 against the real console member).

Go (`services/server`): `TestConsoleServing` (files with types, SPA fallback only below `/console/`, `/api/` untouched, path traversal rejected, cache headers, SEC-HDR-1 and the listed SEC-HDR-3 headers present, `404` with empty `console.assets_dir`), `TestConsoleVersionJSON`.

Image: `make image-test` asserts `/console/` serves `index.html` from the image.

Commands that must pass, from the repository root:

- `cd apps/console && flutter analyze --fatal-infos && flutter test --coverage && flutter build web --release`
- `cd services/server && go test -race -count=1 ./internal/platform/... ./internal/wire/...`
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make config-doc-check`, `make image`, `make image-test`
- `make check`

## Acceptance criteria

- [ ] Architecture test: `console` does not depend on `sync_engine` and `local_store`; the route test finds no content routes; semantics are on at startup.
- [ ] The server serves the console under `/console/` from the image; the console version equals the API version and a mismatch is shown.
- [ ] Login following E2 and E7 with the cookie session, step-up directly after login, no device screens, no token in the browser.
- [ ] `console.assets_dir`, 15 11, 02 3.1, the README, the CHANGELOGs and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart and Go symbol; comments explain why.
- `apps/console/README.md` written in English.
- Rulings of Design note 7 recorded in `docs/DECISIONS.md` section 10.
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
4. The size of the web build and of the image layer it adds; the browsers the web-only tests ran in.
5. What is not proven (the CSP comes with M6-03).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
