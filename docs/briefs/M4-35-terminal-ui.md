# M4-35: Package `terminal_ui` and terminal view

Status: ready | Depends on: M4-07, M4-09, M2-44 | Milestone: M4

## Goal

After this ticket `packages/terminal_ui` holds the terminal view of 15 23 for the desktop app, the mobile app and the console: a `TerminalController` that runs the flow of 15 23.2 through ports (`TerminalTransport`, `HostKeyPinStore`, the step-up of `auth`, `HostEvents`, and an API port for `getHost`, `openTerminalSession`, `attachTerminalSession` and `closeTerminalSession`): check the pin, create an ephemeral X25519 key in memory with its hash, step-up, open, verify that the returned key equals the checked one, connect the stream, `identify`, Noise message 1 with the session token through `bothy_noise`, message 2, `start`, then `data` and coalesced `resize` frames, `exit`, detach and reattach with replay between `replay_begin` and `replay_end`, and the stream close codes `4100` to `4104` with their reactions; the widget with the renderer `xterm` behind an escape sequence filter that blocks OSC 52, opens OSC 8 links only after a click and a confirmation and answers no title or clipboard query (TM-T14); the header (host, connection, LLM login badge, fingerprint start, remaining time), tabs, "Detach" and "End", the dialogs "Open terminal" and "Key changed" with `pin_changed` (TM-T18), the paste confirmation for text with line breaks, the extra keys bar, the states of components.md 12, and a renderer theme generated from the tokens `color.terminal.*`. `packages/domain` gains the fingerprint display rules. Why: one widget and one crypto path for every client (R-CL-14); the blocks and the pin warning are the client half of the terminal's threat model (T-TM-10, T-TM-01) and must not depend on the renderer's version.

## Read first

