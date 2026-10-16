# sync_engine

Gateway connection, catch-up, outbox, reconciliation, run documents and focus, without Flutter.

Status: the run documents of the live box (M4-16) and the outbox (M4-15) are built; connection,
catch-up and focus follow with M4-13, M4-14 and M4-17. Kind: pure Dart package, a member of the Dart
workspace at the repository root (`resolution: workspace`, `publish_to: none`).

## Dependencies

Allowed members (docs/spec/15-client-architecture.md, 3.1; `tools/arch-check/rules.yaml`): `local_store`, `auth`, `api_client`, `domain` (arrows of the diagram).
Every other member is out of reach; `make arch` fails on a dependency or an import of it.

Further rules checked by `make arch`:

- no import of `package:flutter/`, `dart:ui` (rule 1).
- no native source (rule 4).

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
  HTTP client's connection limit.
- An entry with `depends_on` (a message after its uploads) waits until each of them is confirmed;
  a failed dependency keeps it waiting and visible with that reason (`blockedByCode`).
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
  edited or sent again becomes a new entry with a new ID.

Error classes (`classify`, docs/DECISIONS.md D-M4-15-1):

| Response | Class |
|---|---|
| `2xx`, `200` with `Idempotent-Replay: true`, `204` | confirmed |
| network error, timeout, `5xx` except `503 setup_required`, `429`, `409 request_in_progress` | retry with full jitter between `client.outbox_retry_base` and `client.outbox_retry_cap`; `429` and `503` wait `Retry-After` |
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

## Layout

| Path | Content |
|---|---|
| `lib/sync_engine.dart` | the public library; every public symbol has a doc comment (`make lint`, `public_member_api_docs`) |
| `lib/src/outbox/` | `Outbox`, the sender, `classify`, the backoff, the executor registry and the operations (M4-15) |
| `lib/src/runs/` | `RunStore`, `applyRunPatch`, `RunDocument` and the ports (M4-16) |
| `test/outbox/` | tests of the outbox on an in-memory database with scripted executors, a fake clock and a seeded jitter |
| `test/runs/` | tests of the run documents, fixtures of a document at versions 1 to 4 under `fixtures/` |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`) |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
