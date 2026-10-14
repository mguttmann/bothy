# M7-14: Portability test in CI with the full stack

Status: ready | Depends on: M2-29, M7-03, M7-09, M7-29 | Milestone: M7

## Goal

After this ticket `make portability` with `PORTABILITY_TAG` runs the portability test of 16 11.1 on a fresh machine: it reads `BOTHY_REGISTRY` and `BOTHY_VERSION` from the bundle of the tag, pulls every image that the bundle's `compose.yaml` names with `${BOTHY_VERSION}`, verifies the signature of each digest with cosign run as a container with a digest pin (the public key of the private phase from the repository, or keyless with the identity of the release workflow), fails before `install.sh` runs when an image is unsigned, installs with exactly the options of `tests/portability/install-options` (once `--storage fs`, once `--storage s3`), checks that the containers run exactly the verified images, then runs the smoke tests through the public API (owner from the wizard, space, channel, thread, message, WebSocket reception, file upload clean and with the EICAR test file, a run of a test adapter, search, status page), triggers a backup and a restore test with all checks green, receives telemetry through the overlay `local` without finding the privacy canary, and reports the digests. On failure it shows `docker compose ps --all` and the logs of exactly its installation before it cleans up. The interface is the one of 16 11.1 (`PORTABILITY_TAG`, `PORTABILITY_INSTALL_SOURCE` with `url` or `release-download`, `PORTABILITY_RELEASE_DIR`); an invalid value aborts before anything is downloaded, pulled or installed and names the variable; without `PORTABILITY_TAG` the smoke test of M1-73 stays as it is. `portability.yml` runs it on `ubuntu-24.04` and `ubuntu-24.04-arm` and `release.yml` calls it only for product tags after a successful `publish`. Because no product tag exists yet and tags stay with the owner (D-W-125), this ticket proves the whole test end to end against the local trial release of M7-29 (`make portability-trial`, source `release-download`); the status check on the first real release candidate follows the owner's first product tag. Why: exit criterion 3 of M7 and the promise "any host with the same images" (16 1 goal 2) need one test that installs exactly like a stranger and verifies what it runs.

## Read first

- `docs/spec/16-operations.md`: 3.1 (options, sources table), 8.3, 11.1 (steps 1 to 6, interface table, rules 1 to 5), 12, 14.
- `docs/spec/18-release.md` 2.1 (bundle values, rules 2 to 4), 6 (verification), 7; `docs/spec/17-observability.md` (privacy canary), `docs/spec/12-files.md` (scan states, `infected`).
- `docs/operations/release.md` 2.2 and 5, `docs/quality/public-switch.md` (step 21: the public `curl` path), ADR 0039, ADR 0041, ADR 0062.
- `.github/workflows/portability.yml`, `.github/workflows/release.yml` (job `portability`), `Makefile` (`portability`), `infra/compose/smoke.sh`, `infra/compose/compose.yaml` (profiles `direct`, `scan`, `s3`, overlay `local`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125), the trial release (D-W-127), D-W-99; section 10 (format of `D-M7-14-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-73-compose-smoke.md` (smoke test, canary scan, cleanup), `M1-69-canary-test.md`, `M3-04-clamd-scan.md` (EICAR), `M7-03-restore-test.md`, `M7-10-server-release-chain.md` (trial release, verification commands), `M7-29-setup-wizard.md` (`install.sh`, `install-options`, trial of the product, `RELEASE_TRIAL_REGISTRY`), `M2-29-python-sdk.md` (the test double of the SDK for the test adapter run).

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
8. White label: the test uses `https://chat.example.com`, the owner `alex` (`Alex`) and the people Sam and Robin; no company, host or personal names.
9. cosign, the registry and every helper image are pinned by digest (ADR 0041); options of `install.sh` only from `tests/portability/install-options` (16 11.1 rule 2).
10. Specification first: a change to the flow updates `docs/spec/16-operations.md` 11.1 in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): GitHub-hosted standard runners and local throwaway installations only; no real model call (the test adapter is the SDK's test double).
12. Never tag, never create a release, never publish (D-W-125); the run against a real tag happens only after the owner's first product release.

Product requirements implemented here:

13. Invalid interface values abort before any download, pull or install and name the variable; `PORTABILITY_RELEASE_DIR` is read only with `release-download`; without `PORTABILITY_TAG` the M1-73 smoke test runs unchanged.
14. Source `url` builds the URL of `install.sh` and of the bundle from `GITHUB_REPOSITORY` and `PORTABILITY_TAG`, never from documentation of the tag (16 11.1 rule 3); the job logs in to no registry in the public phase; in the private phase the image pull logs in only where 16 11.1 step 1 allows it.
15. A case with an unsigned image fails before `install.sh` runs; after the installation the containers use exactly the verified digests, otherwise the run fails; the result names the digests.
16. On failure the test prints `docker compose ps --all` and the end of the logs of exactly its installation (its folder and project name) before cleanup; `portability.yml` has no log step of its own (16 11.1 rule 5).
17. The installation Compose file sets only keys the configuration schema knows (16 "Configuration (ADR 0011)" item 3); in the tailnet case only the exposed port answers.

## Scope

- `tests/portability/` (the test runner, a shell or Python program with tests, and the smoke steps through the public API; it reuses the probe tools of M1-73 where they fit).
- `Makefile`: `portability` dispatches on `PORTABILITY_TAG` (16 11.1 interface); new target `portability-trial` (Design note 2).
- `.github/workflows/portability.yml`: the matrix as it is, the environment of the interface, the registry login only where step 1 allows it; `release.yml` keeps calling it only for product tags after `publish` and by hand via `workflow_dispatch`.
- `infra/compose/test/` overlays the test needs (never in the bundle).
- Docs: `docs/spec/16-operations.md` 11.1 as built, `docs/operations/release.md` 2.2 step 5, `docs/quality/public-switch.md` step 21 (the public `url` path stays its proof), `docs/quality/gates.md`, `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-14-<n>`.

## Out of scope

- The capacity load test (M7-18), VZ-T13 for the Grafana plugin (M7-26 adds it to this test), OpenTofu targets (M7-15, M7-16).
- The public `curl` path without login (public-switch step 21).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-14-portability-test`; one writer per worktree.
- Conventional Commits with the component as scope (for example `test(portability): ...`, `ci: ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). The installation publishes only on `127.0.0.1`; the trial registry and download server bind only `127.0.0.1`.
- Stop only processes and containers you started, by PID or container ID; the cleanup removes the installation's containers, volumes, networks and folder, and nothing else.
- Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; the enrollment code and the generated secrets never reach the logs the test prints.
- Docker must be available; run `make portability-trial` once on the build host or stop with `Verdict: BLOCKED docker missing`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Order** (16 11.1). Validate the three variables; resolve sources; download or read `install.sh` and the bundle; read `BOTHY_REGISTRY` and `BOTHY_VERSION` from the bundle's `.env.example`; pull the images; verify each digest; run `install.sh` with the options file (source `url`: the documented `curl` command; `release-download`: `sh install.sh --bundle <bundle>` with the same options); compare running digests; smoke steps; backup and restore test (`bothy-backupd backup` and `restore-test` of M7-02 and M7-03 in the `backup` container); canary scan; report; cleanup. Run twice: `--storage fs` (profiles `direct,scan`) and `--storage s3` (profiles `direct,scan,s3`).

