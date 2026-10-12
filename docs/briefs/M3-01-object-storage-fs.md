# M3-01: Object storage port with file system driver

Status: ready | Depends on: M1-07 | Milestone: M3

## Goal

The module `files` has its storage port `ObjectStore` with exactly the operations of 12 "4.1 Port", the key schema of 12 "4.4 Key schema" as pure functions, the file system driver of 12 "4.2 File system driver (default)" as the default driver, and one conformance suite that every driver has to pass. When the ticket is done, the file system driver passes that suite (Put, Get with a byte range, Head, Delete of missing objects, multipart with abort), a property test shows that no key carries a channel ID, a space ID or a name, every key with `..`, an absolute path or a character outside `[a-z0-9/-]` is refused before any file system call, no symbolic link is followed, a reader never sees half an object, and the configuration keys `file.storage.driver` and `file.fs.root` are in the schema with their chapter defaults. This is the first half of ADR 0010: the S3 driver of M3-02 runs the same suite, and upload sessions (M3-03), the verify pass (M3-04), downloads (M3-05) and cleanup (M3-08) reach the bytes only through this port, so an installation moves between a single host and S3-compatible storage by configuration alone.

## Read first

- `docs/spec/12-files.md`: "1. Goals and limits" (item 3 "Portable"), "3.2 Transferring parts (mode `direct`, default)" (last paragraph: on the file system parts go to `uploads/<file_id>/<n>`), "3.4 Completion and verification pass" (one read pass; on the file system the assembly of the parts belongs to that pass), "4.1 Port", "4.2 File system driver (default)", "4.3 S3 driver (core S3 only)" (only to keep the port neutral), "4.4 Key schema", "9.3 Response" (exactly one range; `ETag` form), "12. Retention, deletion, deduplication" (a missing object counts as deleted), "15. Tests" (rows "Driver conformance" and "Key schema"), "16. Decisions of this chapter" (R-FI-04, R-FI-11), "18. Configuration values of this chapter" (`file.storage.driver`, `file.fs.root`, `file.fs.min_free_bytes`).
- `docs/spec/03-data-model.md` "4.7 Files (`files`)": `files.object_key` (no names, no channel IDs, unique), `file_thumbnails.variant`, `file_upload_sessions.storage_upload_id` (an upload ID exists only for the S3 driver), `file_upload_parts` (`part_number` from 1, `etag`).
- `docs/spec/02-architecture.md`: "5. Layers within a module" (outgoing ports are interfaces of `app`, adapters implement them), "6. Code layout" (test helpers in `internal/<module>/adapters/<module>test`, imported only by `_test.go` files), "7.1 Module register as the source", "13. Configuration loading" (items 4 and 5), "14.4 Failure of a dependency" (row "object storage"), "15.2 Health".
- `docs/spec/16-operations.md`: "3. Deployment targets" (column "Files"), "4.2 Two listeners" (readiness "(from M3) the object storage answers"), "12. Status page" (`health_source` `files.storage`).
- `docs/spec/19-security.md`: SEC-UP-8 (keys are random and carry no names), SEC-UP-11 (clients get no storage URLs).
- `docs/api/openapi.yaml`: operation `uploadPart`, schemas `UploadPartResult` (`etag` is required) and `UploadComplete`, parameter `PartNumberPath` (minimum 1). Read only; this ticket enables no operation.
- ADR 0010 (decision, "Consequences", "Confirmation"), ADR 0008, ADR 0011, ADR 0050.
- `docs/DECISIONS.md`: D-B4, D-FI-04, D-FI-10, D-FI-11, D-W-100; section 10 for the format of new rows.
- `docs/security/threat-model.md`: section 5.8 and register row T-UP-01.
- `docs/operations/configuration.md`: section 3 "Check"; section 4 rows `file.storage.driver` and `file.fs.root` (generated, never edited by hand); section 9 row `BOTHY_STORAGE_DRIVER`.
- `docs/observability/signals.md` section "4.4 Push, files, search, jobs": no metric and no span names the object store; this ticket adds none.
- `docs/PLAN.md`: "3. Way of working per ticket"; "9. M3 Platform services" (Goal, Scope, Test strategy "conformance suite against the file system and SeaweedFS", Red team, Risks) with the rows M3-01 to M3-05 and M3-08; P-11.
- M1 briefs: M1-07 (configuration API: `config.Key`, `config.KindEnum`, `config.KindString`, `config.Bind`, `Schema.AddCheck`; a key enters the schema with its first consumer), M1-05 (module register, `tools/archgen`, module READMEs), M1-01 (gates `make arch` and `make codegen-check`), M1-33 "Scope" item 7 (pattern of a test helper package and the test with `go list -deps ./cmd/bothy`), M1-18 (property tests with `testing/quick`), M1-32 (atomic file writes and the start check of a server-owned directory).
- Dependencies: M1-07 is the only direct one; M1-05 and M1-01 follow from it and are therefore not in the header. Nothing of M1-11 (health), M1-12 (`pgtest`), M1-17 (`operations.yaml`) or M1-23 (image) is in the closure, and this ticket needs none of them. No M2 interface is used.
- Code on the base branch: `services/server/internal/files/doc.go` and `internal/files/README.md` (port package of M1-05), `internal/platform/config`, `internal/wire/config.go` (`configKeyGroups`), `internal/wire/configchecks.go` (`configCheck`, `configChecks`), `internal/wire/configchapters.go`, `internal/wire/configkeys_egress.go` with `internal/wire/egress.go` (the pattern to follow), `internal/wire/config_test.go` (`TestConfigSchemaBuilds`, `TestConfigKeysMatchChapterTables`, `TestDefaultsPassCrossChecks`), `internal/kernel/ids` (`ids.ID`, `ids.New`, `ids.Parse`, `ID.String`, `ID.IsZero`), `architecture/modules.yaml` (entry `files`).

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

