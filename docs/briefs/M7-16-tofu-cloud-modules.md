# M7-16: OpenTofu modules `hetzner`, `aws`, `azure`

Status: ready | Depends on: M7-15 | Milestone: M7

## Goal

After this ticket the three cloud modules create what their target needs and then call `generic-vm` (`infra/tofu/README.md` structure 1): network with only the edge port public, a host with the user data of `generic-vm`, optionally a managed PostgreSQL service (AWS RDS, Azure Database for PostgreSQL) and object storage (Hetzner Object Storage, Amazon S3; on Azure a volume or SeaweedFS, because Blob Storage has no S3 API, 16 3 rule 4), and an off-site backup target with Object Lock where the provider offers it (16 8.2 item 3, R-OP-09). Every module has the shared inputs and outputs of the contract, pinned providers, no secret in inputs, outputs or state, and passes `tofu validate` and the static checks. The availability of PostgreSQL 18 per managed service is verified in the providers' documentation and written down with sources (16 3 rule 6). A live test against real accounts costs money and runs only after the owner's approval (OQ-09, D-W-125); it is not part of this ticket. Why: relocation to any host is a goal of M7; the modules must exist and be checked so that an instance operator can use them without the project paying for cloud accounts.

## Read first

- `infra/tofu/README.md` (structure, outputs, rules 1 to 5), the READMEs and `variables.tf` of `hetzner`, `aws`, `azure` and `generic-vm`.
- `docs/spec/16-operations.md`: 3 (targets table, rules 3 to 6), 4 (network rules), 8.1 to 8.2 (managed service path, off-site append-only), 11, 15 (R-OP-08, R-OP-09).
- `docs/operations/cloud-migration.md`, `docs/operations/backup-restore.md` 2 and 6, `docs/OPEN-QUESTIONS.md` OQ-09, ADR 0012.
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-16-<n>` rows).
- Briefs (merged names win, D-W-100): `M7-15-tofu-generic-vm.md` (`tofu-check`, user data, the test root).

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
8. White label: examples use `chat.example.com` and neutral resource names (`bothy-<instance_name>`); no company, account, project or personal names.
9. Provider versions pinned in `required_providers`; Renovate keeps them current.
10. Specification first: a change to the module contract updates `infra/tofu/README.md`, the module READMEs and, for facts about targets, `docs/spec/16-operations.md` 3 in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): no `tofu apply` or `tofu plan` against a provider account, no cloud credentials; only `init -backend=false`, `validate`, static analysis and plans with mocked providers where the tool supports them.
12. Never tag, never create a release, never publish, never create cloud resources (D-W-125).

Product requirements implemented here:

13. Each module exposes exactly the shared inputs of the contract plus its target-specific inputs, and exactly the shared outputs; a test compares the variable and output names across modules.
14. Only the edge port is reachable from the internet; database and storage only in the private network (`infra/tofu/README.md` rule 3); a check over the security group, firewall and network security group rules asserts it.
15. No secret is an input or output; state encryption is required in the example root (rule 2).
16. The off-site target is append-only for the instance's credentials (Object Lock where available; record per provider what the module can enforce and what the owner sets up by hand).
17. The PostgreSQL 18 question of 16 3 rule 6 is answered per provider with a source and a retrieval date; where a service does not offer 18, the module offers only the container path for that target and the README says why.

## Scope

- `infra/tofu/modules/{hetzner,aws,azure}/`: `main.tf`, `outputs.tf`, `versions.tf`, target inputs in `variables.tf`, READMEs; each calls `generic-vm`.
- `infra/tofu/test/`: the root that calls all four modules extended to the three cloud modules for `tofu validate`; a static test (Python with pytest under `tools/` or `tofu test` with mocked providers if the pinned OpenTofu supports it) for requirements 13, 14 and 15.
- `make tofu-check` covers the new modules; `make trivy` stays green on them.
- Docs: `infra/tofu/README.md` (status column, "Checked" with the date and version), `docs/spec/16-operations.md` 3 (rules 3 to 6 with the verified facts and sources), `docs/operations/cloud-migration.md`, `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-16-<n>`.

## Out of scope

- Any live test (owner approval, OQ-09); Key Vault and Secrets Manager drivers (V3); Helm (V3).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-16-tofu-cloud-modules`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(tofu): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories or cloud resources; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- No network calls except downloads of pinned providers and tools and read-only fetches of the providers' public documentation for requirement 17.
- Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; never configure provider credentials.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Static proof** (OQ-09; Ruling for docs/DECISIONS.md). Where the pinned OpenTofu supports `tofu test` with mocked providers, use it to plan each module with representative inputs and assert the network rules and outputs; otherwise parse the configuration with a pinned HCL parser in Python. Record which.

2. **Azure object storage** (16 3 rule 4). The module offers a volume (driver `fs`) or SeaweedFS on the host (profile `s3`), never Blob Storage through the S3 driver.

3. **Rulings for `docs/DECISIONS.md`** (`D-M7-16-<n>`): static proof (note 1), PostgreSQL 18 per provider with sources, the off-site lock per provider.

## Tests (write them first)

- Interface test: variables and outputs equal across modules (requirement 13).
- Network test: only the edge port is public per module (requirement 14).
- Secret test: no input or output is a secret; the example root requires state encryption (requirement 15).
- `make tofu-check` over all four modules.

Commands that must pass, from the repository root:

- `make tofu-check`, `make trivy`, `make lint`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `tofu validate` is green for `hetzner`, `aws` and `azure`.
- [ ] The interface, network and secret tests pass.
- [ ] The availability of PostgreSQL 18 per provider is verified with sources in 16 3 and the module READMEs.
- [ ] READMEs, 16, the cloud migration guide, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; comments explain why; every variable and output has a description.
- Module READMEs and `infra/tofu/README.md` updated in English.
- Rulings of Design note 3 recorded in `docs/DECISIONS.md` section 10.
- `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The PostgreSQL 18 findings per provider with sources.
5. What is not proven (every live deployment).
6. Owner decision with a recommendation (a live test with a cost budget, OQ-09); follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
