# sync_engine

Gateway connection, catch-up, outbox, reconciliation, run documents and focus, without Flutter.

Status: the gateway connection with focus and typing (M4-13) and the run documents of the live
box (M4-16) are built; catch-up and the outbox follow with M4-14 and M4-15. Kind: pure Dart package, a member of the Dart
workspace at the repository root (`resolution: workspace`, `publish_to: none`).

## Dependencies

Allowed members (docs/spec/15-client-architektur.md, 3.1; `tools/arch-check/rules.yaml`): `local_store`, `auth`, `api_client`, `domain` (arrows of the diagram).
Every other member is out of reach; `make arch` fails on a dependency or an import of it.

Further rules checked by `make arch`:

- no import of `package:flutter/`, `dart:ui` (rule 1).
- no native source (rule 4).

## Gateway connection (M4-13)

`GatewayConnection` holds the one WebSocket of the device (04 6.1): it never opens a second
socket while one is open or opening. It answers `hello` through `GatewaySession` with `resume`
(stored gateway session and cursor, last disconnect within `resume_window_ms`) or `identify`
(cursor and epoch only when stored), both with a gateway DPoP proof of `auth`
(`DpopProofFactory.forGateway`: `htm` `GET`, `htu` the HTTPS form of the gateway URL, the
`hello` nonce, `ath`). `ready` stores `gateway_session_id`, `permission_version` and `head_seq`
through `SyncStateDao`; the `sync_epoch` stays in memory, because M4-14 stores it with the
cursor. `reauth_required` renews the token (`RefreshCoordinator.refreshNow`) and sends `reauth`
with a proof over the nonce of `reauth_required`. Heartbeats go every `heartbeat_interval_ms`,
the first after a random fraction; a missing `heartbeat_ack` within one interval closes with
`1000` and reconnects. Waits are full jitter in `[0, min(cap, base * 2^n)]`
(`sync.reconnect_backoff_base`, `sync.reconnect_backoff_cap`), reset after `ready` or
`resumed`; a network change or the foreground starts an attempt at once without raising `n`.
A server frame that does not decode is counted (`decodeErrors`) and skipped; unknown ops and
fields are ignored.

### States

`connection.states` publishes `ConnectionState(phase, reason)`. Phases (15 7.2): `offline`,
`connecting`, `identifying`, `catchingUp`, `live`, `resyncing`, `backoff`, `suspended`,
`signedOut`, `updateRequired`. Reasons: `none`, `networkUnavailable`, `devicePending`,
`deviceReplaced`, `forbidden`, `rateLimited`, `revoked`, `protocolUnsupported`,
`closedByServer(code)`. On mobile the connection closes with `1000` in the background
(`suspended`) and resumes in the foreground; with `keepConnectedInBackground` (desktop) it
stays connected; no reconnect path (backoff timer, token renewal, approved device) opens a
socket in the background on mobile.

### Close codes

One table, `closeCodeTable`, resolved by `reactTo(code, reason, foreground:)` (docs/DECISIONS.md
D-M4-13-2). "Next frame" is the answer to the next `hello`.

| Code | Phase, reason | Next frame | Wait |
|---|---|---|---|
| `1000` | foreground: `backoff`; background: `offline`, `closedByServer(1000)` | `resume` | backoff; in the background until the foreground |
| `1001`, `4000` to `4003`, `4005`, `4009`, `4014` | `backoff`, `closedByServer(code)` | `resume` | backoff |
| `4004` | `backoff` after one `refreshNow`; failed refresh: `signedOut` | `resume` | none (a second renewal before `ready` or `resumed`: backoff) |
| `4006` | `backoff` | `identify` | backoff |
| `4007` | `updateRequired`, `protocolUnsupported` | none | never |
| `4008` | `backoff`, `rateLimited` | `resume` | `retry_after=<seconds>` of the reason, else backoff |
| `4011` | `backoff` after one `refreshNow`; failed refresh: `signedOut` | `resume` | none (a second renewal before `ready` or `resumed`: backoff) |
| `4012` | `signedOut`, `revoked`; `AuthSession.sessionRevoked` (`SignedOut.deleteLocalData`) | none | never |
| `4013` | `backoff`, `closedByServer(4013)` | `resume` | uniform in `[0, base]`, counter unchanged |
| `4015` | `offline`, `closedByServer(4015)`, logged as unexpected | none | `connectNow` |
| `4016` | `offline`, `devicePending` | `resume` | until `pollPendingDevice` reports `active` |
| `4017` | `offline`, `deviceReplaced` | `resume` | foreground or `connectNow` |
| `4018` | `offline`, `forbidden`; no token renewal | `resume` | `connectNow` |
| other `4000` to `4099`, other codes, a drop | `backoff` (a drop without network: `offline`, `networkUnavailable`) | `resume` | backoff |

