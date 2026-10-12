# M3-03: Upload sessions, parts and completion

Status: ready | Depends on: M1-69, M1-70, M3-01 | Milestone: M3

## Goal

A person, agent or app with `write` and the capability `upload` on a channel announces a file with name, size and SHA-256, sends it in parts of fixed size that the server checks by length and `Content-Digest` while reading, resumes after a network loss, and completes or aborts the upload, all in mode `direct`. The `files` module owns the tables `files`, `file_upload_sessions` and `file_upload_parts` exactly as 03 defines them, adds the deferred foreign key `principals.avatar_file_id`, enforces `file.max_upload_bytes`, the quota per principal, the number of open sessions and the free-space reserve of the file system driver, and enables `createUpload`, `uploadPart`, `getUpload`, `completeUpload`, `abortUpload` and `getFile`. Until a file is attached only its uploader can read it; every change of `upload_state` writes `file.updated` for that uploader only; the object key never carries a name; the file name is a canary path. A completed upload stays `pending` until the verify pass of M3-04 runs, so nothing here ever delivers bytes (12 "Goals and limits" point 1). This is the upload half of 12 sections 3 and 13 and the base for scan, download, images, attachments and cleanup.

## Read first

- `docs/spec/12-files.md`: 1 "Goals and limits", 2 "States of a file", 3.1 "Creating an upload session", 3.2 "Transferring parts (mode `direct`, default)", 3.3 "Presigned PUT (mode `presigned`, optional)" (why the mode stays off), 3.4 "Completion and verification pass", 3.5 "Sequence (direct upload with scan)", 3.6 "Error cases", 3.7 "Purpose of a file", 4.1 "Port", 4.2 "File system driver (default)", 4.3 "S3 driver (core S3 only)", 4.4 "Key schema", 6.1 "Type", 6.2 "File names", 12 "Retention, deletion, deduplication", 13 "Endpoints and actions", 14 "Observability", 15 "Tests", 16 (R-FI-01, R-FI-02, R-FI-11), 18 "Configuration values of this chapter".
- `docs/spec/03-data-model.md`: 4.7 tables `files`, `file_upload_sessions`, `file_upload_parts` (and `file_thumbnails`, `message_attachments`, `file_text_extracts` for what is not built here); 4.2 `principals` (`avatar_file_id` and its foreign key); 5 "Search vectors" (row `files`); 6.3 "Lock order"; `event_log` (`audience_principal_id`, its check); R-DM-13.
- `docs/spec/05-actions.md`: rule 1.8; 2.2 "Target of a resource" (rows `file`); 2.6 "Scopes for apps"; 4.8 "Files and search" (`file.upload`, `file.read`, `file.delete`); 6 "Durable event types and receive action" (row `file.updated`).
- `docs/spec/07-permissions.md`: 4.1 "Decisions and reasons", 4.2 "Flow" (deferred entries), 4.3 "Visibility", 4.5 (archived channels), 7.4 "Query pattern" rule 1 (exception list), 10.1 "Support only for Alex", 11.2 "Required tests" (AZ-P1 to AZ-P4, AZ-P12).
- `docs/spec/02-architecture.md`: 4.3 "Ports, events and sources per module" (row `files`), 4.4 "Allowed dependencies" (row `files`), 7.1 "Module register as the source" and 7.2 "go-arch-lint" (vendors only in adapters), 8.2 "Lock order", 8.4 "Idempotency in the Unit of Work", 8.5 "What never happens in a transaction", 9.1 "Chain in the api node" (step 5: body limit `file.max_upload_bytes`, deadline `http.transfer_timeout`), 12.3 "Job catalogue" (row `files.scan`).
- `docs/spec/04-sync-protocol.md`: 2.2 "Envelope" (`scope`, `resource`, `audience`), 7.2 "Flow" (catch-up predicate with `audience_principal_id`).
- `docs/spec/06-api.md`: 5 "Errors" (closed code list, `errors[]` pointers), 7 "Idempotency", 10 "Files", 16 "Endpoints" (table "Files").
- `docs/spec/19-security.md`: SEC-IN-4, SEC-UP-1, SEC-UP-2, SEC-UP-8, SEC-UP-9, SEC-T08, SEC-T10. `docs/spec/14-search.md` 3.2 ("files: unchanged from 03").
- `docs/api/openapi.yaml`: operations `createUpload`, `uploadPart`, `completeUpload`, `getUpload`, `abortUpload`, `getFile` (and `deleteFile`, `downloadFile` for the boundary); schemas `UploadCreate`, `UploadSession`, `UploadPart`, `HeaderPair`, `UploadPartResult`, `UploadComplete`, `UploadState`, `File`, `Thumbnail`, `FilePayload`, `FileUpdatedEvent`, `Problem` (`x-problem-codes`), `FieldError`; parameters `ChannelIdPath`, `FileIdPath`, `PartNumberPath`, `IdempotencyKey`; responses `Conflict`, `Gone`, `PayloadTooLarge`, `UnsupportedMediaType`, `TooManyRequests`. `docs/api/asyncapi.yaml`: message `event.file.updated`.
- ADRs 0007, 0008, 0009, 0010, 0013, 0015, 0017, 0050, 0052, 0053; for the boundary to later tickets 0033 and 0048.
- `docs/DECISIONS.md`: section 5 "12 Files" (D-FI-01 to D-FI-11), D-W-20 (closed problem codes), D-W-100, section 10 (row format `D-<Ticket>-<n>`).
- `docs/operations/upgrade.md` 4 "Rules for migrations (Expand/Contract)" (M3, M5, M6); `docs/operations/configuration.md` (keys of Design note 3).
- `docs/observability/signals.md` 4.4 (`bothy.file.upload.size`) and "Attributes in spans and logs" (`bothy.file.id`, never on metrics).
- `docs/security/threat-model.md` T-UP-01, T-AZ-02, T-LG-01; `docs/RISKS.md` RK-42.
- `docs/PLAN.md`: 3 "Way of working per ticket"; 9 "M3 Platform services" (Goal, Scope, Out of scope, Exit criteria, Test strategy, Red team, Risks) and rows M3-01 to M3-12, M3-15; rows M1-26, M1-55, M1-69, M1-70, M1-71.
- Briefs: `M3-01-object-storage-fs.md` (port `ObjectStore`, file system driver, key builder), M1-12 (migrations, `pgtest`), M1-13 (unit of work), M1-14 (jobs: why this ticket inserts no job), M1-16 (appender, `AudiencePrincipalID`), M1-17 (enable list; it assigns `getUpload` to this ticket), M1-18 (route class limits, problem rendering), M1-20 (the idempotency stage runs the handler inside the unit of work), M1-26 (deferred avatar foreign key), M1-28 (`Resource`, deferred `file.read`, visibility table, `adapters/fake`), M1-53 and M1-55 (chat port), M1-59 (pattern of an `adapters/unicode` outbound port), M1-63 and M1-65 (`ResourceFromEvent`, delivery of audience events), M1-69, M1-70, M1-71 (canary, leak and contract suites this ticket extends).
- Implied dependencies not in the header: M3-01 brings M1-07; M1-69 and M1-70 bring M1-71, M1-68 and M1-65 and through them M1-55 (`chat` threads), which brings M1-53 (`channels`), M1-28 (`authorize`), M1-20 (idempotency), M1-18 (HTTP chain), M1-16 (event log), M1-14 (jobs), M1-13 (unit of work) and M1-12 (migrations, `pgtest`).
- Code (all under `services/server/`): `internal/files` (M3-01: port, `ObjectStore`, `adapters/fs`, key builder), `internal/kernel/{authorize,txn,eventlog,errs,ids,clock,actor,textcheck}`, `internal/authz` (`domain/requirements.go`, `domain/visibility.go`, `adapters/fake`), `internal/chat` (port), `internal/platform/pg/pgtest`, `internal/platform/httpserver`, `internal/wire` (`configkeys_*.go`, `configchecks.go`), `api/operations.yaml`, `api/routes`, `migrations/`, `sqlc.yaml`, `architecture/modules.yaml`, `test/canary`, `test/contract`, `test/harness`, `test/leak`.

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
10. Specification first: a change to behaviour, API, event, permission or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Product requirements implemented here:

