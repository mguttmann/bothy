# M1-50: Audit verification, checkpoints and queries

Status: ready | Depends on: M1-35 | Etappe: M1

## Goal

After this ticket the audit log is provable: a worker job recomputes the hash chain, signs a
checkpoint and stores it outside the database; `audit.verify` recomputes the chain and compares it
with the checkpoints from that outside sink, so a changed row and a rewritten chain are both
detected (SEC-T16). Owners read, filter, export and verify the log over the API, always with
step-up, and every read is itself audited. Without a configured sink the instance reports a
permanent warning instead of pretending to be protected.

## Read first

- `docs/PLAN.md`, section 7: row M1-50, "Voraussetzungen aus dem Kern-Vertrag" (row "Zweck
  `audit_checkpoint` und Prüfpunkte"), "Exit-Kriterien" item 5 (SEC-T16), "Red Team" item 7; section
  3 "Arbeitsweise je Ticket". `docs/briefs/README.md`.
- `docs/adr/0021-audit-log-hash-verkettet-und-getrennt-von-telemetrie.md` (whole).
- `docs/spec/19-sicherheit.md`: "12. Lieferkette, Sicherungen und Audit-Integrität" (SEC-AUD-1 to
  SEC-AUD-3), "14. Tests" (SEC-T10, SEC-T16), "16. Konfigurationswerte" (`audit.checkpoint_interval`,
  `audit.checkpoint_sink`).
- `docs/spec/03-datenmodell.md`: tables `audit_events`, `audit_checkpoints`, `server_signing_keys`,
  `instance`, `sequence_counters`; "6.3 Sperrreihenfolge"; R-DM-28.
- `docs/spec/05-aktionen.md`: "4.2 Instanz und Verwaltung" rows `audit.read`, `audit.verify`;
  "2.4" (columns Owner, Step-up, Audit).
- `docs/spec/20-datenschutz.md`: "4.5 Audit-Log und IP-Adressen", table of controls ("jeder
  Lesezugriff steht selbst im Audit-Log").
- `docs/spec/02-architektur.md`: "8.5 Was nie in einer Transaktion passiert", "12.1" to "12.4"
  (River, queue `maintenance`, job `audit.chain_verify`), "13. Konfigurationsladen", "14.1
  Fehlerarten", "14.2 Fristen" (`http.transfer_timeout`).
- `docs/spec/06-api.md`: "6. Paginierung", "7. Idempotenz" (`verifyAuditChain` is a reading POST
  without `Idempotency-Key`), "16. Endpunkte" group "Audit".
- `docs/api/openapi.yaml`: operations `listAuditEvents`, `exportAuditEvents`, `verifyAuditChain`;
  schemas `AuditEvent`, `AuditEventPage`, `AuditVerifyRequest`, `AuditVerifyResult`,
  `AuditDecision`, `Cursor`.
- `docs/observability/signals.md` "4.6" (`bothy.audit.verify.failures`,
  `bothy.audit.verify.last_success.timestamp`), `docs/observability/alerting.md`
  (`BothyAuditChainBroken`), `docs/observability/audit-vs-telemetry.md`.
- `docs/security/threat-model.md` row T-DB-01; `docs/spec/01-produkt.md` wireframe A6.
- Code already merged: `services/server/internal/audit` (M1-24: canonical form after RFC 8785,
  chain, recorder, migration with `REVOKE`), `internal/instance` (M1-32: `Keys` with a signer per
  purpose including `audit_checkpoint`, JWKS), `internal/platform/jobs` (M1-14),
  `internal/platform/pg/pgtest` (M1-12, roles `bothy_owner`, `bothy_app`, `bothy_maint`,
  `bothy_monitor`), the step-up of M1-35.
- `docs/quality/gates.md`, `CONTRIBUTING.md`, `docs/quality/red-team-gate.md`.

## Fixed requirements that apply

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.

Note on rule 3: In this repository `docs/api/openapi.yaml` is normative and the server interface is
generated from it (ADR 0013); the route test keeps both identical.

6. Test first: every behaviour gets a test that fails before the code exists (red), then passes
   (green). Keep the red output for the report.
7. No en dash or em dash characters (U+2013, U+2014) in any file or commit message.
8. White label: no company, person, host or domain names except Alex, Sam, Robin and
   `example.com`; no paths of a real machine in code, tests or docs.
9. Limits and durations only through named keys from `docs/operations/configuration.md`; no bare
   numbers in production code or documentation.
10. Specification first: API and chapter changes below land before the code that uses them.
11. Product requirements implemented here: SEC-AUD-2 (signed checkpoint every
    `audit.checkpoint_interval`, stored at `audit.checkpoint_sink` outside the database, permanent
    warning without a sink), SEC-AUD-3 (`audit.verify` compares with the checkpoints from the sink,
    never from the database), SEC-T16 in full, 05 `audit.read` and `audit.verify` (owner, step-up,
    audit), 20 "every read of the audit log is audited", ADR 0021 "Confirmation" items 1 to 3.

## Scope

The specification is in place (planning, spec first): `AuditVerifyResult` with `failure_reason`,
`checkpoints_compared`, `last_checkpoint_seq`, `warnings` and the description of
`exportAuditEvents` in `docs/api/openapi.yaml`; 19 SEC-AUD-2, SEC-AUD-7 with the table "Format der
Prüfpunkte" and the value format of `audit.checkpoint_sink` in 19 16; 03 `audit_checkpoints`; 02
12.3 row `audit.chain_verify` (verify, then sign). Change them only where the implementation proves
them wrong; run `make api-lint`, `make api-checks` and regenerate the server code (M1-17
generation target) so that `make codegen-check` shows no diff.

Code (paths under `services/server/`):

- `internal/audit` (port): extend `Log` with `List(ctx, AuditQuery) (AuditPage, error)`,
  `Export(ctx, from, to time.Time, w AuditEventWriter) error`,
  `Verify(ctx, VerifyRequest) (VerifyResult, error)`.
- `internal/audit/domain`: `verify.go` (pure: walk rows in `seq` order, recompute each hash with
  the M1-24 canonical form, check links, gaps and checkpoint matches; returns the first failure),
  `checkpoint.go` (payload type, canonical bytes, `previous_checkpoint_hash`).
- `internal/audit/app`: `query.go`, `export.go`, `verify.go`, `checkpoint_job.go`; outgoing port
  `CheckpointSink` with `Put(ctx, SignedCheckpoint) error` (write once),
  `List(ctx) ([]SignedCheckpoint, error)` and `Configured() bool`.
- `internal/audit/adapters/filesink`: filesystem implementation of `CheckpointSink`.
- `internal/audit/adapters/postgres`: queries for the filtered list (keyset), the export batches,
  the chain walk, `audit_checkpoints` insert and `exported_at` update.
- `internal/audit/adapters/httpapi`: handlers `listAuditEvents`, `exportAuditEvents` (route class
  of downloads, deadline `http.transfer_timeout`), `verifyAuditChain`.
- Job registration of `audit.chain_verify` through `internal/platform/jobs` (queue `maintenance`,
  periodic every `audit.checkpoint_interval`, role `worker` only).
- `internal/instance` (port `Keys`): if the base lacks it, add
  `Verify(ctx, purpose, compactJWS []byte) (payload []byte, kid string, err error)` that accepts
  keys of the purpose in every state including `retired`; the compact JWS is built and parsed with
  `internal/kernel/jose` (ADR 0067).
- Metrics `bothy.audit.verify.failures` (counter) and `bothy.audit.verify.last_success.timestamp`
  (gauge) exactly as in `docs/observability/signals.md` "4.6", no attributes.
- Configuration: register `audit.checkpoint_interval` (duration) and `audit.checkpoint_sink`
  (string, empty allowed) in the typed `audit` config and the config schema; `bothy config check`
  rejects a sink value with an unsupported scheme.
- `internal/wire`, `services/server/api/operations.yaml` (enable the three operations).
- Docs: `internal/audit/README.md`, `services/server/CHANGELOG.md` under Unreleased, rulings in
  `docs/DECISIONS.md`.

## Out of scope

- Hash chain, recorder, deny aggregation, `ip_hmac` (M1-24, merged).
- Signing keys, rotation and JWKS themselves (M1-32, merged).
- Retention trim with anchor entry (`audit.retention_trim`, M7); verify starts at the smallest
  retained `seq` until anchors exist.
- An object storage sink with retention lock (PLAN M7-22); the console warning banner and CSV
  export (M6).
- Canary test over all outputs (M1-69).

## Boundaries

- Work in a dedicated git worktree on branch `feat/M1-50-audit-verify`.
- Conventional Commits with scope (`feat(audit): ...`, `docs(security): ...`) and DCO sign-off.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except the CHANGELOG and docs entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d` (also for the
  file sink in tests); remove them at the end.
- Never read or print secrets; private keys stay in the secret provider. No network calls except
  package downloads for pinned tools.

## Design notes

Chain rules (from 03 and ADR 0021, already implemented by M1-24; reuse, do not copy):
`hash = SHA-256(prev_hash || canonical_json(row without hash))`, first row `prev_hash` is 32 zero
bytes, `seq` from `audit_seq` without gaps. Hashes appear in the API as lowercase hex.
`listAuditEvents` and `exportAuditEvents` deliver each row in exactly the canonical form of 19
SEC-AUD-4 plus `hash` (the API representation `AuditEvent`), so an exported stream verifies
offline with nothing but RFC 8785 and SHA-256; a test recomputes the chain from an export.

Checkpoints:

- Payload object, canonical bytes after RFC 8785:
  `{"instance_id", "seq", "hash", "occurred_at", "previous_checkpoint_hash"}` where `seq` and
  `hash` are those of the audit row covered, `occurred_at` is that row's `occurred_at` formatted
  exactly as the M1-24 canonical form formats timestamps, `hash` and `previous_checkpoint_hash` are
  lowercase hex, and `previous_checkpoint_hash` is the SHA-256 of the previous checkpoint's
  canonical payload bytes (absent for the first checkpoint).
- Signature: compact JWS with header `{"alg": <alg of the key>, "kid": <kid>, "typ":
  "audit-checkpoint+json"}`, signed by `instance.Keys` with purpose `audit_checkpoint`. The column
  `audit_checkpoints.signature` stores the complete compact JWS as UTF-8 bytes (self-contained:
  header with `kid`, payload, signature). Verification accepts only the algorithms of 03
  `server_signing_keys` and rejects `none`.
- Sink value format: empty (no sink) or `file://<absolute path>`. Other schemes fail
  `bothy config check` with a message naming the supported scheme. File layout:
  `<path>/<instance_id>/<seq zero-padded to 20 digits>.jws`, content the compact JWS. Directory mode
  `0700`, file mode `0600`, created with `O_EXCL` through a temporary file in the same directory
  plus rename, then `fsync` of file and directory. An existing file with different content is an
  error, never overwritten.

Job `audit.chain_verify` (worker, queue `maintenance`, every `audit.checkpoint_interval`):

1. If no sink is configured, log a structured warning `audit_checkpoint_sink_not_configured`
   (every run, and once at start of every process with role `worker` or `all`).
2. Read the last checkpoint from the database. Recompute the chain from its `seq` (or the first
   row) to the current head. On any failure: increment `bothy.audit.verify.failures`, write an
   audit entry with action `audit.verify`, decision `error`, no actor, details
   `{"first_broken_seq", "failure_reason"}`, log an error, do not sign, end the job successfully
   (River must not retry a detected break; the alert `BothyAuditChainBroken` takes over).
3. If the head moved, sign and insert the new checkpoint in one transaction (no file IO inside).
4. After the commit export every row with `exported_at IS NULL` to the sink in `seq` order and set
   `exported_at`. A sink error fails the job (River retries); nothing is signed twice.
5. On success set `bothy.audit.verify.last_success.timestamp`.

`verifyAuditChain` (`audit.verify`, owner, step-up, audit, class `rl.admin`):

- Body `AuditVerifyRequest`: `from_seq` default smallest retained `seq`, `to_seq` default head and
  capped at head; `from_seq > to_seq` gives `400 validation_failed`. A range starting after the
  first row trusts the stored hash of row `from_seq - 1` as starting point (document this).
- Reads checkpoints only from the sink (`CheckpointSink.List`), never from `audit_checkpoints`.
  Each sink checkpoint is checked: signature valid for its `kid`, `instance_id` equal to this
  instance, `previous_checkpoint_hash` chain consistent; else `checkpoint_invalid`.
- Walk rows in batches of `api.page_size_max` (named value; no new key). First failure wins:
  `hash_mismatch` (stored hash differs from recomputed), `link_mismatch` (`prev_hash` differs from
  the previous stored hash), `seq_gap`, `checkpoint_mismatch` (recomputed hash at a checkpoint
  `seq` differs from the sink; `first_broken_seq` is that checkpoint `seq`).
- Result: `ok`, `checked_from_seq`, `checked_to_seq`, `first_broken_seq` and `failure_reason` only
  when not ok, `checkpoints_compared`, `last_checkpoint_seq` when any, `warnings` (contains
  `checkpoint_sink_not_configured` when no sink is configured; then no checkpoint is compared).
- A sink that cannot be read gives `503 unavailable` (`dependency_unavailable`), not `ok = false`.
- Audit entry details: `{"from_seq", "to_seq", "ok"}`.

`listAuditEvents` and `exportAuditEvents` (`audit.read`, owner, step-up, audit, `rl.admin`):

- List: newest first by `seq`, keyset cursor bound to the filters (`actor_id`, `action`,
  `decision`, `resource_kind`, `resource_id`, `from` inclusive, `to` exclusive on `occurred_at`);
  a cursor used with other filters gives `400 cursor_invalid`; `limit` per `api.page_size_default`
  and `api.page_size_max`; no totals. Use the indexes of 03 `audit_events`.
- Export: `from` and `to` required (`400 validation_failed` otherwise); `application/x-ndjson`, one
  `AuditEvent` per line in ascending `seq` within the range, batches of `api.page_size_max`,
  flushed per batch, no buffering of the whole range, no `null`.
- Each list or export call writes its own audit entry (allow) with details naming the filters (IDs
  and codes only). Denials are audited by the recorder as usual.
- Responses never contain anything but the stored columns; `details` is passed through as stored.

SEC-AUD-1 holds already; the tests below re-check it as part of SEC-T16. To simulate an attacker
with write access to the database, tests connect as the superuser of the throwaway container,
disable the protection trigger of `audit_events` for the change, and enable it again.

Rulings to record in `docs/DECISIONS.md` (link 02 12.3 and 19 SEC-AUD-7): a detected break ends
the job without retry; file mode and atomic write of the file sink.

## Tests (write them first)

All integration tests use `pgtest` (PostgreSQL 18 throwaway container on `127.0.0.1` and a random
port), a sink in a `mktemp -d` directory and a controllable clock.

- `TestSECT16_AppRoleCannotUpdateOrDelete`: `UPDATE` and `DELETE` on `audit_events` as `bothy_app`
  fail with a permission error.
- `TestSECT16_ChangedRowIsDetected`: change `details` of row k as attacker; `verifyAuditChain`
  gives `ok = false`, `first_broken_seq = k`, `failure_reason = hash_mismatch`.
- `TestSECT16_RewrittenChainIsDetectedByCheckpoint`: after a checkpoint at seq c >= k, change row
  k and recompute every later hash consistently, also overwrite `audit_checkpoints` in the
  database; verify gives `failure_reason = checkpoint_mismatch` at c.
- `TestVerifyIgnoresDatabaseCheckpoints`: with the sink intact, deleting or altering rows of
  `audit_checkpoints` changes nothing in the result; with an empty sink a rewritten chain is not
  reported as protected (`checkpoints_compared = 0`).
- `TestCheckpointFileTamperIsInvalid`: one changed byte in the payload of a sink file gives
  `checkpoint_invalid`; a file with another `instance_id` too.
- `TestCheckpointJobSignsOnlyVerifiedSegments`: break the chain, run the job: no new checkpoint,
  `bothy.audit.verify.failures` + 1, audit entry with decision `error`.
- `TestCheckpointJobWritesOnceAndLinks`: two runs with new audit rows between them give two files,
  the second links the first by `previous_checkpoint_hash`, `exported_at` is set, a third run without
  new rows writes nothing, the gauge holds the last success time.
- `TestSinkWriteOnce`: `Put` of a different content for an existing `seq` fails; same content is
  accepted as idempotent.
- `TestMissingSinkWarns`: without a sink the verify result has `warnings` with
  `checkpoint_sink_not_configured` and every job run logs the warning.
- `TestSinkConfigRejectsUnknownScheme`: `s3://bucket/prefix` fails `bothy config check`.
- `TestAuditOperationsOwnerAndStepUp`: for all three operations: non-owner human `403`, agent token
  `403`, owner without fresh step-up `401 step_up_required`, owner with step-up `200`.
- `TestAuditReadIsItselfAudited`: one list call adds exactly one `audit.read` allow entry.
- `TestListAuditEventsFiltersAndCursor`: each filter narrows the result; changing a filter with a
  cursor gives `400 cursor_invalid`; pages are stable while new rows arrive.
- `TestExportStreamsNdjsonInSeqOrder`: missing `from` or `to` gives `400`; every line parses as
  `AuditEvent`, `seq` ascends, each `prev_hash` equals the previous line's `hash`, no `null`.
- `TestVerifyRange`: `from_seq > to_seq` gives `400`; `to_seq` above head is capped.
- Domain unit tests for the chain walk with table cases for every failure reason.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/audit/... ./internal/instance/...
make codegen-check
make api-lint
make check
```

## Acceptance criteria

- [ ] SEC-T16 is complete: change through the database admin and rewrite of the whole chain are
      detected through the sink checkpoints; `UPDATE` with the application role is refused.
- [ ] Verification reads checkpoints only from `audit.checkpoint_sink`.
- [ ] Without a sink there is a warning in every verify result and every job run.
- [ ] List, export and verify only for owners with step-up; every call is audited.
- [ ] Job `audit.chain_verify` signs only verified segments, exports after commit and emits both
      metrics from the catalog.
- [ ] The implementation matches `openapi.yaml`, 02, 03 and 19 (changed only where proven wrong);
      `make api-lint`, `make api-checks`, `make codegen-check` and `make config-doc-check` are green.
- [ ] The three operations are enabled and registered with their actions.
- [ ] `make check` is green.

## Definition of done

Code and tests as above; `internal/audit/README.md` updated (ports, job, sink, operations); doc
comment on every public symbol; an ADR only if you decide something beyond ADR 0021 and the rulings
listed; `services/server/CHANGELOG.md` entry under Unreleased; rulings in `docs/DECISIONS.md`;
`make check` green; an independent review and the red team (`docs/quality/red-team-gate.md`) over
the diff with the verdict recorded; merge locally into the base branch only if both are green, then
`git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits,
files with line counts, test results with counts (including the red runs), what is not proven,
follow-ups that could not be done in scope with reason. Then write `{{DONE_PATH}}` with
`exit=0 sha=<sha>` or `exit=1 <reason>`.
