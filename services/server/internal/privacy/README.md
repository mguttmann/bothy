# Module `privacy`

Status: skeleton (M1-05). The port package exists; interfaces and use cases come with the tickets of the milestone.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapter applies.

## Purpose

Privacy requests, export and deletion through the participants of all modules.

## Code

- `internal/privacy` (`doc.go`): port as the root package, so far only with a package comment. Interfaces, DTOs and In-Tx events come with the ticket that implements them.
- `domain/`, `app/` and `adapters/<kind>/` are created with the first use case. `tools/archgen` generates their components from `architecture/modules.yaml`; a package outside these layers fails `make arch`.

## Port (root package `internal/privacy`)

- Inbound: `Requests`.
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: collects all kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `identity`, `files`.

Reason: data subject; store the export package.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`gdpr_requests`, `deletion_journal_state` (numbering of the deletion journal, R-DM-52).

Only this module writes and reads these tables via SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`gdpr.request.create`, `gdpr.request.read`, `gdpr.request.decide`, `gdpr.export.download`.

## Events and WebSocket ops

- Durable: `gdpr.request_updated`, `gdpr.request_due`.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `privacy.export`, `privacy.erase` | approval of a privacy request | `privacy` |

## Notes for the build

- Collects all implementations of `kernel/personaldata.Participant`; a new module with tables of personal data must register a participant.
- Export packages are stored as a file through the port of `files`; deadline `due_at` as defined in 20-privacy.md.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [20-privacy.md](../../../../docs/spec/20-privacy.md)
