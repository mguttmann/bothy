# sync_engine

Gateway connection, catch-up, outbox, reconciliation, run documents and focus, without Flutter.

Status: the run documents of the live box are built (M4-16); connection, catch-up, outbox and
focus follow with M4-13 to M4-15. Kind: pure Dart package, a member of the Dart
workspace at the repository root (`resolution: workspace`, `publish_to: none`).

## Dependencies

Allowed members (docs/spec/15-client-architektur.md, 3.1; `tools/arch-check/rules.yaml`): `local_store`, `auth`, `api_client`, `domain` (arrows of the diagram).
Every other member is out of reach; `make arch` fails on a dependency or an import of it.

Further rules checked by `make arch`:

- no import of `package:flutter/`, `dart:ui` (rule 1).
- no native source (rule 4).

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
| `lib/src/runs/` | `RunStore`, `applyRunPatch`, `RunDocument` and the ports (M4-16) |
| `test/runs/` | tests of the run documents, fixtures of a document at versions 1 to 4 under `fixtures/` |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`) |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
