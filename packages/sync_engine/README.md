# sync_engine

Gateway connection, catch-up, outbox, reconciliation, run documents and focus, without Flutter.

Status: the gateway connection with focus and typing (M4-13), catch-up and reconciliation
(M4-14) and the run documents of the live box (M4-16) are built; the outbox follows with M4-15. Kind: pure Dart package, a member of the Dart
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

## Catch-up and reconciliation (M4-14)

`CatchUpListener` is the `SessionListener` of the connection (15 7.3, 04 7):

- `ready` without a stored cursor (the first connection, or a cursor dropped by a failed
  resync) empties the mirror and applies the whole snapshot in one transaction with
  `cursor = head_seq` and the epoch, then goes `live` at once (no catch-up follows an `identify`
  without a cursor); with a cursor only what is not an event (counters, agent
  presence, feature flags, active run summaries) and the catch-up delivers the rest.
- A gateway page is the run of `dispatch` frames up to the next `cursor` frame or
  `catchup.complete`; it is applied with `SyncStateDao.commitPage` and the cursor of that frame
  (never below the stored one) and the epoch. Pages above `client.sync.parse_offload_bytes`
  decode in a background isolate (`decodeEnvelopes`). A failing page leaves rows and cursor as
  they were and the listener applies nothing more until the next `ready` or `resumed`.
- After `catchup.complete` the phase is `live` and `onLive` runs; every live durable event is its
  own transaction with `cursor = seq`; ephemeral events go to the `EphemeralStore` (typing capped
  at `sync.typing_ttl`, presence seeded from `ready`) or raw to `onRunUpdated`.
- `resync_required` (any reason) keeps the connection: `getSnapshot`, then in one transaction
  `clearMirror` (outbox, drafts and share inbox stay) and the snapshot with its cursor and epoch;
  the threads in focus load their newest page; phase `live`, notice `Resynced`. Frames that
  arrive meanwhile wait and pass the duplicate rule afterwards. When the snapshot cannot be
  loaded, mirror and cursor stay and the snapshot is retried after the reconnect backoff on the
  same connection.
- REST reads after a commit store nothing for a target removed meanwhile (the channel row must
  exist and no removal of the channel, its space or the thread may have happened).
- `access.changed` stores `permission_version`; `removed` deletes the targets with
  `removeTargets` inside the page (rows, files, search entries, drafts; outbox entries stay),
  then deletes the file bytes through `FileCacheEraser` and emits `TargetsRemoved`; `added`
  loads the structure through `Fetcher.loadStructure`; `changed` reloads the view with the new
  level and capabilities. The rest of the page applies after it.
- `4012` (`ConnectionState` `signedOut`/`revoked`, watched with `watchConnection`) and
  `session.revoked` of this device's session wipe the database, stop the page and emit
  `SignedOutNotice(deleteLocalData: true)`; `session.revoked` of another session of the person
  changes nothing.
- A new message that does not continue the loaded range of its thread is stored, the gap is
  recorded and loaded with `Fetcher.range`; `MessagesDao.watchContiguous` hides it until then. A
  failed fill stays recorded; `onFocus` retries it.

### Appliers

Rule "version": `resource.version` must be greater than the stored version (the conditional
upsert of `local_store`); "seq": no `resource.version`, applied in `seq` order with absolute
values (04 9 item 3). A `truncated` durable event loads its message, thread, channel or space
through the `Fetcher` instead.

