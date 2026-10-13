# M5-12: Terminal on mobile with extra keys bar

Status: ready | Depends on: M4-36, M5-08 | Milestone: M5

## Goal

After this ticket the phone apps open, use, detach and reattach terminal sessions (01 5.3 M8 and M9, 5.4; 15 23; 24 12): M9 lists the hosts the person may see with connection, LLM login badge and own sessions, and opens or attaches a session; M8 shows the session full screen with the `terminal_ui` view of M4-35 and, above the system keyboard, the extra keys bar with Esc, Tab, Ctrl, Alt, the four arrows, `|`, `~`, `/` and `-` (R-PR-11): Ctrl and Alt act once for the next key, a second tap latches them until the next tap (filled indicator, label "Control, latched"), and a hardware keyboard hides the bar. Two-finger swipes scroll the scrollback, a long press selects text for copying, and pasting more than one line asks first (15 23.4). When the app moves to the background, when the app lock of M5-08 locks, or on Android when system back is used in the terminal, the app detaches the session (it keeps running on the host) and the app switcher image and the recent apps screen show no terminal content (R-PR-12); back in the foreground it reattaches within `terminal.quick_reattach_window` without a new step-up and only after the app lock was passed, later with a step-up through the step-up device key with biometrics (the server decides, `401 step_up_required`; 08 6.3 item 5, 24 6.2). Terminal content never reaches the local database, logs, crash reports, client metrics or keyboard suggestions (R-CL-17). Widget tests cover the bar and goldens of M8 in light and dark with text scaling 1 and 2; integration tests against a host service container cover detaching in the background and reattaching within and after the window. Why: the terminal is core from M2 on, and each client milestone brings its view (P-20); on mobile the system ends connections in the background and the app switcher would show content (R-PR-12).

## Read first