11. The module `files` reaches its storage only through the port `ObjectStore`, which has exactly the operations `Put`, `Get` (optionally one byte range), `Head`, `Delete`, `CreateMultipart`, `UploadPart`, `CompleteMultipart`, `AbortMultipart` and `PresignUploadPart`; no listing, copying, batch deletion, versioning, bucket policies, lifecycle or encryption headers (12 "4.1 Port", ADR 0010, D-FI-11).
12. `Delete` of a missing object is not an error; a missing object counts as deleted (12 "4.1 Port", 12 "12. Retention, deletion, deduplication").
13. The file system driver is the default (`file.storage.driver`); its root `file.fs.root` is a directory used only by the server; directories have mode `0700`, files `0600` (12 "4.2", 12 "18").
14. Paths arise only from the key schema; the driver refuses keys with `..`, absolute paths or characters outside `[a-z0-9/-]` and follows no symbolic link (12 "4.2").
15. A write goes to a temporary file on the same file system, then `fsync`, `rename` and `fsync` of the directory; a reader never sees half an object (12 "4.2").
16. `Get` with a range reads through `ReadAt`; the server keeps no cache of objects (12 "4.2").
17. On the file system the parts of an upload are stored at `uploads/<file_id>/<n>` below `file.fs.root` (12 "3.2", 12 "4.4").
18. Keys follow 12 "4.4": `f/<aa>/<bb>/<file_id>/orig-<n>` (revision n from 1) and `f/<aa>/<bb>/<file_id>/small`, `.../medium`, `.../large`, `.../poster`, where `<aa>` and `<bb>` are the last four hex characters of the file ID; a key holds no names, no channel or space IDs and no file name (03 `files.object_key`, SEC-UP-8). Objects are never overwritten by the module: a changed original gets the next revision (12 "4.4").
19. One conformance suite runs against every driver and covers Put, Get with range, Head, Delete of missing objects and multipart with abort (12 "15. Tests", ADR 0010 "Confirmation"); a property test shows that no key contains a channel ID, a space ID or a name (12 "15. Tests" row "Key schema").
20. A key enters the configuration schema with its first consumer, typed through a struct that `internal/wire` fills; problem messages name keys, never values (02 "13. Configuration loading" items 4 and 5).
21. Encryption at rest is a task of operations, not of the driver (12 "4.2", 16).
22. A failing object store must not affect the rest of the server: the port returns errors and never panics, so callers can answer `503` for uploads and downloads while messages without files continue (02 "14.4").

## Scope

All paths under `services/server/` unless noted.

Create:

- `internal/files/app/objectstore.go`: the outgoing port `ObjectStore`, the value types `ByteRange`, `ObjectInfo`, `Part`, `PresignedPart` and the errors of Design note 3; `internal/files/app/doc.go`.
- `internal/files/domain/objectkey.go`: type `Variant` with its four constants, `OriginalKey`, `VariantKey`, `UploadPartKey`, `ParseOriginalKey`, `ValidateKey`, `ErrInvalidKey` (Design note 4); `internal/files/domain/doc.go`.
- `internal/files/adapters/fs/`: `doc.go`, `store.go` (`Config`, `Open`, `Store`), `write.go`, `read.go`, `multipart.go`, a Unix-only file for the no-follow flag, and tests.
- `internal/files/adapters/filestest/objectstore.go`: `Options` and `RunObjectStoreConformance` (Design note 14), with `doc.go`.
- `internal/wire/configkeys_files.go`: `filesConfigKeys()`.
- `internal/wire/files.go`: `filesStorageValues`, `FilesObjectStore`, the cross-check function `fileFSRootAbsolute`.
- Tests as listed under "Tests".

Change:

- `internal/wire/config.go` (one line `filesConfigKeys` in `configKeyGroups`), `internal/wire/configchecks.go` (row `file_fs_root_absolute`), `internal/wire/configchapters.go` (`chapter12 = "12-files.md"`, unless a parallel ticket added it), `internal/wire/config_test.go` (expected keys of `TestConfigSchemaBuilds`).
- `internal/files/README.md` (English, like the other module READMEs).
- `docs/spec/12-files.md` sections "4.1 Port" and "4.2 File system driver (default)" (English; Design note 16); `docs/spec/02-architecture.md` "13. Configuration loading" item 4 (the new cross-check); ADR 0010 "Confirmation" (names of the suite and the property test).
- `docs/DECISIONS.md` section 10: rows `D-M3-01-1` to `D-M3-01-13` (Design note 16).
- `services/server/CHANGELOG.md` (`Added`), root `CHANGELOG.md` (`Changed`: chapters 12 and 02).
- Only if the base branch has the schema export of M1-08 (`services/server/tools/configschema`): `architecture/config-schema.json` and section 4 of `docs/operations/configuration.md`, both regenerated, never edited by hand.

