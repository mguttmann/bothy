# M3-10: Avatar and logo

Status: ready | Depends on: M3-06 | Milestone: M3

## Goal

A person sets and removes the own avatar (`setMyAvatar`, `deleteMyAvatar`), an owner with fresh step-up uploads and removes the instance logo (`uploadBrandingLogo`, `deleteBrandingLogo`), and anyone, also before sign-in, loads the logo with `getBrandingLogo`. Both images are files of the module `files` with `files.purpose` `avatar` or `branding`: they run through the same verification pass, malware scan, metadata removal and thumbnails as every attachment (M3-03, M3-04, M3-06), no byte leaves the server before the file is deliverable, and no stored original or variant carries location, device or text metadata. The read rule follows the purpose (05 2.2): every active signed-in principal reads an avatar file through `file.read`; the logo is public only through `getBrandingLogo` with `instance.discovery.read`, and no other route hands an anonymous caller any file. This is the part "avatar and logo" of the M3 scope, and it closes the branding path that M1-46 left open (`branding.logo_file_id` is written only here).

## Read first

- `docs/PLAN.md`: section 3 'Way of working per ticket'; section 9 'M3 Platform services' with Goal, Scope, Out of scope, Exit criteria (SEC-T08), Test strategy, Red team ('metadata in images'), Risks, and the rows M3-01, M3-03, M3-04, M3-05, M3-06, M3-07, M3-08 and M3-10; section 7 rows M1-26, M1-28, M1-31, M1-46; row M6-09 (later consumer of the logo operations); decisions P-04, P-09, P-10, P-11 in section 19.
- `docs/spec/12-files.md`: 1 'Goals and limits'; 2 'States of a file' (deliverable, final, `file.updated`); 3.1 'Creating an upload session' (order of checks), 3.2, 3.4 'Completion and verification pass', 3.6 'Error cases', 3.7 'Purpose of a file'; 4.1 'Port', 4.4 'Key schema'; 6.1 'Type' (two-step type check, `file_type_denied`), 6.2 'File names'; 7.1 'Rule', 7.2 'Protection against decompression bombs', 7.3 'Thumbnails'; 9.3 'Response'; 12 'Retention, deletion, deduplication'; 13 'Endpoints and actions' (last paragraph); 14 'Observability'; 15 'Tests'; 16 R-FI-05, R-FI-07; 18 for the keys.
- `docs/spec/05-actions.md` (core contract, read only): 1 'Binding rules' rules 1 and 2; 2.1, 2.2 'Target of a resource' (the four `file` rows by purpose), 2.4, 2.5 (pending principal and device), 2.6 (scope `files:read`); rows `instance.discovery.read` (4.1), `instance.branding.update` (4.2), `self.profile.update` (4.3), `file.upload` and `file.read` (4.8), `agent.update` (4.10); section 6 rows `file.updated`, `user.updated`, `instance.updated`.
- `docs/spec/03-data-model.md` (core contract, read only): 4.1 `instance_settings`; 4.2 `principals` (`avatar_file_id`, foreign key `ON DELETE SET NULL`); 4.7 `files` (`purpose` and the check `(purpose = 'attachment') = (scope_channel_id IS NOT NULL)`), `file_thumbnails`, `file_upload_sessions`, `file_upload_parts`; 4.10 `event_log` (`audience_principal_id`, `receive_action`, the check on `self`); 6.3 'Lock order'.
- `docs/spec/04-sync-protocol.md`: 2.2 'Envelope' (audience, empty scope, no JSON `null`).
- `docs/spec/06-api.md`: 2 'Basic rules' (rows 'Media types', 'Caching', 'Unknown values'); 3 (anonymous operations run through `authorize` as `anonymous`); 5 'Errors' (closed code list, `reason`, `errors[].code` examples `required`); 7 'Idempotency' (PUT and DELETE); 8 'Concurrency' points 1 to 3; 10 'Files' point 4; 16 'Endpoints' (rows `getBrandingLogo`, `setMyAvatar`, `deleteMyAvatar`, `uploadBrandingLogo`, `deleteBrandingLogo`).
- `docs/spec/07-permissions.md`: 4.2 'Flow' (anonymous principals pass only `public` actions; visibility before requirement before step-up), 4.3 'Visibility', SR5, 11 tests AZ-P1, AZ-P4, AZ-P13, AZ-M35, AZ-M36.
- `docs/spec/02-architecture.md`: 4.3 'Ports, events and sources per module' (identity declares `AvatarFiles`, files implements it), 4.4 'Allowed dependencies', 4.6 'Actions per module', 5 'Layers within a module', 7.3 'Other checks in the gate' (row 'Routes and ops': the handler lives in the module that owns the action), 8 'Transactions and Unit of Work' (8.1, 8.2, 8.5), 9.1 step 5 (body limit per route class), 12.3 'Job catalogue' (`files.scan`, `files.thumbnail`, `files.cleanup_pending`, `files.purge_objects`).
- `docs/spec/01-product.md`: 12 'Branding per instance' (the app falls back to the product mark while no logo exists).
- `docs/spec/19-security.md`: SEC-HDR-1, SEC-HDR-2, SEC-HDR-4, SEC-CORS-2, SEC-UP-1 to SEC-UP-11 (SEC-UP-5: no parser for untrusted formats in the API process), SEC-T03, SEC-T08, R-SE-06.
- `docs/spec/20-privacy.md`: row `principals` (avatar is personal data of the account class).
- `docs/api/openapi.yaml`: operations `setMyAvatar`, `deleteMyAvatar`, `uploadBrandingLogo`, `getBrandingLogo`, `deleteBrandingLogo`, `updateBranding`, `getDiscovery`, `getInstance`, `getFile`, `downloadFile`, `downloadThumbnail`; schemas `ImageUpload`, `User`, `PrincipalRef`, `Branding`, `PublicBranding`, `InstanceInfo`, `File`, `Thumbnail`, `FilePayload`, `FileUpdatedEvent`, `UserPayload`, `UserUpdatedEvent`, `InstanceUpdatedPayload`, `InstanceUpdatedEvent`, `EventEnvelope`, `FieldError`, `Problem`; responses `PayloadTooLarge`, `UnsupportedMediaType`, `NotFound`. `docs/api/asyncapi.yaml`: messages `event.file.updated`, `event.user.updated`, `event.instance.updated` (`x-receive-action`).
- ADRs 0008, 0010, 0013, 0017, 0033, 0044, 0047, 0048, 0050, 0053.
- `docs/DECISIONS.md`: D-L16, D-FI-06, D-FI-11, D-K-G2, D-W-100; section 10 for the row format.
- `docs/security/threat-model.md`: T-UP-01, T-AZ-01. `docs/RISKS.md`: RK-41. `docs/OPEN-QUESTIONS.md`: no open question applies.
- `docs/observability/signals.md`: `bothy.file.upload.size`, `bothy.file.scan` (no new metric here).
- `docs/operations/configuration.md`: section 6 (`branding.logo_file_id`, type uuid, default empty) and the keys `file.max_upload_bytes`, `file.upload_part_bytes`, `file.upload_session_ttl`, `file.upload_max_open_sessions`, `file.fs.min_free_bytes`, `file.denied_mime_types`, `file.image.max_pixels`, `file.thumbnail.medium_px`, `file.scan.mode`, `auth.step_up_max_age`.
- Briefs this ticket builds on: `docs/briefs/M1-26-identity-principals.md` (`Directory`, `PrincipalView.AvatarFileID`, the avatar foreign key left to M3-03), `M1-28-authz-core.md` (`kernel/authorize.Resource`, requirement table, visibility table, the deferred `file.read` entry), `M1-31-identity-people.md` (`UpdateMyProfile`, event `user.updated`, the answer for non-humans), `M1-46-instance-settings.md` (settings registry, `branding.logo_file_id` "written only by M3-10", branding version as sum, discovery `logo_url`), `M1-16-event-log.md` (`eventlog.Event` with `AudiencePrincipalID`), `M1-24-audit-chain.md` (`auditing.Recorder`, `DetailsV1`), `M1-14-background-jobs.md` (`jobs.Inserter.InsertTx`), `M1-18-http-chain.md` (route limits, SEC-HDR defaults, CORS for `instance.discovery.read`), `M1-35-identity-step-up.md`, `M1-12-postgres-migrations.md` (`pgtest`). The M3 briefs `M3-01-object-storage-fs.md`, `M3-03-upload-sessions.md`, `M3-04-clamd-scan.md`, `M3-06-image-metadata-thumbnails.md` and, for the neighbouring scope, `M3-05-file-download.md` (in `docs/briefs/` once committed); where they and the merged code differ, the merged code wins.
- Not in the header because they follow from M3-06: M3-04, M3-03, M3-01, M1-55, M1-46, M1-31, M1-28 and the rest of their closure. M1-63 (`authorize.ResourceFromEvent`) and M1-65 (gateway delivery) are in the closure as well; see Design note 7.
- Code: `internal/files` (M3-01 object store port, M3-03 upload sessions and `getFile`, M3-04 job `files.scan` and the `Scanner` port with its fake, M3-06 job `files.thumbnail`), `internal/identity` (M1-26, M1-31), `internal/instance` (M1-46), `internal/authz` (M1-28, and the `file.read` entry that M3-03 put in place of the deferred one), `internal/kernel/authorize`, `internal/kernel/auditing`, `internal/kernel/eventlog`, `internal/platform/httpserver`, `internal/wire`, `architecture/modules.yaml`, `api/operations.yaml`.

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

