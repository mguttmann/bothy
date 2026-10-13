# Module `audit`

Status: built in part (M1-24): the hash chain of `audit_events`, the recorder behind the kernel port `auditing.Recorder` with the audit phase hook, the aggregation of identical denials and the keys of the IP pseudonyms (`audit_ip_keys`). Verification, checkpoints (`audit_checkpoints`), the job `audit.chain_verify`, the port `Log` and the operations `listAuditEvents`, `exportAuditEvents`, `verifyAuditChain` come with M1-50; truncation with an anchor and the destruction of IP keys with M7.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Audit log with hash chain, signed checkpoints, keys of the IP pseudonyms, query, verification, truncation with anchor.

## Code

| Package | Content |
|---|---|
| `internal/audit` | port root package; `Log` comes with M1-50 |
| `internal/kernel/auditing` | the cross-cutting port: `Recorder`, `Entry`, `Decision`, `Reason` with `KnownReasons` (the closed list of 07 "4.1"), the typed values `ID`, `Code`, `Int`, `Bool`, `Time`, `List`, `ReasonText`, `Violations` |
| `domain/` | `Row`, `CanonicalRow`, `ChainHash`, `ZeroHash`, `DetailsV1` with `Validate`, `IPHMAC`, `KeyPeriod`, `SealIPKey` and `OpenIPKey` (AES-256-GCM), `DecodeKEK`, the aggregation `Aggregation` with `AggregateKey` and `Bucket` (time always passed in) |
| `app/` | `Recorder` (implements `auditing.Recorder`), `Chain` (audit phase hook), `Aggregator` (`Run`, `FlushDue`, `Flush`), `Keyring`; outgoing ports `Store`, `KEKSource`, `Reporter` |
| `adapters/postgres/` | `Repository` over `platform/pg`, sqlc queries in `queries/audit.sql`, generated code in `sqlcgen/` (entry in `services/server/sqlc.yaml`) |
| `adapters/logs/` | `Reporter` over the server logger: constant messages, catalogue keys only |

`internal/wire` (`audit.go`) builds the module (`wire.NewAudit`), registers `(*Chain).Hook` as the
hook of phase 3.3 in `wire.NewTxnRegistry` (golden file `testdata/txn_registrations.golden`),
starts the aggregator in start step 6a of `bothy serve` (both roles) and registers the stop step
`audit_denials`, which flushes the open aggregations before the pool closes.

### Tables and grants

Migration `00004_audit_events.sql` creates `audit_events` and `audit_ip_keys` exactly as 03
"4.12" states them. `bothy_app` may only `SELECT` and `INSERT` on `audit_events`; `UPDATE`,
`DELETE` and `TRUNCATE` are revoked, and the triggers `audit_events_no_update_delete` (row,
`BEFORE UPDATE OR DELETE`) and `audit_events_no_truncate` (statement, `BEFORE TRUNCATE`) refuse
them for every role, the table owner included; only a `DELETE` by `bothy_maint` passes, for the
retention trim of M7, which also brings the grant. On `audit_ip_keys` `bothy_app` may `SELECT`,
`INSERT` and `UPDATE` only `key_ciphertext` and `destroyed_at`.

### Write path

There is one path for every entry: the audit phase hook. `Record` checks the entry and appends
the row to a buffer of the unit of work (`txn.Store`); outside a unit of work, or in the audit
phase itself, it is an error of kind `internal`. `Chain.Hook` does nothing for an empty buffer, so
a unit without entries never locks `audit_seq`. Otherwise it reserves `n` numbers with
`instance.Counters.Advance(audit_seq, n)` (the row lock holds until the commit, last in the lock
order of 03 "6.3"), reads the hash of row `first - 1` (`ZeroHash` for row 1; a missing row is
`audit chain broken`, logged, and fails the unit), computes the hashes in order and inserts all
rows in one statement. If the hook fails, the business transaction fails. `RecordDetached` and the
aggregator run the same hook in an own unit of work that never joins the ambient one
(`txn.Detach`). Storage and chain errors are kind `internal` (`internal_error` towards clients);
details go to the log only.

### Canonical form