### Ports

| Port | Production implementation | Tests |
|---|---|---|
| `GatewayTransport`, `GatewayChannel` | `IoGatewayTransport` on `web_socket_channel` (this package) | a fake transport that records frames and closes with any code |
| `NetworkStatus` | connectivity of the platform (M4-18) | a switchable fake |
| `AppLifecycle` | lifecycle of the Flutter app (M4-18) | a switchable fake |
| `RandomSource` | `SystemRandomSource` (`Random.secure`) | a seeded source |
| `Clock` (of `auth`) | `SystemClock` | a clock with `elapse` |

### What M4-14 and M4-15 plug in

`SessionListener` (`onReady`, `onResumed`, `onFrame`, `onDisconnected`) receives the frames in
order (the control frames `heartbeat_ack` and `reauth_required` act outside that queue, so a long
listener step cannot delay them); M4-14 applies the snapshot, catch-up pages, `cursor`, `catchup.complete` and
`resync_required`, and sets `catchingUp`, `resyncing` or `live` through `SessionControl`,
which also sends frames and sets `lastProcessedSeq` for the heartbeat. M4-15 (outbox) reads
`connection.states` to know when the gateway is live. `connection.focus` (`FocusManager`, at
most `sync.max_focused_threads`, the complete set per frame with `idle`) and
`connection.typing` (`TypingThrottle`, `sync.typing_min_interval`) are called by the interface.

## Run documents (M4-16)

`RunStore` keeps the document (`run_document.v1`, API schema `Run`) of every focused run in
memory; run documents never go into the local database (docs/spec/15 5).

Client rule (docs/spec/04 11.3 item 4) per `run.updated` frame of a focused run, with `local`
the version of the held document:

| Frame | Result |
|---|---|
| `version` not newer than `local` | ignored |
| `snapshot` | replaces the document (04 11.3 item 3) |
| `base_version` equal to `local`, with `ops` | `applyRunPatch` applies the ops |
| anything else: a gap, a truncated frame (only `run_id` and `version`), no document yet, an invalid patch | refetch of the whole document; it replaces the local one only when newer |

While a refetch of a run is in flight, frames with ops of that run wait in a queue and run
through the rule again when it returns; frames not newer than the fetched version drop out. A
failed fetch keeps the last good document and retries on the next frame or `focus`. A fetch
that returns after `unfocus` or `onRunEnded` is ignored. `applyRunPatch` supports exactly
RFC 6902 `add`, `replace` and `remove` with RFC 6901 pointers, copies only the touched branches,
and throws `RunPatchError` for anything else; the store never applies a patch in part.

`watch(runId)` publishes at most one state per frame, the newest one (15 7.4). `RunDocument`
reads the live box fields of 09 5.4 (state, metrics, cost, plan, steps in `ordinal` order with
children, open subagents, current tool, follow-ups, `stall_warned_at`, times) and keeps unknown
fields in `raw`.

Ports (no transport dependency):

| Port | Production implementation (M4-17) | Tests |
|---|---|---|
| `RunDocumentFetcher.fetchRun(RunId)` | `GET /api/v1/runs/{id}` (`getRun`) through `api_client`, decoded body | a fake that counts calls |
| `FrameTicker.requestFrame(callback)` | `SchedulerBinding.instance.scheduleFrameCallback` | a manual ticker |

Wiring by M4-17: the dispatcher of M4-14 passes the decoded payload of every `run.updated` to
`RunStore.onRunUpdated`; the focus manager calls `focus` and `unfocus` for the runs of focused
threads; `run.state_changed` to an end state calls `onRunEnded`, after which the collapsed row
comes from the table `runs`. A `Notifier` per run listens to `watch`.

## Layout

| Path | Content |
|---|---|
| `lib/sync_engine.dart` | the public library; every public symbol has a doc comment (`make lint`, `public_member_api_docs`) |
| `lib/src/*.dart` | the gateway connection: ports, `dart:io` transport, frames, states, close table, backoff, heartbeat, session, connection, focus, typing, listener (M4-13) |
| `lib/src/runs/` | `RunStore`, `applyRunPatch`, `RunDocument` and the ports (M4-16) |
| `test/connection/` | tests of the gateway connection, one per close code, with the fakes of `test/support/` |
| `test/runs/` | tests of the run documents, fixtures of a document at versions 1 to 4 under `fixtures/` |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`) |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
