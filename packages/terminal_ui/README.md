# terminal_ui

The terminal view of every client: the renderer `xterm` behind an own escape filter, one
controller per tab that runs the flow of opening, the stream and reattaching, key pinning
through ports, the header, the tabs, the dialogs and the extra keys bar
([15 Client architecture](../../docs/spec/15-client-architecture.md), section 23;
[24 Terminal](../../docs/spec/24-terminal.md), sections 5, 8.4, 11.2 and 12).

Status: built in M4-35 (docs/decisions.d/M4-35.md D-M4-35). Kind: Flutter package, a member of the
Dart workspace at the repository root (`resolution: workspace`, `publish_to: none`).

## Dependencies

Allowed members (docs/spec/15-client-architecture.md, 3.1; `tools/arch-check/rules.yaml`):
`design_system`, `auth`, `api_client`, `bothy_noise`, `domain` (arrows of the diagram), and
through them `l10n`. Every other member is out of reach; `make arch` fails on a dependency or
an import of it. Outside the workspace: `xterm` (the renderer, see below) and the types of
`api_client` and `bothy_noise` it touches (`built_collection`, `cryptography`).

Further rules checked by `make arch`:

- never depends on `sync_engine` or `local_store`, directly or through another member (rule 7).
- no native source (rule 4).

## Ports per app

The controller sees nothing of the app but the ports of `lib/src/ports.dart` (15 23.1). Each
app implements them its own way:

| Port | Desktop and mobile app | Web console |
|---|---|---|
| `TerminalServerApi` (`getHost`, `openTerminalSession`, `attachTerminalSession`, `closeTerminalSession`) | the REST client of `api_client` | the same, with the cookie session |
| `TerminalTransport`, `TerminalChannel` (one WebSocket per stream, text until `identify`, then binary only) | own WebSocket; `identify` returns the access token and a DPoP proof with the nonce of `terminal.hello`, both from `auth` | the browser WebSocket on the same origin; the cookie session of the upgrade signs in, `identify` carries no token ([08 Identity](../../docs/spec/08-identity.md), 7.5) |
| `HostKeyPinStore` (per host `noise_public_key`, `key_generation`, fingerprint, first seen) | the encrypted table `host_key_pins` of `local_store`, deleted with all local data on sign-out | IndexedDB of the origin, deleted on sign-out ([ADR 0070](../../docs/adr/0070-terminal-in-the-web-console.md)) |
| `TerminalStepUp` (the step-up before opening and when the server asks) | `session.stepUp.confirm` of `auth` (step-up device key with biometrics or PIN) | the passkey step-up (M6) |
| `HostEvents` (changes of the host and of the own sessions; optional) | `sync_engine`: `host.updated`, `host.presence_changed`, `terminal_session.changed` | none: the console reloads through REST when the view opens and after each own action |
| `LinkOpener` (opens a confirmed `https` or `http` link outside the view) | the app's own opener | the console's own opener |
| `TerminalPrompts` (the dialogs the flow asks) | `DialogTerminalPrompts` of this package | `DialogTerminalPrompts` of this package |

## Flow

`TerminalController` runs 15 23.2 for one tab; every outcome is a phase of its state
(`TerminalTabState`), its methods never throw.

1. `open`: `getHost`, then the pin check (`checkPin`, 15 23.3). No pin: the pin is written
   after the first successful handshake. Same key: no notice. Other key: the dialog "Key
   changed" with both fingerprints; "Do not connect" opens nothing, accepting sends
   `pin_changed = true`. A host assigned to another person shows the warning of 24 8.4, before
   any step-up.
2. The dialog "Open terminal" (profile, working directory), then a fresh ephemeral X25519 key
   in memory and its `client_ephemeral_key_hash`.
3. Step-up, then `openTerminalSession` with profile, size, hash and `pin_changed`. A
   `noise_public_key` or `key_generation` in the answer other than the checked one blocks the
   tab and closes the session.
4. The stream: wait for `terminal.hello` (protocol version 1 required), send `identify`, Noise
   message 1 with the session token, check message 2, destroy the ephemeral private key, send
   `start`. Only then the tab accepts input.