`CanonicalRow` is the input of `hash = SHA-256(prev_hash || canonical)` (19 SEC-AUD-4): the column
names except `hash`, NULL columns omitted, `seq` as integer (at most `domain.MaxSafeInteger`), UUIDs in lower
case with hyphens, `occurred_at` as [RFC 3339](https://www.rfc-editor.org/rfc/rfc3339) (retrieved 2026-10-13) UTC with millisecond precision and `Z`
(the recorder cuts the time to milliseconds before storing and hashing), `prev_hash` as lower-case
hex, `details` as stored. Serialisation follows [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785) (retrieved 2026-10-13) in the restricted domain of the log:
sorted ASCII keys, strings escape only `"`, `\` and characters below U+0020, everything else
literal UTF-8, integers in decimal, no floats, no `null`, no NUL. `Repository.Rows` reads rows back
with `UseNumber`, so the hash recomputes from the stored row; the API representation `AuditEvent`
without `hash` is exactly this object (M1-50 exports it). `testdata/chain_v1.json` holds a golden
chain whose hashes were computed independently.

### Details schema `audit_details.v1`

A flat object (19 SEC-AUD-5): the reserved members `ip_hmac` (the hex form of the whole HMAC) with `ip_key_id`
(only together), `aggregate` (`count >= 1`, `first_at`, `last_at`), `violations` (list of `code`,
`agent_id`, `credential_holder_id`, `principal_ids`; only through `auditing.Violations`) and
`reason` (the only free text, only through `auditing.ReasonText`, checked with
`kernel/textcheck`; its length bound `audit.reason_max_chars` belongs to the request validation of
the action). Callers may not set `ip_hmac`, `ip_key_id` or `aggregate`. Every other member has a
key matching `^[a-z][a-z0-9_]*$` and a value that is a UUID, a code (`^[a-z0-9][a-z0-9_.:-]*$`, at
most `domain.MaxCodeLen` bytes, never an IP address), an integer, a boolean, a canonical time or a flat list of one of
these kinds. Anything else is refused when the entry is recorded (a programming error, kind
`internal`).

### Reasons

`auditing.Reason` is the single closed list of 07 "4.1" (R-DM-28); `authz` reuses it with M1-28.
`TestReasonsMatchChapter07` parses the chapter table and compares both ways. The column
`deny_reason` has no `CHECK`: a code outside the list is written anyway and logged as an error.

### Aggregation of denials

`RecordDenial` is for denials whose business transaction rolls back (05 "2.4", R-AZ-04, SEC-AUD-6).
Key: the actor principal, or for anonymous requests the IP pseudonym, plus action and reason. The
first denial of a key opens a bucket with `first_at`; identical denials within
`audit.deny_aggregate_window` from `first_at` raise `count` and `last_at`. A bucket becomes one row
with `occurred_at = first_at`, the actor, resource and correlation of its first denial and
`details.aggregate`: when its window ends (`Aggregator.Run` waits for the earliest deadline), at
`Flush` and at shutdown (`Close`, which reports what it cannot write as dropped), or at once when `audit.deny_aggregate_max_keys` buckets are held and a
new key arrives (the oldest is written). A failed write keeps the buckets and retries after one
window; only a bucket that leaves the cap and still cannot be written is dropped, logged with
action and reason. A denial whose IP pseudonym key is unavailable is kept without IP fields and
logged.

### Keyring of the IP pseudonyms

`ip_hmac = hex(HMAC-SHA-256(key, text))` with the client IP of `kernel/reqmeta` after `Unmap()`
without zone, in the text form of [`netip.Addr.String()`](https://pkg.go.dev/net/netip#Addr.String) (retrieved 2026-10-13); raw addresses are stored nowhere in
`audit_events`. Every entry of a request with a client IP carries `ip_hmac` and `ip_key_id`. One
key per `privacy.audit_ip_key_period`, `valid_from` aligned in Unix time; on a cache miss the
keyring reads the row covering now or creates one in an own short transaction (`domain.IPKeyBytes` random bytes,
AES-256-GCM under the key behind `privacy.audit_ip_kek_ref` with the row id as additional data,
`ON CONFLICT (valid_from) DO NOTHING`, then read again) and caches the decrypted key until
`valid_until`. A destroyed key (`key_ciphertext` NULL) is never used. `bothy serve` resolves the
key-encryption key at start, so a missing or malformed secret ends the start.

### Limits

- Aggregation is per process: several processes may each write one row per key and window. A
  hard crash loses the open buckets of at most one window (residual risk of SEC-AUD-6 and
  threat model T-AZ-04).
- The keyring creates a key in its own transaction; on the first entry of a new period this needs
  a second pool connection while the caller holds one.
- Logs of the module carry action, reason and `error.type`; the count of dropped denials and the
  number of a missing row are not logged, because the attribute catalogue has no key for them.
- Not proven here: end-to-end AZ-P13 through `authorize` (M1-28, M1-29) and the rest of SEC-T16
  (detection of a changed row and a rewritten chain, M1-50).

## Port (root package `internal/audit`)

- Inbound: `Log` (query, verification).
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: kernel `auditing.Recorder` (`app.Recorder`).
- Phase hook: `audit` (phase 3.3, `app.(*Chain).Hook`).

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
