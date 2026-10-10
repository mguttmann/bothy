# M1-24: Audit: hash chain, recorder and aggregation (ADR 0021)

Status: ready | Depends on: M1-15, M1-18, M1-26 | Etappe: M1

## Goal

After this ticket every module can write the audit log through one port, `kernel/auditing`.
Entries of a business transaction land in `audit_events` in the same transaction, chained by
`audit_seq` and SHA-256 over a canonical form (RFC 8785); denials whose business transaction
rolled back land in their own transaction, and identical denials are aggregated per
`audit.deny_aggregate_window`. The application role can only insert and read. Client IP
addresses appear only as keyed HMAC with a key id. M1-50 (verification, checkpoints, API) and every
audited action of M1 build on this.

M1-18 is needed for the correlation ID and the client IP in the request context
(`internal/kernel/reqmeta`); M1-11 (shutdown step for the aggregation flush) comes with it.
M1-26 is needed for the shared text validator `internal/kernel/textcheck` that checks `reason`.

## Read first

- `docs/adr/0021-audit-log-hash-verkettet-und-getrennt-von-telemetrie.md` (all of it).
- `docs/spec/03-datenmodell.md`: "1. Grundsätze" (1, 2, 8, 9, 11, 13), "3. Aufbewahrungsklassen"
  (`audit`), "4.1" table `sequence_counters`, "4.12 Audit, Datenschutz, Status" tables
  `audit_events`, `audit_checkpoints` and `audit_ip_keys`, "6.1 Globale Ereignis-Sequenz", "6.3 Sperrreihenfolge",
  "6.4 I1-Prüfung" (denial audit in an own transaction), R-DM-28 in the decisions table.
- `docs/spec/04-sync-protokoll.md`: "3.2 Muster mit Zähler-Zeile", "3.3 Weitere Zähler".
- `docs/spec/05-aktionen.md`: "1. Verbindliche Regeln" (rules 5 and 10), "2.4 Spalten Owner,
  Step-up, Audit, Rechte" (aggregation, R-AZ-04), "2.5 Entscheidung", "10. Konfigurationswerte"
  (`audit.deny_aggregate_window`, `audit.deny_aggregate_max_keys`).
- `docs/spec/07-rechte.md`: "4.1 Entscheidungen und Gründe" (closed list of reason codes), "11.2
  Pflicht-Tests" (AZ-P13).
- `docs/spec/02-architektur.md`: "4.2 Modulliste" (`audit`), "4.3 Ports, Ereignisse und Quellen je
  Modul", "4.4 Erlaubte Abhängigkeiten" (`audit` may use `instance`), "4.5 Querschnitts-Ports"
  (`kernel/auditing`), "5. Schichten im Modul", "7.3 Weitere Prüfungen im Gate", "8.1 Ablauf"
  (phase 3.3), "8.2 Sperrreihenfolge", "8.5 Was nie in einer Transaktion passiert", "14.1
  Fehlerarten", "15.3 Shutdown".
- `docs/spec/19-sicherheit.md`: "1. Vertrauensannahmen", "9." (SEC-LOG-4, SEC-LOG-5), "10.
  Geheimnisse" (SEC-SEC-3), "12. Lieferkette, Sicherungen und Audit-Integrität" (SEC-AUD-1,
  SEC-AUD-4 canonical form, SEC-AUD-5 schema `audit_details.v1`, SEC-AUD-6 aggregation),
  "14. Tests" (SEC-T16), "16. Konfigurationswerte".
- `docs/spec/20-datenschutz.md`: "4.5 Audit-Log und IP-Adressen", "10. Tests" (DP-T02, DP-T06),
  "13. Konfigurationswerte" (`privacy.audit_ip_key_period`, `privacy.audit_ip_key_retention`,
  `privacy.audit_ip_kek_ref`); `docs/operations/configuration.md` section 5 (`audit_ip_kek`).
