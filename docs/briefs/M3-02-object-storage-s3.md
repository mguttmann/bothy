# M3-02: S3 driver against SeaweedFS

Status: ready | Depends on: M1-09, M1-22, M3-01 | Milestone: M3
## Goal

After this ticket the module `files` can keep its objects in any S3-compatible service: `internal/files/adapters/s3` is the second implementation of the object storage port `ObjectStore` (12 4.1), selected with `file.storage.driver = s3`. The driver uses only core S3 (PUT, GET with a range, HEAD, DELETE of single objects, multipart upload and presigned URLs for upload parts) through one narrow Go interface, takes its access keys only from the secret provider, sends every request through the egress client, and leaves SDK checksums to `file.s3.checksum_mode` (12 4.3, ADR 0010). The conformance suite that M3-01 runs against the file system driver runs unchanged and green against SeaweedFS in a throwaway container, an architecture rule keeps the S3 SDK inside this one adapter, and no code path can send `DeleteObjects`. Why: several API nodes and managed cloud storage need the S3 driver (ADR 0010, consequence on the file system driver), and staying with the few operations every compatible service implements keeps the product portable (B4, R-Objektspeicher); presigned part uploads are proven to bind their length against SeaweedFS (12 3.3, RK-42), so the optional `presigned` mode has its first proof.

## Read first

