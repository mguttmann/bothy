# Modul `instance`

Status: built in part (M1-15): the instance row, the sequence counters with the port `Counters`, the port `State` and the start check of `instance.min_server_version`. M1-28 adds the lock-free read of the rights version (`Versions.PermissionVersion`, `app.NewVersions`) for the rights cache of authz. M1-32 adds the server signing keys (`Keys`) with JWKS, rotation and the job `instance.key_rotation`. M1-46 adds the instance settings, the branding, discovery, the instance view and the problem types (`Settings`, `Admin`, `LoginOptionsSource`). M1-47 adds the feature flags (`Flags`, `FlagAdmin`, `FlagGroupSource`, `FlagUserSource`, the registry and `tools/flagcheck`). The further reads of `Versions` (`event_seq` for catch-up) come with the tickets that need them.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Instanz-Identität, Sync-Epoche, Mindest-Server-Version, Einstellungen und Branding, Feature-Flags, Server-Signierschlüssel mit JWKS und Rotation, Sequenz-Zähler.

## Code

| Package | Content |
|---|---|
| `internal/instance` | port: `CounterName` and the three counter constants, `Counters`, `Record` (`SetupCompleted`), `State`, `Versions`; signing keys (`keys.go`): `Purpose`, `KeyState`, `RotateMode`, `Signer`, `PublicKey`, `JWK`, `JWKSet`, `RotateCommand`, `Rotation`, `Keys`, `ErrJWSInvalid`, `SignatureAlgs`; settings (`settings.go`): `SupportedLocales`, `Settings`, `Branding`, `LoginOptionsSource`, `LoginOptions`, `ProviderInfo`, `Admin` with its DTOs (`Setting`, `IfMatch`, `PutSetting`, `UpdateBranding`, `Info`, `Limits`, `Discovery`, `ClientVersion`, `ProblemType`); feature flags (`flags.go`): the registry `RegisteredFlags` with `FlagDefinition`, `FlagKind` and the key constants, `Flags`, `FlagSubject`, `SubjectOf`, `EvaluatedFlag`, `FlagAdmin` with `FeatureFlag`, `FlagRollout`, `PutFlagCmd`, the source ports `FlagGroupSource` and `FlagUserSource` |
| `domain/` | counter names and the phase rule (`CheckAdvance`, `CheckRead`, `CheckAdvanceCount`), `CompareSemVer`, `ValidateSemVer`, `CheckServerVersion` (SemVer 2.0.0 precedence, hand-written: the domain layer imports no third-party library); the transitions of the signing keys (`signingkey.go`: `DueTransitions`, `EmergencyTransitions`, `LostTransitions`, `PendingNext`, `ActiveKey`, `Window`); the settings registry (`settings.go`: `Registry`, `Validate`, `ValidateProductName`, `ValidateAccentColor`, `DefaultProductName`), the branding version and changes (`branding.go`) and the discovery rules (`discovery.go`: `MinClientVersions`, `LocalPasswordEnabled`, `DiscoveryFeatures`, `ContentETag`); the flag rules (`flags.go`: `FlagSpec`, `CheckRegistry`, `Evaluate`, `InRollout`, `Rollout`, `NormalizeRollout`, `ReachedRemoveBy`) |
| `app/` | `NewCounters` (phase guard, then the repository) and `NewState` (row in a unit of work), outgoing port `Repository`; `NewKeys` (`Keys`, `KeysConfig`) with the outgoing ports `KeyRepository` and `KeyStore`; `NewAdmin` (`Admin`, `Config`, `Contract`) with the outgoing ports `SettingsRepository` and `FlagRepository` (`flags.go`: `Admin` also implements `Flags` and `FlagAdmin`; `FlagSpecs`) |
| `adapters/postgres/` | `Repository` over `platform/pg` (both outgoing ports), sqlc queries in `queries/`, generated code in `sqlcgen/` (entry in `services/server/sqlc.yaml`) |
| `adapters/keystore/` | `Store`: the key store `secrets.signing_key_dir` over the secret provider (Ed25519, PKCS#8 PEM, `Thumbprint` after RFC 7638) |
| `adapters/httpapi/` | `Keys`: the REST handlers `getJwks` and `rotateSigningKey`; `Admin`: `getDiscovery`, `getInstance`, `listInstanceSettings`, `putInstanceSetting`, `updateBranding`, `getProblemType`, `listFeatureFlags`, `putFeatureFlag` |
| `adapters/jobs/` | `Register`, `Handler`: the periodic job `instance.key_rotation` |

Migration `00002_instance_counters.sql` creates both tables, the three counter rows with value 0
and the trigger function `instance_counter_monotonic()`, which refuses a lower value and a new
name for every role. `bothy_app` may read both tables and update only the column `value` of the
counters; it never inserts or deletes a counter row and never writes the instance row.

### Counters (`Counters`)

`Advance(ctx, name, n)` locks the counter row until the commit (`UPDATE ... SET value = value + n
... RETURNING value`) and returns the new value; the caller's numbers are `last - n + 1 .. last`.
A waiting transaction rereads the committed value in READ COMMITTED, so numbers are gapless and
follow the commit order, also with rollbacks (04 "Muster mit Zähler-Zeile"). `Current(ctx, name)`
reads without a lock. Nothing in this module runs `SELECT ... FOR UPDATE` on the counters.

Phase guard (02 "Sperrreihenfolge"): the service reads `txn.CurrentPhase(ctx)` and allows
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

`LockForSetup(ctx)` and `CompleteSetup(ctx, at)` (M1-37) run inside the caller's unit of work for
`bothy setup bootstrap-owner`: the first locks the row (`SELECT ... FOR UPDATE`), so two runs
serialise, the second sets `setup_completed_at` once (a set value never changes, kind `conflict`).
`bothy_app` may do both through its column right `UPDATE (setup_completed_at)` (migration
`00016_identity_enrollment_codes.sql`); every other column stays read only. The setup gate of the
public listener reads the state through `Load` (`wire.InstallSetupGate`).

`bothy migrate up` calls it through the after-step of `wire.MigrationSteps` with the goose
version and `migrations.MinServerVersion`, only when the database is at the newest migration the
binary knows; a newer database keeps its row (upgrade.md "Rückweg").

### Start check

After the schema check the start loads the record (missing row: the start ends with exit code 1
and the hint to run `bothy migrate up`), keeps it for later tickets (setup state, `instance_id`,
`sync_epoch`) and runs `CheckServerVersion(own version, min_server_version)` once. Below the
minimum the process keeps running for every role, the readiness check
`schema_requires_newer_server` fails for good and the log line
`server version below instance.min_server_version` carries `bothy.version.own` and
`bothy.version.required`; `/readyz` names only the check. An empty minimum is no lock.

### Signing keys (`Keys`, M1-32)

Normative: [19-sicherheit.md](../../../../docs/spec/19-sicherheit.md) section 10,
[09-agenten.md](../../../../docs/spec/09-agenten.md) section 4.5 and
[ADR 0066](../../../../docs/adr/0066-signierschluessel-im-beschreibbaren-schluesselspeicher.md).

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
checkpoints verify through `Verify` in any state).

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
returns a key in any state. `Verify(ctx, purpose, compactJWS)` (M1-50) parses a compact JWS with
`kernel/jose` (algorithms of `server_signing_keys` only, never `none`), reads the key its header
`kid` names fresh from the database in any state including `retired`, requires the purpose and the
algorithm of the row and returns payload and `kid`; every failure about the JWS wraps
`instance.ErrJWSInvalid`, a database error does not. The audit checkpoints verify through it. `JWKS` authorizes `instance.discovery.read` and publishes the `next`,
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

