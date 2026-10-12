# M3-05: File download with download tickets

Status: ready | Depends on: M3-04 | Milestone: M3

## Goal

After this ticket the server hands out the bytes of a file, its thumbnails and its transcript only through itself and only after `authorize(file.read)` at fetch time (ADR 0048, R-FI-03). `downloadFile` and `downloadThumbnail` stream from the object store with the headers of 12 "9.3 Response" and support exactly one byte range (`206`). `createFileTicket` issues a short-lived HMAC ticket bound to file, variant, principal and session, and a media player that cannot send a DPoP proof fetches with that ticket in the query parameter `ticket` alone. Every fetch with a ticket checks signature, expiry, session and current rights again, and every failure answers the same `404` as an unknown id, so no ticket survives a revoked grant, a revoked session or its own expiry. A file that is still being checked answers `409 file_not_ready` with `Retry-After`, a blocked or failed file `409 file_blocked`, and a principal without read access always gets `404`. HTML, SVG and every other type outside the inline list leave the server only as `attachment` with `application/octet-stream`, `nosniff` and the sandbox CSP, so an upload never runs as a page of the instance's origin (SEC-T03, part files). This is the read half of 12 "9. Download" and makes the M3 goal that files "reach people and agents only after a check" true for downloads.

## Read first

- `docs/spec/12-files.md`: "1. Goals and limits" (points 1 and 2), "2. States of a file" (definition of "servable" and the table), "3.7 Purpose of a file", "4.1 Port" (`Get(key, range)`, no presigned GET), "4.4 Key schema" (objects are never overwritten), "6.2 File names", "7.1 Rule" (which stored images are inline), "7.3 Thumbnails", "8.2 Transcript", "9.1 Pass-through via the server", "9.2 File tickets", "9.3 Response", "13. Endpoints and actions", "15. Tests" (rows "clamd stopped", "HTML and SVG upload", "File name with U+202E ...", "Someone else's file ID and a non-existent ID", "Ticket: range download ..."), "16. Decisions of this chapter" (R-FI-03), "18. Configuration values of this chapter".
- `docs/spec/05-actions.md`: "1. Binding rules" rules 1 and 2; "2.2 Target of a resource" (read path per `purpose`); "4.8 Files and search" (`file.read`: resource `file`, right `read`, Audit `no`, `rl.read`); `docs/spec/actions.yaml` entry `file.read` (M1-02).
- `docs/spec/07-permissions.md`: "4.1 Decisions and reasons" (`no_grant`, `not_self`, `session_revoked`, `step_up_not_possible`), "4.2 Flow" (order, audit of denials, deferred entries), "4.3 Visibility", "8.1 Delivery filter" (row "File fetch"), "8.2 Immediate effect on revocation", "11.2 Required tests" (AZ-P4).
- `docs/spec/06-api.md`: "4. Authorization in the API" point 3, "5. Errors" (`file_not_ready`, `file_blocked`, `range_not_satisfiable`, `not_found`), "7. Idempotency" (`createFileTicket` is `read_only`), "9. Rate limits", "10. Files" points 2 and 3, "15. Contract tests and gate" (G1 to G3, G7, G8).
- `docs/spec/08-identity.md`: "7.1 Overview", "7.3 DPoP" (why players need tickets).
- `docs/spec/02-architecture.md`: "4.3 Ports, events and sources per module" (`files` port `Files`: read, open, preview, file tickets, delete), "4.4 Allowed dependencies" (`files` may use `identity` and `chat`), "4.5 Cross-cutting ports (`internal/kernel`)" (`kernel/actor`, `kernel/authorize`), "9.1 Chain in the api node", "9.3 Proof of the check", "14.2 Deadlines" (download under `http.transfer_timeout`).
- `docs/spec/04-sync-protocol.md`: "8.1 Basic rule", "8.5 Revocation of a session, device or principal", "13.1 Local cache" (`client.cache_max_age`).
- `docs/spec/03-data-model.md` "4.7 Files (`files`)": `files` (`object_key`, `sha256`, `size_bytes`, `mime_type`, `original_name`, `uploaded_by`, `purpose`, `scope_channel_id`, `upload_state`, `scan_status`, `deleted_at`, the `transcript_*` columns), `file_thumbnails`, `message_attachments` (M3-07); R-DM-13, R-DM-28.
- `docs/spec/19-security.md`: "3. HTTP security headers" (SEC-HDR-1, SEC-HDR-2, SEC-HDR-4), "7. Upload hardening" (SEC-UP-3, SEC-UP-6, SEC-UP-8, SEC-UP-11), "9. Logs without secrets and without content" (SEC-LOG-3), "14. Tests" (SEC-T03, SEC-T10).
- `docs/spec/17-observability.md`: "4. Privacy", "10. Tests that hold these rules".
- `docs/spec/01-product.md` "3.4 Files and voice" (row "Files and images with preview, in and out").
- `docs/api/openapi.yaml`: operations `downloadFile`, `downloadThumbnail`, `createFileTicket`, `getTranscript`, `getFile`; schemas `File`, `Thumbnail`, `FileTicketCreate`, `FileTicket`, `Transcript`, `TranscriptStatus`, `TranscriptSource`, `Problem` (`x-problem-codes`); parameters `FileIdPath`, `VariantPath`; responses `Conflict`, `NotFound`, `RangeNotSatisfiable`, `Unauthorized`, `Forbidden`; header `Retry-After`, `ETag`; security schemes `dpopAuth`, `bearerAuth`, `fileTicket`.
- ADRs 0048 (whole document, including "Confirmation"), 0010, 0013, 0015, 0017, 0020, 0033, 0047, 0049, 0053.
- `docs/DECISIONS.md`: D-FI-03, D-FI-11, D-K-X6, D-L16, D-W-32, D-W-100; section "10. Rulings of the build milestones" (format of the rows you add).
- `docs/security/threat-model.md`: "5.8 Files, object storage, virus scanner", T-UP-01, T-AZ-02, T-AZ-03, T-LG-01. `docs/RISKS.md`: RK-43 (a long clamd outage keeps files `pending`).
- `docs/operations/configuration.md`: `file.ticket_ttl`, `file.ticket_key_ref`, `client.cache_max_age`, `http.transfer_timeout`, `http.request_timeout`, `http.unavailable_retry_after`, `secrets.cache_ttl`; section "5. Secrets" row `file_ticket_key`.
- `docs/PLAN.md`: "3. Way of working per ticket"; "9. M3 Platform services" (Goal, Scope, Out of scope, Exit criteria, Test strategy, Red team: "file tickets after revocation", Risks); rows M3-01, M3-03, M3-04, M3-06, M3-07, M3-08, M3-09, M3-10, M3-11 and M3-22.
- Briefs this ticket builds on: `docs/briefs/M1-09-secret-provider.md` (Provider, cache and rotation), `M1-10-telemetry-foundation.md` (span attribute allowlist), `M1-17-api-server-stubs.md` (enable list; `createFileTicket` is listed there for M3-05; route metadata), `M1-18-http-chain.md` (stages, headers, problem writer, proof of the check, "a file handler of M3 may override" `Cache-Control`), `M1-19-rate-limits.md` (anonymous requests use the client IP), `M1-28-authz-core.md` (descriptor, requirements, visibility, deferred `file.read`), `M1-29-authz-invariants.md` (`RevokeGrant`), `M1-33-identity-devices-dpop.md` (`identitytest`), `M1-34-identity-sessions.md` (Authenticator step 6, `Auth.RevokeSession`), and the M3 briefs `M3-01-object-storage-fs.md`, `M3-03-upload-sessions.md`, `M3-04-clamd-scan.md`.
- Dependencies implied through M3-04 and therefore not in the header: M3-03 (the `files` table, upload flow, `getFile` and with it the `file.read` requirement), M3-01 (`ObjectStore` port and file system driver), M1-55 and through it M1-09, M1-10, M1-17, M1-18, M1-28, M1-29, M1-33, M1-34.
- Code: `services/server/internal/files` (M3-01, M3-03, M3-04), `internal/identity` (M1-33, M1-34), `internal/authz` (M1-28, M1-29), `internal/kernel/authorize`, `internal/kernel/actor`, `internal/kernel/errs` (`CodeFileNotReady`, `CodeFileBlocked`, `CodeRangeNotSatisfiable`, `WithRetryAfter`), `internal/kernel/clock`, `internal/platform/httpserver`, `internal/platform/secrets`, `internal/platform/otel`, `internal/wire`, `api/routes`, `api/operations.yaml`.

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

