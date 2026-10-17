# sync_engine

Gateway connection, catch-up, outbox, reconciliation, run documents and focus, without Flutter.

Status: the gateway connection with focus and typing (M4-13), catch-up and reconciliation
(M4-14), the outbox (M4-15), the run documents of the live box (M4-16), the composition root
`SyncEngine` and the fake server of the integration tests (M4-17) are built. Kind: pure Dart package, a member of the Dart
workspace at the repository root (`resolution: workspace`, `publish_to: none`).

## Dependencies

Allowed members (docs/spec/15-client-architektur.md, 3.1; `tools/arch-check/rules.yaml`): `local_store`, `auth`, `api_client`, `domain` (arrows of the diagram).
Every other member is out of reach; `make arch` fails on a dependency or an import of it.

Further rules checked by `make arch`:

- no import of `package:flutter/`, `dart:ui` (rule 1).
- no native source (rule 4).
- `package:sync_engine/testing.dart` and `lib/src/testing/` are reached only from `test/` and
  `integration_test/` folders, also inside this package (rule 10).

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

## Outbox (M4-15)

`Outbox` is the one write path of the apps (15 1 principle 3, 04 13.2, 15 7.5). Every write is
stored in the table `outbox` of `local_store` before anything is sent, with its own UUIDv7: the
`client_msg_id` of a message, the `client_interaction_id` of an interaction, the
`Idempotency-Key` of every other write (04 5). The ID never changes on retry.

Life cycle of an entry:

| State | Meaning | Next |
|---|---|---|
| `pending` | stored, waits for its turn in its lane or for an entry it depends on | `sending` |
| `sending` | the executor runs | confirmed (row deleted), `waiting`, `failed` |
| `waiting` | backing off until `next_attempt_at` | `sending` |
| `failed` | refused for good, with the problem code and reason; after `412` also the `current` revision | edited, sent again with a new ID, or discarded by the person |

- One worker per FIFO lane sends the oldest entry and the next only after the response (04 13.2
  item 2). The lane is the thread, or the lane the executor names for writes outside a thread
  (`setting:<key>`, `message:<id>`, `upload:<id>`); lanes run in parallel, bounded only by the
  HTTP client's connection limit. An entry goes no later than the oldest entry that waits for
  it, also through other lanes, so a replacement at the end of a lane never waits behind its
  own dependent.
- An entry with `depends_on` (a message after its uploads) waits until each of them is confirmed;
  a failed dependency keeps it waiting and visible with that reason (`blockedByCode`). One
  resolver (`dependencies.dart`) decides this for every path, over every stored entry: a row
  that leaves the table re-points every entry that names it (confirmed: dropped; retried or
  edited: its replacement; a dropped follow-up: what it waited for; discarded: dropped after
  its chain failed), and a dependency that is gone without a recorded confirmation is never
  taken as confirmed: the entry fails with `dependency_discarded` instead of going without it.
  `enqueue` resolves the IDs it is given through the replacements and confirmations of the
  running outbox; an ID gone before the outbox was created is unknown and fails the entry the
  same way.
- `start()` sends (M4-17 calls it after `catchup.complete`, 04 13.4), `pause()` lets running
  requests finish and starts none. The first `start()` resets `sending` entries of an earlier run
  to `pending`; they are sent again with their stored IDs.
- `confirmByEcho(clientMsgId)` confirms an entry when its echo `message.created` arrives, also
  before the response (04 16.2); the late response then changes nothing.
- `watchThread(threadId)` streams the entries of a thread for the display; an entry in
  `waiting` for longer than `client.outbox_stuck_after` since it was written reports `delayed`
  ("Sending delayed"). `depth` streams the number of entries that are not final.
- Editing and deleting follow 04 13.3: an entry with `attempts == 0` is changed or removed in
  place; an attempted entry stays as it is and gets a follow-up `updateMessage` or
  `deleteMessage` that waits for it and receives its `message_id` and `version`; a failed entry
  edited or sent again becomes a new entry with a new ID, and the entries that waited for it
  wait for the new one (never sent follow-up edits behind it are dropped, the edit is newer).
  An edit holds the changed members and is merged into the stored payload. Discarding an entry
  fails every entry that waits for it, directly or through others (`dependency_discarded`).
  An entry the outbox writes itself (a retry, the replacement of an edit, a follow-up) never
  waits for an entry that failed with its chain (`dependency_discarded`); it waits for what
  that entry waited for. An entry that failed for its own reason (an upload) stays a
  dependency until the person retries or discards it.

