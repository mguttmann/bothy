# M3-06: Images: removing metadata and thumbnails

Status: ready | Depends on: M3-04 | Milestone: M3

## Goal

When this ticket is done, every JPEG and PNG the server stores is free of metadata in the original itself: a lossless segment and chunk filter with an allow list writes a new original revision `orig-2` whose image data equals the uploaded data byte for byte, with the orientation as the only EXIF tag and the unchanged ICC profile, and without GPS, XMP, IPTC, comments, embedded thumbnails or appended images. GIF is re-encoded, still WebP becomes PNG or JPEG, and the formats the pure Go build cannot process stay download cards. The job `files.thumbnail` then renders the variants `small`, `medium` and `large` in pure Go with the orientation applied and the colour profile of the original, reads width, height and duration of MP4 and QuickTime files with a small own box reader, and never decodes an image whose pixel count exceeds `file.image.max_pixels`. An image becomes deliverable (`upload_state = 'complete'`) only in the transaction that stores its cleaned original, so neither a person nor an agent ever receives the location data of a photo. This is Ruling L16 with ADR 0047, R-SE-06 and SEC-UP-10, and the image part of the M3 goal that files, images and voice messages reach people and agents only after they are checked.

## Read first

- `docs/spec/12-files.md`: 1 "Goals and limits" (point 5); 2 "States of a file" (the table, "servable", "final", the `file.updated` paragraph); 3.4 "Completion and verification pass"; 3.5 "Sequence (direct upload with scan)"; 3.7 "Purpose of a file" (all purposes take the same path); 4.1 "Port" (`ObjectStore`); 4.4 "Key schema" (`orig-<n>` with "1 = as uploaded", the variant keys, "Objects are never overwritten"); 7 "Images: metadata and thumbnails" with 7.1 "Rule" (the table and every paragraph after it), 7.2 "Protection against decompression bombs", 7.3 "Thumbnails"; 8.1 "Format and duration" (`mvhd`, used later by M3-09); 9.3 "Response" (inline list, served by M3-05); 10 "Attaching and agent invocation" ("final", `files.FileFinalized`); 12 "Retention, deletion, deduplication" (row "Failed or blocked"); 14 "Observability"; 15 "Tests" (rows "JPEG with GPS EXIF ...", "truncated JPEG", "Image with huge dimensions and a small file size"); 16 R-FI-07; 18 "Configuration values of this chapter" (`file.image.*`, `file.thumbnail.*`).
- `docs/adr/0047-remove-image-metadata-losslessly.md` (whole; "Confirmation" lists the tests this ticket writes), ADR 0010 (object storage), ADR 0033 (scan), ADR 0048 (download), ADR 0053 (no JSON `null`), ADR 0007 (migrations), ADR 0008 and ADR 0050 (layers, register, arch gate).
- `docs/spec/03-data-model.md` (core contract, read only): `files` (`object_key`, `sha256` "over the stored bytes", `size_bytes`, `mime_type`, `width`, `height`, `duration_ms`, `upload_state`, `scan_status`, `deleted_at`, the closing paragraph on JPEG and PNG), `file_thumbnails`, `file_upload_sessions` (`declared_sha256`, `failure_reason` "code without contents"), 6.3 "Lock order".
- `docs/spec/02-architecture.md`: 4.3 "Ports, events and sources per module" (`files` emits `FileFinalized`), 5 "Layers within a module" (rules 2, 4 and 5), 6 "Code layout" (`internal/<module>/adapters/<module>test`), 8.5 "What never happens in a transaction", 12.2 "Queues" (`files`), 12.3 "Job catalogue" (rows `files.scan` and `files.thumbnail`), 12.4 "Rules for jobs" (rules 1, 2, 5 and 6).
- `docs/spec/05-actions.md`: 4.8 "Files and search" (`file.upload` is audited only on a malware finding), 6 "Durable event types and receive action" (row `file.updated`).
- `docs/spec/04-sync-protocol.md`: 2.2 "Envelope" (`scope`, `resource`).
- `docs/spec/19-security.md`: 7 "Upload hardening" (SEC-UP-3, SEC-UP-4, SEC-UP-5, SEC-UP-10), SEC-LOG-4 and SEC-LOG-6, 14 "Tests" (SEC-T08), 15 "Decisions of this chapter" (R-SE-03, R-SE-06).
- `docs/spec/18-release.md`: 7 "Server images" (static binary without cgo).
- `docs/api/openapi.yaml`: schemas `File` (`sha256` as hex of the stored bytes, `width`, `height`, `duration_ms`, `thumbnails`), `Thumbnail`, `FilePayload`, `FileUpdatedEvent`, `UploadCreate` (no `poster` field, see "Out of scope"); operations `getFile` (M3-03), `downloadFile` and `downloadThumbnail` (M3-05) for context. `docs/api/asyncapi.yaml`: message `event.file.updated`.
- `docs/security/threat-model.md`: 5.8 "Files, object storage, virus scanner" and T-UP-01. `docs/RISKS.md`: RK-41.
- `docs/observability/signals.md`: 4.4 "Push, files, search, jobs" and the span `job <bothy.job.kind>`.
- `docs/operations/configuration.md`: the subsection "12 Files" (keys and environment names).
- `docs/PLAN.md`: 3 "Way of working per ticket"; 9 "M3 Platform services" (Goal, Exit criteria 1 and 2, Test strategy "decompression bombs", Red team "metadata in images" and T-UP-01, Risks "re-encoding loses color fidelity"); rows M3-03, M3-04, M3-05, M3-06, M3-07, M3-09 and M3-10; 7 table "Configuration names (W1)" row "images" (the plan row of M3-06 still uses a retired synonym of `file.image.max_pixels`; the key is `file.image.max_pixels`).
- `docs/DECISIONS.md`: D-L16, D-FI-06, D-FI-11, D-R-OBJEKTSPEICHER, D-W-100, section 10 "Rulings of the build milestones".
- Briefs: M3-04 (direct dependency: `files.scan`, `Scanner`, detected `mime_type`, the `file.updated` helper), M3-03 and M3-01 (implied through M3-04: `files` tables, `getFile`, upload sessions, `ObjectStore` and the file system driver, the key schema), and from M1: M1-05 (register, arch gate), M1-07 (config schema and `config.Key`), M1-08 (configuration reference, `planned.yaml`, retired names), M1-12 (migrations, `pgtest`), M1-13 (unit of work, `txn`, `pg.Tx`), M1-14 (`platform/jobs`), M1-16 (`eventlog.Appender`), M1-23 (image built with `CGO_ENABLED=0`).
- Code after the merge of the dependencies: `internal/files` (port, `domain`, `app`, `adapters/postgres`, the object storage port with the file system driver, the `files.scan` handler and its adapter package, the test helpers `internal/files/adapters/filestest` or whatever M3-03 and M3-04 named them), `internal/platform/jobs`, `internal/platform/pg` with `pgtest`, `internal/kernel/txn`, `internal/kernel/eventlog`, `internal/wire` (`configkeys_*.go`, `configchapters.go`, `configchecks.go`, job registration), `architecture/modules.yaml`, `migrations/`.

