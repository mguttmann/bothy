# Module `authz`

Status: planned (M0). This directory contains no code yet. The normative source is
[docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

`authorize`, permission cache, visible targets, groups, grants, permission version, I1 check, "Who has access", "View as person".

## Port (root package `internal/authz`)

- Inbound: `Admin` (groups, grants, reconciliation of the claim groups, explanation, preview, view as person); declares `PrincipalSource`, `CredentialSource`, `StructureSource`, `AgentPolicySource`, `HostSource`.
- Emits in-tx events: kernel `authorize.AccessChanged` (with the triggering action; `runs` cancels with reason `user_disabled` on `user.disable`, otherwise with `access_revoked`).
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: kernel `authorize.Authorizer`, `instance.FlagGroupSource`.

## Dependencies

May use the ports of these modules: `instance`.

Reason: counter `permission_version`; all other data through source ports.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`groups`, `group_memberships`, `grants`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`access.explain`, `access.preview`, `access.view_as`, `group.read`, `group.create`, `group.update`, `group.delete`, `group.member.add`, `group.member.remove`, `group.member.expire`, `group.member.sync`, `grant.read`, `grant.create`, `grant.revoke`, `grant.expire`.

## Events and WebSocket ops

- Durable: `grant.changed`, `group.changed`, `access.changed`, `access.expiring`.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `authz.expiry_reminder` | periodic, `access.expiring` before expiry | `maintenance` |
| `authz.expiry_sweep` | periodic, `access.changed` after expiry (system actions `grant.expire`, `group.member.expire`) | `maintenance` |

## Notes for the build

- The only place with permission logic (B7). Other modules call `authorize` through `kernel/authorize` and never join with `grants` themselves.
- Data of other modules comes through the source ports `PrincipalSource`, `StructureSource`, `AgentPolicySource`; `authz` imports no domain modules.
- The I1 check and `access.changed` run in phase 3.1 of the Unit of Work (02, 8.1).
- The permission cache follows `access.changed` through `Observe` in log order (02, 11.3).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [07-permissions.md](../../../../docs/spec/07-permissions.md)
