# M7-29: Setup wizard `install.sh` (installation in one command)

Status: ready | Depends on: M1-73, M7-10 | Milestone: M7

## Goal

After this ticket a stranger installs an instance with exactly one documented command (16 1 goal 1, 3.1, R-OP-20): the POSIX shell script `infra/compose/install.sh` runs the seven steps of 16 3.1 (preflight check, bundle with checksum by download or `--bundle <file>`, prompts through `/dev/tty` or options, `.env` and `secrets/`, with the edge `direct` the gateway of the Compose network in `BOTHY_TRUSTED_PROXIES`, start with readiness, the check of `GET /api/v1/instance/discovery` through the published port, the first owner, the hints) and obeys its five rules (a second run overwrites neither `.env` nor `secrets/`, no `sudo`, Linux and macOS, every abort with a non-zero code and the step on standard error, the documented options as the single interface of the portability test). `make release-build` of the component `bothy` produces the bundle and the separate artifact `install.sh` as 18 2.1 rules 1 to 4 require: `BOTHY_VERSION` from the commit, `BOTHY_REGISTRY=ghcr.io/<owner>` in lower case from `GITHUB_REPOSITORY_OWNER`, version, SHA-256 and download address `https://github.com/<GITHUB_REPOSITORY>/releases/download/v<version>/` built into `install.sh`, and an abort without output on every placeholder and missing value. `make release-prep COMPONENT=bothy SERVER_VERSION=<version>` writes the line `BOTHY_VERSION=` with its three refusals (ADR 0060 item 6). `tests/portability/install-options` is the one place of the options the portability test passes, held equal to the second block of 16 3.1 by a check in `make check`. ShellCheck runs in the gate. The trial release of the product (`make release-trial COMPONENT=bothy`, D-W-127) builds bundle and `install.sh` against the local trial registry. Why: one command is the promise of the product to instance operators; every rule of 16 3.1 protects them from a script that changes more than its folder or leaks a secret.

## Read first

- `docs/spec/16-operations.md`: 1, 3 (rule 7: minimum versions), 3.1 (all steps and rules), 4.3 (`direct` and `BOTHY_TRUSTED_PROXIES`), 5.3, 5.4, 11.1 (interface of `make portability`, rules 1 to 5), 14 (`install.ready_timeout`), 15 (R-OP-20).
- `docs/spec/18-release.md`: 2 (component `bothy`), 2.1 (all rules), 5, 6 (signing of files), 13 (R-RL-09).
- `docs/adr/0060-release-automation-in-the-monorepo.md` (item 6), `docs/operations/install-compose.md` (the manual path, sections 1 to 4), `docs/operations/release.md` 2.2, `docs/quality/public-switch.md` (step 21, V1-03 replaces `OWNER`).
- `infra/compose/compose.yaml`, `infra/compose/.env.example`, `Makefile` (`release-build`, `release-pins`, `portability`), `tools/ci/components.py` (`pins`), `.github/workflows/release.yml` (job `build` for the component `bothy`).
- `docs/spec/08-identity.md` 4 (`bothy setup bootstrap-owner`, enrollment code shown once).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125), the trial release (D-W-127), D-W-76, D-W-95, D-W-98, D-W-99; section 10 (format of `D-M7-29-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-73-compose-smoke.md` (`infra/compose/smoke.sh`, `test_release_bundle_excludes_smoke_overlay`, overlays under `infra/compose/test`), `M7-10-server-release-chain.md` (`release-prep`, `release-trial`, signing of files).

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
8. White label: examples use `https://chat.example.com`, the owner handle `alex` and the name `Alex`; the repository is `OWNER/bothy` until V1-03; no company, host or personal names.
9. Timeouts and versions only through the named values of 16 14 and 18 2.1; code comments name the value, never a number.
10. Specification first: a change to the steps, options or bundle rules updates `docs/spec/16-operations.md` 3.1 and 11.1 and `docs/spec/18-release.md` 2.1 in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125).
12. Never tag, never create a release, never publish to a registry (D-W-125); the trial uses the local registry of M7-10 only.

