# M4-13: Sync engine: connection and session

Status: ready | Depends on: M4-09, M4-12 | Milestone: M4

## Goal

After this ticket `packages/sync_engine` holds the gateway connection of the apps: a `Connection` that opens the WebSocket through a transport port, reads and writes the frames `{op, d}` of 04 2.3 with the generated gateway models of `api_client`, sends heartbeats with a random first delay and closes itself when an acknowledgement is missing, and reconnects with full-jitter backoff that a network change or a return to the foreground cuts short; a `Session` that answers `hello` with `identify` or `resume` (DPoP proof for the gateway with the nonce of `hello`, cursor and sync epoch from `local_store`), takes `ready` and `resumed`, renews the token on `reauth_required` through `auth`, and reacts to every close code of 04 6.4 as the table and 15 7.2 say; the state machine of 15 7.2 as an observable `ConnectionState`; and a `FocusManager` and typing throttle for the outgoing ephemeral ops. Catch-up, the dispatcher and the outbox follow in M4-14 and M4-15 and plug in through a listener interface defined here. Every close code has one unit test with a fake transport and a fake clock. Why: every live update and every catch-up of the apps runs over this connection, and a wrong reaction to a close code either loses data (no resume), floods the server (no backoff) or keeps a revoked device connected (`4012`).

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-13 to M4-17.
- `docs/spec/15-client-architecture.md`: 1 (principle 5), 3.1 (`sync_engine` depends on `local_store`, `auth`, `api_client`, `domain`; no Flutter), 7.1 "Building blocks" (rows `Connection`, `Session`, `FocusManager`, `EphemeralStore`), 7.2 "Connection states" (diagram and the text on `4004`, `4018`, `4006`, `Suspended`), 8 (gateway proof in `identify`), 21 (R-CL-03).
- `docs/spec/04-sync-protocol.md`: 2.3 (frames, `sync.ws_max_frame_bytes`), 6 "WebSocket gateway" (6.1 one connection per device and `4017`, 6.2 ops, 6.3 lifecycle items 1 to 8, 6.4 close codes and the mapping table, 6.5 reconnecting), 7.5 (epoch sent with the cursor), 11.3 item 2 (focus limit), 12 item 1 (typing throttle and `typing.stop`), 18 (the `sync.*` values).
- `docs/api/asyncapi.yaml`: channel `gateway` (messages, `x-close-codes`, `info.x-protocol-versions`); `docs/api/openapi.yaml` schemas `GatewayHello`, `GatewayIdentify`, `GatewayClientInfo`, `GatewayReady`, `GatewayResume`, `GatewayResumed`, `GatewayHeartbeat`, `GatewayHeartbeatAck`, `GatewayReauth`, `GatewayReauthRequired`, `GatewayFocus`, `GatewayTyping`, `GatewayError`, `Discovery.gateway`.
- `docs/spec/08-identity.md` 7.3 (DPoP nonce rules for the gateway).
- `docs/DECISIONS.md`: D-W-100, D-W-114, D-W-115 (the client reads `sync.reconnect_backoff_*`, `sync.typing_min_interval` and `sync.max_focused_threads` through `AppConfig`); section 10.
- Briefs: `docs/briefs/M4-09-auth-dpop.md` (`DpopProofFactory.forGateway`, `RefreshCoordinator.refreshNow`, `AuthEvent`, the `Clock` port), `docs/briefs/M4-12-local-store.md` (`SyncStateDao`), `docs/briefs/M4-02-dart-api-client.md` (generated models, `decodeEnvelope`), `docs/briefs/M4-08-domain.md` (`AppConfig`, `GlobalSeq`); for the server side `docs/briefs/M1-64-gateway-connection.md` and `docs/briefs/M1-65-gateway-delivery.md` (what the server sends and when it closes).

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
9. Intervals only from the server or through named keys: heartbeat interval and resume window from `hello`; the `4008` wait from the close reason; backoff from `sync.reconnect_backoff_base` and `sync.reconnect_backoff_cap`, typing throttle from `sync.typing_min_interval`, focus limit from `sync.max_focused_threads`, all through `AppConfig` (D-W-115). No literal duration in code except in tests.
10. Specification first: 15 7.2 does not draw `4016`, `4017` and `4008`; this ticket follows Design note 3 and adds the transitions to the diagram of 15 7.2 in the same change.