2. **Local proof with the trial** (D-W-127). `make portability-trial` builds the trial release of the product (M7-29, which builds the server trial of M7-10), serves the bundle and `install.sh` on a local static server, and runs `make portability` with `PORTABILITY_TAG=v<version of the trial>`, `PORTABILITY_INSTALL_SOURCE=release-download` and `PORTABILITY_RELEASE_DIR` of the trial; the signature check uses the trial's public key passed through a trial-only variable that the test refuses when `GITHUB_ACTIONS` is set. Docker treats a registry on `127.0.0.1` as plain HTTP without extra configuration; record the evidence. The negative case (one image of the trial unsigned) must fail before `install.sh` runs.

3. **Tailnet case** (16 11.1, "in the tailnet test only the exposed port answers"). Run the `tailscale` profile without a real tailnet: assert from inside the network namespace that only the exposed port listens and that the server listens on loopback only (R-OP-03); a real tailnet needs an account and is not part of the test (record it).

4. **Rulings for `docs/DECISIONS.md`** (`D-M7-14-<n>`): the order (note 1), the trial path and its variables (note 2), the tailnet check without an account (note 3).

## Tests (write them first)

- Unit tests of the runner (Python with pytest or shell tests in a container): every invalid interface value aborts before any download and names the variable; `url` builds the expected addresses from `GITHUB_REPOSITORY` and the tag; the options come only from the options file; the trial variables are refused under `GITHUB_ACTIONS`.
- `make portability-trial` on the build host: both storage runs green, EICAR becomes `infected`, backup and restore test green, canary not found, digests reported; the unsigned negative case fails before `install.sh`; a forced failure prints state and logs of the installation before cleanup.

Commands that must pass, from the repository root:

- the runner's unit tests (in `make test` through `tools/` or `tests/portability`)
- `make portability-trial` on the build host
- `make portability` without `PORTABILITY_TAG` (the M1-73 smoke test stays green)
- `make lint`, `make runners-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `make portability` follows the interface of 16 11.1; invalid values abort early and name the variable; without `PORTABILITY_TAG` the smoke test of M1-73 stays green.
- [ ] Against the local trial release: installation without a terminal with exactly the documented options, `/readyz` green and `GET /api/v1/instance/discovery` `200` through the published port, for `fs` and `s3`; EICAR `infected`; backup and restore test green; canary not found; digests reported; an unsigned image fails before `install.sh`.
- [ ] `portability.yml` and `release.yml` call the test as 16 11.1 describes; `make runners-check` green.
- [ ] 16, the runbooks, `gates.md`, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `tests/portability/README.md` written in English.
- Rulings of Design note 4 recorded in `docs/DECISIONS.md` section 10.
- `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make portability-trial` with the digests and the duration per storage run.
3. Review and red team verdicts and the findings fixed.
4. The architecture of the build host; which run proves the other architecture (the first real product release).
5. What is not proven (the status check on a real release candidate on amd64 and arm64, the public `curl` path, a real tailnet).
6. Owner steps with a recommendation (the first product beta tag that starts the real test); follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
