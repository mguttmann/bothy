# M7-23: Release chain of the host service

Status: ready | Depends on: M2-48, M7-12 | Milestone: M7

## Goal

After this ticket the component `host` can be released by a tag `host/v<version>` (18 2): `make -C services/host release-build` builds `bothy-host` for macOS (arm64, amd64), Linux (amd64, arm64) and Windows (amd64), each with a signature, an SBOM and an entry in the signed `SHA256SUMS`, and the installation packages with the service definitions of 16 4.4 (launchd LaunchDaemon, systemd system service with socket activation, Windows service) from `infra/host/`. `release.yml` runs the macOS signing and notarization in `release-apple` and the Authenticode signing in `release-windows` with the publisher credentials of the owner, which are paid accounts and stay an owner decision (OQ-08, D-W-125); the trial release (`make release-trial COMPONENT=host`, D-W-127) proves the chain with the test publisher of M7-12: `cosign verify-blob` and `sha256sum --check` green for every binary, `codesign --verify --strict` green on macOS, `signtool verify /pa` green on Windows, a missing SBOM aborts, `VERSION` equals the tag. A host service below the minimum version that the server supports gets `4114 host_version_unsupported`, does not reconnect and shows the update path of its installation package (18 9.1). Why: exit criterion 7 of M7; the host service runs with administrator-installed services on machines with real data (24 2.2), so its binaries must be verifiable, and it never updates itself (18 9.1), so the package route is the only update path.

## Read first

- `docs/spec/18-release.md`: 2 (row `host`), 5 (channels), 6, 9.1, 13.
- `docs/spec/16-operations.md` 4.4 (installation of the host service, items 1 to 5: services, owners, socket activation), `docs/spec/24-terminal.md` 2.2 (HD1 to HD11, the service user has no write permission on its binary), the close code table (`4114`).
- `docs/api/asyncapi.yaml` (control channel, `x-protocol-versions`, `host_version_unsupported`), ADR 0036, ADR 0062, `docs/operations/release.md` (sections 2 and 5 pattern).
- `.github/workflows/release.yml` (jobs `build`, `build-apple`, `build-windows`, `publish`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125), the trial release and the test publisher (D-W-127); section 10 (format of `D-M7-23-<n>` rows).
- Briefs (merged names win, D-W-100): `M2-38-noise-core.md` (module path, `cmd/bothy-host`), `M2-42-host-install-doctor.md` (`internal/buildinfo`, `release-build` with `-ldflags -X`, `bothy-host install`, its service templates; "Developer ID signatures, notarization and Authenticode ... installation packages and their service definitions in `infra/host/`: M7-23"), `M2-48-terminal-conformance.md` (host service in a Linux container), `M7-10-server-release-chain.md`, `M7-12-desktop-distribution.md` (test publisher scripts).
- Code on the base branch: `services/host` (Makefile, `internal/buildinfo`, `internal/install`), `tools/distribution`.

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
8. White label: package metadata uses `Bothy` and `example.com`; no company, host or personal names.
9. Every tool is pinned (checksum or digest); the packaging tools pass the license gate as build tools.
10. Specification first: a change to the release or installation rules updates `docs/spec/18-release.md` and `docs/spec/16-operations.md` 4.4 in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): no Apple Developer Program, no Windows certificate or Artifact Signing account; GitHub-hosted standard runners only.
12. Never tag, never create a release, never publish (D-W-125).

Product requirements implemented here:

13. Builds are static where the platform allows (Linux without cgo) and reproducible from `VERSION` (`-ldflags -X` as M2-42 set up); the binary's `version` output equals `services/host/VERSION`.
14. The packages install the binary and the service definitions owned by a trusted owner (root, `SYSTEM`) with modes that keep the service user from writing them (24 2.2, HD10); the packages never start a service for a session user and never pair; setup stays `bothy-host install` and `pair` (16 4.4).
15. Signing steps only in jobs with a release environment (ADR 0062, `make runners-check`); without credentials the release job stops clearly.
16. On `4114` the host service logs one line with the update path of its package kind (recorded at build time) and exits with its own exit code that the service manager does not restart (24 2.2).

## Scope

