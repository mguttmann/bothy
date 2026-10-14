# M6-18: Terminal in the console (ADR 0070)

Status: ready | Depends on: M2-48, M4-35, M4-37, M6-04 | Milestone: M6

## Goal

After this ticket owners and assignees can open terminals in the web console under the same rules as in the apps plus the console's own (ADR 0070; 24 12; 08 6.3 item 3): `terminal.open` and `terminal.attach` from a console session require a fresh, unconsumed step-up by passkey whose stored WebAuthn session required `userVerification = required`; a password with TOTP, a new OIDC login or a passkey without user verification give `401 step_up_required` with `reason: passkey_required`, and the quick reattach within `terminal.quick_reattach_window` never applies in the console (24 6.2), so every attach needs a fresh passkey. The console authenticates the terminal stream by its cookie session on the WebSocket upgrade with the `Origin` check (a foreign `Origin` is rejected before the upgrade, cross-site WebSocket hijacking), and its `identify` carries no token; an `identify` without a token from any session that is not this console session gives `4004`. In the console the `terminal_ui` view of M4-35 gets the console's ports: `TerminalTransport` over the browser's WebSocket on the same origin, `HostKeyPinStore` in IndexedDB of the origin, deleted at logout, and `StepUpPrompt` with the passkey (or a notice how to set one up when the person has none). The view states that pinning in the console does not protect against the server itself (24 5.1). Screen A8 (hosts, pairing, grants) from `admin_ui` (M4-37) runs in the console. ID-T28, TM-T07 and TM-T12 for the console are green; SEC-T05 passes with an open terminal; a `patrol` web test opens, detaches and reattaches; and the report measures `xterm` and `cryptography` under Flutter Web (PLAN 12 risk). Why: owners must reach a host from the console, right after the installation, without weakening the terminal's protection (T-TM-07, R-ID-16).

## Read first

- `docs/adr/0070-terminal-in-the-web-console.md` (all; "Confirmation" names these tests).
- `docs/spec/24-terminal.md`: 5.1 (protection goal; the console paragraph), 5.2 to 5.5, 6.1, 6.2 (no quick reattach in the console), 12 (row Web console), 13 (problem and close codes), 14 (TM-T07, TM-T12, TM-T14, TM-T18).
- `docs/spec/08-identity.md`: 6.3 (items 1 to 5; item 3 the console rule), 7.5 (cookie and `Origin` also on the WebSocket upgrade), 15 (ID-T28).
- `docs/spec/15-client-architecture.md`: 8 (step-up in the console), 23.1 (ports per app: console column), 23.2 (opening, streaming, reattaching), 23.3 (pinning; the console pins per browser profile), 21 (R-CL-15).
- `docs/spec/19-security.md`: 4 (SEC-CORS-3), 5, 11a (SEC-TERM-3, SEC-TERM-7, SEC-TERM-8), 14 (SEC-T05, SEC-T18).
- `docs/spec/01-product.md`: 5.5 (terminal opens only with passkey; pinned keys in the storage of this browser, deleted at logout), 5.2 A8, 5.1 D11.
- `docs/api/openapi.yaml`: `openTerminalSession`, `attachTerminalSession`, `closeTerminalSession`, `getHost`, `TerminalStreamIdentify` (its description already states the console rule), `createStepUpChallenge`, `completeStepUpWithPasskey`; `docs/api/asyncapi.yaml` channel `terminalStream` with `x-close-codes`.
- `docs/security/threat-model.md` 6.7 (T-TM-07 and the console items of PLAN 12 "Red team": XSS and extensions, cross-site WebSocket hijacking of the stream path, step-up without a passkey or without user verification, pinned keys after logout, CSP with an open terminal).
- `docs/DECISIONS.md`: ruling T3, D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-18-<n>` rows) and the rows of M2-40, M2-44, M4-35, M4-37, M6-01, M6-03.
- Briefs (merged names win, D-W-100): `M2-40-terminal-authz.md` (`StepUp.ConsumeFresh`, the console fails closed until this ticket, item 25 and the table row `passkey_required`), `M2-44-terminal-sessions-relay.md` (`internal/terminal/adapters/wsrelay`, identify, close codes), `M2-48-terminal-conformance.md` (host service container and test overlay), the briefs of M4-35 (`terminal_ui` and its ports) and M4-37 (A8 in `admin_ui`), `M6-01-console-session.md` (cookie session, `Origin` on upgrades, `ClientComponent` on the actor), `M6-03-console-csp.md` (`console-e2e`), `M6-04-console-admin.md` (hosting `admin_ui`, step-up in the browser), `M1-40-identity-passkeys.md` (stored WebAuthn session, `virtualauthn`).
- Code on the base branch: `services/server/internal/terminal`, `internal/identity`, `internal/authz`, `packages/terminal_ui`, `packages/bothy_noise`, `packages/admin_ui`, `apps/console`, `tools/console-e2e`.

This ticket touches no table, no action, no event and no configuration key.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names; the host in the tests is "Build host".
9. Windows only through named configuration keys (`terminal.step_up_max_age`, `terminal.quick_reattach_window`); code comments name the key, never the number.
10. Specification first: a change to behaviour updates `docs/spec/` or `docs/api/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. In the console, `terminal.open` and `terminal.attach` need a fresh, unconsumed passkey step-up with `userVerification = required`; anything else gives `401 step_up_required` with `reason: passkey_required` (08 6.3 item 3; ID-T28; TM-T07).
12. No quick reattach in the console: an attach within `terminal.quick_reattach_window` without a passkey gives `401` with `passkey_required` (24 6.2; TM-T12).
13. The stream upgrade from the console authenticates by the console cookie and requires `Origin` equal to the console origin; a foreign or missing `Origin` with the cookie is rejected before the upgrade; an `identify` without a token from a session other than the console session gives `4004` (08 7.5; SEC-CORS-3; `TerminalStreamIdentify`).
14. The ephemeral Noise key lives only in the memory of the tab; pinned host keys live in IndexedDB of the origin and are deleted at logout and at session expiry handling (R-CL-15; ADR 0070 item 3).
15. The view states that pinning in the console catches only a key change an honest server reports and does not protect against the server itself (24 5.1, 23.3).
16. The terminal runs under the CSP of M6-03 without exception; no HTML from terminal output (SEC-TERM-7; SEC-T05 with an open terminal).
17. A8 runs in the console from `admin_ui`, with step-up for every change.