- `docs/PLAN.md`: section 3; section 10 (Exit criteria item 5, Test strategy "Terminal", Red team "Terminal", Risks "the renderer `xterm` ..."; rows M4-35 to M4-38); section 8 rows M2-38, M2-44.
- `docs/spec/15-client-architecture.md`: 3, 3.1 (rule 7: `terminal_ui` depends on `design_system`, `auth`, `api_client`, `bothy_noise`, `domain`; not on `sync_engine` or `local_store`), 8 (terminal stream never through `sync_engine`), 21 (R-CL-14 to R-CL-17), 22 (`client.terminal.scrollback_lines`, `client.terminal.font_size_default`, `client.perf.terminal_*`), 23 "Terminal view" (23.1 to 23.5).
- `docs/spec/24-terminal.md`: 4.3 (fingerprint), 8.4 (owner on another person's host), 5 "End-to-end stream" (5.1 protection goal, 5.2 protocol, 5.3 token, 5.4 frames, 5.5 pinning), 6.1 and 6.2 (opening, detach and reattach, `terminal.quick_reattach_window`), 6.6, 10.3 (close reasons), 11.2 "Rendering", 12 "Clients" (the `xterm` check), 13 (problem codes and stream close codes), 14 (TM-T14, TM-T18).
- `docs/spec/19-security.md` SEC-TERM-1 and SEC-TERM-6; `docs/security/threat-model.md` T-TM-01, T-TM-03, T-TM-10.
- `docs/spec/01-product.md`: 5.1 D11 (screen and the two dialogs), 7 (shortcuts in the terminal: bound by M4-36), 8 (row "Terminal"), 9 (terms), 13 (R-PR-10 to R-PR-12).
- `docs/design/components.md` 12 "Terminal view"; `docs/design/tokens.md` (roles `color.terminal.*`).
- `docs/api/openapi.yaml`: `getHost`, `openTerminalSession` (`TerminalSessionOpen`), `attachTerminalSession` (`TerminalSessionAttach`), `closeTerminalSession`, `TerminalSessionGrant`, `TerminalSession`; `docs/api/asyncapi.yaml` channel `terminalStream` (`terminal.hello`, `terminal.identify`, `terminal.error`, `noise.frame`, `x-close-codes`).
- `docs/DECISIONS.md`: D-W-100, D-W-114 (M2-38 is implied by M2-44); the rows of M2-38 (`D-M2-38-<n>`: byte order, frame rules, responder library); section 10.
- Briefs: `docs/briefs/M2-38-noise-core.md` (Design notes 6 to 8: the Dart API of `bothy_noise`, `NkInitiator`, `NoiseTransport`, frames, fingerprint, `testing.dart` with `NkResponder`), `docs/briefs/M2-44-terminal-sessions-relay.md` (session grant, stream path, `identify`), `docs/briefs/M4-07-core-components.md` (components, `DsBanner`, view model rule), `docs/briefs/M4-04-design-system-theme.md` (generated tokens, `dsGoldenMatrix`), `docs/briefs/M4-09-auth-dpop.md` (`StepUpService`, `StepUpPrompt`, `DpopProofFactory` for the stream `identify` in the apps).

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
9. Values only through named keys: `client.terminal.scrollback_lines` and `client.terminal.font_size_default` from `AppConfig`; the remaining time of the header is `opened_at` of the session plus a maximum duration that the app passes to the widget (`terminal.max_session_duration`, which no operation delivers; M4-36 passes it from `AppConfig`, the console later its own source); without it the header shows no remaining time; no literal in code.
10. Colors of the screen only from `color.terminal.*`: the renderer theme is generated from the tokens, no color value in code (components.md 12).
11. No terminal content in any log, error message, metric or crash report; the scrollback lives only in memory (R-CL-17, SEC-TERM-1).

Product requirements implemented here:

12. The flow of 15 23.2 in order; input is accepted only after message 2 and `start`; the ephemeral private key is never stored and is discarded after the handshake (R-CL-15).
13. A returned `noise_public_key` or `key_generation` that differs from the checked one aborts and closes the session (15 23.2 item 3).
14. Pinning (15 23.3, 24 5.5): without a pin store it after the first successful handshake; same key no notice; a different key shows the hard warning with both fingerprints, "Do not connect" is the main action and opens nothing, "Accept new key" opens with `pin_changed = true` and stores the new pin after the handshake; a higher `key_generation` alone does not change the warning (TM-T18).
15. Escape sequences: OSC 52 ignored, OSC 8 links opened only after a click with the visible target and a confirmation, title and clipboard queries unanswered; the blocks sit before the renderer (TM-T14, SEC-TERM-6).
16. Copy only by the person; pasting text with a line break asks "Paste with line breaks?" first (15 23.4).
17. Stream close codes: `4100` shows the end with `close_reason` and does not retry; `4101` asks to open or attach again with a new step-up; `4102` shows "detached" and attaches later; `4103` says another device attached and does not retry; `4104` shows a hard warning without a new attempt (15 23.2 item 8).
18. Detach keeps the session; reattach uses a new ephemeral key and replaces the screen with the replay; the server decides whether a new step-up is needed (`401 step_up_required`) (15 23.2 item 6).
19. The line at the cursor and, on command, the visible screen are readable for screen readers; running output is not read aloud (01 8).
20. If an owner opens or attaches on a host whose `assigned_user_id` is another person, the view shows a warning before the step-up and requests the step-up only after an explicit confirmation (24 8.4, TM-T26, T-TM-12).
21. `terminal_ui` imports neither `sync_engine` nor `local_store` (15 3.1 rule 7).

## Scope

Create or fill in `packages/terminal_ui/lib/src/`:

- `ports.dart` (`TerminalTransport`, `TerminalChannel`, `HostKeyPinStore` with `HostKeyPin`, `HostEvents`, `TerminalApi`, `LinkOpener`), `controller/terminal_controller.dart` (the state machine of 15 23.2, Design note 1), `controller/session_state.dart`, `controller/close_reactions.dart` (table of the stream close codes), `controller/frame_pump.dart` (encode, decode, coalesced resize), `pinning/pin_check.dart`.
- `render/escape_filter.dart` (Design note 2), `render/terminal_theme.dart` (from the generated tokens), `render/terminal_view.dart` (the `xterm` widget with the filter, font size, scrollback limit), `render/semantics.dart` (line at the cursor, read screen on command), `render/paste_guard.dart`.
- `widgets/terminal_header.dart`, `widgets/session_tabs.dart`, `widgets/open_terminal_dialog.dart`, `widgets/key_changed_dialog.dart`, `widgets/extra_keys_bar.dart` (one-shot and held Ctrl and Alt, labels), `widgets/terminal_states.dart` (connected, detached with "Reattach", ended, blocked), `widgets/link_confirm_dialog.dart`, `widgets/foreign_host_warning.dart` (24 8.4).
- `lib/terminal_ui.dart` exports; `lib/testing.dart` with a fake host on `NkResponder` of `bothy_noise/testing.dart` and a fake transport (Design note 4).
- `packages/domain/lib/src/rules/fingerprint_display.dart` (groups of four, the short form "7KQ2 9XTA ..", comparison ignoring separators) with tests.
- Tests under `packages/terminal_ui/test/` and goldens (Tests); ARB keys for every text in `packages/l10n`.

Change:

- `packages/terminal_ui/pubspec.yaml` (`xterm` at the newest release, `bothy_noise`, `auth`, `api_client`, `design_system`, `domain`, `l10n`), `packages/terminal_ui/README.md` (ports per app, flow, blocks, the `xterm` assessment).
- `tools/design-tokens` only if the generated Dart tokens lack an accessor `terminal_theme.dart` needs (with its test and `make tokens` green).
- `docs/spec/15-client-architecture.md` 23 (the filter position, the theme generation, the `xterm` state found), `docs/spec/24-terminal.md` 12 (the result of the `xterm` check with date and sources).
- `docs/DECISIONS.md` section 10: rows `D-M4-35-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- The desktop app's implementations of the ports (stream with DPoP `identify`, pin table in `local_store`, host events from `sync_engine`), the group "Hosts" and the shortcuts in the terminal (M4-36).
- Administration of hosts (M4-37); E2E and performance against a host service container (M4-38).
- Mobile wiring and the app switcher cover (M5); console wiring with the passkey step-up and IndexedDB pins (M6).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-35-terminal-ui`; one writer per worktree.
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

## Design notes

1. **Controller.** `TerminalController({required TerminalApi api, required TerminalTransport transport, required HostKeyPinStore pins, required StepUpService stepUp, required LinkOpener links, required Clock clock})`; `open({required HostId host, required String profile, int? workdirIndex, required int cols, required int rows})`: `getHost`, pin check (note 3), `generateEphemeralKeyPair` and `ephemeralKeyHash` of `bothy_noise`, step-up through `StepUpService` (the dialog "Open terminal" explains first), `openTerminalSession` with the hash and `pin_changed` when confirmed, compare `noise_public_key` and `key_generation` with the checked values (on a mismatch `closeTerminalSession` and a hard error), connect the transport to `stream_path`, wait for `terminal.hello`, send `terminal.identify` (the app's transport adds token and proof; the console's adds nothing, 15 23.1), `NkInitiator.start` with the host key, session ID and host ID, message 1 with the session token as payload, read message 2, send `start` with the size, and only then accept input. `data` frames for input; `resize` coalesced to at most one per frame tick; `exit` shows the code and keeps the tab; `detach()` closes the stream and keeps the session; `attach()` checks the pin against `getHost` first, exactly as `open` does (a changed key shows the warning, and a confirmed change sends `pin_changed = true` in `TerminalSessionAttach`, which the audit of `terminal.attach` records), calls `attachTerminalSession` with a new ephemeral key (step-up only when the server answers `401 step_up_required`), compares the returned `noise_public_key` and `key_generation` with the checked values (a mismatch closes and warns), replays between `replay_begin` and `replay_end` into a fresh buffer that replaces the screen at `replay_end`; `end()` sends `close` and calls `closeTerminalSession`. Any Noise or frame error is a hard warning without retry (`4104` semantics). The ephemeral private key is dropped right after message 2 and never leaves the controller.

2. **Escape filter.** A byte-stream parser in front of the renderer with its own tests, independent of `xterm`: it passes printable text and the CSI and SGR sequences the renderer needs, drops OSC 52 completely, rewrites OSC 8 hyperlinks into inert text plus a link record (target shown on hover; a click opens `LinkConfirmDialog` with the full target; only `https` and `http` open, through `LinkOpener`), drops queries that would make the terminal answer (window title reports `CSI 21 t` and similar, OSC queries with `?`, DECRQSS, and any sequence the renderer would answer with data other than cursor position and device attributes that shells need; record the allow list as a ruling), and handles sequences split across frames. The renderer's own handlers for these sequences are also disabled where `xterm` allows it, as a second layer.

3. **Pins.** `HostKeyPinStore.read(hostId)` and `write(HostKeyPin)`; without a pin: open, and after a successful handshake write `noise_public_key`, `key_generation`, fingerprint and first seen; same key: nothing; different key: `KeyChangedDialog` with both fingerprints in groups (domain rule), main action "Do not connect" (no session is opened), secondary "Accept new key" (open with `pin_changed = true`, write the new pin after the handshake).

4. **Test support.** `testing.dart` offers a `FakeHost` that runs `NkResponder` with a static key pair, checks message 1's payload as the expected token, answers message 2, records frames, can send `data`, `exit`, `replay_begin` and `replay_end`, and can close with any stream close code; and a `FakeTerminalTransport` pairing the controller with it in memory. M4-36 and M4-38 use the real host service instead.

5. **`xterm` assessment.** Check pub.dev and the repository of `xterm` for the latest release, open issues and security advisories (GitHub advisories and OSV for `xterm`) and its handling of OSC 52, OSC 8 and query answers; write the result with date and sources into 24 12 and the report; `make licenses` and `make osv` stay green. If a known advisory affects a feature the view uses and the filter cannot block it, stop with `Verdict: BLOCKED xterm advisory <id>`.

6. **Rulings for docs/DECISIONS.md** (`D-M4-35-<n>`): the controller as the only owner of the ephemeral key (note 1); the filter allow list and its place (note 2); pin write after the handshake (note 3); the fake host in `testing.dart` (note 4); the `xterm` assessment (note 5).

## Tests (write them first)

`packages/terminal_ui/test/` (fake host and transport of `testing.dart`, fake API, fake step-up, fake clock):

- `tm_t14_escape_filter_test.dart` (TM-T14): OSC 52 with and without BEL or ST terminator puts nothing into the clipboard (a fake clipboard records nothing); OSC 8 shows the target and opens only after a click and a confirmation; a title query and a clipboard query get no answer on the stream (the fake host receives no `data` frame); sequences split across two frames are handled.
- `tm_t26_foreign_host_test.dart` (the client part of TM-T26): an owner opening or attaching on a host assigned to another person sees the warning before the step-up and no step-up is requested without the confirmation; the person's own host shows none.
- `tm_t18_pin_test.dart` (TM-T18 as a widget test, for open and for attach): a pinned old key and a new key from `getHost` show the warning with both fingerprints; "Do not connect" opens nothing; "Accept new key" opens with `pin_changed = true` and stores the new pin after the handshake; a higher `key_generation` alone still warns.
- `controller_flow_test.dart`: input before `start` is not sent; a mismatch of the returned key closes the session; `exit` keeps the tab; detach and attach replace the screen with the replay; each stream close code `4100` to `4104` and `1000`, `1001`, `4013`, `4014` has its reaction.
- `ephemeral_key_test.dart`: the private key is not reachable after message 2 (the controller holds no reference; a weak reference test or an explicit zeroing of the bytes the library allows).
- `paste_guard_test.dart`, `extra_keys_test.dart` (one-shot and held modifiers, labels), `semantics_test.dart` (line at the cursor, read screen on command, no live region for output).
- `theme_test.dart`: every ANSI color and the screen roles come from the tokens; no color literal in `lib/` (a source scan in the test).
- `no_content_test.dart`: canary output on the stream appears in no log record and no exception message.
- Goldens (components.md 12): the view connected, with the key warning, detached, ended, blocked, with the extra keys bar; light and dark, text scaling 1 and 2, `de` and `en`.
- `packages/domain/test/fingerprint_display_test.dart` with the known vector of M2-38.

Commands that must pass, from the repository root:

- `cd packages/terminal_ui && flutter analyze --fatal-infos && flutter test --coverage`
- `cd packages/domain && dart test`
- `make arch` (no `sync_engine` or `local_store` in `terminal_ui`), `make lint`, `make fmt-check`, `make coverage`, `make licenses`, `make osv`, `make tokens`
- `make check`

## Acceptance criteria

- [ ] TM-T14 green; TM-T18 as a widget test with `pin_changed` green for open and attach; the owner warning of 24 8.4 (TM-T26 client part) green.
- [ ] Goldens light and dark with warning, detached and extra keys bar according to components.md 12; screen colors only from `color.terminal.*`.
- [ ] Semantics of the line at the cursor; no terminal content in logs or errors.
- [ ] State and open security advisories of `xterm` in the report and in 24 12; license gate green.
- [ ] No import of `sync_engine` or `local_store` (architecture test); rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 15 23 or 24.
- `packages/terminal_ui/README.md` (English) as in Scope.
- 15 23 and 24 12 updated; rulings of Design note 6 in `docs/DECISIONS.md` section 10.
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
4. The `xterm` version with its last release date, open issues and advisories found (with sources), the filter allow list, and which renderer handlers could be disabled as the second layer.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
