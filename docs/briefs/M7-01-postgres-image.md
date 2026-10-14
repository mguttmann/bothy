# M7-01: Image `bothy-postgres` with pgBackRest

Status: ready | Depends on: M1-08, M1-23 | Milestone: M7

## Goal

After this ticket the image `bothy-postgres` exists: the official `postgres:18` image, pinned by digest, plus pgBackRest from version 2.59.3 on and the program `bothy-backupd`, built for linux/amd64 and linux/arm64 without emulation (18 7 rules 2 and 4). One image serves two Compose services (16 2, R-OP-07): `postgres` runs PostgreSQL with `archive_command` = pgBackRest `archive-push` over the TLS protocol of pgBackRest to the repository host `backup`; `backup` runs `bothy-backupd`, which in this ticket starts the pgBackRest TLS server, creates the stanza on its first start and answers a health check. The database container has no egress and no credentials of an off-site target (R-OP-08). `bothy secrets init` generates the catalogue entry `pgbackrest_tls/` (own CA, a server certificate for `backup`, a client certificate for `postgres`), whose generator the configuration reference assigns to this ticket. An integration test with two containers proves `archive-push`, a full backup, a point-in-time restore and `pgbackrest verify` on amd64 and arm64. Why: every later backup ticket (M7-02 to M7-04, M7-07) and the release (M7-10) build on this image; a backup that only works on one architecture or that needs egress from the database would break R-OP-08 and B12.

## Read first

- `docs/spec/16-operations.md`: 2 (building blocks `postgres` and `backup`), 5.3 (secrets per service, owners of third-party services), 7.1 (roles), 7.2 (settings `wal_level`, `archive_mode`, `archive_timeout`), 8.1 (rationale 1 and 4: pgBackRest, version 2.59.3), 8.2 (flow item 1), 15 (R-OP-06 to R-OP-08).
- `docs/spec/18-release.md`: 2 (component `server` publishes `bothy-postgres`), 5 rule 3 (no `latest`), 7 (rules 1, 2, 4, 5, 6).
- `docs/adr/0058-backup-with-pgbackrest-object-backup-and-restore-test.md` (decision item 1, "Confirmation"), `docs/adr/0012-oci-images-compose-opentofu.md`.
- `docs/operations/backup-restore.md` 1, 2 and 5.2; `docs/operations/configuration.md` section "Secrets" (rows `pgbackrest_tls/`, `pgbackrest_cipher_pass`, `db_superuser_password`, the paragraph "Generators of later milestones") and the environment variables `BOTHY_PG_ARCHIVE_TIMEOUT`, `BOTHY_PGBACKREST_REPO_HOST`.
- `infra/compose/compose.yaml` (services `postgres` and `backup`), `services/server/Dockerfile`, `services/server/Dockerfile.dockerignore`, `Makefile` (targets `image`, `image-test`, `trivy-image`), `.github/workflows/ci.yml` (job `plan`, `ci-ok`), `tools/ci/dockerfile_pins.py`, `tools/ci/check_runners.py`.
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-01-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-23-server-image.md` (image tests with the build tag `imagetest`, the pin check), `M1-09-secret-provider.md` (`bothy secrets init`, the catalogue), `M1-12-postgres-migrations.md` (roles), `M1-73-compose-smoke.md` (overlays under `infra/compose/test`).

This ticket touches no table, no action and no event.

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
9. Schedules, limits and periods only through named configuration keys or environment variables listed in `docs/operations/configuration.md`; code comments and docs name the key, never the value.
10. Specification first: a change to behaviour, configuration or a secret entry updates `docs/spec/` and `docs/operations/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): only free tools and GitHub-hosted standard runners, which are free for public repositories; no paid plan or service.

Product requirements implemented here:

12. Base `postgres:18` pinned by digest (SEC-SC-4); pgBackRest version 2.59.3 or later, pinned (a version below fails the build and a test); no moving tag.
13. Built per architecture on its own runner, never under emulation (18 7 rule 2); one multi-arch manifest for linux/amd64 and linux/arm64.
14. `archive_command` pushes over pgBackRest TLS with mutual certificates; the container `postgres` is attached only to the internal network and holds no off-site credential (R-OP-08).
15. The repository is encrypted with `pgbackrest_cipher_pass` (16 8.1 rationale 1); the cipher pass and the TLS keys are read from files under `/run/bothy/secrets`, never from environment variables (R-OP-05).
16. OCI labels as in 18 7 rule 6; a build without a version fails.