### Settings, branding and discovery (M1-46)

Normative: [01-produkt.md](../../../../docs/spec/01-produkt.md) sections 9 and 12, 03 table
`instance_settings`, [06-api.md](../../../../docs/spec/06-api.md) sections 5, 8 and 13 and
[ADR 0044](../../../../docs/adr/0044-arbeitsname-und-branding-als-konfiguration.md). Rulings:
[DECISIONS.md](../../../../docs/DECISIONS.md) M1-46.

**Settings registry.** `domain.Registry` holds one entry per key of
[configuration.md](../../../../docs/operations/configuration.md) section 6 with its JSON type,
default and route: `branding.accent_color` (`spruce`, an accent key of `kernel/designkeys`),
`branding.logo_file_id` (no default, UUIDv7, written only by M3-10), `branding.product_name`
(`DefaultProductName`, trimmed, 1 to `branding.product_name_max_chars` characters, the display text
check of `kernel/textcheck`) and `locale.default` (`de`, one of `SupportedLocales()`).
`TestSettingsRegistryMatchesConfigurationDoc` parses section 6 and fails on a difference. Migration
`00017_instance_settings.sql` creates the table; `bothy_app` reads, inserts and updates and never
deletes a row. A key without a row has its default and version 0; the stored row wins.

**Writes.** `putInstanceSetting` (owner, step-up, optional `If-Match`) rejects unknown keys (`404`)
and branding keys (`400`, `#path/key`, `use_branding_endpoint`), validates the value at `/value`,
takes the advisory lock of all settings writers and locks the row (so two writers of keys
without a row cannot pass each other, DECISIONS M1-46/14), compares `If-Match` with the current version (`412 version_conflict` with
`current`), writes nothing for the same value and otherwise inserts version 1 or raises the version
by one. `updateBranding` (owner, step-up, `If-Match` required) writes the two branding keys whose
value changes; an absent accent writes the default. Both append `instance.updated` with the sorted
changed keys (resource `instance`, receive action `instance.read`) and write the allow audit with
the key or the changed keys, never a value. Owner, step-up and the deny audit come from the
Authorizer.