11. Every file has a purpose that decides its read path (03 `files.purpose`, 05 2.2, 12 3.7): `avatar` is readable by every active signed-in principal (`auth`); `branding` is public like `instance.discovery.read`, and the only anonymous path to its bytes is `getBrandingLogo`; `gdpr_export` is never readable through `file.read`.
12. Stored images are free of metadata, without a switch (12 7.1, ADR 0047, D-L16, SEC-UP-10, R-SE-06, 06 10 point 4): JPEG and PNG originals are filtered losslessly by the allow list, still WebP is converted, every variant is re-encoded in pure Go with the orientation applied and only the unchanged colour profile kept.
13. Avatar and logo run through the same verification pass and the same scan as every file (12 3.7, 12 3.4, 12 5, R-FI-05): nothing is delivered before the file is deliverable (12 2), and the scan fails closed.
14. Operations and rights (05 4.1 to 4.3, `openapi.yaml`): `setMyAvatar` and `deleteMyAvatar` with `self.profile.update` (self, no step-up, no audit, `rl.write`); `uploadBrandingLogo` and `deleteBrandingLogo` with `instance.branding.update` (owner, step-up, audit, `rl.admin`); `getBrandingLogo` with `instance.discovery.read` (public, `rl.public_auth`).
15. Images arrive as `multipart/form-data` with the part `file` (06 2 'Media types', `ImageUpload`); errors use the closed codes of 06 5: `400 validation_failed`, `413 payload_too_large` (detail names the key), `415 unsupported_media_type` (with `reason` `file_type_denied` for types in `file.denied_mime_types`, 12 6.1).
16. Events (05 6, 12 2, ADR 0053): `user.updated` when an avatar changes; `instance.updated` naming only `branding.logo_file_id` when the logo changes; `file.updated` for every state change of an avatar or logo file; no JSON `null`.
17. No parser for an untrusted format runs in the API process (SEC-UP-5): the request path reads bytes, counts them, hashes them and checks the declared type; type detection, filtering and decoding run in the worker jobs of M3-04 and M3-06.
18. File responses carry the headers of SEC-HDR-1, SEC-HDR-4 and 12 9.3; the public logo response additionally carries `Access-Control-Allow-Origin: *` without credentials (SEC-CORS-2).
19. No object store call inside a database transaction (02 8.5); the database knows every object key before the object exists (12 4.1, D-FI-11).
20. The handler of an operation lives in the module that owns its action (02 4.6, 7.3): `identity` for `self.profile.update`, `instance` for `instance.branding.update` and `instance.discovery.read`; both reach `files` only through declared source ports (02 4.3).
21. No read right gives `404`, the same for an unknown id (05 rule 2, ADR 0017, AZ-P4).

