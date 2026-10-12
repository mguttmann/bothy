# M3-04: Scan pass with clamd

Status: ready | Depends on: M3-03 | Milestone: M3

## Goal

After this ticket every completed upload runs through one worker job `files.scan` that reads the stored bytes exactly once and, from that single stream, counts the size and computes the SHA-256 against the declaration, detects the type from the bytes, scans with clamd over `INSTREAM` and, with the file system driver, writes the assembled object. The file then ends `complete` with `scan_status` `clean` (deliverable), `infected` or `error` (blocked: objects deleted at once, malware audited), `skipped` (scan switched off by the operator) or with `upload_state = 'failed'` (wrong size, wrong checksum or a denied detected type). While clamd is unreachable the file stays `pending`, is never deliverable, and the job keeps retrying without an attempt limit (fail closed, ADR 0033). clamd is reachable only through a Unix socket that the Compose stack shares over a volume instead of `tcp://clamav:3310`, configured with the mandatory options of 12 5.4, and the server refuses to start when `file.max_upload_bytes` is larger than `file.scan.clamd_stream_max_bytes` while the scan is required. This is the guarantee of 12 "Goals and limits" item 1 (no unchecked byte reaches a person or an agent); downloads (M3-05), images (M3-06) and the release of held agent invocations (M3-07) build on the states and the in-transaction event this ticket provides.

## Read first

- `docs/spec/12-files.md`: 1 "Goals and limits" item 1; 2 "States of a file" (table, definitions "servable" and "final", the rule that every change writes `file.updated`); 3.2 (file system parts under `uploads/<file_id>/<n>`); 3.4 "Completion and verification pass" (the single read pass; the spec still calls the job `file_verify`); 3.5 (sequence); 3.6 "Error cases" (last row); 4.1 "Port" (`ObjectStore`), 4.2 "File system driver (default)" (temporary file, `fsync`, `rename`), 4.4 "Key schema" (objects are never overwritten); 5 "Malware scan" with 5.1 "Flow", 5.2 "Result mapping", 5.3 "When ClamAV is missing or not ready", 5.4 "Required settings for clamd"; 6.1 "Type"; 10 "Attaching and agent invocation" (`files.FileFinalized`); 12 "Retention, deletion, deduplication" (row "Failed or blocked"); 14 "Observability"; 15 "Tests" (rows EICAR, clamd stopped, Archive above `MaxRecursion`, Startup check, Total checksum wrong); 16 R-FI-04, R-FI-05, R-FI-06; 18 "Configuration values of this chapter" (`file.scan.*`, `file.sniff_bytes`, `file.denied_mime_types`); 19 Q7 to Q10, Q14.
- `docs/adr/0033-malware-scan-of-uploads-as-a-separate-service.md` (Decision Outcome, Consequences, Confirmation: the five checks this ticket proves), `docs/adr/0010-object-storage-behind-an-s3-interface-local-driver-as-default.md`, ADR 0008 and ADR 0050 (module register, vendor groups), ADR 0047 (only for the boundary to M3-06), ADR 0052 and ADR 0053 (envelope, no JSON `null`).
- `docs/spec/19-security.md`: 2 "Transport" SEC-TLS-5 (virus scanner only over a Unix socket); 7 "Upload hardening" SEC-UP-2, SEC-UP-5, SEC-UP-6, SEC-UP-7; 9 "Logs without secrets and without content"; 12 SEC-AUD-5 (`audit_details.v1`, code format); 14 "Tests" SEC-T08 and SEC-T10.
- `docs/spec/03-data-model.md` (core contract, read only): 4.7 `files` (`upload_state`, `scan_status`, `scan_engine`, `scanned_at`, `mime_type`, the partial index on `scan_status = 'pending'`), `file_upload_sessions` (`declared_size_bytes`, `declared_sha256`, `failure_reason` "code without contents", `storage_upload_id`), `file_upload_parts`; 4.10 `event_log` (`actor_kind` NULL for a job without principal); 4.12 `audit_events`; 6.3 "Lock order".
- `docs/spec/05-actions.md` (core contract, read only): row `file.upload` (Audit "yes on malware detection"); 6 "Durable event types and receive action" row `file.updated` (resource `file`, receive action `file.read`).
- `docs/spec/04-sync-protocol.md` (core contract, read only): 2.2 "Envelope" (`actor` absent for jobs without principal and without system action, `resource.version` absent where the object has no version); 15.3 "Invocation and run start" step 1.
- `docs/spec/07-permissions.md`: 4.1 "Decisions and reasons" row `malware_detected`.
- `docs/spec/02-architecture.md`: 3.3 "Commands of the binary" (row `bothy files rescan`, see Out of scope); 4.3 "Ports, events and sources per module" (row `files`: emits `FileFinalized`); 5 "Layers within a module" (rules 2, 4 and 5: metrics port and `adapters/otelmetrics`); 8.3 "In-Tx events"; 8.5 "What never happens in a transaction"; 12.1 to 12.4 "Background jobs (River)" (job catalogue row `files.scan`, queue `files`, rules 1, 2, 5, 6); 13 "Configuration loading" item 4 (cross-checks) and item 5 (a key enters the schema with its first user); 14.4 "Failure of a dependency" (row clamd).
- `docs/spec/06-api.md` 10 "Files" item 2; `docs/api/openapi.yaml`: operation `completeUpload` (its description), `getFile`, schemas `File`, `FilePayload`, `FileUpdatedEvent`, webhook `file.updated` (`onFileUpdatedEvent`), problem codes `file_not_ready` and `file_blocked` (used by M3-05).
- `docs/spec/16-operations.md`: 2 "Building blocks of an instance" (`worker`, `clamav`), 3 rule 2 (memory of the Docker VM), 12 "Status page" (`health_source` `files.scan` = `PING` to clamd).
- `docs/observability/signals.md`: 2 "Cardinality"; 4.4 rows `bothy.file.scan`, `bothy.file.scan.duration`, `bothy.file.scan.queue`; 4.7 "Histogram boundaries" (long durations); 5.2 "Spans" row `file.scan`; 5.3 `bothy.file.id`, `bothy.outcome`. `docs/observability/alerting.md` 3.2 `BothyMalwareScanFailing` and its runbook anchor; `infra/observability/prometheus/rules/bothy-alerts.yaml` (the rule reads `bothy_file_scan_total`).
- `docs/operations/configuration.md`: section 4 rows `file.scan.mode`, `file.scan.clamd_address`, `file.scan.chunk_bytes`, `file.scan.timeout`, `file.scan.max_concurrent`, `file.scan.retry_backoff_cap`, `file.scan.health_interval`, `file.scan.clamd_stream_max_bytes`, `file.sniff_bytes`, `file.denied_mime_types`, `file.max_upload_bytes`, `jobs.queue.files.max_workers`; section 9 rows `BOTHY_CLAMD_ADDRESS`, `BOTHY_CLAMAV_VERSION`, `BOTHY_CLAMAV_MEMORY_LIMIT`.
- `infra/compose/compose.yaml` (`x-server-env`, services `worker` and `clamav`, volumes), `infra/compose/.env.example`, `infra/compose/README.md` ("Security rules in this file"); `docs/architecture/seq-7-file-upload.mmd`.
- `docs/security/threat-model.md` 5.8 "Files, object storage, virus scanner" and T-UP-01; `docs/RISKS.md` RK-43 and RK-52.
- `docs/DECISIONS.md`: D-FI-04, D-FI-05, D-FI-11 (12 Files), D-K-G5, D-K-G6 (6.3), D-K-X3 (names `file.scan.*`), D-W-100, section 10 (format of `D-M3-04-<n>`).
- `docs/PLAN.md`: 3 "Way of working per ticket"; 5 "Test strategy" (row "Test doubles": clamd container); 9 "M3 Platform services" (Goal, Scope, Out of scope, Exit criteria 1 and 2, Test strategy, Red team, Risks) and the rows M3-01, M3-03 (before), M3-05, M3-06, M3-07, M3-08, M3-15 (after); 13 row M7-09.
- Briefs: `docs/briefs/M3-01-object-storage-fs.md` and `docs/briefs/M3-03-upload-sessions.md` (written in the same batch; the merged code of both wins over any name in this brief); M1 briefs for the interfaces used: M1-07 (merged: `internal/platform/config`, `internal/wire/configkeys_*.go`, `configchecks.go`, `configchapters.go`), M1-10 (`internal/platform/otel`), M1-11 (`lifecycle.Component`), M1-12 (`pgtest`, testcontainers pattern), M1-13 (`txn.UnitOfWork`, `txn.Publish`, golden file of registrations), M1-14 (`internal/platform/jobs`: `Args`, `Register`, `HandlerOptions`, `Inserter.InsertTx`, `InsertOptions.ScheduledAt`, the argument type check), M1-16 (`eventlog.Appender`), M1-24 (`auditing.Recorder.Record`, `auditing.Code`, `auditing.Int`, `ReasonMalwareDetected`), M1-68 (instrument registry `internal/platform/otel/metrics`, see Design note 12).
- Implied dependencies not in the header: M3-01 (object store port and file system driver) and M1-55 through M3-03, and with them M1-07, M1-10 to M1-14, M1-16, M1-18, M1-20, M1-24 and M1-26.
- Code: `services/server/internal/files` (port, `domain`, `app`, `adapters/fs`, `adapters/postgres`, `adapters/httpapi` from M3-01 and M3-03), `internal/platform/jobs`, `internal/platform/otel`, `internal/kernel/txn`, `internal/kernel/eventlog`, `internal/kernel/auditing`, `internal/wire`, `cmd/bothy`, `architecture/modules.yaml`, `tools/archgen`.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

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

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Product requirements implemented here:

11. One read pass: the job reads the uploaded bytes exactly once and distributes the stream to SHA-256 and byte counter, type detection over the first `file.sniff_bytes`, clamd `INSTREAM` and, with the file system driver, the assembly into the target object (temporary file, `fsync`, `rename` in the driver); afterwards the parts are deleted (12 3.4, R-FI-04, D-FI-04).
12. Size or SHA-256 different from the declaration gives `upload_state = 'failed'` with reason `size_mismatch` or `checksum_mismatch` in the session, the objects are deleted and `file.updated` is written (12 3.4, 3.6).
13. The detected type is stored as `mime_type`; `declared_mime_type` stays diagnosis only; a detected type listed in `file.denied_mime_types` gives `upload_state = 'failed'` (12 6.1, SEC-UP-2). The server never unpacks archives (SEC-UP-7); detection reads only the buffered head.
14. Result mapping exactly as 12 5.2: `stream: OK` gives `clean`; `<signature> FOUND` with a signature starting with `Heuristics.Limits.Exceeded` gives `error` with reason `scan_limits_exceeded`; any other `FOUND` gives `infected`; `INSTREAM size limit exceeded` or another `ERROR` gives `error`; a timeout or connection error leaves `pending` and retries.
15. `infected`: objects deleted at once, the row stays, and an audit entry with `action = 'file.upload'`, `decision = 'deny'`, `deny_reason = 'malware_detected'` and in `details` the signature, the SHA-256 and the size, never content (12 5.2, 05 `file.upload`, 07 4.1, SEC-AUD-5, D-K-G5). Blocked and failed files lose their objects at once (12 12).
16. Fail closed with `file.scan.mode = required` (default): clamd unreachable or `PING` not answered with `PONG` leaves the file `pending`; the job retries with backoff up to `file.scan.retry_backoff_cap` without an upper limit of attempts while the file is not deleted; nothing unchecked is delivered. `file.scan.mode = off` gives `scan_status = 'skipped'` (12 5.3, R-FI-05, SEC-UP-6, 02 14.4).
17. The server does not start when `file.scan.mode = required` and `file.max_upload_bytes` is larger than `file.scan.clamd_stream_max_bytes` (12 5.4, 12 15 "Startup check", ADR 0033 Confirmation).
18. clamd only over a Unix socket (19 SEC-TLS-5): `file.scan.clamd_address` accepts only `unix://` with an absolute path; the Compose stack shares the socket through a volume instead of `tcp://clamav:3310` and sets the mandatory clamd options of 12 5.4 (`AlertExceedsMax yes`, `StreamMaxLength` and `MaxFileSize` at least `file.max_upload_bytes`, defaults for `MaxScanSize`, `MaxRecursion`, `MaxFiles`, `AlertEncrypted no`).
19. Protocol (12 5.1): command `zINSTREAM\0`; data in chunks of at most `file.scan.chunk_bytes`, each preceded by its length as 4 bytes in network byte order, ended by a chunk of length 0; the reply is read until clamd closes the connection; `file.scan.timeout` per file; at most `file.scan.max_concurrent` scans per worker process; an own small client, no `go-clamd`, no cgo scanner library (ADR 0033).
20. Every change of `upload_state` or `scan_status` writes the durable event `file.updated` (resource `file`, receive action `file.read`, no content) in the same transaction (12 2, 05 6); when a file becomes final ("final", 12 2) the same transaction publishes the in-transaction event `files.FileFinalized` (02 4.3, 12 10, D-K-G6).
21. The worker pings clamd every `file.scan.health_interval`; the result is kept for the status component with `health_source` `files.scan` (12 5.3, 16 12).
22. Signals only from the catalogue: `bothy.file.scan` (outcome `clean`, `infected`, `error`, `skipped`), `bothy.file.scan.duration`, `bothy.file.scan.queue`, span `file.scan`; no file names, no contents and no signature names in metrics, spans or logs; signature names only in the audit log (12 14, signals.md 2).
23. Job `files.scan` in queue `files`, inserted with `InsertTx` in the transaction that completes the upload, arguments only IDs (02 12.1 to 12.4); no clamd or object store call inside a database transaction (02 8.5).