5. Input goes as `data`, size changes as `resize`, at most one `resize` per frame tick. `exit`
   shows the exit code and leaves the tab open.
6. `detach` closes the stream, the session keeps running. `attach` checks the pin again, makes
   a new key and handshake and runs a step-up only when the server answers
   `401 step_up_required` (or before the request after `4101`). The replay between
   `replay_begin` and `replay_end` replaces the screen in a fresh terminal model.
7. Item 7 (mobile background) is the app's part: it calls `detach` and `attach`; the package
   has no lifecycle hook.
   `adopt` (M4-36) shows a session the server already lists as a detached tab with "Reattach",
   without a request, so an app keeps one tab per own session; `attach` then runs as above.
8. Close codes (`closeReactionFor`): `4100` ends the tab with the `close_reason`, a revocation
   blocks it; `4101`, `4102`, `4103` and a lost connection detach it with their cause; `4104`
   and every Noise or frame error block it as tampering, without a retry. A closed session in
   `terminal_session.changed` reacts like `4100`. Local work is no tampering: a failed
   `identify` (no access token or DPoP proof) detaches the tab with `authenticationFailed`, and
   a pin that cannot be stored leaves the session connected (D-M4-35-21). A refused opening or attach keeps the
   server's `reason` next to the problem code (`TerminalTabState.problemReason`), and the banner
   shows its text (`problemText`, M4-36).

## Blocks

- **Escape filter** (`EscapeFilter`, 24 11.2, SEC-TERM-6, TM-T14): on the byte stream before
  UTF-8 decoding, in front of xterm, so the locks do not depend on the renderer version. Text,
  C0 controls and plain ESC sequences pass. CSI passes except window operations and reports
  (final `t`), status and other reports (`n` other than CPR `6 n`), device attributes other
  than DA1 and DA2, `$` requests, DECREQTPARM and XTVERSION; so the answers that reach the host
  are exactly CPR and DA1/DA2. DCS, SOS, PM and APC are dropped with their content. Every OSC
  is dropped (OSC 52 clipboard, titles, color queries) except OSC 8. CSI sequences known to
  make xterm 4.0.0 throw or hang are dropped too (parameters above
  `EscapeFilter.maxParameterValue`, SGR 38/48 without sub-parameters or with an index above 255,
  REP counts above `EscapeFilter.maxRepeatCount`, `CSI 0 X`). Sequences may be
  split at any byte across frames; memory per sequence is bounded. The full allow list is the
  doc comment of `EscapeFilter`.
- **OSC 8 links**: rewritten into inert text plus a record (`TerminalLink`). A click shows the
  full target in a confirmation; only absolute `https` and `http` URIs with a host open
  (`isOpenableLink`), through `LinkOpener`.
- **Paste guard**: pasted text loses ESC and the other controls except tab and line breaks
  (they could end a bracketed paste early or run a line); text with a line break is then shown
  with its line count before it is pasted ("Paste with line breaks?"); the paste keys map to
  the guard, never to xterm's own paste.
- **Copy only by the person**: selection plus the copy shortcut (Cmd on Apple platforms,
  Ctrl+Shift elsewhere, so Ctrl+C and Ctrl+V stay with the shell). An app hands its own
  shortcuts of the terminal to `TerminalPane.shortcuts`; the renderer handles them before it
  sends a key to the shell (M4-36). The host cannot write the
  clipboard: OSC 52 is filtered and xterm has no clipboard handler.
- **No content in logs** (R-CL-17): the filter logs nothing and keeps no content beyond the
  current sequence; errors carry codes only; a renderer exception is caught and dropped without
  its text; the scrollback lives in memory only (`client.terminal.scrollback_lines`).
- **Second layer**: the screen (`TerminalScreen`) leaves `onTitleChange`, `onIconChange` and
  `onPrivateOSC` of xterm unset, so a sequence the filter missed changes nothing and answers
  nothing.

## Renderer xterm

Assessed on 2026-10-20 ([24 Terminal](../../docs/spec/24-terminal.md), 12): `xterm` 4.0.0 of
TerminalStudio, MIT, published 2024-02-27, the latest release; repository not archived, no
known vulnerability in OSV or the GitHub advisory database, no security report among its
issues. What xterm itself does with hostile output (read in its source):

