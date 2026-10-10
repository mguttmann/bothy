# M1-23: Server image (ADR 0012)

Status: ready | Depends on: M1-11 | Etappe: M1

## Goal

After this ticket the repository builds the image `bothy-server` for `linux/amd64` and
`linux/arm64` from `services/server/Dockerfile`: a static Go binary `bothy` on a minimal base image
without a shell, running as an unprivileged numeric user, with a read-only root file system and
`bothy healthcheck` as the container health check. Base images are pinned by digest and a gate
keeps them pinned. `make image` builds and loads it, `make image-test` proves its structure, and
the release workflow builds the same file with version and revision. The Compose smoke test
(M1-73) and the release (M7) start from this image.

## Read first

- `docs/adr/0012-oci-images-compose-opentofu.md` (decision point 1, "Confirmation").
- `docs/spec/18-release.md`: "2. Komponenten", "5. Kanäle" (rule 3: no `latest`), "6. Lieferkette",
  "7. Server-Images".
- `docs/spec/16-betrieb.md`: "2. Bausteine einer Instanz", "4.2 Zwei Listener", "5.2 Secret-Provider",
  "5.3 Geheimnisse im Compose-Stack" (`secrets-init` runs as `0:0`), "10. Upgrade".
- `docs/spec/02-architektur.md`: "3.3 Befehle des Binarys" (`bothy healthcheck --url`,
  `bothy config check`), "6. Code-Layout" (name-bound places: binary `bothy`, image name).
- `docs/spec/19-sicherheit.md`: "12. Lieferkette, Sicherungen und Audit-Integrität" (SEC-SC-4,
  SEC-SC-5).
- `docs/spec/23-qualitaet.md`: "1. Grundsätze" (pinned tools), "8. Lieferkette".
- `docs/PLAN.md`: section 7, rows M1-11, M1-12, M1-23, M1-73; decisions P-05, P-06, P-09.
- `infra/compose/compose.yaml` (anchors `x-hardening`, `x-server-healthcheck`, `x-api`, service
  `secrets-init`), `.github/workflows/release.yml` (job `build`: context `.`, file
  `services/server/Dockerfile`, push by digest, `make trivy-image`), `Makefile` (targets `trivy`,
  `trivy-image`, `lint`), `renovate.json` (`config:best-practices` keeps digests current),
  `docs/quality/toolchain.md` ("Trivy: Version halten"), `docs/quality/gates.md`.
- Code merged by earlier tickets: `services/server/cmd/bothy` (subcommands `serve`, `migrate`,
  `healthcheck`, `config`), `services/server/VERSION`, `internal/platform/buildinfo` (M1-10; 18
  "Komponenten" rule 5: the version is a constant held equal to `VERSION` by a test).
- `CONTRIBUTING.md`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision,
   a CHANGELOG entry per release, docs site kept current. A change without matching documentation
   is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour change starts with a failing test (red output goes into the
   report), then code until green, then cleanup.
7. No en dash and no em dash in any file you write. White label: no company, person, host or
   instance names; the repository placeholder is `OWNER` (P-05).
8. Configuration only through named keys listed in `docs/operations/configuration.md`.
9. 18 "7. Server-Images": linux/amd64 and linux/arm64 as one multi-arch manifest; not built by
   emulation; static Go binary without cgo on a minimal base image without shell, user without root;
   `HEALTHCHECK` and Compose call `bothy healthcheck`; base images pinned by digest; OCI labels
   `org.opencontainers.image.source`, `.version`, `.revision`, `.licenses`.
10. 18 "5. Kanäle" rule 3: no moving tag `latest`.
11. SEC-SC-4: container base images pinned by digest.
12. 16 "1. Ziele" rule 5: secrets never in images.

## Scope

