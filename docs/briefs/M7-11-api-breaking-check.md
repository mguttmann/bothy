# M7-11: Breaking-change check of the API against the last publication (G6)

Status: ready | Depends on: M1-03, M7-10 | Milestone: M7

## Goal

After this ticket the gate G6 of 06 runs in `make check`: it compares `docs/api/openapi.yaml` (and the event schemas it holds) with the version of the last release tag of the component `server` and fails on every breaking change of 06 13 "Versioning" (the right column of its table): a removed field, a new required parameter or request property, a removed enum value, a changed type, a changed status code of an existing situation, a changed sort order or pagination kind, a changed `x-authz-action`; additive changes pass. Without any release tag yet, the gate reports that there is no published version and passes, so it works before the owner's first tag. The tool is oasdiff if it handles this OpenAPI 3.1 document (06 13 item 7 leaves that open, its 3.1 support is beta); otherwise a verified alternative or a small own comparison over the parsed documents is built and recorded in DECISIONS.md. Mutation fixtures prove each detection. Why: apps from stores and self-hosted servers are not updated together (06 13 item 5); a breaking change that slips through breaks installed clients.

## Read first

- `docs/spec/06-api.md`: 13 "Versioning" (items 1 to 7 and the table of additive and breaking changes), 14 (generators), 15 (G1 to G10, G5 tolerance), the source rows Q8.
- `docs/spec/23-quality.md` (gates), `docs/quality/gates.md`, `docs/quality/toolchain.md`, ADR 0013, ADR 0014, ADR 0038.
- `tools/ci/components.py` (`release`: tags and versions), `Makefile` (`api-lint`, `api-checks`, `gates`), `tools/api-checks/`.
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-11-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-03-api-contract-checks.md` (G1 to G3 and the tool layout), `M1-71-contract-tests.md`, `M7-10-server-release-chain.md` (tags `server/v*`, `release-prep`).

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
8. White label: fixtures use `example.com` and the people Alex, Sam and Robin; no company, host or personal names.
9. The tool is pinned with a checksum in `tools/bin/tools.lock.json` or as a locked Python dependency; no network call in the gate.
10. Specification first: 06 13 item 7 and the table G6 of 06 15 record the chosen tool and the base version rule in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125). Never tag or release (D-W-125).

Product requirements implemented here:

12. The base is the `docs/api/openapi.yaml` of the newest tag `server/v*` that is not a pre-release, read with `git show <tag>:docs/api/openapi.yaml`; a beta tag is the base only when no stable tag exists (record the rule); no tag: pass with the notice.
13. Every breaking kind of the 06 table is detected; every additive kind passes (mutation fixtures for each row).
14. A deliberate break is possible only by a major version of the API document (06 13 items 4 and 5): the gate accepts a break when `info.version` of the document raised its major against the base; nothing else lowers the gate.
15. The gate runs offline, in `make check` and therefore in CI, and names each finding with its path and the 06 rule.

## Scope

- `tools/api-checks/breaking.py` (or the oasdiff wrapper) with tests and fixtures (`tools/api-checks/tests/fixtures/g6/`).
- `Makefile`: target `api-breaking` in `gates` (component `server` and the API prefix select it); `docs/quality/gates.md` row.
- Pin of oasdiff (`make tools-lock`) or of the alternative; `docs/quality/toolchain.md` row.
- Docs: `docs/spec/06-api.md` 13 item 7 and 15 (G6 as built), `docs/quality/gates.md`, `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-11-<n>` (the entry the plan row names).

## Out of scope

- The AsyncAPI document beyond the event payload schemas that `openapi.yaml` holds (record whether `asyncapi.yaml` changes are covered; the G5 tolerance test stays as it is).
- Clients and SDK generation (G4).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-11-api-breaking-check`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(api-checks): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Temporary directories only via `mktemp -d`, removed at the end; tests that need a tag create it in a fixture repository in a temporary directory, never in the working repository.
- Never read or print secrets.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Tool choice** (06 13 item 7; Ruling for docs/DECISIONS.md). Run the pinned oasdiff `breaking` against two copies of the real document; if it parses the document and detects every fixture of note 2, use it with a small wrapper that maps its findings to the 06 rows; if it fails on 3.1 constructs used here or misses a row, build the comparison over the parsed YAML in Python (the rows of the 06 table are few and structural) and record the evidence.

2. **Mutation fixtures** (06 13 table). Start from a small valid document; one fixture per breaking row (remove a response field, add a required parameter, add a required request property, remove an enum value, change a property type, change a status code, change `x-authz-action`, change pagination kind) and one per additive row (new optional field, new op, new enum value in a response, new optional parameter); the test asserts exactly the expected findings.

3. **Rulings for `docs/DECISIONS.md`** (`D-M7-11-<n>`): the tool (note 1), the base tag rule (requirement 12), the major-version exception (requirement 14).

## Tests (write them first)

- `tools/api-checks/tests/test_breaking.py`: every fixture of Design note 2; the base selection with a fixture repository (no tag, beta only, stable and beta); the major-version exception; the real `docs/api/openapi.yaml` against itself has no finding.

Commands that must pass, from the repository root:

- `uv run --project tools --locked pytest -q tools/api-checks`
- `make api-breaking`, `make api-lint`, `make api-checks`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] Mutation fixtures (remove a field, add a required parameter, remove an enum value, change a type and the other breaking rows) are detected; additive ones are not.
- [ ] Without a release tag the gate passes with a notice; with a tag it compares against that tag's document.
- [ ] The chosen tool or the verified alternative is pinned and recorded in DECISIONS.md and 06.
- [ ] `make api-breaking` runs in `make check`.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
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
4. The evidence for the tool choice (what oasdiff did with this document).
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