Not touched: migrations, `api/operations.yaml` (no operation is enabled), events, jobs, metrics, `architecture/modules.yaml` (no new package outside the existing components of `files`, no vendor), `go.mod`, `services/server/Dockerfile`, `infra/compose/`. This ticket creates no Makefile target.

## Out of scope

- S3 driver `internal/files/adapters/s3`, the keys `file.s3.*` (including `file.s3.prefix` and the credentials through the secret provider), presigned part uploads, the conformance run against SeaweedFS in a throwaway container, the architecture rule that only `adapters/s3` imports the S3 SDK, and the absence of `DeleteObjects`: M3-02.
- Upload sessions, parts and completion with the migrations `files`, `file_upload_sessions` and `file_upload_parts`, the operations `createUpload`, `uploadPart`, `completeUpload`, `abortUpload` and `getFile`, quota, the free-space check with `file.fs.min_free_bytes` (`503 unavailable`, `reason` `storage_full`), and opening the store in the server roles: M3-03.
- The single read pass with SHA-256, size, type detection and scan, including how the file system parts are assembled in that pass: M3-04 (this ticket only guarantees what Design note 10 says).
- Downloads with `Range` and the headers of 12 "9.3": M3-05. Revisions `orig-2` and thumbnail variants: M3-06. Deleting the objects of deleted or abandoned files and the cleanup jobs: M3-08.
- The readiness check "object storage answers" of 16 "4.2": no ticket owns it yet, and it needs the health registry of M1-11, which is outside this closure (reported as a plan gap; M3-03 proposed).
- The directory `/var/lib/bothy/files` in the server image, owned by the runtime user with mode `0700`: no ticket owns it yet (reported as a plan gap; M3-03 proposed, as M1-32 did for the signing keys).
- The status page source `files.storage` with its probe object: M7-09. Object backup and restore test: M7-02 and M7-03.
- A sweep of temporary files that a crashed process left in `<root>/.tmp`: the specification has no rule yet (reported as a spec gap; M3-08 proposed).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M3-01-object-storage-fs`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(files): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests open no network listener and need no container runtime.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d` or `t.TempDir()`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules. Run Go with `GOTOOLCHAIN=local`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- File system tests work only inside directories from `t.TempDir()`; never touch `/var/lib/bothy` or any path outside the test's own temporary directories. Tests with symbolic links point them only at canary files in another `t.TempDir()`.
- No S3 SDK, no S3 service and no container in this ticket; the suite runs against the file system driver only (M3-02 adds the S3 run).
- Standard library only: no new Go module; `go.mod`, `go.sum` and the vendor groups of `architecture/modules.yaml` stay unchanged.
- Do not open the store in `bothy serve` or in any role, and do not change `services/server/Dockerfile` or `infra/compose/`: the image has no writable `/var/lib/bothy/files` yet, so opening the store at start would stop the stack (M3-03 does both together).
- Add no metric, span, log attribute or log line: `docs/observability/signals.md` names none for the object store.

## Design notes

1. Placement. Ruling for docs/DECISIONS.md: `ObjectStore` is an outgoing port of the application layer, so it lives in `internal/files/app` with its value types and errors (02 "5. Layers within a module": the application layer holds outgoing ports such as repositories and foreign services as interfaces; adapters implement them). The root package `internal/files` stays the incoming port for other modules (02 "4.3"); no other module ever sees `ObjectStore`. Key schema and key grammar are pure functions in `internal/files/domain`. The conformance suite is the test helper package `internal/files/adapters/filestest` (02 "6. Code layout"); only `_test.go` files import it, and a test holds it out of the binary.

2. Port, in `internal/files/app` (the order and arguments follow 12 "4.1"; every call takes a context first):

    ```go
    type ObjectStore interface {
        Put(ctx context.Context, key string, body io.Reader, size int64) error
        Get(ctx context.Context, key string, rng *ByteRange) (io.ReadCloser, error) // rng nil: whole object
        Head(ctx context.Context, key string) (ObjectInfo, error)
        Delete(ctx context.Context, key string) error
        CreateMultipart(ctx context.Context, key string) (uploadID string, err error)
        UploadPart(ctx context.Context, key, uploadID string, n int, body io.Reader, size int64) (Part, error)
        CompleteMultipart(ctx context.Context, key, uploadID string, parts []Part) error
        AbortMultipart(ctx context.Context, key, uploadID string) error
        PresignUploadPart(ctx context.Context, key, uploadID string, n int, ttl time.Duration) (PresignedPart, error)
    }

    type ByteRange struct{ Offset, Length int64 } // Offset >= 0, Length >= 1
    type ObjectInfo struct{ Size int64 }
    type Part struct {
        Number int    // from 1, as file_upload_parts.part_number
        ETag   string // exactly as UploadPart returned it, quotes included
    }
    type PresignedPart struct {
        URL       string
        ExpiresAt time.Time
    }
    ```

    Implementations are safe for concurrent use. `Get` streams; the caller closes the reader. Nothing in the port reads a whole object into memory.

