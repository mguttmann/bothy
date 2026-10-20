# Module `audit`

Status: built in part (M1-24, M1-50, M7-22): the hash chain of `audit_events`, the recorder behind the kernel port `auditing.Recorder` with the audit phase hook, the aggregation of identical denials and the keys of the IP pseudonyms (`audit_ip_keys`) (M1-24); the port `Log` with query, export and verification, the signed checkpoints (`audit_checkpoints`) with the file sink, the job `audit.chain_verify` and the operations `listAuditEvents`, `exportAuditEvents`, `verifyAuditChain` (M1-50). The object store sink with retention lock (`s3://`, M7-22) is built. Truncation with an anchor and the destruction of IP keys come with M7.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Audit log with hash chain, signed checkpoints, keys of the IP pseudonyms, query, verification, truncation with anchor.

## Code

| Package | Content |
|---|---|
| `internal/audit` | port root package: `Log` (`List`, `Export`, `Verify`) with `Query`, `Page`, `Event`, `EventWriter`, `VerifyRequest`, `VerifyResult`, `WarningSinkNotConfigured` (M1-50) |
| `internal/kernel/auditing` | the cross-cutting port: `Recorder`, `Entry`, `Decision`, `Reason` with `KnownReasons` (the closed list of 07 "4.1"), the typed values `ID`, `Code`, `Int`, `Bool`, `Time`, `List`, `ReasonText`, `Violations` |
| `domain/` | `Row`, `CanonicalRow`, `EventJSON`, `ChainHash`, `ZeroHash`, `DetailsV1` with `Validate`, `IPHMAC`, `KeyPeriod`, `SealIPKey` and `OpenIPKey` (AES-256-GCM), `DecodeKEK`, the aggregation `Aggregation` with `AggregateKey` and `Bucket` (time always passed in); the chain walk `Walk` with `Failure`, `FailureReason` and `FirstFailure` (`verify.go`), the checkpoint payload `Checkpoint` with `NewCheckpoint`, `Canonical`, `ParseCheckpoint`, `CheckpointDigest` and the check of the sink chain `CheckCheckpoints` (`checkpoint.go`), `ParseSink` with `Sink`, `SinkKind` and `ValidBucket` (`sink.go`) |
| `app/` | `Recorder` (implements `auditing.Recorder`), `Chain` (audit phase hook), `Aggregator` (`Run`, `FlushDue`, `Flush`), `Keyring`; `Log` (implements `audit.Log`: `query.go`, `export.go`, `verify.go`, and `RunCheckpoint`, the job, in `checkpoint_job.go`), `NoSink`; outgoing ports `Store`, `KEKSource`, `Reporter`, `CheckpointSink`, `Cursors`, `Metrics` |
| `adapters/postgres/` | `Repository` over `platform/pg`, sqlc queries in `queries/audit.sql`, generated code in `sqlcgen/` (entry in `services/server/sqlc.yaml`) |
| `adapters/logs/` | `Reporter` over the server logger: constant messages, catalogue keys only |
| `adapters/filesink/` | `Sink`, the `file://` implementation of `CheckpointSink` |
| `adapters/s3sink/` | `Sink`, the `s3://` implementation of `CheckpointSink` over an S3-compatible object store with `Config`, `New` and the lock check `CheckLock` (M7-22); the second and last package that imports the AWS SDK |
| `app/sinktest/` | `Run`, the conformance suite of `CheckpointSink` that `filesink` and `s3sink` pass (only tests import it, `TestFilestestNotInBinary`) |
| `adapters/jobs/` | the periodic job `audit.chain_verify` on the queue `maintenance` |
| `adapters/httpapi/` | the handlers of `listAuditEvents`, `exportAuditEvents`, `verifyAuditChain` and the cursor codec `Cursors` |
| `adapters/otelmetrics/` | `bothy.audit.verify.failures` and `bothy.audit.verify.last_success.timestamp`, declared in the metric registry of `platform/otel/metrics` (M1-68) |