The header names only M3-04. M3-03, M3-01, M1-55 and M1-07 and the M1 chain behind them are implied through it.

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

11. Images are free of metadata as soon as they are stored, with no switch to turn this off; JPEG and PNG originals are cleaned losslessly with an allow list, never with a block list (12 7.1, Ruling L16, ADR 0047, R-SE-06, R-SE-03, SEC-UP-10).
12. JPEG keeps exactly the markers for header, tables, frames and image data (`SOI`, `DQT`, `DHT`, `DRI`, `SOFn`, `SOS` with its data, `EOI`), `APP0` (JFIF), `APP2` only with the identifier `ICC_PROFILE` (all parts of a multi-part profile), `APP14` (Adobe) and one newly written `APP1` with only the EXIF tag "Orientation"; all other `APPn` (EXIF with its embedded thumbnail, XMP, IPTC, Multi-Picture Format), `COM` and every byte after the first `EOI` fall away (12 7.1 table, ADR 0047 point 1).
13. PNG keeps exactly `IHDR`, `PLTE`, `IDAT`, `IEND`, `tRNS`, `cHRM`, `gAMA`, `iCCP`, `sBIT`, `sRGB`, `cICP`, `mDCV`, `cLLI`, `bKGD`, `pHYs`, `acTL`, `fcTL` and `fdAT`; `tEXt`, `zTXt`, `iTXt`, `eXIf`, `tIME`, `hIST`, `sPLT` and every unknown chunk fall away, also when it is marked safe to copy (12 7.1 table, ADR 0047 point 2).
14. If the filter fails on a file detected as JPEG or PNG (broken structure), the file is not stored: `upload_state = 'failed'` and no object remains in the store (12 7.1, ADR 0047 point 3, 12 15 row "truncated JPEG").
15. The cleaned original is the object `orig-2`; `files.object_key` points to it in the same transaction, `orig-1` is deleted afterwards, and no object is ever overwritten; `sha256`, `size_bytes`, `mime_type`, `width` and `height` then describe the stored object, and `file_upload_sessions.declared_sha256` keeps the declared checksum (12 4.4 and 7.1, 03 `files`).
16. GIF is re-encoded losslessly with its palette; comments and application blocks except the loop count fall away. Still WebP is decoded and stored as PNG with alpha or as JPEG without alpha with `file.image.jpeg_quality`. Animated WebP, HEIC, HEIF, AVIF, TIFF and other types are stored unchanged and shown as download cards (`file.image.libvips_enabled` is not built, see "Out of scope"). SVG is never inline and never gets a thumbnail (12 7.1, ADR 0047 point 5).
17. Thumbnails `small`, `medium` and `large` have the longest edges `file.thumbnail.small_px`, `file.thumbnail.medium_px` and `file.thumbnail.large_px`, are never larger than the original, are JPEG with `file.thumbnail.jpeg_quality` or PNG when the image has alpha, have the orientation applied and carry no metadata except the unchanged colour profile of the original (12 7.1 point 2, 12 7.3, ADR 0047 point 4).
18. Before decoding, the server reads only header and dimensions; when width times height (for GIF times the number of frames) exceeds `file.image.max_pixels`, nothing is decoded and no thumbnail exists; every image job has the time limit `file.image.processing_timeout`; at most `file.image.max_concurrent` image jobs run at once per worker process (12 7.2, SEC-UP-4, SEC-T08).
19. Pure Go in the standard build: no cgo, no libvips, no external process (12 7.1, ADR 0047 "Confirmation", 18 "Server images"); the parsers run in the worker's jobs, never in a request of the API process (SEC-UP-5 with its exception for pure Go image previews).
20. Width, height and duration of MP4 and QuickTime videos come from the boxes `mvhd` and `tkhd`, read with a small own reader (12 7.3).
21. Every change of `upload_state`, of the thumbnails or of the file's dimensions writes the durable event `file.updated` (resource `file`, receive action `file.read`) with the full `File` and without content or JSON `null` (12 2, 05 6, ADR 0053).
22. The work runs in the job `files.thumbnail` of queue `files`, inserted with `InsertTx` in the final transaction of the scan; no object store call inside a database transaction; every run can repeat without double effect (02 12.3, 12.4 and 8.5).
23. Logs, spans and metrics carry no file name, no metadata value and no image byte (12 14, SEC-LOG-4, SEC-LOG-6).

## Scope

All paths under `services/server/` unless noted. No API operation is enabled, no action, event type or top-level make target is added.

Create:

- `internal/files/domain/images.go` (pure, no I/O):
    - `type OriginalPlan uint8` with `PlanNone`, `PlanStripJPEG`, `PlanStripPNG`, `PlanReencodeGIF`, `PlanConvertWebP`; `func OriginalPlanFor(mimeType string) OriginalPlan` for the detected types `image/jpeg`, `image/png`, `image/gif` and `image/webp`.
    - `func HasContainerMetadata(mimeType string) bool` for MP4 and QuickTime video and MP4 audio, with the exact type names the pinned `mimetype` library of M3-04 detects (table test).
    - `func ExceedsPixelLimit(width, height, frames, maxPixels int64) bool` with overflow-safe multiplication.
    - `type Orientation uint8` (EXIF values 1 to 8) with `func (o Orientation) DisplaySize(width, height int) (int, int)`.
    - `type Variant string` with `VariantSmall`, `VariantMedium`, `VariantLarge`, `VariantPoster`; `func RenderedVariants() []Variant` (small, medium, large in this order) and `func VariantSize(width, height, longestEdge int) (int, int)`.
    - `func JPEGMarkerKept(marker byte) bool` and `func PNGChunkKept(chunkType string) bool`: the allow lists of 12 7.1 as data (Design notes 4 and 6).