**Branding version.** `Branding.Version`, the ETag of `updateBranding`, is the sum of the three
branding row versions (absent row 0); every write raises it.

**Discovery and the instance view.** `getDiscovery` is public and works before setup:
`setup_required` comes from `instance.setup_completed_at` on every call, the spec version, the
gateway protocol versions and the problem catalogue from the generated layer `api/routes`
(`app.Contract`, built in `wire.Contract`), the sign-in ways from the source port
`LoginOptionsSource`, the client versions from the configuration (`client.min_version.<p>`,
`client.recommended_version.<p>`). `getInstance` serves every active principal with the limits of
the configuration and `features`, the registered flags evaluated for the caller (M1-47); its ETag is a hash of the body
(`domain.ContentETag`). `getProblemType` serves every code of `x-problem-codes`.

**Source port.** `LoginOptionsSource` is declared here, because `instance` may not import
`identity`; `identity/adapters/loginoptions` implements it over the provider store of M1-36 and
`auth.local_password_enabled`, and `internal/wire` (`NewInstanceSettings`, start step `settings`
before `identity`) passes it in. `identity` reads `locale.default` through `Settings` for
`createUser`.

### Feature flags (M1-47)

Normative: [16-betrieb.md](../../../../docs/spec/16-betrieb.md) 6.2 (R-OP-14),
[environments-and-flags.md](../../../../docs/operations/environments-and-flags.md) 2, 03 table
`feature_flags` and [ADR 0064](../../../../docs/adr/0064-feature-flag-registrierung-mit-aufraeum-gate.md).
Rulings: [DECISIONS.md](../../../../docs/DECISIONS.md) M1-47.

**Registry.** The one registry is `registry` in `flags.go`, read through `RegisteredFlags()`
(sorted by key): key, kind (`release` or `option`), English description, default, owning module
and, for `release` only, the server version `remove_by`. Kind, description, default and module
live only there, never in the table. A flag only switches a function on or off: it never encodes
a number, a limit, a security boundary or a secret (16 6.2 "Grenze"); no check can see that
intent, so the review of every new flag holds it. `TestRegisteredFlagsValid` runs
`domain.CheckRegistry` on the real registry, and `make flag-check` (`tools/flagcheck`) runs the
same rules, fails when `services/server/VERSION` reaches `remove_by` of a registered `release`
flag, and compares the registry with the table of
[configuration.md](../../../../docs/operations/configuration.md) 7.

**Evaluation.** `Flags.Enabled(subject, key)`: an unknown key is off without a read; otherwise one
primary-key read of `feature_flags`, the global value is the row or the default, and it is
inverted when the subject is a person who stands in `rollout.user_ids` or has a valid group in
`rollout.group_ids`. Valid groups come from `FlagGroupSource.ValidGroupIDs` at the time of
`kernel/clock`, read once per call and only when the rollout names a group; a membership with
`expires_at = t` is already invalid at `t` (07 2.5). Agents and apps get the global value.
`EvaluateAll` reads all rows in one query and returns the registered flags only; orphaned rows
(stored keys without registration) are off and never evaluated. There is no cache, and errors are
returned, never turned into off.