- `docs/spec/01-product.md`: 5.3 M8 (extra keys bar, one-shot and latched modifiers, hardware keyboard, two-finger scroll, long press selection, detaching in the background, covered app switcher, reattach within the window, later with Face ID or fingerprint), M9 (hosts and sessions), 5.4 (Android: system back detaches instead of ending), 5.6 E6 (the lock detaches terminal sessions), 7 (terminal: all keys go to the shell), 8 (row "Terminal"), 13 (R-PR-10 to R-PR-12).
- `docs/spec/15-client-architecture.md`: 8 (step-up for `terminal.open` and `terminal.attach` fresh and consumed; terminal stream separate from `sync_engine`), 23 (all of it: 23.1 packages and ports, 23.2 opening, streaming, reattaching, item 7 mobile detach and cover, item 8 close codes, 23.3 pinning, 23.4 rendering and input, 23.5 tests), 21 (R-CL-14 to R-CL-17), 22 (`client.terminal.scrollback_lines`, `client.terminal.font_size_default`).
- `docs/spec/24-terminal.md`: 5.3 to 5.5, 6.1, 6.2 (quick reattach from the same device in the same login session within `terminal.quick_reattach_window`), 6.3 (limits), 10.3 (close reasons), 11.2 (escape sequences), 12 (row Mobile), 13 (problem codes and close codes), 14 (TM-T12, TM-T14, TM-T18).
- `docs/spec/08-identity.md`: 6.3 (fresh step-up, item 5 the quick reattach exception).
- `docs/design/components.md` 12 "Terminal view" (the extra keys bar row and the rules).
- `docs/security/threat-model.md` 6.7 (T-TM-03, T-TM-10) and `docs/PLAN.md` 11 "Red team" (terminal items: stolen unlocked phone, app switcher image, keyboard suggestions, screen recordings, hardware keyboard shortcuts).
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M5-12-<n>` rows) and the rows of M4-35, M4-36, M5-08.
- Briefs (merged names win, D-W-100): the briefs of M4-35 (`terminal_ui`, ports `TerminalTransport`, `HostKeyPinStore`, `StepUpPrompt`, `HostEvents`, the extra keys bar golden) and M4-36 (desktop terminal: hosts group, opening with step-up, `host_key_pins`, events, the canary test), `M2-44-terminal-sessions-relay.md` and `M2-48-terminal-conformance.md` (the host service container and test overlay), `M5-08-app-lock.md` (the `locked` event), `M5-02-iphone-layout.md` and `M5-03-android-layout.md` (phone shell, lifecycle, recent apps protection).
- Code on the base branch: `packages/terminal_ui`, `packages/bothy_noise`, `apps/client/lib` (desktop terminal of M4-36, `phone/`, `lock/`), the host service test overlay of M2-48.

This ticket touches no API operation, no table, no action, no event and no configuration key.

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
9. Windows and limits only through named configuration keys (`terminal.quick_reattach_window` is decided by the server; `client.terminal.scrollback_lines`, `client.terminal.font_size_default` from `AppConfig`); code comments name the key, never the number.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Extra keys bar with Esc, Tab, Ctrl, Alt, the four arrows, `|`, `~`, `/`, `-`; Ctrl and Alt one-shot on one tap, latched on a second tap until the next tap; labels for screen readers; a hardware keyboard hides the bar and its own keys go to the shell except the platform modifier shortcuts (01 5.3 M8, R-PR-10, R-PR-11).
12. Background, the app lock and system back on Android detach the session; nothing ends it except "End" or the server (01 5.4, 5.6 E6).
13. The app switcher image and the recent apps screen never show terminal content; while a terminal is open, screenshots and screen recordings of the app are blocked where the platform allows it (Android `FLAG_SECURE`; iOS hides the view on capture, Design note 4).
14. Reattach in the foreground: the app passes the app lock first (if on), then calls `attachTerminalSession`; within `terminal.quick_reattach_window` the server accepts without step-up, after it the app follows `401 step_up_required` with the step-up device key and biometrics (08 6.3, 24 6.2); the client never decides the window itself.
15. The terminal text field disables keyboard suggestions, autocorrection, smart punctuation and learning (`enableSuggestions: false`, `autocorrect: false`, `TextInputType.visiblePassword` or the platform equivalent; Design note 3), so no terminal input enters the keyboard's dictionary.
16. No terminal content in `local_store`, logs, crash reports, client metrics or the clipboard except by the person's explicit copy (R-CL-17, 15 23.4).
17. Close codes and their texts as 15 23.2 item 8; `4103` and `4100` never reconnect automatically.

## Scope

Create:

- `apps/client/lib/phone/terminal/`: M9 hosts and sessions (`HostsScreen`, using the controllers of M4-36), M8 session screen (`TerminalSessionScreen`) with header, session switcher, Detach and End, the extra keys bar and the input field, the mobile lifecycle handling (Design notes 1 to 5).
- `packages/terminal_ui/lib/src/extra_keys/`: `ExtraKeysBar` behaviour (one-shot and latched modifiers, key mapping to bytes, hardware keyboard detection), extending the widget of M4-35 if it exists there.
- Tests (Tests).

Change:

- `packages/terminal_ui`: the input path accepts the modifier state of the bar (Ctrl plus a letter gives the control byte, Alt gives an ESC prefix, arrows give the cursor sequences of the current mode); selection and paste confirmation for touch (long press, two-finger scroll).
- `apps/client/lib/platform/lifecycle.dart` (M5-02) and the lock controller hook (M5-08): detach on `paused` and on `locked`; reattach on `resumed` after the lock.
- iOS runner: hide the terminal view on screen capture (`UIScreen.capturedDidChangeNotification`) and while inactive; Android: `FLAG_SECURE` while a terminal is open (M5-03 Design note 5 rule extended).
- `apps/client/platform-tests.json`: the integration tests against the host service container.
- `packages/l10n` ARB files; `packages/terminal_ui/README.md`, `apps/client/README.md`; `docs/spec/15-client-architecture.md` 23.4 (the keyboard settings, the capture rule), root `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M5-12-1` to `D-M5-12-6`.

## Out of scope

- Device E2E against the Compose stack with a host service container (M5-13); the VoiceOver and TalkBack checklist for M8 (M5-13).
- Pairing and host administration on the phone (not an emergency action, R-PR-07; desktop and console).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-12-mobile-terminal`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(terminal-ui): ...`, `feat(client): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0; the host service runs as the container of the M2-48 test overlay with ports on the loopback; the fake server of M4-17 serves the REST part where the test needs no real server. Temporary directories only via `mktemp -d`, removed at the end. Stop only processes and containers you started, by PID or container ID; simulators by UDID, emulators by serial.
- Never read or print secrets; never log terminal content, a terminal session token or a Noise key.
- The integration tests need Docker for the host service container and macOS with Xcode or an Android emulator for the device side; where missing, `make apps-platform` prints its SKIP line; the report names where they ran.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- No permission logic in the client (15 1 principle 1); the client never computes the reattach window.
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Screens** (01 5.3 M8, M9). M9 reuses the host and session controllers of M4-36 (events through `sync_engine` while connected; a refresh on opening). M8 is a full-screen route with the header of components.md 12 (host, connection, badge, fingerprint start, remaining time), a session switcher instead of desktop tabs, Detach and End; the screen area uses the `terminal_ui` view; the bar sits above the keyboard inset.

2. **Extra keys bar** (01 5.3 M8; components.md 12; Ruling for docs/DECISIONS.md). State per modifier: off, one-shot (after one tap; cleared after the next key from the bar or the keyboard), latched (after a second tap within a short double-tap interval of the platform; cleared by the next tap on it). Ctrl with a letter or `@[\]^_` gives the control byte; Ctrl with other keys sends the key unmodified (and the modifier clears); Alt prefixes ESC; arrows send `ESC [ A` to `D` or `ESC O A` to `D` in application cursor mode (from the renderer's mode); the characters send themselves. Hardware keyboard: when the platform reports a connected physical keyboard (`HardwareKeyboard` events or the platform's keyboard notifications), hide the bar; keys go to the shell; the app's own shortcuts only with the platform modifier (R-PR-10).