- `internal/files/app/images.go`: the use case `ProcessImage(ctx context.Context, fileID ids.ID) error` (Design note 3) and the outgoing ports `ImageCodec` and `MovieReader` (Design note 2); the repository methods it needs go into the existing outgoing repository port of `files`.
- `internal/files/adapters/imaging/`: `jpeg.go` (segment filter), `exif.go` (orientation reader and `APP1` writer), `png.go` (chunk filter), `gif.go` (block scan for frames and loop count, re-encoding), `webp.go` (RIFF scan for animation, `EXIF` and `ICCP`, conversion), `guard.go` (header read and pixel check before any decode), `render.go` (orientation, resampling, encoding of variants), `icc.go` (read and write of `APP2` `ICC_PROFILE` segments and `iCCP`), `codec.go` (implements `app.ImageCodec`), unit and fuzz tests, test image builders under the package's test files.
- `internal/files/adapters/isobmff/reader.go`: `func ReadMovieInfo(r io.ReaderAt, size int64) (MovieInfo, error)` with `MovieInfo` holding display width and height, duration in milliseconds and flags for which values were found; implements `app.MovieReader`.
- The job: args `ThumbnailArgs struct { FileID ids.ID }` with `Kind() == "files.thumbnail"`, its handler and the per-process limiter, in the adapter package where M3-04 put the handler of `files.scan` (for example `internal/files/adapters/jobs/thumbnail.go`).
- `internal/files/adapters/postgres/queries/images.sql`: lock and read of the file row, the updates of Design note 10, insert and read of `file_thumbnails`, the session's `failure_reason`.
- Migration `migrations/<next free number>_file_thumbnails.sql` creating `file_thumbnails` exactly as 03 defines it (columns, `CHECK` on `variant`, primary key `(file_id, variant)`, foreign key with `ON DELETE CASCADE`, unique `object_key`), grants to `bothy_app` `SELECT`, `INSERT` and `DELETE` only (rows are never updated, so variant objects are never rewritten). Skip this file if a merged migration of M3-03 or M3-05 already creates the table, and say so in the report.

Change:

- The final transaction of the scan use case of `files.scan` (M3-04): Design note 1.
- The mapping to `File` used by `getFile` and by the payload of `file.updated`: `thumbnails` lists the `file_thumbnails` rows of the file (Design note 12).
- `internal/platform/jobs`: if `HandlerOptions` has no per-job timeout yet, add `Timeout time.Duration` (zero keeps the River default), mapped to River's per-worker timeout, with a test; 02 12.4 rule 5 lets a job set the limit named in its chapter. Update the README of `platform/jobs`.
- `internal/wire`: register the handler with queue `files` and `Timeout = file.image.processing_timeout`; build the codec, the movie reader and the limiter; register the configuration keys of Design note 13 in `configkeys_files.go` (or the file M3-01 created for `file.*` keys) and the cross-check `thumbnail_sizes_ordered` in `configchecks.go`; regenerate `architecture/config-schema.json` with `go generate ./tools/configschema`.
- `architecture/modules.yaml`: a vendor group `ximage: [golang.org/x/image, golang.org/x/image/**]`, added to `adapter_vendors` of `files`; regenerate `.go-arch-lint.yml` with `go generate ./tools/archgen`.
- `go.mod` and `go.sum`: `golang.org/x/image` at its newest release (BSD-3-Clause is on `quality/licenses/allowed.txt`).
- `docs/spec/12-files.md` (English, like the chapter): 2 (two rows: image being cleaned, broken image structure), 3.4 (job names of 02 12.3 and the order of Design note 1), 3.5 (the steps after `clean`), 7.1, 7.2 and 7.3 (the spec-level rulings of Design note 17, marked as such).
- `docs/spec/02-architecture.md` 12.3: the trigger of `files.thumbnail` reads "after `clean` or `skipped`: image originals (12 7.1), thumbnails (12 7.3), dimensions and duration of MP4 and QuickTime".
- `docs/architecture/seq-7-file-upload.mmd` (steps after `clean`) and its `.svg` via `uv run --project tools --locked --quiet python tools/docs/render_diagrams.py --write docs/architecture/seq-7-file-upload.mmd`.
- `docs/operations/configuration.md` section 4 regenerated with `make config-reference`; `tools/configdoc/planned.yaml` only if `make config-doc-check` reports a rule that no planned key matches any more.
- `internal/files/README.md` (jobs, adapters, vendors, rules of this ticket), `services/server/CHANGELOG.md`, `docs/DECISIONS.md` section 10 rows `D-M3-06-<n>` (Design note 17).

## Out of scope

- Upload sessions, parts, completion, the tables `files`, `file_upload_sessions` and `file_upload_parts`, and `getFile` itself (M3-03). The object storage port and its drivers (M3-01, M3-02).
- The read pass, clamd, type detection and `files.scan` itself (M3-04); this ticket changes only the final transaction of the scan for the types of Design note 1.
- Delivery: `downloadFile`, `downloadThumbnail`, tickets, `Content-Disposition` and the inline list (M3-05). Design note 16 names the marker M3-05 uses.
- Attachments, `files.FileFinalized` and the release of agent invocations (M3-07); Design note 16 says how this ticket publishes the event when M3-07 is merged first.
- Cleanup of failed rows, never attached files and deleted objects (M3-08).
- Voice messages: format and duration rules, transcripts (M3-09); M3-09 uses the box reader of this ticket.
- Avatar and logo (M3-10), which reuse this pipeline.
- Text extraction (M3-15) and the playback variant (V3-11).
- The `poster` variant of videos: 12 3.1 and 7.3 let the client send a poster with the upload, but `UploadCreate` in `openapi.yaml` has no such field and no table holds the bytes before processing; reported as a spec gap. `file.poster_max_bytes` is not registered here.
- The optional libvips build (`file.image.libvips_enabled`, conversion of HEIC, HEIF, AVIF, TIFF and animated WebP): no ticket builds it yet; the key stays planned and an operator who sets it gets the unknown-key error of the config loader.
- A metadata filter for WebP (ADR 0047 "More Information": a decision of its own).
- Metadata inside other files (PDF, Office, archives; 12 7.1 "Limit of this promise").
- Showing images in the apps (M4-22, M5).
- Changes to the core contract.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M3-06-image-metadata-thumbnails`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(files): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d` or `t.TempDir()`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M3-01, M3-03 and M3-04, whose briefs were written next to this one.
- Test images are generated by the tests themselves (synthetic pixels, synthetic EXIF with GPS coordinates that point nowhere, a synthetic ICC profile with a valid header); no real photos, no photos of people, no files downloaded from the internet. A committed fixture must come with the test code that generated it.
- No cgo, no libvips, no external image or metadata tool (for example an EXIF command line tool) in code or tests; tests check outputs with the Go standard library and the parsers of this ticket.
- No real third-party service in tests: the object store is the file system driver in `t.TempDir()`; integration tests run with `file.scan.mode = off` (`scan_status = 'skipped'` takes the same path as `clean`, 12 3.4), so no clamd container is needed.
- New Go modules only at a pinned version in `go.mod` and `go.sum`; `make licenses` must stay green.