11. An upload is a session of parts of fixed size, announced with name, size and SHA-256 before any byte; only mode `direct` (12 3.1, R-FI-01, R-FI-02, 12 3.3).
12. `createUpload` checks `authorize(file.upload, channel)` first (`404` without read, `403` with read but without `write` or `upload`), then size against `file.max_upload_bytes` (`413 payload_too_large`), denied types (`415 unsupported_media_type` with `reason` `file_type_denied`), quota `file.quota_bytes_per_principal` (`409 conflict` with `reason` `quota_exceeded`), open sessions `file.upload_max_open_sessions` (`429` with `Retry-After`) and, for the file system driver, `file.fs.min_free_bytes` (`503 unavailable` with `reason` `storage_full`) (12 3.1, 3.6, 6.1; openapi `createUpload`).
13. Parts: only the session's principal (else `404`); length and `Content-Digest` (RFC 9530) are checked while reading; a mismatch gives `400 validation_failed` with `errors[].code` `part_digest_mismatch` and stores nothing; the same number with the same digest gives `200` without new work, with another digest `409 conflict` with `reason` `part_conflict`; parts arrive in parallel and in any order; an expired session gives `410 gone` (12 3.2, 3.6; openapi `uploadPart`).
14. `getUpload` lists the confirmed parts so a client resumes (12 3.2 point 5).
15. Completion requires all parts, else `409 conflict` with `reason` `upload_incomplete` and the missing numbers in `detail`; the session becomes `completed` and the answer is `200` with the `File` (12 3.4).
16. Abort deletes the parts; the file becomes `upload_state = 'failed'`, the session `aborted` (12 2, 12 12).
17. The file name is cleaned by 12 6.2 and stored as `original_name`; the object key holds no name and no channel or space id (12 4.4, SEC-UP-8); the client's MIME type is a hint only and `mime_type` stays `application/octet-stream` until detection (SEC-UP-2, 12 3.1).
18. Before attachment only the uploader reads a file (`file.read`, 05 2.2); a foreign id and an unknown id answer the same `404` (12 1 point 2, 12 15, AZ-P4).
19. Every change of `upload_state` writes the durable event `file.updated` with receive action `file.read` and no content beyond the `File` schema (12 2, 05 6).
20. Tables `files`, `file_upload_sessions`, `file_upload_parts` exactly as 03 4.7, `files.search_vector` from 03 5, and the foreign key `principals.avatar_file_id -> files(id) ON DELETE SET NULL` (03 `principals`, deferred by M1-26).
21. No object store call inside a database transaction (02 8.5).
22. The file name is a canary value of SEC-T10: it appears in no log, trace, metric, audit detail, error response or event of another principal.

## Scope

All paths under `services/server/` unless noted.

Create:

- `migrations/<next>_files_uploads.sql` and `migrations/<next + 1>_identity_principals_avatar_fk_validate.sql` (Design note 2), lines in `migrations.sha256` through `make migrations-lock`.
- `internal/files` port (root package, next to what M3-01 put there):

```go
// Uploads is the upload half of 12 section 3. The actor comes from kernel/actor in the context.
type Uploads interface {
    // Create announces an upload on a channel (12 3.1). Joins the ambient unit of work when one
    // exists (keyed request, M1-20), else opens its own.
    Create(ctx context.Context, cmd CreateUpload) (UploadSession, error)
    // PutPart stores one part in mode direct (12 3.2). Never runs inside a unit of work.
    PutPart(ctx context.Context, cmd PutPart) (PartRef, error)
    // State returns the confirmed parts of an own session (12 3.2 point 5).
    State(ctx context.Context, fileID ids.ID) (UploadState, error)
    // Complete closes an own session (12 3.4).
    Complete(ctx context.Context, cmd CompleteUpload) (File, error)
    // Abort ends an own open session and deletes its parts.
    Abort(ctx context.Context, fileID ids.ID) error
}
// Files reads file metadata (12 13, GET /api/v1/files/{id}).
type Files interface {
    Get(ctx context.Context, fileID ids.ID) (File, error)
}
// CreateUpload{ChannelID ids.ID; Name string; SizeBytes int64; SHA256 [32]byte; DeclaredMIMEType string}
// PutPart{FileID ids.ID; PartNumber int; Digest [32]byte; ContentLength int64 /* -1 unknown */; Body io.Reader}
// CompleteUpload{FileID ids.ID; Parts []PartRef}; PartRef{PartNumber int; ETag string}
// UploadSession{FileID ids.ID; Mode string; PartSizeBytes int64; PartCount int; ExpiresAt time.Time}
// UploadState{FileID ids.ID; View string /* pending, uploading, complete */; PartSizeBytes int64; Confirmed []PartRef; ExpiresAt time.Time}
// File mirrors the openapi schema File (zero values mean absent).
```