Product requirements implemented here:

11. One connection per device; the client never opens a second one while one is open (04 6.1).
12. `identify` carries `access_token`, `dpop` (`htm = GET`, `htu` = HTTPS form of the gateway URL, `nonce` from `hello`, `ath`), `device_id`, `protocol_version`, `cursor` and `sync_epoch` (both absent on the first connection) and `client`; the proof is in the frame, never in upgrade headers (04 6.3 item 3, R-SY-01).
13. `resume` within `resume_window_ms` after a disconnect with `gateway_session_id`, token, proof, cursor and epoch; `4006` falls back to `identify` (04 6.3 item 7).
14. Heartbeat every `heartbeat_interval_ms` with `last_seq`, the first one delayed by a random fraction; no `heartbeat_ack` within one interval closes and reconnects (04 6.3 item 5).
15. Backoff before attempt `n` uniform in `[0, min(cap, base * 2^n)]`; reset after `ready` or `resumed`; a network change or the foreground triggers an immediate attempt without raising `n` (04 6.5).
16. Every close code reacts as 04 6.4 says; `4012` signs out and requests the deletion of local data (R-SY-14); `4007` stops for an app update; `4018` and `4017` do not reconnect on their own.
17. Focus: at most `sync.max_focused_threads` threads, the complete set per frame, with `idle`; typing: `typing.start` at most once per `sync.typing_min_interval`, `typing.stop` resets the throttle (04 11.3, 12).

## Scope

Create in `packages/sync_engine/lib/src/`:

- `ports.dart`: `GatewayTransport` (`Future<GatewayChannel> connect(Uri url)`), `GatewayChannel` (`Stream<String> get frames`, `void send(String text)`, `Future<void> close(int code, [String? reason])`, `int? get closeCode`, `String? get closeReason`), `NetworkStatus` (`Stream<bool> get changes`, `bool get online`), `AppLifecycle` (`Stream<AppActivity> get changes` with `foreground`, `background`), `RandomSource`; the `Clock` port is the one of `auth` (M4-09).
- `transport_io.dart`: `IoGatewayTransport` on `web_socket_channel` (the dart:io implementation). The client sends small frames only; the server enforces `sync.ws_max_frame_bytes` (`4002`), so the client has no size check of its own.
- `frames.dart`: encode and decode `{op, d}` with the generated models; unknown ops and unknown fields are ignored (04 6.1: changes within a version are additive).
- `connection_state.dart` (`ConnectionState` with `phase` and `reason`, Design note 3), `backoff.dart`, `heartbeat.dart`, `session.dart`, `connection.dart` (`GatewayConnection` orchestrating them), `focus_manager.dart`, `typing_throttle.dart`, `session_listener.dart` (Design note 5).
- `lib/sync_engine.dart` exports; tests under `test/connection/` (Tests).

Change:

- `packages/sync_engine/pubspec.yaml` (`web_socket_channel`, `api_client`, `auth`, `domain`, `local_store`), `README.md` (states, ports, close code table, what M4-14 and M4-15 plug in).
- `docs/spec/15-client-architecture.md` 7.2 (diagram: `4008`, `4016`, `4017` and the reason field, Design note 3).
- `docs/DECISIONS.md` section 10: rows `D-M4-13-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Applying `ready.snapshot`, catch-up pages, `cursor` frames, `catchup.complete`, `resync_required`, the dispatcher and the `EphemeralStore` (M4-14).
- The outbox (M4-15) and the run documents (M4-16).
- The fake server (M4-17) and the app wiring of network status and lifecycle (M4-18).
- The terminal stream (15 8: never through `sync_engine`; M4-36).
- The statistics subscription `stats.subscribe` (M6).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-13-sync-connection`; one writer per worktree.
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