Product requirements implemented here (each is a test in Scope):

13. ShellCheck without a finding; POSIX `sh` only.
14. Missing or too old Docker or Compose (16 3 rule 7) aborts before any write; a wrong bundle checksum aborts, also with `--bundle`; without a terminal and without a required value the script aborts naming the option; the script never calls `sudo`.
15. A second run overwrites neither `.env` nor `secrets/`; `.env` has mode `0600` and contains no secret except `TS_AUTHKEY`; `secrets/` has mode `0700`; the enrollment code appears only on the terminal, in no file of the target directory.
16. `--storage s3` sets the profile `s3` and `BOTHY_STORAGE_DRIVER=s3`.
17. With the edge `direct` the gateway of the Compose network is in `BOTHY_TRUSTED_PROXIES` as `/32` (IPv6 `/128`); a second run leaves a value set by the instance operator in place; if `/api/v1/instance/discovery` does not answer `200` through the published port, the script aborts with a hint to `BOTHY_TRUSTED_PROXIES`.
18. `BOTHY_VERSION` and `BOTHY_REGISTRY` in `.env` are the values of the bundle's `.env.example`, not the version of `install.sh` (16 3.1 step 4).
19. The version in the documented command in `README.md` and `docs/operations/install-compose.md` equals `VERSION`; `tests/portability/install-options` holds exactly the options after `sh -s --` of the second block of 16 3.1, only `--storage` and `--bundle` may differ; the check compares the options, not the source of the script (both checks in `make check`).
20. `make release-build` of `bothy` (18 2.1) against a fixture repository and a registry stub: `BOTHY_VERSION` is the value of the commit, `BOTHY_REGISTRY` equals `ghcr.io/<owner>` in lower case, every other file equals the commit; one case each aborts without a bundle and names the value for `0.0.0-placeholder`, a version without SemVer form, a server pre-release in a stable product, a missing `GITHUB_REPOSITORY_OWNER`, `OWNER` in the result and an image from `compose.yaml` missing in the registry; it sets the download address from `GITHUB_REPOSITORY` also when the source names `OWNER`, and aborts without `GITHUB_REPOSITORY` and on `OWNER` or a placeholder left in the finished `install.sh`; an `install.sh` from the fixture downloads its bundle from exactly this address (download stub).
21. `make release-prep COMPONENT=bothy SERVER_VERSION=<version>` writes the line `BOTHY_VERSION=` and aborts without a written file and naming the value without the tag `server/v<version>`, without `CHANNEL=beta` for a version with a pre-release identifier, and without `SERVER_VERSION` while the line carries the placeholder (18 2.1 rule 1).
22. `test_release_bundle_excludes_smoke_overlay` of M1-73 stays green against the fixture repository.

## Scope

- `infra/compose/install.sh` (POSIX shell, steps and rules of 16 3.1, options of 16 3.1 plus `--dir`, `--bundle`, `--ready-timeout`, `--yes`).
- Tests in `infra/compose/test/install/` (bats pinned by digest in a container, or POSIX shell tests in a container) with stubs for `docker`, `docker compose`, `curl`, `cosign` and `sudo` (a `sudo` stub that fails the test when called).
- `Makefile`: `release-build` of the component `bothy` per 18 2.1 (Design note 2); `release-trial COMPONENT=bothy` extends the target of M7-10 (Design note 3).
- `tools/ci/release_prep.py`: the option `SERVER_VERSION` for `bothy` with tests in `tools/ci/tests/`.
- `tests/portability/install-options` (the only place of the options) and the two checks of requirement 19 (`tools/ci/install_options_check.py` with tests, in `make check`).
- `.github/workflows/release.yml`: the product path builds bundle and `install.sh`, signs both like every artifact (M7-10), and logs in to `ghcr.io` also for the component `bothy`, so that `release-build` can check the images.
- ShellCheck pinned (`tools-lock`) and run over `infra/compose/*.sh` in `make lint`.
- `infra/compose/.env.example` (comments only, values stay placeholders), `README.md` and `docs/operations/install-compose.md` (the documented command with the version), `docs/spec/16-operations.md` 3.1 and 11.1 and `docs/spec/18-release.md` 2.1 as built, `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-29-<n>`.