3. Error contract. Ruling for docs/DECISIONS.md:
    - `ErrInvalidKey` (the same value as `domain.ErrInvalidKey`): the key is outside the grammar of note 4. Every operation checks the key first and returns this error before any storage call.
    - `ErrNotFound`: `Get` or `Head` of a missing object. On the file system a directory or any other non-regular entry at the path counts as missing.
    - `ErrInvalidRange`: `Offset < 0`, `Length < 1`, or `Offset` at or beyond the object size. A range whose end lies beyond the object ends at the last byte, as one HTTP range does (12 "9.3": exactly one range is supported).
    - `ErrSizeMismatch`: `size < 0`, or the body of `Put` or `UploadPart` yields fewer or more bytes than `size`. Nothing is stored.
    - `ErrUploadNotFound`: `UploadPart` or `CompleteMultipart` for an upload that was never created, was aborted or was completed.
    - `ErrInvalidPart`: `n < 1` in `UploadPart` or `PresignUploadPart`; in `CompleteMultipart` an empty list, numbers that are not strictly ascending, a number without an uploaded part, or an ETag that does not match. The upload stays open; a later correct call succeeds. Drivers check the list shape before they call any service.
    - `ErrNotSupported`: an operation the driver does not offer (`PresignUploadPart` on the file system).
    - `Delete` of a missing object and `AbortMultipart` of an unknown, aborted or completed upload return nil (12 "4.1"; 12 "12": "a missing object counts as deleted").
    - Every other error is a storage failure that callers map to `503 unavailable` (02 "14.4"). Errors wrap their cause with `%w` and name the operation and the key; they never contain object bytes.

4. Key schema in `internal/files/domain`:

    ```go
    type Variant string // VariantSmall "small", VariantMedium "medium", VariantLarge "large", VariantPoster "poster"

    func OriginalKey(fileID ids.ID, revision int) (string, error) // f/<aa>/<bb>/<file_id>/orig-<revision>, revision >= 1
    func VariantKey(fileID ids.ID, v Variant) (string, error)      // f/<aa>/<bb>/<file_id>/<variant>, only the four variants
    func UploadPartKey(fileID ids.ID, n int) (string, error)       // uploads/<file_id>/<n>, n >= 1, file system driver only
    func ParseOriginalKey(key string) (fileID ids.ID, revision int, err error)
    func ValidateKey(key string) error                             // nil or wraps ErrInvalidKey
    ```

    - `<file_id>` is the canonical lower-case form of `ID.String`; `<aa>` and `<bb>` are the last four hex characters of it, `<aa>` the first two of them and `<bb>` the last two. Example: the file ID `0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b` gives `f/5a/6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/orig-1`. Revisions and part numbers are decimal without leading zeros. The nil ID, a revision or part number below 1 and an unknown variant are errors.
    - The builders take only `ids.ID`, `int` and `Variant`, so no text (file name, channel name, person) can ever reach a key; a test holds this (Tests).
    - `ParseOriginalKey` accepts exactly the form of `OriginalKey` and checks that `<aa>/<bb>` match the file ID.
    - Ruling for docs/DECISIONS.md: `ValidateKey` is one grammar for all drivers: one or more segments of `[a-z0-9-]`, joined by single `/`, without a leading or trailing `/`. It refuses therefore `.`, `..`, absolute paths, empty segments, backslashes, upper case, percent signs, spaces, control characters and non-ASCII characters, and `filepath.Clean(key) == key` holds for every accepted key. Both drivers refuse the same keys, so a key that works on one works on the other (ADR 0010: "the key scheme is the same in both drivers").

5. Opening the file system driver: `fs.Open(cfg fs.Config) (*fs.Store, error)` with `fs.Config{Root string}`. Ruling for docs/DECISIONS.md (the same start check as the key store of M1-32): the root is an absolute, clean path; it exists; `Lstat` shows a directory and not a symbolic link; it is owned by the effective user of the process; its permission bits are exactly `0700`. `Open` creates `<root>/.tmp` with mode `0700` when missing, which also proves that the root is writable. Any failure returns an error that names `file.fs.root` and the expected state ("a directory owned by the server user with mode 0700"), not the path. `Open` never creates the root itself: a missing root means a wrong mount. Make the expected owner injectable for tests (an unexported variable set in `export_test.go`), because a test cannot create a directory owned by another user.

6. No escape, no links. Ruling for docs/DECISIONS.md: the store keeps an `os.Root` of the root directory (Go standard library; operations through it cannot leave the directory, also not through links), and in addition it follows no symbolic link at all, also not inside the root: before an operation it checks each existing component of the path with `Lstat` through the `os.Root` and fails on a link, and the final open uses `O_NOFOLLOW` (Unix-only file with a build constraint; the server image is Unix). A link found is a storage failure, not `ErrNotFound`, so callers answer `503` and the operator sees the cause. Keys map to paths below the root with `filepath.FromSlash`.

