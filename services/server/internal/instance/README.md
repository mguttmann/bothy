# Module `instance`

Status: built in part (M1-15): the instance row, the sequence counters with the port `Counters`, the port `State` and the start check of `instance.min_server_version`. Settings, flags, signing keys and the lock-free reads (`Versions`) come with M1-46, M1-47, M1-32 and the tickets that need them.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Instance identity, sync epoch, minimum server version, settings and branding, feature flags, server signing keys with JWKS and rotation, sequence counters.

## Code

| Package | Content |
|---|---|
| `internal/instance` | port: `CounterName` and the three counter constants, `Counters`, `Record` (`SetupCompleted`), `State` |
| `domain/` | counter names and the phase rule (`CheckAdvance`, `CheckRead`, `CheckAdvanceCount`), `CompareSemVer`, `ValidateSemVer`, `CheckServerVersion` (SemVer 2.0.0 precedence, hand-written: the domain layer imports no third-party library) |
| `app/` | `NewCounters` (phase guard, then the repository) and `NewState` (row in a unit of work), outgoing port `Repository` |
| `adapters/postgres/` | `Repository` over `platform/pg`, sqlc queries in `queries/`, generated code in `sqlcgen/` (entry in `services/server/sqlc.yaml`) |

Migration `00002_instance_counters.sql` creates both tables, the three counter rows with value 0
and the trigger function `instance_counter_monotonic()`, which refuses a lower value and a new
name for every role. `bothy_app` may read both tables and update only the column `value` of the
counters; it never inserts or deletes a counter row and never writes the instance row.

### Counters (`Counters`)

`Advance(ctx, name, n)` locks the counter row until the commit (`UPDATE ... SET value = value + n
... RETURNING value`) and returns the new value; the caller's numbers are `last - n + 1 .. last`.
A waiting transaction rereads the committed value in READ COMMITTED, so numbers are gapless and
follow the commit order, also with rollbacks (04 "Pattern with a counter row"). `Current(ctx, name)`
reads without a lock. Nothing in this module runs `SELECT ... FOR UPDATE` on the counters.

Phase guard (02 "Lock order"): the service reads `txn.CurrentPhase(ctx)` and allows
`Advance(permission_version)` only in `rights_begin`, `Advance(event_seq)` only in `sync`,
`Advance(audit_seq)` only in `audit`, and `Current` only in one of the four hook phases. Any other
call, an unknown name and `n < 1` are errors of kind `internal` before any SQL; the table `CHECK`
is the second line. `internal/wire` builds the service (`wire.InstanceCounters`) and passes it only
to the constructors of the phase hooks of authz, sync and audit (M1-29, M1-16, M1-24; none exist
yet). `TestCountersOnlyUsedByPhaseHooks` in `internal/wire` loads every package of the module with
its tests and fails when a package other than this module, `internal/wire` or a package of a
registered hook handler names `Counters` or `CounterName` (an interface of the same shape must
name it), calls the methods of `Counters`, has an expression whose type is or contains it, or
instantiates a generic function or type with it. A value converted to `any` and asserted to a
type of another package later escapes the check; the run-time phase guard still refuses such a
call outside its phase.

### State (`State`)

`RecordMigration(ctx, schemaVersion, minServerVersion)` runs in a unit of work: it inserts the row
with new UUIDv7 values for `instance_id` and `sync_epoch` unless it exists (`INSERT ... ON
CONFLICT (id) DO NOTHING`, safe for concurrent `bothy migrate` runs) and sets `schema_version`
and `min_server_version`. It never changes `instance_id`, `sync_epoch`, `setup_completed_at` or
`created_at`, and never lowers `schema_version`, so an older binary racing a newer one cannot
undo the newer lock. Two binaries at the same schema version may overwrite each other's
`min_server_version`; under rule M1 of upgrade.md both constants are valid for that schema. `Load(ctx)` returns the row or kind `not_found`.

`bothy migrate up` calls it through the after-step of `wire.MigrationSteps` with the goose
version and `migrations.MinServerVersion`, only when the database is at the newest migration the
binary knows; a newer database keeps its row (upgrade.md "Rollback").

### Start check

After the schema check the start loads the record (missing row: the start ends with exit code 1
and the hint to run `bothy migrate up`), keeps it for later tickets (setup state, `instance_id`,
`sync_epoch`) and runs `CheckServerVersion(own version, min_server_version)` once. Below the
minimum the process keeps running for every role, the readiness check
`schema_requires_newer_server` fails for good and the log line
`server version below instance.min_server_version` carries `bothy.version.own` and
`bothy.version.required`; `/readyz` names only the check. An empty minimum is no lock.

## Port (root package `internal/instance`)

- Inbound: `Settings`, `Flags`, `Keys` (signer per purpose, JWKS, rotation), `Counters` (only for the phase hooks of `authz`, `sync`, `audit`), `Versions` (read counter values without a lock, for example `permission_version` for the permission cache, `event_seq` for catch-up); declares `LoginOptionsSource` (login drivers and password path for the discovery), `FlagUserSource` (people of a rollout), `FlagGroupSource` (valid groups of a person for the evaluation).
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: none.

## Dependencies

May use the ports of these modules: none.

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