1. **Frames.** `GatewayFrame` sealed class with one subclass per op of 04 6.2 that a person's client sends or receives (`identify`, `resume`, `heartbeat`, `reauth`, `focus`, `typing.start`, `typing.stop`; `hello`, `ready`, `resumed`, `dispatch`, `cursor`, `catchup.complete`, `resync_required`, `heartbeat_ack`, `reauth_required`, `error`), each wrapping the generated model; `dispatch` keeps the raw envelope map and hands it to `decodeEnvelope` of M4-02 only in M4-14. Outgoing JSON contains no `null` (04 2.2). A frame from the server that is not valid JSON or has no `op` is counted as a decode error (logged without content) and skipped; the client keeps the connection, because closing with a protocol code is the server's role (04 6.4).

2. **Session flow.** After `hello`: check that the client's protocol version (`1`, the value of `info.x-protocol-versions`) is in `protocol_versions`, otherwise go to `UpdateRequired` without sending; store the nonce in the `NonceStore` of `auth`; within `sync.identify_timeout` (the client sends at once) send `resume` when `sync_state.gateway_session_id` exists and `now - disconnected_at < resume_window_ms`, otherwise `identify`. `ready`: store `gateway_session_id`, `permission_version` and `head_seq` in `sync_state` (through `SyncStateDao`) and pass the frame to the listener; the `sync_epoch` of `hello` and `ready` is kept in memory and handed to the listener, never stored apart from the cursor (04 7.5 item 2: cursor and epoch are stored together, by M4-14 in the transaction of a page or snapshot); `resumed`: the same without a snapshot. `reauth_required`: `RefreshCoordinator.refreshNow()`, then `reauth` with the new token and a fresh gateway proof with the current nonce (the one `reauth_required` carries if its schema has one, otherwise the newest of the `NonceStore`); a failed refresh that ends in `signedOut` closes with `1000` and goes to `SignedOut`. On every disconnect store `disconnected_at`.

3. **States and reasons.** `ConnectionPhase`: `offline`, `connecting`, `identifying`, `catchingUp`, `live`, `resyncing`, `backoff`, `suspended`, `signedOut`, `updateRequired` (15 7.2). `ConnectionState(phase, reason)` with `reason` one of `none`, `networkUnavailable`, `devicePending`, `deviceReplaced`, `forbidden`, `rateLimited`, `revoked`, `protocolUnsupported`, `closedByServer(code)`. Transitions for the codes the diagram omits: `4016` goes to `offline` with `devicePending` and reconnects only after `LoginService.pollPendingDevice` (M4-09) reports `active`; `4017` goes to `offline` with `deviceReplaced` and reconnects only on `AppActivity.foreground` or `GatewayConnection.connectNow()` (user action); `4008` goes to `backoff` and waits for the time in the close reason, which the server writes as `retry_after=<seconds>` (M1-64; if absent or malformed, the normal backoff) instead of the jitter; `4018` goes to `offline` with `forbidden` (no token renewal). `catchingUp`, `resyncing` and `live` are set by the listener of M4-14 through `SessionControl` (note 5); without a listener, `ready` leads straight to `live`. Desktop (`keepConnectedInBackground: true`) never enters `suspended`; mobile closes with `1000` on background and resumes on foreground (15 7.2). Ruling for docs/DECISIONS.md; draw the transitions in 15 7.2.

4. **Close code table.** One function `CloseReaction reactTo(int code, String? reason)` that returns the next phase, whether to `resume` or `identify`, whether to renew the token first, whether to wait for backoff or reconnect at once with a random delay (`4013`: uniform in `[0, base]`), and whether to emit `signOut(deleteLocalData: true)` (`4012`). `1000` from the server reconnects only in the foreground; `1001`, `4000`, `4001`, `4002`, `4003`, `4005`, `4009`, `4014` back off and try `resume`; `4015` (agents only, 04 6.4) does not reconnect and is reported as an unexpected code; `4004` renews the token once (`refreshNow`) and reconnects at once, and on a failed refresh signs out; `4011` renews the token and resumes; an unknown code in `4000` to `4099` backs off. The table is data, so the test iterates it.

5. **Listener and control.** `abstract interface class SessionListener { Future<void> onReady(GatewayReady ready, SessionControl control); Future<void> onResumed(GatewayResumed resumed, SessionControl control); Future<void> onFrame(GatewayFrame frame, SessionControl control); void onDisconnected(); }` and `SessionControl` (`setPhase`, `send`, `lastProcessedSeq` setter for the heartbeat). M4-14 implements the listener; the tests here use a recording listener. The heartbeat sends `last_seq` from `lastProcessedSeq` (the stored cursor until M4-14 sets it).