7. Writing (`Put`, and each part of note 10):
   a. Validate the key (note 4) and `size >= 0`.
   b. Create a temporary file with mode `0600` in `<root>/.tmp/`. Ruling for docs/DECISIONS.md: `.tmp` starts with a dot, which no key may contain, so no key ever addresses a temporary file.
   c. Copy exactly `size` bytes from the body with a bounded buffer, checking `ctx` between chunks; then read once more to detect a longer body. Short or long body: `ErrSizeMismatch`. Cancelled context: return the context error.
   d. `Sync` the temporary file; create missing directories of the target with mode `0700`; rename the temporary file to the target; `Sync` the target directory. If the platform refuses to sync a directory with "not supported", treat only that error as success, and say so in the README and the report.
   e. On every error path remove the temporary file. If the target directory vanished between creating it and the rename (a concurrent `Delete` removed it, note 9), create it again and retry the rename once.
   f. `Put` on an existing key replaces it atomically. Ruling for docs/DECISIONS.md: the drivers do not enforce "Objects are never overwritten"; the module does, through the key schema (a changed original gets the next revision) and the unique `files.object_key`, and a retried job may rewrite a key that no committed row names yet.

8. Reading: `Get` validates key and range, opens the file (regular files only, else `ErrNotFound`), reads the size with `Stat`, checks the range (note 3) and returns a reader over `io.NewSectionReader` (which reads with `ReadAt`); `Close` closes the file, also on every error path. `Head` returns `ObjectInfo{Size}`. No cache, no read-ahead into memory.

9. Deleting. Ruling for docs/DECISIONS.md: `Delete` removes the file (a link at the final component is refused, note 6), syncs the directory, then removes the directory `f/<aa>/<bb>/<file_id>` (or `uploads/<file_id>`) if it is empty now; "not empty" and "already gone" are ignored. The fan-out levels `f/<aa>` and `f/<aa>/<bb>` stay. A missing object gives nil. Reason: without this, every deleted file would leave a directory behind, and the server never lists to find them (12 "4.1").

10. Multipart on the file system. Ruling for docs/DECISIONS.md:
    - `CreateMultipart(key)` accepts only original keys (`ParseOriginalKey` succeeds), else `ErrInvalidKey`, because clients upload only originals in parts (12 "3.1", "3.2") and the part directory is named after the file ID. It creates `uploads/<file_id>/` with mode `0700` if missing, is idempotent, and returns the upload ID `""`: 03 `file_upload_sessions.storage_upload_id` holds an upload ID only for the S3 driver, and the place of the parts follows from the key (12 "3.2").
    - `UploadPart(key, uploadID, n, body, size)`: `uploadID` must be `""` and `uploads/<file_id>/` must exist, else `ErrUploadNotFound`; `n >= 1`, else `ErrInvalidPart`. It writes `uploads/<file_id>/<n>` as in note 7 (a part with the same number is replaced atomically, as S3 does; conflicts are decided by M3-03 through `file_upload_parts`) and returns `Part{Number: n, ETag: <the lower-case hex SHA-256 of the part bytes in double quotes>}`, computed while writing. Reason: `UploadPartResult.etag` in openapi is required for both drivers, and the form equals the `ETag` header of 12 "9.3".
    - `CompleteMultipart(key, uploadID, parts)`: the same upload checks, then the list checks of note 3; it writes the listed parts in order into one temporary file while hashing each part, and refuses with `ErrInvalidPart` (removing the temporary file and keeping the upload) when a listed part is missing or its hash does not match its ETag. Then `Sync`, rename to the key, `Sync` of the directory, and removal of `uploads/<file_id>/` with all parts, listed or not (S3 also discards unlisted parts). Each part is read exactly once (R-FI-04).
    - `AbortMultipart(key, uploadID)` removes `uploads/<file_id>/` with its parts; an unknown or already removed upload, or an `uploadID` other than `""`, gives nil.
    - While an upload is open, each part is a regular object under its key `uploads/<file_id>/<n>` (12 "4.4" lists these keys), so `Head` and `Get` work on it. M3-04 may build the single read pass of 12 "3.4" on the file system from `Get` of the parts and `Put` of the original, or from `CompleteMultipart`; that choice is M3-04's. The conformance suite never relies on parts being objects, because S3 parts are not.
    - The driver takes no lock; the upload session row of M3-03 serializes completion against late parts.

11. `PresignUploadPart` on the file system returns `ErrNotSupported` after the key check and without any file system call (12 "4.1": only S3, only mode `presigned`).

12. Concurrency and processes: `*fs.Store` is safe for concurrent use. Two stores on the same root work together (in Compose the roles `api` and `worker` share the volume `files`), because every change is a rename or an unlink. `Open` therefore never removes entries of `.tmp`: they may belong to the other process. A temporary file left by a crashed process stays (spec gap, see Out of scope).

