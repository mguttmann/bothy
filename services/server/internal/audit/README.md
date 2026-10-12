# Module `audit`

Status: skeleton (M1-05). The port package exists; interfaces and use cases come with the tickets of the milestone.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Audit log with hash chain, signed checkpoints, keys of the IP pseudonyms, query, verification, truncation with anchor.

## Code

- `internal/audit` (`doc.go`): port as the root package, so far only with a package comment. Interfaces, DTOs and In-Tx events come with the ticket that implements them.
- `domain/`, `app/` and `adapters/<kind>/` are created with the first use case. `tools/archgen` generates their components from `architecture/modules.yaml`; a package outside these layers fails `make arch`.

## Port (root package `internal/audit`)

- Inbound: `Log` (query, verification).
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: kernel `auditing.Recorder`.

## Dependencies

May use the ports of these modules: `instance`.

Reason: counter `audit_seq`.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`audit_events`, `audit_checkpoints`, `audit_ip_keys`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`audit.read`, `audit.verify`.

## Events and WebSocket ops

- Durable: none.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `audit.chain_verify` | periodic | `maintenance` |
| `audit.retention_trim` | periodic, separate maintenance role of the database | `maintenance` |

## Notes for the build

- The application role of the database may only `INSERT` and `SELECT`; truncation runs through a separate maintenance role and writes an anchor (03, 4.12).
- `Recorder` writes in the running transaction (phase 3.3 of the Unit of Work) or separately when a denial must go into the audit after a rollback.
- Identical denials are aggregated per `audit.deny_aggregate_window` (05, R-AZ-04).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [19-security.md](../../../../docs/spec/19-security.md), [20-privacy.md](../../../../docs/spec/20-privacy.md)