## Design notes

1. Order of processing (Ruling for docs/DECISIONS.md).
   a. `files.scan` keeps its single read pass (R-FI-04). Change only its final transaction for the verdicts `clean` and `skipped`: when `domain.OriginalPlanFor(mime_type)` is not `PlanNone`, it writes `scan_status`, `scanned_at`, `scan_engine` and the detected `mime_type` but leaves `upload_state = 'pending'`, and inserts `files.thumbnail` with `InsertTx`; when `domain.HasContainerMetadata(mime_type)`, it sets `upload_state = 'complete'` as before and inserts `files.thumbnail`; for every other type nothing changes. `infected`, `error` and a failed checksum or size never insert the job. The scan's `file.updated` stays as M3-04 built it.
   b. Reason: 12 3.4 sets `complete` before the processing of section 7, which leaves a window in which the original with location data is deliverable and already "final" for a held agent invocation (12 10); 12 7.1 says images are free of metadata as soon as they are stored, and SEC-UP-10 says always. Deferring `complete` closes the window: M3-05 answers `409 file_not_ready` while `upload_state = 'pending'`, and `files.FileFinalized` (M3-07) fires only when the cleaned original is stored or the file failed. Update 12 2, 3.4 and 3.5 and the sequence diagram accordingly.
   c. An M3-04 test that expects `complete` right after the scan of a JPEG, PNG, GIF or WebP changes its expectation to `pending` plus the queued job; never delete it.

2. Ports and packages (02 5).
    - `internal/files/app` declares two outgoing ports. `ImageCodec` with three operations: clean an original for a plan (input: an opener for the stored object, so the filter can read it twice; output: a body with its size and SHA-256, the resulting `mime_type`, display width and height, or the errors `ErrImageMalformed`, `ErrTooManyPixels`, `ErrImageUndecodable`, `ErrImageAnimated`), inspect an image (dimensions, frames, orientation, alpha, profile, without decoding pixels) and render the variants (input: the stored original and the edges, qualities and pixel limit; output: per variant the encoded bytes, `mime_type`, width and height). `MovieReader` with `ReadMovieInfo`. `app` sees only `io`, plain values and its own error values; image types stay in the adapter.
    - `adapters/imaging` implements `ImageCodec`, `adapters/isobmff` implements `MovieReader`; `internal/wire` connects them. Only these adapters import `golang.org/x/image` (`webp` for decoding, `draw` for resampling), enforced by the vendor group in `architecture/modules.yaml` and `make arch`.
    - The use case reads objects only through the `ObjectStore` port of M3-01 (`Get` with a range, `Put` with a size, `Delete`); a small `io.ReaderAt` over ranged `Get` calls lives in `app` for the box reader.