13. Configuration and wiring:
    - `internal/wire/configkeys_files.go`: `filesConfigKeys()` returns `file.storage.driver` (`config.KindEnum`, `Enum` `fs` and `s3`, default `fs`) and `file.fs.root` (`config.KindString`, default exactly the cell of 12 "18"), each with one English sentence as `Doc` and `Chapter: chapter12`. `TestConfigKeysMatchChapterTables` then holds the defaults against 12. `file.fs.min_free_bytes` is read by the upload check of M3-03 and enters the schema there.
    - Cross-check `file_fs_root_absolute` on `file.storage.driver` and `file.fs.root`. Ruling for docs/DECISIONS.md: with the driver `fs`, `filepath.IsAbs(root)` and `filepath.Clean(root) == root` must hold, else the problem "file.fs.root must be an absolute path without . or .. elements and without a trailing slash when file.storage.driver is fs"; with `s3` nothing is checked, because the root is unused. Reason: a relative root would depend on the working directory of the process. Add the check to the list in 02 "13. Configuration loading" item 4.
    - `internal/wire/files.go` after the pattern of `EgressConfig` in `egress.go`: `filesStorageValues` with `config:"file.storage.driver"` and `config:"file.fs.root"` tags, filled by `config.Bind`, and `FilesObjectStore(v *config.Values) (filesapp.ObjectStore, error)`. The driver `fs` gives `fs.Open(fs.Config{Root: ...})`. Ruling for docs/DECISIONS.md: the driver `s3` gives the error `config: file.storage.driver: driver s3 is not supported by this server version` until M3-02 replaces this branch, as M1-07 did for the secret schemes `azkv` and `awssm`.
    - No role calls `FilesObjectStore` yet; say so in its doc comment (M3-03 opens the store in the roles together with the image directory).
    - If the base branch has `services/server/tools/configschema` (M1-08): run `go generate ./tools/configschema` in `services/server` and `make config-reference`; the rule for the prefix `file.` in `tools/configdoc/planned.yaml` stays, because the other `file.*` keys remain planned.

14. Conformance suite in `internal/files/adapters/filestest`: `RunObjectStoreConformance(t *testing.T, newStore func(t *testing.T) app.ObjectStore, opts Options)` with `Options{PartSize int64}` (size of every part but the last in the multipart cases; at least the smallest part the driver accepts: the file system test passes a small size of your choice, M3-02 passes the S3 minimum that 12 "3.1" cites). Each subtest gets a fresh, empty store from `newStore`, creates file IDs with `ids.New()` and keys only through the domain builders, draws content from a seeded generator and logs the seed. The suite asserts nothing a driver may do differently: the form of the upload ID, whether parts are objects, and what `Put` on an existing key does. The cases are listed under Tests.

15. Observability: none. The driver returns errors and logs nothing; callers log. No metric, span or attribute is added (signals.md names none for the object store).

16. Specification and rulings:
    - Write the contract of notes 2, 3, 4 (grammar) and 8 into 12 "4.1 Port", and the file system rules of notes 5, 6, 7, 9, 10 and 12 into 12 "4.2 File system driver (default)", in English like the chapter; add `file_fs_root_absolute` to 02 "13. Configuration loading" item 4; name `filestest.RunObjectStoreConformance` and `TestObjectKeysCarryNoContext` in ADR 0010 "Confirmation". Change nothing else in these documents unless the build proves it wrong.
    - Rows for `docs/DECISIONS.md` section 10, one each with date, decision, reason and place: D-M3-01-1 placement (note 1); D-M3-01-2 error contract with idempotent `Delete` and `AbortMultipart` (note 3); D-M3-01-3 one byte range, cut at the end, offset at or beyond the size refused (notes 3 and 8); D-M3-01-4 one key grammar for all drivers (note 4); D-M3-01-5 upload ID `""` and parts derived from the original key, parts readable as objects (note 10); D-M3-01-6 part ETag as quoted SHA-256 checked at completion (note 10); D-M3-01-7 temporary files in `<root>/.tmp` (note 7); D-M3-01-8 no symbolic link followed at any level (note 6); D-M3-01-9 start check of the root (note 5); D-M3-01-10 atomic replace without overwrite protection in the driver (note 7); D-M3-01-11 empty file directories removed on delete, writers recreate them (notes 7 and 9); D-M3-01-12 cross-check `file_fs_root_absolute` (note 13); D-M3-01-13 driver `s3` refused by the wiring until M3-02 (note 13).

## Tests (write them first)

The plan row names no test IDs from the chapter test tables; its acceptance tests and the rows "Driver conformance" and "Key schema" of 12 "15. Tests" are held by the named tests below. There are no integration tests against PostgreSQL in this ticket.

Unit, `internal/files/domain`:

- `TestOriginalKeyLayout`: the example of note 4 gives exactly `f/5a/6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/orig-1`; revision 2 gives `orig-2`; revision 0, a negative revision and the nil ID are refused.
- `TestVariantKeys`: `small`, `medium`, `large` and `poster` share the prefix of the original; an unknown variant is refused.
- `TestUploadPartKey`: `uploads/<file_id>/1`; part number 0 is refused.
- `TestParseOriginalKey`: round trip for random IDs and revisions; variant keys, part keys, a leading zero in the revision and a fan-out that does not match the ID are refused.
- `TestValidateKey`: accepted keys include every builder output; refused with `ErrInvalidKey`: the empty key, `..`, `../x`, `a/../b`, `/abs`, `a//b`, `a/`, `.tmp/x`, `A`, `a.b`, `a b`, `a\b`, `%2e%2e`, a key with U+202E, with NUL and with a non-ASCII letter.
- `TestObjectKeysCarryNoContext` (property test with `testing/quick`, seed logged): for random contexts of file ID, channel ID, space ID, revision, variant, part number, file name, channel name and space name (names include `../../etc/passwd`, `Report from Alex.pdf`, names with U+202E and names equal to the channel ID), every key from every builder matches the grammar of 12 "4.4" exactly, the only UUID in it is the file ID, it contains neither the channel ID nor the space ID (with or without dashes), and it contains none of the names.
- `TestKeyBuildersTakeOnlyIDsAndNumbers`: by reflection, every exported key builder takes only `ids.ID`, `int` and `Variant` parameters, so no name can flow into a key.
- `FuzzValidateKey` (seed corpus runs in every `go test`): every accepted key matches `^[a-z0-9-]+(/[a-z0-9-]+)*$`, equals `filepath.Clean` of itself, is not absolute and has no `..` segment; every input containing `..` is refused; no panic.

