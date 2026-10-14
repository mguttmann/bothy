# M7-19: API reference in the docs site

Status: ready | Depends on: M1-17 | Milestone: M7

## Goal

After this ticket the documentation site carries a readable reference of the REST API from `docs/api/openapi.yaml` and of the gateway from `docs/api/asyncapi.yaml`, produced during `make docs` from the normative documents (ADR 0013, owner rule 3: API documentation is generated, here from the one source the routes and clients are generated from), with the webhook envelope of `webhooks.md` linked. The site loads no script, style or font from a foreign server: everything the reference pages need is built or bundled into the site at build time with pinned tools, and a check in `make docs` fails when a built page references an external script. Why: integrators and agent authors (11, the agent guide of M2-37) need the reference without reading YAML, and a public docs site that pulls foreign scripts leaks visitors and breaks the CSP rules the project applies elsewhere.

## Read first

- `docs/spec/06-api.md` (the role of the two documents, 14 generators), ADR 0013, ADR 0045 (docs site), `docs/briefs/README.md` note on rule 3 (`docs/api/openapi.yaml` is normative).
- `mkdocs.yml` (nav "API", plugins, hooks), `Makefile` (`docs`, `api-lint`, `linkcheck`), `docs/api/README.md`, `docs/api/redocly.yaml`, `docs/quality/toolchain.md`, `docs/quality/licenses.md` (license gate for npm and Python dependencies).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125), D-W-54 (MkDocs today, Zensical later); section 10 (format of `D-M7-19-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-03-api-contract-checks.md` (Redocly lint and the API checks), `M1-17-api-server-stubs.md`.

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
8. White label: examples stay as the documents have them (`example.com`, Alex, Sam, Robin).
9. Every generator is pinned (lock file or `tools.lock.json`) and passes the license gate; no CDN.
10. Specification first: the reference is generated from `docs/api/`; nothing is written by hand that the documents already say. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125).

Product requirements implemented here:

12. `make docs` generates both references from the current documents in every build; a stale generated file cannot be committed (generated into the build directory, or checked like other generated copies).
13. The built site references no external script, stylesheet or font: a check over the built HTML fails on any `src` or `href` to another origin for these kinds (links to external pages as plain anchors stay allowed).
14. `make docs` stays strict (`mkdocs build --strict`), and `make linkcheck` stays green.

## Scope

- The generators (Design note 1), pinned; their configuration under `tools/docs/` or `docs/api/`.
- `Makefile` `docs` (the generation step before `mkdocs build`), `mkdocs.yml` (nav "API": REST reference, gateway reference, webhooks), the hook or plugin that places the generated pages.
- `tools/docs/external_assets_check.py` with tests (requirement 13), run in `make docs`.
- Docs: `docs/api/README.md` (where the reference lives), `docs/quality/toolchain.md`, `docs/quality/gates.md`, `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-19-<n>`.

## Out of scope

- Python and Dart API docs of the SDKs and packages (their tickets); a hosted docs site deployment (owner, V1).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-19-api-reference-docs`; one writer per worktree.
- Conventional Commits with the component as scope (for example `docs(api): ...`, `build(docs): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- No network calls except package downloads of pinned tools; the build itself runs offline.
- Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Generators** (Ruling for docs/DECISIONS.md). Prefer generators that emit static Markdown or HTML at build time and need no browser script (for example a Python MkDocs plugin that renders OpenAPI and AsyncAPI into pages, or the pinned Redocly CLI producing a page whose script is bundled into the site). Compare two candidates by: renders this OpenAPI 3.1 document and the AsyncAPI 3 document without errors, output without external assets, license, maintenance; record the evidence and the choice.

2. **Rulings for `docs/DECISIONS.md`** (`D-M7-19-<n>`): the generators (note 1), where generated pages live, the external asset rule.

## Tests (write them first)

- `tools/docs/tests/test_external_assets_check.py`: a page with an external `<script src>` fails, a local script passes, an external anchor passes, an external stylesheet and font fail.
- A test that both reference pages exist in the built site and contain every `operationId` of `openapi.yaml` and every message name of `asyncapi.yaml`.

Commands that must pass, from the repository root:

- `uv run --project tools --locked pytest -q tools/docs`
- `make docs`, `make linkcheck`, `make licenses`
- `make check`

## Acceptance criteria

- [ ] `make docs` generates both references from `docs/api/`; every operation and every gateway message appears.
- [ ] The built site loads no external scripts, stylesheets or fonts, held by a check in `make docs`.
- [ ] The generators are pinned and pass the license gate; README, toolchain, gates, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- Rulings of Design note 2 recorded in `docs/DECISIONS.md` section 10.
- `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and the build time of `make docs` before and after.
3. Review and red team verdicts and the findings fixed.
4. The generator comparison.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
