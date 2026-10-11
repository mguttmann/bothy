# Module `search`

Status: planned (M0). This directory contains no code yet. The normative source is
[docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapter applies.

## Purpose

Search query over messages, thread titles and files with a permission filter, ranking.

## Port (root package `internal/search`)

- Inbound: `Search`.
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: none.

## Dependencies

May use the ports of these modules: `chat`, `messaging`, `files`.

Reason: thread titles, messages, file names and transcripts.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

None. The module reads through the ports of other modules (03-data-model.md, principle 12).

## Actions (05-actions.md)

`search.query`.

## Events and WebSocket ops

- Durable: none.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Notes for the build

- The permission filter comes from `Authorizer.Scope`; every query is combined with `channel_id = ANY(...)` (03, section 5). No permission logic of its own.
- Reads messages, thread titles and files only through the ports of `messaging`, `chat`, `files`.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [14-search.md](../../../../docs/spec/14-search.md)