Error classes (`classify`, docs/DECISIONS.md D-M4-15-1):

| Response | Class |
|---|---|
| `2xx`, `200` with `Idempotent-Replay: true`, `204` | confirmed |
| network error (also a TLS error), timeout, `5xx` except `503 setup_required`, `429`, `409 request_in_progress` | retry with full jitter between `client.outbox_retry_base` and `client.outbox_retry_cap`; `429` and `503` wait `Retry-After` |
| `412 version_conflict` | conflict: `failed` with the `current` revision |
| `400`, `401`, `403`, `404`, any other `409`, `410`, `413`, `415`, `422`, `428`, `503 setup_required` | final: `failed` with code and reason |
| an error that is no transport error and no response (a bug in an executor) | final: `executor_error` |
| a kind without an executor | final: `no_executor` |

How a feature registers a write: an executor sends one entry through the generated client and
returns an `OutboxResponse` on `2xx`; it passes `ctx.extra` so `api_client` writes the
`Idempotency-Key`.

```dart
executors.register(
  'upload',
  (entry, ctx) async {
    final body = decodePayload(entry.payload);
    // ... call the generated API with extra: ctx.extra ...
    return const OutboxResponse();
  },
  lane: (entry) => 'upload:${entry.id}', // null: the entry's thread
);
```

Registered by this package: `MessageOps.register` (`createMessage`, `updateMessage`,
`deleteMessage`, `addReaction`, `removeReaction`) and `PersonalOps.register` (`putReadState`,
`putThreadDraft`, `deleteThreadDraft`, `putMySetting`, `deleteMySetting`). M4-21 adds the
interactions and commands, M4-22 the uploads.

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

## Composition root (M4-17)

`SyncEngine.create(...)` is the one place that builds and wires the parts; the apps never wire
them themselves (docs/DECISIONS.md D-M4-17-1).

| Part | Built from |
|---|---|
| REST client | `AuthSession.api` (the client of the session with its DPoP, refresh and step-up interceptors) |
| `GatewayConnection` | the transport, network, lifecycle and clock ports; gateway `wss://<host>/api/v1/gateway` from `baseUrl` |
| `CatchUpListener` and `Dispatcher` | the local database, `ApiFetcher` over the REST client |
| `Outbox` | `MessageOps` and `PersonalOps` on the REST client |
| `RunStore` | a fetcher on `getRun` and the `FrameTicker` |