## Scope

Image (ruling: the plan names `images/bothy-postgres`; the image is an artifact of the component `server` (18 2), so it lives in that component's directory, where the component path filter of `tools/ci/components.py` sees it):

- `services/server/images/bothy-postgres/Dockerfile` with the repository root as build context and its own `Dockerfile.dockerignore` (allows only what the image needs), stages: a Go builder for `bothy-backupd` (static, without cgo, `--platform=$BUILDPLATFORM` cross compile like `bothy-server`), pgBackRest installed from a pinned package or built from a checksum-verified source (Design note 1), final stage `postgres:18@sha256:...`.
- `services/server/images/bothy-postgres/` entrypoint scripts: for `postgres` they write `pgbackrest.conf` (repo host `BOTHY_PGBACKREST_REPO_HOST`, TLS client certificate) and set `archive_mode`, `archive_command`, `archive_timeout` from `BOTHY_PG_ARCHIVE_TIMEOUT`; for `backup` the configuration of the repository host.
- `services/server/cmd/bothy-backupd` (ruling: a second command of the server module, see Design note 2): in this ticket `bothy-backupd` (default: start the pgBackRest TLS server as a child process, create the stanza if missing, stop the child on `SIGTERM`), `bothy-backupd healthcheck` and `bothy-backupd version`; its README.
- `bothy secrets init`: the generator of `pgbackrest_tls/` (CA, server and client key pairs, ECDSA P-256, validity as a named value in the catalogue), copied to the services `postgres` and `backup` only; the catalogue status changes from `pending` to generated; tests in the secrets package.
- `infra/compose/compose.yaml`: services `postgres` and `backup` as the image needs them (secret file paths, the internal network only for `postgres`); `docs/spec/16-operations.md` 2 and 8.2 if the build sharpens them.
- `Makefile`: targets `postgres-image` (build both platforms into an OCI archive and load the host platform as `bothy-postgres:pg18-<server version>-local`) and `postgres-image-test` (the integration test below); both outside `check`, like `image` and `image-test`.
- `.github/workflows/ci.yml`: a job `postgres-image` with a matrix of `ubuntu-24.04` and `ubuntu-24.04-arm` that runs both targets when the plan job lists the component `server`; `ci-ok` counts it; `make runners-check` and `make lint` stay green.
- Tests: `services/server/test/postgresimage/` with the build tag `imagetest` (Design note 3).
- Docs: `services/server/images/bothy-postgres/README.md`, `docs/operations/backup-restore.md` 1 and 2 where the build sharpens them, `docs/operations/configuration.md` (secret row and generator paragraph), `docs/quality/gates.md` (the two targets and the CI job), `services/server/CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-01-<n>`.

## Out of scope

- Schedules, logical and object backup, manifest, `backup_runs`, metrics (M7-02); the restore test (M7-03); `bothy restore finalize` (M7-04).
- Publishing the image and the release workflow (M7-10); the image is built and tested locally and in CI only.
- A managed PostgreSQL service (16 3 rule 3).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-01-postgres-image`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(server): ...`, `feat(compose): ...`, `ci: ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). Containers publish no host port; the test network is created per run and removed at the end.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; tests generate throwaway keys in their temporary directory.
- No network calls except pulling the pinned base images and package downloads of pinned tools; the egress test uses an address of the documentation range `192.0.2.0/24` that is never routed.
- Docker with buildx must be available; run `make postgres-image postgres-image-test` once on the build host or stop with `Verdict: BLOCKED docker missing`. The other architecture is proven by the CI job.
- Never lower a threshold, disable a gate, delete a test or add an unpinned `FROM`.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **pgBackRest source** (16 8.1 rationale 4; Ruling for docs/DECISIONS.md). Prefer the package of the PostgreSQL project's Debian repository for the Debian release of the pinned `postgres:18` digest, pinned with an exact version and verified by the repository signature key that is itself pinned by fingerprint in the Dockerfile; if no package of version 2.59.3 or later exists for both architectures, build from the release tarball verified by a SHA-256 in the Dockerfile. A build step fails when `pgbackrest version` reports less than 2.59.3. Renovate keeps the pin current (a comment rule in the Dockerfile like the existing pins).

2. **`bothy-backupd` as its own command** (02 3.3 registers the commands of `bothy`; Ruling for docs/DECISIONS.md). `bothy-backupd` runs inside `bothy-postgres` next to pgBackRest and PostgreSQL client tools; the server binary with its API role has no business in the database image. It is a second `main` package `services/server/cmd/bothy-backupd` of the server module, version from `internal/platform/buildinfo`, so the server release (M7-10) builds and versions both images from one commit. Its later use cases (M7-02, M7-03) live in the module `status` (owner of `backup_runs`, port `BackupRuns`); this ticket adds only the process supervision in `cmd/bothy-backupd`, which `make arch` already covers under `cmd`.

3. **Integration test** (ADR 0058 confirmation; `imagetest`). In Go with `os/exec` for the Docker CLI: a fresh internal network (`docker network create --internal`), container `backup` (command `bothy-backupd`) and container `postgres` with throwaway certificates and cipher pass from `bothy secrets init` into a temporary directory. Steps: insert rows, force `pg_switch_wal()`, the segment appears in the repository (`pgbackrest info`); full backup triggered with `pgbackrest backup --type=full` from inside `backup`; insert more rows, note the time, insert again; stop `postgres`, `pgbackrest restore --type=time --target=<time>` into a fresh volume, start, assert exactly the rows before the target; `pgbackrest verify` exits `0`. Negative cases: a client certificate from another CA is rejected by the TLS server; from inside `postgres` a TCP connection to `192.0.2.1:443` fails and the container has exactly one network, the internal one; the image has no tag `latest` and its labels carry version, revision and source.

4. **CI job** (B12, 18 7 rule 2). The job builds natively on each runner; it never sets up QEMU. It runs only when the component `server` changed, so other pull requests stay fast; release builds of the image belong to M7-10.

5. **Rulings for `docs/DECISIONS.md`** (`D-M7-01-<n>`): the image directory (Scope), the pgBackRest source and pin (note 1), the separate command (note 2), the certificate profile and validity of `pgbackrest_tls/`, the CI job (note 4).

## Tests (write them first)

- Go unit tests in the secrets package: `pgbackrest_tls/` is generated once (second run `kept`), the client and server certificates chain to the CA, keys are `0400` in the store, only `postgres` and `backup` receive copies.
- Go unit tests in `cmd/bothy-backupd`: the child process is stopped on `SIGTERM`; `healthcheck` fails while the TLS server does not answer.
- `services/server/test/postgresimage` (`imagetest`): the steps and negative cases of Design note 3.
- A check in `make lint` (existing pin check) covers the new Dockerfile; a test that a pgBackRest version below 2.59.3 fails the version step (for example by running the version guard script against a fake `pgbackrest` that prints an old version).

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./cmd/... ./internal/platform/secrets/...`
- `make postgres-image postgres-image-test` on the build host
- `make lint`, `make runners-check`, `make trivy`, `make config-doc-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `make postgres-image` builds linux/amd64 and linux/arm64 natively; the archive holds both platforms; the base is pinned by digest and pgBackRest is 2.59.3 or later.
- [ ] `make postgres-image-test` proves `archive-push` over TLS, a full backup, a PITR restore to a point in time and `pgbackrest verify`; the CI job `postgres-image` runs it on amd64 and arm64 and is green.
- [ ] The database container has only the internal network and no egress; a foreign client certificate is rejected.
- [ ] `bothy secrets init` generates `pgbackrest_tls/` and gives it only to `postgres` and `backup`.
- [ ] Compose, 16, the runbook, the configuration reference, `gates.md`, the README, the CHANGELOG and the rulings describe the image as built.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of the image and of `cmd/bothy-backupd` written in English.
- Rulings of Design note 5 recorded in `docs/DECISIONS.md` section 10.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make postgres-image-test` with the architecture of the build host, and the result of the CI job on both architectures.
3. Review and red team verdicts and the findings fixed.
4. The pinned pgBackRest version and how it is installed; the image size per architecture.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