- OSC 0/1/2 call `onTitleChange` and `onIconChange`; every other OSC, OSC 52 and OSC 8
  included, goes to `onPrivateOSC`; there is no clipboard handler. All three stay unset here.
- It answers DA1, DA2, DA3, DSR 5, CPR (zero-based, a bug: `ESC[0;0R` at the home position;
  the screen adds one to row and column) and
  `CSI 18 t` through `onOutput`; `CSI 8;r;c t` resizes the terminal. The filter drops all of
  these but CPR, DA1 and DA2.
- DCS is not parsed; its content would be printed as text. The filter drops it.
- Known bugs: palette index 15 (SGR 97) resolves to white instead of bright white; selection
  and block cursor are painted over the text, underline and bar cursor in row 0;
  `TerminalView.onTapUp` is never called (the view
  detects link clicks with its own pointer listener); several sequences throw or hang (see the
  escape filter above).

The theme comes from the tokens: `buildTerminalTheme` maps `DsColors` `color.terminal.*` to
xterm's `TerminalTheme`. xterm's cursor and selection are transparent; `TerminalMarksPainter`
paints both over the view, opaque in the pairings of components.md 12: the cursor cell in
`cursor` with its character in `cursor-text`, a selected cell in `selection-bg` with its
character in `fg`, and a frame in `cursor` without the focus (15 23.4, D-M4-35-18).

## Layout

| Path | Content |
|---|---|
| `lib/terminal_ui.dart` | the public library; every public symbol has a doc comment (`make lint`, `public_member_api_docs`) |
| `lib/testing.dart` | test support: `FakeHost` (answers the handshake with `NkResponder`) and `FakeTerminalTransport`; import only from tests |
| `lib/perf_hooks.dart` | the test-only timing hooks of the performance level (`TerminalPerfHooks`, `TerminalPerfProbe`, brief M4-38); they exist only with `--dart-define=TERMINAL_PERF_HOOKS=true` |
| `lib/src/ports.dart` | the ports of 15 23.1 |
| `lib/src/controller/` | `TerminalController`, the tab state, the close reactions, the frame pump (`data` and coalesced `resize`), the prompts interface |
| `lib/src/pinning/` | the pin check of 15 23.3 |
| `lib/src/render/` | the escape filter, the screen (xterm model behind the filter, link records, replay swap), the view, the theme, the paste guard, the semantics |
| `lib/src/widgets/` | pane, header, tabs, states, extra keys bar and the dialogs (open, key changed, foreign host, link, the `TerminalPrompts` with dialogs) |
| `test/` | unit, widget and golden tests: the filter, the controller flow, the ephemeral key, the theme, the paste guard, the extra keys bar, the semantics, no terminal content in logs, TM-T14, TM-T18 and the client part of TM-T26; `test/support/` holds the fakes of the ports and widget helpers, `test/goldens/` the golden images |

## Timing hooks of the performance level

The desktop app measures `client.perf.terminal_frame_to_paint_p95` and
`client.perf.terminal_throughput_min` (15 19) on the real path, so the controller and the screen
carry two test-only hooks (brief M4-38): the controller reports each decrypted `data` frame
(`TerminalPerfProbe.dataDecrypted`, right before the screen or the replay buffer takes it) and the
screen view reports the first frame after new output once its post-frame callbacks run
(`frameDrawn`); times are microseconds of `Timeline.now`, the clock of the engine's `FrameTiming`.
Both calls sit behind the constant `terminalPerfHooksEnabled`
(`bool.fromEnvironment('TERMINAL_PERF_HOOKS')`), so a build without the define compiles them out,
and `TerminalPerfHooks.probe` refuses a probe there. `test/perf_hooks_test.dart` holds both: run
plainly it checks that the hooks are absent and that every call site sits behind the guard; with
`--dart-define=TERMINAL_PERF_HOOKS=true` it checks the reports.

## Tests

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)). In the package
alone: `flutter test` or `flutter test test/<file>_test.dart`, and
`flutter analyze --fatal-infos`.