11. Bytes of files, thumbnails and transcripts leave only through the server; there is no presigned GET and no other download path, and every fetch calls `authorize(principal, file.read, file)` with the rights at fetch time (12 "9.1", ADR 0048 point 1, R-FI-03, 05 rule 1, 07 "8.1" row "File fetch", SEC-UP-11).
12. A file is deliverable exactly when `upload_state = 'complete'`, `scan_status IN ('clean','skipped')` and `deleted_at IS NULL`. `downloadFile`, `downloadThumbnail` and `getTranscript` answer `409 file_not_ready` with `Retry-After` while the upload or the check is pending and `409 file_blocked` for `infected`, `error` or a failed upload; an authenticated caller or a ticket without read access always gets `404`, while an anonymous request without a ticket gets `401` before the file is looked up (design note 2a; 12 "2", "9.3", 06 "10" point 2, SEC-UP-6).
13. Response headers of 12 "9.3" and SEC-HDR-4: `Content-Type` is the detected type only for the inline list, otherwise `application/octet-stream`; `Content-Disposition: inline` only for the inline list (stored `image/jpeg`, `image/png`, `image/gif`, thumbnails, `audio/mp4`, `audio/mpeg`, `video/mp4`), `attachment` for everything else (HTML, SVG, PDF and the rest, SEC-UP-3), always `filename="<ASCII fallback>"` before `filename*=UTF-8''<percent-encoded>`; `X-Content-Type-Options: nosniff`; `Content-Security-Policy: default-src 'none'; sandbox`; `Cross-Origin-Resource-Policy: same-origin`; `Cache-Control` `private, max-age=<client.cache_max_age>, immutable` with normal authentication and `private, no-store` with a ticket; `ETag: "<sha256 hex>"` and `Repr-Digest: sha-256=:<base64>:` of the delivered variant; `Accept-Ranges: bytes`.
14. Exactly one byte range gives `206`; several ranges are ignored (`200` with the whole variant); a range outside the variant gives `416 range_not_satisfiable` (12 "9.3", 06 "5", ADR 0048 point 1).
15. `createFileTicket` (`file.read`, `rl.read`, `x-idempotency-exempt: read_only`) answers `201` with `FileTicket {url, expires_at}`. The ticket is `base64url(payload || HMAC-SHA-256(key, payload))`; the payload holds `file_id`, `variant`, `principal_id`, `auth_session_id` and the expiry (`file.ticket_ttl`); the key comes from the secret provider through `file.ticket_key_ref`, and two keys are valid at the same time so a key change causes no outage (12 "9.2" points 1 and 2, ADR 0048 point 2).
16. Every fetch with a ticket checks signature, expiry, that the session is not revoked, and `authorize` with current rights; every failure gives the same `404` as an unknown id (12 "9.2" point 3, ADR 0048 point 3). The ticket is accepted only in the query parameter `ticket` (security scheme `fileTicket`) and only by `downloadFile` and `downloadThumbnail` (openapi `fileTicket`); responses with a ticket carry `Cache-Control: private, no-store` (ADR 0048 point 4).
17. The ticket is a bearer secret: it never appears in the server's logs, span attributes, problem bodies or audit rows (12 "9.2" point 4, SEC-LOG-3, 17 "4", SEC-T10).
18. A foreign file id and an unknown file id give identical `404` answers (B7, 05 rule 2, R-AZ-01, 12 "15", AZ-P4).
19. The read path follows `files.purpose` (05 "2.2", 12 "3.7"): an attachment before it is attached is read only by its uploader with read access on `scope_channel_id`; `avatar` by every active authenticated principal; `branding` publicly; `gdpr_export` never through `file.read`.
20. Downloads run under `http.transfer_timeout`, not `http.request_timeout` (02 "14.2", ADR 0048 "Consequences").

## Scope

All server paths under `services/server/` unless noted.

Create:

- `internal/files/domain/ticket.go`: the ticket codec, pure and clock-injected: `EncodeTicket(key []byte, t Ticket) string`, `DecodeTicket(keys [][]byte, s string, now time.Time) (Ticket, error)` with `Ticket{FileID, PrincipalID, SessionID ids.ID; Variant Variant; ExpiresAt time.Time}` (layout in Design note 5).
- `internal/files/domain/byterange.go`: `ParseRange(header, ifRange, etag string, size int64) RangeResult` (outcomes `Full`, `Partial{First, Last}`, `Unsatisfiable`; rules in Design note 9).
- `internal/files/domain/delivery.go`: `Variant` (`original`, `small`, `medium`, `large`, `poster`), `Readiness(uploadState, scanStatus string) Readiness` (`Deliverable`, `NotReady`, `Blocked`), `Presentation(mimeType string, variant Variant) (contentType string, inline bool)`, `ContentDisposition(name string, inline bool) string`, `ThumbnailName(originalName string, v Variant, mimeType string) string`.
- `internal/files/app/download.go` (use case `Open`), `internal/files/app/tickets.go` (use case `IssueTicket`, ticket verification, key ring), `internal/files/app/transcript.go` (use case `Transcript`).
- `internal/files/adapters/httpapi/download.go`: the strict-server methods `DownloadFile`, `DownloadThumbnail`, `CreateFileTicket`, `GetTranscript` and the streaming response objects (Design note 12).
- `internal/files/adapters/postgres/queries/download.sql`: `GetFileForDelivery` (one row of `files` by id with the columns of Design note 2) and `GetThumbnail` (`file_thumbnails` by `(file_id, variant)`).
- Test helpers in `internal/files/adapters/filestest` (extend the package if M3-03 or M3-04 created it): seed a file through the real upload flow in a chosen state, seed a thumbnail object and row, set a transcript. Test code only; never wired into the server.
- Only if the base branch has no migration for `file_thumbnails` (03 "4.7"; M3-06 is not in this ticket's dependency closure): `migrations/<next free number>_files_thumbnails.sql` creating `file_thumbnails` exactly as 03 describes (columns, `CHECK`, PK `(file_id, variant)`, FK with `ON DELETE CASCADE`, unique `object_key`), then `make migrations-lock`. If M3-06 already merged it, use it and create nothing.

Change:

- `internal/files` port (root package): add to the `Files` port of M3-03 `Open(ctx context.Context, a actor.Actor, req OpenRequest) (Delivery, error)`, `IssueTicket(ctx context.Context, a actor.Actor, fileID ids.ID, v Variant) (IssuedTicket, error)` and `Transcript(ctx context.Context, a actor.Actor, fileID ids.ID) (TranscriptView, error)`; `OpenRequest{FileID ids.ID; Variant Variant; Ticket string; Range, IfRange string}`; `Delivery` carries status (`200` or `206`), total size, served range, content type, disposition, ETag, Repr-Digest, whether a ticket authenticated it, and the body reader; `IssuedTicket{URL string; ExpiresAt time.Time}`. Doc comment on each. M3-22 builds the push preview `image_url` on `IssueTicket`.
- `internal/identity` port `Auth` (M1-34): add `SessionActor(ctx context.Context, userID, sessionID ids.ID) (actor.Actor, error)` with implementation and tests (Design note 6).
- `internal/kernel/authorize` and `internal/authz`: the descriptor fact `IssueTicket bool` (`file.read` through `createFileTicket`) and the session rule of Design note 7; complete the `file.read` requirement and the `file` visibility per purpose where M3-03 left a gap (Design note 7).
- `internal/platform/secrets/stack` (M1-09): the catalogue entry `file_ticket_key` (generated, length as in `docs/operations/configuration.md` "5. Secrets", service `server`) only if it is missing.
- `internal/wire`: build the ticket key source from the M1-09 secret provider and `file.ticket_key_ref`; pass the files `Config` (`file.ticket_ttl`, `file.not_ready_retry_after`, `client.cache_max_age`) and the identity `Auth` port to `files`; wire the four handlers.
- Configuration schema (M1-07): `file.ticket_ttl` (duration, greater than zero), `file.ticket_key_ref` (secret reference) and `client.cache_max_age` (duration) if absent; the new key `file.not_ready_retry_after` (duration, greater than zero; default the same value as `http.unavailable_retry_after`). Start check: role `api` and `all` fail to start when the ticket key is missing or shorter than the length the secrets catalogue in `docs/operations/configuration.md` section "5. Secrets" names for `file_ticket_key`.
- Route metadata generator of M1-17 (`api/routes`) and `StageRoute` of M1-18 (`internal/platform/httpserver`): a flag on `routes.Operation` for operations whose `2xx` response has a `format: binary` body, and `StageRoute` applies `http.transfer_timeout` (through `http.ResponseController`) to them; regenerate, `make codegen-check`.
- `api/operations.yaml`: move `createFileTicket`, `downloadFile`, `downloadThumbnail` and `getTranscript` from `later` to `enabled`; rerun the route generator.
- `docs/api/openapi.yaml`: Design note 13.
- `docs/spec/12-files.md`: "9.2" (tickets need a session; current and previous key; every failure of the ticket path including a non-allow decision gives `404`), "9.3" (`Retry-After` key, range rules, missing variant, thumbnail ETag, thumbnail file name), "18" (row `file.not_ready_retry_after`); `docs/spec/07-permissions.md` "4.1" row `step_up_not_possible` (also `createFileTicket` without a session). Then `make config-reference`.
- `docs/security/threat-model.md` "5.8": one STRIDE row `I | Ticket URL reaches third parties (Open with, log of a player) | short validity file.ticket_ttl, binding to principal and session, file.read on every fetch, ticket never in logs | T-AZ-03, T-LG-01`.
- `test/canary` (M1-69), `test/leak` (M1-70), `tools/contract` and the G8 validator (M1-71); see Design note 14.
- READMEs of `files`, `identity`, `authz`, `internal/platform/httpserver` and `api`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M3-05-<n>`.

This ticket creates no Makefile target, no event, no job and no metric.

## Out of scope

- Upload sessions, parts and completion, `getFile`, `getUpload`, `abortUpload` and the `files`, `file_upload_sessions`, `file_upload_parts` migrations (M3-03); the `file.read` requirement itself is M3-03's because `getFile` needs it (this ticket completes it only where Design note 7 finds a gap).
- Scan pass, states and `files.scan` (M3-04); the order in which metadata removal and deliverability happen (M3-04, M3-06; see the report item in Design note 16).
- Metadata removal, thumbnail generation and writes to `file_thumbnails` (M3-06).
- Attaching files to messages, `message_attachments` and the read path "channel of the message" for attached files (M3-07); until then every attachment is unattached and only its uploader reads it.
- `deleteFile`, cleanup jobs and object purge (M3-08).
- `requestTranscription`, `submitTranscript` and voice messages (M3-09); this ticket only reads `transcript_*`.
- Avatar and logo uploads and `getBrandingLogo` (M3-10); channel export (M3-11); push preview and its `image_url` (M3-22).
- The `playback` variant and transcoding (V3; 12 "8.3"; PLAN M3 "Out of scope").
- Players and caches in the apps (M4, M5); console cookie sessions (M6-01); `downloadGdprExport` (M7).
- A reverse proxy that serves the bytes (ADR 0048, rejected option kept as later optimisation).
- The canary, leak and contract test frameworks themselves (M1-69, M1-70, M1-71); this ticket only extends them.
- Changes to the core contract.

## Boundaries

- A new migration (only the conditional `file_thumbnails` migration of Scope) needs its line in
  `migrations.sha256`: run `make migrations-lock` after creating the file and before `make check`;
  never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M3-05-file-download`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(files): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d` or `t.TempDir()`, removed at the end; the file system driver of the object store runs only in such a directory.
- Never read or print secrets. Ticket keys are generated per test run into a temporary secrets directory; tests never print a ticket, a key or a ticket URL (assert on them, log only their length). No network calls except package downloads for pinned tools and Go modules.
- No real third-party service in tests: the object store is the file system driver of M3-01; scan results come from a fake `Scanner` (or the pinned clamd container of M3-04, only if you need it); no external media player.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M3-01, M3-03 and M3-04, whose briefs were written in parallel with this one: follow their names; if a column of 03 that this ticket reads is missing from the merged `files` table, stop with `Verdict: BLOCKED <column>`.

## Design notes

1. **Layers.** The adapter `internal/files/adapters/httpapi` parses path, query and the headers `Range` and `If-Range`, calls the port and writes headers and body; it holds no rule. Ticket verification, the actor of a ticket, `authorize`, readiness, variant lookup and the range decision run in `internal/files/app`; the pure rules of Scope "Create" live in `internal/files/domain`. `app` reaches the object store only through the `ObjectStore` port of M3-01 and identity only through its port (02 "4.4"). Downloads, ticket issue and transcript reads write nothing: no unit of work, no lock, no event, no job.

2. **`Open` (downloadFile with variant `original`, downloadThumbnail with its variant), in this order:**
   a. Who asks. If the chain gave an authenticated actor, use it and ignore any `ticket` parameter (Ruling for docs/DECISIONS.md: the `Authorization` header wins; such a response gets the authenticated `Cache-Control`). If the actor is anonymous and `ticket` is not empty, verify the ticket (note 5): bad encoding, bad MAC under every key of the ring, `now >= ExpiresAt`, `FileID` different from the path or `Variant` different from the route (`original` for `content`) gives `404 not_found`. Then `identity.Auth.SessionActor(ticket.PrincipalID, ticket.SessionID)` (note 6); an error gives `404`. If the actor is anonymous and there is no ticket, call `authorize` with the anonymous actor and a descriptor that carries only kind `file` and the id: step 1 of 07 "4.2" answers `unauthenticated` before any resolution, and the chain renders `401 unauthenticated` (the operation is not public, so an anonymous caller learns nothing about the id).
   b. Load the row with `GetFileForDelivery` (`id`, `object_key`, `sha256`, `size_bytes`, `mime_type`, `original_name`, `uploaded_by`, `purpose`, `scope_channel_id`, `upload_state`, `scan_status`, `deleted_at`). No row, or `deleted_at` set, gives `404 not_found` (Ruling for docs/DECISIONS.md: a deleted file answers like an unknown id; 12 "2" ends the state "deleted" without delivery).
   c. `authorize(actor, file.read, descriptor)` with the descriptor of note 7. On the normal path a non-allow decision becomes `Decision.Err()` (`404` for `not_found`, `401`, `403` as M1-28 maps them). On the ticket path every non-allow decision, including `unauthenticated` with `session_revoked`, `principal_inactive` or `device_inactive`, becomes the same `404 not_found` (12 "9.2" point 3). Denials are audited by `authz` as for every request (07 "4.2"); do not use filter mode.
   d. Readiness (`domain.Readiness`): `upload_state = 'failed'` or `scan_status IN ('infected','error')` gives `409 file_blocked`; any other state that is not deliverable gives `409 file_not_ready` with `Retry-After` from `file.not_ready_retry_after` in whole seconds (`errs.New(errs.CodeFileNotReady, ...).WithRetryAfter(...)`). Readiness comes after `authorize`, so a principal without read access never learns the state (12 "9.3": "Without read permission always `404`").
   e. Variant. `original`: the row of `files`. A thumbnail: `GetThumbnail(file_id, variant)`; no row gives `404 not_found` (Ruling for docs/DECISIONS.md: clients request only the variants that `File.thumbnails` lists, and `file.updated` announces new ones; a missing variant of a readable file reveals nothing).
   f. Range (note 9) against `size_bytes` of the variant; `Unsatisfiable` gives `416 range_not_satisfiable` with `Content-Range: bytes */<size>`.
   g. Stream with `ObjectStore.Get(key, range)`; headers of note 8 before the first body byte. A missing object for a deliverable file is a server fault: `500 internal_error`, the file id in the error log, never in the body (Ruling for docs/DECISIONS.md). An error after the first byte aborts the connection (`http.ErrAbortHandler`); the client sees a short body, never a wrong one.
   Unknown ids end in step b without calling `authorize`, foreign ids in step c; both produce the identical problem `404` (status, body after removing `correlation_id`, headers), as M1-62 does for unknown threads.

3. **`IssueTicket` (`createFileTicket`), in this order:** the chain authenticated the actor (a `ticket` query parameter is never read here: an anonymous request gets `401`); load as 2b; `authorize(actor, file.read, descriptor with IssueTicket = true)` with the session rule of note 7 (`403 forbidden`, `reason` `step_up_not_possible`, for a principal without a session; Ruling for docs/DECISIONS.md, because 12 "9.2" binds every ticket to `auth_session_id` and a principal with an API token sends its token with every request and needs no ticket); readiness as 2d (Ruling for docs/DECISIONS.md: a ticket is never issued for a file that cannot be delivered); variant as 2e for `small`, `medium`, `large`, `poster`. Then mint with the current key: `ExpiresAt` = now plus `file.ticket_ttl`, truncated to whole seconds; `url` is the relative reference `/api/v1/files/{file_id}/content?ticket=<t>` for `original` and `/api/v1/files/{file_id}/thumbnails/{variant}?ticket=<t>` otherwise (as the `FileTicket` example); answer `201` with `FileTicket`. No `Idempotency-Key` (`read_only`); a repeated call mints a new ticket.

4. **`Transcript` (`getTranscript`):** normal authentication only (no ticket, openapi security of the operation); load as 2b; `authorize` as 2c; readiness as 2d; answer `200` with `Transcript {file_id, status}` from `transcript_status`, plus `text`, `language` and `source` only when `status = done` (no JSON `null`, ADR 0053). Any readable deliverable file answers, also one with `status = none` (Ruling for docs/DECISIONS.md). JSON responses keep the API headers of M1-18 (`Cache-Control: no-store`); there is no range on this operation (ADR 0048 point 1 names bytes with `Range` for content and thumbnails).

5. **Ticket format and keys** (12 "9.2", ADR 0048 point 2; Ruling for docs/DECISIONS.md, layout recorded in the files README):
    - Payload, fixed layout: a format version byte, the file id as raw UUID bytes, a variant code byte (`original`, `small`, `medium`, `large`, `poster` in this order), the principal id and the auth session id as raw UUID bytes, and the expiry as unsigned big-endian Unix seconds (`uint64`). Ticket = base64url without padding (RFC 4648 section 5) of payload followed by the full HMAC-SHA-256 tag over the payload. Any other length, alphabet, version or variant code is invalid. Compare tags with `hmac.Equal`.
    - Key ring: a narrow port in `internal/files/app` (`TicketKeySource`, returning the current key value and its version), implemented in `internal/wire` with the M1-09 `secrets.Provider` and `file.ticket_key_ref`. `app` keeps the key it saw before the last version change as the previous key and verifies against current and previous; it mints with current. A key change therefore takes effect per process after `secrets.cache_ttl`, and tickets minted before keep working until they expire; a ticket minted by a process that already switched may be refused by a process that has not, for at most `secrets.cache_ttl`, and the player asks for a new ticket. Write this rotation procedure into the files README.
    - The `file_ticket_key` secret is generated by `bothy secrets init` from the M1-09 catalogue (`docs/operations/configuration.md` "5. Secrets"); check that the catalogue entry exists in code and add it if missing.

6. **Actor of a ticket.** `identity.Auth.SessionActor(userID, sessionID)` builds the `actor.Actor` with the same single lookup as Authenticator step 6 of M1-34 (session joined with its device and the device's active `dpop` key): kind `user`, `ID` = `userID`, `SessionID`, `DeviceID` from the session row, `DeviceState`, `ClientComponent`, `AuthMethod`; `SessionActive` true only if the row exists, `revoked_at` is null, `expires_at` is in the future and `user_id` equals `userID`; `DeviceKeyActive` true only if the device's active `dpop` key has the `jkt` of the session's refresh family (the binding a DPoP request proves with `cnf.jkt`); no step-up time. It never rejects on these flags: `authorize` (`check_active`) does, so the denial is audited with its reason (07 "4.2"). An unknown session gives an actor with `SessionActive` false. Session state is read per call, so a revoked session takes effect on the next fetch in every process (04 "8.5").

7. **`file.read` in `authz`** (05 "2.2", rules 1 and 2; 07 "4.2", "4.3"). M3-03 replaced the deferred entry `file.read` of M1-28 when it enabled `getFile`; check the merged requirement and visibility against this table and add what is missing with a test per row (list each addition in the report). If `file.read` is still deferred, replace the entry here and remove it from the golden list of deferred actions.

    | `purpose` | Visible to (otherwise `not_found`) | Requirement after visibility |
   |---|---|---|
   | `attachment`, not attached | the uploader (`OwnerID` = `uploaded_by`) with effective `read` on the path of `scope_channel_id`; DM channels follow M1-28 (participants only, owners only as participants) | `Level(read)` |
   | `attachment`, attached | path of the message's thread (M3-07 adds this row) | `Level(read)` |
   | `avatar` | every active authenticated principal | `Auth` |
   | `branding` | everybody; anonymous callers reach it only through `getBrandingLogo` (M3-10), because `downloadFile` is not a public operation | `Public` |
   | `gdpr_export` | nobody through `file.read` (only `gdpr.export.download`, M7) | none |

    A non-uploader with read access on the channel gets `not_found`, never `forbidden` (05 rule 2). The session rule for tickets is part of the `file.read` entry: `IssueTicket` true and the principal has no session (`SessionID` zero: agents, apps, API tokens) gives `forbidden` with reason `step_up_not_possible`, whose 07 "4.1" meaning is "token without a session"; evaluate it after visibility, like step 8 of 07 "4.2", so an invisible file still gives `404`. Rights logic stays in `authz` (05 rule 1). REST decisions resolve the file's path per request from the row loaded in 2b; nothing caches a file-to-channel mapping.

8. **Headers of `200` and `206`** (12 "9.3", SEC-HDR-1, SEC-HDR-2, SEC-HDR-4; Ruling for docs/DECISIONS.md where marked):
    - `Content-Type` and disposition from `domain.Presentation`: originals with `mime_type` in {`image/jpeg`, `image/png`, `image/gif`, `audio/mp4`, `audio/mpeg`, `video/mp4`} keep their type and are `inline`; thumbnails keep their `file_thumbnails.mime_type` and are `inline`; every other type (including `text/html`, `image/svg+xml`, `application/pdf`, `image/webp` stored unchanged, `image/heic`) is `application/octet-stream` and `attachment`.
    - `Content-Disposition: <inline|attachment>; filename="<fallback>"; filename*=UTF-8''<encoded>` (RFC 6266 with RFC 8187). Fallback (ruling): every character outside printable ASCII and every `"`, `\`, `%` and `;` becomes `_`. Encoding: UTF-8 bytes, every byte outside the RFC 8187 `attr-char` set as `%XX`. The name is `original_name`, already cleaned by 12 "6.2" (M3-03); for a thumbnail it is `domain.ThumbnailName`: the original name without its last extension, then `-<variant>`, then `.jpg` or `.png` from the thumbnail's type (ruling).
    - `X-Content-Type-Options: nosniff` and `Referrer-Policy: no-referrer` (already set by M1-18), `Content-Security-Policy: default-src 'none'; sandbox`, `Cross-Origin-Resource-Policy: same-origin`.
    - `Cache-Control`: with normal authentication `private, max-age=<client.cache_max_age in whole seconds>, immutable`; with a ticket `private, no-store`. This overrides the `no-store` that M1-18 sets before the handler; problem responses keep the API headers.
    - `ETag: "<lowercase hex of sha256>"` and `Repr-Digest: sha-256=:<standard base64 of sha256>:` of the whole variant: `files.sha256` for the original. `file_thumbnails` has no checksum column (03), so for a thumbnail compute both by reading the object once in full through `ObjectStore.Get` before serving it with a second, ranged `Get` (ruling: constant memory, thumbnails are small; see spec gaps).
    - `Accept-Ranges: bytes`; `Content-Length` of the served bytes; `206` adds `Content-Range: bytes <first>-<last>/<size>`.
    - Make sure no chain stage derives an `ETag` from a JSON `version` for these routes (M1-17 `ETagStatuses`); the file handler owns it.

9. **Range rules** (`domain.ParseRange`, RFC 9110 section 14; Ruling for docs/DECISIONS.md where 12 is silent): only the unit `bytes`. One range in the forms `first-last`, `first-` or `-suffix` gives `Partial` (`last` capped at `size - 1`, a suffix longer than the variant means the whole variant); a set with more than one range gives `Full` (12 "9.3"); a syntactically invalid value is ignored and gives `Full`; a single range whose first byte is at or after `size`, or a zero-length suffix, gives `Unsatisfiable`. `If-Range` is honoured only with a strong ETag equal to the current one; any other value (also an HTTP date, the server sends no `Last-Modified`) gives `Full`. `If-None-Match` is not evaluated and there is no `304`, because the contract has none and content never changes once deliverable (12 "4.4").

10. **Retry-After value.** 12 "9.3" demands `Retry-After` but names no key. Add `file.not_ready_retry_after` to 12 "18" (default: the same value as `http.unavailable_retry_after`; meaning "`Retry-After` of the `409 file_not_ready` responses (in whole seconds)"), to the schema and through `make config-reference` to the reference (Ruling for docs/DECISIONS.md).

11. **Rate limits and audit of tickets.** A ticket request is anonymous in the chain, so M1-19 takes the `rl.read` token from the bucket of the client IP before the handler runs; an invalid ticket is a `404` from the handler, not a failed credential, so it does not count against `rl.public_auth` and is not audited, because no principal exists yet (Ruling for docs/DECISIONS.md; the MAC is not guessable). A valid ticket whose `authorize` fails is audited by `authz` with its reason.

12. **Strict server and streaming.** Return response objects that implement the generated `...ResponseObject` interfaces of `downloadFile` and `downloadThumbnail` and write headers and body themselves (`Visit...Response(w)`), so the body streams from the object store without buffering; the M1-18 response writer streams non-JSON bodies. Route metadata marks both operations as binary responses, and `StageRoute` applies `http.transfer_timeout` to them (02 "14.2"; Ruling for docs/DECISIONS.md). `StageValidate` must accept a request that carries only the `ticket` query parameter on these two operations; add a test if the validator checks security requirements.

13. **`docs/api/openapi.yaml` (spec first, rule 10):**
    - `downloadFile` and `downloadThumbnail`: header parameters `Range` and `If-Range` (`type: string`, no pattern, semantics in the description, so invalid values stay valid input that the server ignores); `200` and a new `206` with the media types of note 8 (`downloadFile`: `application/octet-stream`, `image/jpeg`, `image/png`, `image/gif`, `audio/mp4`, `audio/mpeg`, `video/mp4`; `downloadThumbnail`: `image/jpeg`, `image/png`), each with the headers of the existing `200` plus `Content-Disposition`, `Content-Security-Policy`, `Cross-Origin-Resource-Policy`, `Cache-Control`, `ETag` (described as the SHA-256 of the variant, not a version), `Repr-Digest`, `Accept-Ranges`, and `Content-Range` on `206`; add the header `Content-Range` to the response component `RangeNotSatisfiable` (only file downloads use it) and reference that component from `downloadThumbnail` too; descriptions name ADR 0048 and 12 "9.3".
    - `409` of `downloadFile`, `downloadThumbnail`, `createFileTicket` and `getTranscript`: a response component `FileConflict` (`x-status: 409`, "Problem codes: `file_not_ready`, `file_blocked`.") with headers `Correlation-Id` and `Retry-After` (sent with `file_not_ready`).
    - `createFileTicket` description: session requirement (`403 forbidden` with `reason` `step_up_not_possible`), readiness and missing variant; `getTranscript` description: no ticket.
    - Run `make api-lint`, `make api-checks`, `make api-tables`, `make codegen-check`; regenerate client packages if one has a `generate-check` target (`make api-clients`).

14. **Suites that iterate over every enabled operation** (M1-69, M1-70 and M1-71 are in this ticket's dependency closure; extend each):
    - Canary (`test/canary`): call the four operations in `scenario.go` and register every issued ticket as a canary of class `secret` (allowed only in the `createFileTicket` response and the request that carries it), so `TestSEC_T10_ScenarioCoversEveryEnabledOperation` and `TestSEC_T10_CanaryAcrossAllOutputs` pass (ADR 0048 "Confirmation").
    - Leak (`test/leak`): `TestAZ_P4_RouteCoverage` covers `downloadFile`, `downloadThumbnail` and `getTranscript`: a `file_id` fixture (a file of Alex for a requester who is not its uploader; add it if M3-03 did not) and a valid value for the non-UUID parameter `variant`.
    - Contract (`tools/contract`, G8 validator): the four operations pass G7 in both passes, and every integration response of this ticket passes G8.

15. **Rulings for docs/DECISIONS.md** (one row each, `D-M3-05-<n>`, with reason): ticket layout and key ring (note 5); session rule and its reason code (notes 3, 7); `Authorization` wins over `ticket` (note 2a); every ticket-path failure is the unknown-id `404` (note 2c); deleted file and missing variant `404` (notes 2b, 2e); missing object `500` (note 2g); readiness check on ticket issue (note 3); transcript answer (note 4); headers, fallback file name and thumbnail name (note 8); thumbnail digest by an extra read (note 8); range, `If-Range`, no `304` (note 9); `file.not_ready_retry_after` (note 10); tickets and rate limits or audit (note 11); transfer deadline for binary responses (note 12); the start check on the key length (Scope); the conditional `file_thumbnails` migration if you created it. Already fixed by the spec and not to be repeated: no presigned GET, ticket contents, `404` for every ticket failure, `Cache-Control` per path, the inline list.

16. **Report item, not a change here.** 12 "3.4" and "3.5" make a file deliverable before the job that removes image metadata, while 12 "7.1" removes metadata "when the upload completes" and fails the upload when the filter fails. This ticket serves `files.object_key` as stored; name in the report whether, on the merged base, a JPEG with location data can be downloaded between `clean` and the end of the image job (M3-04, M3-06 own the order).

## Tests (write them first)

Unit:

- `internal/files/domain`: `TestTicket_RoundTrip` (encode then decode gives the same fields; URL-safe alphabet, no padding); `TestTicket_TamperedByteRejected` (flipping any single byte of payload or tag fails); `TestTicket_WrongLengthAlphabetVersionVariantRejected`; `TestTicket_ExpiryBoundary` (valid one second before `ExpiresAt`, invalid at and after it, fake clock); `TestTicket_CurrentAndPreviousKey` (minted with the previous key verifies while it is in the ring, with an older key fails, minting uses current); `FuzzDecodeTicket` (never panics, random input never verifies).
- `internal/files/domain`: `TestParseRange_Table` (each form of note 9, caps, suffix longer than size, two ranges, invalid syntax, start at size, zero suffix, `If-Range` equal, different, date); `FuzzParseRange` (never panics, `Partial` always inside the variant).
- `internal/files/domain`: `TestPresentation_InlineList` (table over the inline types, `text/html`, `image/svg+xml`, `application/pdf`, `image/webp`, `image/heic`, thumbnails); `TestContentDisposition_Table` and `FuzzContentDisposition` (non-ASCII such as "Résumé for Sam.pdf", quotes, backslash, percent, semicolon; output has no CR, LF or unescaped quote, `filename` before `filename*`); `TestThumbnailName`; `TestReadiness_Table` (every combination of `upload_state` and `scan_status`).
- `internal/authz`: `TestFileReadRequirement` (one row per line of the note 7 table: uploader with read gets `allow`; Sam with read on the channel gets `not_found`; Robin without read gets `not_found`; an owner who is not the uploader gets `not_found`; avatar for any authenticated principal; `gdpr_export` `not_found` for its uploader); `TestFileReadIssueTicketNeedsSession` (API token principal: `forbidden` `step_up_not_possible`; invisible file with API token: `not_found`; session principal: `allow`).
- `internal/identity`: `TestSessionActor_ActiveRevokedExpiredForeignUnknown` (flags as in note 6; device and family key binding; no step-up time).
- `api/routes`: `TestOperations_BinaryResponseFlag` (set for `downloadFile` and `downloadThumbnail`, not for `getTranscript`); `internal/platform/httpserver`: `TestStageRoute_BinaryResponseUsesTransferTimeout`.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain, people, devices and sessions from the identity test helpers, the file system object store in `t.TempDir()`, a fake `Scanner` whose answer each test sets, `services/server/test/harness` (M1-71); the world: space Support with channel Allgemein, Alex with `write` and `upload`, Sam with `read`, Robin without a grant; Alex uploads every file through the real upload flow of M3-03):

- `TestDownload_InlineImageHeadersAndBody`: a PNG answers `200` with `Content-Type: image/png`, `inline`, both file name parameters, `nosniff`, the sandbox CSP, `Cross-Origin-Resource-Policy: same-origin`, `Cache-Control: private, max-age=<client.cache_max_age seconds>, immutable` (computed from the configuration, never a literal), `ETag` and `Repr-Digest` equal to the SHA-256 of the bytes, `Accept-Ranges: bytes`, `Content-Length`; the body equals the upload.
- `TestSECT03_HTMLAndSVGAsAttachment` (SEC-T03 part files; 12 "15" row "HTML and SVG upload"): an HTML file with a script and an SVG with a script answer `Content-Type: application/octet-stream`, `Content-Disposition: attachment`, `X-Content-Type-Options: nosniff` and `Content-Security-Policy: default-src 'none'; sandbox`; same for a PDF.
- `TestSECT03_FileResponseHeaders` (SEC-T03): for each kind (`200` authenticated, `200` with ticket, `206`, `416`, `409`, `404`) the header set equals the target list of SEC-HDR-1, SEC-HDR-2 and SEC-HDR-4; no `Server` version.
- `TestDownload_ContentDispositionSanitizedName` (12 "15" row "File name with U+202E, `../`, control characters"): names with U+202E, `../` and control characters give the cleaned name in both parameters, with correct fallback and encoding.
- `TestDownload_Range` (12 "9.3"): `bytes=0-99`, `bytes=100-` and `bytes=-10` give `206` with `Content-Range`, `Content-Length` and the exact slice; two ranges give `200` with the whole file; a range starting at the size gives `416 range_not_satisfiable` with `Content-Range: bytes */<size>`; an invalid value gives `200`; `If-Range` with the ETag gives `206`, with another value `200`.
- `TestDownload_ForeignAndUnknownIDSame404` (12 "15" row "Someone else's file ID and a non-existent ID"; AZ-P4 status, body and header part): Sam (reads the channel, not the uploader) and Robin (no read) request Alex's file and a random UUIDv7 on `downloadFile`, `downloadThumbnail`, `getTranscript` and `createFileTicket`: identical status, headers and body after removing `Correlation-Id`, `Date`, `correlation_id` and the id in `instance`.
- `TestDownload_NotReadyAndBlocked` (12 "15" row "clamd stopped"; EICAR row "never servable"): with the fake scanner unavailable the file stays `pending` and `downloadFile`, `downloadThumbnail`, `getTranscript` and `createFileTicket` answer `409 file_not_ready` with `Retry-After` equal to `file.not_ready_retry_after` in seconds; after the scanner answers `OK` and the job runs, `200`; a `FOUND` answer (`infected`), an error answer (`error`) and an upload whose declared SHA-256 does not match its bytes (`upload_state = 'failed'`) each give `409 file_blocked` without `Retry-After`; Sam gets `404` for the pending file, never `409`.
- `TestDownload_DeletedFileIs404`: `deleted_at` set gives the unknown-id `404`.
- `TestTicket_RangeAfterRevocationSessionRevocationExpiry` (12 "15" row "Ticket: range download; after revocation of permissions, after session revocation, after expiry"; ADR 0048 "Confirmation"; PLAN M3 Red team "file tickets after revocation"): Alex creates a ticket (`201`, relative `url`, `expires_at` = now plus `file.ticket_ttl`); the URL without `Authorization` and with `Range` gives `206` and `Cache-Control: private, no-store`; after `RevokeGrant` of Alex's grant (M1-29) the same URL gives `404`; with a new grant and a new ticket, after `Auth.RevokeSession` (M1-34) `404`; with a new session and ticket, after advancing the clock by `file.ticket_ttl`, `404`; each `404` is identical to the unknown-id `404`; after the denial aggregation of the M1-24 recorder is flushed, the audit log holds `file.read` denials for Alex with the reasons `no_grant` and `session_revoked`, and no row contains the ticket.
- `TestTicket_OnlyQueryParameterOnlyDownloads`: the ticket as `Authorization: Bearer <ticket>` gives `401`; as query parameter on `getTranscript` and `createFileTicket`, `401`; a ticket of file A on file B, a `small` ticket on `content`, an `original` ticket on `thumbnails/small` and a tampered ticket give `404`; `Authorization` plus a `ticket` uses `Authorization` (authenticated `Cache-Control`).
- `TestTicket_ThumbnailVariant`: a seeded `small` thumbnail; the ticket URL points to `thumbnails/small` and answers `200` inline with the thumbnail's type, ETag and Repr-Digest of the thumbnail bytes and the thumbnail file name; a ticket request for a missing `large` gives `404`.
- `TestCreateFileTicket_NeedsSession`: Alex with a personal API token (M1-44) gets `403 forbidden` with `reason` `step_up_not_possible`; for Robin's invisible file with that token `404`.
- `TestTicket_KeyRotation`: replace the key file; after `secrets.cache_ttl` on the fake clock old tickets still verify and new tickets verify only with the new key; a ticket under a key two versions back gives `404`.
- `TestTicket_NeverInLogsOrSpans` (17 "4", SEC-LOG-3): with log capture and the in-memory span exporter of M1-10, a ticket download and a failed one leave the ticket (raw, base64 and hex forms of the decoded bytes) in no log record, span attribute, span event or problem body; the access log shows the route pattern.
- `TestDownload_PurposeRules` (05 "2.2"): a seeded `avatar` file is readable by Robin; a `branding` file by every authenticated principal; a `gdpr_export` file gives `404` to its uploader.
- `TestTranscript_Read`: `done` returns `text`, `language`, `source`; `none` returns `status` only; no JSON `null`.
- Every response above passes the G8 validator of M1-71.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/files/... ./internal/identity/... ./internal/authz/... ./internal/kernel/... ./internal/platform/httpserver/... ./internal/wire/... ./api/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`, `make api-tables`, `make api-clients`
- `make config-reference` (no diff afterwards), `make config-doc-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `createFileTicket`, `downloadFile`, `downloadThumbnail` and `getTranscript` are enabled, registered and pass AZ-P1; `openapi.yaml` declares `206`, `416`, `Range`, `If-Range`, the file headers and `FileConflict` with `Retry-After`.
- [ ] HTML and SVG (and every type outside the inline list) are served as `attachment` with `application/octet-stream`, `nosniff` and `Content-Security-Policy: default-src 'none'; sandbox` (SEC-T03, part files); inline types and thumbnails as `inline`; both file name parameters are correct.
- [ ] One range gives `206` with `Content-Range`; several ranges give `200`; a range outside gives `416 range_not_satisfiable`.
- [ ] The ticket is accepted only in the query parameter `ticket` (security scheme `fileTicket`) and only by `downloadFile` and `downloadThumbnail`; after grant revocation, session revocation or expiry it gives `404`, identical to an unknown id.
- [ ] A pending file gives `409 file_not_ready` with `Retry-After` from `file.not_ready_retry_after`; a blocked or failed file gives `409 file_blocked`; without read access `404`.
- [ ] Foreign and unknown file ids give the same `404` on all four operations.
- [ ] `Cache-Control`, `ETag`, `Repr-Digest` and `Accept-Ranges` follow 12 "9.3" for originals and thumbnails, with and without a ticket.
- [ ] A ticket never appears in logs, span attributes or problem bodies; tickets require a session (`403 forbidden`, `step_up_not_possible`).
- [ ] 12 "9.2", "9.3", "18", 07 "4.1", the threat model and `docs/operations/configuration.md` are updated; the rulings are in `docs/DECISIONS.md`.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `files`, `identity`, `authz`, `internal/platform/httpserver` and `api` updated (port `Files` with `Open`, `IssueTicket`, `Transcript`; ticket layout, key ring and rotation procedure; delivery headers; `Auth.SessionActor`; the `file.read` rule per purpose and the session rule; binary routes under `http.transfer_timeout`).
- Rulings of Design note 15 recorded in `docs/DECISIONS.md` (one row each with reason).
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
