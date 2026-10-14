# M7-10: Server release chain with a trial release (ADR 0036, 0060, 0062)

Status: ready | Depends on: M7-01 | Milestone: M7

## Goal

After this ticket the component `server` can be released by a tag, and the whole chain is proven before anyone tags: `release.yml` builds `bothy-server` (cross compiled) and `bothy-postgres` (per architecture on its own runner, no emulation) for linux/amd64 and linux/arm64, joins each into one multi-arch manifest, scans, runs ScanCode over the tree, creates SBOMs (SPDX and CycloneDX per image and binary), signs every digest and `SHA256SUMS` (key pair without a transparency log while the repository is private, keyless with attestations when it is public, ADR 0062), writes checksums and only then sets the version tags on the images (18 6 and 7). A missing SBOM aborts; `VERSION` must equal the tag. The trial release (`make release-trial COMPONENT=server`, D-W-127) runs the same component targets on the build host against throwaway local targets: images into a local registry, signatures with a throwaway key pair, SBOMs and checksums into a temporary directory, then `cosign verify --key` and `sha256sum --check`; it never pushes to a public registry, never creates a GitHub release and never tags. `make release-prep` implements ADR 0060 with git-cliff as a pinned binary: the next version per component from Conventional Commits under its path, the beta counter, the changelog section renamed, `VERSION` written and the checklist of commits without a changelog entry. M7-10 also settles the tag form for the server module and corrects the rationale in 18 (the plan's risk on `services/server/` tags). Why: every later release (product bundle M7-29, host service M7-23, plugin M7-26, apps M7-12) builds on this chain; signatures, SBOM and checksums are owner requirements, and the chain must have run once for real before the owner sets the first tag.

## Read first

- `docs/spec/18-release.md` (all: 2, 2.1, 3, 4, 5, 6, 7, 11, 13), `docs/operations/release.md` (all, section 5 gets the exact private-phase verification command).
- `docs/adr/0036-releases-semver-per-component-signed-sbom-provenance.md`, `docs/adr/0060-release-automation-in-the-monorepo.md` (all, "Confirmation"), `docs/adr/0062-release-signatures-while-the-repo-is-private.md` (all), `docs/adr/0039-only-github-hosted-runners-public-from-the-first-push.md`, ADR 0041 (pins).
- `.github/workflows/release.yml` (jobs `resolve`, `gates`, `build`, `publish`, `portability`), `tools/ci/components.py` (`release`, `pins`), `tools/ci/check_runners.py` (rules on release jobs and environments), `tools/bin/tools.lock.json` and `make tools-lock`, `docs/quality/toolchain.md`, `docs/quality/licenses.md` (ScanCode before every release), `docs/quality/public-switch.md` (keyless switch), `docs/quality/push-discipline.md`.
- `docs/RISKS.md` (RK-57), `docs/OPEN-QUESTIONS.md` (OQ-12: placeholder `OWNER`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125), the trial release (D-W-127), D-W-52, D-W-76, D-W-95, D-W-98, D-W-110, D-W-111; section 10 (format of `D-M7-10-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-23-server-image.md` (`make image`, Dockerfile rules, `trivy-image`), `M7-01-postgres-image.md` (`make postgres-image`, per-architecture build).
- Code on the base branch: `Makefile` (`image`, `release-build`, `trivy-image`, `licenses`, `tools-lock`), `services/server/Dockerfile`, `services/server/images/bothy-postgres`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; the repository is written `OWNER/bothy`; no company, host or personal names.
9. Every tool is pinned with a checksum (`tools.lock.json`) or by digest (images, actions); no moving tag.
10. Specification first: a change to the release rules updates `docs/spec/18-release.md` and `docs/operations/release.md` in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): GitHub-hosted standard runners (free for public repositories) and local throwaway targets only.
12. Never tag, never create a release, never publish to a registry (D-W-125). The trial release uses only local targets; the first real tag of `server` is an owner step after the merge.

Product requirements implemented here:

13. Images are built per architecture without emulation (18 7 rule 2) and joined into one multi-arch manifest per image; the version tag is set only after the vulnerability scan and the ScanCode run passed.
14. Every image digest, every binary and `SHA256SUMS` is signed; in the private phase with the key pair and a signing config without Rekor, Fulcio and timestamp service, in the public phase keyless with attestations (`github.event.repository.visibility`); signing steps only in jobs with `environment: release` (ADR 0062 confirmation, `make runners-check` holds it).
15. A missing SBOM, a `VERSION` other than the tag, or a missing changelog section aborts before anything is published.
16. `make release-prep` follows ADR 0060 items 1 to 5 exactly; it writes nothing on an error.

## Scope

- `.github/workflows/release.yml`: the server path builds `bothy-server` (existing step) and `bothy-postgres` in a matrix of `ubuntu-24.04` and `ubuntu-24.04-arm`, joins manifests, scans each digest (`make trivy-image`), runs ScanCode (Design note 3), creates SBOMs with Syft (pinned action or binary), signs (Design note 2), then tags; `publish` attaches SBOMs, checksums and signature bundles.
- Component Makefile `services/server/Makefile` with `release-build VERSION=<v> OUT=<dir>` (binary `bothy` per platform, image builds through the root targets) as the component contract of 23 3 requires.
- `Makefile`: targets `release-prep` (calls `tools/ci/release_prep.py`) and `release-trial` (Design note 1).
- `tools/ci/release_prep.py` with tests (`tools/ci/tests/test_release_prep.py`) against a fixture repository built in a temporary directory.
- `tools/ci/release_trial.py` (or a shell script under `tools/ci/`) with tests.
- Pins: git-cliff, cosign and syft in `tools/bin/tools.lock.json` (`make tools-lock`), the local registry image by digest; `docs/quality/toolchain.md` rows and `test_toolchain_doc.py` stay green; a `cliff.toml` per ADR 0060.
- `tools/ci/check_runners.py`: the addition of ADR 0062 confirmation (signing steps only in jobs with `environment: release`) with tests, if the rule is not there yet.
- Public key file of the private phase: the path is fixed in the ruling (for example `quality/release/cosign-private-phase.pub`); this ticket does not create the owner's real key pair (Design note 2).
- Docs: `docs/spec/18-release.md` 2 (tag form rationale, Design note 4), 6 and 7 as built; `docs/operations/release.md` 2.1 and 5 (exact commands for the private and the public phase, the trial release); `docs/RISKS.md` RK-57 (mitigation now the trial release); `CHANGELOG.md` and `services/server/CHANGELOG.md` under `[Unreleased]`; `docs/DECISIONS.md` section 10 rows `D-M7-10-<n>`.

## Out of scope

- The product bundle, `install.sh` and `release-build` of the component `bothy` (M7-29); the portability test (M7-14); app distribution (M7-12, M7-13); the host service release (M7-23); the plugin release (M7-26).
- Creating the owner's signing key pair and the GitHub environment secrets (owner step; the runbook names it).
- Any real tag, release or registry push.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-10-server-release-chain`; one writer per worktree.
- Conventional Commits with the component as scope (for example `ci(release): ...`, `feat(tools): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). The local registry binds only `127.0.0.1` on a random port and is removed at the end.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; the trial key pair is generated in the temporary directory and deleted with it.
- Docker with buildx must be available for `make release-trial`; run it once on the build host or stop with `Verdict: BLOCKED docker missing`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Trial release** (D-W-127). `make release-trial COMPONENT=server` (other components join in their tickets): start a pinned registry container on `127.0.0.1:0`; build both images for both platforms (the build host may build the foreign platform only for the server binary image, which cross compiles; `bothy-postgres` for the foreign platform comes from the CI job of M7-01, so the trial accepts `TRIAL_PLATFORMS` and defaults to the host platform for `bothy-postgres`); push by digest; create the manifests; generate a throwaway cosign key pair; sign with the private-phase signing config; Syft SBOMs; `SHA256SUMS`; sign it; then verify everything as an instance operator would (`cosign verify --key`, `cosign verify-blob`, `sha256sum --check`) and print the digests. Negative cases in the tests: without an SBOM the trial aborts; a changed file fails the checksum; an unsigned digest fails verification. The target refuses to run when `GITHUB_ACTIONS` is set, so it never runs with release credentials.