1. **`services/server/Dockerfile`**, build context is the repository root (as in `release.yml`):
   - Builder stage `FROM --platform=$BUILDPLATFORM golang:<toolchain>-<debian codename>@sha256:<index
     digest>`, where `<toolchain>` is the `toolchain` directive of `services/server/go.mod` (the
     `go` directive if there is none). `ARG TARGETOS TARGETARCH VERSION REVISION`. Fail the build if
     `VERSION` is empty or differs from `services/server/VERSION`. Copy `go.mod` and `go.sum` first,
     `go mod download` with a cache mount, then the sources. Build with `CGO_ENABLED=0`,
     `GOOS=$TARGETOS`, `GOARCH=$TARGETARCH`, `GOTOOLCHAIN=local`, `GOFLAGS=-mod=readonly`,
     `go build -trimpath -buildvcs=false -ldflags "-s -w" -o /out/bothy ./cmd/bothy`. The binary
     embeds its version through `buildinfo`; add no linker flag for it (18 "Komponenten" rule 5).
   - Final stage `FROM gcr.io/distroless/static-debian<NN>:nonroot@sha256:<index digest>`, using
     the newest Debian line the distroless project lists as supported when you build; record tag
     and digest in the README. `COPY --from=build --chown=0:0 --chmod=0555 /out/bothy
     /usr/local/bin/bothy`. `USER 65532:65532`. `ENTRYPOINT ["/usr/local/bin/bothy"]`,
     `CMD ["serve"]`. `HEALTHCHECK CMD ["/usr/local/bin/bothy", "healthcheck", "--url",
     "http://127.0.0.1:8081/readyz"]` (the default of `server.internal_listen_addr`; same command as
     Compose). Labels: `org.opencontainers.image.title=bothy-server`, `.source` from `ARG
     SOURCE_URL` (default `https://github.com/OWNER/bothy`), `.version=$VERSION`,
     `.revision=$REVISION`, `.licenses=Apache-2.0`. No `VOLUME`, no `EXPOSE` of host ports, no
     `ENV` with secrets, no `RUN` in the final stage (so no emulation is needed for arm64).
   - No `# syntax=` line: the built-in frontend of current BuildKit supports `RUN --mount`. If you
     need a newer frontend, pin it by digest and extend the pin check of item 4.
2. **`services/server/Dockerfile.dockerignore`** (Dockerfile-specific ignore file, so the root
   context of `release.yml` stays small and no other build is affected): ignore everything, then
   allow `services/server/go.mod`, `go.sum`, `VERSION`, `cmd/`, `internal/`, `api/`, `migrations/`;
   ignore again `**/*_test.go` and `**/testdata/`. If a package embeds files elsewhere, allow exactly
   those paths and say so in the report.
3. **Makefile** (root), with `##` help texts and `.PHONY` entries:
   - Variables `IMAGE_NAME ?= bothy-server`, `IMAGE_TAG ?= $(VERSION of services/server)-local`,
     `IMAGE_PLATFORMS ?= linux/amd64,linux/arm64`, `IMAGE_OCI ?= .cache/image/bothy-server.oci.tar`,
     `IMAGE_BUILDER ?=` (empty: current buildx builder).
   - `image`: fails clearly if `docker buildx` is missing; builds `IMAGE_PLATFORMS` with
     `--file services/server/Dockerfile`, `--build-arg VERSION=...`, `--build-arg
     REVISION=$(git rev-parse HEAD)` and `--output type=oci,dest=$(IMAGE_OCI)` from context `.`; then
     builds the host platform with `--load --tag $(IMAGE_NAME):$(IMAGE_TAG)`. If the builder cannot
     build several platforms, the target exits non-zero with the hint to create one
     (`docker buildx create --name <name> --driver docker-container`) and to pass
     `IMAGE_BUILDER=<name>`; it never creates a builder itself.
   - `image-test`: runs `golangci-lint run --build-tags imagetest ./test/image/...` and
     `go test -tags imagetest -count=1 ./test/image/...` in `services/server` with
     `BOTHY_TEST_IMAGE=$(IMAGE)` (default `$(IMAGE_NAME):$(IMAGE_TAG)`) and
     `BOTHY_TEST_IMAGE_OCI=$(abspath $(IMAGE_OCI))`; fails clearly without Docker (P-09).
   - Neither target joins `gates`; both are documented for the release (M7) and for M1-73.
4. **Pin gate** `tools/ci/dockerfile_pins.py` (standard library only, typed for `mypy --strict`)
   with `tools/ci/tests/test_dockerfile_pins.py`, called from the `lint` target: every tracked file
   matching `*Dockerfile*` (not `*.dockerignore`) is read; every `FROM` must reference an image
   with `@sha256:` and 64 hex digits, except a reference to an earlier stage name of the same file
   and `scratch`; `--platform=...` flags and `AS <name>` are understood; a `# syntax=` line must be
   pinned the same way. Output names file, line and reference; exit code 1 on a finding.
5. **Image structure test** `services/server/test/image/image_test.go` with build tag
   `imagetest`, using `os/exec` for the Docker CLI and the standard library for tar, JSON and ELF
   (details under Tests).