- `docs/spec/16-betrieb.md`: "7.1 Rollen" (`bothy_app`, `bothy_maint`, `bothy_owner`).
- `docs/spec/17-observability.md` rule 6 and R-OB-07, `docs/observability/audit-vs-telemetry.md`.
- `docs/api/openapi.yaml` schemas `AuditEvent`, `AuditDecision`, `Timestamp` (the canonical form
  equals this representation without `hash`).
- `docs/PLAN.md`: section 7, rows M1-13, M1-15, M1-16, M1-18, M1-24, M1-28, M1-29, M1-50, M1-69;
  "Voraussetzungen aus dem Kern-Vertrag" (rows for M1-24); decisions P-09, P-11, P-14.
- `docs/security/threat-model.md` register row T-DB-01.
- `services/server/internal/audit/README.md`, `services/server/README.md` (kernel table).
- Code merged by earlier tickets: `internal/kernel/txn` and the unit of work in `internal/platform/pg`
  (M1-13, phase hooks, golden file of registrations), `internal/instance` port `Counters` (M1-15),
  `internal/platform/pg/pgtest` (M1-12), `internal/platform/lifecycle` (M1-11),
  `internal/kernel/actor` and the correlation ID and client IP that `internal/platform/httpserver`
  puts into the request context (M1-18), `internal/platform/config` (M1-07),
  `architecture/modules.yaml`, `sqlc.yaml` (M1-05), `make migrations-lint` (M1-01).

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision,
   a CHANGELOG entry per release, docs site kept current. A change without matching documentation
   is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour change starts with a failing test (red output goes into the
   report), then code until green, then cleanup.
7. No en dash and no em dash in any file you write. White label: examples use `example.com`,
   people Alex, Sam, Robin, addresses from RFC 5737 and RFC 3849.
8. Configuration only through named keys listed in `docs/operations/configuration.md`.
9. ADR 0021 and 03 `audit_events`: `seq` from `sequence_counters('audit_seq')` in the same
   transaction; `prev_hash` is the `hash` of row `seq - 1`, 32 zero bytes for the first row;
   `hash = SHA-256(prev_hash || canonical_json(row without hash))` with JCS (RFC 8785).
10. 03 "6.3 Sperrreihenfolge": `audit_seq` is locked last, after `event_seq`; 02 "8.1": the audit
    phase hook is phase 3.3.
11. SEC-AUD-1 and 03: the application role has only `INSERT` and `SELECT` on `audit_events`;
    `UPDATE` and `DELETE` are blocked by `REVOKE` and a trigger.
12. 05 "2.4" (R-AZ-04) and SEC-AUD-6: every denial is audited; identical denials (same principal,
    or same IP pseudonym without login; same action; same reason) are aggregated per
    `audit.deny_aggregate_window` into one row per key, written at window end, at shutdown or when
    `audit.deny_aggregate_max_keys` keys are open (then the oldest).
13. 03 "6.4": a denial whose business transaction rolled back is written in an own transaction.
14. 20 "4.5" and K-DP-2: IP addresses only as `ip_hmac` with `ip_key_id`, one key per
    `privacy.audit_ip_key_period`, stored encrypted in `audit_ip_keys` (03) under the key of
    `privacy.audit_ip_kek_ref`.
15. ADR 0021, SEC-LOG-4, SEC-SEC-3: no content, names, e-mail addresses or secrets in `details`.
16. 17 rule 6: the audit log is not telemetry; no audit rows leave through the collector.

## Scope

All Go paths are below `services/server/`.

0. **Core contract and spec.** 03 defines `audit_ip_keys` (module `audit`; `key_ciphertext`
   AEAD-encrypted, `key_ref`, `destroyed_at`, checks and rights as listed there); 19 defines the
   canonical form (SEC-AUD-4), the schema `audit_details.v1` (SEC-AUD-5) and the aggregation
   (SEC-AUD-6); 05 holds `audit.deny_aggregate_max_keys`; 20 "4.5" says where the keys live.
   Build exactly that; change a chapter only if the build proves it wrong, in the same change.