## Scope

All paths under `services/server/` unless noted. Where M3-01 or M3-03 already created a file named below, extend it.

Create:

- `internal/files/domain/scan.go` (standard library and `kernel-core` only):
    - `type VerdictKind` with `VerdictClean`, `VerdictInfected`, `VerdictError`; `type Verdict struct { Kind VerdictKind; Signature string; Reason string }`.
    - `func ParseClamdReply(reply []byte) (Verdict, error)`: the mapping of requirement 14; any other form returns `ErrClamdProtocol`.
    - `func DecidePass(in PassFacts) PassOutcome`: the outcome table of Design note 5 (pure).
    - `func NextScanDelay(round int64, first, ceiling time.Duration) time.Duration`: Design note 7.
    - `func AuditSignature(sig string) string`: Design note 15.
    - `func Deliverable(...) bool` and `func Final(...) bool` over `upload_state`, `scan_status` and `deleted_at` exactly as 12 2 defines them (keep M3-03's if it already has them).
- `internal/files/app/scan.go`: the use case `ScanPass.Run(ctx, fileID ids.ID, round int64) (PassResult, error)`; outbound ports `Scanner { Scan(ctx context.Context, r io.Reader) (domain.Verdict, error); Ping(ctx context.Context) error }` (12 5: port `Scanner`), `TypeDetector { Detect(head []byte) string }`, `ScanQueue { EnqueueScan(ctx context.Context, fileID ids.ID, round int64, at time.Time) error }`; the repository methods of Design notes 2 and 6.
- `internal/files/app/scanhealth.go`: the health monitor of Design note 11.
- `internal/files/app/metrics.go`: outbound metrics port with a no-op implementation for unit tests (02 5 rule 5), or extend M3-03's.
- `internal/files/adapters/clamd/`: `client.go` (the `Scanner` implementation), `config.go` (`Config` with `Address`, `ChunkBytes`, `Timeout`, `MaxConcurrent`), `doc.go`, tests; `clamdtest/` (throwaway clamd container and a fake clamd server, Design note 13).
- `internal/files/adapters/sniff/`: the `TypeDetector` with `github.com/gabriel-vasile/mimetype` (12 6.1, Q14).
- `internal/files/adapters/jobs/scan.go`: args `Scan struct { FileID ids.ID; Round int64 }` with `Kind() = "files.scan"`, the handler, the `ScanQueue` implementation over `jobs.Inserter`.
- `internal/files/adapters/otelmetrics/metrics.go`: the three instruments (Design note 12).
- `internal/files/adapters/postgres/queries/scan.sql`: the queries of Design notes 2, 6, 7 and 11.
- `infra/compose/clamav/clamd.conf` (repository root path): the clamd configuration of Design note 10.
- `tools/docs/tests/test_compose_clamd.py` (repository root path): the static Compose checks of the test list.

Change:

- `internal/files` (port): in-transaction event `FileFinalized struct { FileID ids.ID }`; `type ScannerHealth interface { ScannerState() ScannerState }` with `type ScannerState struct { Status string; CheckedAt time.Time }`, status one of `ok`, `unreachable`, `off`, `unknown`.
- The completion use case of M3-03 (operation `completeUpload`): call `ScanQueue.EnqueueScan(ctx, fileID, 0, time.Time{})` inside its unit of work. Keep it if M3-03 already inserts a job of this kind.
- `internal/platform/jobs`: if `HandlerOptions` has no per-kind timeout, add `Timeout time.Duration` (zero: River default; negative: no River timeout) mapped to River's per-worker timeout, with a test and a line in its README (Design note 7).
- `internal/wire`: `configkeys_files.go` (or the area file M3-01/M3-03 created) with the keys of Design note 9 and the chapter constant `chapter12 = "12-files.md"` in `configchapters.go` if missing; the cross-checks `scan_stream_limit` and `clamd_address_unix` in `configChecks`; the expected keys in `TestConfigSchemaBuilds`; job registration of `files.scan` (queue `files`); construction of scanner, detector, metrics and the health monitor as a `lifecycle.Component` in roles `worker` and `all`; regenerate `testdata/txn_registrations.golden` only if a registration changes (publishing `FileFinalized` without a subscriber changes none).
- `architecture/modules.yaml`: vendor group `mimetype: [github.com/gabriel-vasile/mimetype, github.com/gabriel-vasile/mimetype/**]`; `files.adapter_vendors` gains `mimetype` and the vendor groups `clamdtest` and `otelmetrics` need (the testcontainers group of M1-12, the OpenTelemetry group the metrics pattern uses); regenerate `.go-arch-lint.yml` with `cd services/server && go generate ./tools/archgen`.
- `go.mod`, `go.sum`: `github.com/gabriel-vasile/mimetype` at its newest release, pinned; MIT is on the allowed list (`make licenses`).
- `internal/files/README.md`: status, the job, the ports `Scanner`, `TypeDetector`, `ScanQueue`, `ScannerHealth`, the event `FileFinalized`, the adapters `clamd`, `sniff`, `jobs`, `otelmetrics`, the retry rule, the residual crash window of Design note 6 and the test requirements (Docker runtime, pinned clamd image).
- `services/server/CHANGELOG.md` under "Unreleased".
- `infra/compose/compose.yaml`, `infra/compose/.env.example`, `infra/compose/README.md` (Design note 10).
- `docs/spec/12-files.md` (not core contract): 3.4 and 3.5 name the job `files.scan` instead of `file_verify`; 5.1 item 1 says "only a Unix socket (19 SEC-TLS-5)" and drops TCP; 18 changes the default of `file.scan.clamd_address` to `unix:///run/clamav/clamd.sock` and its meaning.
- `docs/spec/02-architecture.md` 13 item 4: add the two cross-checks to "Cross conditions today".
- `docs/spec/06-api.md` 10 item 2 and the description of `completeUpload` in `docs/api/openapi.yaml`: completion queues the pass `files.scan`, which checks size and SHA-256, detects the type and scans; nothing else in the API changes.
- `docs/operations/configuration.md`: section 4 regenerated (`make config-reference`); section 9 row `BOTHY_CLAMD_ADDRESS` with the new template value.
- `docs/architecture/seq-7-file-upload.mmd` and its `.svg`: completion only sets the session `completed` and inserts `files.scan`; the job checks size and SHA-256, detects the type, scans and then sets `upload_state` and `scan_status` (12 3.4); render with `uv run --project tools --locked --quiet python tools/docs/render_diagrams.py --write`.
- `docs/DECISIONS.md` section 10: rows `D-M3-04-<n>` for the rulings of Design note 14.

This ticket enables no operation in `api/operations.yaml`, adds no migration, no action and no Makefile target.

## Out of scope

- Upload sessions, parts, completion and the early checks against declared type and extension (M3-03); the object store port and the file system driver (M3-01); the S3 driver and the presigned mode (M3-02, M3-03).
- Download, thumbnails over HTTP, transcripts, fetch tickets and the answers `409 file_not_ready` with `Retry-After` and `409 file_blocked` (M3-05); this ticket provides the states and `domain.Deliverable`.
- Metadata removal and thumbnails, and inserting `files.thumbnail` after `clean` (M3-06, see Design note 13).
- Attachments, the subscriber of `files.FileFinalized`, `ReleaseInvocation` and `runs.invocation_hold_timeout` (M3-07).
- `files.cleanup_pending`, `files.purge_objects`, `file.failed_row_ttl` and the catch-up of missed object deletions (M3-08).
- The transcript condition of `FileFinalized` for voice messages in mode `service` (M3-09); text extraction after `clean` (M3-15).
- The status component "Virus scan", `degraded` after `file.scan.pending_alert_after`, its alarm and the key `file.scan.pending_alert_after` in the schema (M7-09); "Virus scan off" in console and status page (M6, M7-21).
- Choosing the profile `scan` and `file.scan.mode` during installation (M7-29); Compose runs with the profile `scan` on real hosts (M7-14).
- `IngestFile` and the queue `files_bulk` of 12 11 (import, M8-03).
- `bothy files rescan --skipped` (02 3.3, 12 5.3): no ticket of the plan owns it yet; reported as a plan gap, not built here.
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M3-04-clamd-scan`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(files): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same applies to M3-01 and M3-03: follow their merged names (object store port, repository, completion use case, event builder for `file.updated`) and list each difference.
- clamd runs in tests only as a throwaway container of the image pinned in `infra/compose/.env.example` (`BOTHY_CLAMAV_VERSION`), pulled once like any pinned tool; it never downloads signatures (freshclam off) and binds only `127.0.0.1` with a random port. Stop only containers you started, by id.
- Never commit the EICAR test string or any real malware sample: tests assemble the EICAR string at run time from two parts. No real third-party service in tests.
- The production code accepts only `unix://` clamd addresses; the loopback TCP seam of Design note 13 exists only in test files.

## Design notes

1. Start of the pass. The completion use case of M3-03 calls `ScanQueue.EnqueueScan(ctx, fileID, 0, time.Time{})` in its unit of work, so the job exists exactly when the session is `completed` (02 8.5, 12 3.4 step 3). An `Idempotency-Key` replay of `completeUpload` inserts nothing (M1-20 returns the stored answer). The job `files.scan` runs in queue `files` (02 12.2, 12.3); its args are `FileID` and `Round` (`int64`, the retry round), which the type check of M1-14 accepts.
2. Awaiting the scan. A file awaits the pass when `upload_state = 'pending'`, `scan_status = 'pending'`, `deleted_at IS NULL` and its session `state = 'completed'`. The handler first reads this without a transaction; any other state ends the job with `nil` and no effect (02 12.4 rule 1). Order in the handler:
   a. read the target (file row, session row: declared size and SHA-256, `part_count`, `storage_upload_id`);
   b. with `file.scan.mode = required`: `Scanner.Ping`; an error goes to the retry of note 7 without reading any byte;
   c. run the pass of note 4 under a context with the deadline `file.scan.timeout`;
   d. decide the outcome (note 5) and write it (note 6), or reschedule (note 7).
   The handler in `adapters/jobs` starts the span `file.scan` (kind CLIENT) with `bothy.file.id` around steps b to d and sets `bothy.outcome` from the result.
3. Never in a transaction (02 8.5): `Ping`, the pass, `Head`, `Get`, `Put` and `Delete` run outside every unit of work; the unit of work of note 6 or note 7 only reads and writes PostgreSQL and inserts jobs.
4. The single pass (12 3.4, R-FI-04).
   a. Source. `ObjectStore.Head(object_key)` first. If the object exists, the pass reads it with `Get(object_key, nil)`: always with S3 (M3-03 completed the multipart upload), and with the file system after an earlier attempt already assembled it (objects are never overwritten, 12 4.4, D-FI-11). Otherwise, with the file system layout, the pass reads the parts `uploads/<file_id>/<n>` for `n` from 1 to `part_count` in this order, opening each part only when the previous one is exhausted, and writes the same stream with `ObjectStore.Put(object_key, reader, declared size)` through a pipe; the driver writes a temporary file, `fsync` and `rename` (12 4.2). If the object is missing and the session has a `storage_upload_id` (S3), that is an internal error: Error log, the file stays `pending`, retry (note 7). If merged M3-03 already assembles file system parts at completion, the pass simply reads `object_key`; name this second read of the bytes in the report as a deviation from D-FI-04.
   b. The source is wrapped in a reader limited to the declared size plus one byte, so an oversized object is detected without reading unbounded data.
   c. A tee distributes every byte once to: SHA-256 and the byte counter; the head buffer of at most `file.sniff_bytes` bytes for the type; the `Put` pipe (file system assembly only). The scanner consumes the tee: `Scanner.Scan(ctx, tee)` with `file.scan.mode = required`, otherwise the pass copies the tee to `io.Discard`.
   d. If the scanner returns before the end of the stream (a verdict such as the stream limit, or a transport error), the pass drains the rest of the tee, so size, SHA-256 and the assembled object are complete and the size or checksum decision of note 5 never depends on clamd.
   e. The `Put` pipe is closed with an error whenever the pass fails or the byte count differs from the declared size, so no half object stays; an object that does remain belongs to a failed or blocked file and is deleted with it (note 6).
   f. Type detection runs once on the head buffer after the stream ends. Store the media type without parameters, in lower case. A detected type is denied when it equals an entry of `file.denied_mime_types` case-insensitively without parameters; reuse M3-03's matcher for the early check if it exists, so both checks use one rule (12 6.1).
5. Outcome (first matching row wins; Ruling for docs/DECISIONS.md for the order, the codes marked "ruling" and the audit in the first three rows):

    | Condition | `upload_state` | `scan_status` | session `failure_reason` | `mime_type` | objects | audit | metric outcome |
   |---|---|---|---|---|---|---|---|
   | byte count differs from `declared_size_bytes` | `failed` | `pending` | `size_mismatch` | unchanged | delete | malware entry if the scan reported a `FOUND` that maps to `infected` | none |
   | SHA-256 differs from `declared_sha256` | `failed` | `pending` | `checksum_mismatch` | unchanged | delete | as above | none |
   | detected type denied | `failed` | `pending` | `file_type_denied` (ruling, the reason of the early `415`) | detected | delete | as above | none |
   | `file.scan.mode = off` | `complete` | `skipped` | none | detected | keep | none | `skipped` |
   | `stream: OK` | `complete` | `clean` | none | detected | keep | none | `clean` |
   | `FOUND` with `Heuristics.Limits.Exceeded` | `complete` | `error` | `scan_limits_exceeded` | detected | delete | none | `error` |
   | other `FOUND` | `complete` | `infected` | `malware_detected` (ruling) | detected | delete | malware entry | `infected` |
   | `INSTREAM size limit exceeded` | `complete` | `error` | `scan_stream_limit_exceeded` (ruling) | detected | delete | none | `error` |
   | any other `ERROR` reply | `complete` | `error` | `scan_engine_error` (ruling) | detected | delete | none | `error` |

    The three `failed` rows decide the outcome even when the scan failed transiently, because size, SHA-256 and type are known from the complete stream (note 4d) and the bytes are deleted anyway; otherwise a transient scanner error or a reply that `ParseClamdReply` rejects leaves the file `pending` (note 7). `scanned_at = now()` and `scan_engine = 'clamd'` (ruling) are set whenever clamd returned a verdict; both stay NULL for `skipped` and for failed files. The reason codes are machine codes without content (03 `failure_reason`); the free text of a clamd `ERROR` line is never stored or logged. The malware entry for a failed file closes the gap that a client could hide a detection behind a wrong checksum.

6. Result unit of work (one `txn.UnitOfWork.Do`):
   a. Lock the `files` row `FOR UPDATE`, then the `file_upload_sessions` row (03 6.3: domain rows; keep the order M3-03 uses for the same pair if it differs, and name it in the README). Check note 2 again; if the file is no longer awaiting (final, or `deleted_at` set) change nothing.
   b. Update `files` (`upload_state`, `scan_status`, `mime_type`, `scanned_at`, `scan_engine`) and the session (`failure_reason`).
   c. Append `file.updated` with `eventlog.Appender`: no actor (job without principal and without system action, 04 2.2, 03 `event_log.actor_kind`); resource `{kind: file, id}` without `version` (`files` has no version column); scope `{space_id, channel_id}` of `scope_channel_id` for `purpose = 'attachment'` and empty for the other purposes; receive action `file.read`; payload `FilePayload {file: File}` with the row after the change, without JSON `null` (ADR 0053). Reuse M3-03's builder for this payload and scope if it exists.
   d. Malware entry with `auditing.Recorder.Record` (the transaction commits, so not `RecordDenial`): `Action "file.upload"`, `Decision deny`, `Reason ReasonMalwareDetected`, `ResourceKind "file"`, `ResourceID` the file, `Attrs` exactly `signature` (`Code(AuditSignature(sig))`), `sha256` (`Code` of the lowercase hex of the computed digest) and `size_bytes` (`Int` of the counted bytes). The actor is NULL because the job has none (03 `audit_events.actor_principal_id`). If the merged code bound of `auditing.Code` is shorter than a hex SHA-256, raise it in the same change with its test and record the ruling.
   e. `txn.Publish(ctx, files.FileFinalized{FileID})` whenever the new state is final (12 2: deliverable, `failed`, `infected` or `error`).
   f. After the commit, and only if this run made the transition: delete `object_key` (for `failed`, `infected`, `error`) and, with the file system layout, every part `uploads/<file_id>/<n>` (all outcomes); a missing object is no error (12 4.1). A failed delete is logged and left to the sweep of M3-08; the crash window between commit and delete is a residual named in the module README. If step a found the file deleted, the run deletes the objects it wrote itself.
   g. Record the metrics of note 12 after the commit.
7. Fail closed and retry (Ruling for docs/DECISIONS.md). River's attempt limit must never end the retries (12 5.3: no upper limit while the file is not deleted), so a transient failure (ping failed, connection error, timeout, protocol error, object store error) does not return an error to River: in a short unit of work the handler checks note 2 again and, if the file still awaits, inserts the next `files.scan` with `Round + 1` and `ScheduledAt = now + NextScanDelay(Round, file.scan.health_interval, file.scan.retry_backoff_cap)`, then returns `nil`. `NextScanDelay` doubles from `file.scan.health_interval` per round, never exceeds `file.scan.retry_backoff_cap` and never overflows for large rounds. No state change, no event, no audit; a Warn log with `bothy.file.id` and a reason class (`unreachable`, `timeout`, `protocol`, `storage`). Only when this unit of work itself fails (database) does the handler return the error to River. A cancelled job context (shutdown) returns `ctx.Err()` (M1-14). The pass enforces `file.scan.timeout` itself; register `files.scan` with no River job timeout (`HandlerOptions.Timeout` negative), because River's default job timeout from the `platform/jobs` README can be shorter than `file.scan.timeout` and would cut the follow-up unit of work; the database work stays bounded by `database.statement_timeout` and `database.lock_timeout` (02 12.4 rules 5 and 6).
8. clamd client (`adapters/clamd`, standard library only, ADR 0033).
   a. `New(cfg Config) (*Client, error)` accepts only `unix://` followed by an absolute path; anything else is an error naming the key, never the value.
   b. `Ping`: connect, send `zPING\0`, read the NUL-terminated reply; anything but `PONG` is an error.
   c. `Scan`: acquire the process-wide semaphore of size `file.scan.max_concurrent` (respecting `ctx`), connect, send `zINSTREAM\0`, then read from the reader into a buffer of `file.scan.chunk_bytes` and send each non-empty read as one chunk with its 4-byte big-endian length (a read of zero bytes sends nothing), end with a zero-length chunk, read the reply until the connection closes, strip the trailing NUL, and return `domain.ParseClamdReply`. Connection deadlines follow `ctx`.
   d. A write error does not end the conversation at once: clamd closes the stream on its size limit and replies first, so the client tries to read a reply and returns the verdict if there is one, otherwise a transport error.
   e. The reply is bounded by `file.scan.chunk_bytes` (one chunk is the largest unit the client exchanges with clamd); a longer reply is a protocol error.
   f. No signature, file name or reply text in logs or span attributes of the adapter.
9. Configuration and start. Add to the schema, with the defaults and kinds of the chapter tables (02 13 item 5): `file.scan.mode` (enum `required`, `off`), `file.scan.clamd_address` (string), `file.scan.chunk_bytes`, `file.scan.timeout`, `file.scan.max_concurrent`, `file.scan.retry_backoff_cap`, `file.scan.health_interval`, `file.scan.clamd_stream_max_bytes`, `file.sniff_bytes`, and, if M3-03 has not added them, `file.max_upload_bytes` (chapter 03) and `file.denied_mime_types`. Two cross-checks in `configChecks`, whose messages name keys and never values: `scan_stream_limit` (keys `file.scan.mode`, `file.max_upload_bytes`, `file.scan.clamd_stream_max_bytes`: fails when the mode is `required` and the upload limit is larger than the mirror) and `clamd_address_unix` (key `file.scan.clamd_address`: fails unless `unix://` plus an absolute path). `bothy config check` and `bothy serve` therefore fail before start with exit code `2` (02 3.3). With `file.scan.mode = off` the worker logs one Warn at start that the virus scan is off.
10. Compose (Ruling for docs/DECISIONS.md: socket path and permissions).
    - A named volume `clamav-socket`, mounted at `/run/clamav` in `clamav` and read-only in `worker` (connecting to a socket does not write to the file system). `api` and `migrate` do not mount it; they never scan.
    - `x-server-env`: `BOTHY_FILE__SCAN__CLAMD_ADDRESS: ${BOTHY_CLAMD_ADDRESS:-unix:///run/clamav/clamd.sock}`; `.env.example`: `BOTHY_CLAMD_ADDRESS=unix:///run/clamav/clamd.sock` with a comment that the socket lives in the volume `clamav-socket`.
    - `infra/compose/clamav/clamd.conf`, mounted read-only where the pinned image reads its clamd configuration: start from the image's own configuration of the pinned version (read it from the image, do not guess) and set `LocalSocket /run/clamav/clamd.sock`, a socket mode the worker's user can connect to (the volume is shared only with `worker`), no `TCPSocket`, `AlertExceedsMax yes`, `StreamMaxLength` equal to the default of `file.scan.clamd_stream_max_bytes` and `MaxFileSize` not below the default of `file.max_upload_bytes` (a comment names both keys), `MaxScanSize`, `MaxRecursion`, `MaxFiles` and `AlertEncrypted` left at their defaults; verify every option name against Q10.
    - `clamav` publishes no `ports` and no `expose`; it keeps the network `default` because freshclam needs egress for signatures (install-compose "Network"). Its health check must work without a TCP listener reachable from other containers: prefer a Compose `healthcheck` that pings over the socket; if the pinned image cannot do that, set `TCPAddr` to loopback inside the container for the image's own check and record which variant you took. `worker` keeps `depends_on: clamav: service_healthy, required: false`.
    - `infra/compose/README.md`: a rule under "Security rules in this file" that clamd is reached only over the socket volume (SEC-TLS-5) and the profile table entry for `scan` names the volume.
11. Health monitor (12 5.3). A loop in the worker process (roles `worker`, `all`) wired as a `lifecycle.Component`: every `file.scan.health_interval` it calls `Ping` with that interval as deadline (mode `required` only), stores `ScannerState{Status, CheckedAt}` (`ok`, `unreachable`; `off` with mode `off`; `unknown` before the first ping), logs each change of status once (Warn when clamd becomes unreachable, Info when it is back), and records `bothy.file.scan.queue` as the number of files awaiting the pass (note 2, a count over the partial index on `scan_status = 'pending'`). It implements the port `files.ScannerHealth`, which M7-09 reads for `health_source` `files.scan` (Ruling for docs/DECISIONS.md: per process, kept in memory). It never changes readiness: a missing clamd keeps files pending but the worker stays ready for its other queues.
12. Observability (12 14, signals.md 4.4 and 5.2). `bothy.file.scan` counts one per final scan outcome (`clean`, `infected`, `error`, `skipped`; failed files and retries are not counted, so `BothyMalwareScanFailing` keeps its meaning); `bothy.file.scan.duration` measures the whole pass per outcome (ruling: the pass and the scan are one stream, so one histogram covers both, with the long-duration bounds of signals.md 4.7); `bothy.file.scan.queue` from note 11. Recording goes through the metrics port of `app` and `adapters/otelmetrics` (02 5 rule 5). Declare the three instruments in the instrument registry `internal/platform/otel/metrics` of M1-68 with exactly the catalogue names, units, buckets and closed attribute sets. Logs use constant messages and only `bothy.file.id`, `bothy.outcome` and the reason class; never file names, signatures or reply text.
13. Interfaces for later tickets and tests.
    - `domain.Deliverable` and `domain.Final` are the only definitions of 12 2; M3-05 maps them to `409 file_not_ready` and `409 file_blocked`.
    - Keep the outcome write of note 6 in one function with a clear place where M3-06 inserts `files.thumbnail` and M3-15 inserts `files.extract_text` for a deliverable file in the same unit of work (12 3.4 last paragraph). This ticket inserts neither.
    - `clamdtest` (non-test package used only by tests): starts the pinned image through testcontainers-go as M1-12 does for `pgtest` (Ryuk off, label `org.bothy.test=clamdtest`, loopback binding, `AutoRemove`), with the image tag read from `BOTHY_CLAMAV_VERSION` in `infra/compose/.env.example` and pinned by digest with a renovate comment like `pgtest.Image`. Ruling for docs/DECISIONS.md: the test container runs `clamd` in the foreground directly (not the image's init script, which may download signatures) with a test `clamd.conf` from `mktemp -d` (`AlertExceedsMax yes`, small explicit `StreamMaxLength`, `MaxRecursion` and `MaxScanSize`, `LocalSocket` and a TCP listener for the test seam) and a database directory holding only one hash signature for the EICAR string, computed at run time (MD5, size and the name `Bothy.Test.EICAR`). Reason: the full signature database needs minutes and gigabytes of memory to load (RK-52) and must not be downloaded in tests. ClamAV may append `.UNOFFICIAL` to local signature names; assert with a prefix.
    - Test seam: VM-based Docker runtimes on macOS do not pass Unix sockets of bind mounts through to the host, so the real-clamd tests dial the container's loopback TCP port through an unexported dialer option set from `export_test.go`; the dialer resolves the mapped port at every dial, because a restarted container gets a new port. The production constructor has no such option.
    - A fake clamd server in `clamdtest` listens on a Unix socket in a `mktemp -d` directory and scripts replies, delays and early closes.
14. Rulings for `docs/DECISIONS.md` section 10 (one row each, `D-M3-04-<n>`, with reason): the retry by re-insertion without attempt limit and its delay rule (note 7); no River job timeout for `files.scan` (note 7); the outcome precedence, the reason codes `file_type_denied`, `malware_detected`, `scan_stream_limit_exceeded`, `scan_engine_error` and the malware entry on failed files (note 5); a rejected clamd reply stays `pending` (note 5); `scan_engine = 'clamd'`, NULL for `skipped` (note 5); media type without parameters and the denial match (note 4f); the source rule with `Head` (note 4a); the audit signature normalisation (note 15); the socket path, volume and socket permissions (note 10); the health state per process (note 11); one histogram for the pass (note 12); the test clamd with a local signature and the TCP test seam (note 13). The job name `files.scan`, the unix-only address and the mandatory clamd options come from 02 12.3, 19 SEC-TLS-5 and 12 5.4 and need no ruling.
15. Audit signature (Ruling for docs/DECISIONS.md). Codes in `audit_details.v1` allow only lower-case letters, digits and `_.:-` (SEC-AUD-5), while ClamAV names contain upper-case letters. `AuditSignature` lower-cases the name, replaces every other character by `_` and cuts it to the code bound of `auditing.Code`; the original name is never stored elsewhere.

## Tests (write them first)

Unit:

- `internal/files/domain`: `TestParseClamdReply` (table: `stream: OK`; `stream: <name> FOUND`; `stream: Heuristics.Limits.Exceeded.MaxRecursion FOUND` gives `error` with `scan_limits_exceeded`; `INSTREAM size limit exceeded. ERROR` gives `error` with `scan_stream_limit_exceeded`; another `... ERROR` gives `scan_engine_error`; a trailing NUL is accepted; empty, `stream: ok`, `stream:  FOUND`, `PONG` and binary noise give `ErrClamdProtocol`); `FuzzParseClamdReply` (never panics; returns `VerdictClean` only for exactly `stream: OK` with an optional trailing NUL); `TestDecidePass` (one row per line of the table in Design note 5, plus a checksum mismatch together with a `FOUND`, a size mismatch and a denied type each together with a transient scan failure); `TestNextScanDelay` (starts at `file.scan.health_interval`, doubles, stops at `file.scan.retry_backoff_cap`, no overflow for the largest `int64` round); `TestDeliverableAndFinal` (every row of 12 2 and a deleted clean file, which is not deliverable); `TestAuditSignature` (upper case, spaces and slashes, an over-long name).
- `internal/files/adapters/clamd` against the fake server: `TestNewRejectsNonUnixAddress` (`tcp://clamav:3310`, an `http` URL, a relative `unix` path; the error names `file.scan.clamd_address` and not the value); `TestInstreamFraming` (command `zINSTREAM\0`, every chunk at most `file.scan.chunk_bytes` with a correct big-endian length, no empty chunk before the terminator, exactly one zero-length terminator); `TestPing` (`PONG` ok, anything else an error); `TestScanReadsReplyAfterWriteError` (the fake replies with the stream limit and closes mid-stream: the verdict is `error`, not a transport error); `TestScanTimeout` (a stalled fake: transport error by the context deadline); `TestScanMaxConcurrent` (with `file.scan.max_concurrent` set low in the test config, one more parallel scan waits; the fake never sees more connections at once); `TestReplyBound`; `TestClientLogsNoSignature` (captured logs and spans contain neither the signature nor the reply text).
- `internal/files/adapters/sniff`: `TestDetect` (PNG bytes declared as `text/plain` give `image/png`; an OOXML document whose markers lie inside the head is detected as such; parameters are stripped; an empty head gives `application/octet-stream`).
- `internal/files/app` with fakes (counting object store, scripted scanner, fake clock): `TestScanPassReadsEachByteOnce` (file system layout: every part byte read once and `Put` receives identical bytes; existing object: read once, no `Put`); `TestScanPassModeOff` (`skipped`, scanner never called); `TestScanPassTransientReschedules` (ping failure: no byte read, no state change, no event, no audit, one follow-up job with `Round + 1` at the expected time); `TestScanPassNotAwaiting` (final or deleted file: no effect, no delete); `TestScanPassDrainsAfterEarlyVerdict`.

Integration (PostgreSQL 18 via `pgtest`, the file system driver of M3-01 in a `mktemp -d` root, real modules through `internal/wire`, uploads through the real HTTP chain with `createUpload`, `uploadPart` and `completeUpload` as Alex in a channel of a test space, people from the identity test helpers; tests marked "real clamd" use `clamdtest`):

- `TestScanEICARInfected` (real clamd; 12 15 row EICAR, ADR 0033): `scan_status = 'infected'`, `upload_state = 'complete'`, `domain.Deliverable` false and `getFile` shows `infected`; `object_key` and all parts are gone (`Head` reports missing), the row stays; exactly one audit row with `action = 'file.upload'`, `decision = 'deny'`, `deny_reason = 'malware_detected'`, NULL actor, resource the file, `details` with exactly `signature` (prefix of the normalised test name), `sha256` and `size_bytes`; exactly one `file.updated`; `FileFinalized` published once (test subscriber registered only in the test).
- `TestScanCleanFile` (real clamd): `clean`, `complete`, detected `mime_type`, `scanned_at` set, `scan_engine = 'clamd'`, the object exists and its SHA-256 equals the declaration, parts deleted; the `file.updated` row validates against `FileUpdatedEvent` and contains no JSON `null`; `FileFinalized` once.
- `TestScanClamdStoppedStaysPending` (real clamd; 12 15 row "clamd stopped", ADR 0033): with the container stopped the job leaves the file `pending`, not deliverable, writes no event, and inserts the next `files.scan` delayed by `file.scan.health_interval`; repeated failures keep inserting rounds with growing delay up to `file.scan.retry_backoff_cap` and never stop; after the container starts again the due job sets `clean`. (`409 file_not_ready` on download is asserted by M3-05.)
- `TestScanArchiveOverMaxRecursion` (real clamd; 12 15, ADR 0033): ZIP archives nested one level deeper than the test configuration's `MaxRecursion` give `error` with `failure_reason = 'scan_limits_exceeded'`, never `clean`; objects deleted; no malware audit entry.
- `TestScanArchiveOverMaxScanSize` (real clamd; plan Test strategy "archive limits"): a small archive that expands beyond the test `MaxScanSize` gives `error` with `scan_limits_exceeded`.
- `TestScanStreamLimitExceeded` (real clamd): with the test clamd's `StreamMaxLength` below the file size while the server's mirror is consistent, the reply `INSTREAM size limit exceeded` gives `error` with `scan_stream_limit_exceeded`.
- `TestScanChecksumMismatch` (12 15 row "Total checksum wrong"): one stored part is changed on disk with the same length after its digest was accepted: `upload_state = 'failed'`, `failure_reason = 'checksum_mismatch'`, `scan_status = 'pending'`, objects deleted, one `file.updated`, `FileFinalized` once. With an EICAR payload the malware audit entry is written as well.
- `TestScanSizeMismatch` with a fake object store in the assembled layout whose object is one byte longer: `failed` with `size_mismatch`; the pass reads at most the declared size plus one byte.
- `TestScanDetectedTypeDenied` and `TestScanDeclaredTypeIgnored` (SEC-UP-2, part of SEC-T08): a PNG declared as `text/plain` is stored as `image/png`; with `image/png` in `file.denied_mime_types` the same upload ends `failed` with `file_type_denied`.
- `TestCompleteUploadInsertsScanJob`: `completeUpload` inserts exactly one `files.scan` job with `Round` 0 in its transaction; a replay with the same `Idempotency-Key` inserts none; a rolled-back completion inserts none.
- `TestScanThroughRiver`: with the jobs runtime in role `all`, the job is picked from queue `files` and the file ends `clean` (real clamd).
- `TestScanConcurrentRuns`: two passes for the same file in parallel produce one transition, one event, one audit entry at most and no deletion by the losing run.
- `TestScanFileDeletedDuringPass`: `deleted_at` set while the pass runs: no state change, no event, and the assembled object is removed by the job.
- `TestScanNoNamesOrSignaturesOutsideAudit` (SEC-T10 for this path): the upload's file name is a canary and the file is EICAR; captured logs, spans and metric attributes of the pass contain neither the canary nor the signature; the signature appears only in the audit row.
- `TestScanHealthMonitor` (real clamd and fake clock): state `unknown`, then `ok`; `unreachable` after the container stops, `ok` after it starts again; each change logged once; `bothy.file.scan.queue` equals the number of files awaiting the pass; with mode `off` the state is `off` and no ping happens.
- `TestScanMetrics` (manual metric reader): `bothy.file.scan` per outcome, `bothy.file.scan.duration` with the catalogue buckets, no attribute outside `bothy.outcome`; failed files and retries are not counted.

Configuration and start (`internal/wire`, `cmd/bothy`):

- `TestConfigScanStreamLimit` (12 15 row "Startup check", ADR 0033): mode `required` with `file.max_upload_bytes` above `file.scan.clamd_stream_max_bytes` makes `bothy config check` exit `2` with a message that names both keys and contains no value; equal values pass; mode `off` passes.
- `TestServeRefusesInconsistentScanLimits`: `bothy serve --role=worker` with that configuration ends before any listener opens, with exit code `2`.
- `TestConfigClamdAddressUnixOnly` (SEC-TLS-5): `tcp://clamav:3310`, an `http` URL and `unix://relative/path` are rejected; `unix:///run/clamav/clamd.sock` passes.
- `TestConfigSchemaBuilds` lists the new keys with the chapter defaults.

Static and platform:

- `tools/docs/tests/test_compose_clamd.py` (pytest, PyYAML, no network): the default of `BOTHY_FILE__SCAN__CLAMD_ADDRESS` in `x-server-env`, `BOTHY_CLAMD_ADDRESS` in `.env.example` and the default of `file.scan.clamd_address` in the table of 12 18 are the same `unix://` address; neither `compose.yaml` nor `.env.example` contains a `tcp://` clamd address; `clamav` has no `ports` and no `expose`; the volume `clamav-socket` is mounted in `clamav` and read-only in `worker` at the directory of the socket, and in no other service; `clamd.conf` is mounted read-only into `clamav`, sets `AlertExceedsMax yes`, its `LocalSocket` lies in that directory, it has no `TCPSocket` unless `TCPAddr` is a loopback address, `StreamMaxLength` equals the default of `file.scan.clamd_stream_max_bytes` and `MaxFileSize` is not below the default of `file.max_upload_bytes` (both defaults read from the chapter tables, sizes parsed with clamd's suffixes).
- `docker compose --env-file infra/compose/.env.example -f infra/compose/compose.yaml config --quiet` is valid with the template profiles.
- Architecture (ADR 0033 Confirmation): `make arch` is green with `mimetype` only in `internal/files/adapters`, the S3 rule of M3-02 untouched, and no scanner library in the register (any unregistered third-party import fails go-arch-lint).

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/files/... ./internal/platform/jobs/... ./internal/wire/... ./cmd/bothy/...`
- a short fuzz run of `FuzzParseClamdReply` (`cd services/server && go test -run '^$' -fuzz FuzzParseClamdReply ./internal/files/domain/` stopped after a short `-fuzztime`); its seed corpus runs in every `go test`
- `uv run --project tools --locked pytest tools/docs/tests/test_compose_clamd.py`
- `docker compose --env-file infra/compose/.env.example -f infra/compose/compose.yaml config --quiet`
- `make arch`, `make codegen-check`, `make register-check`, `make config-reference` (no diff afterwards), `make config-doc-check`, `make api-lint`, `make api-checks`, `make licenses`
- `make diagrams`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] EICAR against a real clamd container becomes `infected`, is never deliverable, its objects are deleted, the row stays, and one audit entry `file.upload`, `deny`, `malware_detected` carries signature, SHA-256 and size and nothing else.
- [ ] With clamd stopped the file stays `pending` and undeliverable, the job retries with backoff up to `file.scan.retry_backoff_cap` without an attempt limit, and after clamd starts the file becomes `clean`.
- [ ] An archive over `MaxRecursion` (and over `MaxScanSize`) becomes `error` with `scan_limits_exceeded`, never `clean`.
- [ ] `bothy config check` and `bothy serve` refuse to start when `file.scan.mode = required` and `file.max_upload_bytes` is larger than `file.scan.clamd_stream_max_bytes`.
- [ ] The pass reads every byte once and decides size, SHA-256, detected type and scan from that one stream; size or checksum mismatches and denied detected types end `failed` with their reason and without objects.
- [ ] clamd is reached only over a Unix socket: the configuration rejects every other address, and `infra/compose/compose.yaml` shares the socket through the volume `clamav-socket` instead of `tcp://clamav:3310`, with the mandatory clamd options of 12 5.4 in `infra/compose/clamav/clamd.conf`.
- [ ] Every state change writes one `file.updated` that validates against `FileUpdatedEvent`; a final state publishes `files.FileFinalized` once.
- [ ] Metrics, span and logs follow signals.md; no file name, content or signature name outside the audit log.
- [ ] Chapter 12 (job name, unix-only address, default), 02 13 item 4, 06 10 item 2, the `completeUpload` description, `configuration.md`, the Compose README, `seq-7-file-upload`, the `files` README, the CHANGELOG and the rows `D-M3-04-<n>` are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `files` updated (ports, job, adapters, retry rule, residual crash window of note 6f, test requirements of note 13); README of `platform/jobs` if `HandlerOptions` changed; `infra/compose/README.md` updated.
- Rulings of note 14 recorded in `docs/DECISIONS.md` (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