3. **Keyboard privacy** (PLAN 11 red team "keyboard suggestions"; Ruling for docs/DECISIONS.md). The hidden input field that feeds the terminal sets `enableSuggestions: false`, `autocorrect: false`, `enableIMEPersonalizedLearning: false`, `smartDashesType: disabled`, `smartQuotesType: disabled` and a keyboard type without prediction; on Android also the IME flag for no personalized learning. Record that third-party keyboards may still log input and that the app cannot prevent it (residual risk for the threat model).

4. **Capture and app switcher** (R-PR-12; 15 23.2 item 7; Ruling for docs/DECISIONS.md). On `inactive` the privacy cover of M5-02 replaces the terminal before the snapshot. While a terminal route is visible: Android sets `FLAG_SECURE` (blocks screenshots, recordings and the recents image); iOS listens to `capturedDidChangeNotification` and `userDidTakeScreenshotNotification` and replaces the terminal surface with a notice while the screen is captured (iOS cannot block screenshots; record the residual risk).

5. **Detach and reattach** (15 23.2 items 6 and 7; 24 6.2; 08 6.3 item 5). Detach closes the stream WebSocket with `1000` and keeps the session ID; the view shows "detached". On `resumed`: if the app lock applies, wait for `unlocked`; then `attachTerminalSession` with a new ephemeral key and handshake (M4-35 flow); a `401 step_up_required` runs the step-up device key with biometrics and repeats once with the same `Idempotency-Key`; the scrollback replay replaces the screen. A failed attach shows the reason and offers "Attach" without retrying in a loop. System back on Android in M8 detaches and returns to M9.

6. **Rulings for `docs/DECISIONS.md`** (`D-M5-12-<n>`): modifier states and key mapping (note 2); keyboard privacy settings and the residual risk (note 3); capture handling per platform (note 4); detach and reattach order with the app lock (note 5); system back detaches (requirement 12); the bar hidden with a hardware keyboard (note 2).

## Tests (write them first)

Widget (`packages/terminal_ui/test/extra_keys/`, `apps/client/test/phone/terminal/`):

- `extra_keys_test.dart` (PLAN row): Ctrl one-shot then `c` sends `0x03` and clears; Ctrl latched then `c`, `d` sends `0x03`, `0x04` and stays latched until tapped; Alt then `b` sends `ESC b`; arrows in normal and application cursor mode; `|`, `~`, `/`, `-` send themselves; labels "Control", "Control, latched"; a hardware keyboard event hides the bar.
- `terminal_m8_golden_test.dart`: M8 in light and dark, text scaling 1 and 2, de and en, with the bar, detached state, the key change warning and the open dialog.
- `keyboard_privacy_test.dart`: the input field has suggestions, autocorrect and personalized learning off.
- `lifecycle_terminal_test.dart`: `paused` detaches (fake transport closed with `1000`); `locked` detaches; `resumed` waits for `unlocked`, then attaches; `401 step_up_required` runs the step-up prompt once and repeats; Android back detaches and pops to M9.
- `no_content_in_store_test.dart` (R-CL-17): a canary terminal output appears in no table of the local database, no log line of the app's logger fake and no client metric.

Integration (`apps/client/integration_test/phone/terminal_test.dart`, iOS simulator and Android emulator, host service container of M2-48, server through the test overlay):

- open a session with step-up (simulated biometric), run `echo` and see the output; Ctrl+C through the bar ends a running `sleep`.
- background the app: the session is detached on the server (`detached`); foreground within `terminal.quick_reattach_window` (short in the test configuration): attach without step-up and the scrollback replay shows the earlier output; foreground after the window: the step-up prompt appears, and after it the attach succeeds.
- the app switcher snapshot taken by the test (iOS: the cover is in the tree at `inactive`; Android: the window has `FLAG_SECURE`) contains no terminal content.

Commands that must pass, from the repository root:

- `cd packages/terminal_ui && flutter analyze --fatal-infos && flutter test --coverage`
- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on a host with Docker and macOS with Xcode or an Android emulator: `make apps-platform`; otherwise the SKIP lines
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Widget tests: Ctrl one-shot and latched, Alt, arrows and the characters of the bar; goldens of M8 in light and dark with text scaling 1 and 2.
- [ ] Integration test: the background detaches, the foreground within `terminal.quick_reattach_window` reattaches without step-up, afterwards with step-up; system back detaches on Android.
- [ ] No terminal content in the app switcher image, the recents screen, keyboard suggestions, the local database, logs or metrics.
- [ ] README files, 15 23.4, ARB files, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart symbol; comments explain why.
- `packages/terminal_ui/README.md` and `apps/client/README.md` updated in English.
- Rulings of Design note 6 recorded in `docs/DECISIONS.md` section 10; the residual risks of Design notes 3 and 4 named in the report for the threat model update at the milestone closing.
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
4. Where the integration tests ran; the residual risks for the threat model.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
