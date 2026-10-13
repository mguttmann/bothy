# M4-36: Terminal on the desktop

Status: ready | Depends on: M4-35, M4-24, M2-48 | Milestone: M4

## Goal

After this ticket the desktop app has its terminal (01 D11): the sidebar shows the group "Hosts" below the spaces with every host the person can see, its connection and the count of own sessions; a host opens the terminal view of `packages/terminal_ui` on the right with one tab per own session; "New session" offers the host's profiles and, for the profile `claude`, the working folders only from the person's own entry in `reported_session_users`; opening asks for step-up through the step-up device key; and the app implements the ports of 15 23.1: `TerminalTransport` as its own WebSocket per session with `identify` carrying the access token and a DPoP proof of the device (`auth`), `HostKeyPinStore` as the table `host_key_pins` in `local_store` (a new schema version, migrated without loss and never cleaned), and `HostEvents` from `sync_engine` (`host.updated`, `host.presence_changed`, `terminal_session.changed`). The terminal shortcuts of 01 7 work through the pass-through layer of M4-24, and no shortcut swallows Ctrl+C. Integration tests run the app against the fake server with an in-process host and against a real host service container of M2-48 with the real server, and prove that terminal output reaches neither `local_store` nor logs, crash reports or client metrics. Why: the desktop is the first client with a terminal (24 12); the device side of the pinning, the step-up and the stream must hold against a server that misbehaves (T-TM-01, T-TM-03, R-CL-17).

## Read first

- `docs/PLAN.md`: section 3; section 10 (Exit criteria item 5, Test strategy "Terminal", Red team "Terminal"; rows M4-24, M4-32, M4-35 to M4-38); section 8 rows M2-39 to M2-48.
- `docs/spec/01-product.md`: 3.8 "Terminal", 4 ("Hosts" in the information architecture), 5.1 D11 (sidebar group, header, tabs, footer, dialogs), 7 (both tables: app shortcuts and "In the terminal"; R-PR-10), 8 (row "Terminal").
- `docs/spec/15-client-architecture.md`: 6.1 (`host_key_pins`: never cleaned), 6.2 (`host_key_pins` migrated without loss), 8 (terminal stream never through `sync_engine`), 21 (R-CL-14 to R-CL-17), 23 (all of it).
- `docs/spec/24-terminal.md`: 2.2 (session users, `reported_session_users`, working folders of the profile `claude`), 5.3 (token), 5.5, 6, 8 (permissions, revocation `access_revoked`), 10.2 (events), 10.3 (close reasons), 11.1 (profiles), 13, 14 (TM-T06, TM-T10, TM-T12, TM-T14, TM-T18).
- `docs/spec/04-sync-protocol.md` 2.1 (host and terminal events: durable and ephemeral) and 8.2 (`access.changed` with `kind: host`).
- `docs/spec/07-permissions.md` 2.12 and SR10 to SR12 (shown as server reasons, never derived).
- `docs/api/openapi.yaml`: `listHosts`, `getHost` (`Host` with `profiles`, `session_users`, `reported_session_users`, `connected`, `llm_credentials`), `listTerminalSessions`, `getTerminalSession`, `openTerminalSession`, `attachTerminalSession`, `closeTerminalSession`; problem codes `terminal_denied` with its reasons, `host_unavailable`, `step_up_required`; `docs/api/asyncapi.yaml` channel `terminalStream`.
- `docs/DECISIONS.md`: D-W-100, D-W-114 (this ticket depends on M2-48 for the host service container and test overlay), D-W-115 and D-W-116 (`terminal.max_session_duration` in `AppConfig`); section 10.
- Briefs: `docs/briefs/M4-35-terminal-ui.md` (ports, controller, `testing.dart`), `docs/briefs/M4-24-shortcuts-focus.md` (`KeyboardScope.passThrough`), `docs/briefs/M4-12-local-store.md` (schema versions, `MigrationRules`, the survival test, `CacheCleaner`), `docs/briefs/M4-14-sync-catch-up.md` (appliers, skipped types until now), `docs/briefs/M4-09-auth-dpop.md` (`DpopProofFactory`), `docs/briefs/M4-17-fake-server.md` (handler registry, `FakeServer`), `docs/briefs/M4-31-client-metrics.md` if its collector exists (this ticket asserts the canary on whatever telemetry paths the base branch has), `docs/briefs/M2-48-terminal-conformance.md` (the overlay `infra/compose/test/compose.terminal.yaml`, host containers, pairing and confirming in the harness), `docs/briefs/M2-44-terminal-sessions-relay.md`.

This ticket touches no API operation, no table of chapter 03, no action, no event and no configuration key.

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
9. No terminal content in `local_store`, logs, crash reports or client metrics; scrollback only in memory (R-CL-17, SEC-TERM-1).
10. No permission logic: the hosts and profiles shown are what the server returns; denials show the server's `reason` (SR10 to SR12) (15 1).
11. In the terminal all keys go to the shell; app shortcuts only with Cmd on macOS and Ctrl+Shift on Windows (R-PR-10).