Hooks: the echo `message.created` calls `Outbox.confirmByEcho` with `message_id` and
`version`; the catch-up's live signal (`catchup.complete`, or `ready` without a cursor) calls
`Outbox.start`, every disconnect `Outbox.pause` (04 13.4); `run.updated` goes to
`RunStore.onRunUpdated`, a `run.state_changed` to `completed`, `failed` or `cancelled` to
`RunStore.onRunEnded`; after a resync the focused runs that the new snapshot no longer lists
as active end too. A confirmed `createMessage` or `updateMessage` stores the returned message
at once (`CatchUpListener.storeConfirmedMessage`), so the entry is replaced before its echo
(04 13.2 item 6). `EngineFocus` joins the visible threads and the threads of focused runs
into the gateway focus, so `focusRun(run, thread)` both fetches the document and lets
`run.updated` arrive. `baseUrl` must be `https`; only `allowPlainHttpForTests: true` (the
headless tests and the app's test entry point) accepts `http` with a `ws` gateway
(D-M4-17-6).

## Fake server for integration tests (M4-17)

`package:sync_engine/testing.dart` is a separate library: the headless tests of this package and
the app tests from M4-18 on share one fake of the server (D-W-114). Start one per test:

```dart
final server = await FakeServer.start(
  options: const FakeServerOptions(
    timings: GatewayTimings(runPatchCoalesce: Duration(milliseconds: 40)),
  ),
);
final world = WorldSeed.standard(server.world); // Alex, Sam and the agent Robin
// point the client at server.baseUri (http://127.0.0.1:<port>, test builds only)
server.faults.dropResponseAfterCommit('createMessage');
// ... drive the client ...
expect(server.violations, isEmpty);
await server.stop();
```

It binds `127.0.0.1` with port 0, serves REST under `/api/v1/` and the gateway at
`/api/v1/gateway`, and holds no global state.

### World

`FakeWorld` keeps the instance and its sync epoch, people and agents, devices, sessions and
pairings, spaces, channels with the level of each principal, threads, messages with
`thread_seq`, the event log with a global `seq`, read states, drafts, settings and run
documents. Every write appends the events of 05 section 6 with a new `seq` and pushes them to
the connections that may see them. Test controls: `addPerson`, `addSpace`, `addChannel`,
`addThread`, `createMessage` (as any author), `setAccess` (writes `access.changed` as 04 8.2),
`approveDevice`, `expirePendingDevice`, `rejectPendingDevice`, `expirePairing`,
`revokeSession`, `revokeDevice`, `dropEventsUpTo` (retention), `changeEpoch`, `createRun`,
`setRunState`, `patchRun`, and the values `catchupPageSize`, `cursorAdvanceEvery` and
`catchupMaxEvents`. `WorldSeed.standard` builds the space "Team" with the channels `general`,
`support` and `private` (only Sam) and one thread each.

### Operations

`getDiscovery`, `loginWithPassword` (password and TOTP), `beginPairing`, `completePairing`,
`approvePairing`, `getDevice`, `approveDevice`, `refreshToken`, `logout`, `getMe`,
`getInstance`, `getSnapshot`, `listEvents`, `listSpaces`, `getSpace`, `listSpaceChannels`,
`getChannel`, `listChannelThreads`, `getThread`, `listThreadMessages`, `getMessage`,
`createMessage` (04 5.1 replay and `422 idempotency_key_reuse`), `updateMessage` (`If-Match`,
`412 version_conflict` with `current`, `410 message_deleted`), `deleteMessage` (idempotent
`204`), `addReaction`, `removeReaction`, `putReadState`, `putThreadDraft`,
`deleteThreadDraft`, `listMyReadStates`, `listMyDrafts`, `listMySettings`, `putMySetting`,
`deleteMySetting`, `getRun`, `cancelRun`. Other writes with `Idempotency-Key` repeat their
stored answer (04 5.2). Later tickets add operations with `server.handle(operationId,
handler)` in their own `rest/` file (D-M4-17-2). Gateway: `hello`, `identify`, `resume`,
`ready` with the snapshot, catch-up in pages with `cursor` frames, `catchup.complete`,
`resync_required` for `epoch_changed`, `retention`, `too_far_behind` and `cursor_ahead`, live
delivery by visibility at delivery time, `heartbeat_ack`, `reauth`, `focus` (with `error`
`validation_failed` above `sync.max_focused_threads`), `typing.started` to other focusing
connections, coalesced `run.updated` with the whole document on focus, one connection per
device (`4017`) and the close codes of 04 6.4 for the session checks.

### Faults

`server.faults` is a `FaultPlan`, a queue of rules matched in order and fully deterministic
(D-M4-17-3):

| Fault | Effect |
|---|---|
| `onRequest(op, status:, code:, retryAfter:, times:)` | answers `409`, `410`, `412`, `429` (with `Retry-After`), `5xx` or any problem instead of the handler |
| `dropResponseAfterCommit(op)` | stores the write and appends its events, then closes the connection before the response (04 16.2) |
| `delay(duration, operationId:)` | holds the response after the handler ran |
| `closeGateway(code, reason:, after:)` | closes every gateway connection with any code |
| `duplicateNextDispatch()` | sends the next `dispatch` twice |
| `resyncRequired(reason)` | answers the next catch-up with `resync_required` |
| `revoke(deviceId:)` | closes the device's connections with `4012` and revokes its sessions |
| `holdDispatches()`, `releaseDispatches()` | holds live delivery, as a slow gateway does |

`server.disconnect(deviceId:)` ends connections without a close code (a lost network).

### Checks

DPoP (08 7.3): ES256 signature with the header `jwk`, `typ`, `htm`, `htu` (the fake's origin
plus the path; the gateway's `http` form), `ath`, `iat`, `jti` reuse and the bound key; a
missing or stale nonce answers `401 use_dpop_nonce` with `DPoP-Nonce`, any other failure
`401 invalid_dpop_proof`; the gateway closes with `4004`. Contract (D-M4-17-4): every request
body is validated against `docs/api/openapi.yaml` (a violation answers
`400 validation_failed` with the pointer), every response and every frame the fake sends
against `openapi.yaml` and `asyncapi.yaml`, every client frame too (a violation closes with
`4002`); all of them land in `server.violations`. The validator (`ContractSchemas`) reads the
documents once per isolate, walking up from the working directory.

### Scenarios

`scenarios.dart` holds the flows of 04 16 as functions over a `ScenarioClient`:
`scenarioSendOnline` (16.1), `scenarioSendOffline` (16.2), `scenarioCatchUp` and
`scenarioResync` (16.3), `scenarioAccessRemoval` (16.4), `scenarioAgentRun` (16.5, with a
`ScriptedAgent` whose steps come from an `AgentScript`). `test/scenarios/` runs them over
`SyncEngine`; M4-18 implements `ScenarioClient` over the app (D-M4-17-5).

### What the fake simplifies

- Permissions: only visibility and the level per channel; no grants, groups, `manage`, scopes
  or step-up (`approveDevice` and `approvePairing` need none); no rate limits unless injected.
- Identity: a fixed TOTP code per person; a password login gives an active device unless the
  person has `newDevicesPending`; access tokens are unsigned JWS-shaped strings the fake
  recognizes by value; nonces are counters (current and previous accepted) instead of HMAC
  windows; only `ES256` proofs; no refresh token reuse detection beyond rejecting unknown
  tokens with `401 session_revoked`.
- Gateway: no heartbeat timeout (`4009`), no `reauth_required`, no `identify` timeout, no
  outbound queue limit (`4014`); catch-up runs synchronously, so no live event interleaves.
- Events: `agent.invocation` and the agent consumer side are not modelled; `device.pending` is
  not written (it is not in the API documents of this base); `typing.stop` has no effect.
- `cancelRun` ends the run at once with `cancelled`.
- Snapshot: `unread_threads` and the read state's `mention_count` are computed (messages of
  others after the read position, not deleted); `sidebar` and the agent presence (`agents`) are
  always empty.

## Layout

| Path | Content |
|---|---|
| `lib/sync_engine.dart` | the public library; every public symbol has a doc comment (`make lint`, `public_member_api_docs`) |
| `lib/src/*.dart` | the gateway connection: ports, `dart:io` transport, frames, states, close table, backoff, heartbeat, session, connection, focus, typing, listener (M4-13) |
| `lib/src/catch_up.dart`, `dispatcher.dart`, `appliers/`, `snapshot_applier.dart`, `fetcher.dart`, `gap_filler.dart`, `ephemeral_store.dart`, `sync_events.dart`, `isolate_decode.dart`, `wire.dart` | catch-up, dispatcher, appliers per resource group, REST reads, gaps, ephemeral store, notices and ports, decoding, the wire mappers (M4-14) |
| `lib/src/outbox/` | `Outbox`, the sender, `classify`, the backoff, the executor registry and the operations (M4-15) |
| `lib/src/runs/` | `RunStore`, `applyRunPatch`, `RunDocument` and the ports (M4-16) |
| `lib/src/engine.dart` | `SyncEngine`, `EngineFocus` (M4-17) |
| `lib/testing.dart`, `lib/src/testing/` | the fake server: world, gateway, REST with one handler file per group under `rest/`, DPoP check, faults, scripted agent, contract validator, seed, scenarios (M4-17) |
| `test/connection/` | tests of the gateway connection, one per close code, with the fakes of `test/support/` |
| `test/catch_up/` | tests of the catch-up on the connection fakes, an in-memory store and recorded REST responses |
| `test/outbox/` | tests of the outbox on an in-memory database with scripted executors, a fake clock and a seeded jitter |
| `test/runs/` | tests of the run documents, fixtures of a document at versions 1 to 4 under `fixtures/` |
| `test/fake_server/` | the fake's own tests: contract, DPoP, idempotency, faults |
| `test/scenarios/` | the scenarios of 04 16 and the wiring of `SyncEngine`, headless against the fake |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`) |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
