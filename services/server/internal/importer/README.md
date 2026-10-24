# Module `importer`

Status: the import interface of the server (M8-03): sources with their state machine and mapping,
the ownership list per channel, the batches of structure items (spaces, channels, forum tags,
people) with origin keys, the cursor and the counters. Content items (M8-11): thread batches with
thread, message and pin items, and file items through `createImportFile`, `uploadImportFilePart`
and `completeImportFile`.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Import sources, batches with origin key (`import.apply`), ownership list and read-only state of mirrored channels; the dry run lives in the tool.

## Code

- `internal/importer` (`importer.go`): the port `Imports` (list, create, read, items, start,
  pause, cancel, complete, `SetLead`, `ApplyBatch`, `SetCursor`, `SourceOfTarget`), the DTOs, the
  reason codes of `409 conflict` (`import_state`, `lead_not_bothy`, `channel_not_imported`,
  `import_not_running`, `target_outside_source`) and the in-transaction event `SourceChanged`.
  `Register` declares `import.progress`.
- `internal/importer/domain`: the state machine (`Transition`; a cancelled source is `failed` with
  `server.cancelled`), the mapping of schema `import_source.discord.v1` with its field-by-field
  validation that reports every violation at once (`ParseMapping`, `Mapping.JSON` stores it
  normalized), the origin key forms (`ParseOriginKey`; a forum tag key is
  `forum_tag:<channel>:<tag>`), the data of the structure items, the placeholder handle and display
  name and the counters of `import_sources.stats` (`server.*` of the server, `tool.*` of the tool).
- `internal/importer/app`: `Service` implements `Imports`; `Guard` implements the source port
  `chat.ReadOnlyGuard` over the store alone, so `internal/wire` builds it before chat. Every change
  of a source locks its row first, so state changes, the ownership list, batches and the cursor of
  one source are serialized. A batch is one unit of work across modules: per item the pending row
  (`INSERT ... ON CONFLICT DO NOTHING`), the target through `chat.ImportStructure` or
  `identity.ImportPeople`, then `posted` or `skipped`; an existing key is locked and answered with
  its stored item (with `source_changed` when the hash differs). Only a batch that made a new
  transition writes the counters and `import.progress`, so a repeated batch appends no event.
- `internal/importer/adapters/postgres`: the sqlc queries over `import_sources` and `import_items`
  and `Repository`. `adapters/httpapi`: the fourteen operations of the tag Import and the cursor
  codec.

## Content items (M8-11)

- `domain/content.go`: `CheckThreadBatch` (the shape of a thread batch: `thread:<id>` or
  `channel:<id>`, the thread item only first and only with its own key, then messages and pins),
  the parsers of `ImportThreadData`, `ImportMessageData` and `ImportPinData`, `CutTitle` and
  `MirrorChange` (22 12.2 step 3: an `auto` title follows a rename, archive and reopen, a later
  `last_activity_at`).
- `app/content.go`: `applyThreadBatch` in the unit of work of the batch. The thread is the main
  thread of a posted text channel (`chat.ImportThreads.MainThread`), a posted thread, or the thread
  its first item creates (`chat.ImportThreads.Create`: planned operator of the mapping, times and
  done state of the source, starter message, posted forum tags) or mirrors
  (`Lock`, `ApplyMirror`); every message goes through `messaging.ImportMessages.Append` (author of
  the posted user item, else `409 conflict` with reason `author_not_imported`; reply target and files of posted items),
  every pin through `ImportMessages.Pin` with the owner as `pinned_by`. A batch that wrote anything
  ends with `chat.ImportThreads.FinishBatch` (floor, version, one `thread.updated`), the counters
  and `import.progress`; a replayed batch writes nothing. Imported messages never publish
  `MessagePosted`, so nothing triggers or notifies.
- `app/files.go`: `CreateFile` (the channel of a posted channel item, the author of a posted user
  item, then the `pending` item with the file of `files.ImportFiles.Reserve`; a repeat answers the
  same file ID through `ImportFiles.Slot`), `UploadFilePart` (outside a unit of work) and
  `CompleteFile` (the completion and `posted` in one unit), all `import.apply`.

## Port (root package `internal/importer`)

- Inbound: `Imports` (sources, batches through `import.apply`).
- Emits in-tx events: `SourceChanged` (every committed change of a source; no subscriber yet).
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: `chat.ReadOnlyGuard` (`app.Guard`, M8-03).

## Dependencies

May use the ports of these modules: `identity`, `chat`, `agents`, `messaging`, `files`.

Reason: creates people, structure, messages, earlier session references and files; sets the read-only state of the mirrored channels.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`import_sources`, `import_items`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`import.read`, `import.manage`, `import.apply`.

## Events and WebSocket ops

- Durable: `import.progress`.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Notes for the build

- The server never calls Discord (R-IM-01); the tool reads and writes only through this interface
  (ADR 0043). A `cli` session cannot provide step-up, so the steps of `import.manage` run in the
  web console (08 6.4).
- `import.apply` needs an owner; the use case checks the running source (`409 conflict`,
  `import_not_running`, also for a paused source) and the target in a channel of the source
  (`target_outside_source`) and records each refusal as a denial with reason `import_not_running`.
- The mapping decides: a space or channel not named in it is outside the source, one mapped to
  `skip` is stored as `skipped`, a new object takes the name of the mapping, and an existing one is
  only adopted when the mapping names exactly it. New channels get no grant, operator mode `none`
  and `read_only` from the stored lead.
- Placeholders: `imported-<name>` with the counter `-2`, `-3`, ... until a free handle; the person
  is `disabled` without identity, credential or grant (R-IM-06).
- `import_items` is written `pending` before the target and `posted` in the same transaction
  (03, 4.11); a file item stays `pending` with its reserved file until `completeImportFile`.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [22-import-migration.md](../../../../docs/spec/22-import-migration.md)