1. **Configuration and secrets.** Schema entries `audit.deny_aggregate_window`,
   `audit.deny_aggregate_max_keys`, `privacy.audit_ip_key_period`, `privacy.audit_ip_kek_ref`; the
   catalogue entry `audit_ip_kek` (generator `key`, service `server`) in the secrets catalogue of
   M1-09 (find it with `grep -rn token_pepper services/server/internal/platform/secrets`).
   Regenerate section 4 of `docs/operations/configuration.md` if the schema adds keys.
2. **Port `internal/kernel/auditing`** (fill the package that M1-05 created):

   ```go
   type Decision string // "allow", "deny", "error" (audit_events.decision)
   type Reason string   // closed list of 07 "4.1 Entscheidungen und Gründe" (deny_reason)
   const ReasonUnauthenticated Reason = "unauthenticated" // ... one constant per row of 07 4.1
   func KnownReasons() []Reason

   // Entry is one auditable decision. Actor, client IP and correlation ID come from ctx.
   type Entry struct {
       Action       string   // action id from 05, system actions included
       Decision     Decision
       Reason       Reason   // empty for allow
       ResourceKind string   // optional
       ResourceID   ids.ID   // optional; zero value means none
       Attrs        Attrs    // optional; flat members of details (19 SEC-AUD-5)
   }
   type Attrs map[string]Value // keys must not collide with ip_hmac, ip_key_id, aggregate, violations
   type Value struct { /* unexported */ }
   func ID(v ids.ID) Value
   func Code(s string) Value // short machine code, never free text
   func Int(n int64) Value
   func Bool(b bool) Value
   func Time(t time.Time) Value
   func List(vs ...Value) Value // list of one of the kinds above
   // Violations is the reserved member for i1_violation (19 SEC-AUD-5); M1-29 fills it.
   type Violation struct { Code string; AgentID, CredentialHolderID ids.ID; PrincipalIDs []ids.ID }

   type Recorder interface {
       // Record adds e to the caller's unit of work; the audit phase hook writes it before commit.
       // Outside a unit of work it returns an internal error.
       Record(ctx context.Context, e Entry) error
       // RecordDenial aggregates a denial whose business transaction rolls back and writes it in
       // an own transaction later. It returns an error only for an invalid entry, never for storage.
       RecordDenial(ctx context.Context, e Entry) error
       // RecordDetached writes e at once in an own transaction, never joining an ambient one
       // (decision error, for example unknown_action).
       RecordDetached(ctx context.Context, e Entry) error
   }
   ```

3. **Module `internal/audit`**:
   - `domain`: `Row` (all columns of `audit_events`), `CanonicalRow(Row) ([]byte, error)`,
     `ChainHash(prev [32]byte, canonical []byte) [32]byte`, `ZeroHash`, `DetailsV1` with
     validation, `IPHMAC(key []byte, addr netip.Addr) string`, aggregation key and bucket logic as
     pure functions with the time passed in. Standard library only.
   - `app`: the `auditing.Recorder` implementation, the aggregator (`Run(ctx)` with a timer at the
     earliest bucket deadline, `Flush(ctx)`), the phase-3.3 hook logic, the keyring (current key,
     in-memory cache until `valid_until`), outgoing ports for storage. Imports only the own port,
     own domain, `kernel` and the `instance` port.
   - `adapters/postgres`: sqlc queries in `queries/audit.sql` (insert rows, read hash by `seq`,
     read current IP key, insert IP key with `ON CONFLICT (valid_from) DO NOTHING`, read key by
     id), repository, `sqlc.yaml` entry for `audit`.
   - `internal/wire`: construct the recorder, register the hook in the audit slot (phase 3.3) as a
     method value of the `audit` hook type (never a closure in `internal/wire`, 02 "Ablauf") and
     update the golden file of registrations, start the aggregator with the role's lifecycle and
     register a shutdown step that flushes it before the pool closes (both roles), fill the typed
     `audit` and `privacy` config.
