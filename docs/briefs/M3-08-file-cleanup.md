# M3-08: Cleanup and deletion of files

Status: ready | Depends on: M3-07 | Milestone: M3

## Goal

After this ticket no byte of a deleted, abandoned or blocked file stays in the object store longer than the rules of 12 "Retention, deletion, deduplication" allow, and nothing of a deleted file stays readable in the database. Deleting a message removes the objects of its attachments: the deletion transaction of M3-07 marks the files deleted, now also redacts the payloads of their earlier `file.updated` events and inserts the job `files.purge_objects`, which deletes every object the key schema of 12 4.4 allows for the file (originals of every revision, the four variants, the parts or the multipart upload) and then the `files` row with its session, parts and dependent rows, within `privacy.object_delete_delay` (20 4.2, 4.3, DP-T01 for files). The periodic job `files.cleanup_pending` runs every `file.gc_interval`: it ends open upload sessions after `file.upload_session_ttl` (`expired`, file `failed`, parts and multipart upload removed), deletes files with `purpose = 'attachment'` that were never attached and are older than `file.unattached_ttl` unless a draft of their own uploader still holds them (a file ID in somebody else's draft protects nothing), deletes the rows of failed and blocked attachment files that hang on no message after `file.failed_row_ttl`, catches up every purge that failed, removes temporary files a crashed process left in the file system driver's `.tmp`, and reports the deletion backlog. A person deletes an own file that is not attached with `deleteFile`. The objects of a file that becomes `failed`, `infected` or `error` are removed by the same purge job, inserted in the transaction of that transition. Every object store call runs outside a database transaction (02 8.5), and no file name appears in a log, span, metric or remaining event payload. This closes 12 12 and the "Cleanup" row of 12 15, and it is the file part of DP-T01 (the deletion journal entry of DP-T01 is M7-07).

## Read first

- `docs/PLAN.md`: 3 "Way of working per ticket"; 9 "M3 Platform services" (Goal, Scope, Out of scope, Exit criteria 1 and 2, Test strategy, Red team, Dependencies, Risks) with the rows M3-01, M3-03, M3-04 and M3-07 (what exists), M3-08 (this ticket, the contract), M3-02, M3-05, M3-06, M3-10, M3-14 and M3-15 (beside it, not in its closure), and M7-05, M7-06 and M7-07 (who builds on it); 7 rows M1-58, M1-61, M1-63, M1-68, M1-69, M1-70, M1-71.
- Dependencies: the header names M3-07. Implied through it and therefore not in the header: M3-04 (scan pass, `domain.Final`, `domain.Deliverable`, the outcome write and its post-commit deletes), M3-03 (tables `files`, `file_upload_sessions`, `file_upload_parts`, `PartStore`, `abortUpload`, `getFile`, the visibility of kind `file`), M3-01 (`ObjectStore`, key schema, file system driver), M2-11 and the M2 run chain down to M2-02, M1-71 (`test/contract` and `test/harness`), M1-70 (`test/leak`, `tools/lint/sql_visibility.py`), M1-69 (`test/canary`), M1-68 (instrument registry, `make signal-check`), M1-63 (`listEvents` with `redacted`), M1-62, M1-61 (`drafts`), M1-58 (`eventlog.Redactor`), M1-56, M1-28, M1-16, M1-14, M1-13, M1-12 and the rest of their closure. Not in the closure but used where merged (each place says how): M3-02 (S3 driver), M3-05 (download, tickets), M3-06 (`file_thumbnails`, image job), M3-10 (avatar and logo release), M3-14 (search), M3-15 (`file_text_extracts`).
- `docs/spec/12-files.md`: 1 "Goals and limits" point 5; 2 "States of a file" (table, "servable", "final", the state diagram with "deleted", the rule that every change of `upload_state` writes `file.updated`); 3.4 "Completion and verification pass" step 3; 3.6 "Error cases" (rows "Session expired", "Permissions revoked during the upload"); 3.7 "Purpose of a file"; 4.1 "Port" (no listing, idempotent `Delete`); 4.2 "File system driver (default)"; 4.3 "S3 driver (core S3 only)" (last bullet: the server aborts expired uploads itself); 4.4 "Key schema"; 10 "Attaching and agent invocation" point 1; 12 "Retention, deletion, deduplication" completely; 13 "Endpoints and actions" (row `DELETE /api/v1/files/{id}`); 14 "Observability" ("deletion backlog"); 15 "Tests" (row "Cleanup"); 18 `file.unattached_ttl`, `file.failed_row_ttl`, `file.gc_interval`, `file.upload_session_ttl`.
- `docs/spec/03-data-model.md` (core contract, read only): 1 "Principles" items 7 and 12; 3 "Retention classes"; 4.2 `principals` (`avatar_file_id`, `ON DELETE SET NULL`); 4.7 `files`, `file_thumbnails`, `message_attachments` (`ON DELETE RESTRICT`), `file_upload_sessions`, `file_upload_parts`, `file_text_extracts`; 4.10 `event_log` (`redacted_at`, index `(resource_kind, resource_id)`) and `drafts` (`user_id`, `file_ids`); 6.3 "Lock order"; 6.5 "Redaction on deletion".
- `docs/spec/04-sync-protocol.md` (core contract, read only): 2.2 "Envelope" (`actor` absent for jobs without principal, `redacted`, payload rules 4 and 5); 14 "Cross-device synchronization" (row "Drafts": `file_ids` are checked only in form and count).
- `docs/spec/05-actions.md` (core contract, read only): rule 10 "System actions"; 2.2 "Target of a resource" (rows `file`); 4.8 row `file.delete` ("delete one's own unattached file; attached ones via `message.delete`", `self or manage`, Audit `no`, `rl.write`); 6 row `file.updated`.
- `docs/spec/02-architecture.md`: 4.1 "Rules" point 4 (source ports); 4.3 "Ports, events and sources per module" (rows `files`, `sync`); 4.4 "Allowed dependencies" (rows `files`, `sync`, "Tiers"); 4.5 (row `kernel/eventlog`: `Redactor` "only `messaging` (and `files` with M3)"); 8.1 "Flow" (events are buffered until phase 3.2); 8.2; 8.5 "What never happens in a transaction"; 12.1 to 12.4 (job catalogue rows `files.cleanup_pending`, `files.purge_objects`; rules for jobs).
- `docs/spec/20-privacy.md`: 2 rows `files`, `file_upload_sessions`, `event_log`, `drafts`; 4.1; 4.2 "Deleting a message"; 4.3 "Files"; 4.6 "Backups and deletion journal"; 10 "Tests" DP-T01; 13 `privacy.object_delete_delay`.
- `docs/spec/06-api.md`: 5 "Errors"; 7 "Idempotency" (row "PUT and DELETE": a deleted resource `204`, a never visible one `404`, a resource without a mark `404` after deletion); 16 "Endpoints" (table "Files").
- `docs/spec/07-permissions.md`: 4.3 "Visibility"; 7.4 "Query pattern" rule 1; 11.2 "Required tests" AZ-P1, AZ-P4, AZ-P12. `docs/spec/19-security.md`: SEC-UP-8, SEC-LOG-4, SEC-T10. `docs/spec/14-search.md` 2 "What is searchable" (row `files`: only deliverable files of a visible, not deleted message).
- `docs/api/openapi.yaml`: operations `deleteFile` (and `getFile`, `abortUpload`, `uploadPart`, `getUpload`, `completeUpload`, `deleteMessage`, `putThreadDraft`, `putChannelDraft`, `deleteThreadDraft`, `listEvents` for the tests); schemas `File`, `FilePayload`, `FileUpdatedEvent`, `EventEnvelope` (`redacted`), `Draft`, `Problem` (`x-problem-codes`: `conflict`, `not_found`); responses `Conflict`, `NotFound`. `docs/api/asyncapi.yaml`: message `event.file.updated`.
- ADRs 0007, 0008, 0010, 0013, 0017, 0048, 0050, 0052, 0053; 0058 for the boundary (deleted objects stay in backups).
- `docs/DECISIONS.md`: D-FI-07, D-FI-10, D-FI-11, D-W-100, D-W-104, D-W-106, section 10 (format of the rows `D-M3-08-<n>`).
- `docs/operations/configuration.md`: `file.unattached_ttl`, `file.failed_row_ttl`, `file.gc_interval`, `file.upload_session_ttl`, `file.storage.driver`, `file.fs.root`, `privacy.object_delete_delay`, `database.tx_retry_max`. `docs/observability/signals.md` 4.4 and 5.3 (`bothy.file.id`, never on metrics). `docs/security/threat-model.md` T-UP-01, T-BK-02; `docs/RISKS.md` RK-33. No item of `docs/OPEN-QUESTIONS.md` concerns this ticket.
- Briefs (merged names win, D-W-100): `M3-07-attachments-invocation.md` (Design note 7 c: the one function of `files/app` that sets `deleted_at`, `DeleteForMessage`; note 9 `fileResource`; note 10 `recordFileChange` and `domain.BecameFinal`; note 16 the SQL lint categories), `M3-04-clamd-scan.md` (Design note 6: the result unit of work, the post-commit deletes and the residual it leaves to this ticket; `domain.Final`, `domain.Deliverable`), `M3-03-upload-sessions.md` (Design notes 4 and 13 to 15: `PartStore` with `Discard`, the abort, the pair lock order, the visibility of kind `file` and `file.delete`), `M3-01-object-storage-fs.md` (Design notes 3, 4, 9, 10 and 12: `ObjectStore` in `internal/files/app`, `domain.OriginalKey`, `domain.VariantKey`, `domain.UploadPartKey`, `domain.ParseOriginalKey`, idempotent `Delete` and `AbortMultipart`, `<root>/.tmp`), `M1-58-messaging-edit-delete.md` (`eventlog.Redactor`, `RedactFunc`, redaction rules), `M1-61-sync-personal-data.md` (`drafts`, the `Drafts` port, `drafts.sql`), `M1-14-background-jobs.md` (`jobs.Args`, `jobs.Register`, `HandlerOptions`, `Registry.AddPeriodic`, `PeriodicJob`, `Inserter.InsertTx`), `M1-13-unit-of-work.md` (`txn.UnitOfWork.Do`, `txn.InUnit`), `M1-16-event-log.md`, `M1-63-sync-catch-up.md` (`listEvents`, `redacted`), `M1-68-core-metrics.md` (instrument registry, observable gauges), `M1-69-canary-test.md`, `M1-70-leak-tests.md`, `M1-71-contract-tests.md`, `M1-06-module-register-check.md` (C7, C11, C12). Beside it: `M3-06-image-metadata-thumbnails.md` Design notes 10 e and 16, `M3-10-avatar-logo.md` Design notes 2 and 3 (release with `deleted_at`, staged sessions), `M3-15-file-text-extraction.md` Design note 10, `M3-14-search-coordinator.md` (predicate for files).
- Code (all under `services/server/`): `internal/files` (port, `domain`, `app` with `ObjectStore`, `PartStore`, `recordFileChange`, the `deleted_at` function, `fileResource`, `adapters/postgres`, `adapters/jobs`, `adapters/fs`, `adapters/otelmetrics`, `adapters/httpapi`), `internal/sync` (`drafts`, `adapters/postgres`, the `Redactor` implementation), `internal/messaging` (delete path), `internal/kernel/{eventlog,txn,clock,ids,authorize,errs}`, `internal/authz` (visibility of kind `file`), `internal/platform/jobs`, `internal/wire` (`configkeys_*.go`, `configchecks.go`), `api/operations.yaml`, `architecture/modules.yaml`, `tools/archgen`, `test/canary`, `test/contract`, `test/leak`; at the repository root `tools/lint/sql_visibility.yaml` (M1-70).

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

11. Deleting a message removes the objects of its attachments: in the deletion transaction the files get `deleted_at` (M3-07), the payloads of their stored `file.updated` events are redacted and `files.purge_objects` is inserted; the job removes original, variants and parts by their known keys and then the row, within `privacy.object_delete_delay` (12 12 row "Message deleted", 20 4.2 and 4.3, DP-T01).
12. Never attached: a file with `purpose = 'attachment'`, `upload_state = 'complete'`, no attachment and no reference from a draft of its uploader (`drafts.file_ids` with `drafts.user_id = files.uploaded_by`) is deleted after `file.unattached_ttl`; a file ID in the draft of another principal keeps nothing alive (12 12 row "Never attached", 04 14 row "Drafts").
13. Open sessions past `expires_at` (`file.upload_session_ttl`) become `expired`, their file `upload_state = 'failed'` with `file.updated`, and their parts and multipart upload are removed (12 12 row "Session open and expired", 12 3.6).
14. Failed or blocked files lose their objects at once (a purge job inserted in the transaction of the transition backs up the post-commit deletes of M3-03 and M3-04); the row of such a file with `purpose = 'attachment'` that hangs on no message stays for the uploader until `file.failed_row_ttl` and is then deleted (12 12 row "Failed or blocked", Design note 6 c).
15. A cleanup job runs every `file.gc_interval` and catches up missed deletions; a failed `Delete` is retried; a missing object counts as deleted (12 12, 12 4.1).
16. `deleteFile` deletes only own files that are not attached ("or with `manage`" as `authorize` decides); attached files are deleted through `message.delete` (05 4.8, 12 12 row "`file.delete`"); deleting an already deleted file answers `204`, an invisible or unknown one the same `404` (06 7, AZ-P4).
17. Keys come only from the key schema of 12 4.4 and the database; the server never lists the store (12 4.1, D-FI-11). No object store call inside a database transaction; consequences go through a job inserted with `InsertTx` (02 8.5); job arguments carry only IDs (02 12.1).
18. A deleted file is not deliverable and not readable: every read answers `404` (12 2, M3-03), search never shows it (14 1), and no stored event payload keeps its name (03 6.5, DP-T01).
19. No deduplication: deleting a file never touches the file of another person (R-DM-13, 20 4.3).
20. No file name, object key, digest or channel name in logs, spans, metrics, audit details or problem bodies (12 14, SEC-LOG-4, SEC-T10).

## Scope

All paths under `services/server/` unless they start with `docs/` or `tools/`. Names of M3-01, M3-03, M3-04 and M3-07 below are the expected ones; where the merged code names them differently, follow the code (Boundaries). Where one of those tickets already created a file named below, extend it.

Create:

- `internal/files/domain/purge.go`: `ObjectKeysOf(fileID ids.ID, objectKey string) ([]string, error)` (Design note 4 a), `PurgeFacts`, `PurgePlan` with the constants `PurgeNothing`, `PurgeObjects`, `PurgeObjectsAndRow`, and `PlanPurge(PurgeFacts) PurgePlan` (Design note 4 b).
- `internal/files/domain/cleanup.go`: `CleanupFacts`, `LastChange(CleanupFacts) time.Time`, `UnattachedExpired(f CleanupFacts, now time.Time, ttl time.Duration) bool`, `FailedRowExpired(f CleanupFacts, now time.Time, ttl time.Duration) bool`, `SessionExpired(state string, expiresAt, now time.Time) bool` (Design notes 6 and 7).
- `internal/files/domain/redact.go`: `RedactFileEvent(eventType string, schemaVersion int, payload json.RawMessage) (json.RawMessage, error)`, used as `eventlog.RedactFunc` (Design note 3).
- `internal/files/app/purge.go` (`Purge`, Design note 4), `internal/files/app/cleanup.go` (`Cleanup` with the steps of Design note 6), `internal/files/app/delete.go` (`Delete`, Design note 9); outgoing ports in `internal/files/app/ports.go`: `PurgeQueue { Enqueue(ctx context.Context, fileID ids.ID) error }` and `TempSweeper { SweepTemp(ctx context.Context, olderThan time.Time) (int, error) }`.
- `internal/files/adapters/jobs/purge.go` and `cleanup.go`: args `PurgeObjects{FileID ids.ID}` with kind `files.purge_objects` and `CleanupPending{}` with kind `files.cleanup_pending`, their handlers (queue `maintenance`), and the `PurgeQueue` implementation over `jobs.Inserter.InsertTx`.
- `internal/files/adapters/postgres/queries/cleanup.sql` with the queries of Design note 13 and the regenerated sqlc code.
- `internal/files/adapters/fs/sweep.go`: `(*Store).SweepTemp` (Design note 6 e), not part of `ObjectStore`.
- In `sync`: the query `HeldByUploaderDrafts` in `internal/sync/adapters/postgres/queries/drafts.sql` and the type that implements `files.DraftFileSource` over it (Design note 8), in the adapter package where `sync` implements the source ports and kernel ports of other modules today (follow the merged layout).

Change:

- `internal/files` root package (port): `DraftFileSource` and `UploaderFile` (Design note 1); `Files` gets `Delete(ctx context.Context, fileID ids.ID) error` (`deleteFile`).
- `internal/files/app`: the one function that sets `files.deleted_at` (M3-07 Design note 7 c; this brief calls it `markDeleted`) redacts and inserts the purge job (Design note 2); `recordFileChange` (M3-07 Design note 10) inserts the purge job when a file becomes final without being deliverable (Design note 5); the result unit of work of M3-04 treats a missing row like a deleted file (Design note 10); where M3-06 or M3-10 are merged, their code paths named in Design note 10 and in the callers paragraph of Design note 2.
- `internal/files/adapters/httpapi/files.go`: strict-server method `DeleteFile`.
- `internal/files/adapters/otelmetrics` and `internal/files/app/metrics.go`: the gauge `bothy.file.purge.backlog` (Design note 12).
- `internal/wire`: jobs `files.purge_objects` (handler) and `files.cleanup_pending` (handler and periodic job with `Interval = file.gc_interval`, `RunOnStart = true`) for the roles `worker` and `all`; the `PurgeQueue` for all roles (the role `api` inserts with its insert-only client); the `DraftFileSource` of `sync`; the `TempSweeper` (file system driver: the store; S3: a sweeper that does nothing); the handler of `deleteFile`; configuration keys (with the new key `file.gc_batch_size`) and the cross-check of Design note 11 in `configkeys_files.go`, the key file of chapter 20 and `configchecks.go`; regenerate the jobs golden file if one exists.
- `api/operations.yaml`: move `deleteFile` from `later` to `enabled` (sorted); regenerate (`make codegen-check`).
- `architecture/modules.yaml`: `files` in `sync.depends_on`; regenerate `.go-arch-lint.yml` with the generator of M1-05 (`cd services/server && go generate ./tools/archgen`, as M3-07 does); `make arch` and `make register-check`.
- `tools/lint/sql_visibility.yaml`: the exception entries of Design note 13.
- `test/canary`, `test/contract`, `test/leak`: the paths of "Suites of other tickets" below.
- `docs/spec/12-files.md`, `docs/spec/02-architecture.md`, `docs/spec/20-privacy.md`, `docs/observability/signals.md`, `docs/api/openapi.yaml` (description of `deleteFile` only), `docs/architecture/c4-3-server-components.mmd` and its `.svg` (Design note 15); `docs/operations/configuration.md` regenerated (`make config-reference`).
- READMEs of `files` and `sync`; `services/server/CHANGELOG.md` under "Unreleased"; `docs/DECISIONS.md` section 10 rows `D-M3-08-<n>` (Design note 16).

This ticket creates no migration, enables one operation (`deleteFile`), adds one configuration key (`file.gc_batch_size`), one metric (`bothy.file.purge.backlog`), one configuration cross-check (`object_delete_delay_gc`) and the two `reason` values `file_attached` and `purpose_not_deletable` of `409 conflict`, and adds no action, no event type and no top-level Makefile target.

## Out of scope

- Upload sessions, parts, `abortUpload` and their post-commit deletes (M3-03); the scan pass, `files.scan`, its post-commit deletes and `FileFinalized` (M3-04); attaching, `message_attachments`, the delete path of a message with `DeleteForMessage`, `recordFileChange` and `fileResource` (M3-07). This ticket only extends the functions Design notes 2, 5 and 10 name.
- The deletion journal entry of DP-T01, SEC-T15 and DP-T07 (M7-07); retention jobs per class (`retention.content`, `retention.transient`, so also rows of completed sessions and their parts of live files) and DP-T05 (M7-05); erasure of a person's files and the export files with `purpose = 'gdpr_export'` (M7-06).
- Search: that deleted files are never hits is M3-14's predicate (14 1, M3-14 `TestSearch_NotSearchable`); the deletion of `file_text_extracts` rows inside `markDeleted` is M3-15's (its Design note 10).
- Thumbnails and `file_thumbnails` (M3-06): the purge deletes the variant keys by the key schema whether or not the table exists. Avatar and logo release (M3-10). The S3 driver itself (M3-02); downloads and fetch tickets (M3-05).
- An event that announces a deleted file: 05 6 has none and this ticket adds none (Design note 2 c); clients learn it from `message.deleted` or from a `404`.
- An alert on the deletion backlog: `docs/observability/alerting.md` names none; this ticket adds the signal only.
- Deleting objects that a backup still holds (20 4.6, ADR 0058).
- Changes to the core contract (R-DM-45 and R-DM-46 are already in it).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M3-08-file-cleanup`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(files): ...`, `feat(sync): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11). The redacted form of `file.updated` with the redaction of resource kind `file` (Design note 3) and the removal of a deleted file's row by the purge (Design note 4 d) are core contract since D-W-108 (03 6.5 and 04 2.2 rule 5, R-DM-45; 03 1 item 7, R-DM-46); build them as written.
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M3-01, M3-03, M3-04 and M3-07 (and M3-02, M3-06, M3-10, M3-15 where they are merged): follow their names (`ObjectStore`, the key builders, `PartStore`, the abort, the result unit of work, `recordFileChange`, the `deleted_at` function, `fileResource`) and list each difference.
- The file system driver runs in tests only with `file.fs.root` in a directory from `mktemp -d`; never the default path of 12 18. The S3 strategy is tested against the in-memory fake of `ObjectStore` that M3-03 uses; no real S3 service and no SeaweedFS unless M3-02 is merged, and then only through its throwaway container helper.
- Time only through `kernel/clock`; tests move a fake clock and call the job handlers directly, they never sleep for a window.
- The SQL visibility lint of M1-70 is on the base branch: every new reading query on a content table carries the visibility predicate or an exception entry with category and reason (Design note 13); never widen an existing exception.

## Design notes

1. **Dependencies and ports** (02 4.1 point 4, 4.3, 4.4, 4.5; ADR 0008, ADR 0050; Ruling for docs/DECISIONS.md). The table `drafts` belongs to `sync` (03 4.10); `files` may neither read it (03 1 item 12) nor import `sync`, which is the higher module (02 4.4 "Tiers"). `files` therefore declares a source port in its root package and `sync` implements it, the pattern of `authz.PrincipalSource`:

    ```go
    // DraftFileSource answers which files a draft of their own uploader still holds
    // (12 section 12, row "Never attached"). Implemented by the module sync.
    type DraftFileSource interface {
        // HeldByUploader returns the IDs of the given files that appear in drafts.file_ids
        // of a draft whose user_id equals the file's uploader. A file listed only in the
        // draft of another principal is not returned. Runs in the ambient unit of work.
        HeldByUploader(ctx context.Context, files []UploaderFile) (map[ids.ID]bool, error)
    }

    // UploaderFile names one file and its uploader (files.uploaded_by).
    type UploaderFile struct{ FileID, UploaderID ids.ID }
    ```

    Add `files` to `sync.depends_on` in `architecture/modules.yaml` and to "May use" of the row `sync` in 02 4.4 (reason, in English: "implements `files.DraftFileSource`; drafts protect the files of their author, 12 12"), `DraftFileSource` as "declares" to the row `files` of 02 4.3 and `files.DraftFileSource` to "Implements sources" of the row `sync`, the edge `sync --> files` to `docs/architecture/c4-3-server-components.mmd` (render with `make diagrams`) and to the dependency line of the `sync` README; `make register-check` holds the four equal (M1-06 C7, C11, C12). The edge points down (`files` lies below `sync`), so the graph stays acyclic. `files` also uses `eventlog.Redactor`, which 02 4.5 already allows ("and `files` with M3"), and no other new port of another module.

2. **The one deletion function** (M3-07 Design note 7 c; 12 12; 20 4.2, 4.3; Ruling for docs/DECISIONS.md for b to d). This brief calls it `markDeleted(ctx context.Context, fileIDs []ids.ID) error`; keep the merged name of M3-07. It runs inside the caller's unit of work after the caller locked the `files` rows `FOR UPDATE` in ascending `id`:

    a. `UPDATE files SET deleted_at = $now WHERE id = ANY($ids) AND deleted_at IS NULL RETURNING id` with one `kernel/clock` reading; only the returned files take steps b to e, so a second call has no effect.

    b. An `open` upload session of such a file becomes `aborted`; `upload_state` stays and no `file.updated` is written. Reason: the file is gone for every reader (`getFile`, `uploadPart`, `getUpload` and `abortUpload` answer `404` for a deleted file, M3-03 Design notes 9 a and 15), and the purge discards the parts.

    c. Redaction: `eventlog.Redactor.Redact(ctx, "file", id, domain.RedactFileEvent)` per file (note 3). No new event: 05 6 has none for a deleted file, `message.deleted` announces a deletion with the message, and the caller of `deleteFile` gets its `204` (M3-07 note 7 c stays).

    d. `PurgeQueue.Enqueue(ctx, id)` per file: `jobs.Inserter.InsertTx` of `PurgeObjects{FileID}` without `ScheduledAt`, so the job runs at once and the objects go well within `privacy.object_delete_delay`; mark and job commit together, and a rollback leaves neither (02 8.5).

    e. Where M3-15 is merged, its deletion of the `file_text_extracts` row (M3-15 Design note 10) stays in this function after a.

    Callers: M3-07's `DeleteForMessage` (unchanged), `deleteFile` (note 9), the cleanup (note 6 b and c) and, where M3-10 is merged, its release of a replaced or removed avatar or logo, which today sets `deleted_at` itself (M3-10 Design note 3) and must call this function instead, so the purge runs. `TestOneDeletedAtWriter` asserts that the sqlc queries of `files` contain exactly one statement that writes `deleted_at`.

3. **Redacted `file.updated`** (03 6.5, 04 2.2 rules 4 and 5, R-DM-18; core contract R-DM-45, D-W-108). `domain.RedactFileEvent` is a pure function used as `eventlog.RedactFunc`:

    - `file.updated` with `schema_version` 1: `{"file": F}` where F keeps `id`, `uploaded_by`, `purpose`, `upload_state`, `scan_status`, `transcript_status` and `created_at` of the stored payload, sets `original_name` to `""`, `size_bytes` to `0`, `mime_type` to `application/octet-stream` and `thumbnails` to `[]`, and drops every other member (`sha256`, `scope_channel_id`, `width`, `height`, `duration_ms`, `transcript_language`, `transcript_source`). The result validates against `FileUpdatedEvent` (every required member of `File` stays) and carries no JSON `null` (ADR 0053).
    - Any other type or schema version on resource kind `file`: an internal error, so the unit of work fails; a later event type on resource `file` must define its redaction first (the rule of M1-58 note 6 for `message`).
    - Applied to its own output, the function returns the same bytes.

    The envelope keeps every field; `sync` sets `redacted_at`, and the envelope shows `"redacted": true` (M1-58 note 8, M1-63). Reason: 20 4.2 and DP-T01 require empty event payloads, every `file.updated` carries the name and digest of the file, and 03 6.5 speaks of "Deleting a message or a file" but names only `resource_kind = 'message'`. 03 6.5 and 04 2.2 rule 5 state this since D-W-108 (R-DM-45).

4. **Purge** (`files.purge_objects`; 12 12, 12 4.1, 4.4; 02 8.5, 12.1, 12.4; Ruling for docs/DECISIONS.md for a to d).

    a. `domain.ObjectKeysOf(fileID, objectKey)`: `ParseOriginalKey(objectKey)` gives the revision r and checks the file ID (a mismatch or a malformed key is an error, and nothing is deleted); the result is `OriginalKey(fileID, n)` for every n from 1 to r + 1, then `VariantKey(fileID, v)` for the four variants. r + 1 covers an original that a processing run (M3-06, `orig-2`) wrote but did not yet reference; deleting a key that never existed is no error (12 4.1). Keys come only from the key schema and the row; nothing is listed (D-FI-11).

    b. `domain.PlanPurge(PurgeFacts) PurgePlan` over facts read without a transaction (row exists, `deleted_at`, `upload_state`, `scan_status`): no row gives `PurgeNothing`; `deleted_at` set gives `PurgeObjectsAndRow`; a file that is final and not deliverable (`domain.Final` and not `domain.Deliverable`: `upload_state = 'failed'` or `scan_status IN ('infected','error')`) gives `PurgeObjects`; every other file (upload running, scan pending, deliverable) gives `PurgeNothing`, so a stray job never touches a live file.

    c. `app.Purge.Run(ctx, fileID)`: (1) read the facts, `object_key` and the session row (`part_count`, `storage_upload_id`) without a transaction; `PurgeNothing` returns `nil`. (2) Outside every unit of work: `PartStore.Discard` of M3-03 when a session row exists (file system: `Delete` of every part key; S3: `AbortMultipart` with the stored upload id), then `ObjectStore.Delete` for every key of a. The first error is returned, so River retries with its defaults (02 12.4 rules 4 and 5); a missing object counts as deleted (12 12). (3) Only for `PurgeObjectsAndRow`: one unit of work locks the row `FOR UPDATE`, checks that it still exists with `deleted_at` set and without a `message_attachments` row (such a row is an internal error with one Error log carrying `bothy.file.id`; the file row stays), and runs `DELETE FROM files WHERE id = $1`. `file_upload_sessions`, `file_upload_parts`, `file_thumbnails` and `file_text_extracts` go by their `ON DELETE CASCADE`; a remaining `principals.avatar_file_id` becomes NULL by its `ON DELETE SET NULL` (03 4.2). No event and no audit (05 4.8: `file.delete` is not audited; the deletion journal is M7-07).

    d. Row removal (core contract R-DM-46, D-W-108): a deleted file's row is removed by the purge once its objects are gone. Reason: 03 1 item 7 allows hard deletion only for detail rows and retention jobs and names files under neither, while the row keeps `original_name`, `sha256`, `transcript_text` and the generated `search_vector`, which 20 4.2, 4.3 and DP-T01 require to be gone; a tombstone row would need a second redaction of every content column and serves nobody, because nothing references a deleted file (its attachment row went with the message, 03 `message_attachments`).

    e. Job adapter `adapters/jobs/purge.go`: args `PurgeObjects{FileID ids.ID}` with `Kind()` `files.purge_objects`, queue `maintenance` (02 12.3), arguments only the ID (02 12.1); a cancelled context returns `ctx.Err()` (02 12.4 rule 2).

5. **Failed and blocked files** (12 12 row "Failed or blocked"; M3-03 Design note 13 d; M3-04 Design note 6 f; Ruling for docs/DECISIONS.md). `recordFileChange` (M3-07 Design note 10) inserts `PurgeObjects{FileID}` through `PurgeQueue.Enqueue` in the same unit of work whenever a change makes the file final without making it deliverable (`domain.BecameFinal(before, after)` and not `domain.Deliverable(after)`): the abort of M3-03, the size, checksum and type failures and the outcomes `infected` and `error` of M3-04, the expiry of note 6 a, and the failure transaction of M3-06 where merged. The post-commit deletes of M3-03 and M3-04 stay (they remove the bytes at once); the job is committed with the transition and so closes the crash window that M3-04 Design note 6 f leaves to this ticket. A blocked file that is attached keeps its row; its objects go all the same.

6. **`files.cleanup_pending`** (12 12, 12 3.6, 12 4.3 last bullet; 02 12.3, 12.4). Registered with `Registry.AddPeriodic(PeriodicJob{Args: CleanupPending{}, Interval: file.gc_interval, RunOnStart: true})`, handler on queue `maintenance`, in the roles `worker` and `all`. `app.Cleanup.Run(ctx)` reads `now` once from `kernel/clock` and runs the steps a to f in this order. Each step reads candidates in batches of `file.gc_batch_size` (order of note 13) until a batch is smaller than the limit or the context ends, and handles each candidate in its own short unit of work, so one failing candidate never rolls back the others (02 12.4 rule 6). A database error ends only its step (one Warn log with the step name); the handler returns the first error after the last step so River records it; a repeated run has no double effect (02 12.4 rule 1).

    a. Expired sessions (12 12 row "Session open and expired"): candidates `ExpiredOpenSessions` (session `open`, `expires_at <= now`, file not deleted). Unit of work: lock file and session row in the order of the merged abort (M3-03 Design note 13 d, which M3-04 Design note 6 a follows), re-check `domain.SessionExpired(state, expires_at, now)`, set the session `expired` and the file `upload_state = 'failed'`, delete its `file_upload_parts` rows, and call `recordFileChange`, which writes `file.updated` (no `actor`: a job without principal, 04 2.2), publishes `files.FileFinalized` and inserts the purge job of note 5. The purge removes the parts of the file system driver and aborts the multipart upload of S3 (12 4.3: the server does not rely on the bucket's lifecycle rule).

    b. Never attached (12 12 row "Never attached", 04 14 row "Drafts"): candidates `UnattachedCandidates` (`purpose = 'attachment'`, deliverable, not deleted, no `message_attachments` row, `created_at <= now - file.unattached_ttl` as a cheap prefilter). Unit of work per file: lock the row `FOR UPDATE`; re-read the facts in a new statement and require that the file is still deliverable, not deleted, not attached and `domain.UnattachedExpired(facts, now, file.unattached_ttl)`; then `DraftFileSource.HeldByUploader` for the file and its uploader; a held file stays. Otherwise `markDeleted`. A file ID in the draft of another principal never matches the query of note 8 and protects nothing.

    c. Failed and blocked rows (12 12 row "Failed or blocked"): candidates `FailedRowCandidates` (`purpose = 'attachment'`, not deleted, `upload_state = 'failed'` or `scan_status IN ('infected','error')`, no attachment row, `created_at <= now - file.failed_row_ttl`). Unit of work: lock, re-check `domain.FailedRowExpired(facts, now, file.failed_row_ttl)` and the missing attachment row, then `markDeleted`; the purge removes the row. Ruling for docs/DECISIONS.md: blocked files (`complete` with `infected` or `error`) follow this row, not the row "Never attached", because 12 12 names them here; drafts do not protect these rows, because their bytes are gone and the row only serves the uploader's display. Ruling for docs/DECISIONS.md: rows with the purposes `avatar` and `branding` are not touched by b or c, because `principals.avatar_file_id` or the setting `branding.logo_file_id`, which other modules own, may still point to them; they go when the release of M3-10 marks them deleted. Files with `purpose = 'gdpr_export'` belong to M7-06.

    d. Catch-up of purges (12 12 "catches up on missed deletions"): candidates `DeletedRows` (`deleted_at IS NOT NULL`, oldest first). For each, `Purge.Run` inline (note 4 c) instead of inserting a further job, so a long store outage does not pile up jobs (Ruling for docs/DECISIONS.md). The first storage error ends this step for the run (one Warn log with the step name and the number of rows left); the next run tries again. A row that a running purge job deletes meanwhile is skipped (`PurgeNothing`).

    e. Temporary files (12 4.2; M3-01 Out of scope): `TempSweeper.SweepTemp(ctx, now - file.upload_session_ttl)` removes the entries of `<file.fs.root>/.tmp` whose modification time is older than the bound and returns their count (one Info log with the count when it is not zero). Ruling for docs/DECISIONS.md: a writer of the driver changes its temporary file with every chunk and ends with `rename` or removal, and no write outlives an upload session (a part ends with its session, the scan pass with `file.scan.timeout`, an image job with `file.image.processing_timeout`), so an older temporary file belongs to a crashed process. The sweep reads only `.tmp`, never the key space, so the port stays without listing (12 4.1); it goes through the store's `os.Root` and never follows a symbolic link (M3-01 Design note 6), and it removes a link as an entry. With the S3 driver the sweeper does nothing.

    f. Backlog (12 14 "deletion backlog"): `PurgeBacklog` counts the rows with `deleted_at IS NOT NULL` and returns the oldest mark; the count goes to the gauge of note 12, and when the oldest mark is older than `privacy.object_delete_delay` the run logs one Warn with the count and no IDs.

7. **Ages** (12 12 "older than"; Ruling for docs/DECISIONS.md). `domain.CleanupFacts` holds `created_at`, `scanned_at` and the session's `state`, `completed_at` and `expires_at`. `LastChange` is the latest of `created_at`, `completed_at`, `scanned_at`, and `expires_at` when the session is `expired`. `UnattachedExpired` and `FailedRowExpired` are true when `now - LastChange >= ttl`. Reason: a file whose upload or scan took long (an open session up to `file.upload_session_ttl`, a scan waiting for clamd) keeps its full window after it became attachable or failed; an aborted session stores no time of its own, so its row counts from the latest stored time, at most `file.upload_session_ttl` early. `SessionExpired(state, expiresAt, now)` is true for `open` when `now` is not before `expires_at`, the same edge at which M3-03 answers `410 gone`.

8. **`sync` implements `DraftFileSource`** (note 1; 03 4.10 `drafts`). Query `HeldByUploaderDrafts` in `internal/sync/adapters/postgres/queries/drafts.sql`:

    ```sql
    -- name: HeldByUploaderDrafts :many
    SELECT DISTINCT u.file_id
      FROM unnest(@file_ids::uuid[], @uploader_ids::uuid[]) AS u(file_id, uploader_id)
      JOIN drafts d ON d.user_id = u.uploader_id
     WHERE u.file_id = ANY(d.file_ids);
    ```

    The two arrays are built from the `UploaderFile` list in the same order; the partial unique indexes of `drafts` lead with `user_id`, so no index is added. Thread drafts and channel drafts count alike, and a draft of a channel the author lost still protects, because it stays stored (M1-61 note 13). The type lives where `sync` implements the ports of other modules in the merged layout; `internal/wire` passes it to `files`. Ruling for docs/DECISIONS.md: the check runs in the deleting unit of work after the file lock and locks no draft; a draft written after the check does not save the file, the same outcome as a draft written after the deletion (04 14: a draft checks `file_ids` only in form and count), and the send then answers `409 conflict` with `reason` `file_not_attachable` (M3-07).

9. **`deleteFile`** (`DELETE /api/v1/files/{file_id}`, action `file.delete`, `rl.write`, no `Idempotency-Key`; 05 4.8, 06 7, 12 12 row "`file.delete`"; Ruling for docs/DECISIONS.md for b to d). `app.Delete(ctx, fileID)`, exposed as `Files.Delete`:

    a. Resolve without a lock: `FileForDelete` loads the row by id, a deleted one included, and M3-07's attachment read by `file_id` gives the reference. No row (never existed or already purged): `404 not_found` with the body and headers of an unknown id (06 7: after the purge the server cannot tell the file from one that never existed).

    b. Deleted (`deleted_at` set): `204` when `uploaded_by` is the caller (06 7: DELETE of a deleted resource), otherwise the same `404` as an unknown id, without asking `authorize` about the dead row (AZ-P4; a deleted file is `not_found` for everyone else, M3-10 Design note 6).

    c. Not deleted: `authorize(actor, file.delete, fileResource(f, ref))` (M3-07 Design note 9; visibility of kind `file`, M3-03 Design note 15): `not_found` answers `404`; a denial with read answers `403 forbidden` with the `reason` of `authorize`. An unattached file is visible only to its uploader, so `manage` reaches only files the caller can see; this ticket does not widen the visibility.

    d. After `authorize`: an attached file answers `409 conflict` with `reason` `file_attached` (attached files go with `message.delete`, 05 4.8); a file whose `purpose` is not `attachment` answers `409 conflict` with `reason` `purpose_not_deletable` (avatar and logo are replaced or removed through their own operations, M3-10; export files are M7-06's).

    e. Unit of work: lock the row `FOR UPDATE`, re-read deletion and attachment in a new statement (a concurrent delete or attach wins and gives the answer of b or d), then `markDeleted([]ids.ID{fileID})`; answer `204`. No event and no audit (05 4.8). A running upload's session becomes `aborted` (note 2 b), its next `uploadPart` answers `404`, and the purge discards its parts.

    f. Handler `DeleteFile` in `internal/files/adapters/httpapi/files.go` through the strict server; `deleteFile` moves to `enabled` in `api/operations.yaml` (sorted); `make codegen-check`.

10. **Writers that meet a purged row** (M3-04 Design notes 2 and 6; M3-06 Design note 10 e; Ruling for docs/DECISIONS.md). From now on a file row can vanish. Every merged job of `files` treats a missing row like a deleted file: the first read and the result unit of work of M3-04 end with `nil` and no change, and the pass deletes the objects it wrote itself (its note 6 f); a missing row is never an internal error there, because the retry would never end (12 5.3). Where M3-06 is merged, its transactions T1, T2, T3 and its failure transaction treat a missing row as a failed precondition, and its note 10 e deletes the keys the run wrote. Where M3-10 is merged, its release calls `markDeleted` (note 2). `getFile`, M3-05's download and ticket paths and M3-07's reads already answer `404` for a missing row. One test per merged writer.

11. **Configuration** (02 13 items 4 and 5; 12 18; 20 13). Add to the schema with the chapter defaults where absent: `file.unattached_ttl`, `file.failed_row_ttl`, `file.gc_interval` (chapter 12) and `privacy.object_delete_delay` (chapter 20); `file.upload_session_ttl` exists since M3-03. New key `file.gc_batch_size` (chapter 12, integer, at least 1, default `500`, "candidates per batch of a cleanup step"), added to the table of 12 18 in the same change. Ruling for docs/DECISIONS.md: 02 12.4 rule 6 asks for batches, and a limit needs a named key (rule 9). Cross-check `object_delete_delay_gc` in `configChecks` (keys `file.gc_interval` and `privacy.object_delete_delay`; the message names keys, never values): it fails unless `file.gc_interval` is shorter than `privacy.object_delete_delay`, because the catch-up of note 6 d must run at least once inside the delay; `bothy config check` and `bothy serve` then exit `2` (02 3.3). Add the check to "Cross conditions today" of 02 13 item 4. `TestConfigSchemaBuilds` lists the keys; regenerate `docs/operations/configuration.md` with `make config-reference`.

12. **Observability** (12 14; signals.md 2, 4.4, 5.3; Ruling for docs/DECISIONS.md). New instrument `bothy.file.purge.backlog`: Gauge, unit `{file}`, no attributes, source worker, Prometheus name `bothy_file_purge_backlog`, value: the number of `files` rows with `deleted_at` set at the end of the last cleanup run (note 6 f). Declare it in the instrument registry of M1-68 as an observable gauge that reads the value the cleanup stored, never a database query in the callback (M1-68: observable gauges read cheap state); before the first run it reports nothing. Recording goes through `internal/files/app/metrics.go` and `adapters/otelmetrics` (02 5 rule 5). Add the row to signals.md 4.4 and name the signal in 12 14 ("deletion backlog"); `make signal-check` stays green. No new span name: the job spans of `platform/jobs` cover both jobs. Logs and spans of purge, cleanup and `deleteFile` carry at most `bothy.file.id`, step names and counts, never file names, object keys, digests or channel names (12 14, SEC-LOG-4); metrics carry no ID (signals.md 5.3).

13. **Queries and the SQL visibility lint** (07 7.4 rule 1, AZ-P12, M1-70; `make sql-visibility`). `internal/files/adapters/postgres/queries/cleanup.sql` holds:

    - `ExpiredOpenSessions` (ordered by `expires_at`, `file_id`), `UnattachedCandidates` and `FailedRowCandidates` (ordered by `created_at`, `id`), `DeletedRows` (ordered by `deleted_at`, `id`), each with `LIMIT @batch_size`; `PurgeBacklog`; `PurgeFacts`; the lock read `LockFileForCleanup`. Exception category `system_job`, reason "job without principal; rows keyed by id or state".
    - `FileForDelete` (by id, deleted rows included): category `authorize_resolution`, reason "resolves the resource of `file.delete`"; the lock read `LockFileForDelete` of note 9 e: category `after_authorize`.
    - `sync.HeldByUploaderDrafts`: category `system_job`, reason "file cleanup asks whether a draft of the uploader holds a file; keyed by uploader".
    - The statements that set `deleted_at`, set a session `expired` or `aborted` and delete a row are writes and are not linted (M1-70 rule 2).

    Ruling for docs/DECISIONS.md: no new index (03 4.7 names the indexes of `files`; another one is a core-contract change). `ExpiredOpenSessions` uses `(expires_at) WHERE state = 'open'`; `DeletedRows` and `PurgeBacklog` read only rows that are about to go; `UnattachedCandidates` and `FailedRowCandidates` may scan `files` once per step and run, and the prefilter on `created_at` and the batch keep each statement within `database.statement_timeout`. The `files` README names this.

14. **Concurrency and lock order** (03 6.3; 02 8.2; a deadlock is retried by the unit of work up to `database.tx_retry_max`).

    - Message delete (M3-07): thread, message, files ascending, `message_attachments`; then in `markDeleted` the session rows, the `event_log` rows of the files (`Redactor`, `FOR UPDATE`) and the job insert.
    - `deleteFile` and cleanup steps b and c: the file row, then sessions and `event_log` rows in `markDeleted`; `drafts` are read without a lock.
    - Expiry (6 a): the pair in the order of the merged abort, then the `event_log` append and the job insert.
    - Purge row delete: the file row; the cascades lock the dependent rows.
    - Attach (M3-07) against cleanup or `deleteFile`: both lock the file row first; the attach checks `deleted_at` after its lock, the deleting side checks the attachment row after its lock, so a file ends attached or deleted, never both.
    - Scan outcome (M3-04) against a deletion: the outcome write locks the row and sees `deleted_at` or no row (note 10).
    - Two purges of one file, or a purge job and the catch-up: object deletes are idempotent, the row delete happens once, the second run finds no row.

15. **Specification and API** (rule 10; nothing in 00, 03, 04 or 05 changes; every sentence this ticket adds to `docs/` is English, like the existing text; YAML stays English):

    - `docs/spec/12-files.md`: 2 (after the state diagram: the purge removes a deleted file's row once its objects are gone); 4.2 (the cleanup removes temporary files in `.tmp` older than `file.upload_session_ttl`; the sweep reads only `.tmp`); 12 (the job name `files.purge_objects` instead of `file_delete` unless M3-03 changed it; the keys the purge deletes and the row removal; the redaction of `file.updated`; the answers of `deleteFile` with `reason` `file_attached` and `purpose_not_deletable`; the ages of note 7; blocked files under the failed-row rule; drafts protect only the row "Never attached"; avatar and branding rows go with their release; the catch-up; temporary files; the purge job inserted with a failure); 14 (the gauge `bothy.file.purge.backlog` for "deletion backlog"); 18 (`file.gc_batch_size`).
    - `docs/spec/02-architecture.md`: 4.3 and 4.4 as note 1; 12.3 rows `files.cleanup_pending` (trigger: periodic every `file.gc_interval` and at worker start; expiry, never attached files, failed rows, catch-up, temporary files) and `files.purge_objects` (trigger: `InsertTx` when a file is deleted or becomes failed or blocked; caught up by `files.cleanup_pending`); 13 item 4 the cross-check `object_delete_delay_gc`.
    - `docs/spec/20-privacy.md` 4.3: the purge also removes the row, and the deleting transaction redacts the payloads of the file's `file.updated` events.
    - `docs/observability/signals.md` 4.4: the row of note 12.
    - `docs/api/openapi.yaml`, description of `deleteFile` only: deletes an own file that is not attached; `204` also for a file the caller already deleted; `404` for an unknown, foreign or purged file; `409 conflict` with `reason` `file_attached` (delete the message instead) or `purpose_not_deletable`; objects and row go within `privacy.object_delete_delay`. Then `make codegen-check`, `make api-lint`, `make api-checks`, and `make api-tables` if a generated block of 06 changes.
    - `docs/architecture/c4-3-server-components.mmd` and its `.svg` (edge `sync --> files`, `make diagrams`).
    - The core-contract rules of notes 3 and 4 d are R-DM-45 and R-DM-46 (D-W-108); this ticket does not edit 03 or 04.

16. **Rulings for `docs/DECISIONS.md`** (section 10, one row `D-M3-08-<n>` each, in English, with reason and place): the source port `DraftFileSource` and the edge `sync -> files` (note 1); one deletion function that aborts an open session, redacts and inserts the purge job, without an event (note 2); the redacted shape of `file.updated` (note 3); the key list with revision r + 1, the purge plan and the row removal (note 4); the purge job inserted with every failing transition (note 5); the run order, batches and per-candidate units of the cleanup, the inline catch-up and its stop on the first storage error (note 6); blocked files under the failed-row rule without draft protection, and avatar and branding rows left to their release (note 6 c); the sweep of `.tmp` by modification time against `file.upload_session_ttl` (note 6 e); the ages from the latest stored time (note 7); the draft check without a draft lock (note 8); the answers of `deleteFile`, including `204` only for the uploader of a deleted file and the reasons `file_attached` and `purpose_not_deletable` (note 9); a missing row treated like a deleted file by every writer (note 10); `file.gc_batch_size` and the cross-check `object_delete_delay_gc` (note 11); the gauge `bothy.file.purge.backlog` (note 12); no new index and the lint categories (note 13). The two core-contract rules of notes 3 and 4 d get their R-IDs from the planner and need no row here.

## Tests (write them first)

Test world: 07 10 and 11.1 (space Support with the channels Allgemein and Eskalation; G-1 Alex `trigger` with `upload` on Support; group Leser with Sam, G-4 `read` on Support/Allgemein; Robin without a grant on Support; the Owner). Files are uploaded through the real chain with `createUpload`, `uploadPart` and `completeUpload` (M3-03) and scanned by calling the `files.scan` handler of M3-04 directly against the fake clamd server of `clamdtest` (`stream: OK`, or the EICAR reply assembled at run time). M3-01's file system driver runs in a `mktemp -d` root; the S3 strategy runs against the in-memory fake of `ObjectStore` that M3-03 uses. The fake clock moves past `file.unattached_ttl`, `file.failed_row_ttl` and `file.upload_session_ttl` (small values in the test configuration); tests call the handlers of `files.purge_objects` and `files.cleanup_pending` directly.

Unit:

- `internal/files/domain`:
    - `TestObjectKeysOf`: key `orig-1` gives `orig-1`, `orig-2` and the four variant keys with the prefix of the file; key `orig-2` gives `orig-1` to `orig-3` and the variants; a key of another file ID, a variant key, a part key and a malformed key give an error. `FuzzObjectKeysOf` (property of 12 15 row "Key schema"): every returned key passes `ValidateKey`, carries the file ID and contains no other ID.
    - `TestPlanPurge`: no row gives `PurgeNothing`; deleted rows give `PurgeObjectsAndRow` in every state; `failed`, `infected` and `error` give `PurgeObjects`; upload running, scan `pending`, `clean` and `skipped` give `PurgeNothing`.
    - `TestLastChange`, `TestUnattachedExpired`, `TestFailedRowExpired`: the latest of the four times decides; exactly `ttl` after it is expired, one nanosecond earlier is not; an `expired` session counts from `expires_at`, an `aborted` one from the latest stored time.
    - `TestSessionExpired`: `open` at and after `expires_at` is expired, before it not; `completed`, `aborted` and `expired` never.
    - `TestRedactFileEvent`: a full `file.updated` v1 payload with name, digest, scope channel, sizes and transcript fields keeps exactly the members of note 3, the result validates against `FileUpdatedEvent` and holds no JSON `null`; a second application returns the same bytes; another type, another version and invalid JSON give an error.
- `internal/files/app` (fakes: repository, counting `ObjectStore` that fails every call while `txn.InUnit(ctx)` is true, `PartStore`, `PurgeQueue`, `DraftFileSource`, clock):
    - `TestPurge_NoStoreCallInUnitOfWork` for both strategies (02 8.5).
    - `TestPurge_LiveFileUntouched`: a deliverable, a pending and a running upload get no store call and keep their row.
    - `TestPurge_FailedKeepsRow`: objects and parts deleted, row kept.
    - `TestPurge_StoreErrorReturned`: the first failing `Delete` returns its error and the row stays; a missing object counts as deleted.
    - `TestCleanup_StepOrderAndBatches`: steps run in the order of note 6; with `file.gc_batch_size` set to 2 in the test configuration five candidates are handled in three batches; one failing candidate leaves the others done.
    - `TestCleanup_CatchUpStopsOnStoreError`: the first storage error ends step d; steps e and f still run; the next run continues.
    - `TestDelete_Answers`: the table of note 9 (missing, deleted by the caller, deleted by another uploader, not visible, attached, `purpose = 'avatar'`, own unattached) with status, code and `reason`.
    - `TestOneDeletedAtWriter`: exactly one sqlc statement of `files` writes `deleted_at`.
- `internal/files/adapters/fs`: `TestSweepTemp`: an entry of `.tmp` older than the bound is removed and counted; a fresh entry and every object under `f/` and `uploads/` stay; a symbolic link in `.tmp` to a canary file outside the root is removed as a link and the canary keeps its content.
- `internal/wire`: `TestFilesCleanupConfigCheck` (`file.gc_interval` equal to or longer than `privacy.object_delete_delay` makes `bothy config check` exit `2` with a message naming both keys and no value; shorter passes); `TestConfigSchemaBuilds` lists the keys of Design note 11; the jobs golden file shows `files.purge_objects` and `files.cleanup_pending` on queue `maintenance` and the periodic job with `Interval` = `file.gc_interval` and `RunOnStart`.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain with the response validation of M1-71):

- `TestPurge_MessageDeleteRemovesObjects` (plan "deleting a message removes the objects", 12 12 row "Message deleted"): Alex sends a message with two files in Support/Allgemein, then deletes it with `deleteMessage`. In that transaction both files get `deleted_at`, exactly one `files.purge_objects` job per file is inserted (scheduled now), and the stored `file.updated` events of both files have `redacted_at`. After the purge handler ran: `ObjectStore.Head` of every original and variant key reports missing, no regular file remains under `f/<aa>/<bb>/<file_id>/` in the temporary root, and no row of `files`, `file_upload_sessions` or `file_upload_parts` exists for the files; the purge ran within `privacy.object_delete_delay` of the deletion by the fake clock. `getFile` answers Alex and Sam the same `404` as an unknown id. A test subscriber of `MessageDeleted` that vetoes leaves marks, events and jobs unchanged.
- `TestPurge_CatchUpAfterLostJob`: as above, but the purge job is removed from the River table in the test; the next `files.cleanup_pending` run removes objects and rows (12 12 "catches up on missed deletions").
- `TestCleanup_UnattachedAfterTTL` (plan "a never attached file disappears after the period", 12 15 row "Cleanup"): Alex's clean file F is never attached; one second before `file.unattached_ttl` after its completion a cleanup run keeps it; after the TTL the run marks it deleted and the purge removes object and row; `getFile` answers `404`. Variant: a file whose session stayed open almost `file.upload_session_ttl` before completion keeps its full window from the completion.
- `TestCleanup_UploaderDraftKeepsFile` (plan "a file in the draft of the uploader stays"): Alex saves a thread draft with F (`putThreadDraft`); after `file.unattached_ttl` F stays (row and object); the same with a channel draft (`putChannelDraft`); after `deleteThreadDraft` and removal of the channel draft the next run deletes F.
- `TestCleanup_ForeignDraftProtectsNothing` (plan "a foreign file ID in a draft protects nothing"): Sam saves a draft in Support/Allgemein whose `file_ids` holds the ID of Alex's F (accepted, 04 14 checks form only); after `file.unattached_ttl` F is deleted and purged; Sam's draft still lists the ID (drafts are not cleaned), and a later send of Alex with F answers `409 conflict` with `reason` `file_not_attachable`.
- `TestCleanup_ExpiredSession` (12 12 row "Session open and expired", 12 3.6): an open session past `expires_at`: session `expired`, file `upload_state = 'failed'`, part rows gone, exactly one `file.updated` with `audience_principal_id` = Alex, without `actor`, valid against `FileUpdatedEvent`; after the purge no part object remains under `uploads/<file_id>`; with the S3 strategy the fake saw `AbortMultipart` with the stored upload id; the next `uploadPart` answers `410 gone`.
- `TestCleanup_FailedAndBlockedRows` (12 12 row "Failed or blocked"): a checksum failure and an EICAR file (fake clamd): their objects are gone after the purge job inserted with the transition even when the post-commit delete of M3-04 failed (fake store fails that call once); the rows stay and `getFile` shows `failed` and `infected` to Alex until `file.failed_row_ttl`, then the rows are gone; an infected file attached to a message keeps its row after the TTL; an avatar row of a failed upload (fixture through the `pgtest` connection) stays.
- `TestDeleteFile` (05 4.8, 06 7, AZ-P4): Alex deletes his unattached clean file: `204`, objects and row gone after the purge; a second call before the purge `204`, after it the `404` of an unknown id; Sam, Robin and the Owner get that same `404` for Alex's unattached file; an attached file answers `409 conflict` with `reason` `file_attached` and stays; a file with a running upload answers `204`, its session is `aborted`, the next `uploadPart` answers `404` and the purge removes its parts.
- `TestDeleteFile_ConcurrentWithAttach`: a send with F and `deleteFile` of F at the same time: either `201` and `409 conflict` (`file_attached`) with F attached, or `409 conflict` (`file_not_attachable`) and `204` with F deleted; never both.
- `TestCleanup_ConcurrentWithAttach`: a send with F while the cleanup handles F after its TTL: F ends attached or deleted, never both.
- `TestDPT01_FilePart` (DP-T01, part files; 20 4.2, 4.3): the file name and the message text carry the run's canary value; after `deleteMessage` and the purge no row of `event_log` contains the canary in `payload` (SQL over `payload::text`), the `file.updated` events of the file are listed by `listEvents` with `redacted: true` and a payload valid against `FileUpdatedEvent`, the `message.created` event shows `attachments: []` (M1-58), and no `files`, session, part or object of the file remains. With M3-15 merged: no `file_text_extracts` row; with M3-14 merged: a search for the file name has no hit. The journal entry of DP-T01 is M7-07's.
- `TestPurge_WhileScanRuns`: `deleteFile` while the `files.scan` pass of the file runs (fake clamd delays its reply): after both no object and no row exist, and the scan job ends with `nil`.
- `TestPurge_Idempotent`: two purge jobs of one file and a catch-up run in parallel: one row delete, no error, no object left.
- `TestNoDeduplication` (R-DM-13, 20 4.3): Alex and the Owner upload identical bytes in Support/Allgemein; deleting Alex's file leaves the Owner's row and object unchanged.
- `TestCleanup_TempSweep`: a stale temporary file in `<root>/.tmp` older than `file.upload_session_ttl` is removed by the run, a fresh one stays.
- `TestCleanup_BacklogGauge` (manual metric reader): `bothy.file.purge.backlog` equals the number of rows with `deleted_at` after a run, has no attributes, and one Warn without IDs appears when the oldest mark is older than `privacy.object_delete_delay`.
- `TestPurgedRowWriters`: the `files.scan` handler for a purged file ends with `nil` and writes nothing; with M3-06 merged the same for `files.thumbnail`; with M3-10 merged a replaced avatar is purged.

Suites of other tickets, extended here:

- SEC-T10, `test/canary` (M1-69): add the path "deleted file": a file whose name carries the canary is uploaded, attached, deleted with its message, and a second one is deleted with `deleteFile`; after the purge the canary appears in no log, trace, metric attribute, audit detail, error response, remaining event payload or event delivered to Sam, Robin or the Owner.
- G7 and G8, `test/contract` (M1-71, SEC-T06): `deleteFile` is called with a fixture file; no unexpected `5xx`; every response validates.
- AZ-P4, `test/leak` (M1-70): `deleteFile` answers a foreign, an unknown and a purged id alike in status, body, headers and median time within `authz.not_found_timing_tolerance`; AZ-P12 entries of Design note 13.
- AZ-P1 (the route count includes `deleteFile` with `x-authz-action` `file.delete`) stays green.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/files/... ./internal/sync/... ./internal/messaging/... ./internal/authz/... ./internal/wire/...`
- `cd services/server && go test -race -count=1 ./test/canary/... ./test/leak/...`
- a short fuzz run of `FuzzObjectKeysOf` (`cd services/server && go test -run '^$' -fuzz FuzzObjectKeysOf ./internal/files/domain/` stopped after a short `-fuzztime`); its seed corpus runs in every `go test`
- `make codegen-check`, `make arch`, `make register-check`, `make api-lint`, `make api-checks`, `make config-reference` (no diff afterwards), `make config-doc-check`, `make signal-check`, `make sql-visibility`, `make contract-test`
- `make diagrams`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] Deleting a message removes the objects of its attachments: the files get `deleted_at` and redacted `file.updated` payloads in the deletion transaction, and `files.purge_objects` removes originals, variants and parts by their known keys and then the row within `privacy.object_delete_delay`; a lost purge is caught up by `files.cleanup_pending`.
- [ ] A never attached file disappears after `file.unattached_ttl`; a file in a draft of its uploader stays; a foreign file ID in a draft protects nothing.
- [ ] Open sessions past `file.upload_session_ttl` become `expired` with their file `failed` and one `file.updated`, and their parts and multipart upload are removed.
- [ ] Failed and blocked files lose their objects through a purge job committed with the transition; the rows of unattached attachment files go after `file.failed_row_ttl`.
- [ ] `deleteFile` is enabled: `204` for an own unattached file and for a file the caller already deleted, the `404` of an unknown id for foreign, unknown and purged files (AZ-P4), `409 conflict` with `reason` `file_attached` for an attached file.
- [ ] DP-T01, part files: after deleting a message with an attachment no file name remains in an event payload, a row, an object or a search hit.
- [ ] No object store call runs inside a database transaction, no key is listed, and deleting a file never touches the file of another person (R-DM-13).
- [ ] `bothy.file.purge.backlog` is in signals.md and in the instrument registry; no file name appears in logs, spans, metrics or problem bodies (SEC-T10).
- [ ] 12 2, 4.2, 12, 14 and 18, 02 4.3, 4.4, 12.3 and 13, 20 4.3, signals.md, the `deleteFile` description, `architecture/modules.yaml`, the C4 diagram, `configuration.md`, the READMEs, the CHANGELOG and the rows `D-M3-08-<n>` are updated; `make register-check`, `make arch`, `make signal-check` and `make sql-visibility` are green.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `files` (the jobs `files.purge_objects` and `files.cleanup_pending` with their steps, the deletion function and its callers, `deleteFile`, the ports `DraftFileSource`, `PurgeQueue` and `TempSweeper`, the gauge, the query exceptions and the missing index of Design note 13) and `sync` (dependency on `files`, `HeldByUploaderDrafts`) updated.
- Rulings of Design note 16 recorded in `docs/DECISIONS.md` (one row each with reason).
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