## Out of scope

- The portability test itself (M7-14); replacing `OWNER` (V1-03).
- Windows outside WSL 2 (16 3.1 rule 3: the script is verified there only as the build host allows; report it).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-29-setup-wizard`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(compose): ...`, `ci(release): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). Script tests use stubs; a real installation (in the trial) publishes only on `127.0.0.1` and is removed at the end with its volumes.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; the tests assert that the enrollment code and generated secrets never reach a file outside `secrets/`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Script structure** (16 3.1). One function per step, each prefixed in its error messages (`install.sh: preflight: ...`); state only in the target folder; the prompts read from `/dev/tty` only when it is a terminal; options always win over prompts; `--yes` takes defaults but never invents the public address, the owner handle or the owner name.

2. **`release-build` of `bothy`** (18 2.1 rules 2 to 4). In order: read and check `BOTHY_VERSION` of the commit; check `GITHUB_REPOSITORY_OWNER` and `GITHUB_REPOSITORY`; list the images of `compose.yaml` with `${BOTHY_VERSION}` and check each exists under the registry (`docker manifest inspect` or `crane` pinned; the stub in tests); write the bundle with only the registry line changed; render `install.sh` with version, checksum and address; scan both for `OWNER` and placeholders; on any failure delete partial output and exit non-zero naming the value.

3. **Trial of the product** (D-W-127). `make release-trial COMPONENT=bothy` runs the server trial of M7-10 first, then `release-build` with the trial registry: a variable `RELEASE_TRIAL_REGISTRY` replaces `ghcr.io/<owner>` and is accepted only when `GITHUB_ACTIONS` is unset (a test asserts the refusal), the trial download address points at a local static server; the result is a bundle and an `install.sh` that install from the local trial registry, used by M7-14 for its local end-to-end run. Record the variable in 18 2.1 as trial-only.

4. **Rulings for `docs/DECISIONS.md`** (`D-M7-29-<n>`): the test framework for the script, the image existence check tool, the trial registry variable (note 3), the WSL 2 verification state.

## Tests (write them first)

- `infra/compose/test/install/`: one test per item of requirements 13 to 18 (Docker stub, download stub, `sudo` stub, terminal and no terminal, second run, IPv4 and IPv6 gateway, discovery not `200`).
- `tools/ci/tests/test_release_build_bothy.py` (fixture repository, registry stub): every case of requirement 20.
- `tools/ci/tests/test_release_prep.py`: the cases of requirement 21.
- `tools/ci/tests/test_install_options_check.py`: equal options pass; a changed option fails; differing `--storage` and `--bundle` pass; a changed script source alone does not matter.

Commands that must pass, from the repository root:

- `uv run --project tools --locked pytest -q tools/ci/tests`
- the script tests (the target the build adds to `make test`, documented in `docs/quality/gates.md`)
- `make release-trial COMPONENT=bothy` on the build host
- `make lint`, `make release-pins`, `make runners-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] Every item of requirements 13 to 22 has a passing test.
- [ ] `make release-build` of `bothy` writes bundle and `install.sh` exactly per 18 2.1 and aborts on every listed case without output.
- [ ] `tests/portability/install-options` exists and the two checks hold it equal to 16 3.1 and the documented command to `VERSION`.
- [ ] The trial release of the product produces an `install.sh` that installs from the local trial registry.
- [ ] 16, 18, the install guide, README, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol and a header comment per shell function (purpose, arguments, exit codes); comments explain why.
- `docs/operations/install-compose.md` and `README.md` show the one command.
- Rulings of Design note 4 recorded in `docs/DECISIONS.md` section 10.
- `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of the product trial.
3. Review and red team verdicts and the findings fixed.
4. The documented command as it now stands in the README.
5. What is not proven (the public `curl` path, WSL 2).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