4. **Migration** in `migrations/` with the next free sequential number (follow
   `make migrations-lint`): `audit_events` exactly as in 03 (columns, `CHECK` on `decision`, PK
   `seq`, unique `id` and `hash`, indexes `(actor_principal_id, occurred_at DESC)`,
   `(resource_kind, resource_id)`, `(action, occurred_at DESC)`, BRIN `(occurred_at)`); `REVOKE ALL
   ON audit_events FROM PUBLIC`; `GRANT SELECT, INSERT ON audit_events TO bothy_app`; `REVOKE
   UPDATE, DELETE, TRUNCATE ON audit_events FROM bothy_app` (default privileges may have granted
   them); a row trigger `BEFORE UPDATE OR DELETE` that raises for every role except a `DELETE` by
   `bothy_maint`, and a statement trigger `BEFORE TRUNCATE` that always raises. The IP key table
   from item 0 with its grants. No grants for `bothy_maint` here (the retention ticket of M7 adds
   them).
5. **Configuration schema**: `audit.deny_aggregate_window` (default `60s`, `> 0`),
   `audit.deny_aggregate_max_keys` (default `10000`, `> 0`), `privacy.audit_ip_key_period`
   (default `30d`), `privacy.audit_ip_key_retention` (default `90d`); cross check retention `>=`
   period.
6. **Register and docs**: `architecture/modules.yaml` lists the new table for `audit`;
   `internal/audit/README.md` (German: status, tables, port, hook, canonical form, details schema,
   aggregation, keyring, limits); package doc of `kernel/auditing` with an example;
   `docs/security/threat-model.md` row T-DB-01 notes the part of SEC-T16 this ticket proves;
   `docs/DECISIONS.md` entries only for the IP canonical form and the location of the reason list
   (the canonical form, `audit_details.v1` and the aggregation are in 19 and 05);
   `services/server/CHANGELOG.md` under `Unreleased` (`Added`, `Security`).

## Out of scope

- `audit.verify`, checkpoints, `audit_checkpoints`, the job `audit.chain_verify`, the operations
  `listAuditEvents`, `exportAuditEvents`, `verifyAuditChain` and the rest of SEC-T16 (detection of a
  changed row and of a rewritten chain): M1-50. Expose `CanonicalRow` and `ChainHash` so M1-50
  reuses them.
- Calling the recorder from `authorize` according to the "Audit" column of 05 (end-to-end AZ-P13):
  M1-28 and M1-29 own `authorize` and the rights changes. This ticket proves aggregation at the
  recorder.
- Truncation with anchor, grants for `bothy_maint`, destruction of IP keys after
  `privacy.audit_ip_key_retention` and DP-T06: M7 (P-14).