6. **Release workflow** `.github/workflows/release.yml`, step "Build and push the multi-arch server
   image by digest only": add `build-args` with `VERSION=${{ needs.resolve.outputs.version }}`,
   `REVISION=${{ github.sha }}` and `SOURCE_URL=${{ github.server_url }}/${{ github.repository }}`,
   and remove the `docker/setup-qemu-action` step: the server image is cross compiled and needs no
   emulation (18 "Server-Images" rule 2). Nothing else in the workflow changes; `make lint`
   (actionlint, zizmor) and `make runners-check` stay green.
7. **Docs and records**: `services/server/README.md` section "Image" (German: build, local tag,
   user `65532`, read-only root file system, no shell, health check, base images with tag and
   digest, how to create a multi-platform builder); `docs/spec/18-release.md` "7. Server-Images"
   already states the numeric user, cross compilation, the digest check and the build arguments;
   add the build context and the ignore file; `docs/quality/gates.md` names `image`, `image-test`
   and the pin check in the `lint` row; `CONTRIBUTING.md` says that image work needs Docker with
   buildx and a multi-platform builder; `docs/DECISIONS.md` entry for the base image choice
   (distroless static instead of `scratch`: maintained CA certificates and time zone data, digest
   updates by Renovate); `services/server/CHANGELOG.md` under `Unreleased` (`Added`).

## Out of scope

- `bothy-postgres` and `bothy-backupd` (M7), signing, SBOM, provenance and tagging in the release
  (M7; the workflow already pushes by digest and scans first).
- The Compose smoke test with the test overlay (M1-73) and making the installation Compose
  startable (M7).
- Reproducible image digests across builds: not required here; report whether two builds of the
  same commit matched if you observe it.
- Changes to `bothy healthcheck` or `bothy config` behaviour (M1-11, M1-07); if they block the
  image tests, stop with `Verdict: BLOCKED` and name the gap.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-23-server-image`, created from the base
  branch the launcher names. One writer per worktree.
- Conventional Commits with the component as scope (for example `build(server): multi-arch
  image`), each commit signed off for the DCO (`git commit -s`); squash into meaningful commits
  before the merge.
- Never push, tag, open pull requests or create remote repositories; never push an image to any
  registry.
- Do not modify files outside the scope above, except `docs/DECISIONS.md` and
  `services/server/CHANGELOG.md`.
- The core contract (00, 03, 04, 05) is read-only; a gap there ends the ticket with
  `Verdict: BLOCKED` (P-11).
- Follow the host rules supplied by the launcher. If no multi-platform buildx builder exists and
  the host rules allow it, create one with a unique name for this session and remove it at the
  end with `docker buildx rm <name>`; if the host rules forbid it, stop with `Verdict: BLOCKED`.
- Containers you start are removed by their ID; images you tag locally are listed in the report.
  Stop only processes you started, by PID. Temporary directories only via `mktemp -d` (in Go tests
  `t.TempDir()`), removed at the end.
- Never read or print secrets; never open `.env` files. Test configuration values contain
  `EXAMPLE` where they look like credentials.
- No network calls except downloads of pinned tools, Go modules and the pinned base images.
- Do not lower a threshold, disable a gate or delete a test.

## Design notes

1. **Cross compilation instead of emulation.** The builder stage runs on `$BUILDPLATFORM` and
   compiles for `$TARGETOS/$TARGETARCH`; the final stage only copies. This meets 18 "7." rule 2 even
   on one machine, so `release.yml` loses its QEMU step (Scope 6).
2. **User.** Numeric `65532:65532` (the `nonroot` user of distroless) so that runtimes that check
   "run as non-root" can verify it without `/etc/passwd`. The binary belongs to root with mode
   `0555`, so the running user cannot replace it. `secrets-init` in Compose overrides the user with
   `0:0`; the image must not prevent that.
3. **Read-only root.** The server writes only to mounted volumes and to `/tmp` (tmpfs in Compose
   `x-hardening`); the image declares no `VOLUME`, so no anonymous volumes appear.
4. **Health check path.** Compose and the image call
   `/usr/local/bin/bothy healthcheck --url http://127.0.0.1:8081/readyz`; the binary path is part
   of the contract with `infra/compose/compose.yaml`.
5. **Version.** `VERSION` comes from `services/server/VERSION` (make) or the resolved release
   version (workflow); the build fails if they differ, so labels and binary never disagree.
   `REVISION` is the commit SHA; a build from a dirty tree still labels `HEAD` (documented).
6. **Local tag.** `<version>-local`, never `latest`.
7. **Trivy.** `make trivy` scans the committed Dockerfile in config mode (stage it before you run
   the gate; the target scans tracked files only); `make trivy-image IMAGE=<local tag>` scans the
   loaded image with the pinned Trivy `0.69.3`.

## Tests (write them first)