## Scope

Server (paths under `services/server/` unless they start with `docs/`; every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`):

- `internal/authz` (where M2-40 enforces fresh step-up: `domain.FreshStepUp`, its facts type (`FreshFacts` in M2-40) and the decision mapping row for `passkey_required`): the console rule, with the user-verification fact supplied by `identity` from the stored WebAuthn session of the challenge; M2-40's tests that assert the console denial ("fails closed until M6-18") are rewritten to the rule below, not kept. The rule: for an actor with `ClientComponent = console`, fresh means `step_up_method = passkey` and the stored WebAuthn session of the challenge (`auth_challenges.webauthn_session`) required `userVerification = required`; otherwise `401 step_up_required` with `reason: passkey_required`; the quick reattach exception is never granted to a console actor (Design note 1).
- `internal/terminal/adapters/wsrelay`: the console path of `identify` (Design note 2): the upgrade request with the console cookie and a matching `Origin` yields the console session; an `identify` without a token is accepted only from that session for the session it opened; every other combination closes with `4004`; an upgrade with the cookie and a foreign or missing `Origin` is answered `403` before the upgrade.
- `docs/api/openapi.yaml` and `asyncapi.yaml`: descriptions only where the build sharpens them (the problem `reason` value `passkey_required` is listed already; verify).

Client:

- `apps/console/lib/terminal/`: `ConsoleTerminalTransport` (browser `WebSocket` to the stream path on the same origin, text `identify` without token, then binary frames), `IndexedDbHostKeyPinStore` (per host: public key, key generation, fingerprint, first seen; cleared at logout and when the session ends with the clearing cookie), `PasskeyStepUpPrompt` (WebAuthn with `userVerification: required`; without a passkey a notice with the way to set one up), the route "Hosts and terminal" with A8 from `admin_ui` and the terminal view of `terminal_ui` (Design notes 3 and 4).
- Tests (Tests).

Change:

- `tools/console-e2e` and its Compose overlay: include the host service container of the M2-48 test overlay, pair it through the API as the seed, and add the terminal suites (Design note 5).
- `packages/l10n` ARB files (passkey notice, pinning statement); `apps/console/README.md`; `services/server/internal/terminal/README.md`, `internal/identity/README.md`; `docs/spec/24-terminal.md` 12 (row Web console: the ports and the measured state of `xterm` under Flutter Web); root and server `CHANGELOG.md` under `[Unreleased]`; `docs/DECISIONS.md` section 10 rows `D-M6-18-1` to `D-M6-18-6`.

## Out of scope

- The apps' terminal (M4-36, M5-12); host administration server behaviour (M2-39 to M2-43).
- Local confirmation on the host (`host_service.require_local_confirm`, 24 6.4; M2).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-18-console-terminal`; one writer per worktree.
- Conventional Commits with the module or component as scope (for example `feat(identity): ...`, `feat(terminal): ...`, `feat(console): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules); the stack and the host service container publish ports only on `127.0.0.1`.
- Server tests bind only `127.0.0.1` and port 0; PostgreSQL only through `pgtest`; passkeys from `virtualauthn` (Go) and the browser's virtual authenticator (headless). Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; never log terminal content, a terminal session token, a Noise key or a cookie.
- Docker and the pinned browsers must be available for `make console-e2e`; the ticket must run it once on the build host or stop with `Verdict: BLOCKED <tool> missing`.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never relax the CSP for the terminal.
- No permission logic in the client (15 1 principle 1); the client never decides whether a step-up is fresh.
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Console step-up rule** (08 6.3 item 3, item 5; M2-40 item 25; Ruling for docs/DECISIONS.md). In `authz` where M2-40 decides fresh step-ups (`FreshStepUp` or its merged name; `identity.StepUp.ConsumeFresh` checks freshness again through the same authz check and then consumes, so the user-verification fact must reach both): if the actor's `ClientComponent` is `console`, require `step_up_method = passkey` and the challenge's stored WebAuthn session with `userVerification = required` (M1-40 stores it); a failure returns the problem with `reason: passkey_required`; the quick reattach branch is skipped for console actors. The consumption stays in the same transaction as the opening (M2-44). M2-40 left the console failing closed; replace that placeholder with this rule and rewrite its console tests accordingly.

2. **Stream authentication** (08 7.5; SEC-CORS-3; Ruling for docs/DECISIONS.md). On the upgrade of `/api/v1/terminal/sessions/{id}/stream`: with the console cookie, `Origin` must equal the console origin, else `403` before the upgrade; the cookie's session is attached to the connection. `identify` without a token: allowed only when the connection carries a console session, that session belongs to the person who opened or attached the terminal session, and the session's current terminal token was issued to that console session (the device of the console session matches `terminal_sessions` as for app devices); otherwise close `4004`. `identify` with a token from a console connection: close `4004` (one credential per path, as M6-01 Design note 4). Then the Noise handshake as for every device.

3. **Ports in the console** (15 23.1; R-CL-15; ADR 0070 item 3). The transport uses the browser `WebSocket` (binary type `arraybuffer`); the ephemeral X25519 key comes from `bothy_noise` with `cryptography` in the tab's memory and is dropped after the handshake. The pin store writes one IndexedDB object store per origin; logout (M6-02's logout and the clearing-cookie path) deletes the store before showing E7. `HostEvents` in the console is a reload through REST on opening the view and after each own action (no live update, 15 23.1).

4. **Pinning statement and A8** (24 5.1; 01 5.5). The open dialog and the key change warning in the console add one sentence: "In the browser this check only notices a key change that the server reports; it does not protect against the server itself." A8 from `admin_ui` (M4-37) is hosted under "Hosts and terminal" with the console's step-up adapter (M6-04).

5. **Tests in the browser** (ID-T28, TM-T07, TM-T12, SEC-T05; Ruling for docs/DECISIONS.md). The `console-e2e` overlay adds the host service container, paired by the seed; it connects over TLS to the sidecar of M6-03 Design note 7 under the name `api` and trusts the test CA, never in plaintext (24 2.2, SEC-TLS-1). The pairing declares the host as the server host (the host does not detect it, because `api` is not its own address; do not assert `reported_is_server_host`), so the suites open terminals as the seeded owner (SR10). The headless runner uses the browser's virtual WebAuthn authenticator with `isUserVerified` true or false per case. Suites: open with a UV passkey succeeds; with a password-TOTP step-up and with a passkey without UV the open gives `passkey_required` and no session; detach (close the stream) and attach within `terminal.quick_reattach_window` without a new passkey gives `passkey_required`, with a new UV passkey succeeds and replays the scrollback; an upgrade from a page on a foreign origin (a second test origin in the runner) with the cookie is rejected `403`; an `identify` without a token on a connection authenticated by an app access token (Go test) closes `4004`; logout leaves no IndexedDB pin store; with an open terminal the CSP collector records zero violations.

6. **Measurement** (PLAN 12 risk; 24 12). In the headless Chromium with the CSP enforced, measure the frame-to-paint time and the throughput of the terminal view with the output generator of the host service test image (the one M4-38 uses on the desktop, if the overlay offers it; otherwise `yes` piped through a rate limiter in the session) and record the values next to the desktop targets `client.perf.terminal_frame_to_paint_p95` and `client.perf.terminal_throughput_min`; the console has no target of its own, so the result is a measurement for 24 12, not a gate. Record the version and the open advisories of `xterm` and `cryptography`.

7. **Rulings for `docs/DECISIONS.md`** (`D-M6-18-<n>`): the console step-up rule replacing M2-40's placeholder (note 1); stream authentication and `4004` (note 2); IndexedDB pins and deletion (note 3); the pinning statement (note 4); the browser test setup (note 5); the measurement without a gate (note 6).

## Tests (write them first)

Go (`pgtest`, full chain, `virtualauthn`):

- `TestIDT28_Console` (ID-T28 console part): console session with password-TOTP step-up, with a passkey without UV, with a UV passkey older than `terminal.step_up_max_age`, and with a consumed UV step-up: each `401 step_up_required` with `reason: passkey_required` (or `terminal_step_up_not_fresh` for age and consumption where 08 6.3 names it); with a fresh UV passkey `201`; two simultaneous opens with one confirmation: exactly one session.
- `TestTMT07_Console` (TM-T07): `terminal.open` without step-up and with a password step-up from the console: `401`.
- `TestTMT12_Console` (TM-T12): attach within `terminal.quick_reattach_window` without a passkey from the console: `401` with `passkey_required`; from the app device within the window: allowed (regression).
- `TestStreamUpgrade_ForeignOrigin`: upgrade with the console cookie and a foreign or missing `Origin`: `403` before the upgrade.
- `TestStreamIdentify_NoTokenOnlyFromConsole`: `identify` without token from a connection without a console session: `4004`; from the console session that opened it: the handshake proceeds.

Dart (`apps/console/test/terminal/`): the IndexedDB pin store (with a fake IndexedDB) is cleared at logout; the passkey prompt shows the setup notice without a passkey; the pinning statement is present in the open dialog and the warning; the transport sends `identify` without token.

Headless (`make console-e2e`): the suites of Design note 5 (`patrol` web: open, detach, reattach; SEC-T05 with an open terminal) and the measurement of Design note 6.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/identity/... ./internal/terminal/... ./internal/authz/... ./internal/wire/...`
- `cd apps/console && flutter analyze --fatal-infos && flutter test --coverage`
- `make console-e2e` on the build host
- `make codegen-check`, `make api-lint`, `make api-checks`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] ID-T28 and TM-T07 for the console: password step-up and a passkey without user verification give `401 step_up_required` with `passkey_required`.
- [ ] TM-T12 for the console: reattaching within `terminal.quick_reattach_window` without a passkey gives `401` with `passkey_required`.
- [ ] An upgrade with a foreign `Origin` is rejected; an `identify` without a token from a session other than the console session gives `4004`; after logout the pins are deleted.
- [ ] The view claims no protection by pinning against the server; SEC-T05 with an open terminal has no violation; the `patrol` web test opens, detaches and reattaches.
- [ ] A8 runs in the console; the measurement of `xterm` and `cryptography` under Flutter Web is in the report and in 24 12.
- [ ] READMEs, 24 12, ARB files, CHANGELOGs and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Go and Dart symbol; comments explain why.
- READMEs of `terminal`, `identity` and `apps/console` updated in English.
- Rulings of Design note 7 recorded in `docs/DECISIONS.md` section 10.
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
4. The measured frame-to-paint and throughput in the browser, the versions and advisories of `xterm` and `cryptography`.
5. What is not proven (for example malicious browser extensions, the residual risk T-TM-07).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
