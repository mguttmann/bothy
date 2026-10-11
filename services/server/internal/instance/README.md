# Module `instance`

Status: planned (M0). This directory contains no code yet. The normative source is
[docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Instance identity, sync epoch, minimum server version, settings and branding, feature flags, server signing keys with JWKS and rotation, sequence counters.

## Port (root package `internal/instance`)

- Inbound: `Settings`, `Flags`, `Keys` (signer per purpose, JWKS, rotation), `Counters` (only for the phase hooks of `authz`, `sync`, `audit`), `Versions` (read counter values without a lock, for example `permission_version` for the permission cache, `event_seq` for catch-up); declares `LoginOptionsSource` (login drivers and password path for the discovery), `FlagUserSource` (people of a rollout), `FlagGroupSource` (valid groups of a person for the evaluation).
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: none.

## Dependencies

Must not use the ports of any other module.

Reason: foundation.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`instance`, `instance_settings`, `server_signing_keys`, `feature_flags`, `sequence_counters`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`instance.discovery.read`, `instance.read`, `instance.settings.read`, `instance.settings.update`, `instance.branding.update`, `feature_flag.read`, `feature_flag.update`, `signing_key.rotate`, `signing_key.transition`.

## Events and WebSocket ops

- Durable: `instance.updated`.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `instance.key_rotation` | periodic, states `next` to `retired` | `maintenance` |

## Notes for the build

- The module hands out the sequence counters (`sequence_counters`) only through `Counters` to the phase hooks of `authz`, `sync` and `audit` (02, 8.1 and 8.2); no use case locks them itself.
- Private keys are only in the secret provider (`private_key_ref`); the table holds the public JWK and the state.
- Settings are data of the owner with a schema per key, not server configuration (02, section 13).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [06-api.md](../../../../docs/spec/06-api.md), [16-operations.md](../../../../docs/spec/16-operations.md), [19-security.md](../../../../docs/spec/19-security.md)