**Operations.** `listFeatureFlags` (owner) lists every registered flag (a flag without a row has
its default, `version = 0` and no `updated_by` or `updated_at`) and then every orphaned row
(`orphaned: true`, without kind, description, default, module and `remove_by`), sorted by key.
`putFeatureFlag` (owner, step-up, reason, optional `If-Match`) answers `404` for a key that is not
registered, `400` for a blank reason (`/reason`) and for rollout IDs that are not people
(`/rollout/user_ids/<i>`, `unknown_user`, through `FlagUserSource`) or not live groups
(`/rollout/group_ids/<i>`, `unknown_group`, through `FlagGroupSource`), then under the row lock
`412 version_conflict` with `current`; the same value and normalized rollout writes nothing,
otherwise it inserts version 1 or raises the version, appends `instance.updated` with
`changed_keys = [key]` and writes the allow audit with `key`, `enabled`, `rollout_user_ids`,
`rollout_group_ids` (IDs only) and `reason`. `getInstance` returns `EvaluateAll` for the caller.

**Source ports.** `FlagGroupSource` is implemented by `authz/adapters/flagsource` (the M1-27
validity `Membership.InEffect` over the authz store), `FlagUserSource` by
`identity/adapters/flagsource` (principals of kind `user` that are not `deleted`);
`internal/wire` (`NewInstanceSettings`) passes both in. identity asks `tokens.personal_enabled`
for `createToken` through `identity/adapters/instanceflags` over `Flags`.

**Adding a flag.** Add an entry to `registry` with a new key constant; for an `option` add the row
to configuration.md 7; for a `release` set `remove_by` to a later server version. Check the flag
with `Flags.Enabled` where its function runs. `make flag-check` must pass.

**Removing a flag.** When `make flag-check` reports `remove flag <key>`, remove the code that reads
it, its registry entry, its configuration.md row (if any) and its row with a new migration
(`DELETE FROM feature_flags WHERE key = '<key>'`) in the same change. A `release` flag never
becomes an `option`; a lasting switch gets a new key.

## Port (Wurzelpaket `internal/instance`)

- Eingehend: `Settings`, `Flags`, `Keys` (Signer je Zweck, JWKS, Rotation), `Counters` (nur für die Phasen-Hooks von `authz`, `sync`, `audit`), `Versions` (Zählerstände ohne Sperre lesen, z. B. `permission_version` für den Rechte-Cache, `event_seq` für Catch-up); deklariert `LoginOptionsSource` (Anmelde-Treiber und Passwortweg für die Discovery), `FlagUserSource` (Menschen eines Rollouts), `FlagGroupSource` (gültige Gruppen eines Menschen für die Auswertung).
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: keine.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: keine.

Grund: Fundament.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`instance`, `instance_settings`, `server_signing_keys`, `feature_flags`, `sequence_counters`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`instance.discovery.read`, `instance.read`, `instance.settings.read`, `instance.settings.update`, `instance.branding.update`, `feature_flag.read`, `feature_flag.update`, `signing_key.rotate`, `signing_key.transition`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `instance.updated`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `instance.key_rotation` | periodisch alle `signing_key.rotation_check_interval`, Zustände `next` bis `retired` (M1-32) | `maintenance` |

## Hinweise für den Bau

- Sequenz-Zähler (`sequence_counters`) gibt das Modul nur über `Counters` an die Phasen-Hooks von `authz`, `sync` und `audit` heraus (02, 8.1 und 8.2); kein Anwendungsfall sperrt sie selbst.
- Private Schlüssel liegen nur im Secret-Provider (`private_key_ref`); die Tabelle hält öffentliche JWK und Zustand.
- Einstellungen sind Daten des Owners mit Schema je Schlüssel, keine Server-Konfiguration (02, Abschnitt 13).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [06-api.md](../../../../docs/spec/06-api.md), [16-betrieb.md](../../../../docs/spec/16-betrieb.md), [19-sicherheit.md](../../../../docs/spec/19-sicherheit.md)