- Telemetry about audit entries: none (17 rule 6).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-24-audit-chain`, created from the base
  branch the launcher names. One writer per worktree.
- Conventional Commits with the module as scope (for example `feat(audit): hash chain in the
  commit phase`), each commit signed off for the DCO (`git commit -s`); squash into meaningful
  commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except `docs/DECISIONS.md`,
  `services/server/CHANGELOG.md` and generated references named above.
- The core contract (00, 03, 04, 05) is read-only (item 0 and P-11).
- Follow the host rules supplied by the launcher. PostgreSQL 18 only through `pgtest` as a
  throwaway container on a random port bound to `127.0.0.1` (P-09, P-10); without a Docker runtime
  the integration tests fail, they never skip.
- Stop only processes and containers you started, by PID or ID. Temporary directories only via
  `mktemp -d` (in Go tests `t.TempDir()`), removed at the end.
- Never read or print secrets; never open `.env` files.
- No network calls except package downloads for pinned tools and the pinned PostgreSQL image.
- Do not lower a threshold, disable a gate or delete a test.

## Design notes

1. **Write path.** One path for all entries: the phase-3.3 hook. `Record` appends to a
   per-transaction buffer held through the unit of work (use the per-transaction state kernel/txn
   offers; if it offers none, add a minimal one there and report it). The hook does nothing when the
   buffer is empty (no lock on `audit_seq`). Otherwise it reserves `n` numbers through
   `instance.Counters` (the lock on `audit_seq` holds until commit), reads `hash` of row `first - 1`
   (`ZeroHash` when `first == 1`; a missing row is an internal error `audit chain broken` that fails
   the transaction and is logged), computes the rows in order and inserts them in one statement.
   `RecordDenial` buckets and `RecordDetached` run the same hook in an own unit of work that never
   joins an ambient transaction. If the hook fails, the business transaction fails ("scheitert das
   Audit, scheitert die Änderung").
2. **Canonical form** of a row (the JSON object `canonical_json` hashes). Members are the column
   names except `hash`. `seq` is a JSON integer (reject values above 2^53 - 1); `id`,
   `actor_principal_id`, `actor_device_id`, `actor_session_id`, `resource_id`, `correlation_id` are
   lower-case hyphenated UUID strings; `occurred_at` is RFC 3339 UTC with exactly three fractional
   digits and `Z`, the API `Timestamp` form (the recorder truncates the time to milliseconds before
   storing and hashing, so the database round trip is exact); `action`, `decision`, `deny_reason`,
   `resource_kind` are strings; `details` is the object as stored; `prev_hash` is lower-case hex.
   NULL columns are omitted, never `null` (03 rule 13). Serialise per RFC 8785 in the restricted
   domain: object keys sorted (all keys are ASCII, so byte order equals UTF-16 order); strings
   escape only `"`, `\` and control characters below U+0020 (`\b \t \n \f \r`, otherwise `\u00xx`
   with lower-case hex); everything else, including non-ASCII, `<`, `>`, `&`, U+2028 and U+2029, is
   written literally in UTF-8; integers in decimal without exponent; no floats, no `null`. Values
   read back from `jsonb` are decoded with `UseNumber` so integers stay exact. With this form the
   API representation `AuditEvent` without `hash` is exactly the hashed object, so an exported log
   can be verified offline (M1-50 must export the same form).
3. **Schema `audit_details.v1`** exactly as 19 SEC-AUD-5: a flat object with the reserved members
   `ip_hmac`, `ip_key_id`, `aggregate` (`count` integer `>= 1`, `first_at`, `last_at` in the
   canonical time form), `violations` and `reason` (the only free text: the reason a management
   action requires, checked with `kernel/textcheck`; port function `ReasonText(s string) Value`,
   accepted only under the key `reason`), plus attribute members whose keys match
   `^[a-z][a-z0-9_]*$` and whose values are only the `Value` kinds of the port (UUID strings, codes,
   integers, booleans, times, lists of these). Codes match `^[a-z0-9][a-z0-9_.:-]*$` with a length
   bound held by a named constant and a test. No free text, no `null`, no floats, no NUL characters.
   A value outside these forms is rejected at `Record` time (a programming error); later tickets
   that need more extend the schema deliberately in the code and in 19.
4. **IP pseudonyms.** Input: the client IP from the request context, `Unmap()`, zone removed,
   text form of `netip.Addr.String()`. `ip_hmac = hex(HMAC-SHA-256(key, text))`, full length.
   Keys: one row of `audit_ip_keys` per period; `valid_from = floor(now / period) * period` in Unix
   time, `valid_until = valid_from + period`; on a cache miss the keyring reads the row covering
   `now`, or inserts a new one in an own short transaction: 32 random bytes from `crypto/rand`,
   encrypted with AES-256-GCM under the key behind `privacy.audit_ip_kek_ref` (hex-decoded to 32
   bytes, generator `key` of M1-09; random nonce stored
   in front of the ciphertext, the row `id` as additional data), `key_ref` set to that reference
   (`ON CONFLICT (valid_from) DO NOTHING`, then read again). The decrypted key is cached until
   `valid_until`. A destroyed key (`key_ciphertext` NULL) is never used. Raw IP addresses are stored
   nowhere in `audit_events`. The destruction job and DP-T06 belong to M7-05.