`internal/wire` (`audit.go`) builds the module (`wire.NewAudit`), registers `(*Chain).Hook` as the
hook of phase 3.3 in `wire.NewTxnRegistry` (golden file `testdata/txn_registrations.golden`),
starts the aggregator in start step 6a of `bothy serve` (both roles) and registers the stop step
`audit_denials`, which flushes the open aggregations before the pool closes. The start step
`audit_log` after `signing_keys` builds `Log` over the signing keys, the Authorizer and the
instance ID (`Audit.BindLog`, every role) and, in the roles `worker` and `all`, writes the warning
`audit_checkpoint_sink_not_configured` once when no sink is set; `wire.Handlers` embeds the
handlers, and the job registry of every role holds `audit.chain_verify` every
`audit.checkpoint_interval`.

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
the action) and `os_user` (M2-39: the name of a session user of a terminal host, only through
`auditing.OSUserName`, only in the form `^[A-Za-z0-9_][A-Za-z0-9_.-]*$`, never a path; M2-43
reuses it for `host.connect`). Callers may not set `ip_hmac`, `ip_key_id` or `aggregate`. Every other member has a
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
- Not proven here: end-to-end AZ-P13 through `authorize` (M1-28, M1-29). SEC-T16 is complete
  since M1-50 (`adapters/httpapi`, `migrations`).

### Checkpoints and the sink (M1-50)

Migration `00015_audit_checkpoints.sql` creates `audit_checkpoints` as 03 states it. `bothy_app`
may `SELECT`, `INSERT` and `UPDATE (exported_at)`; the trigger `audit_checkpoints_append_only`
refuses for every role every `UPDATE` except setting `exported_at` once with nothing else changed,
every `DELETE` except one by `bothy_maint` and every `TRUNCATE`.

A checkpoint (19 SEC-AUD-7) is a compact JWS with header `{alg, kid, typ: "audit-checkpoint+json"}`,
signed with the active key of purpose `audit_checkpoint` (`instance.Keys.Signer`), over the RFC
8785 payload `{instance_id, seq, hash, occurred_at, previous_checkpoint_hash}` of the covered row;
`previous_checkpoint_hash` is the SHA-256 of the previous canonical payload and absent for the
first. `audit_checkpoints.signature` holds the whole JWS.

`audit.checkpoint_sink` is empty (`NoSink`), `file://<absolute path>` or `s3://<bucket>[/<prefix>]`
(`ParseSink` returns a `Sink` with its `SinkKind`; the cross-check `audit_checkpoint_sink` of `bothy
config check` refuses every other value, also percent escapes, and names the supported schemes). The file sink writes `<path>/<instance_id>/<seq with 20 digits>.jws`, directory `0700`,
file `0600`: a temporary file with `O_EXCL`, synced, then a hard link to the name, which fails when
the name exists, then `fsync` of the directory. The same content again is accepted, other content
is an error and the file stays. `<path>` must exist: the sink never creates it. The file system
must support hard links; a link in place of the directory or of a file is refused.

### Object store sink (M7-22)

`audit.checkpoint_sink = s3://<bucket>[/<prefix>]` selects `adapters/s3sink` (`wire.CheckpointSink`
selects the sink by scheme). Every checkpoint is the object `[<prefix>/]<instance_id>/<seq with 20
digits>.jws`. `Put` reads the oldest version of the key first (the same bytes count as stored,
other bytes are `ErrCheckpointConflict`), then writes with `If-None-Match: *`; on a refusal of a
concurrent writer the stored bytes decide the same way. `List` pages through `ListObjectVersions`
below `[<prefix>/]<instance_id>/`, takes the oldest version of every checkpoint key (delete
markers are skipped) and returns the checkpoints in `seq` order. A process reads every version once: the bodies stay
in memory by key and version ID, and new ones are read with at most 16 concurrent GETs
(docs/DECISIONS.md M7-22/7), so a run of the job costs one listing and the new checkpoints. The bucket must have Object Lock
with a default retention in mode `COMPLIANCE` of at least `retention.audit`: `CheckLock` reads it,
and `bothy config check` (`wire.CheckCheckpointSink`) and the start step `audit_log` fail
otherwise. The sink has its own endpoint, region, path style and credentials
(`audit.checkpoint_s3_*`), goes through the egress client with purpose `audit_checkpoint`
(`egress.timeout`, `egress.max_response_bytes`), never uses the default configuration or
credential chain of the SDK, and holds only `PutObject`, `GetObject`, `ListObjectVersions` and
`GetObjectLockConfiguration`. Its errors name the operation and the S3 code or status, never
the endpoint or credentials. The tests run against the SeaweedFS image the files driver pins
(docs/DECISIONS.md M7-22/2).

### Job `audit.chain_verify`

