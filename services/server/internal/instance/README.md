# Module `instance`

Status: built in part (M1-15): the instance row, the sequence counters with the port `Counters`, the port `State` and the start check of `instance.min_server_version`. M1-28 adds the lock-free read of the permission version (`Versions.PermissionVersion`, `app.NewVersions`) for the permission cache of authz. M1-32 adds the server signing keys (`Keys`) with JWKS, rotation and the job `instance.key_rotation`. Settings, flags and the further reads of `Versions` (`event_seq` for catch-up) come with M1-46, M1-47 and the tickets that need them.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Instance identity, sync epoch, minimum server version, settings and branding, feature flags, server signing keys with JWKS and rotation, sequence counters.

## Code

| Package | Content |
|---|---|
| `internal/instance` | port: `CounterName` and the three counter constants, `Counters`, `Record` (`SetupCompleted`), `State`, `Versions`; signing keys (`keys.go`): `Purpose`, `KeyState`, `RotateMode`, `Signer`, `PublicKey`, `JWK`, `JWKSet`, `RotateCommand`, `Rotation`, `Keys` |
| `domain/` | counter names and the phase rule (`CheckAdvance`, `CheckRead`, `CheckAdvanceCount`), `CompareSemVer`, `ValidateSemVer`, `CheckServerVersion` (SemVer 2.0.0 precedence, hand-written: the domain layer imports no third-party library); the transitions of the signing keys (`signingkey.go`: `DueTransitions`, `EmergencyTransitions`, `LostTransitions`, `PendingNext`, `ActiveKey`, `Window`) |
| `app/` | `NewCounters` (phase guard, then the repository) and `NewState` (row in a unit of work), outgoing port `Repository`; `NewKeys` (`Keys`, `KeysConfig`) with the outgoing ports `KeyRepository` and `KeyStore` |
| `adapters/postgres/` | `Repository` over `platform/pg` (both outgoing ports), sqlc queries in `queries/`, generated code in `sqlcgen/` (entry in `services/server/sqlc.yaml`) |
| `adapters/keystore/` | `Store`: the key store `secrets.signing_key_dir` over the secret provider (Ed25519, PKCS#8 PEM, `Thumbprint` after RFC 7638) |
| `adapters/httpapi/` | `Keys`: the REST handlers `getJwks` and `rotateSigningKey` |
| `adapters/jobs/` | `Register`, `Handler`: the periodic job `instance.key_rotation` |

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

### Signing keys (`Keys`, M1-32)

Normative: [19-security.md](../../../../docs/spec/19-security.md) section 10,
[09-agents.md](../../../../docs/spec/09-agents.md) section 4.5 and
[ADR 0066](../../../../docs/adr/0066-signing-keys-in-the-writable-key-store.md).

**Purposes.** `access_token`, `invocation`, `webhook_out` and `audit_checkpoint`, each with
Ed25519 keys (`alg = EdDSA`). The table also allows `terminal`; M2-39 adds it to the managed
purposes, until then `Rotate` refuses it with `400 validation_failed` (`/purpose`,
`not_supported`).

**States.** `next` (published, not signing), `active` (signs; exactly one per purpose through the
partial unique index), `retiring` (stopped signing at `not_after`, still published) and `retired`
(withdrawn, private file deleted, row kept as the record).

**Files.** The private key lives only in the key store `secrets.signing_key_dir` as
`<purpose>-<kid>.pem` (PEM `PRIVATE KEY`, PKCS#8, mode `0600`), written through
`secrets.Provider.Put`, read through `secrets.Provider.Get` (cached `secrets.cache_ttl`). The row
holds the public JWK `{kty, crv, x, kid, alg, use}`, the state and `private_key_ref`
(`file:<dir>/<purpose>-<kid>.pem`). `kid` is the RFC 7638 thumbprint, so anyone can recompute it
and it is safe as a file name. In memory the seed sits in a `secrets.Value`; logs, audit details,
errors and responses name purpose and `kid` only.

**Windows.** A retiring key becomes retired once the longest validity of what it signed has
passed: `auth.access_token_ttl` + `auth.clock_leeway` (`access_token`),
`agent.invocation_token_ttl` + `auth.clock_leeway` (`invocation`), `webhook.delivery_timeout` +
`auth.clock_leeway` (`webhook_out`, every attempt signs anew), zero for `audit_checkpoint` (old
checkpoints verify through `PublicKey` in any state).

**Transitions** (pure functions in `domain`, applied in `app`):

| Transition | When | Effect |
|---|---|---|
| announce | `invocation` and `access_token` only: no `next`, `active.created_at + interval <= t` (`signing_key.invocation_rotation_interval`, `signing_key.access_token_rotation_interval`) | new `next` |
| promote | `next.created_at + agent.jwks_max_age <= t` | old `active` to `retiring` with `not_after = t`, `next` to `active` |
| retire | `retiring` with `not_after + window <= t` | `retired`, file deleted after the commit |
| `Rotate` with `mode = scheduled` | owner request | returns the pending `next` or announces one; the job promotes it |
| `Rotate` with `mode = emergency` | owner request | every `next`, `active` and `retiring` key `retired` at once, a fresh key `active` |
| `EnsureActive` | start of every role, later `bothy setup bootstrap-owner` (M1-37) | an `active` key for every purpose without one; keys whose private file is missing are retired first (`LostTransitions`) |

Every change of the keys of a purpose runs in a unit of work of its own that first takes
`pg_advisory_xact_lock(hashtext('bothy.server_signing_keys'), hashtext(<purpose>))`, also when no
row exists yet. A new key is generated and written before the transaction; when the transaction
fails or no longer needs it, its file is removed (best effort, logged with `kid` only). `Rotate`
runs its unit detached from the ambient one of the Idempotency-Key stage, because files of
retired keys are deleted after the commit and must never go for a change that rolls back
(docs/DECISIONS.md M1-32). The job and `EnsureActive` write one `signing_key.transition` audit
entry per transition (no actor, `resource_kind = instance`, details `purpose`, `from_state`
(`none` for a new key), `to_state`, `kid`); `Rotate` writes one `signing_key.rotate` entry with
`purpose`, `mode`, `new_kid`, `previous_kid` and `reason`. Audit details carry a `kid` as the hex
form of the thumbprint (`app.KIDCode`, 64 lower-case characters), because SEC-AUD-5 codes are
lower case.

**Reading.** `Signer` reads the `active` row fresh on every call (one indexed row), loads the
private key and checks that it belongs to the public key of the row. `VerificationKey` answers
from a per-process cache of the `next`, `active` and `retiring` keys, refreshed after
`signing_key.cache_ttl`; an unknown `kid` reloads at most once per `signing_key.cache_ttl`, and
every transition of the own process drops the cache, so an emergency rotation withdraws the old
key at once in the same process and within `signing_key.cache_ttl` in others. `PublicKey`
returns a key in any state. `JWKS` authorizes `instance.discovery.read` and publishes the `next`,
`active` and `retiring` keys of `access_token` and `invocation`, ordered by purpose, state
(`active`, `next`, `retiring`) and `kid`; `webhook_out` keys go out through `getWebhookKeys` (M2),
`audit_checkpoint` keys never.

**HTTP.** `getJwks` answers `application/jwk-set+json` with `Cache-Control: public,
max-age=<agent.jwks_max_age in whole seconds>`; the CORS stage adds
`Access-Control-Allow-Origin: *` without credentials for its action (SEC-CORS-2).
`rotateSigningKey` maps `SigningKeyRotate` to `RotateCommand` and answers `200
SigningKeyRotation`; owner with fresh step-up only.

**Job.** `instance.key_rotation` on the queue `maintenance` every
`signing_key.rotation_check_interval` without arguments (worker and all run it): per purpose
the due transitions of `DueTransitions`, each purpose in its own transaction, one log line
`signing key transition` per transition with `bothy.signing_key.purpose`,
`bothy.signing_key.kid`, `bothy.signing_key.from_state` and `bothy.signing_key.to_state`.

**Start and readiness.** The secrets step of every role checks the key store
(`secrets.CheckSigningKeyDir`: owner, mode `0700`, writable); the step `signing_keys` after
identity runs `EnsureActive`, and `/readyz` fails with `signing_keys` while a purpose has no
active key or its active key cannot sign (`Keys.Unready`: the private file is unreadable or does
not belong to the row).

**Failure modes.**

- Key store not writable or wrong owner or mode: the start ends with a message naming
  `secrets.signing_key_dir` and mode `0700`.
- Key store not writable at run time: `Rotate` and the job fail with `503 unavailable`; nothing
  changes in the database.
- Private file of the active key unreadable: `Signer` answers `unavailable` and logs
  `signing key not loadable` with purpose and `kid`; `/readyz` fails with `signing_keys`.
- Private file missing (a restore without the key store, or a role without the key store
  volume): the next start retires the keys without a file, creates a fresh active key and logs
  `signing key file missing, key retired` at level error; tokens signed with the old keys stop
  verifying (16 section 5.4). Every role must mount the same key store; one without it retires the
  keys of the others at its start.
- Crash between the file and the row: an unreferenced file `<purpose>-<kid>.pem` stays in the key
  store. It is never used (no row names it); `bothy secrets export` includes it, and it can be
  deleted by hand. The same holds for the file of a retired key whose removal after the commit
  failed (logged as `signing key file not removed`) and for a leftover `.bothy-tmp-*` of a write
  interrupted by a crash: nothing sweeps them, the directory mode `0700` keeps them private.
- A plan that changes under the lock (another process changed the keys between the read and the
  lock) and now needs a key that was not generated rolls back, generates one and runs again (up to
  three times, then `409 conflict`).
- Multi-node key stores (stage 2) need a shared volume or a writable vault driver (M7-17, V3-12).

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

`instance.discovery.read`, `console.csp_report`, `instance.read`, `instance.settings.read`, `instance.settings.update`, `instance.branding.update`, `feature_flag.read`, `feature_flag.update`, `signing_key.rotate`, `signing_key.transition`.

## Events and WebSocket ops

- Durable: `instance.updated`.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `instance.key_rotation` | periodic every `signing_key.rotation_check_interval`, states `next` to `retired` (M1-32) | `maintenance` |

## Notes for the build

- The module hands out the sequence counters (`sequence_counters`) only through `Counters` to the phase hooks of `authz`, `sync` and `audit` (02, 8.1 and 8.2); no use case locks them itself.
- Private keys are only in the secret provider (`private_key_ref`); the table holds the public JWK and the state.
- Settings are data of the owner with a schema per key, not server configuration (02, section 13).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [06-api.md](../../../../docs/spec/06-api.md), [16-operations.md](../../../../docs/spec/16-operations.md), [19-security.md](../../../../docs/spec/19-security.md)