- `services/host/Makefile`: `release-build VERSION=<v> OUT=<dir>` for the five targets, packages, SBOMs; `release-trial` support for `make release-trial COMPONENT=host`.
- `infra/host/`: service definitions (launchd plist, systemd unit and socket, Windows service configuration) and package recipes (Design note 1), a README.
- `services/host/internal/buildinfo`: the package kind as a build value; the handling of `4114` in the control connection (update path line and exit code), if the control connection of M2-43 does not already do exactly this.
- `.github/workflows/release.yml`: the host path (Linux builds and packages in `build`, macOS signing and `.pkg` in `build-apple`, Windows signing and MSI in `build-windows`, all for the component `host`).
- CI: the platform trials run in the existing `apps-macos` and `apps-windows` jobs or in new jobs on `macos-15` and `windows-2025` when `services/host` or `infra/host` change.
- Docs: `docs/spec/18-release.md` 2 and 9.1 as built, `docs/spec/16-operations.md` 4.4 (installing from the package), `docs/operations/release.md` (host release and verification), `services/host/README.md`, `services/host/CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-23-<n>`.

## Out of scope

- Self-update of the host service (never, 18 9.1); package repositories or a hosted update channel; real Developer ID and Authenticode signatures (owner).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-23-host-release-chain`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(host): ...`, `ci(release): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). Package installation tests run only in throwaway containers (Linux) or on the GitHub-hosted runners (macOS, Windows), never on the build host's own system.
- The macOS and Windows trials need those platforms; run what the build host supports and prove the rest in CI; a trial neither could run is a stop: `Verdict: BLOCKED <platform> trial not run`.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; test publisher keys live only in the run's temporary keychain or certificate store.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Packages** (16 4.4; Ruling for docs/DECISIONS.md). Linux: `.deb` and `.rpm` (for example with nfpm, pinned); macOS: a signed `.pkg` built with `pkgbuild` and `productbuild`; Windows: an MSI (for example WiX, pinned) that installs the binary and registers the service in a stopped state. Each package names its update route in the `4114` line ("install the newer package of the same kind"). Installation tests: install the `.deb` and `.rpm` in throwaway containers of the supported distributions, assert file owners and modes and that no service runs before `bothy-host install`; macOS and Windows in CI with the test publisher.

2. **`spctl` with the test publisher** (D-W-127, as in M7-12). `spctl --assess` accepts only an Apple-issued Developer ID with notarization; the trial records its rejection, the release job runs it with the real identity; report it as not proven.

3. **Rulings for `docs/DECISIONS.md`** (`D-M7-23-<n>`): package tools and formats (note 1), the `spctl` limit (note 2), the `4114` exit code and update line.

## Tests (write them first)

- Go: `TestVersionEqualsVERSION`; `Test4114ShowsUpdatePathAndExits` against a fake server that closes with `4114` (no reconnect, exit code, one line with the package route).
- Packaging tests (Linux containers): owners and modes, no running service, uninstall removes the binary and keeps the state folder as 16 4.4 says.
- Trial: `make release-trial COMPONENT=host` with `cosign verify-blob`, `sha256sum --check`, SBOM per binary, abort without SBOM; `codesign --verify --strict` and `signtool verify /pa` in CI.

Commands that must pass, from the repository root:

- `cd services/host && go test -race -count=1 ./...`
- `make release-trial COMPONENT=host` on the build host
- `make lint`, `make runners-check`, `make licenses`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] Trial release: `cosign verify`, `codesign --verify` on macOS and `signtool verify` on Windows with a test publisher green; a missing SBOM aborts; `VERSION` equal to the tag.
- [ ] Binaries for the five targets and the packages with service definitions are built; the Linux packages install correctly in containers.
- [ ] A host service below the minimum version shows the update path on `4114` and does not reconnect.
- [ ] 18, 16, the runbook, READMEs, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `infra/host/README.md` and `services/host/README.md` written in English.
- Rulings of Design note 3 recorded in `docs/DECISIONS.md` section 10.
- `services/host/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`, of the trial and of the CI platform trials.
3. Review and red team verdicts and the findings fixed.
4. Package formats, sizes and the verified owners and modes.
5. What is not proven (Developer ID, notarization, `spctl` acceptance, a real Authenticode certificate).
6. Owner steps with a recommendation (OQ-08); follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