Unit, `internal/files/app`:

- `TestObjectStoreHasExactlyThePortOperations`: by reflection, the method set of `ObjectStore` is exactly the nine operations of 12 "4.1"; no method for listing, copying, batch deletion or versioning exists.

Adapter, `internal/files/adapters/fs` (real file system in `t.TempDir()`):

- `TestFSConformance`: runs `filestest.RunObjectStoreConformance` against `fs.Open` on a fresh root per subtest.
- `TestOpenChecksRoot`: a relative path, a missing directory, a regular file, a symbolic link to a directory, a directory with group or other permission bits and a directory of another owner (injected expected owner) are refused with an error that names `file.fs.root` and contains no path; a correct root opens and gets `.tmp` with mode `0700`.
- `TestKeyMapsToPathBelowRoot`: `Put` of an original key creates exactly `<root>/<key>`; every directory created has mode `0700`, every file `0600`; nothing else appears besides `.tmp`.
- `TestTraversalKeysTouchNothing`: a canary file next to the root; every operation with `../canary`, `f/../../canary`, `/` plus the canary's absolute path and `..` returns `ErrInvalidKey`; the canary keeps content and modification time; nothing is created outside the root. Run once more after removing the root: the result is still `ErrInvalidKey`, which proves the check comes before any file system call.
- `TestNoSymbolicLinkFollowed`: a link at `f/<aa>` to a directory outside the root, a link as the final component to a canary file outside the root, and a link inside the root to another object: `Put`, `Get`, `Head`, `Delete`, `CreateMultipart` and `UploadPart` fail with an error that is not `ErrNotFound`; the canary keeps its content; no file appears outside the root.
- `TestPutIsAtomic`: a body that blocks halfway: meanwhile `Head` and `Get` give `ErrNotFound`, the target path does not exist and `.tmp` holds exactly one entry; after release `Get` returns the full content and `.tmp` is empty.
- `TestPutSyncOrder`: through an unexported seam the store records write, sync of the file, rename and sync of the directory in this order, for `Put`, `UploadPart` and `CompleteMultipart`.
- `TestFailedWritesLeaveNothing`: a short body, a long body, a cancelled context and a body that returns an error leave no object, no part and an empty `.tmp`.
- `TestPutReplacesAtomically`: while a second `Put` of the same key runs, `Get` returns the complete old content; afterwards the complete new content; never a mix.
- `TestDeleteRemovesEmptyFileDirectory`: deleting the last object of a file removes `f/<aa>/<bb>/<file_id>`, while `f/<aa>/<bb>` stays; with another object of the file left the directory stays.
- `TestPutRecreatesVanishedDirectory`: a seam removes the target directory between its creation and the rename; `Put` succeeds.
- `TestMultipartLayout`: `CreateMultipart` returns `""`; parts lie at `uploads/<file_id>/<n>` and are readable with `Get` and `Head` under that key; each ETag is the quoted lower-case hex SHA-256 of the part; after `CompleteMultipart` and after `AbortMultipart` the directory `uploads/<file_id>` is gone.
- `TestMultipartOnlyForOriginalKeys`: `CreateMultipart` with a variant key or a part key gives `ErrInvalidKey`.
- `TestUploadPartWithForeignUploadID`: `UploadPart` and `CompleteMultipart` with a non-empty upload ID give `ErrUploadNotFound`; `AbortMultipart` with it gives nil and leaves the open upload untouched.
- `TestUploadPartReplacesSameNumber`: the same bytes again give the same ETag; other bytes replace the part atomically with a new ETag; `CompleteMultipart` with the old ETag gives `ErrInvalidPart`.
- `TestPresignNotSupported`: `ErrNotSupported`, and no entry appears below the root.
- `TestTwoStoresShareRoot`: two stores on one root (as roles `api` and `worker`) run `Put`, `Get` and `Delete` in parallel on keys of the same files without error and with correct content, under `-race`.

Conformance cases of `RunObjectStoreConformance` (each a subtest; every driver must pass them):