2. **Signing paths** (ADR 0062). Private: `cosign signing-config create` without default Rekor, Fulcio, TSA and OIDC; `cosign sign --key env://...` with the key from the environment `release`; verification with `--key` and `--insecure-ignore-tlog` (check the flags of the pinned cosign version and write the exact commands into `release.md` 5). Public: keyless with the workflow identity and `actions/attest`. The trial proves the private path; the keyless path can only run in GitHub Actions with OIDC and is proven at the first real public release (report it as not proven).

3. **ScanCode** (licenses.md 4). ScanCode Toolkit as a container pinned by digest over the tree; the report is a release artifact; a finding outside the allowed licenses of `quality/licenses/allowed.txt` stops the release before tags.

4. **Tag form** (18 2 rule 2; plan risk). Keep `server/vX.Y.Z` (the module is a main module that is never loaded, P-05); correct the rationale text in 18 so that it states the Go rule as written in the source and why it does not apply; record the ruling.

5. **`release-prep`** (ADR 0060). git-cliff `--bumped-version` with `--include-path` per component and `--tag-pattern` per prefix; before 1.0.0 a break bumps the minor; `CHANNEL=beta` appends `-beta.N` with N from existing tags; the changelog section `Unreleased` becomes `[X.Y.Z] - <date>`; `VERSION` written; the checklist of commits of the component without a change to its `CHANGELOG.md` printed. Fixture repository tests cover patch, minor and breaking change before and after 1.0.0, the beta counter, commits of other components, and a commit without a changelog change.