- `internal/files/domain/`: `partplan.go` (`PlanParts(size, partSize int64) (count int, last int64)`, `PartLength(n, count int, size, partSize int64) int64`), `name.go` (`CleanName(nfc string, maxBytes int) string`, steps 1 to 4 of Design note 6), `digest.go` (`ParseContentDigest(h string) ([32]byte, error)`), `session.go` (session states, allowed transitions, `View` for `UploadState`), `etag.go` (etag of a file system part), `ranges.go` (missing part numbers as ranges for `detail`), plus `PartKey(fileID ids.ID, n int)` next to M3-01's key builder if M3-01 built none.
- `internal/files/app/`: `create.go`, `parts.go`, `state.go`, `complete.go`, `abort.go`, `get.go`, `events.go` (builder of `file.updated`, reused by M3-04 to M3-10), `partstore.go` (interface `PartStore` with the `fs` and `s3` strategies over `ObjectStore`, Design note 4), `ports.go` (outbound ports: repository, `PartStore`, `FreeSpace{FreeBytes(ctx) (int64, error)}`, `NameNormalizer{NFC(s string) string}`, `TypeExtensions{Extensions(mediaType string) []string}`, channel locator over the `chat` port), `metrics.go`.
- `internal/files/adapters/postgres/queries/files.sql`, `upload_sessions.sql`, `upload_parts.sql`; sqlc output in `sqlcgen/`; the repository; the `files` entry in `sqlc.yaml`.
- `internal/files/adapters/httpapi/uploads.go` and `files.go`: strict-server methods of the six operations.
- `internal/files/adapters/unicode`: `NameNormalizer` with `golang.org/x/text/unicode/norm` (BSD-3-Clause).
- `internal/files/adapters/mimetype`: `TypeExtensions` with `github.com/gabriel-vasile/mimetype` (MIT, the library 12 6.1 names).
- `internal/files/adapters/otelmetrics`: `bothy.file.upload.size`.
- In M3-01's `internal/files/adapters/fs`: `FreeBytes` (statfs of `file.fs.root`), not part of `ObjectStore`.
- `internal/wire/configkeys_files.go` (or extend M3-01's) and the cross-checks in `internal/wire/configchecks.go` (Design note 3).
- Tests listed below.

Change:

- `internal/kernel/authorize`: `Resource.File *FileFacts` with `type FileFacts struct { Purpose string; Attached bool }` (additive, doc comment).
- `internal/authz`: requirement and visibility for `file.read` and kind `file` (Design note 15); remove `file.read` from the deferred list and its golden file; README.
- `internal/wire`: build the files use cases with M3-01's `ObjectStore`, the `PartStore` strategy chosen by `file.storage.driver`, the authorizer, unit of work, appender, clock and adapters; register the handlers.
- `api/operations.yaml`: move `abortUpload`, `completeUpload`, `createUpload`, `getFile`, `getUpload`, `uploadPart` from `later` to `enabled` (sorted); regenerate (`make codegen-check`).
- `architecture/modules.yaml`: vendor groups for `golang.org/x/text` and `github.com/gabriel-vasile/mimetype` (reuse a group of the same import path if one exists), `adapter_vendors` of `files`; regenerate `.go-arch-lint.yml` (`make arch`, `make register-check`).
- `go.mod`, `go.sum`: both libraries pinned to a released version; `make licenses` green.
- `docs/api/openapi.yaml`, `docs/spec/12-files.md`, `docs/spec/06-api.md`, `docs/spec/19-security.md` (Design note 19).
- `docs/operations/configuration.md` regenerated (`make config-reference`).
- `test/canary`, `test/contract`, `test/leak`: the new paths and routes (Tests).
- `internal/files/README.md`, `internal/authz/README.md`, `services/server/CHANGELOG.md` (Unreleased), `docs/DECISIONS.md` section 10 rows `D-M3-03-1` to `D-M3-03-20`.

This ticket creates no top-level Makefile target.

## Out of scope

- Port `ObjectStore`, file system driver, key schema and its property test, conformance suite: M3-01. S3 driver and its SeaweedFS container: M3-02.
- Verify pass (SHA-256, size, type detection, the second denied-type check, clamd), job `files.scan` and its `InsertTx` in `completeUpload`, assembly of file system parts into `orig-1`, `CompleteMultipart` for S3, `upload_state` `complete` or `failed` from the pass, audit `malware_detected`, `bothy.file.scan*` metrics: M3-04.
- Download, thumbnails download, fetch tickets, `Content-Disposition` with both `filename` parameters, `409 file_not_ready` and `409 file_blocked`: M3-05.
- Metadata removal, thumbnails and the table `file_thumbnails` (`File.thumbnails` stays empty here): M3-06.
- Attachments, `message_attachments`, `AttachmentLocator`, `FileFinalized`, `MessageCreate.attachment_file_ids`, setting `FileFacts.Attached`: M3-07.
- Expiry of open sessions (`expired`), cleanup of never attached and failed files, `deleteFile`, jobs `files.cleanup_pending` and `files.purge_objects`: M3-08.
- Voice messages and transcripts: M3-09. Avatar and logo, read rules of the purposes `avatar` and `branding`, `identity.AvatarFiles`: M3-10. Channel export: M3-11. Runner attachments: M3-12.
- File text and `file_text_extracts`: M3-15. Search over files: M3-14.
- Mode `presigned`: not planned; it stays off until the proofs of 12 3.3 exist (RK-42).
- Status page warning on `storage_full`: M7-09. Retention jobs: M7-05. Export and erasure of files (`personaldata.Participant`, purpose `gdpr_export`): M7-06. Import ingestion `IngestFile` (12 11): M8 import tickets (`createImportFile`, M8-03).
- Delivery code of the gateway and catch-up (M1-63, M1-65) stays unchanged; this ticket only writes events they can filter.
- Changes to the core contract.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M3-03-upload-sessions`; one writer per worktree.
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
- The same holds for M3-01: follow the names of its merged code (`ObjectStore`, its operations, the key builder, the file system adapter package) and list each difference in the report.
- The file system driver runs in tests only with `file.fs.root` in a directory from `mktemp -d`; never the default path of 12 18.
- No real S3 service and no clamd in this ticket: the `s3` strategy is tested against an in-memory fake of `ObjectStore`; SeaweedFS belongs to M3-02, clamd to M3-04.
- New Go libraries only pinned to a released version in `go.mod` and `go.sum`, with a license from `quality/licenses/allowed.txt`.
- `test/canary` (M1-69), `test/contract` with `test/harness` (M1-71) and `test/leak` (M1-70) must exist in the base branch; if one is missing, stop with `Verdict: BLOCKED dependency <ticket> missing` (M1-69 and M1-70 are in the header, M1-71 is in the dependency closure through both).

## Design notes

0. Start check. `make check` green on the fresh worktree, else `Verdict: BLOCKED base branch red`. M3-01 and M1-55 merged (`ObjectStore`, the file system driver and the table `channels` exist), else `Verdict: BLOCKED dependency <id> missing`. Check the suites of the last Boundaries bullet.

1. Layering (02 4.1, 7.2): domain functions are pure and use only `kernel-core`; the app layer holds the use cases and outbound ports; vendors (`x/text`, `mimetype`, pgx, OpenTelemetry) appear only in adapters. Other modules use only the root package. The actor comes from `kernel/actor`, ids are UUIDv7 from `kernel/ids`, time comes from `kernel/clock` for `created_at`, `expires_at` and every expiry comparison (never SQL `now()`), so tests control time.

2. Tables (03 4.7, verbatim). `migrations/<next>_files_uploads.sql` (goose, sequential number and slug rules of `make migrations-lint`):
    - `files` with every column, type, default and CHECK of 03 (`purpose` default `'attachment'`, `upload_state`, `scan_status`, `transcript_status`, `transcript_source`), `search_vector tsvector GENERATED ALWAYS AS (setweight(to_tsvector('simple', original_name), 'A') || setweight(to_tsvector('simple', coalesce(transcript_text, '')), 'B')) STORED` (03 5; `STORED` is checked by `make migrations-lint`); PK `id`; FKs `uploaded_by -> principals(id) ON DELETE RESTRICT`, `scope_channel_id -> channels(id) ON DELETE RESTRICT`; table CHECK `(purpose = 'attachment') = (scope_channel_id IS NOT NULL)`; UNIQUE `object_key`; indexes `(uploaded_by, created_at DESC)`, `(scan_status) WHERE scan_status = 'pending'`, `(upload_state, created_at) WHERE upload_state = 'pending'`, GIN `(search_vector)`.
    - `file_upload_sessions`: columns, defaults and CHECKs `mode`, `state` of 03; PK `file_id`; FK `file_id -> files(id) ON DELETE CASCADE`; index `(expires_at) WHERE state = 'open'`.
    - `file_upload_parts`: columns of 03; PK `(file_id, part_number)`; FK `file_id -> file_upload_sessions(file_id) ON DELETE CASCADE`.
    - `ALTER TABLE principals ADD CONSTRAINT principals_avatar_file_id_fkey FOREIGN KEY (avatar_file_id) REFERENCES files(id) ON DELETE SET NULL NOT VALID`.

    `migrations/<next + 1>_identity_principals_avatar_fk_validate.sql`: `ALTER TABLE principals VALIDATE CONSTRAINT principals_avatar_file_id_fkey` (upgrade.md M5: a new constraint on an existing table is added `NOT VALID` and validated in a later migration; M6 `migrate.lock_timeout` applies through the runner of M1-12). The new tables get their constraints and indexes at creation because they are empty. Add no column, CHECK or index that 03 does not name; the range of `part_number` (from 1 to `part_count`) is an application rule. Retention classes (`content` for `files`, `transient` for sessions and parts) go into the module README; the jobs are M7-05.

3. Configuration (12 18, 03 9; names in `docs/operations/configuration.md`). Register in the schema what M1-18 and M3-01 did not: `file.fs.min_free_bytes`, `file.upload_part_bytes`, `file.upload_session_ttl`, `file.upload_max_open_sessions`, `file.quota_bytes_per_principal`, `file.name_max_bytes`, `file.denied_mime_types`, `file.s3.presigned_upload`; reuse `file.max_upload_bytes`, `file.storage.driver`, `file.fs.root`. Cross-checks in `internal/wire/configchecks.go` (they run at start and in `bothy config check`; messages name keys, never values):
    - `upload_part_plan`: the part count of a file of `file.max_upload_bytes` with parts of `file.upload_part_bytes` stays under the S3 multipart part-count limit (12 3.1, source Q1). The limit is a protocol constant with a comment citing 12 3.1 Q1, not a setting.
    - `upload_part_size_s3` (Ruling): with `file.storage.driver = s3`, `file.upload_part_bytes` is not below the S3 minimum part size (12 3.1, Q1); otherwise every upload with more than one part would fail at `CompleteMultipart`.
    - `presigned_mode_unavailable` (Ruling): `file.s3.presigned_upload = true` fails, because the mode is not built and stays off until the proofs of 12 3.3 exist (R-FI-02, RK-42). Every session answers `mode: direct`.

4. Part storage per driver (Ruling for docs/DECISIONS.md). M3-01's `ObjectStore` has exactly the operations of 12 4.1. The app uses one interface `PartStore` with two strategies, chosen in `internal/wire` from `file.storage.driver`:
    - `fs` (12 3.2 last paragraph, 4.4): part `n` is the object `uploads/<file_id>/<n>`, written with `Put`. Completion touches no object; M3-04's verify pass concatenates the parts into `f/<aa>/<bb>/<file_id>/orig-1` and deletes them (12 3.4). Discard (abort) calls `Delete` for every key from part 1 to `part_count`; a missing object is not an error (12 4.1). The etag of a file system part is the quoted lowercase hex SHA-256 of the part, derived from `file_upload_parts.sha256`; `file_upload_parts.etag` stays NULL (03: "ETag of the S3 part"). If M3-01's `Put` refuses an existing key, delete the unconfirmed part object first (only when no row exists for that number).
    - `s3` (12 3.2 last paragraph): part `n` is multipart part `n` of the final object key. `CreateMultipart` runs lazily at the first part, outside any transaction; store its id with `UPDATE file_upload_sessions SET storage_upload_id = $id WHERE file_id = $f AND storage_upload_id IS NULL AND state = 'open'`; a request that loses this race calls `AbortMultipart` for its own id and uses the stored one. `UploadPart` returns the ETag kept in `file_upload_parts.etag`. Discard calls `AbortMultipart` when an id is stored. `CompleteMultipart` does not run in `completeUpload`; M3-04's verify pass runs it before reading.

    Reason: `createUpload` and `completeUpload` accept `Idempotency-Key`, the idempotency stage of M1-20 runs their handlers inside the unit of work (02 8.4), and 02 8.5 forbids object store calls inside a transaction; 12 3.4 step 2 ("S3: `CompleteMultipartUpload`" in the request) and 12 3.2 are updated (Design note 19). The test fake of `ObjectStore` fails every call made while `txn.InUnit(ctx)` is true.

5. `createUpload` (`POST /api/v1/channels/{channel_id}/uploads`, action `file.upload`, `rl.upload`), in this order (Ruling: 12 3.1 numbers authorize, size, quota, sessions and space; 6.1 gives the early type check no place; it comes right after the size because it needs no database):
    a. Schema validation by the generated layer (M1-18): `name`, `size_bytes` at least 1, `sha256` lowercase hex (after Design note 19), `declared_mime_type` optional.
    b. `authorize(actor, file.upload, Resource{Kind: channel, ID: channel_id})`: no read gives `404`; read without `write` gives `403 forbidden` with `reason` `level_insufficient`, without `upload` `capability_missing`; an archived or read-only channel gives `403 channel_read_only` (05 rule 1.8, 07 4.5). Apps need `files:write` (05 2.6).
    c. `size_bytes` above `file.max_upload_bytes`: `413 payload_too_large`, `detail` names the key (SEC-UP-1).
    d. Clean the name (Design note 6). Denied type (Design note 7): `415 unsupported_media_type` with `reason` `file_type_denied`.
    e. Quota, open sessions, free space (Design note 8).
    f. Insert, in the ambient unit of work when the request is keyed, else in an own one: `files` (id; `object_key` = final key of revision 1 from M3-01's key builder, without name, channel or space id; `sha256` and `size_bytes` = declared values; `mime_type = 'application/octet-stream'`; `declared_mime_type` = media type without parameters in lower case, NULL when absent; `original_name`; `uploaded_by` = actor; `purpose = 'attachment'`; `scope_channel_id` = channel; both states `pending`) and `file_upload_sessions` (`mode = 'direct'`, `state = 'open'`, `declared_size_bytes`, `declared_sha256`, `part_size_bytes = file.upload_part_bytes`, `part_count` from `PlanParts`, `created_at`, `expires_at = created_at + file.upload_session_ttl`).
    g. Answer `201` `UploadSession`: `file_id`, `mode: direct`, `part_size_bytes`, `parts` with one entry per number (`url` `/api/v1/uploads/<file_id>/parts/<n>`, `headers` `[{name: Content-Type, value: application/octet-stream}]`), `expires_at`. No event and no job: neither `upload_state` nor `scan_status` changed (12 2). `Idempotency-Key` is handled by M1-20.

6. File name (12 6.2), `original_name`: the app calls `NameNormalizer.NFC` (adapter `internal/files/adapters/unicode`, because the architecture gate allows vendors only in adapters, 02 7.2; same pattern as M1-59) and then `domain.CleanName(nfc, file.name_max_bytes)`:
    1. Keep only the last path segment: split at `/` and `\`.
    2. Remove control characters, bidi control characters (U+061C, U+200E, U+200F, U+202A to U+202E, U+2066 to U+2069) and zero-width characters (U+200B to U+200D, U+2060, U+FEFF); reuse the character classes of `kernel/textcheck` where it exposes them, so the sets stay one source. Trim white space at both ends.
    3. An empty rest, `.` or `..` becomes `file` (Ruling: 12 6.2 calls them invalid; the name is only a label, and a `400` for an already cleaned name would give the client nothing to correct).
    4. Longer than `file.name_max_bytes` UTF-8 bytes: keep the extension (the last `.` and what follows, when that part alone fits), cut the stem at a rune boundary so that stem and extension fit.

    Names are cleaned, never rejected (12 6.2, 12 15); SEC-IN-4 is aligned (Design note 19). Never log the raw or the cleaned name.

7. Denied types, early check (12 6.1): with an empty `file.denied_mime_types` nothing is checked. Otherwise the upload is denied when (a) the declared media type, lower case and without parameters (`mime.ParseMediaType`; an unparsable value gives `400 validation_failed` with pointer `/declared_mime_type` and code `invalid`), equals a denied type, or (b) the extension of the cleaned name equals, ignoring case, an extension of a denied type. Ruling: extensions come from `github.com/gabriel-vasile/mimetype` (`mimetype.Lookup` and the extensions of the found type) through the port `TypeExtensions`, because the standard library's `mime.ExtensionsByType` reads tables of the operating system and differs between hosts. The second check against the detected type is M3-04.

8. Quota, open sessions, free space:
    - Serialize per principal (Ruling): before the two counts take `pg_advisory_xact_lock` on a key derived from a files-specific namespace and the principal id; without it two parallel creates of one principal both pass. It is a domain lock in step 2 of 03 6.3.
    - Quota, only when `file.quota_bytes_per_principal` is not `0`: the sum of `size_bytes` of the principal's files with `deleted_at IS NULL AND upload_state <> 'failed'` plus the new size may not exceed it, else `409 conflict` with `reason` `quota_exceeded`. Ruling: failed files count as deleted, because their objects are removed at once (12 12) and an aborted upload must not block the quota until `file.failed_row_ttl`. The index `(uploaded_by, created_at DESC)` serves the sum.
    - Open sessions: if the principal has at least `file.upload_max_open_sessions` sessions with `state = 'open'` and `expires_at` in the future, answer `429 rate_limited` with `Retry-After` = time until the earliest of their `expires_at`, rounded up to whole seconds and never zero; `detail` names `file.upload_max_open_sessions` (Ruling: 12 3.6 names no code, and `rate_limited` is the only code with status `429`).
    - Free space, driver `fs` only: `FreeSpace.FreeBytes()` (statfs of `file.fs.root`; a local system call, not a network call) minus `size_bytes` below `file.fs.min_free_bytes` gives `503 unavailable` with `reason` `storage_full` and `Retry-After` from `http.unavailable_retry_after`; log one warning with neither name nor channel.

9. `uploadPart` (`PUT /api/v1/uploads/{file_id}/parts/{part_number}`, action `file.upload`, `rl.upload`, no `Idempotency-Key`, never inside a unit of work):
    a. Load session and file by `file_id` and `uploaded_by = actor`. No row, or `files.deleted_at` set: `404` with exactly the body and headers of an unknown id (12 3.2 point 1).
    b. `authorize(actor, file.upload, Resource{Kind: channel, ID: scope_channel_id})`: `not_found` triggers Design note 14 and answers `404`; `forbidden` answers `403`.
    c. Session `aborted` or `expired`, or `open` with `expires_at` reached: `410 gone`.
    d. `part_number` above `part_count`: `400 validation_failed` with pointer `#path/part_number` and code `out_of_range`.
    e. `Content-Digest` (RFC 9530 dictionary): the member `sha-256` must exist and hold the bytes of one SHA-256 digest; other members are ignored. Missing or malformed: `400 validation_failed` with pointer `#header/Content-Digest` and code `invalid`.
    f. A row for this number exists (12 3.2 point 3): the same digest answers `200` with `UploadPartResult` and reads no body; another digest answers `409 conflict` with `reason` `part_conflict`. This also holds after completion, so a client can repeat a part whose answer was lost.
    g. Expected length L = `PartLength` (the last part is the rest). A `Content-Length` other than L fails at once, before any byte is read, with the length error below. Otherwise wrap the body in a reader that hashes and counts, stops after one byte more than L, and at the end returns an error when the count is not L or the digest differs; give it to `PartStore.Put` with size L. The error makes the store keep nothing (file system: the temporary file is never renamed, 12 4.2; S3: the part request fails). Both cases answer `400 validation_failed` with `errors[].code` `part_digest_mismatch` (pointer `#header/Content-Digest` for a wrong digest, empty pointer for a wrong length; Ruling) and store nothing (12 3.2 point 2). If M3-01's `Put` can leave an object after a reader error, stop with `Verdict: BLOCKED` (12 4.2: "A reader never sees half an object").
    h. Short unit of work: `SELECT ... FOR SHARE` on the session row. Now `aborted` or `expired`: answer `410 gone` and delete the object just stored after the commit (the session's parts are discarded anyway). `open`: `INSERT INTO file_upload_parts ... ON CONFLICT (file_id, part_number) DO NOTHING RETURNING`; on conflict compare digests as in f. `completed`: a completed session has a row for every number, so compare with that row as in f and delete nothing (with the file system driver the key is shared with the confirmed part).
    i. Answer `200` `UploadPartResult {part_number, etag}`. No event.

    Body limit `file.max_upload_bytes` and deadline `http.transfer_timeout` come from the route class of M1-18. Parts may arrive in parallel and in any order (12 3.2 point 4).

10. The same new part number twice in parallel (Ruling): both requests stream; the first committed row wins; the other answers `200` with the same digest or `409 conflict` with `reason` `part_conflict` with another one. Because 12 4.4 fixes the part key, the later write can replace the bytes of the winner; the verify pass of M3-04 then fails the file with `checksum_mismatch` (12 3.4: the total checksum catches errors of assembly), and with S3 the stored ETag no longer matches at `CompleteMultipart`. No wrong byte is ever delivered. A confirmed part is never written again (note 9 f); "Objects are never overwritten" (12 4.4) concerns the final objects under `f/`.

11. `getUpload` (`GET /api/v1/uploads/{file_id}`, action `file.upload`, `rl.upload`): lookup and authorize as note 9 a and b. `aborted`, `expired` or `expires_at` reached: `410 gone`. Answer `UploadState` (Ruling, the schema has its own enum): `upload_state` `pending` (open, no confirmed part), `uploading` (open, at least one), `complete` (session `completed`); `part_size_bytes`; `completed_parts` ascending with etags (note 4); `expires_at`.

12. `completeUpload` (`POST /api/v1/uploads/{file_id}/complete`, action `file.upload`, `rl.upload`, `Idempotency-Key`):
    a. In the unit of work: the actor's session `FOR UPDATE`; no row: `404`.
    b. Authorize as note 9 b.
    c. Already `completed`: `200` with the current `File`, nothing changes (Ruling). `aborted`, `expired` or `expires_at` reached: `410 gone`.
    d. `UploadComplete.parts` (Ruling; in mode `direct` the confirmed rows are the truth): a duplicate number gives `400 validation_failed` (pointer `/parts`, code `duplicate`); a number outside 1 to `part_count` gives `400 validation_failed` (pointer `/parts`, code `out_of_range`); the list may name fewer parts than exist.
    e. Any number from 1 to `part_count` without a confirmed row: `409 conflict` with `reason` `upload_incomplete`; `detail` lists the missing numbers ascending as ranges (for example `4-6,9`) and nothing else (12 3.4 point 1). The session stays open.
    f. A listed `etag` that differs from the confirmed one: `409 conflict` with `reason` `part_conflict`.
    g. The sum of confirmed part sizes equals `declared_size_bytes` (holds by construction; otherwise an internal error).
    h. `state = 'completed'`, `completed_at`. No object store call (note 4), no event (the states of the file do not change), no job: `jobs.Inserter.InsertTx` of M1-14 rejects a kind without a registered handler, so M3-04 adds `files.scan` here together with its handler (02 12.3).
    i. `200` with `File` (`upload_state: pending`, `scan_status: pending`). Record `bothy.file.upload.size` (note 17).

13. `abortUpload` (`DELETE /api/v1/uploads/{file_id}`, action `file.delete`, `rl.write`, no key):
    a. Load as note 9 a.
    b. `authorize(actor, file.delete, Resource{Kind: file, ID: file_id, ChannelID: scope_channel_id, OwnerID: uploaded_by, File: &FileFacts{Purpose: "attachment", Attached: false}})` (05 4.8: `self or manage`; visibility as note 15).
    c. `aborted` or `expired`: `204`, nothing changes (06 7: DELETE of a deleted resource). `completed`: `409 invalid_state_transition` (Ruling; openapi gets the response, note 19; removing a completed, unattached file is `deleteFile`, M3-08).
    d. `open` (also after `expires_at`): one unit of work locks the session row `FOR UPDATE` (so it serializes with note 9 h and note 12 a), sets the session `aborted` and the file `upload_state = 'failed'`, deletes its `file_upload_parts` rows and appends `file.updated` (note 16). After the commit `PartStore.Discard`. A storage error after the commit is logged with the file id only and the answer stays `204`; leftovers belong to the cleanup of failed files (12 12, M3-08).
    e. `204`.

14. Permissions revoked during an upload (12 3.6, Ruling): when `authorize` answers `not_found` for an existing own session in `uploadPart`, `getUpload` or `abortUpload` (requests without `Idempotency-Key`, so no ambient unit of work), the use case aborts the open session as in note 13 d in its own unit of work and then answers `404`. In `completeUpload` the idempotency stage rolls back every final error (M1-20), so the abort cannot persist there; the session is aborted by the next request without key or ends after `file.upload_session_ttl` (M3-08). A `403` aborts nothing.

15. `getFile` and the rule of `file.read`:
    - `getFile` (`GET /api/v1/files/{file_id}`, `rl.read`): load by id with `deleted_at IS NULL` (no row: `404`); `authorize(actor, file.read, Resource{Kind: file, ID, ChannelID: scope_channel_id, OwnerID: uploaded_by, File: &FileFacts{Purpose, Attached: false}})`; answer `File` with the stored fields, `scope_channel_id`, `thumbnails: []` (M3-06 fills it), no JSON `null` (ADR 0053), and `sha256` as lowercase hex only when `upload_state = 'complete'` (Ruling: the schema says "of the stored bytes"; before the verify pass only the declared value exists). Metadata is readable in every state; "only after the scan" in 05 `file.read` governs bytes, which M3-05 serves.
    - `internal/authz`: replace the deferred entry of `file.read` with `Level(read)` (05 "read") and drop it from the golden deferred list (AZ-P3 stays green). Visibility of kind `file` (05 2.2, 07 4.3), Ruling: purpose `attachment` and not attached: visible only when the principal equals `OwnerID` and has at least `read` on the path of `ChannelID` (DM channels: participant); attached: at least `read` on the path of the hints (the message's thread and channel); purposes `avatar`, `branding`, `gdpr_export`: not visible (`no_grant`) until M3-10 and M7-06 replace this, fail closed. `File == nil` is allowed only in filter or delivery mode (`authorize.Filter(ctx)`, the delivery mode of M1-65), where the event scope is the path (`ResourceFromEvent`, M1-63); in request mode it is an internal error, so no caller widens access by omitting the facts. `file.delete` keeps `Any(Self, Cap(manage))` with the same visibility; `file.upload` stays on the channel. Document the file facts in the authz README.

16. Event `file.updated` (12 2, 05 6), here written only by the abort (notes 13 and 14): `resource {kind: file, id}` without `version` (`File` has none), `scope {space_id, channel_id}` of the scope channel (ids from the `chat` port without a permission check; if M1-53's port offers no such read, add one additively next to `Threads.Info`), receive action `file.read`, payload `FilePayload {file}` that validates against `FileUpdatedEvent`, actor of the request. Before attachment the event carries `AudiencePrincipalID` = uploader (Ruling): `ResourceFromEvent` (M1-63) carries neither uploader nor attachment, so without an audience the hub and the catch-up would deliver the uploader's file name to every reader of the channel, against 05 2.2 ("then only the uploader reads it"); 03 `event_log` allows an audience with a receive action other than `self`. M3-07 writes attached files without audience in the scope of the message. `createUpload` and `completeUpload` write no event (Ruling). Keep the builder in `internal/files/app/events.go` for M3-04 to M3-10.

17. Audit, logs, metrics: `file.upload` is audited only on a malware finding (M3-04), `file.read` and `file.delete` never (05 4.8); denials go through `authorize` as M1-28 built it. Logs and spans carry at most `bothy.file.id`; never name, digest, declared type or channel name (12 14, SEC-LOG rules of 19, signals.md). `bothy.file.upload.size` (histogram, unit `By`, no attributes, source api; signals.md 4.4) is recorded once per successful `completeUpload` with `declared_size_bytes` through `internal/files/app/metrics.go` and `adapters/otelmetrics`, the pattern of M1-68.

18. Queries (07 7.4 rule 1, AZ-P12): every reading query is keyed by `file_id` (resolution for `authorize`, loading a checked resource) or by `uploaded_by` (quota and open sessions of the actor); none lists content across channels. Add each query with this reason to the exception list of the lint of M1-70. Writes touch only rows of a session the use case just authorized. SQL names only tables of `files` (02 7.3); the migration on `principals` is the deferred foreign key that 03 and M1-26 assign to this ticket.

19. Specification and API (spec first, rule 10; nothing in 00, 03, 04 or 05 changes):
    - `docs/api/openapi.yaml`: `UploadCreate.required` adds `sha256` with `pattern: '^[0-9a-f]{64}$'` and the description "Lowercase hex SHA-256 of the whole file; the verify pass checks it", and the `createUpload` example gets a value; `uploadPart` gets a required header parameter `Content-Digest` (RFC 9530, member `sha-256`); `completeUpload` description: size and SHA-256 are checked by the verify pass, `parts` is cross-checked as note 12; `getUpload` description: the mapping of note 11; `abortUpload` gets a `409` response (`Conflict`, code `invalid_state_transition`) and a description of note 13. Then `make codegen-check`, `make api-lint`, `make api-checks`, and `make api-tables` if a generated block of 06 changes.
    - `docs/spec/12-files.md`: 2 (audience before attachment); 3.1 (`sha256` required as lowercase hex, `declared_mime_type` optional as in openapi, check order with the type check, quota without failed files, the `429` code, `Idempotency-Key` as the openapi parameter); 3.2 (part objects for the file system, lazy `CreateMultipart` for S3, etag rule); 3.4 step 2 (S3 `CompleteMultipart` in the verify pass); 3.5 (parts and completion answer `200`, as openapi); 6.2 (`.` and `..` become `file`); 3.4 and 12 use the job names of 02 12.3 (`files.scan`, `files.purge_objects` instead of `file_verify`, `file_delete`).
    - `docs/spec/06-api.md` 10 point 2: `complete` leads to the check of size and SHA-256 (no longer "optionally").
    - `docs/spec/19-security.md` SEC-IN-4: file names are cleaned by 12 6.2 instead of rejected.

20. Rulings for `docs/DECISIONS.md` section 10, one row each with date, decision, reason and place: D-M3-03-1 `sha256` required and lowercase hex (note 19); -2 part storage per driver (note 4); -3 etag of a file system part (note 4); -4 check order of `createUpload` (note 5); -5 early type check with extensions from `mimetype` (note 7); -6 quota without failed files (note 8); -7 advisory lock per principal (note 8); -8 `429 rate_limited` with `Retry-After` until the earliest expiry (note 8); -9 name cleaning details (note 6); -10 `Content-Digest` parsing, pointers and codes (note 9); -11 answers per session state (notes 9, 11, 12, 13); -12 cross-check of `UploadComplete.parts` (note 12); -13 mapping of `UploadState` (note 11); -14 parallel writes of one new part (note 10); -15 abort after lost read (note 14); -16 audience of `file.updated` and no event on create and complete (note 16); -17 visibility of kind `file` and `File == nil` (note 15); -18 `File.sha256` only when complete (note 15); -19 start checks (note 3); -20 discard after the commit with cleanup by M3-08 (note 13).

## Tests (write them first)

Unit:

- `internal/files/domain`:
    - `TestPlanParts` and `TestPartLength`: size equal to the part size, one byte more, smaller, an exact multiple; count rounded up; the last part is the rest.
    - `TestCleanName`: U+202E inside a name, `../../x.pdf`, `a\b.txt`, control and zero-width characters, a decomposed input stays composed after the adapter, `.`, `..`, empty and blank give `file`; over `file.name_max_bytes` (test config) keeps the extension and never splits a rune. `FuzzCleanName`: output is valid UTF-8, has no control, bidi or zero-width character, no `/` or `\`, is never empty, fits the limit and is stable when cleaned again.
    - `TestParseContentDigest` (valid, other members ignored, missing member, wrong length, bad base64, wrong form) and `FuzzParseContentDigest` (never panics; accepts only one SHA-256 digest).
    - `TestSessionTransitions` (open to completed, open to aborted; completed to aborted and aborted to completed rejected) and `TestUploadStateView`.
    - `TestFSPartETag` and `TestMissingRanges`.
- `internal/files/app` (fakes: repository, `ObjectStore`, authorizer from `internal/authz/adapters/fake` or a kernel-level fake, clock):
    - `TestCreateUploadCheckOrder`: a table where several checks fail at once; the first in the order of note 5 decides.
    - `TestS3PartStrategy`: the first part calls `CreateMultipart` once; two parallel first parts keep one id and abort the other; `UploadPart` gets the stored id and its ETag is stored; abort calls `AbortMultipart`; completion calls nothing on the store.
    - `TestNoObjectStoreCallInUnitOfWork`: the fake fails on any call while `txn.InUnit(ctx)` is true; create, parts, complete and abort pass with both strategies.
    - `TestDeniedTypeEarlyCheck`: declared type with parameters and other case, extension match, unknown type, empty list.
- `internal/authz`: `TestFileVisibility` (uploader with read on the scope channel allows; uploader after losing read gets `not_found`; Sam with read on the channel and Robin get `not_found` for Alex's unattached file; the owner gets `not_found`; attached facts with read on the message's channel allow; purposes `avatar`, `branding`, `gdpr_export` give `not_found`; `File == nil` is an internal error in request mode and resolves the hint path in filter mode), `TestFileDeleteVisibility`, and the updated `TestDeferredActionsGolden`.
- `internal/wire`: `TestFilesConfigChecks` for `upload_part_plan`, `upload_part_size_s3` and `presigned_mode_unavailable`.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain with the response validation of M1-71, M3-01's file system driver rooted in a `mktemp -d` directory; world after 07 10.1: space Support with channel Allgemein, Alex with `trigger` and `upload` on Support, Sam with `write` but without `upload` on Support, Robin without a grant on Support, one owner):

- `TestMigrationFilesTables`: columns, types, defaults and CHECKs of the three tables as in 03; `search_vector` is a stored generated column with its GIN index; unique `object_key`; FKs with their `ON DELETE` actions; `principals_avatar_file_id_fkey` exists and is validated; a second `migrate up` changes nothing; a `files` row with `purpose = 'avatar'` and a scope channel fails the table check.
- `TestCreateUpload_Session`: `201`, `mode` `direct`, `part_size_bytes` from config, one `parts` entry per number with the documented URL and header, `expires_at` = clock plus `file.upload_session_ttl`; rows as note 5 f; `object_key` follows 12 4.4 and contains neither the name nor the channel or space id; no `event_log` row.
- `TestCreateUpload_Rights`: Robin `404`; Sam `403 forbidden` with `reason` `capability_missing`; an archived channel `403 channel_read_only`.
- `TestCreateUpload_Limits`: over `file.max_upload_bytes` `413 payload_too_large` with the key in `detail`; quota exceeded `409 conflict` with `reason` `quota_exceeded`, and an aborted upload frees the quota; at `file.upload_max_open_sessions` `429` with `Retry-After` until the earliest expiry; free space below the reserve (fake `FreeSpace`) `503 unavailable` with `reason` `storage_full`; denied type by declared type and by extension `415 unsupported_media_type` with `reason` `file_type_denied`.
- `TestCreateUpload_ParallelAtLimit`: many parallel creates of Alex one below the open-session limit: exactly one succeeds; the same for the quota.
- `TestCreateUpload_IdempotencyKey`: a repeat returns the stored body with `Idempotent-Replay: true` and creates no second file.
- `TestCreateUpload_NameCleaned` (12 15 row "File name with U+202E, `../`, control characters"): the stored `original_name` is cleaned; `getFile` shows the cleaned name.
- `TestUploadPart_SameDigest200` (plan): the same part twice with the same digest answers `200` twice, one row, the object is written once (fake call count or file modification time).
- `TestUploadPart_OtherDigest409` (plan, 12 15 row "Part sent twice, identical or different"): another digest for a confirmed number answers `409 conflict` with `reason` `part_conflict`; the stored part is unchanged.
- `TestUploadPart_WrongLengthRejected` (plan "wrong length rejected"): a shorter and a longer body, for a middle part and the last part, and a wrong `Content-Length`, each `400 validation_failed` with `errors[].code` `part_digest_mismatch`; no row; no object at the part key.
- `TestUploadPart_WrongDigestRejected`: same result for a correct length with a wrong digest.
- `TestUploadPart_HeaderAndNumber`: missing or malformed `Content-Digest` and a part number above `part_count` give `400 validation_failed` with the pointers of note 9.
- `TestUploadPart_ForeignAndUnknown404`: Sam's part for Alex's session and a part for an unknown id answer the same status, body and headers.
- `TestUploadPart_ParallelAnyOrder`: all parts at once in reverse order; all confirmed; `getUpload` lists them.
- `TestUploadPart_Expired410` and `TestUploadPart_Aborted410` (clock moved past `expires_at`; session aborted).
- `TestGetUpload_Resume`: `pending`, `uploading`, `complete` with ascending `completed_parts` and etags; foreign and unknown ids the same `404`; an aborted session `410 gone`.
- `TestCompleteUpload_MissingParts409` (plan "missing parts `409`"): `409 conflict` with `reason` `upload_incomplete`, `detail` names exactly the missing numbers as ranges; the session stays open.
- `TestCompleteUpload_Completes`: `200` `File` with both states `pending`; session `completed` with `completed_at`; a second complete without the same key answers `200` and changes nothing; no job and no event row.
- `TestCompleteUpload_PartsCrossCheck`: duplicate and out-of-range numbers `400 validation_failed`; a wrong etag `409 conflict` with `reason` `part_conflict`.
- `TestAbortUpload_DeletesParts` (plan "abort deletes the parts"): after `204` every part key reports missing through `ObjectStore.Head` and no regular file remains under `uploads/<file_id>` in the temporary root; part rows gone; session `aborted`; file `failed`; exactly one `file.updated` with `audience_principal_id` = Alex, receive action `file.read`, scope of Support/Allgemein, payload valid against `FileUpdatedEvent`.
- `TestAbortUpload_States`: a second abort `204` without a new event; abort of a completed session `409 invalid_state_transition`; Sam and Robin `404`.
- `TestLostReadAbortsSession` (12 3.6): revoke Alex's grant on Support; the next `uploadPart` answers `404`, the session is `aborted`, its parts are deleted.
- `TestGetFile_UploaderOnly` (12 15 row "Someone else's file ID and a non-existent ID"): Alex gets `200` with `scope_channel_id`, empty `thumbnails`, no `sha256` while pending, no JSON `null`; Sam (read on the channel), Robin and the owner get the same `404` as for an unknown id; a row with `deleted_at` set answers `404`.
- `TestPutFailingReaderLeavesNothing`: M3-01's file system `Put` with a reader that fails at its end leaves no object (precondition of note 9 g).
- `TestUploadSizeMetric`: one histogram point per completion, without attributes (in-process exporter).

Suites of other tickets, extended here:

- SEC-T10, `test/canary` (plan "the canary scenario contains a file name"): add the path "file name": Alex creates an upload whose name carries the run's canary value, sends a part, completes, reads it with `getFile`, aborts a second upload with the same name and provokes a `413` and a `400` with it; the canary appears in no log, trace, metric, audit detail or error response and in no event delivered to Sam, Robin or the owner.
- G7 and G8, `test/contract` (M1-71, SEC-T06): every new operation is called; add fixtures or hooks so `uploadPart`, `getUpload`, `completeUpload` and `abortUpload` reach a real session with a matching `Content-Digest`; no unexpected `5xx`; every response validates.
- AZ-P4, `test/leak` (M1-70): `getFile` and `getUpload` answer a foreign and an unknown id alike in status, body, headers and median time within `authz.not_found_timing_tolerance`; AZ-P12 exception entries for the files queries (note 18).
- AZ-P1 (route count includes the six routes), AZ-P2 (`file.updated` is a registered event type) and AZ-P3 (`file.read` no longer deferred) stay green.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/files/... ./internal/authz/... ./internal/kernel/... ./internal/wire/... ./migrations/...`
- `cd services/server && go test -race -count=1 ./test/canary/... ./test/leak/...`
- `make migrations-lock` (once after creating the migrations), `make migrations-lint`, `make codegen-check`, `make arch`, `make register-check`, `make api-lint`, `make api-checks`, `make config-doc-check`, `make licenses`, `make docs`
- `make check`

## Acceptance criteria

- [ ] Migrations create `files`, `file_upload_sessions` and `file_upload_parts` exactly as 03 4.7 with `files.search_vector` from 03 5, and add `principals.avatar_file_id -> files(id) ON DELETE SET NULL` as `NOT VALID` plus a later `VALIDATE`.
- [ ] `createUpload`, `uploadPart`, `getUpload`, `completeUpload`, `abortUpload` and `getFile` are enabled and registered; AZ-P1 passes; `file.read` is no longer deferred.
- [ ] `createUpload` checks permissions, size, type, quota, open sessions and free space in the order of Design note 5 with the codes of 12 3.6; the name is cleaned and the object key holds no name, channel or space id.
- [ ] A part with the same digest answers `200`; another digest answers `409 conflict` with `reason` `part_conflict`.
- [ ] A part of the wrong length (or with a wrong digest) is rejected with `400 validation_failed` and `errors[].code` `part_digest_mismatch`, and nothing is stored.
- [ ] Completion with missing parts answers `409 conflict` with `reason` `upload_incomplete` and names the missing numbers.
- [ ] Abort deletes the parts, marks the session `aborted` and the file `failed`, and writes one `file.updated` for the uploader only.
- [ ] Only the uploader reads an unattached file; foreign and unknown ids answer the same `404` (AZ-P4 for `getFile` and `getUpload`).
- [ ] No object store call happens inside a database transaction.
- [ ] The canary scenario contains a file name and finds it nowhere it must not be (SEC-T10).
- [ ] openapi, 12, 06 and 19 are updated as Design note 19 says; the rulings `D-M3-03-1` to `D-M3-03-20` are in `docs/DECISIONS.md`.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `files` (status, port, tables with retention classes, actions, event, part strategies, outbound ports, query exceptions of note 18) and `authz` (file facts and visibility of note 15) updated.
- Rulings of note 20 recorded in `docs/DECISIONS.md` (one row each with reason).
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