| Event type | Local tables | Rule |
|---|---|---|
| `message.created`, `message.updated` | `messages`, `principals` (author), `files`, `message_attachments`, `reactions_summary`, `message_components`, `my_mentions`, `search_index`; `loaded_ranges` for `created`; a tombstone clears content and search row | version |
| `message.deleted` | `messages` (tombstone), `reactions_summary`, `my_mentions`, `message_components`, `message_attachments`, `search_index` | version |
| `reaction.added`, `reaction.removed` | `reactions_summary` | seq |
| `pin.added`, `pin.removed` | `messages.pinned` | seq |
| `thread.created`, `thread.updated`, `thread.closed`, `thread.reopened`, `thread.moved` | `threads`, `thread_tags`, `search_index` (title) | version |
| `thread.moved_out` | everything of the thread (`removeTargets`) when it is still in the source channel | seq |
| `space.created`, `space.updated`, `space.archived` | `spaces` | version |
| `channel.created`, `channel.updated`, `channel.archived` | `channels`, `channel_operators` | version |
| `run.state_changed` | `runs` | version |
| `user.updated` | `principals` | seq |
| `agent.updated` | `agents`, `principals` | version |
| `read_state.updated` | `read_states` (`mergeReadState`), counters | version of the read state |
| `draft.updated` | `drafts` (not the echo of this device, not while typed here) | version |
| `draft.deleted` | `drafts` (not while typed here) | seq |
| `settings.updated` | `user_settings` (`deleted: true` removes the key) | version of the setting |
| `sidebar.updated` | `user_space_states` | version of the state |
| `notification_pref.updated` | `notification_preferences` | version of the preference |
| `quiet_hours.updated` | `quiet_hours_rules` (rules missing from the payload are deleted) | version per rule |
| `file.updated` | `files` | seq |
| `command.changed` | `commands` (`removed` deletes) | version |
| `instance.updated` | `instance` (reloaded with `getInstance`) | seq |
| `access.changed` | `sync_state.permission_version`, removal, REST structure | seq |
| `session.revoked` | the whole database (wipe) | seq |

In memory: `typing.started` and `agent.presence_changed` (`EphemeralStore`), `run.updated`
(`onRunUpdated`, raw, also when truncated). Skipped, the cursor advances (no local table):
`access.expiring`, `agent.config_changed`, `agent.followup`, `agent.invocation`,
`agent_session.changed`, `channel.settings_changed`, `command.invoked`, `gdpr.request_updated`,
`grant.changed`, `group.changed`, `host.updated` (M4-36), `import.progress`,
`interaction.created`, `run.cancel_requested`, `status.updated`, `terminal_session.changed`
(M4-36), and every type the catalogue does not know. `test/catch_up/catalogue_test.dart` holds
these sets equal to the catalogue.

### Ports and hooks

| Port or hook | Production (M4-17, M4-18, M4-22) | Tests |
|---|---|---|
| `Fetcher` | `ApiFetcher` over `api_client` (this package) | `ApiFetcher` on a fake REST adapter with recorded responses |
| `LocalIdentity` | `AuthIdentity` over the `AuthSession` | `FixedIdentity` |
| `FileCacheEraser` | the file cache of M4-22 | a recording eraser |
| `DraftEditing` | the composer of the app | a switch |
| `onEcho`, `onRunUpdated`, `onLive` | outbox (M4-15), `RunStore.onRunUpdated` (M4-16), outbox start | recorders |

Notices on `notices`: `TargetsRemoved`, `DraftOverwritten` (from `applyDraftPutResult`, the
answer of `putThreadDraft`), `Resynced`, `SignedOutNotice`. Counters: `countsOf(threadId)` and
`countChanges`, reset from each snapshot and `read_state.updated`, counted from `message.*` in
between (04 14).

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
| `lib/src/catch_up.dart`, `dispatcher.dart`, `appliers/`, `snapshot_applier.dart`, `fetcher.dart`, `gap_filler.dart`, `ephemeral_store.dart`, `sync_events.dart`, `isolate_decode.dart`, `wire.dart` | catch-up, dispatcher, appliers per resource group, REST reads, gaps, ephemeral store, notices and ports, decoding, the wire mappers (M4-14) |
| `lib/src/runs/` | `RunStore`, `applyRunPatch`, `RunDocument` and the ports (M4-16) |
| `test/connection/` | tests of the gateway connection, one per close code, with the fakes of `test/support/` |
| `test/catch_up/` | tests of the catch-up on the connection fakes, an in-memory store and recorded REST responses |
| `test/runs/` | tests of the run documents, fixtures of a document at versions 1 to 4 under `fixtures/` |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`) |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