6. **Focus and typing.** `FocusManager.setVisible(Set<ThreadId> ids, {required bool idle})` sends one `focus` with the complete set after a change (coalesced per microtask), never more than `sync.max_focused_threads` IDs (the most recently shown win; the rest stays unfocused and the caller is told through the return value); re-sends the current set after `ready` (on `resumed` the server keeps it). An `error` frame with `validation_failed` for focus keeps the previous set. `TypingThrottle.onInput(ThreadId)` sends `typing.start` at most once per interval; `onCleared` and `onSent` send `typing.stop` and reset.

7. **Rulings for docs/DECISIONS.md** (`D-M4-13-<n>`): the reasons and the transitions of note 3; the close code table as data (note 4); immediate attempt on network change without raising the counter (04 6.5); ignoring unknown ops and fields; focus overflow rule (note 6).

## Tests (write them first)

`packages/sync_engine/test/connection/` (fake transport that records frames and can close with any code and reason, fake clock with `elapse`, seeded random source, fake network status and lifecycle, `auth` with a software P-256 signer in this package's `test/support/` (the pattern of M4-09; test folders cannot be shared between packages) and a fake API, in-memory `local_store`):

- `close_codes_test.dart`: one test per code of 04 6.4 (`1000`, `1001`, `4000` to `4009`, `4011` to `4018`) that drives a live connection, closes with the code and asserts the next phase and reason, the next frame sent (`resume` or `identify` or none), the waiting time before it (zero, jitter bounds, the `4008` waiting time from `retry_after=<seconds>`) and the side effects (refresh called once for `4004` and `4011`, sign-out with deletion for `4012`, no reconnect for `4007` and `4015`, `4017` waits for foreground, `4018` waits for user action, `4016` waits for the pending device poll).
- `session_test.dart`: first connection sends `identify` without cursor and epoch; a later one with both; within the resume window `resume`, after it `identify`; `4006` after `resume` leads to `identify`; the gateway proof uses the `hello` nonce, `htm` `GET` and the HTTPS form of the URL; a `hello` without the client's protocol version leads to `updateRequired` without sending; `reauth_required` refreshes and sends `reauth`.
- `heartbeat_test.dart`: the first heartbeat waits a random fraction (bounds checked with two seeds), later ones every interval with `last_seq`; a missing `heartbeat_ack` closes the socket with `1000` from the client side and reconnects with backoff and `resume`.
- `backoff_test.dart`: waits stay within `[0, min(cap, base * 2^n)]`; reset after `ready`; a network change and a foreground event connect at once without raising `n`.
- `focus_typing_test.dart`: more IDs than the limit are cut; one frame per change; focus re-sent after `ready`; typing throttled and reset by `typing.stop`.
- `frames_test.dart`: unknown op and unknown fields ignored; outgoing frames contain no `null`.

Commands that must pass, from the repository root:

- `cd packages/sync_engine && dart test --coverage=coverage` (or the command M4-01 uses for pure Dart members)
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`, `make osv`
- `make check`

## Acceptance criteria

- [ ] One unit test per close code of 04 6.4 with a fake transport and a fake clock, each asserting the reaction of the table.
- [ ] `identify` and `resume` carry the fields of 04 6.3 with a correct gateway proof; `4006` falls back to `identify`; `4012` signs out with deletion.
- [ ] Heartbeat with a random first delay; missing acknowledgement reconnects; backoff with full jitter, reset and immediate attempts as 04 6.5 says.
- [ ] The phases of 15 7.2 are observable; 15 7.2 shows the added transitions.
- [ ] Focus and typing follow their limits from `AppConfig`.
- [ ] `sync_engine` imports no Flutter (`make arch`); rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 04 or 15.
- `packages/sync_engine/README.md` (English) as in Scope.
- 15 7.2 updated; rulings of Design note 7 in `docs/DECISIONS.md` section 10.
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
4. The close code table as implemented (code, phase, reason, next frame, wait), and every place where the server behaviour of M1-64 or M1-65 on the base branch differed from 04 6.4.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