`Log.RunCheckpoint`, every `audit.checkpoint_interval` on the queue `maintenance` (only `worker`
and `all` run River): without a sink it logs the warning; it checks the signature, the payload and
the row of the last checkpoint of `audit_checkpoints`, walks the rows after it (or from the first
row) to the head; on a failure it adds one to `bothy.audit.verify.failures`, records `audit.verify`
with decision `error`, no actor and `{first_broken_seq, failure_reason}`, logs `audit chain
verification failed` and ends without error (no retry, the alert `BothyAuditChainBroken` takes
over). Otherwise it signs the next checkpoint before the transaction and inserts it under an
advisory transaction lock (a checkpoint of a concurrent run in between makes it skip), then exports
every checkpoint with `exported_at IS NULL` in `seq` order and sets `exported_at`; a sink error fails
the run, River retries, nothing is signed twice. Last it sets
`bothy.audit.verify.last_success.timestamp`.

### Operations (05 `audit.read`, `audit.verify`)

All three authorize on the instance (owner with fresh step-up, `rl.admin`) and record their own
allow entry (20: every read of the log is audited).

- `listAuditEvents`: every item is the stored row as `EventJSON`, newest first by `seq`, keyset cursor bound to the caller and every filter
  (`actor_id`, `action`, `decision`, `resource_kind`, `resource_id`, `from` inclusive, `to`
  exclusive); `limit` per `api.page_size_default` and `api.page_size_max`; `action` and
  `resource_kind` must be codes. Details of the entry: the filters that are set.
- `exportAuditEvents`: `from` and `to` required; `application/x-ndjson`, one `EventJSON` line per
  row (the canonical row plus `hash`, so a line without `hash` is the input of its hash) in
  ascending `seq`, batches of `api.page_size_max`, flushed per batch, route class of downloads
  (deadline `http.transfer_timeout`). The stream is the contiguous `seq` span of the rows with
  `occurred_at` in the range, so every line links the previous one. An error after the first line
  aborts the response. Details: `{from, to}`.
- `verifyAuditChain`: reads the checkpoints only from the sink, checks typ, signature (any state of
  the key, `instance.Keys.Verify`), `instance_id` and the links (`checkpoint_invalid` at the first
  that fails), walks the range in batches of `api.page_size_max` (`hash_mismatch`,
  `link_mismatch`, `seq_gap`, `checkpoint_mismatch`), reports a trusted checkpoint whose row is gone
  as `checkpoint_mismatch`, and returns the failure with the smallest `seq`. `from_seq` defaults to
  the first retained row (and is raised to it), `to_seq` to the head (and is capped); a range that
  starts later trusts the stored hash of row `from_seq - 1`. Without a sink the result carries
  `checkpoint_sink_not_configured` and compares nothing; a sink that cannot be read is `503
  unavailable` (`dependency_unavailable`). Details: `{from_seq, to_seq, ok}`.

### Limits of the verification

- The job recomputes only the segment since the last checkpoint (SEC-AUD-2); a change to an older
  row is found by `verifyAuditChain` against the sink, not by the job and its alert.
- Whoever controls both the database and the sink can rewrite both (threat model T-DB-01); the
  removal of the newest checkpoint files together with a rewrite of the rows after the remaining
  ones is not detectable. The object store sink with retention lock (M7-22) closes this for a
  bucket in mode `COMPLIANCE`, whose objects nobody can delete or replace before the period ends.
- Until the anchors of M7 every walk from the first retained row starts at row 1, so a log whose
  first rows are gone is `seq_gap` at 1; the retention trim of M7 changes this rule with its
  anchors.
- Rows deleted after the newest sink checkpoint (the newest rows) leave a shorter chain that
  verifies; nothing outside the database knows them yet. The next checkpoint narrows this window
  to `audit.checkpoint_interval`.
- A row with a `seq` below 1 lies outside the chain and is `seq_gap` at that `seq`.
- With a sink the job reads the sink every run, before the last checkpoint of the database, so an
  overlapping run of another worker is no break: a sink that cannot be read fails the run before
  it signs, and a sink checkpoint newer than the database, an invalid unexported checkpoint or
  other content in the sink under its seq are breaks (DECISIONS M1-50/16, M1-50/18).
- When the audit entry of a verification cannot be written (a deleted newest row breaks the audit
  hook of M1-24), the result is still returned and the failure is logged.

## Port (root package `internal/audit`)

- Inbound: `Log` (query, verification), `RowsVerifier` (verification of a supplied chain, M7-03).
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: kernel `auditing.Recorder` (`app.Recorder`).
- Phase hook: `audit` (phase 3.3, `app.(*Chain).Hook`).

## Dependencies

May use the ports of these modules: `instance`.

Reason: counter `audit_seq`; the signing keys of purpose `audit_checkpoint` (`instance.Keys`, M1-50).

Vendor (go-arch-lint `adapter_vendors`): `awssdk` for `adapters/s3sink` (M7-22); depguard narrows it to that one adapter.

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