5. **Actor and correlation.** `actor_principal_id`, `actor_device_id`, `actor_session_id` come
   from `kernel/actor`; they are absent for system actions and server commands. `correlation_id`
   is the trace ID that M1-18 puts into the request context, rendered as UUID text; it is not a
   UUIDv7 and must not be validated as one (02 "Querschnitts-Ports"). Read correlation ID and
   client IP through `internal/kernel/reqmeta` (M1-18), never from `platform`.
6. **Aggregation of denials.** Key: actor principal (or, for anonymous requests, `ip_hmac`), action,
   reason. The first denial of a key opens a bucket with `first_at`; later identical denials within
   `audit.deny_aggregate_window` from `first_at` only raise `count` and `last_at`. A bucket is
   written as one row when its window ends, at `Flush`, at shutdown, or at once when
   `audit.deny_aggregate_max_keys` buckets are open and a new key arrives (the oldest is written).
   The row carries the actor, resource and correlation of the first denial, `occurred_at =
   first_at`, `decision = deny`, and `details.aggregate`. A denial after the window opens a new
   bucket. If writing fails, the buckets stay and the next timer retries; only when the cap is
   reached and the oldest bucket still cannot be written is it dropped with an `Error` log naming
   action, reason and count (principal only as ID). Aggregation is per process; several processes
   may each write one row per window. A hard crash loses the open buckets of at most one window
   (residual risk, 19 SEC-AUD-6 and threat model T-AZ-04); name it in the README.
7. **Reason list (R-DM-28).** `kernel/auditing.Reason` is the single closed list of 07 "4.1";
   `authz` reuses it in M1-28 instead of defining its own. The database has no `CHECK` on
   `deny_reason`; the recorder writes an unknown reason anyway (never lose an entry) and logs an
   `Error`.
8. **Errors.** Storage and chain errors are kind `internal` with code `internal_error` towards
   clients (02 "14.1"); details only in the log with the correlation ID.
9. **Never in a transaction:** no network calls (02 "8.5"); the keyring touches only PostgreSQL.

## Tests (write them first)

Unit tests in `internal/audit/domain` and `internal/audit/app` (fake clock from `kernel/clock`,
fake storage); integration tests in `internal/audit/adapters/postgres` and `internal/wire` with
`pgtest` (PostgreSQL 18).

- `TestCanonicalRow_Golden`: `testdata/chain_v1.json` with at least three rows (one with all
  optional columns, one with none, one with non-ASCII text in an attribute code is not allowed, so
  use a control character case through `deny_reason` text read from the database path instead);
  for at least one row the expected canonical string is written literally in the test, not
  computed; expected hashes are stored in the file.
- `TestCanonicalRow_Escaping`: quote, backslash, each control character class, `<>&`, U+2028 and
  a non-ASCII letter in a string value.
- `TestCanonicalRow_OmitsNull`, `TestCanonicalRow_KeyOrderIndependent`,
  `TestCanonicalRow_RejectsOutOfDomain` (float, `null`, non-ASCII key, integer above 2^53 - 1, NUL).
- `TestChainHash_FirstRowZeroPrevHash`, `TestCanonicalRow_TimeHasMilliseconds`.
- `TestDetailsV1_ClosedSchema`: reserved member with a wrong shape, attribute key colliding with a reserved member, free-text attribute, float, bad key, bad code rejected; `violations` accepted.
- `TestReasonsMatchChapter07`: parses the table in `docs/spec/07-rechte.md` section "4.1
  Entscheidungen und Gründe" (first column) and compares with `KnownReasons()` both ways.
- `TestIPKey_EncryptedAtRest` (the stored `key_ciphertext` differs from the key and decrypts only with the configured key), `TestIPHMAC_Vectors` (fixed key, `192.0.2.44` and `2001:db8::44`, literal expected hex),
  `TestIPHMAC_MappedEqualsIPv4`, `TestIPHMAC_ZoneIgnored`.
