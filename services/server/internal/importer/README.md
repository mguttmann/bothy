# Module `importer`

Status: planned (M0). This directory contains no code yet. The normative source is
[docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Import sources, batches with origin key (`import.apply`), ownership list and read-only state of mirrored channels; the dry run lives in the tool.

## Port (root package `internal/importer`)

- Inbound: `Imports` (sources, batches through `import.apply`).
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: none.

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

- Only reads the source; dry run before the real run (ADR 0043).
- `import_items` becomes `pending` before the creation and `posted` in the same transaction as the target object (03, 4.11).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [22-import-migration.md](../../../../docs/spec/22-import-migration.md)