- `tools/ci/tests/test_dockerfile_pins.py`: pinned `FROM` passes; tag without digest, `latest`,
  digest of the wrong length, unpinned `# syntax=` line fail; stage reference and `scratch` pass;
  `--platform` and `AS` are parsed; a file without `FROM` passes; output names file and line. Tools
  coverage stays at or above `quality.coverage.tools_min_percent`.
- `services/server/test/image/image_test.go` (`//go:build imagetest`), image from
  `BOTHY_TEST_IMAGE`, archive from `BOTHY_TEST_IMAGE_OCI`:
  - `TestImageConfig`: `docker image inspect` gives `User` `65532:65532`, `Entrypoint`
    `["/usr/local/bin/bothy"]`, `Cmd` `["serve"]`, the health check command of design note 4, the four
    OCI labels with `version` equal to `services/server/VERSION` and `licenses` `Apache-2.0`, and no
    environment variable whose name contains `SECRET`, `PASSWORD`, `TOKEN` or `KEY`.
  - `TestImageHasNoShell`: `docker create` plus `docker export` of the file system; the tar has
    `usr/local/bin/bothy` (owner 0, mode `0555`) and no entry whose base name is `sh`, `bash`,
    `ash`, `dash`, `zsh` or `busybox`; `docker run --rm --entrypoint /bin/sh <image> -c true` fails.
  - `TestBinaryIsStatic`: the exported `usr/local/bin/bothy` parsed with `debug/elf` has no
    `PT_INTERP` program header and no imported libraries, and its machine matches the host
    platform.
  - `TestReadOnlyRunAsNonRoot`: `docker run --rm --read-only --cap-drop ALL --security-opt
    no-new-privileges:true --tmpfs /tmp --network none -e
    BOTHY_SERVER__PUBLIC_URL=https://chat.example.com <image> config check` exits 0 (add exactly
    the variables `config check` needs on the base branch; secret refs as `env:` with values
    containing `EXAMPLE`).
  - `TestHealthcheckPresent`: `docker run --rm --read-only --network none <image> healthcheck --url
    http://127.0.0.1:8081/readyz` exits 1 (nothing listens), which proves the command exists and
    runs.
  - `TestMultiArchArchive`: the OCI archive's `index.json` leads to manifests for exactly
    `linux/amd64` and `linux/arm64` (attestation manifests with platform `unknown/unknown` are
    ignored).
  - Every container the test creates is removed by its ID in `t.Cleanup`.

Commands that must pass: `make image`, `make image-test`, `make trivy`,
`make trivy-image IMAGE=bothy-server:<version>-local`, `make lint`, `make check`. Also run
`zizmor` and `actionlint` through `make lint` for the workflow change, and `make runners-check`.

## Acceptance criteria

- [ ] `docker buildx build --platform linux/amd64,linux/arm64` succeeds through `make image`, and
      the archive holds both platforms.
- [ ] The container runs as `65532:65532` with a read-only root file system and without
      capabilities; it has no shell; the binary is static.
- [ ] Every `FROM` is pinned by digest, and the pin check in `make lint` fails on an unpinned
      `FROM`.
- [ ] The health check of the image equals the Compose health check.
- [ ] `make trivy` and `make trivy-image IMAGE=<local image>` are green.
- [ ] `release.yml` passes `VERSION`, `REVISION` and `SOURCE_URL`; `make lint` (actionlint,
      zizmor) and `make runners-check` are green.
- [ ] README, 18 "7. Server-Images", `gates.md`, `CONTRIBUTING.md`, `DECISIONS.md` and the
      CHANGELOG describe the image as built.
- [ ] `make check` is green.

## Definition of done

- Code, tests and documentation from the scope are complete: doc comments on every public symbol
  of the pin check and the test helpers, README and spec updates, `docs/DECISIONS.md` entries,
  CHANGELOG entry under `Unreleased` in `services/server/CHANGELOG.md`. Write an ADR only if you
  take an architecture decision beyond ADR 0012 and the rulings listed here.
- `make check`, `make image` and `make image-test` are green.
- Independent review: `make review` plus a code review over the diff; then the red team of
  `docs/quality/red-team-gate.md` over the diff with verdict `JA` or `NEIN`, recorded in the
  report. On `NEIN`: fix with a test per finding and have the delta reviewed again.
- Merge locally into the base branch only if review and red team are green and `make check` is
  green on the merged state; then `git worktree remove` the worktree.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, test results with counts (including the red output of the
first test run), the base image tags and digests, the local image tags you created, what is not
proven, and follow-ups that could not be done in scope with the reason. Then write
`{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