Product requirements implemented here:

12. The group "Hosts" below the spaces lists the visible hosts with connection state and the count of own sessions; a disabled host shows "disabled" (01 D11).
13. Opening: the dialog names host, profile and LLM login badge; the profile `claude` offers only the working folders of the person's own entry in `reported_session_users`; confirmation with the step-up device key (01 D11, 24 2.2, 08 6.3).
14. The stream `identify` carries the access token and a DPoP proof of the device; the stream never runs through `sync_engine` (15 8, 15 23.1).
15. Pins live in `host_key_pins` of the encrypted database, survive every migration and are never cleaned; a logout deletes them with all local data (15 6.1, 6.2, 23.1).
16. Host and session changes follow `host.updated`, `host.presence_changed` and `terminal_session.changed`; a revocation ends the open session with `4100` and `access_revoked`; another device attaching closes this one with `4103` (15 23.2, 24 8.3).
17. The shortcuts of the terminal table of 01 7 work; Ctrl+C, Ctrl+D, Ctrl+K and Alt combinations reach the shell (R-PR-10).

## Scope

Create or change:

- `packages/local_store`: table `host_key_pins` (`host_id`, `noise_public_key`, `key_generation`, `key_fingerprint`, `first_seen_at`) in a new schema version with its stored schema file, the generated migration test, `MigrationRules` extended so the table is migrated row by row, the survival test of M4-12 extended to it, `CacheCleaner` and `removeTargets` leaving it untouched (only `wipe` deletes it), and a `HostKeyPinsDao`.
- `packages/sync_engine`: appliers that turn `host.updated` and `terminal_session.changed` (durable) and `host.presence_changed` (ephemeral) into a `HostEventsHub` stream without content (IDs, versions, states), and `access.changed` entries with `kind: host` into a host-removed signal; no table for hosts (the app reloads through REST).
- `apps/client/lib/terminal/` (`hosts_group.dart` in the sidebar, `host_terminal_page.dart` with the `terminal_ui` widgets, `terminal_transport_io.dart` implementing `TerminalTransport` with `web_socket_channel` and the DPoP proof of `auth` for `identify`, `pin_store_adapter.dart`, `host_events_adapter.dart`, `terminal_api_adapter.dart` over `api_client`, `terminal_shortcuts.dart` binding the terminal table of 01 7 in the pass-through region of M4-24, `providers.dart`).
- The shortcut "Open hosts and terminal" (Cmd+Shift+T, Ctrl+Shift+T) in the app table of M4-24 and the route `/hosts/<host_id>`.
- `packages/domain` (`AppConfig` of M4-08): `terminal.max_session_duration` joins the server-place keys the client reads with the rule of D-W-115 (D-W-116), so the header shows the time left until the maximum duration (display only; the server ends the session); the terminal page passes it to `terminal_ui`, together with the host's `assigned_user_id` for the owner warning of 24 8.4.
- Fake server support through the registry of M4-17: host operations, terminal session operations, a stream endpoint that relays to an in-process `FakeHost` of `terminal_ui/testing.dart`, the events above, and the stream close codes on demand.
- `services/server/test/terminal/clientstack/` (Go, a `main` package used only by tests): it starts the terminal stack through the Go stack package of M2-48 (`services/server/test/terminal/stack`: Compose overlay, test CA, TLS front, host containers, pairing and `confirmHost`), creates Alex with a session user assignment and the grant `terminal`, writes a JSON file with the instance origin, the published port, the test CA certificate (public part only) and the credentials of the test world, waits until it receives a signal, and tears everything down; and root `Makefile` target `terminal-stack` (not in `gates`) that runs it in the background, runs `apps/client/terminal_stack_test/terminal_stack_test.dart` (outside `integration_test/`, Design note 6), and stops it; `docs/quality/gates.md` row; the table "Make targets created by tickets" in `docs/briefs/README.md` gains `terminal-stack` with M4-36.
- `docs/quality/accessibility-checklist.md`: the rows for the terminal view on macOS and Windows (01 8 row "Terminal"), with their automated evidence, if M4-32 has created the file; otherwise create the file with only the terminal section in the structure 01 8 gives.
- Tests (Tests); ARB keys in `packages/l10n`.

Change:

- `apps/client/README.md`, `packages/local_store/README.md`, `packages/sync_engine/README.md`.
- `docs/spec/15-client-architecture.md` 23.1 (the app's adapters).
- `docs/DECISIONS.md` section 10: rows `D-M4-36-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Administration of hosts and pairing (M4-37); terminal E2E and performance (M4-38).
- Mobile (M5) and console (M6) terminal wiring.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-36-terminal-desktop`; one writer per worktree.
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
- `make terminal-stack` needs Go and a Docker engine; on a host without one it fails with its preflight message, and the report names where it ran. Containers, volumes and temporary folders created by a run are removed by its cleanup.

## Design notes

1. **Transport.** `IoTerminalTransport.connect(Uri streamUrl)` opens the WebSocket, waits for `terminal.hello` (with the nonce for the proof), sends `terminal.identify` with the access token and a DPoP proof built by `DpopProofFactory` for the stream URL (the same form as the gateway proof, `htm = GET` and the HTTPS form of the URL, unless the asyncapi message names another form), then exchanges only binary frames; a text frame from the server after `identify` closes with the client-side error and is reported, never shown as output.

2. **Pins.** `pin_store_adapter.dart` implements `HostKeyPinStore` over `HostKeyPinsDao`. The schema step adds the table; `MigrationRules` lists it among the tables that are migrated row by row; the survival test fills pins and asserts them after the migration; `CacheCleaner` and `removeTargets` never touch the table; `wipe` deletes it with the database. Ruling for docs/DECISIONS.md.

3. **Hosts group and opening.** `listHosts` on start and after every host event; rows with connection (from `connected` and `host.presence_changed`), "disabled" for disabled hosts, and the count of the person's own sessions from `listTerminalSessions`. "New session": the profiles of the host; for `claude` the working folders from the entry of `reported_session_users` that belongs to the person's own session user (`session_users` names which entry is the person's; never offer another entry's folders), passed as `workdir_index`. The `terminal_ui` dialog explains, the step-up runs, and denials show the server's `reason` through `problemText` (`no_session_user`, `not_host_assignee`, `i6_holder_only`, `session_limit`, `profile_unknown`, `server_host_owner_only`).

4. **Shortcuts.** Inside the terminal the `KeyboardScope.passThrough` region of M4-24 lets every key reach the renderer except the platform-modifier shortcuts of the terminal table of 01 7: macOS with Cmd (copy, paste, quick switcher, new session, next and previous session, detach, search in the scrollback, font size), Windows with Ctrl+Shift (the same set with the keys of the table). A test sends Ctrl+C, Ctrl+D, Ctrl+K, Ctrl+L, Alt+F and Esc and asserts each arrives as a `data` frame at the host.

5. **Canary.** `terminal_canary_test.dart` lets the host print a canary line and a canary inside an escape sequence, then searches the database file, the log sink, the recording crash transport (if the app has one) and the metric batches (if the app has a collector) for the canary; none may contain it.

6. **Stack test.** The Go helper starts the stack of M2-48 with one host container and writes the connection data; the Dart test trusts the test CA through a `SecurityContext` that only its own entry point installs (an `HttpOverrides` for the API client and the WebSocket transports) and maps the instance origin to the published port the same way M2-48's Go dialer does; production code has no such hook outside the test entry; `terminal_stack_test.dart` runs the app's adapters and the `TerminalController` against it headless: open, run a command, read its output, detach, reattach with the scrollback replayed, revoke the grant (the server closes with `4100` and `access_revoked`), and attach from a second simulated device (`4103` on the first).

7. **Rulings for docs/DECISIONS.md** (`D-M4-36-<n>`): the stream `identify` proof (note 1); pins migrated and never cleaned (note 2); working folders only from the own entry (note 3); host data from REST plus events without a table (Scope); the stack test target (note 6).

## Tests (write them first)

- `packages/local_store/test/drift/` generated migration test of the new version and the extended survival test; `test/host_key_pins_test.dart` (cleanup and target removal leave pins; wipe removes them).
- `packages/sync_engine/test/hosts/host_events_test.dart` (the three event types reach the hub without content; `access.changed` with `kind: host` signals removal).
- `apps/client/test/terminal/` widget tests: hosts group states; the profile `claude` offers only the own working folders; denials show the server reasons; shortcuts (Design note 4).
- `apps/client/integration_test/terminal_fake_test.dart` (against `FakeServer` and the in-process host): opening with step-up, detaching, reattaching with scrollback, revocation (`4100`), other device (`4103`), changed key with warning (TM-T18 through the app), and the canary (Design note 5).
- `apps/client/terminal_stack_test/terminal_stack_test.dart` through `make terminal-stack` (Design note 6).

Commands that must pass, from the repository root:

- `cd packages/local_store && dart test`, `cd packages/sync_engine && dart test`
- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; on a host with a Docker engine: `make terminal-stack`
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Integration tests against a host service in a container and the fake server: opening, detaching, reattaching with scrollback, revocation (`4100`), other device (`4103`), changed key with warning.
- [ ] Shortcuts of 01 7 in the terminal; no shortcut swallows Ctrl+C.
- [ ] A canary of a terminal output is neither in `local_store` nor in logs, crash reports or client metrics.
- [ ] `host_key_pins` survives migration and cleanup; working folders of the profile `claude` only from the own entry in `reported_session_users`.
- [ ] Rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol and a header comment in the script; comments explain why and cite 15 23 or 24.
- READMEs (English) as in Scope.
- 15 23.1 updated; rulings of Design note 7 in `docs/DECISIONS.md` section 10.
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
4. Where the fake integration tests and the stack test ran (with or without a Docker engine), the schema version that added `host_key_pins`, and the telemetry paths the canary test covered.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
