# Modul `instance`

Status: built in part (M1-15): the instance row, the sequence counters with the port `Counters`, the port `State` and the start check of `instance.min_server_version`. M1-28 adds the lock-free read of the rights version (`Versions.PermissionVersion`, `app.NewVersions`) for the rights cache of authz. Settings, flags, signing keys and the further reads of `Versions` (`event_seq` for catch-up) come with M1-46, M1-47, M1-32 and the tickets that need them.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Instanz-Identität, Sync-Epoche, Mindest-Server-Version, Einstellungen und Branding, Feature-Flags, Server-Signierschlüssel mit JWKS und Rotation, Sequenz-Zähler.

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
| `instance.key_rotation` | periodisch, Zustände `next` bis `retired` | `maintenance` |

## Hinweise für den Bau

- Sequenz-Zähler (`sequence_counters`) gibt das Modul nur über `Counters` an die Phasen-Hooks von `authz`, `sync` und `audit` heraus (02, 8.1 und 8.2); kein Anwendungsfall sperrt sie selbst.
- Private Schlüssel liegen nur im Secret-Provider (`private_key_ref`); die Tabelle hält öffentliche JWK und Zustand.
- Einstellungen sind Daten des Owners mit Schema je Schlüssel, keine Server-Konfiguration (02, Abschnitt 13).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [06-api.md](../../../../docs/spec/06-api.md), [16-betrieb.md](../../../../docs/spec/16-betrieb.md), [19-sicherheit.md](../../../../docs/spec/19-sicherheit.md)