- `TestAZP13_IdenticalDenialsInWindowYieldOneEntry` (count equals the number of denials),
  `TestAZP13_DifferentKeysSeparate` (other principal, action or reason),
  `TestAZP13_NewWindowNewEntry`, `TestAZP13_AnonymousKeyedByIPHMAC`,
  `TestDenyAggregation_CapWritesOldest`, `TestDenyAggregation_FlushOnShutdown`,
  `TestDenyAggregation_WriteFailureKeepsBuckets`.
- `TestAuditChain_ConcurrentWritersGapless`: many goroutines run units of work that record
  entries, a random part of them roll back; afterwards `seq` runs from 1 without gaps, every
  `prev_hash` equals the previous `hash`, every `hash` recomputes from `CanonicalRow` read back from
  the database.
- `TestAudit_InTxEntryRolledBackWithBusinessTx`, `TestAudit_DenialSurvivesBusinessRollback` (deny
  after rollback, `Flush`, row present), `TestAudit_RecordOutsideTransactionFails`,
  `TestAudit_NoEntriesNoCounterLock` (`audit_seq` unchanged after a unit of work without entries).
- `TestSECT16_AppRoleCannotUpdateOrDelete`: connected as `bothy_app`, `UPDATE`, `DELETE` and
  `TRUNCATE` fail with SQLSTATE `42501`; `TestAudit_TriggerBlocksOwnerUpdate`: as `bothy_owner`
  the trigger raises on `UPDATE`.
- `TestAudit_IPStoredOnlyAsHMAC`: a request context with client IP `192.0.2.44` yields
  `details.ip_hmac` and `details.ip_key_id`; the row text does not contain `192.0.2.44`.
- `TestAuditIPKeys_OneKeyPerPeriodUnderConcurrency`, `TestAuditIPKeys_NewPeriodNewKey`.
- The golden file of hook registrations (M1-13) shows the audit hook in phase 3.3.
- `BenchmarkAuditAppend` (reported, not gating).

Commands that must pass: `make test`, `make migrations-lint`, `make arch`, `make codegen-check`,
`make register-check`, `make docs`, `make check`.

## Acceptance criteria

- [ ] The chain stays gapless under concurrency (`audit_seq`), and every hash recomputes from the
      canonical form of the row as read back.
- [ ] SEC-T16, part application role: `UPDATE`, `DELETE` and `TRUNCATE` by `bothy_app` are refused;
      the trigger also stops the table owner.
- [ ] AZ-P13 at the recorder: identical denials produce one entry with a counter per
      `audit.deny_aggregate_window`; different keys stay separate.
- [ ] A denial entry survives the rollback of its business transaction.
- [ ] IP fields only as `ip_hmac` with `ip_key_id`; one key per `privacy.audit_ip_key_period`.
- [ ] `details` follows `audit_details.v1`; unknown members, free text, floats and `null` are
      refused.
- [ ] The reason list equals 07 "4.1" (test).
- [ ] The module README, `modules.yaml` and `configuration.md` describe what was built; 03, 05,
      19 and 20 still match it.
- [ ] `make check` is green.

## Definition of done

- Code, tests and documentation from the scope are complete: doc comment on every public symbol
  (purpose, parameters, return value, errors, example for `Recorder`), module README,
  `docs/DECISIONS.md` entries named in Scope, CHANGELOG entry under `Unreleased` in
  `services/server/CHANGELOG.md`. ADR 0021 covers the architecture; write a new ADR only if you take
  a decision it does not cover.
- `make check` is green.
- Independent review: `make review` plus a code review over the diff; then the red team of
  `docs/quality/red-team-gate.md` over the diff with verdict `JA` or `NEIN`, recorded in the
  report. On `NEIN`: fix with a test per finding and have the delta reviewed again.
- Merge locally into the base branch only if review and red team are green and `make check` is
  green on the merged state; then `git worktree remove` the worktree.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, test results with counts (including the red output of the
first test run), what is not proven (at least: end-to-end AZ-P13 through `authorize`, the full
SEC-T16), and follow-ups that could not be done in scope with the reason. Then write
`{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