## Scope

All paths under `services/server/` unless noted. No new configuration key, no new metric, no new job.

Create:

- `internal/files/domain/purpose.go`: pure rules per purpose: `DeclaredImageTypeAllowed(p Purpose, mime string) bool`, `StoredOriginalAllowed(p Purpose, k OriginalKind) bool` (where `OriginalKind` is the outcome of M3-06's original step: filtered JPEG, filtered PNG, converted still WebP, re-encoded GIF, stored unchanged; reuse M3-06's type if it has one), `FixedOriginalName(p Purpose) string`, `ImageCap(maxUploadBytes, partBytes int64) int64`.
- `internal/files/app/images.go`: use cases `StageImage`, `CompleteStagedImage`, `ReleaseImage`, `OpenImageVariant` (Design notes 2 to 4, 8).
- `internal/files/adapters/profileimages/`: `avatar.go` implementing `identity.AvatarFiles`, `logo.go` implementing `instance.LogoFiles`; thin delegation to `files/app`, no logic.
- `internal/identity/app/avatar.go`: `SetMyAvatar`, `DeleteMyAvatar`; queries `internal/identity/adapters/postgres/queries/avatar.sql` (lock the principal row, set and clear `avatar_file_id`).
- `internal/instance/app/logo.go`: `UploadLogo`, `RemoveLogo`, `OpenLogo`; queries for the locked read and write of the setting row in `internal/instance/adapters/postgres/queries/`.
- Tests listed below, with their fixtures.

Change:

- `internal/identity` (port): declare the source port `AvatarFiles` and its DTO (Design note 1); add `SetMyAvatar(ctx, actor, AvatarImage) (User, error)` and `DeleteMyAvatar(ctx, actor) error` to `Directory` next to `UpdateMyProfile`.
- `internal/identity/adapters/httpapi`: strict-server methods `SetMyAvatar`, `DeleteMyAvatar`; make sure the `User` mapping writes `avatar_file_id` when set and omits it otherwise.
- `internal/instance` (port): declare the source port `LogoFiles` and its DTOs (Design note 1).
- `internal/instance/domain/settings.go`: the registry entry `branding.logo_file_id` accepts a UUIDv7 or the empty string (no logo); `Branding` and discovery treat the empty string like an absent row (Design note 9).
- `internal/instance/adapters/httpapi`: strict-server methods `UploadBrandingLogo`, `DeleteBrandingLogo`, `GetBrandingLogo`, with a response type that writes the headers of Design note 8.
- `internal/files`: the step of M3-04 or M3-06 that decides how the original is stored applies the purpose rule (Design note 4); the one place that appends `file.updated` writes the personal form for `avatar` and `branding` (Design note 7); every `file` descriptor that `files` builds carries the purpose (Design note 6).
- `internal/kernel/authorize`: `Resource.FilePurpose string` (values `attachment`, `avatar`, `branding`, `gdpr_export`) if M3-03 did not add an equivalent field; the event-row descriptor constructor of Design note 7.
- `internal/authz/domain` (`requirements.go`, `visibility.go`): `file.read` by purpose (Design note 6); `internal/authz` README.
- If the merged `files` table lacks `purpose` or its two checks from 03 4.7 (the plan row names `files.purpose` here): a new migration `internal/files/adapters/postgres/migrations/<next>_files_purpose.sql` adding them exactly as 03 4.7, with `make migrations-lock`. If they exist, no migration.
- `internal/wire`: pass the two `profileimages` implementations to `identity` and `instance`; mount the five handlers; keep the golden files current.
- `architecture/modules.yaml`: `depends_on` of `files` becomes `[identity, chat, instance]`; regenerate `.go-arch-lint.yml` (`go generate ./tools/archgen`).
- `api/operations.yaml`: enable `setMyAvatar`, `deleteMyAvatar`, `uploadBrandingLogo`, `getBrandingLogo`, `deleteBrandingLogo`; rerun the route generator (`make codegen-check`).
- `internal/platform/httpserver`: only if the merged tree has no writer for the SEC-HDR-4 file headers yet (M3-05): add one exported helper there and name it in the report so that M3-05 uses the same writer; never two writers.
- Specification (English, in the style of each chapter):
    - `docs/spec/12-files.md` 3.7: a paragraph on avatar and logo with the rules of Design notes 2 to 4, 7 and 8 (one part, allowed types, fixed `original_name`, link at upload, release of the previous file, personal `file.updated`, the variant served by `getBrandingLogo`); 12: a row "Avatar replaced or removed, logo replaced or removed" (deleted mark in the same transaction, objects through `files.purge_objects`); 13: replace the last paragraph (session endpoints under the profile and branding path) by the multipart operations of `openapi.yaml`.
    - `docs/spec/07-permissions.md` 4.3: a bullet "File with `purpose = 'avatar'` or `'branding'`: `p` is logged in (05, 2.2); `gdpr_export` never through `file.read`".
    - `docs/spec/02-architecture.md` 4.3 (row `instance`: "declares ... `LogoFiles`"; row `files`: implements `instance.LogoFiles`), 4.4 (row `files`: `identity`, `chat`, `instance`, reason "implements `AvatarFiles` and `LogoFiles`; check the upload channel"); `docs/architecture/c4-3-server-components.mmd` the edge `files --> instance` and its SVG as `docs/architecture/README.md` describes.
    - `docs/spec/06-api.md` 8 point 2: Branding means name and accent; replacing and removing the logo need no `If-Match` (Design note 10).
    - `docs/api/openapi.yaml`: descriptions only: `ImageUpload` (JPEG and PNG filtered losslessly, still WebP converted, thumbnails re-encoded, at most one upload part), `setMyAvatar` and `uploadBrandingLogo` (checks of Design note 2; the file becomes deliverable later), `getBrandingLogo` (variant `medium` of a deliverable logo, `404` otherwise). No schema, parameter or response change.