6. **Rulings for `docs/DECISIONS.md`** (`D-M7-10-<n>`): trial release shape (note 1), signing commands (note 2), ScanCode gate (note 3), tag form (note 4), the public key path.

## Tests (write them first)

- `tools/ci/tests/test_release_prep.py`: the cases of ADR 0060 confirmation (Design note 5).
- `tools/ci/tests/test_release_trial.py`: argument checks, refusal under `GITHUB_ACTIONS`, abort without an SBOM, checksum and signature negatives (with fakes of the tools where a container is not needed).
- `tools/ci/tests/test_check_runners.py`: signing steps outside a job with `environment: release` are a finding.
- A test that `release.yml` sets version tags only after the scan and ScanCode steps (parse the workflow).

Commands that must pass, from the repository root:

- `uv run --project tools --locked pytest -q tools/ci/tests`
- `make release-trial COMPONENT=server` on the build host
- `make lint`, `make runners-check`, `make licenses`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `cosign verify` and `sha256sum --check` against the trial release are green; a missing SBOM aborts; `VERSION` equals the tag in the workflow check.
- [ ] The ScanCode report is an artifact; tags are set only after scan and ScanCode.
- [ ] The trial publishes every image that `infra/compose/compose.yaml` names with `${BOTHY_VERSION}` under exactly this version (`bothy-server:<version>`, `bothy-postgres:pg18-<version>`) in the local registry.
- [ ] `make release-prep` passes the tests of ADR 0060 confirmation against the fixture repository.
- [ ] git-cliff, cosign and syft are pinned; `toolchain.md` matches.
- [ ] 18, the runbook, RK-57, the CHANGELOGs and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `docs/operations/release.md` names the exact verification commands for both phases.
- Rulings of Design note 6 recorded in `docs/DECISIONS.md` section 10.
- `CHANGELOG.md` entries under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make release-trial COMPONENT=server` with the digests.
3. Review and red team verdicts and the findings fixed.
4. The exact verification commands of both phases as written into the runbook.
5. What is not proven (keyless signing and attestations, a push to the real registry; both come with the owner's first tag).
6. Owner steps before the first tag (key pair, environment secrets), each with a recommendation; follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