- Dependencies: "Depends on" is M1-09, M1-22 and M3-01; M1-07, M1-05 and M1-01 follow from them. The ticket builds on the secret provider (M1-09) and the egress client (M1-22), which the plan row lists; see "Boundaries". The keys `http.transfer_timeout` and `file.max_upload_bytes` are registered by M1-18 (register them here if the merged schema lacks them).
- `docs/PLAN.md`: section "9. M3 Platform services" (Goal, Scope, Out of scope, Exit criteria item 1, Test strategy "conformance suite against the file system and SeaweedFS", Red team T-NW-01, Risks "presigned uploads may not bind the part length"), rows M3-01, M3-02, M3-03, M3-04, M3-05, M3-06 and M3-08; section "3. Way of working per ticket"; section "5. Test strategy" (row "Test doubles": SeaweedFS container for M3-02); P-09, P-10 and P-11 in "19. Decisions of this plan".
- `docs/spec/12-files.md`: "1. Goals and limits" (item 3), "3.1 Creating an upload session" (one part plan for both drivers, the S3 part limits [Q1]), "3.2 Transferring parts (mode `direct`, default)" (the S3 driver passes each part through as a multipart part, no buffer in the server), "3.3 Presigned PUT (mode `presigned`, optional)" (length binding, mode stays off), "3.4 Completion and verification pass" (`CompleteMultipartUpload`, the own SHA-256), "4.1 Port", "4.2 File system driver (default)" (key grammar), "4.3 S3 driver (core S3 only)", "4.4 Key schema" (`file.s3.prefix`, objects are never overwritten), "12. Retention, deletion, deduplication" (a missing object counts as deleted), "14. Observability", "15. Tests" (rows "Driver conformance" and "The S3 driver depends only on a Go interface with the port operations"), "16. Decisions of this chapter" (R-FI-02, R-FI-03), "18. Configuration values of this chapter" (`file.storage.driver`, every `file.s3.*` key), "19. Sources" (Q1, Q3, Q4, Q5).
- ADR 0010 (Decision Outcome "core S3", Confirmation, More Information on `when_required`), ADR 0048 (Confirmation: only `internal/files/adapters/s3` imports the S3 SDK, and the port has no operation for signed download URLs), ADR 0011 (secret provider), ADR 0008 (depguard as the fallback for rules per file pattern), ADR 0050 (module register), ADR 0041 (pinned images and Renovate).
- `docs/spec/02-architecture.md`: "5. Layers within a module" (rule 4, foreign libraries per adapter), "6. Code layout" (test helpers in `internal/<module>/adapters/<module>test`), "7.1 Module register as the source", "7.2 go-arch-lint", "13. Configuration loading" (items 3 to 6), "14.1 Error kinds" (`dependency_unavailable`), "14.2 Deadlines" (rows "upload and download" and "outbound calls"), "14.3 Retries", "14.4 Failure of a dependency" (rows object storage and secret provider), "15.2 Health".
- `docs/spec/03-data-model.md` "4.7 Files (`files`)", table `files` (`object_key`, `size_bytes` up to `file.max_upload_bytes`). Read only.
- `docs/spec/06-api.md`: "5. Errors", "10. Files".
- `docs/spec/16-operations.md`: "4.2 Two listeners" (readiness of the object storage from M3), "5.2 Secret provider (ADR 0011)", "5.3 Secrets in the Compose stack", "8.1 What is backed up" (objects with driver `s3`), "12. Status page" (`files.storage`).
- `docs/spec/19-security.md`: "2. Transport" (SEC-TLS-4), "7. Upload hardening" (SEC-UP-8, SEC-UP-11), "8. SSRF ban and outgoing connections" (SEC-NET-1 to SEC-NET-5), "9. Logs without secrets and without content" (SEC-LOG-1, SEC-LOG-3), "10. Secrets" (row Object storage credentials, SEC-SEC-1 to SEC-SEC-3), "14. Tests" (SEC-T02, SEC-T09, SEC-T11), "15. Decisions of this chapter" (R-SE-04, R-SE-10), "16. Configuration values of this chapter" (`egress.*`).
- `docs/operations/configuration.md`: "2. Naming rule and formats" (`secret_ref`, Egress target), "3. Check", the rows of chapter 12 in section 4, "5. Secrets" (`s3_access_key`, `s3_secret_key` and the paragraph "Generators of later milestones"), "9. Compose variables" (`BOTHY_STORAGE_DRIVER`, `BOTHY_S3_ENDPOINT`, `BOTHY_S3_BUCKET`, `BOTHY_SEAWEEDFS_VERSION`).
- `infra/compose/compose.yaml` (anchor `x-server-env`, service `seaweedfs`), `infra/compose/.env.example`, `infra/compose/README.md` (profile `s3`), `infra/tofu/modules/aws/README.md` (row "Files", note 2).
- `docs/security/threat-model.md`: "2. System boundaries" (TB7), "5.8 Files, object storage, virus scanner", register rows T-NW-01 and T-UP-01.
- `docs/RISKS.md`: RK-42.
- `docs/DECISIONS.md`: D-R-OBJEKTSPEICHER, D-FI-02, D-FI-11, D-W-100 and section "10. Rulings of the build milestones".
- `docs/observability/signals.md`: the counter `bothy.egress.blocked`; the egress log of a blocked attempt carries the purpose (19 SEC-NET-5, M1-22 design note 8).
- Briefs: `M3-01-object-storage-fs.md` (written together with this brief; design notes 1 to 4 on port, errors and key grammar, 7f on replacing a key, 10 on multipart, 13 on configuration and wiring, 14 on the conformance suite), `M1-22-egress-client.md`, `M1-09-secret-provider.md`, `M1-07-config-schema-loader.md`, `M1-05-server-skeleton.md`, `M1-01-server-gates.md`, `M1-12-postgres-migrations.md` (design note D7 on `pgtest`: the pattern for throwaway containers), `M1-18-http-chain.md` (keys it registers), `M1-34-identity-sessions.md` (precedent for a depguard rule).
- Code (merged on the base branch): `services/server/internal/platform/egress` (`New`, `Config`, `Options` with `Purpose`, `Hosts`, `Redirects`, `Timeout`, `MaxResponseBytes`, `Deps`, `HostPolicy`, `RedirectNever`, `Client.HTTPClient`, `ErrBlocked`, `ErrResponseTooLarge`, its README), `internal/wire/egress.go` (`EgressConfig`), `internal/wire/config.go` (`configKeyGroups`), `internal/wire/configchecks.go` (`configChecks`), `internal/wire/configchapters.go`, `internal/wire/config_test.go` (`TestConfigSchemaBuilds`, `TestConfigKeysMatchChapterTables`, `TestDefaultsPassCrossChecks`), `internal/platform/config` (`Key`, the kinds; an empty default is allowed only for string and list keys), `internal/kernel/errs` (`Wrap`, `CodeUnavailable`, `CauseDependencyUnavailable`; `LogValue` writes the wrapped error's text into the log), `internal/platform/secrets` (M1-09), and from M3-01: `internal/files/app/objectstore.go` (`ObjectStore`, `ByteRange`, `ObjectInfo`, `Part`, `PresignedPart` and the errors `ErrInvalidKey`, `ErrNotFound`, `ErrInvalidRange`, `ErrSizeMismatch`, `ErrUploadNotFound`, `ErrInvalidPart`, `ErrNotSupported`), `internal/files/domain/objectkey.go` (`ValidateKey`, `ParseOriginalKey`), `internal/files/adapters/fs`, `internal/files/adapters/filestest` (`RunObjectStoreConformance`, `Options`), `internal/wire/configkeys_files.go` (`filesConfigKeys`) and `internal/wire/files.go` (`filesStorageValues`, `FilesObjectStore`, which still refuses driver `s3`); also `architecture/modules.yaml`, `.golangci.yml` (forbidigo with type analysis), `.semgrep/bothy-go-egress-only.yaml`.

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

11. Core S3 only: PUT, GET with a range, HEAD and DELETE of single objects, multipart upload and presigned URLs for upload parts. No listing, copying, batch delete (`DeleteObjects`), versioning, bucket policies, lifecycle or encryption headers, and no presigned GET (12 4.1, ADR 0010, ADR 0048).
12. The S3 driver depends only on a Go interface with exactly these operations; an architecture rule allows the S3 SDK only in `internal/files/adapters/s3` (12 15, ADR 0010 and ADR 0048 Confirmation).
13. AWS SDK for Go v2 with its own endpoint `file.s3.endpoint`, region `file.s3.region`, bucket `file.s3.bucket`, path style `file.s3.use_path_style` and the optional `file.s3.prefix` before every key (12 4.3, 4.4).
14. Checksum mode `file.s3.checksum_mode` for request and response, default `when_required`; the own SHA-256 of 12 3.4 replaces the SDK checksums (12 4.3, Q3).
15. Credentials only as the references `file.s3.access_key_ref` and `file.s3.secret_key_ref` through the secret provider, read only with `file.storage.driver = s3`; a rotation works without restart after `secrets.cache_ttl`; a secret that cannot be read stops the start (12 4.3, 12 18, 16 5.2, 02 14.4, 19 10).
16. Every S3 request goes through the egress client; the object storage is a target its consumer names; `http` and private addresses only with an `egress.allowed_hosts` entry with `allow_private`; TLS is always verified (19 SEC-NET-1 to SEC-NET-3, SEC-TLS-4).
17. The same conformance suite runs against the file system driver and against SeaweedFS in a throwaway container (12 15, ADR 0010 Confirmation, PLAN 9 "Test strategy").
18. Deleting a missing object is no error; parts pass through as multipart parts without a buffer in the server (12 4.1, 12 3.2, 12 12).
19. An unreachable object storage makes uploads and downloads `503`: the driver reports it as a storage failure without a port sentinel, which callers map to `503 unavailable` (M3-01 note 3); no retry of external calls in the request path (02 14.3, 14.4).
20. Keys carry no names and no channel or space IDs; the driver refuses keys outside the one key grammar of all drivers (12 4.2, `domain.ValidateKey` of M3-01) before any request (12 4.4, SEC-UP-8).
21. A presigned upload part binds the part length (12 3.3, R-FI-02, RK-42); the mode `presigned` stays off by default (`file.s3.presigned_upload`).
22. Logs, errors and telemetry carry no secret, no request URL and no presigned URL (SEC-LOG-1, SEC-LOG-3, SEC-SEC-3).

## Scope

All paths under `services/server/` unless noted. No migration, no operation to enable in `api/operations.yaml`, no event, no job, no audit entry, no new metric and no new top-level Makefile target.

Create (package `internal/files/adapters/s3`):

- `doc.go`: package comment with the rules of this brief and a construction example.
- `config.go`: `type Config struct { Endpoint, Region, Bucket, Prefix string; UsePathStyle bool; ChecksumMode string; AccessKeyRef, SecretKeyRef string }` with `config:"file.s3.<name>"` tags; `func (c Config) Validate(allowed []egress.Rule) error` (design note 9); `func (c Config) HostPolicy() egress.HostPolicy` (design note 2).
- `store.go`: `type Store struct`; `type Deps struct { Egress egress.Config; EgressDeps egress.Deps; Secrets SecretResolver; TransferTimeout time.Duration; MaxObjectBytes int64 }`; `func New(ctx context.Context, cfg Config, deps Deps) (*Store, error)`; every method of the port with the signatures of M3-01; a compile-time assertion that `*Store` implements the port.
- `api.go`: the unexported interfaces `coreAPI` and `presignAPI` (design note 3).
- `credentials.go`: `type SecretResolver interface { Resolve(ctx context.Context, raw string) (secrets.Value, secrets.Version, error) }` (implemented by `*secrets.Provider`) and the unexported credentials provider (design note 7).
- `keys.go`: `objectName(prefix, key string) (string, error)` (design note 4).
- `errors.go`: the error mapping and the error type with a clean text (design note 6).
- Tests of "Tests (write them first)", `export_test.go` (test seam that wraps the HTTP client of the driver with a recorder) and `seaweed_test.go` (container helper with the pinned image constant, design note 13).

Extend in `internal/wire`:

- `filesConfigKeys()` in `configkeys_files.go` (M3-01): the `file.s3.*` keys of design note 9.
- `configchecks.go`: the cross-check `file_s3` (design note 9).
- `files.go` (M3-01): `filesStorageValues` gains the `file.s3.*` fields, and the branch `s3` of `FilesObjectStore` builds the S3 driver instead of refusing it (design note 10).

Change:

- `internal/files/app/objectstore.go` (M3-01): `PresignUploadPart` gains the parameter `size int64` before `ttl` (design note 8); `internal/files/adapters/fs` follows the signature and keeps returning `ErrNotSupported`; their tests follow.
- `internal/wire/config_test.go`: expected keys in `TestConfigSchemaBuilds`; register `http.transfer_timeout` (chapter 02) and `file.max_upload_bytes` (chapter 03) with their chapter defaults if the merged schema lacks them. If the base branch has the schema export of M1-08 (`services/server/tools/configschema`), regenerate it with `go generate ./tools/configschema` as M3-01 does.
- `cmd/bothy`: `bothy config check --secrets` resolves `file.s3.access_key_ref` and `file.s3.secret_key_ref` only with driver `s3` (12 18), if the merged command resolves every `*_ref` key unconditionally.
- `architecture/modules.yaml`: vendor group `awssdk` and `adapter_vendors: [awssdk]` for module `files`; regenerate `.go-arch-lint.yml` with `go generate ./tools/archgen` (design note 11).
- `.golangci.yml`: the depguard rule and two forbidigo entries (design note 11).
- `go.mod`, `go.sum`: `github.com/aws/aws-sdk-go-v2`, `github.com/aws/aws-sdk-go-v2/service/s3` and `github.com/aws/smithy-go` at the newest release when you build (record the versions in the report); `github.com/testcontainers/testcontainers-go` if not yet present.
- `renovate.json` (repository root): a custom regex manager for the image constant of design note 13, unless an existing manager already matches Go image constants with a `// renovate:` comment.
- `infra/compose/compose.yaml`, service `seaweedfs`: only if design note 13 shows that SeaweedFS accepts wrong credentials with the current command.
- Docs: `docs/spec/12-files.md` (3.3 result of the length test, 4.1 part size of `PresignUploadPart`, 4.3 egress client, `allow_private` for `http` or private endpoints, required endpoint, no retries, unsigned streamed bodies, credential rotation; 18 value set of `file.s3.checksum_mode` and the two reference keys in two rows); `docs/spec/02-architecture.md` 13 item 4 (new cross-check); `docs/operations/configuration.md` section 4 regenerated; `docs/security/threat-model.md` row TB7 (object bytes to an S3 service through the egress client, keys from the secret provider); `infra/tofu/modules/aws/README.md` row "Files" (access with the keys of `file.s3.access_key_ref` and `file.s3.secret_key_ref`, not the instance role, because 12 4.3 allows only secret references and 19 SEC-NET-3 always blocks the metadata address); `internal/files/README.md`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M3-02-<n>`.

## Out of scope

- The port `ObjectStore`, the key schema, the file system driver and the cases of the conformance suite (M3-01); this ticket only changes the port where design note 8 says so.
- Opening the store in the server roles and the image directory for the file system driver (M3-03, as M3-01 decided); upload sessions, parts, completion, `file_verify`, the choice of a plain PUT for a single part, `file.upload_part_bytes` and its start checks (M3-03); scanning (M3-04); downloads with tickets (M3-05); image processing and `orig-2` (M3-06); deletion and cleanup jobs, including aborting expired multipart uploads (M3-08).
- The `presigned` mode in the upload flow: URLs in the session answer, ETags at completion, `file.s3.presigned_upload`, `file.s3.presign_ttl` and the 12 15 row "Total checksum wrong (manipulated part in presigned mode, fake S3)". No plan row owns it (reported as a plan gap); this ticket builds and proves only `PresignUploadPart`.
- The readiness check "the object storage answers" of 16 4.2: no plan row owns it and 02 15.2 names no check (reported); the status source `files.storage` (M7-09).
- The Compose profile `s3` end to end: generated `s3_access_key` and `s3_secret_key` in `secrets-init`, creating the bucket, the `egress.allowed_hosts` entry for the stack's endpoint (reported as a plan gap; installation and portability are M7-29 and M7-14).
- Backups of objects with driver `s3` (16 8.1, M7); a CA bundle for private PKI (no configuration key exists, as in M1-22).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M3-02-object-storage-s3`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(files): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; SeaweedFS runs only as a throwaway container from the pinned image on a random loopback port, started and removed by the tests (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d` (in Go tests `t.TempDir()`), removed at the end.
- Never read or print secrets; test keys are generated per run and contain `EXAMPLE`. No network calls except package downloads for pinned tools and Go modules and the pull of the pinned SeaweedFS image. Run Go with `GOTOOLCHAIN=local`. Never contact a real S3 service or a cloud account; the only S3 endpoints in tests are the throwaway container and `httptest` fakes.
- Never call the SDK's default configuration loading or credential chain (environment, shared files, instance metadata service); never build an own `http.Client`, transport or dialer, and add no `nosemgrep` for `bothy-go-egress-only`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The brief of M3-01 was written together with this one. Where the merged code of M3-01 names the port, its errors, the key check or the conformance suite differently, follow the code and list each difference in the report.
- The secret provider (M1-09) and the egress client (M1-22) are in "Depends on". If either is not merged into the base branch, stop with `Verdict: BLOCKED dependency M1-09 or M1-22 not merged`.

## Design notes

1. **Layering.** The driver is an adapter of module `files` (02 5): it implements the outgoing port `ObjectStore` of M3-01 and may import the module's own packages, `kernel`, `platform/egress`, `platform/secrets` and the vendor group `awssdk`. `internal/wire` uses only `s3.Config`, `s3.Deps` and `s3.New`; the app layer never knows which driver runs.

2. **Construction, `New`, in this order.**
   a. `cfg.Validate(deps.Egress.AllowedHosts)` with the rules of design note 9, so a direct caller cannot skip them.
   b. Resolve `AccessKeyRef` and `SecretKeyRef` once through `deps.Secrets`. An error stops the start and names the key (`file.s3.access_key_ref` or `file.s3.secret_key_ref`), never a value (02 14.4 "secret provider", fail fast; the provider's file rules give SEC-SEC-1).
   c. One egress client: `egress.New(deps.Egress, egress.Options{Purpose: "objectstore", Hosts: cfg.HostPolicy(), Redirects: egress.RedirectNever, Timeout: deps.TransferTimeout, MaxResponseBytes: deps.MaxObjectBytes}, deps.EgressDeps)`. The host policy allows exactly the request host and port: the endpoint host with path style, `<bucket>.<endpoint host>` with virtual-hosted style, the port of the endpoint or the default port of its scheme (19 SEC-NET-2: a consumer names its own targets). `TransferTimeout` is `http.transfer_timeout`: 02 14.2 lets a chapter name the limit of an outside call, and these bytes serve uploads and downloads, whose limit it is. `MaxObjectBytes` is `file.max_upload_bytes`: no stored object is larger (03 `files.size_bytes`), so a body is neither cut nor unbounded (19 SEC-NET-5). Redirects are never followed: an S3 redirect means a wrong endpoint or region. Ruling for docs/DECISIONS.md.
   d. Build `aws.Config` by hand: region, the credentials provider of design note 7, `HTTPClient: client.HTTPClient()`, retryer `aws.NopRetryer{}` (02 14.3: no retry of external calls in the request path; jobs retry through River), request checksum calculation and response checksum validation from `ChecksumMode` (design note 9), no SDK log output, no tracer or meter provider (SDK spans and metrics have no names in `signals.md`, 02 5 rule 5). Then `s3.NewFromConfig` with `BaseEndpoint` set to the endpoint and `UsePathStyle`. Never import `github.com/aws/aws-sdk-go-v2/config` or call `LoadDefaultConfig`: it reads the environment and shared files and asks the instance metadata service, whose address 19 SEC-NET-3 always blocks and whose credentials 12 4.3 does not allow. Ruling for docs/DECISIONS.md.
   e. No request at construction; readiness is out of scope.

3. **Narrow interface.** `coreAPI` has exactly `PutObject`, `GetObject`, `HeadObject`, `DeleteObject`, `CreateMultipartUpload`, `UploadPart`, `CompleteMultipartUpload` and `AbortMultipartUpload` with the SDK signatures; `presignAPI` has exactly `PresignUploadPart`. `*s3.Client` and `*s3.PresignClient` satisfy them, and `Store` holds only these interfaces, so batch delete, listing, copying, bucket calls, versioning and presigned GET are unreachable (12 4.1, ADR 0010, ADR 0048). Mapping to the port of M3-01 (note 2 there):
    - `Put` to `PutObject`; an existing key is replaced, as on the file system (M3-01 note 7f).
    - `Get(ctx, key, rng)` to `GetObject`; a non-nil `ByteRange` becomes `Range: bytes=<Offset>-<Offset+Length-1>`; S3 cuts a range at the end of the object exactly as the contract wants. `Offset < 0` and `Length < 1` give `ErrInvalidRange` before any request.
    - `Head` to `HeadObject`, returning `ObjectInfo{Size}`; `Delete` to `DeleteObject`.
    - `CreateMultipart` accepts only original keys (`domain.ParseOriginalKey`), else `ErrInvalidKey`, as the file system driver does (M3-01 note 10), and returns the S3 upload ID.
    - `UploadPart` returns `Part{Number, ETag}` with the ETag exactly as S3 sent it, quotes included; `n < 1` gives `ErrInvalidPart` before any request.
    - `CompleteMultipart` checks the list shape first (not empty, numbers strictly ascending; else `ErrInvalidPart`), then sends the list as completed parts.
    - `AbortMultipart` to `AbortMultipartUpload`; `PresignUploadPart` to the presign client (design note 8).
   No request sets server-side encryption, ACL, tagging, object lock, storage class or conditional headers (12 4.1 names encryption headers as not in the port; core S3 has no conditional write).

4. **Keys and prefix.** Every operation first runs `domain.ValidateKey` (M3-01 note 4: one grammar for all drivers, which also refuses empty segments and a leading or trailing `/`, so path-style URLs can never collapse two keys into one object); a refused key returns `ErrInvalidKey` and sends nothing. The object name is `<file.s3.prefix>/<key>` with a prefix, else the key (12 4.4); the prefix must pass `ValidateKey` as well. Ruling for docs/DECISIONS.md. The driver adds nothing else to a key, so keys keep carrying no names and no channel or space IDs (SEC-UP-8).

5. **Bodies.** `Put` and `UploadPart` stream the caller's reader with `ContentLength` set to the size and never buffer it (12 3.2). The reader is not seekable and the endpoint may be plain `http` inside a private network, where the SDK cannot hash the payload before sending; sign the payload as `UNSIGNED-PAYLOAD` with the SDK's option for it (in current releases `v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware`; check the name in the version you pin). Integrity comes from the part digest (12 3.2) and the file SHA-256 of the single read pass (12 3.4), which 12 4.3 names as the replacement of SDK checksums. A body of another length must give `ErrSizeMismatch` with nothing stored (M3-01 note 3): wrap the reader so that it holds back the last byte until the source reports the end. A longer body fails before the last byte leaves, a shorter one ends before `ContentLength` is reached; in both cases S3 never receives a complete request and stores nothing. A negative size gives `ErrSizeMismatch` before any request. Ruling for docs/DECISIONS.md.

6. **Errors.** The S3 driver keeps the error contract of M3-01 (note 3 there), so every case of the conformance suite gives the same result on both drivers:
    - `NoSuchKey`, `NotFound` or status 404 on `Get` and `Head`: `ErrNotFound`;
    - `InvalidRange` or status 416: `ErrInvalidRange`;
    - `NoSuchUpload` on `UploadPart` and `CompleteMultipart`: `ErrUploadNotFound`; on `AbortMultipart`: nil (an unknown, aborted or completed upload);
    - `InvalidPart`, `InvalidPartOrder` and `EntityTooSmall` on `CompleteMultipart`: `ErrInvalidPart`; S3 keeps the upload open, so a later correct call succeeds;
    - `DeleteObject` of a missing key succeeds at S3 and returns nil (12 4.1, 12 12);
    - everything else a call returns (`NoSuchBucket`, `AccessDenied`, `InvalidAccessKeyId`, `SignatureDoesNotMatch`, `SlowDown`, any 5xx, time-outs, connection errors, `egress.ErrBlocked`, `egress.ErrResponseTooLarge`, credential errors) is a storage failure: no port sentinel, which callers map to `503 unavailable` with the internal cause `dependency_unavailable` (M3-01 note 3, 02 14.1, 02 14.4 row object storage);
    - errors while the caller reads a `Get` body keep wrapping their cause (`egress.ErrResponseTooLarge`, a broken connection) and follow the text rule below.
   If SeaweedFS answers a case differently from the S3 behaviour this list assumes, fix the mapping in the driver, never the suite; if a case cannot pass with core S3, stop with `Verdict: BLOCKED <case>`. Read the S3 code with `errors.As` on `smithy.APIError` and the status from the SDK's response error. As M3-01 requires, errors name the operation and the key and wrap their cause with `%w`. They never contain a request URL, a query, a presigned URL, the endpoint or credentials: net/http errors carry the full request URL, and callers that wrap the error into `kernel/errs` get its text into the log through `LogValue` (SEC-LOG-3, SEC-SEC-3). Use an own error type whose text is `s3 <operation> <key>: <S3 code, HTTP status or "transport">` and whose `Unwrap` keeps `errors.Is` working for the port sentinel and for `egress.ErrBlocked`. Ruling for docs/DECISIONS.md.

7. **Credentials.** An unexported `aws.CredentialsProvider` resolves both references through `SecretResolver` on every `Retrieve` and marks the credentials as expiring, so that the SDK's credential cache asks again on the next request; the driver keeps no copy of its own. A rotated key takes effect without restart once the secret provider re-reads it after `secrets.cache_ttl` (16 5.2, 02 13 item 6). A failed re-read keeps the old value inside the provider (M1-09); any provider error becomes a storage failure. Secret bytes never reach logs, errors or telemetry (SEC-SEC-3). Ruling for docs/DECISIONS.md.

8. **Presigned upload parts.** The port operation becomes `PresignUploadPart(ctx context.Context, key, uploadID string, n int, size int64, ttl time.Duration) (PresignedPart, error)`. The S3 driver presigns `UploadPart` with `ContentLength` equal to `size` and the expiry `ttl` (callers pass `file.s3.presign_ttl`), so the signature binds the part length (12 3.3 first open point, R-FI-02, RK-42), and returns `PresignedPart{URL, ExpiresAt}` with `ExpiresAt` equal to the signing time plus `ttl`. 12 4.1 lists the operation without a size, and no driver can bind a length it does not know: add the parameter to the port, to the file system driver (which keeps returning `ErrNotSupported` after the key check) and to 12 4.1 in the same change. Ruling for docs/DECISIONS.md (chapter 12 is not core contract). If the merged port already carries the size, keep it. `n < 1` gives `ErrInvalidPart`, `size < 1` gives `ErrSizeMismatch`, a non-positive `ttl` is an error, each before signing. The URL goes to the caller only; it is never logged and never part of an error. The mode `presigned` stays off (`file.s3.presigned_upload` keeps its default); building it is out of scope.

9. **Configuration.** Register in `internal/wire`, chapter `12-files.md`, with the defaults of the chapter table "18.": `file.s3.endpoint` (string, empty), `file.s3.region` (string), `file.s3.bucket` (string, empty), `file.s3.prefix` (string, empty), `file.s3.use_path_style` (bool), `file.s3.checksum_mode` (enum `when_required`, `when_supported`), `file.s3.access_key_ref` and `file.s3.secret_key_ref` (secret_ref with the `<secrets.dir>` defaults). `file.s3.endpoint` is a string key, not a URL key, because the schema allows an empty default only for string and list keys; the cross-check parses it. `file.storage.driver` comes from M3-01 with the values `fs` and `s3`. Do not register `file.s3.presigned_upload` or `file.s3.presign_ttl`: their consumer is the `presigned` mode (02 13 item 5).
   Cross-check `file_s3` over `file.storage.driver`, the six non-secret `file.s3.*` keys and `egress.allowed_hosts`; it runs only with driver `s3`, binds `s3.Config`, parses the entries with `egress.ParseRule` and calls `Config.Validate`, so the rules live in one place; every message names keys, never values (02 13 item 4):
   a. `file.s3.endpoint` is set and is an absolute `http` or `https` URL with a host and without user info, query, fragment or a path other than `/`. The driver never uses the SDK's endpoint resolution, so the host policy names exactly one host. Ruling for docs/DECISIONS.md.
   b. `file.s3.bucket` is set and follows the S3 bucket naming rules; `file.s3.region` is set.
   c. `file.s3.prefix` is empty or follows the key grammar of design note 4.
   d. With `file.s3.use_path_style = false` the endpoint host is a DNS name, not an IP literal.
   e. An `http` endpoint needs an `egress.allowed_hosts` entry with `allow_private` for the request host and port of design note 2c, because 19 SEC-NET-2 allows `http` only there; without the entry the server would start and fail at the first request. Ruling for docs/DECISIONS.md.
   `when_required` maps to `aws.RequestChecksumCalculationWhenRequired` and `aws.ResponseChecksumValidationWhenRequired`, `when_supported` to the two `WhenSupported` values; 12 18 names the default but not the value set. Ruling for docs/DECISIONS.md. The two references are read only with driver `s3` (12 18). The chapter 12 table holds both references in one row and `TestConfigKeysMatchChapterTables` reads one key per row: split the row into two rows with the same content (`make config-reference` keeps section 4 unchanged). Add the check to the list in 02 13 item 4.

10. **Wiring.** M3-01 built `FilesObjectStore(v *config.Values)` in `internal/wire/files.go` with a branch that refuses driver `s3` until this ticket. Replace that branch: bind `s3.Config` from the loaded values (extend `filesStorageValues`), and build the store with `s3.New` from `wire.EgressConfig(v)`, the egress logger and metrics that the other egress consumers get, the process's secret provider, `http.transfer_timeout` and `file.max_upload_bytes`. The S3 branch needs a context, the secret provider and the egress dependencies, which the current signature does not carry: extend `FilesObjectStore` with them (for example a `FilesDeps` struct) and keep the `fs` branch unchanged. No role opens the store yet; M3-03 does that together with the image directory (M3-01 note 13), so its doc comment keeps saying so. A construction error is an error of `FilesObjectStore` that names the key, like any configuration or secret error.

11. **Architecture rule** (12 15, ADR 0010 and ADR 0048 Confirmation).
    - `architecture/modules.yaml`: vendor group `awssdk: [github.com/aws/aws-sdk-go-v2, github.com/aws/aws-sdk-go-v2/**, github.com/aws/smithy-go, github.com/aws/smithy-go/**]` and `adapter_vendors: [awssdk]` for module `files` (entries, not fields, 02 7.1). go-arch-lint grants vendors to the component `internal/files/adapters/**` as a whole, so it cannot narrow the SDK to one adapter.
    - `.golangci.yml`: enable `depguard` with one rule that denies `github.com/aws` in every file except `**/internal/files/adapters/s3/**`, production and test files alike (ADR 0008 names depguard for rules per file pattern; M1-34 uses it the same way).
    - `forbidigo` entries for `s3.Client.DeleteObjects` and `s3.PresignClient.PresignGetObject` in the form of the existing `exec.Cmd.Environ` entry, `pkg` `^github\.com/aws/aws-sdk-go-v2/service/s3$`, with a message that names 12 4.1 and ADR 0048.
    - Show that both rules fail once (a temporary SDK import in `internal/files/adapters/fs`, a temporary `DeleteObjects` call in the s3 package), revert, and put the lint output into the report.
    - Ruling for docs/DECISIONS.md.

12. **Conformance suite.** Run `filestest.RunObjectStoreConformance(t, newStore, filestest.Options{PartSize: ...})` of M3-01 unchanged against the S3 driver. `newStore` returns a store on a fresh, empty bucket for every subtest; `PartSize` is the smallest part size S3 accepts for every part but the last, the limit that 12 3.1 cites [Q1] (M3-01 note 14), written in the test as a constant with that source in its comment.

13. **SeaweedFS in a throwaway container** (P-09, P-10, 12 15 "testcontainers").
    - testcontainers-go as in `pgtest` (M1-12 D7): Ryuk disabled, the S3 port bound to `127.0.0.1` with a random host port, label `org.bothy.test=seaweedfs`, auto remove, one container per test binary started in `TestMain` and stopped after `m.Run`. Without a Docker-compatible runtime `TestMain` prints the reason and exits with code 1; never skip.
    - The image constant is `<repository>:<version>@sha256:<digest>` with the image repository of the Compose service `seaweedfs` and a `// renovate: datasource=docker depName=<repository>` comment; the version equals `BOTHY_SEAWEEDFS_VERSION` in `infra/compose/.env.example` and the default in `infra/compose/compose.yaml`, so the test proves the version the stack runs. Ruling for docs/DECISIONS.md.
    - Start command and credentials as in the Compose service `seaweedfs` (`weed server` with the S3 gateway, keys as `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`), with generated test keys that contain `EXAMPLE`. The driver reads the same keys from a temporary secrets directory (files at mode `0600`) through the real secret provider. If `TestS3WrongCredentialsRefused` shows that SeaweedFS accepts a wrong key with this command, switch the test and `infra/compose/compose.yaml` to SeaweedFS's identity configuration and record why: an S3 gateway without authentication in the stack is a finding of this ticket.
    - Each test creates its own bucket with a random name, so tests run in parallel.
    - Test code may use a full SDK client to create buckets and to inspect state (for example open multipart uploads after `AbortMultipart`); the driver never does. Ruling for docs/DECISIONS.md.
    - The egress configuration of the tests holds the entry `127.0.0.1:<port>;allow_private`, so the tests take the production path through the egress client.

14. **Telemetry and logs.** The driver adds no metric, no span and no log line; `bothy.egress.blocked` and the egress log with purpose `objectstore` are its only signals (19 SEC-NET-5). Callers log the port errors, whose texts design note 6 keeps clean.

15. **Rulings for docs/DECISIONS.md** (section 10, `D-M3-02-<n>`, one row each with reason): egress options and host policy (note 2c); no SDK default configuration and a required endpoint (notes 2d, 9a); no SDK retries (note 2d); `UNSIGNED-PAYLOAD` for streamed bodies (note 5); error texts without URL (note 6); credentials re-read on every SDK refresh (note 7); the prefix joined with `/` and checked with the key grammar (note 4); the held-back last byte for `ErrSizeMismatch` (note 5); `PresignUploadPart` with the part size (note 8); value set of `file.s3.checksum_mode` (note 9); `http` endpoint only with an `allow_private` entry, checked at load (note 9e); architecture rule with depguard and forbidigo (note 11); SeaweedFS test image version equal to the stack's and test-only use of the full SDK client (note 13). Record the result of `TestPresignedPartBindsLength` in 12 3.3: the length binding is proven for SeaweedFS at the pinned version; client reachability of the storage stays open.

## Tests (write them first)

The plan row names no AG, BR, AZ, TM, ID, SEC or G test. It holds two rows of 12 15: "Driver conformance" (`TestS3Conformance`) and "The S3 driver depends only on a Go interface with the port operations; architecture rule" (`TestCoreAPIMethodSet`, `TestS3SDKOnlyInAdapter`, `TestS3RequestsAreCoreS3`, the lint rules). Security tests this ticket touches: SEC-T02 (lint: no own HTTP client and no `InsecureSkipVerify` in the driver; `make semgrep` and `make lint` stay green), SEC-T09 (the driver's traffic passes the egress checks: `TestEgressBlocksUnlistedEndpoint`), SEC-T11 (an S3 secret file with mode `0644` stops the start: `TestStartFailsWithInsecureSecretFile`).

Unit (package `internal/files/adapters/s3`, no Docker; a fake S3 is an `httptest` server on `127.0.0.1:0` behind the real egress client with an `allow_private` entry, the recorder of `export_test.go` sees every request):

- `TestCoreAPIMethodSet`: by reflection `coreAPI` has exactly the eight methods of design note 3, and `*s3.Client` satisfies it.
- `TestPresignAPIMethodSet`: `presignAPI` has exactly `PresignUploadPart`; no presign method for GET, HEAD, DELETE or a whole-object PUT.
- `TestObjectName`: table with and without prefix; keys with `..`, a leading or trailing `/`, `//`, upper case, a space or non-ASCII give `ErrInvalidKey`; keys of the schema in 12 4.4 pass; a prefix outside the key grammar is refused.
- `TestChecksBeforeAnyRequest`: an invalid key, a range with a negative offset or length 0, `UploadPart` and `PresignUploadPart` with `n` 0, `CompleteMultipart` with an empty or a descending list, `CreateMultipart` with a variant key, `Put` with a negative size and `PresignUploadPart` with size 0: each returns its error of M3-01 note 3 and the fake counts no request.
- `FuzzObjectName`: for any prefix and key the result is an error or a name in the grammar that starts with the prefix and has no empty or `..` segment; the table cases are the seed corpus, which runs in every `go test`.
- `TestConfigValidate`: each rule of design note 9 a to e fails with a message that names the key; a canary value in the key never appears in the message.
- `TestChecksumModeMapping`: both values map to their SDK pairs.
- `TestHostPolicy`: path style allows only the endpoint host and port, virtual-hosted style only `<bucket>.<host>`; every other host is refused.
- `TestNoDefaultCredentialChain`: with `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `AWS_PROFILE` set through `t.Setenv` to other values, the fake sees only the access key ID from the secret provider in `Authorization`.
- `TestCredentialsFollowRotation`: the secret provider runs on a fake clock; after the key files change and the clock passes `secrets.cache_ttl`, the next request of the same `Store` signs with the new access key ID.
- `TestStartFailsWithoutSecret`: `New` without the access key file returns an error that names `file.s3.access_key_ref` and contains no value.
- `TestStartFailsWithInsecureSecretFile` (SEC-T11 path): the key file at mode `0644` makes `New` fail with the provider's insecure-file error.
- `TestNoRetries`: the fake answers 503 `SlowDown`; it receives exactly one request and the caller gets a storage failure that is none of the port sentinels (02 14.3).
- `TestRedirectNotFollowed`: the fake answers 307 with another host; exactly one request; a storage failure.
- `TestEgressBlocksUnlistedEndpoint`: an `https` endpoint on loopback without an `allow_private` entry: `errors.Is(err, egress.ErrBlocked)`, no request at the fake, `bothy.egress.blocked` counted once through a fake `egress.Metrics`.
- `TestOnlyGivenHTTPClient`: after every port operation, the requests at the fake equal the requests the recorder saw, so the SDK opens no connection of its own (SEC-NET-1).
- `TestErrorsCarryNoURLOrSecret`: unknown key, wrong credentials, a closed port, an oversized body and a failed `PresignUploadPart`: the error text names the operation and the key and contains neither `http://`, `https://`, the endpoint host and port, `X-Amz-`, the access key ID nor the secret; the same holds for the `slog` rendering of the error wrapped into `errs.Wrap(errs.CodeUnavailable, err, ...)`.
- `TestResponseLimitIsMaxObjectBytes`: with a small `MaxObjectBytes` from the test, a `Get` body of exactly that size reads completely; with one byte more the read fails with an error that wraps `egress.ErrResponseTooLarge`, never a short read without error.
- `TestPresignUploadPartSignsLength`: the URL's `X-Amz-SignedHeaders` contains `content-length` and `host`, `X-Amz-Expires` equals `ttl` in seconds, the URL contains no secret; a `ttl` of zero is refused.
- `TestS3SDKOnlyInAdapter`: `go list` over the module with imports, test imports and external test imports; every package that imports a path below `github.com/aws/` is `internal/files/adapters/s3`.
- `TestSeaweedImagePinned`: the constant has the image repository of the Compose service `seaweedfs`, a version and a `sha256` digest; the version equals `BOTHY_SEAWEEDFS_VERSION` in `infra/compose/.env.example` and the default in `infra/compose/compose.yaml`.

Unit and integration in `internal/wire`:

- `TestConfigSchemaBuilds` with the new keys, their kinds, defaults and chapter.
- `TestFileS3Check`: with driver `fs` every `file.s3.*` value is ignored; with driver `s3` each violation of design note 9 fails with a problem that names the keys; a valid `https` endpoint and a valid `http` endpoint with a matching `allow_private` entry load.
- `TestDefaultsPassCrossChecks` stays green.
- `TestWireBuildsS3Store`: driver `s3` with an `http` endpoint on an `httptest` fake at `127.0.0.1` and a matching `allow_private` entry: a `Put` through the wired store reaches the fake with the access key ID from the test secrets directory; the same wiring with an `https` endpoint on loopback without an entry is refused by the egress client and the fake counts no request. No container here: the SeaweedFS helper stays in the test files of the s3 package (design note 13).

Integration (SeaweedFS container, package `internal/files/adapters/s3`):

- `TestS3Conformance`: the suite of M3-01 (Put, Get with a range, Head, Delete of a missing object, multipart with abort and every other case it holds) against SeaweedFS, with the same results as the file system driver.
- `TestS3RequestsAreCoreS3`: during the suite the recorder sees only these requests on `/<bucket>/<object>`: PUT, GET, HEAD and DELETE without query (the SDK's `x-id=<operation>` allowed), `POST ?uploads`, `PUT ?partNumber&uploadId`, `POST ?uploadId`, `DELETE ?uploadId`. Never `POST ?delete` (`DeleteObjects`), never a request on the bucket itself, no listing query, no `x-amz-copy-source`, no `x-amz-server-side-encryption*`, and with the default mode no `x-amz-checksum-*`, `x-amz-sdk-checksum-algorithm` or `x-amz-trailer` header.
- `TestS3StreamsUnseekableBodies`: `Put` and `UploadPart` with readers that are not an `io.Seeker` succeed over plain `http`, and the requests carry `UNSIGNED-PAYLOAD`.
- `TestS3PrefixSeparatesEnvironments`: two stores on one bucket with prefixes `env-a` and `env-b`; the full client sees `env-a/<key>`; neither store reads the object of the other.
- `TestS3WrongCredentialsRefused`: a store with a wrong secret key gets a storage failure, and the full client finds nothing written.
- `TestS3UploadPartSizeMismatch`: a part body one byte shorter and one byte longer than `size` gives `ErrSizeMismatch`, and the full client lists no such part for the upload (the suite case `PutSizeMismatch` holds the same for `Put`).
- `TestPresignedPartBindsLength`: create a multipart upload and presign part 1 with a size of the test; a plain PUT of exactly that many bytes succeeds and the upload completes; a PUT with one byte more, a PUT with chunked transfer encoding and a PUT after `ttl` are refused and leave no part (checked with the full client). If SeaweedFS stores a part of another length, do not weaken the test: stop with `Verdict: BLOCKED presigned part length not bound by SeaweedFS` and report it (12 3.3 keeps the mode off).

Platform runs: the integration tests run on Linux and macOS with a Docker-compatible runtime; the pinned image digest is a multi-architecture index (amd64 and arm64).

Commands that must pass, from the repository root:

```sh
cd services/server && go test -race -count=1 ./internal/files/... ./internal/wire/... ./cmd/...
make arch
make codegen-check
make lint
make semgrep
make licenses
make osv
make config-reference && git diff --exit-code docs/operations/configuration.md
make docs
make check
```

Run `FuzzObjectName` once locally with `go test -run=^$ -fuzz=FuzzObjectName -fuzztime=<duration of your choice> ./internal/files/adapters/s3` from `services/server` and report duration and result; commit any crashing input as a seed after the fix.

## Acceptance criteria

- [ ] The conformance suite of M3-01 runs unchanged in its cases against SeaweedFS in a throwaway container from the pinned image and is green (Put, Get with a range, Head, Delete of missing objects, multipart with abort).
- [ ] The S3 SDK is imported only by `internal/files/adapters/s3`: the depguard rule is part of `make lint`, `TestS3SDKOnlyInAdapter` is green, and a temporary import elsewhere failed `make lint` (output in the report).
- [ ] No `DeleteObjects`: the driver depends only on the eight-method core interface and the one-method presign interface, forbidigo forbids `DeleteObjects` and `PresignGetObject`, and every request recorded against SeaweedFS is a core S3 request.
- [ ] Credentials come only from `file.s3.access_key_ref` and `file.s3.secret_key_ref` through the secret provider; a rotation works without restart; a missing or insecure secret file stops the start; SeaweedFS refuses a wrong key.
- [ ] Every S3 request passes the egress client with purpose `objectstore`; `http` works only with an `allow_private` entry and `bothy config check` fails without it; no retries, no redirects; checksum headers stay off with `when_required`.
- [ ] A presigned upload part binds its length against SeaweedFS, and 12 3.3 records the result.
- [ ] Driver errors name the operation and the key and contain no request URL, query, endpoint, presigned URL or credentials.
- [ ] The `file.s3.*` keys of design note 9 are in the schema, the cross-check `file_s3` holds, 12, 02 and the configuration reference are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `internal/files/README.md` updated (adapter `s3`, its keys, the egress and secret needs, the vendor rule of design note 11, the conformance suite and how to run it with Docker).
- Rulings of design note 15 recorded in `docs/DECISIONS.md` (one row each with reason).
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