- `PutGetHead`: `Get` returns the bytes that `Put` wrote (compare SHA-256), `Head` returns the size.
- `GetRange`: a range in the middle, a range that ends at the last byte, a range whose length goes beyond the end (cut at the end), the whole object with a nil range.
- `InvalidRange`: negative offset, length 0, offset equal to the size: `ErrInvalidRange`.
- `MissingObject`: `Get` and `Head` of a key never written give `ErrNotFound`.
- `DeleteMissing`: `Delete` of a key never written gives nil; `Delete` twice gives nil; `Head` after `Delete` gives `ErrNotFound`.
- `PutSizeMismatch`: a shorter and a longer body give `ErrSizeMismatch`, and `Head` afterwards gives `ErrNotFound`.
- `PutCancelled`: a cancelled context gives an error and no object.
- `InvalidKeys`: every operation with `..`, `../x`, `/abs`, `A`, `a.b`, `a//b` and the empty key gives `ErrInvalidKey`.
- `MultipartComplete`: three parts, the last one smaller, uploaded in parallel and in the order 3, 1, 2; `CompleteMultipart` with the parts ascending; `Get` returns the concatenation and `Head` the sum of the sizes.
- `MultipartAbort`: two parts, then `AbortMultipart`: `Head` of the key gives `ErrNotFound`; `UploadPart` and `CompleteMultipart` give `ErrUploadNotFound`; a second `AbortMultipart` gives nil.
- `MultipartInvalidParts`: a wrong ETag, a number never uploaded, descending numbers and an empty list give `ErrInvalidPart`; afterwards a correct `CompleteMultipart` succeeds.
- `MultipartUnknownUpload`: on a key without an open upload, `UploadPart` and `CompleteMultipart` with the upload ID `unknown` give `ErrUploadNotFound`, and `AbortMultipart` gives nil.
- `KeysIndependent`: original and the four variant keys of one file are independent objects; deleting one leaves the others readable.
- `ConcurrentPuts`: parallel `Put` of different keys of the same file all succeed with correct content.

Wiring and configuration, `internal/wire`:

- `TestConfigSchemaBuilds` with the two new keys in its expected map; `TestConfigKeysMatchChapterTables` and `TestDefaultsPassCrossChecks` stay green unchanged.
- `TestFileFSRootAbsolute`: with the driver `fs`, the roots `relative/x`, `./x`, `/a/../b` and `/a/` each give exactly one problem `config: file.storage.driver, file.fs.root: file_fs_root_absolute: ...` (format as in `TestListenersDistinct`) that does not contain the configured path; an absolute clean root gives none; with the driver `s3` a relative root gives none.
- `TestFilesObjectStoreFS`: defaults plus a root from `t.TempDir()` give a working store (`Put` and `Get` round trip).
- `TestFilesObjectStoreS3NotSupported`: the driver `s3` gives an error that names `file.storage.driver` and contains no value.
- `TestFilestestNotInBinary`: `go list -deps ./cmd/bothy` lists no package `filestest` (pattern of M1-33).

Commands that must pass, from the repository root:

```sh
cd services/server && GOTOOLCHAIN=local go build ./... && GOTOOLCHAIN=local go vet ./...
cd services/server && GOTOOLCHAIN=local go test -race -count=1 ./internal/files/... ./internal/wire/...
make arch
make codegen-check
make fmt-check
make lint
make typecheck
make test
make coverage
make docs
make linkcheck
make config-reference && git diff --exit-code docs/operations/configuration.md
make check
```

Run `FuzzValidateKey` once locally with `go test -run=^$ -fuzz=FuzzValidateKey -fuzztime=<duration of your choice> ./internal/files/domain` from `services/server` and report duration and result; commit any crashing input as a seed after the fix.

## Acceptance criteria

- [ ] The conformance suite is green against the file system driver: Put, Get with range, Head, Delete of missing objects, multipart with abort, and the other cases listed.
- [ ] The property test shows that no key contains a channel ID, a space ID or a name; the builders take only IDs, numbers and the closed variant type.
- [ ] Keys with `..`, absolute paths or characters outside `[a-z0-9/-]` are refused with `ErrInvalidKey` before any file system call, and nothing outside the root is read, written or deleted.
- [ ] No symbolic link is followed at any level; the root is refused unless it is a directory of the server user with mode `0700`.
- [ ] A reader never sees half an object; writes sync the file, rename and sync the directory; failed writes leave nothing; directories have mode `0700`, files `0600`.
- [ ] `ObjectStore` has exactly the operations of 12 "4.1"; `Delete` of a missing object and `AbortMultipart` of an unknown upload succeed; the file system driver refuses `PresignUploadPart` with `ErrNotSupported`.
- [ ] `file.storage.driver` and `file.fs.root` are in the schema with the defaults of 12 "18"; the cross-check `file_fs_root_absolute` works; `FilesObjectStore` opens the file system driver and refuses `s3` without echoing values.
- [ ] `filestest` is not in the binary; `make arch` is green with no change to the register or to `go.mod`.
- [ ] 12 "4.1" and "4.2", 02 "13" item 4 and ADR 0010 "Confirmation" describe exactly the built behaviour; rows D-M3-01-1 to D-M3-01-13 are in `docs/DECISIONS.md`.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful, with an example for `fs.Open`, the key builders and `RunObjectStoreConformance`); comments explain why.
- README of `files` updated (port location and contract, key schema, file system driver with root check, links, temporary files, multipart layout and ETag, conformance suite and how a new driver runs it, configuration keys, and the open points: temporary files after a crash, readiness check, image directory).
- Rulings of note 16 recorded in `docs/DECISIONS.md` (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'; root `CHANGELOG.md` entry for the changes to chapters 12 and 02.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); the red team checks at least keys that reach outside the root, links that are followed, half objects that a reader can see, and names or channel IDs in keys (PLAN "9. M3 Platform services", Red team, T-UP-01). On `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