3. One run of `files.thumbnail` (args `ThumbnailArgs{FileID}`, queue `files`, `HandlerOptions.Timeout = file.image.processing_timeout`, River's default attempts and backoff):
   a. Take a slot of a per-process limiter of size `file.image.max_concurrent`; waiting honours the job context, so a run that times out while waiting is retried by River.
   b. Read the file row outside a transaction. Return `nil` when the row is gone, `deleted_at` is set, `upload_state = 'failed'`, `scan_status` is not `clean` or `skipped`, or nothing is left to do (idempotence, 02 12.4 rule 1).
   c. Phase original, when `upload_state = 'pending'` and the plan is not `PlanNone`. JPEG and PNG: pass one streams `orig-1` through the filter into a byte counter and SHA-256 (no output kept); pass two streams the filter output again into `Put(orig-2, reader, size)` with the size from pass one; if pass two ends with another size or hash, the run fails with an error and River retries (the input changed under the job). The two passes avoid a spool file on the worker, as 12 3.2 and 4.2 keep the server free of intermediate copies. GIF and still WebP within the pixel limit: decode, encode in memory, `Put(orig-2, ...)`. GIF or WebP above the pixel limit, undecodable, or animated WebP: no new object. Then transaction T1 (note 10a), or the failure transaction (note 10b) when the JPEG or PNG filter fails.
   d. Phase thumbnails, when the row is `complete`, its key is revision 2 or later, the plan is not `PlanNone`, the pixel check passes and at least one variant row is missing: render the missing variants from the stored original, `Put` each to its variant key (12 4.4), then transaction T2 (note 10c). `ErrTooManyPixels`, `ErrImageUndecodable` and a profile above the `APP2` capacity end this phase without thumbnails and without a retry; only transient errors of store or database are retried.
   e. Phase container, when `domain.HasContainerMetadata(mime_type)`: `ReadMovieInfo` over the stored object, then transaction T3 (note 10d) when a value is new.
   f. Return `nil` when every phase is done; transient errors of store or database return the error so River retries. Go's image decoders take no context: the pixel limit bounds their run time, and the use case checks `ctx.Err()` between phases and before every transaction.

4. JPEG filter (`adapters/imaging/jpeg.go`), streaming, never decoding pixels:
    - The input starts with `SOI`; fill bytes before a marker are dropped; segment lengths are checked against the input. Kept markers: `SOI`, `EOI`, the frame headers `SOF0` to `SOF15` except the codes that are `DHT`, `JPG` and `DAC`, the tables `DQT`, `DHT`, `DAC`, `DRI`, `DNL`, `DHP`, `EXP` (every marker ITU-T T.81 needs to decode, Ruling for docs/DECISIONS.md), `SOS` with its entropy-coded data including byte stuffing and `RST0` to `RST7`, copied byte for byte. Dropped: every `APPn` except the three below, `COM`, `JPGn`, reserved markers.
    - `APP0` is kept only when its identifier is `JFIF` and only the first one; it is rewritten with the same version, units and densities but without its embedded thumbnail (thumbnail width and height zero, thumbnail bytes removed). `JFXX` and other `APP0` fall away (Ruling for docs/DECISIONS.md: an embedded thumbnail can show what a crop removed, the same reason EXIF thumbnails fall away in ADR 0047).
    - `APP2` is kept only with the identifier `ICC_PROFILE`, every part, in input order; `APP2` with `MPF` or any other identifier falls away. `APP14` is kept only with the identifier `Adobe`.
    - The output order is `SOI`, the rewritten `APP0` if there was one, the new `APP1` if note 5 yields one, then every other kept segment in input order. The first `EOI` ends the output; every byte after it is ignored.
    - The filter fails (`ErrImageMalformed`) on: missing `SOI`, a segment length beyond the input, a truncated entropy-coded segment, end of input before `EOI`, `SOS` before a frame header, `EOI` before any `SOS`, a second frame header outside the hierarchical mode. Width and height come from the frame header and are turned into display size with the orientation.

5. EXIF orientation (`adapters/imaging/exif.go`): the source is the first `APP1` with the identifier `Exif` followed by two zero bytes; the reader parses the TIFF header (both byte orders), IFD0 and the tag "Orientation" (type SHORT, count 1) with bounds checks on every offset; an unreadable EXIF block or an out-of-range value yields orientation 1 and does not reject the image, because the block falls away anyway (Ruling for docs/DECISIONS.md). The writer produces exactly one `APP1`: identifier, big-endian TIFF header, IFD0 with the single entry "Orientation", next-IFD offset zero, no Exif, GPS or interoperability sub-IFD. It is written only for the values 2 to 8; value 1 means no `APP1` at all (Ruling for docs/DECISIONS.md).

6. PNG filter (`adapters/imaging/png.go`): checks the signature, requires `IHDR` first, checks the CRC of every chunk, copies kept chunks unchanged (length, type, data, CRC), drops all others, stops after `IEND` and ignores trailing bytes. A CRC mismatch, a chunk length beyond the input, a missing `IHDR` or `IEND` fail with `ErrImageMalformed` (Ruling for docs/DECISIONS.md: broken structure in the sense of 12 7.1). `eXIf` is dropped like every unlisted chunk; its orientation is not applied to the thumbnails either, so the stored original and its thumbnails show the same pixels (Ruling for docs/DECISIONS.md).

7. GIF and WebP (`gif.go`, `webp.go`):
    - GIF: a block scan without LZW decoding counts the image descriptors (frames) and reads the loop count from the application extension that carries it; the pixel check uses logical screen width times height times frames. Re-encoding uses `image/gif` `DecodeAll` and `EncodeAll` with the same frames, delays, disposal methods, palettes and loop count; comments and every other application extension fall away (12 7.1).
    - WebP: a RIFF chunk scan reports animation (the animation flag of `VP8X` or an `ANIM` chunk), the `EXIF` chunk's orientation and the `ICCP` profile. A still image within the pixel limit is decoded with `golang.org/x/image/webp`, its EXIF orientation is applied to the pixels, and it is stored as PNG when it has alpha, otherwise as JPEG with `file.image.jpeg_quality`, with the `ICCP` profile embedded unchanged (`iCCP` in PNG, `APP2` in JPEG) and no orientation tag (Ruling for docs/DECISIONS.md). `mime_type` becomes `image/png` or `image/jpeg`; `original_name` keeps its extension (12 6.1: the extension decides nothing; Ruling for docs/DECISIONS.md).
    - A GIF or still WebP above `file.image.max_pixels` or failing to decode, and every animated WebP, stays unchanged as revision 1: T1 sets `complete` without a new object and without thumbnails, and the client shows a download card (12 7.2). Ruling for docs/DECISIONS.md, reported as a spec gap: 12 7.2 does not say what happens to the original of such a file.

8. Pixel guard (`guard.go`, SEC-UP-4): before any decode, read only the header (`image.DecodeConfig` for JPEG, PNG and WebP, the block scan for GIF frames) and call `domain.ExceedsPixelLimit` with `file.image.max_pixels`. Above the limit: no decode, no thumbnail. JPEG and PNG above the limit are still cleaned, because the filter never decodes pixels (ADR 0047 "Consequences").

9. Thumbnails (`render.go`, `icc.go`):
    - Decode the stored original (for animated GIF and APNG the first frame or default image, Ruling for docs/DECISIONS.md), apply the orientation of note 5 (JPEG) or none (PNG, GIF, converted WebP), then resample with `golang.org/x/image/draw` `CatmullRom` to `domain.VariantSize` (longest edge from the key, other edge rounded to the nearest pixel and at least one, never larger than the oriented original).
    - All three variants are rendered for every decodable image, also when they end up the same size, so clients always find `medium` and `large` (12 7.1 "The app shows the variant `medium` in the chat and `large` in full screen"; Ruling for docs/DECISIONS.md).
    - Format: PNG when the decoded image is not opaque (`Opaque()` of the Go image type is false), otherwise JPEG with `file.thumbnail.jpeg_quality` (12 7.3; the same rule decides the WebP conversion of note 7; Ruling for docs/DECISIONS.md).
    - Colour profile: JPEG output gets the original's profile in `APP2` `ICC_PROFILE` segments directly after `SOI`, split into the largest parts the segment length allows; PNG output gets an `iCCP` chunk after `IHDR`, copied byte for byte when the original is a PNG with `iCCP`, otherwise compressed from the unchanged profile bytes. Decompressing a PNG profile for JPEG output stops at the capacity of `APP2` profile segments (a format constant of the ICC embedding, not a tuning value); a larger profile means no thumbnails, like a decode error. When the profile's colour space (header field "data colour space") does not match the thumbnail's colour model (for example CMYK input), the thumbnail carries no profile (Ruling for docs/DECISIONS.md; RK-41 names CMYK as a fidelity risk).
    - Nothing else is written: no `APP0`, `APP1`, `APP13`, `COM`, `tEXt`, `zTXt`, `iTXt`, `eXIf` or `tIME`.

10. Transactions, events and objects (each in one unit of work; locks per 03 6.3: only the `files` row, `FOR UPDATE`):
    a. T1 (original done): lock the row; it must still be `deleted_at IS NULL`, `upload_state = 'pending'`, `scan_status` in (`clean`, `skipped`) and point to the key the run read. Then set `object_key` to `orig-2` (or keep `orig-1` in the unchanged case of note 7), `sha256`, `size_bytes`, `mime_type`, `width`, `height` and `upload_state = 'complete'`; append `file.updated`; publish `files.FileFinalized` if the merged code has it (note 16). After the commit, `Delete(orig-1)` when the key moved.
    b. Failure (JPEG or PNG filter error): lock the row with the same preconditions; set `upload_state = 'failed'` and `file_upload_sessions.failure_reason = 'image_malformed'` (Ruling for docs/DECISIONS.md: a code without content, next to `checksum_mismatch` and `size_mismatch` of 12 3.4); append `file.updated`; publish `files.FileFinalized` if present. After the commit delete `orig-1` and any `orig-2` the run wrote (12 12: failed objects are deleted at once; the row stays). No audit entry: 05 audits `file.upload` only on a malware finding.
    c. T2 (thumbnails): lock the row; it must still be `complete`, not deleted and point to the original the run decoded. Insert the new `file_thumbnails` rows (`ON CONFLICT DO NOTHING`); append one `file.updated` when at least one row was inserted.
    d. T3 (container values): lock the row, not deleted; set `width`, `height`, `duration_ms` only where the value is new; append `file.updated` when anything changed (Ruling for docs/DECISIONS.md: the event covers every change of the `File` payload).
    e. Objects: before writing an object the run checks the row; a key referenced by `files.object_key` or a `file_thumbnails` row is never written again (12 4.4). An unreferenced key left by a crashed run may be replaced. When a precondition fails, the run deletes only the keys the locked row does not reference, decided inside the transaction and deleted after it; a failed `Delete` returns the error so River retries, and a missing object counts as deleted (12 12).
    f. `file.updated`: written through the helper `files` uses since M3-04, with `scope` and `resource` as M3-04 built them (04 2.2), receive action `file.read`, payload `FilePayload` with the full `File` after the change, no JSON `null` (ADR 0053), no content.
    g. Rights and audit: the job acts as the system on a file it owns; it calls no `authorize`, writes no audit entry and raises no rights version.

11. Box reader (`adapters/isobmff`): reads box headers (32-bit size, 64-bit `largesize`, size zero meaning "to the end") through `io.ReaderAt`, descends only into `moov`, `trak`, `mdia` and `mvex`, reads `mvhd` (versions 0 and 1: timescale and duration), `tkhd` (width and height as 16.16 fixed point, the matrix), `hdlr` (handler type `vide`) and `mehd` (fragment duration); skips every other box by offset; rejects a child larger than its parent or smaller than its header. Display size is the `tkhd` size of the first video track, swapped when the matrix rotates by 90 or 270 degrees (other matrices keep the size). Duration is `mvhd` duration over timescale in milliseconds; an unknown duration (zero or all bits set) falls back to `mehd`; without either the field stays absent. Any error leaves the fields absent and never blocks the file (Ruling for docs/DECISIONS.md: these values are display hints, not checks). `files.width` and `files.height` of images and videos are display sizes after orientation or rotation (Ruling for docs/DECISIONS.md: clients lay out the original from them, and thumbnails have the same aspect).

12. `File.thumbnails` lists the rows of `file_thumbnails` in the order `small`, `medium`, `large`, `poster` with `variant`, `mime_type`, `width` and `height` (Ruling for docs/DECISIONS.md); an empty list while there are none. `getFile` and the payload of `file.updated` use the same mapping.

13. Configuration (12 18; `config.Key` of M1-07 with `Chapter` = `12-files.md`, `Default` exactly the cell of the chapter table, `Doc` one English sentence): `file.image.max_pixels` (int, positive), `file.image.jpeg_quality` and `file.thumbnail.jpeg_quality` (int, bounds equal to the quality range of Go's `jpeg.Options`), `file.image.processing_timeout` (duration, positive), `file.image.max_concurrent` (int, positive), `file.thumbnail.small_px`, `file.thumbnail.medium_px`, `file.thumbnail.large_px` (int, positive). Cross-check `thumbnail_sizes_ordered`: `small_px` below `medium_px` below `large_px`, error text naming the keys only (Ruling for docs/DECISIONS.md). Do not register `file.image.libvips_enabled` or `file.poster_max_bytes`. Then `make config-reference` and `make config-doc-check`.

14. Observability: the span `job files.thumbnail` comes from the M1-14 handler wrapper. `signals.md` has no signal for image jobs although 12 14 asks for "image jobs by result" (spec gap reported); add no metric name here. If `signals.md` names such a signal when you build, record it through the metrics port of `files` (02 5 rule 5). Logs carry only `bothy.file.id` and an error class; never the original name, EXIF or XMP values, coordinates, profile descriptions or image bytes (SEC-LOG-4).

15. No cgo: the standard build stays `CGO_ENABLED=0` (M1-23). The test `TestStandardBuildWithoutCgo` (in `adapters/imaging`) runs `go list -deps -json ./cmd/bothy` from the module root with `CGO_ENABLED=1` and fails for any non-standard package with cgo files, so a cgo image library cannot slip in through a dependency (ADR 0047 "Confirmation").

16. Neighbours:
    - M3-05 decides inline delivery. The marker for "went through 12 7.1" is the revision in `object_key` (12 4.4: revision 1 is the file as uploaded): every processed original is revision 2, also when the filter changed no byte (Ruling for docs/DECISIONS.md: one code path, and the revision tells a re-encoded GIF from an unchanged one). Expose `domain.OriginalRevision(objectKey string) int` if the key schema of M3-01 has no such function, and name it in the README.
    - M3-07 publishes `files.FileFinalized` in the job that makes a file final. If it is merged when you build, T1 and the failure transaction publish it (an image becomes final there now); if not, state in the `files` README that `files.thumbnail` is a finalizing job for the image types of note 1, so M3-07 wires it.
    - M3-08 sets `deleted_at` and purges objects by their known keys; note 10e keeps this run from leaving unreferenced objects behind.
    - M3-09 and M3-10 reuse `adapters/isobmff` and the codec.

17. Rulings for `docs/DECISIONS.md` section 10 (one row each, `D-M3-06-<n>`, with reason and place): note 1 (images are `complete` only with the cleaned original); 4 (kept JPEG markers, `APP0` JFIF without thumbnail, output order); 5 (unreadable EXIF yields orientation 1, `APP1` only for values 2 to 8, big-endian); 6 (PNG CRC and structure failures, bytes after `IEND` dropped, `eXIf` orientation not applied); 7 (WebP orientation baked in, profile kept, name unchanged; GIF and WebP above the limit, undecodable or animated stay unchanged); 9 (first frame, all three variants, alpha rule, resampler, profile only with a matching colour space); 10b (`image_malformed`); 10d (event for container values); 11 (display sizes; box errors leave fields absent); 12 (order of `thumbnails`); 13 (`thumbnail_sizes_ordered`); 16 (revision 2 for every processed original). Record in 12 the rulings that change behaviour a client or operator sees (notes 1, 4 `APP0`, 7, 9, 11 display sizes), in English, each with its `D-M3-06-<n>`.

## Tests (write them first)

Unit (`internal/files/domain`):

- `TestOriginalPlanFor`: table of detected types: `image/jpeg`, `image/png`, `image/gif`, `image/webp` give their plan; `image/svg+xml`, `image/heic`, `image/avif`, `image/tiff`, `application/pdf` give `PlanNone`.
- `TestHasContainerMetadata`: MP4 and QuickTime video and MP4 audio under the names the pinned `mimetype` reports; HEIC and AVIF (also ISO base media files) do not count.
- `TestExceedsPixelLimit`: exact boundary, frames multiply, values whose product overflows `int64` count as exceeding.
- `TestVariantSize`: landscape, portrait, square; no upscaling; rounding; at least one pixel.
- `TestOrientationDisplaySize`: values 5 to 8 swap width and height, 1 to 4 keep them.
- `TestAllowLists`: every marker and chunk of 12 7.1 is kept, every listed dropped one and an unknown one are not.

Unit (`internal/files/adapters/imaging`; test images built in code):

- `TestStripJPEGRemovesMetadata` (12 15 row and ADR 0047 "Confirmation"): input with `APP0` JFIF including an embedded thumbnail, `APP1` EXIF with a GPS IFD, an embedded EXIF thumbnail and orientation 6, `APP1` XMP, `APP13` IPTC, `COM`, `APP2` with Multi-Picture Format, a two-part `APP2` `ICC_PROFILE`, `APP14` Adobe and a second complete JPEG appended after `EOI`. Output: exactly one `APP1`, whose IFD0 has the single entry "Orientation" = 6 and no sub-IFD; `APP2` only `ICC_PROFILE`, reassembled profile byte for byte equal to the input; no other `APPn` except `APP0` (without thumbnail) and `APP14`; no `COM`; the output ends with the first `EOI`; `DQT`, `DHT`, `SOF` and every `SOS` with its scan data are byte for byte equal to the input; `image/jpeg` decodes input and output to equal pixels.
- `TestStripJPEGProgressiveWithRestarts`: progressive JPEG with several scans, `DRI` and restart markers: every scan byte for byte equal.
- `TestStripJPEGWithoutOrientationHasNoAPP1`: orientation absent or 1 gives no `APP1`.
- `TestStripJPEGMalformed`: truncated in the middle of the scan data (the 12 15 row "truncated JPEG"), segment length beyond the end, no `SOI`, `SOS` before `SOF`, `EOI` before `SOS`: each returns `ErrImageMalformed` and writes nothing usable.
- `TestEXIFOrientation`: both byte orders, tag missing, wrong type, offsets outside the block, values outside 1 to 8: orientation as defined in Design note 5; `TestWriteOrientationAPP1` reads back what it wrote.
- `TestStripPNGRemovesMetadata` (ADR 0047 "Confirmation"): PNG with `tEXt`, `iTXt`, `zTXt`, `eXIf`, `tIME`, `hIST`, `sPLT`, a private safe-to-copy chunk and trailing bytes after `IEND`: output has only allow-list chunks in input order, `IDAT` byte for byte equal, `iCCP` unchanged, `image/png` decodes both to equal pixels.
- `TestStripPNGMalformed`: CRC mismatch, `IHDR` not first, missing `IEND`, chunk length beyond the input: `ErrImageMalformed`.
- `TestGIFScanAndReencode`: frame count and loop count from the block scan; re-encoded GIF has no comment and no application extension other than the loop count; frames, delays, disposal and palette indices equal to the input.
- `TestConvertStillWebP`: with alpha gives PNG, without alpha gives JPEG with `file.image.jpeg_quality` from the test config; EXIF orientation 6 is applied to the pixels; `ICCP` profile equal in the output; no EXIF or XMP in the output. `TestAnimatedWebPDetected`: `VP8X` animation flag and `ANIM` chunk.
- `TestThumbnailsOrientationApplied` (12 15, ADR 0047): a 2:1 JPEG whose left half is one colour and right half another, orientation 6: every variant is portrait and the first colour is at the top (sampled with a tolerance for JPEG loss); no `APP1` in any variant.
- `TestThumbnailsKeepColourProfile` (12 15, ADR 0047): JPEG source: `APP2` profile of every variant byte for byte equal to the original's; PNG with alpha and `iCCP`: the variant's `iCCP` chunk equals the original's; PNG without alpha: JPEG variant with the decompressed profile equal.
- `TestThumbnailsCarryNoMetadata`: no `APP0`, `APP1`, `APP13`, `COM`, `tEXt`, `zTXt`, `iTXt`, `eXIf`, `tIME` in any variant.
- `TestThumbnailFormatByAlpha`, `TestThumbnailNoUpscale` (a tiny image gives three variants of its own size), `TestThumbnailFirstFrame` (animated GIF and APNG), `TestThumbnailCMYKWithoutProfile`.
- `TestPixelGuardDoesNotDecode` (SEC-T08 part "decompression bomb", 12 15 row "huge dimensions"): with a small `file.image.max_pixels` in the test config, a PNG whose `IHDR` and a JPEG whose frame header declare dimensions far above the limit with a tiny file size, and a GIF whose frames multiply above it: the decode function is never called (counter on an injected decoder), `ErrTooManyPixels` is returned, and the bytes allocated during the call (`runtime.MemStats.TotalAlloc` difference) stay below four bytes per pixel of the configured `file.image.max_pixels`.
- `TestStandardBuildWithoutCgo` (Design note 15).
- Fuzz targets with seed corpora from the tests above: `FuzzStripJPEG` and `FuzzStripPNG` (ADR 0047 "Confirmation": for any inserted segments or chunks the output contains only allow-list entries, at most one `APP1` with only "Orientation", nothing after `EOI` or `IEND`, or the call fails; never a panic), `FuzzEXIFOrientation`, `FuzzGIFScan`, `FuzzWebPScan`.

Unit (`internal/files/adapters/isobmff`):

- `TestReadMovieInfo`: MP4 with `mvhd` version 0 and version 1, video track with `hdlr` `vide`, an audio track before it; QuickTime with `wide` and `free` boxes; `moov` after `mdat`; a 90 degree matrix swaps the size; fragmented file with zero `mvhd` duration and `mehd`; audio-only file gives duration without size.
- `TestReadMovieInfoMalformed`: child larger than its parent, size smaller than the header, truncated `mvhd`: an error, never a panic. `FuzzReadMovieInfo`.

Unit (`internal/files/app`, fakes for store, codec, repository and unit of work):

- `TestProcessImageCommitsOriginalBeforeThumbnails`: T1 commits before any variant is written; a render error leaves the file `complete` without thumbnails.
- `TestProcessImageTwoPassMismatchRetries`: pass two with another hash returns an error and writes no row.

Unit (`internal/wire`, `internal/platform/jobs`):

- `TestFileImageConfigKeys`: every key of Design note 13 is registered with the chapter 12 default; `thumbnail_sizes_ordered` rejects equal and descending sizes; the error names keys, never values.
- `TestHandlerTimeout` (only if you add `HandlerOptions.Timeout`): a handler with a short timeout from the test config sees its context end and is retried.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, the file system driver in `t.TempDir()`, `file.scan.mode = off`, jobs through the real `platform/jobs` runtime in role `all`; uploads through the real HTTP chain with `createUpload`, `uploadPart` and `completeUpload` by Alex in space Support, channel Allgemein, with Sam as a reader of the channel):

- `TestImageJPEGEndToEnd` (plan row, 12 15 JPEG row): the JPEG of `TestStripJPEGRemovesMetadata` goes through scan and `files.thumbnail`. The stored object passes every check of the unit test; `object_key` is revision 2 and the object of revision 1 is gone; `files.sha256` and `size_bytes` equal hash and size of the stored object; `width` and `height` are the display size; `declared_sha256` of the session is unchanged; three `file_thumbnails` rows with existing objects; `getFile` as Sam returns the thumbnails in order; the variants show the orientation applied and the same colour profile.
- `TestImageNotCompleteBeforeCleaned` (Design note 1): with the job held by a test hook after the scan, the row is `upload_state = 'pending'`, `scan_status = 'skipped'` and `getFile` shows `pending`; if `downloadFile` is merged (M3-05), it answers `409 file_not_ready`.
- `TestTruncatedJPEGFails` (plan row, 12 15): `upload_state = 'failed'`, `failure_reason = 'image_malformed'`, no object of the file left under the store root (walk the directory), one `file.updated` with `failed`, no audit entry.
- `TestPNGEndToEnd`, `TestGIFReencoded`, `TestStillWebPConverted` (`mime_type` changes, `original_name` unchanged), `TestAnimatedWebPUnchanged` (revision 1, `complete`, no thumbnails).
- `TestImageOverPixelLimit` (plan row "over `file.image.max_pixels`", 12 15 row "huge dimensions"): JPEG and PNG above the limit are cleaned to revision 2 and `complete` with no thumbnails; a GIF above the limit stays revision 1, `complete`, no thumbnails; the decoder never runs.
- `TestSECT08Images` (SEC-T08, image part; EICAR, size and scanner parts belong to M3-03 and M3-04, headers to M3-05): a PNG declared as `image/jpeg` is processed as PNG; an SVG with a script gets no `files.thumbnail` job, no thumbnails and stays `image/svg+xml` revision 1; a decompression bomb is not decoded.
- `TestUnsupportedImagesUnchanged`: HEIC and TIFF are `complete` after the scan, no job, no thumbnails.
- `TestVideoContainerValues`: an MP4 with a rotated video track gets swapped `width` and `height` and `duration_ms`, with one more `file.updated`; a broken box structure leaves them absent and the file `complete`.
- `TestImageEvents`: per image, after the events the upload and the scan write, exactly one event for T1 and one for T2, in this order with rising `seq`; each validates against `FileUpdatedEvent`; no JSON `null`; the payloads contain no EXIF, XMP or GPS value.
- `TestImageJobDeletedMidway`: `deleted_at` set between the object write and T1: the row stays as it was, no event, no object of revision 2 or of a variant remains.
- `TestImageJobCrashBeforeCommit`: T1 fails once through a test hook after `orig-2` was written: the retry ends with revision 2 referenced, revision 1 deleted, no other leftover object, no duplicate event.
- `TestImageJobRunTwice`: running the job again for a done file changes nothing, writes no object and no event.
- `TestImageJobConcurrencyLimit`: with `file.image.max_concurrent` set to one in the test config and two images queued, the two decode phases never overlap (counter in a test hook).
- `TestImageJobLogsNoMetadata` (SEC-LOG-4): captured logs and spans of the whole flow contain neither the original name, nor the GPS values, nor any XMP text of the test image.
- `TestImageFileFinalized` (only if M3-07 is merged): `files.FileFinalized` is published in T1 for a cleaned image and in the failure transaction for a truncated JPEG, never by the scan for these types.

Commands that must pass, from the repository root:

```sh
cd services/server && go test -race -count=1 ./internal/files/... ./internal/platform/jobs/... ./internal/wire/... ./migrations/...
cd services/server && CGO_ENABLED=0 go build ./...
make migrations-lock
make migrations-lint
make codegen-check
make arch
make register-check
make config-reference
make config-doc-check
make licenses
make diagrams
make image-test
make check
```

Skip `make migrations-lock` only when no migration was created (table already merged). Run each fuzz target once locally with `go test -run=^$ -fuzz=<Name> -fuzztime=<duration of your choice>` and report duration and result; commit any crashing input as a seed after the fix.

## Acceptance criteria

- [ ] A JPEG with GPS EXIF, XMP, `APP2` with Multi-Picture Format, an appended image and orientation 6 gives an original with exactly one `APP1` (only "Orientation" = 6), `APP2` only `ICC_PROFILE`, scan data byte for byte equal and nothing after the first `EOI`.
- [ ] A truncated JPEG gives `upload_state = 'failed'` and no object in the store.
- [ ] Thumbnails have the orientation applied and the same colour profile as the original (12 15, ADR 0047), and no other metadata.
- [ ] An image above `file.image.max_pixels` is not decoded and gets no thumbnail; JPEG and PNG above it are still cleaned.
- [ ] SEC-T08 (image part) is green: wrong declared type, SVG with a script, decompression bomb.
- [ ] PNG keeps only the allow-list chunks with `IDAT` byte for byte equal; GIF is re-encoded without comments; still WebP is converted; animated WebP, HEIC, AVIF, TIFF and SVG stay unchanged.
- [ ] No JPEG, PNG, GIF or WebP is `complete` before its cleaned original is stored; `orig-1` is deleted after the switch to `orig-2`; no referenced object is ever written twice.
- [ ] MP4 and QuickTime files get display width, height and duration from `mvhd` and `tkhd`.
- [ ] Every state, thumbnail or dimension change writes one `file.updated` that validates against `FileUpdatedEvent`; `getFile` lists the thumbnails.
- [ ] The standard build has no cgo; `golang.org/x/image` is reachable only from the adapters of `files`.
- [ ] The configuration keys of Design note 13 are registered and documented; the cross-check rejects unordered sizes.
- [ ] 12, 02, the sequence diagram, the `files` README, CHANGELOG and `docs/DECISIONS.md` are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `files` (jobs, adapters `imaging` and `isobmff`, vendor group, the revision marker of note 16, the finalizing job, the no-spool rule of note 3c) and of `platform/jobs` (if `HandlerOptions.Timeout` was added) updated.
- Rulings of note 17 recorded in `docs/DECISIONS.md` (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`) with the M3 focus "metadata in images" and T-UP-01 (PLAN 9 "Red team"); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
