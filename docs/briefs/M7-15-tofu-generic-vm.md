# M7-15: OpenTofu module `generic-vm`

Status: ready | Depends on: M7-29 | Milestone: M7

## Goal

After this ticket the module `infra/tofu/modules/generic-vm` sets up a Linux host for Bothy from its inputs (ADR 0012, `infra/tofu/README.md`): it renders cloud-init user data that installs Docker Engine and the Compose plugin from the Docker package repositories, creates the user `bothy` and the stack directory, downloads the product bundle of `bothy_version` with its signature and checksum, verifies them (18 6), runs the setup wizard `install.sh --bundle` without a terminal with options derived from the inputs (the same path as every installation, 16 3.1), installs a system service that starts the stack at boot, and returns the outputs of the module contract. Secrets are never inputs, outputs or part of the rendered user data; they are generated on the host by the wizard (`infra/tofu/README.md` rule 1). `tofu validate` is green, and a test boots a local VM without any cloud cost (QEMU with a pinned cloud image and a cloud-init seed), applies the user data, and passes the smoke test against the stack started from the local trial release. Why: the cloud modules (M7-16) only create network, host and storage and then call this module; the host setup must be one tested path, and portability means the same images on any host (16 1 goal 2).

## Read first

- `infra/tofu/README.md` (structure, outputs, rules 1 to 5, "Checked"), `infra/tofu/modules/generic-vm/README.md` and `variables.tf` (the shared inputs and `ssh_host`, `ssh_user`; "Open": the tool choice).
- `docs/spec/16-operations.md`: 1, 3 (targets, rule 7), 3.1 (options of `install.sh`), 4.1 to 4.3 (edges), 5.3, 11, 14; `docs/spec/18-release.md` 2 (bundle), 2.1, 6 (verification of files).
- `docs/adr/0012-oci-images-compose-opentofu.md`, `docs/operations/install-compose.md`, `docs/operations/cloud-migration.md`, `docs/OPEN-QUESTIONS.md` OQ-09.
- `tools/bin/tools.lock.json` (`tofu` is pinned), `docs/quality/toolchain.md`, `Makefile` (`trivy` scans Terraform files).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125), the trial release (D-W-127); section 10 (format of `D-M7-15-<n>` rows).
- Briefs (merged names win, D-W-100): `M7-29-setup-wizard.md` (`install.sh` options, `--bundle`, the product trial), `M7-10-server-release-chain.md` (trial registry, verification commands), `M1-73-compose-smoke.md` (`infra/compose/smoke.sh`).

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
8. White label: examples use `chat.example.com`, the owner `alex`; no company, host or personal names, no real IP addresses outside the documentation ranges.
9. Provider versions, the cloud image and every tool are pinned (checksum or digest); Renovate keeps them current.
10. Specification first: a change to the module contract updates `infra/tofu/README.md` and the module README in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): no cloud account; the VM test runs locally or on a GitHub-hosted standard runner.
12. Never tag, never create a release, never publish, never apply against a provider account (D-W-125).

Product requirements implemented here:

13. No secret in inputs, outputs, state or rendered user data; a test scans the rendered user data and the plan output for every secret name of the catalogue and for key material.
14. The bundle and its signature are verified on the host before anything is unpacked; a wrong checksum or signature stops the setup.
15. The host setup calls `install.sh --bundle` with options derived from the inputs, so the module never writes `.env` or `secrets/` itself; `edge_mode` maps to the options of 16 3.1 (`direct` behind the host's reverse proxy, `tailscale`, or no public port for `private`).
16. Only the edge port is public (`infra/tofu/README.md` rule 3); the rendered firewall rules or Compose publishing say so.
17. A second apply on the same host changes nothing (the wizard's rule 1 and idempotent cloud-init parts).

## Scope

- `infra/tofu/modules/generic-vm/`: `main.tf`, `outputs.tf`, `versions.tf` (pinned providers, for example `cloudinit` and `null` or `terraform_data`), templates of the user data and the system service, the README (inputs, outputs, the tool choice of "Open").
- `infra/tofu/test/generic-vm/`: a root module for the test and the VM runner (Design note 2).
- `Makefile`: target `tofu-check` (`tofu fmt -check`, `tofu init -backend=false`, `tofu validate` over a root that calls all four modules; in `make check`, offline with a provider mirror or a cached plugin directory as the toolchain allows) and target `tofu-vm-test` (the local VM test, outside `check`).
- `.github/workflows/ci.yml`: no job for the VM test. `make tofu-vm-test` runs only on the build host, because the product trial it starts refuses to run under `GITHUB_ACTIONS` (D-W-127); CI covers the modules through `tofu-check` in `make check`.
- Docs: `infra/tofu/README.md` (status column, "Checked"), `docs/operations/cloud-migration.md` where it names the module, `docs/quality/gates.md`, `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-15-<n>`.

## Out of scope

- The cloud modules `hetzner`, `aws`, `azure` (M7-16) and any live test in a cloud (OQ-09).
- A managed database or object storage; Kubernetes (V3).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-15-tofu-generic-vm`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(tofu): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories or cloud resources; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). The VM uses user-mode networking; forwarded ports bind only `127.0.0.1` on random ports.
- No network calls except downloads of pinned tools, the pinned cloud image (verified by checksum) and the package repositories the user data names inside the VM.
- Stop only processes you started (the VM by PID). Temporary directories only via `mktemp -d`, removed at the end with the VM disk.
- Never read or print secrets.
- QEMU must be available; run `make tofu-vm-test` once on the build host (install QEMU with a package manager if it is missing), or stop with `Verdict: BLOCKED qemu missing`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Tool choice** ("Open" of the module README; Ruling for docs/DECISIONS.md). New hosts get cloud-init user data (the cloud modules pass it to their VM resource); existing hosts get the same script over SSH from a `terraform_data` provisioner with `ssh_host` and `ssh_user`. One script, two delivery paths.

2. **Local VM test** (OQ-09). Run the trial release of the product (M7-29) on the build host, serve bundle, signature and `install.sh` on a local static server, render the user data with the test root (download address pointed at the host side of QEMU user networking), boot a pinned Ubuntu or Debian cloud image with a NoCloud seed in QEMU (KVM or HVF where available, TCG otherwise; record the time), wait for cloud-init, then run the smoke test of M1-73 against the forwarded port. The VM pulls images from the trial registry on the host through user networking; configure that registry as insecure only inside the test VM and record it. A second apply run inside the VM changes nothing.

3. **Rulings for `docs/DECISIONS.md`** (`D-M7-15-<n>`): tool choice (note 1), VM runner and image pin (note 2), the offline `tofu-check` approach.

## Tests (write them first)

- `tofu validate` over the test root that calls all four modules (`make tofu-check`).
- A script test (Python or shell) that renders the user data for each `edge_mode` and asserts: no secret names or key material, the verification step before unpacking, the options passed to `install.sh`, only the edge port public.
- `make tofu-vm-test`: the VM comes up, the stack is ready, the smoke test passes, a second run changes nothing; a bundle with a wrong checksum stops the setup.

Commands that must pass, from the repository root:

- `make tofu-check`
- `make tofu-vm-test` on the build host
- `make trivy`, `make lint`, `make runners-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `tofu validate` is green; `make tofu-check` runs in `make check`.
- [ ] A test on a local VM without cloud costs starts the stack and passes the smoke test.
- [ ] No secret in inputs, outputs, state or user data; bundle and signature are verified before unpacking.
- [ ] READMEs, `gates.md`, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; comments explain why; every variable and output has a description.
- Module README and `infra/tofu/README.md` updated in English.
- Rulings of Design note 3 recorded in `docs/DECISIONS.md` section 10.
- `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make tofu-vm-test` with the acceleration used and the duration.
3. Review and red team verdicts and the findings fixed.
4. The rendered user data of one `edge_mode` (without values that would identify a host).
5. What is not proven (real cloud hosts).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