- READMEs of `files` (purpose rules, the two source ports, the new dependency on `instance`, "May use the ports of these modules:" line), `identity`, `instance`, `authz`; `services/server/CHANGELOG.md` under 'Unreleased'; `docs/DECISIONS.md` section 10 rows `D-M3-10-<n>` (Design note 11).

## Out of scope

- `downloadFile`, `downloadThumbnail`, `createFileTicket`, the ticket check and the response writer for attachments (M3-05). This ticket proves the avatar read rule through `getFile` and `authorize`, and through the download routes only if M3-05 is merged in the base branch (see Tests).
- Removing the objects of a released avatar or logo and of an abandoned staged session (`files.purge_objects`, `files.cleanup_pending`, M3-08); this ticket sets `files.deleted_at` and leaves sessions to expire.
- Attaching files to messages and the rule that only `attachment` files attach (M3-07).
- Avatars of agents through `agent.update`: `openapi.yaml` has no operation for it (spec gap reported); avatars of other people through `updateUser`: not in the schema.
- Delivery of events over the gateway and REST catch-up (M1-63, M1-65); this ticket writes the events and the receive rule.
- Display of avatars and logo in the apps and the console (M4-22, M4-25, M5, M6-09).
- Export and erasure of avatar files for a data subject request (M7, P-14).
- The S3 driver (M3-02); tests use the file system driver.
- Real clamd and EICAR (M3-04); this ticket uses the fake `Scanner` of M3-04.
- Changes to the core contract.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M3-10-avatar-logo`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(files): ...`, `feat(identity): ...`, `feat(instance): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M3-01, M3-03, M3-04 and M3-06: follow their names. If the merged M3-06 lets a JPEG or PNG become deliverable before its metadata are removed (12 7.1: "free of metadata as soon as they are stored"), stop with `Verdict: BLOCKED <reason>`.
- No real third-party service and no real clamd in tests: the object store is the file system driver of M3-01 in a `mktemp -d` folder, the scanner is the fake of M3-04. Image fixtures are synthetic, built in Go in the test or taken from M3-06's test data; no photo of a real place or person, GPS values are made up.

## Design notes

1. **Modules and ports** (02 4.3, 4.4, 4.6, 7.3). The handlers sit in the module that owns the action: `identity` for `setMyAvatar` and `deleteMyAvatar`, `instance` for the three logo operations. They reach `files` only through source ports that they declare and `files` implements, wired in `internal/wire`. `identity.AvatarFiles` is already named in 02 4.3 and in both READMEs. `instance` may use nothing (02 4.4), so it declares `LogoFiles`, and `files` gains the edge `files -> instance` to implement it. The edge points down (`instance` is the lowest layer) and makes no cycle. Ruling for docs/DECISIONS.md. Change 02 4.3, 02 4.4, the diagram, `modules.yaml` and the files README together; `make register-check` (C7, C11) and `make arch` keep them equal. Port shapes (port packages import only `kernel` and the standard library; each port has its own DTOs):

    ```go
    // package identity
    type AvatarImage struct {
        DeclaredType string    // Content-Type of the multipart part "file"
        Body         io.Reader // the part; read at most ImageCap plus one byte
    }
    type AvatarFiles interface {
        Stage(ctx context.Context, uploadedBy ids.ID, img AvatarImage) (ids.ID, error) // outside any unit of work
        Attach(ctx context.Context, fileID, uploadedBy ids.ID) error                  // inside the caller's unit of work
        Release(ctx context.Context, fileID, uploadedBy ids.ID) error                 // inside the caller's unit of work
    }

    // package instance
    type LogoImage struct { DeclaredType string; Body io.Reader }
    type LogoVariant struct {
        Body        io.ReadCloser
        ContentType string // detected type of the variant
        SizeBytes   int64
    }
    type LogoFiles interface {
        Stage(ctx context.Context, uploadedBy ids.ID, img LogoImage) (ids.ID, error)
        Attach(ctx context.Context, fileID, uploadedBy ids.ID) error
        Release(ctx context.Context, fileID ids.ID) error
        OpenVariant(ctx context.Context, fileID ids.ID) (LogoVariant, error) // errs not_found unless deliverable with variant medium
    }
    ```

2. **Staging an image** (`files/app.StageImage`, called through `Stage`, outside any transaction). The caller has already called `authorize` and got `allow`; nothing of the body is read before that (Design note 5). Checks in this order, nothing stored on any failure:
    a. The first part is named `file`; otherwise `400 validation_failed` with pointer `/file` and code `required`.
    b. The declared `Content-Type` of the part is `image/jpeg`, `image/png` or `image/webp` (`ImageUpload`); otherwise `415 unsupported_media_type` without `reason`. A declared type listed in `file.denied_mime_types` gives `415 unsupported_media_type` with `reason` `file_type_denied` (12 6.1, first step).
    c. Read the part into memory up to `ImageCap(file.max_upload_bytes, file.upload_part_bytes)` (the smaller of the two) plus one byte, counting and hashing (SHA-256). More bytes: `413 payload_too_large`, `detail` names the key that set the cap. No byte: `400 validation_failed`, pointer `/file`, code `required`.
    d. A further part after `file`: `400 validation_failed` (06 2 'Unknown values'; same pointer and code as the chain reports unknown members).
    e. The limits of 12 3.1 points 4 and 5 apply as for every upload: open sessions of the principal under `file.upload_max_open_sessions` (else `429` with `Retry-After`), free space for the file system driver at least `file.fs.min_free_bytes` (else `503 unavailable` with `reason` `storage_full`). The quota of 12 3.1 point 3 is not checked: an avatar or logo replaces the previous one, so it adds at most one live file per person or instance, and the operations document no `409`. Ruling for docs/DECISIONS.md.
    f. Rows before bytes: run M3-03's session creation without channel and without `authorize` (the caller authorized its own action, as callers of every port do): `files` row with `purpose` `avatar` or `branding`, `scope_channel_id` NULL, `uploaded_by`, `upload_state = 'pending'`, `scan_status = 'pending'`, `original_name` = `FixedOriginalName` (`avatar` or `logo`), `declared_mime_type`, `size_bytes` and `sha256` = the counted values; `file_upload_sessions` row in mode `direct` with the counted size and hash as declared values and one part. Then write the one part through M3-03's part logic (object write, part row); for one part the driver may use a single `Put` (12 3.4 point 2). If M3-03's complete use case mixes the object half (for S3 `CompleteMultipart`) with the database half, split it inside `files` so the object half runs here.
    g. Return the file id. The session stays `open`; if anything later fails, it expires after `file.upload_session_ttl` and `files.cleanup_pending` (M3-08) removes it (12 12).

    Reasons: the server must know size and SHA-256 before the first object write (03 `file_upload_sessions.declared_size_bytes` and `declared_sha256` are required, 12 4.1 the server knows every key from the database), and one part is the memory bound that every part upload already has (12 3.2). The multipart file name is ignored: an avatar or logo file is readable by every signed-in principal and the name could carry personal data (12 1 point 5). Both are rulings for docs/DECISIONS.md. No type detection, no image header read and no decoding happen here (SEC-UP-5).

3. **Linking** (one unit of work per operation; lock order: anchor row first, then `files` rows, then the counters through the phase hooks, 03 6.3; deadlocks are retried by the unit of work, `database.tx_retry_max`). The file is linked at upload, before it is deliverable; clients see `409 file_not_ready` with `Retry-After` until it is (12 9.3), the login screen sees `404` and shows the product mark (01 12). Ruling for docs/DECISIONS.md.
    - `SetMyAvatar`: `authorize(actor, self.profile.update, {kind: user, id: actor, OwnerID: actor})`; a principal that is not a human gets the same `403 forbidden` with `reason` `subject_kind_not_allowed` as `updateMyProfile` (M1-31). `Stage`. Unit of work: `SELECT avatar_file_id FROM principals WHERE id = $actor FOR UPDATE`; `Attach` (files: the session is `open`, the file is `avatar` and belongs to the actor, else internal error; session `completed` with `completed_at`; job `files.scan` through `InsertTx`); set `avatar_file_id`; `Release` of the previous file if any (files: `deleted_at = now()` only where `purpose = 'avatar'`, `uploaded_by` = actor and `deleted_at IS NULL`); append `user.updated` with `UserPayload` after the change. Answer `200` with `User`. No audit (05: Audit `no`).
    - `DeleteMyAvatar`: same `authorize`; unit of work: lock the principal row; no avatar: no write, no event, `204`; otherwise clear `avatar_file_id`, `Release`, `user.updated`, `204` (06 7: DELETE is idempotent).
    - `UploadLogo`: `authorize(actor, instance.branding.update, {kind: instance})` (owner, step-up); `Stage`. Unit of work: lock the setting row `branding.logo_file_id` (insert it with `ON CONFLICT DO NOTHING` when absent, then `SELECT ... FOR UPDATE`); `Attach`; write the new id with `version + 1`, `updated_by`, `updated_at`; `Release` of the previous logo file if any (only `purpose = 'branding'`); append `instance.updated` with `changed_keys = ["branding.logo_file_id"]` (M1-46 event shape); audit allow (Design note 9). Answer `200` with `Branding` built by M1-46's code (new `version`).
    - `RemoveLogo`: same `authorize`; unit of work: lock as above; no logo: no write, no event, audit allow with empty `changed_keys`, `204`; otherwise write the empty string with `version + 1`, `Release`, `instance.updated`, audit, `204`.

4. **Purpose rule in the pipeline** (12 6.1 second step, 12 7.1). Where M3-06 decides how the original is stored (filtered JPEG or PNG, converted still WebP, re-encoded GIF, or stored unchanged), a file with purpose `avatar` or `branding` is accepted only as filtered JPEG, filtered PNG or converted still WebP (`StoredOriginalAllowed`). Any other outcome (animated WebP, still WebP that cannot be converted because it exceeds `file.image.max_pixels`, GIF, HEIC, AVIF, TIFF, SVG, any non-image) sets `upload_state = 'failed'` with `failure_reason` `file_type_denied` on the session and `file.updated`, before the file can become deliverable, and the object is removed like any failed upload. Reason: `ImageUpload` admits PNG, JPEG and WebP, and the promise "without metadata" holds only for originals the server filtered or converted (12 7.1 'Limit of this promise'). A JPEG or PNG over `file.image.max_pixels` is filtered without decoding, stays deliverable and gets no variants (12 7.2). Ruling for docs/DECISIONS.md. Attachments are unchanged.

5. **Authorize before the body.** The generated strict handler receives the multipart reader; the use case calls `authorize` before it reads the first part. An anonymous caller, a principal without the right, an owner without fresh step-up (`401 step_up_required`), an owner with a personal token (`403 forbidden`, `reason` `step_up_not_possible`, SR5) and a non-owner on the logo (`403 forbidden`, `reason` `owner_only`) are answered without one byte of the body read. Denials of `instance.branding.update` are audited by the authz layer (Audit `yes`, R-AZ-04). The chain caps the body at `file.max_upload_bytes` and applies `http.transfer_timeout` to multipart routes (02 9.1 step 5, M1-18).

6. **`file.read` by purpose** (05 2.2, 07 4.2 and 4.3). `files` sets `FilePurpose` and `OwnerID` (= `uploaded_by`) in every `file` descriptor it builds. `authz` decides `file.read`, in addition to M3-03's rule for `attachment`:
    - `avatar`: visible and allowed for every principal that passes step 1 of 07 4.2 (active, not anonymous, not pending); apps still need scope `files:read` (05 2.6).
    - `branding`: the same over file routes. Anonymous callers never reach this branch: `file.read` is not `public` in the catalogue, so `authorize` answers `unauthenticated` in step 1. The public path is `getBrandingLogo` with `instance.discovery.read`.
    - `gdpr_export`: never visible through `file.read` (`not_found`); its only path is `gdpr.export.download` (M7).
    - A deleted file (`deleted_at` set) is `not_found` for everyone, with the body and headers of an unknown id (AZ-P4).
    - A `file` descriptor without path hint and without purpose (built from an event row, note 7) is visible only when `OwnerID` is the principal; anything else fails closed.
    Write these rules into `requirements.go` and `visibility.go` with the 05 text as `Source`, and the visibility bullet into 07 4.3.

7. **Events.**
    - `user.updated`: resource `{kind: user, id}` without version, empty scope, `UserPayload{user}` with `avatar_file_id` present when set (M1-31 shape), receive action `user.read`.
    - `instance.updated`: resource `{kind: instance, id: instance_id}`, empty scope, `{changed_keys: ["branding.logo_file_id"]}` (M1-46 shape), receive action `instance.read`.
    - `file.updated` of an avatar or logo file (written by the jobs of M3-04 and M3-06 at every state change): a personal event of the uploader, `audience_principal_id = uploaded_by`, empty scope, resource `{kind: file, id}`, receive action `file.read`, payload `FilePayload`. Reason: only the uploader waits for the result; every other reader learns of the new file through `user.updated` or `instance.updated` and retries a `409 file_not_ready` after `Retry-After`; and the event row has no column for the purpose (03 `event_log`), so a broadcast could not be checked per recipient. Ruling for docs/DECISIONS.md. For the receive check the descriptor built from an event row of kind `file` sets `OwnerID` from `audience_principal_id`; then note 6 admits exactly the uploader. `authorize.ResourceFromEvent` (M1-63) is merged: extend it this way (for kind `file` only) and adjust its call sites.
    - All payloads validate against their schemas; no JSON `null`.

8. **`getBrandingLogo`.** `authorize(principal or anonymous, instance.discovery.read, {kind: instance})`; read `branding.logo_file_id`; empty or absent: `404`. `LogoFiles.OpenVariant` returns the variant `medium` (longest edge `file.thumbnail.medium_px`, re-encoded, 12 7.3) of a deliverable file with purpose `branding`; a file that is pending, blocked, failed, deleted or without that variant gives `404`. Every `404` here has the same status, body and headers apart from `Correlation-Id`. Reasons: the public endpoint serves only pixels the server encoded itself, never bytes whose structure came from a client; one variant keeps the response small. Ruling for docs/DECISIONS.md. Answer `200` with:
    - `Content-Type`: the variant's type (`image/jpeg` or `image/png`);
    - `Content-Disposition: inline; filename="logo.<ext>"; filename*=UTF-8''logo.<ext>` (12 9.3 inline list: thumbnails);
    - `Content-Security-Policy: default-src 'none'; sandbox`, `X-Content-Type-Options: nosniff`, `Cross-Origin-Resource-Policy: same-origin`, `Access-Control-Allow-Origin: *` from the chain (SEC-CORS-2);
    - `Cache-Control: no-store` (the chain default stays): the URL is stable while its content changes, 06 8 point 6 has no `304`, and 12 9.3 defines caching only with sign-in or ticket. Ruling for docs/DECISIONS.md;
    - `ETag: "<sha256 hex of the variant>"` and `Repr-Digest: sha-256=:<base64>:`; `file_thumbnails` has no hash column, so read the variant once, hash it, then send headers and body (its size is bounded by `file.thumbnail.medium_px`);
    - no `Range` support (`openapi.yaml` lists no `206`): `Range` is ignored, `200` with the whole variant.

9. **Logo setting and audit.** The setting row `branding.logo_file_id` is never deleted: removing the logo stores the empty string, which configuration.md section 6 calls the default ("empty"), so the branding version (sum of the three setting versions, M1-46) rises on every change and an old `If-Match` never matches again. `Branding.logo_file_id` and discovery `logo_url` are omitted for the empty string. `putInstanceSetting` keeps rejecting branding keys (M1-46). Audit allow entries of `instance.branding.update` through `auditing.Recorder` in the unit of work: upload `{changed_keys: ["branding.logo_file_id"], file_id}`, removal `{changed_keys: ["branding.logo_file_id"]}` or `{changed_keys: []}` for a removal without logo; only ids and keys, no names (05 2.4: every allow of an action with Audit `yes` is recorded). Rulings for docs/DECISIONS.md.

10. **No `If-Match` on the logo.** `openapi.yaml` gives `uploadBrandingLogo` and `deleteBrandingLogo` neither `IfMatchRequired` nor `412` or `428`, while 06 8 point 2 lists Branding among the objects that need `If-Match`. Follow `openapi.yaml`: the logo is replaced or removed as a whole, last write wins, and the row lock of note 3 serializes concurrent writes. Clarify 06 8 point 2 accordingly. Ruling for docs/DECISIONS.md.

11. **Rulings for `docs/DECISIONS.md`** (section 10, one row each, `D-M3-10-<n>`): the edge `files -> instance` with `instance.LogoFiles` (note 1); staging in a server-opened session of one part, capped by the smaller of `file.upload_part_bytes` and `file.max_upload_bytes` (note 2); fixed `original_name` `avatar` and `logo` (note 2); no quota check for avatar and logo (note 2); link at upload, release of the previous file with `deleted_at` (note 3); purpose rule with `failure_reason` `file_type_denied` (note 4); `file.read` by purpose including `gdpr_export` never through `file.read` (note 6); personal `file.updated` for avatar and logo files and `OwnerID` from the audience for event rows of kind `file` (note 7); `getBrandingLogo` serves variant `medium` with `Cache-Control: no-store` and no `Range` (note 8); empty string instead of a deleted row, audit details and audit of a removal without logo (note 9); no `If-Match` on the logo operations (note 10).

12. **Gaps this ticket does not close** (name them in the report under spec gaps): agent avatars have no operation in `openapi.yaml` although 05 `agent.update` and 12 3.7 name them; the objects of released files and abandoned sessions stay until M3-08 runs `files.purge_objects` and `files.cleanup_pending`.

## Tests (write them first)

Unit:

- `internal/files/domain`: `TestDeclaredImageTypeAllowed`: `image/jpeg`, `image/png`, `image/webp` accepted for `avatar` and `branding`; `image/gif`, `image/svg+xml`, `image/heic`, `text/html`, `application/octet-stream` and the empty string rejected. `TestStoredOriginalAllowed`: filtered JPEG, filtered PNG and converted still WebP accepted; animated WebP, unconvertible still WebP, GIF, HEIC and unchanged storage rejected for both purposes; `attachment` never rejected by this rule. `TestImageCap` (both orders of the two keys). `TestFixedOriginalName`.
- `internal/authz`: `TestFileReadByPurpose`, a table over principals (Alex and Sam as active humans, an agent, an app with and without `files:read`, anonymous, a person on a pending device, a disabled person) and purposes `avatar`, `branding`, `gdpr_export`: humans and agents `allow` on `avatar` and `branding`; app with scope `allow`, without `forbidden` `scope_missing`; anonymous `unauthenticated`; pending device `forbidden` `device_pending`; disabled `unauthenticated`; `gdpr_export` `not_found` for everyone including the owner and the uploader. `TestFileReadEventDescriptor`: a descriptor of kind `file` without path and purpose with `OwnerID` Alex gives `allow` for Alex and `not_found` for Sam.
- `internal/instance/domain`: `TestLogoSettingEmptyMeansAbsent` (`Branding` without `logo_file_id`, discovery without `logo_url`), `TestLogoSettingRejectsOtherValues`.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain, real step-up from the identity test helpers, the file system object store in a `mktemp -d` folder, the fake `Scanner` of M3-04 answering `clean` unless a test sets another verdict, jobs `files.scan` and `files.thumbnail` run to completion in the test as M3-04 and M3-06 do; Alex and Sam are people, Alex is the owner):

- `TestSetMyAvatar_HappyPath`: Alex uploads the JPEG fixture: `200` with `User.avatar_file_id`; the `files` row has `purpose` `avatar`, `scope_channel_id` NULL, `original_name` `avatar`, `uploaded_by` Alex; the session is `completed`; one `files.scan` job exists; one `user.updated` with the new id; after the jobs the file is deliverable and has the variants of M3-06.
- `TestSetMyAvatar_ReplacesPrevious`: a second upload sets `deleted_at` on the first file; `getFile` on the first id answers `404` with status, body and headers equal to an unknown id (AZ-P4).
- `TestSetMyAvatar_Rejections`: declared `image/gif` and `image/svg+xml` give `415`; `file.denied_mime_types` set to `image/png` and a PNG give `415` with `reason` `file_type_denied`; a part one byte over the cap gives `413` whose `detail` names the key; no part `file`, an empty part and a second part give `400 validation_failed`; after each case there is no `files` row, no session and no object.
- `TestSetMyAvatar_AuthorizeBeforeBody`: with a request body whose reader fails the test when read, anonymous gets `401` and an agent gets `403` with `reason` `subject_kind_not_allowed`; nothing is stored.
- `TestSetMyAvatar_DeclaredTypeLies`: declared `image/png`, bytes are HTML: after `files.scan` the file is `upload_state = 'failed'` with `failure_reason` `file_type_denied` and never deliverable; `getFile` shows `failed`.
- `TestSetMyAvatar_AnimatedWebPFails` and `TestSetMyAvatar_GIFAsPNGFails`: same outcome through the purpose rule of note 4.
- `TestDeleteMyAvatar`: `204`, `avatar_file_id` cleared, previous file `deleted_at` set, one `user.updated`; a second call `204` without event; no audit rows for avatar operations.
- `TestAvatarReaders`: Sam reads Alex's avatar with `getFile` (`200`, `purpose` `avatar`); an agent `200`; an app without `files:read` `403` `scope_missing`; anonymous `401`. If `downloadThumbnail` and `downloadFile` are in the enabled list of `api/operations.yaml` in the base branch (M3-05 merged), run the same matrix against them and check the delivered bytes for metadata as in `TestImagesWithoutMetadata`; otherwise list these cases under "What is not proven" in the report.
- `TestAvatarConcurrentReplace`: two parallel uploads of Alex both end `200` or with a defined status (no deadlock reaches the client); afterwards exactly one file is linked and every other avatar file of Alex has `deleted_at`.
- `TestUploadBrandingLogo_HappyPath`: the owner with fresh step-up uploads the PNG fixture: `200` with `Branding.logo_file_id` and `version` raised by one; one `instance.updated` with exactly `["branding.logo_file_id"]`; one audit allow entry with `changed_keys` and `file_id`; after the jobs anonymous `getBrandingLogo` answers `200` with the bytes of the `medium` variant and the headers of note 8 (SEC-T03 for this response kind: `Content-Type`, both `filename` parameters, CSP, `nosniff`, CORP, `Cache-Control: no-store`, `ETag`, `Repr-Digest`, `Access-Control-Allow-Origin: *`, no `Access-Control-Allow-Credentials`).
- `TestUploadBrandingLogo_Rights`: Sam gets `403` with `reason` `owner_only` and one deny entry in the audit log (AZ-P13 shape); the owner with step-up older than `auth.step_up_max_age` gets `401 step_up_required` (AZ-M36 shape); the owner with a personal API token gets `403` with `reason` `step_up_not_possible` (AZ-M35 shape); anonymous gets `401`; the body is never read in these four cases.
- `TestGetBrandingLogo_NotDeliverable`: no logo, logo pending (scanner not ready), logo `infected` (fake verdict) and a JPEG logo over `file.image.max_pixels` (deliverable, no variant) each give `404` with equal status, body and headers apart from `Correlation-Id`; before setup the answer is `404`, not `503 setup_required`.
- `TestLogoPublicOnlyThroughGetBrandingLogo`: anonymous `getFile` on the logo id gives `401`; anonymous `downloadFile` and `downloadThumbnail` give `401` when enabled; Sam signed in reads the metadata with `getFile` (`200`, `purpose` `branding`).
- `TestDeleteBrandingLogo`: `204`; the setting holds the empty string with a raised version; discovery has no `logo_url`; `getInstance` has no `logo_file_id`; `getBrandingLogo` `404`; the old file has `deleted_at`; one `instance.updated`; one audit allow entry; a second call `204`, no event, one audit allow entry with empty `changed_keys`.
- `TestLogoVersionMonotonic`: upload, remove, upload: the branding version rises strictly; `updateBranding` with the `If-Match` of the first `Branding` gets `412 version_conflict`; `updateBranding` keeps `logo_file_id`.
- `TestImagesWithoutMetadata` (plan: "images re-encoded without metadata"), for avatar and for logo, with fixtures: JPEG with GPS EXIF, XMP, IPTC, `COM`, `APP2` with Multi-Picture-Format, `APP2` with `ICC_PROFILE`, orientation 6 and bytes after the first `EOI`; PNG with `tEXt`, `iTXt`, `zTXt`, `eXIf`, `tIME` and an unknown ancillary chunk; still WebP with `EXIF` and `XMP ` chunks. Assert: the stored original of the JPEG has exactly one `APP1` with only "Orientation", `APP2` only `ICC_PROFILE`, no other `APPn`, no `COM`, nothing after the first `EOI`; the PNG has only allow-listed chunks; the WebP is stored as PNG or JPEG without any metadata; every variant decodes, is not larger than its key's edge, has the orientation applied (width and height swapped for orientation 6), carries no EXIF, XMP, `COM` or PNG text or `eXIf` chunk, and keeps the colour profile of the original where there is one; the bytes of `getBrandingLogo` satisfy the same.
- `TestAvatarAndLogoFileEvents`: every `file.updated` of an avatar or logo file has `audience_principal_id` = uploader, empty scope, resource kind `file`, receive action `file.read`, and validates against `FileUpdatedEvent`; `user.updated` and `instance.updated` validate against their schemas; no JSON `null` anywhere.
- `TestImageUploadSEC_T08` (SEC-T08 for these routes): false type declaration (failed, never delivered), SVG with script (`415`), image over `file.image.max_pixels` (no decoding, no variant), size over the cap (`413`), scanner not ready (stays `pending`, logo `404`), infected verdict of the fake scanner (never delivered, logo `404`).
- `TestRoutesRegistered` (AZ-P1): the five routes are registered with their actions and rate-limit classes.

Fuzz:

- `FuzzImageUploadParts` in `internal/identity/adapters/httpapi` (or where the multipart handling sits): random multipart bodies (part count, names, headers, sizes around the cap) never give a `5xx`, and every non-`2xx` answer leaves no `files` row. Run it once with `go test -run=^$ -fuzz=FuzzImageUploadParts -fuzztime=<duration of your choice>` and report duration and result; commit any crashing input as a seed after the fix.

Every response in these tests passes the response validation of the test build (M1-17, M1-18).

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/files/... ./internal/identity/... ./internal/instance/... ./internal/authz/... ./internal/kernel/... ./internal/platform/httpserver/... ./internal/wire/...`
- `make codegen-check`, `make arch`, `make register-check`, `make api-lint`, `make api-checks`
- `make diagrams`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `setMyAvatar`, `deleteMyAvatar`, `uploadBrandingLogo`, `getBrandingLogo` and `deleteBrandingLogo` are enabled and registered with their actions; AZ-P1 passes.
- [ ] Images re-encoded without metadata: the stored originals of avatar and logo carry no metadata (JPEG and PNG filtered losslessly, still WebP converted), every variant is re-encoded without metadata with orientation applied, and an image that cannot be stored without metadata ends `failed` and is never delivered.
- [ ] The logo is publicly readable only through `getBrandingLogo`: anonymous callers get the `medium` variant with the headers of SEC-HDR-4, 12 9.3 and SEC-CORS-2 there, `401` on every file route, and the same `404` while no deliverable logo exists.
- [ ] Avatars only for entitled readers: every active signed-in principal reads an avatar file through `file.read`; anonymous `401`, an app without `files:read` `403`; `gdpr_export` never through `file.read`; a replaced or removed avatar gives the same `404` as an unknown id.
- [ ] Logo changes need an owner with fresh step-up, are audited on allow and deny, write `instance.updated` with only `branding.logo_file_id`, and raise the branding version; avatar changes write `user.updated`; `file.updated` of these files reaches only the uploader.
- [ ] `authorize` runs before the body is read; an upload rejected while it is staged stores nothing (Design note 2); a staged file that fails later stays as Design note 2 g and `TestSetMyAvatar_DeclaredTypeLies` describe.
- [ ] The specification changes of Scope (02, 06, 07, 12, `openapi.yaml` descriptions, diagram, module register) and the `D-M3-10-<n>` rows are in the same change.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `files`, `identity`, `instance` and `authz` updated (the source ports `AvatarFiles` and `LogoFiles`, the purpose rules, the read rules by purpose, the personal `file.updated`, the new dependency `files -> instance`).
- Rulings of note 11 recorded in `docs/DECISIONS.md` section 10 (one row each with reason and place).
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
