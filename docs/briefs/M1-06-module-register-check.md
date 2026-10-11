# M1-06: Module register checked against the core contract

Status: ready | Depends on: M1-02, M1-05 | Milestone: M1

## Goal

A new gate `make register-check` keeps the module register
(`services/server/architecture/modules.yaml`), the table owners of 03, the action catalogue
(`docs/spec/actions.yaml`), the module tables of 02, the server component diagram and the module
READMEs equal. Any difference fails with file and item. After this ticket the register is a
trustworthy single source: go-arch-lint, table ownership and later checks can rely on it, and a
chapter edit cannot silently drift away from the code frame.

## Read first

- `docs/PLAN.md`: section "Way of working per ticket", section "M1 Foundation" (row M1-06), P-11.
- `docs/spec/02-architecture.md`: "Modules of the monolith" ("Rules" items 6 and 7, "Module list",
  "Allowed dependencies", "Cross-cutting ports (`internal/kernel`)", "Actions per module",
  "Event types and WebSocket ops per module"), "Automatic check of the rules" ("Module register as
  the source", "Other checks in the gate").
- `docs/spec/03-data-model.md`: "Modules", "Tables" (each table heading `#### \`<name>\`` with the
  sentence `Module: \`<module>\`.` in its purpose paragraph), "Technical tables".
- `docs/spec/05-actions.md`: "Binding rules" items 3 and 4, "Actions", "Durable event types and
  receive action", "WebSocket ops and action".
- `docs/spec/actions.yaml` and `tools/catalog/` with its README (from M1-02), ADR 0049.
- `services/server/architecture/modules.yaml`, `services/server/tools/archgen`, ADR 0050 (from M1-05).
- `docs/architecture/c4-3-server-components.mmd`, `services/server/internal/*/README.md`.
- `tools/README.md` (rules for new tools), `tools/pyproject.toml`, root `Makefile`,
  `docs/quality/gates.md`, `docs/spec/23-quality.md` "Principles".

## Fixed requirements that apply

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
6. Test first: write the failing test, run it and keep its red output for the report, make it green,
   then clean up.
7. No en dash (U+2013) or em dash (U+2014) in any file you write; `uv run --project tools --locked --quiet
   python tools/hooks/text_rules.py <files>` over your changed files exits 0 (plain `grep -P` fails on macOS).
8. White label: no company, person, host or instance names, no paths of a personal machine, no real
   data.
9. Configuration values only through named keys from `docs/operations/configuration.md`. This ticket
   adds none.
10. Every table of 03 belongs to exactly one module; every action of the catalogue is executed by
    exactly one module (02 "Rules" item 6). The modules that own no tables are the ones 02 "Rules" item 7 and 03
    "Modules" name (`none` in 02 "Module list"); the check reads them, it does not count them.
11. Every durable and every ephemeral event type has exactly one producing module (02 "Event types
    and WebSocket ops per module").
12. The checks "Completeness" and "Docs alignment" of 02 "Other checks in the gate" run in the
    gate.
13. New repository tools: Python with the standard library or PyYAML, one test file per tool, SPDX
    line in every file, one make target, a row in `tools/README.md` (tools README rules).
14. The core contract (00, 03, 04, 05 and `actions.yaml`) is never changed here. A difference that
    can only be fixed there ends the ticket with `Verdict: BLOCKED` (P-11).

## Scope

Create:

- `tools/doccheck/register_check.py` (the check, CLI `python tools/doccheck/register_check.py
  [--repo PATH]`).
- `tools/doccheck/tests/test_register_check.py` and small fixture builders inside the test file
  (use `tmp_path`; no large fixture trees).

Change:

- Root `Makefile`: target `register-check` with a `## ` help text, added to `gates` and `.PHONY`;
  add `tools/doccheck` to the bandit directories of `lint`.
- `tools/pyproject.toml`: add `doccheck` to pytest `testpaths` and `pythonpath`, coverage `source`
  and mypy `files`.
- `tools/README.md` (row), `docs/quality/gates.md` (row for `register-check`).
- `docs/spec/02-architecture.md` "Other checks in the gate": column "Implementation (ticket)" of the
  rows "Completeness" and "Docs alignment" names `make register-check`.
- Data fixes the check finds, only in `services/server/architecture/modules.yaml` (then regenerate
  `.go-arch-lint.yml` with `go generate ./tools/archgen`), in 02, in the component diagram (then
  re-render its SVG with `make diagrams` tooling) or in a module README.
- `docs/DECISIONS.md`, root `CHANGELOG.md` (`Added`).

## Out of scope

- The register format and the go-arch-lint generator (M1-05). Action catalogue structure, its
  generator and the tables of 05 (M1-02). API contract checks (M1-03).
- Table ownership inside SQL files (`adapters/postgres/queries/*.sql`, 02 "Other checks in the
  gate" row "Table ownership"): it needs sqlc queries and comes with M1-12.
- Route and op handler placement (M1-17, M1-25). ERD completeness (not planned in M1).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-06-module-register-check`.
- Conventional Commits with DCO sign-off (`git commit -s`), scope `tools` or `spec`.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside Scope, except docs and changelog entries named here.
- Follow the host rules supplied by the launcher. Stop only processes you started, by PID.
- Temporary directories only via `mktemp -d` or pytest `tmp_path`; remove them.
- Never read or print secrets.
- No network calls except package downloads for pinned tools and what the repository gates in
  `make check` do themselves.

## Design notes

### Sources and parsing (locate sections by title, never by number)

1. Register: PyYAML `safe_load` of `services/server/architecture/modules.yaml`; fields as defined in
   ADR 0050 and 02 "Module register as the source" (`modules.<m>.tables`, `actions`, `events`,
   `ephemeral_events`, `ws_ops`, `depends_on`; `kernel.core`, `kernel.ports`).
2. 03: the level-2 section titled "Tables" up to the next level-2 heading. Each `#### \`<name>\``
   heading is a table; its owner is the first `Module: \`<m>\`` after the heading and before the first
   table row. A table heading without owner is a finding. The section titled "Modules" gives the
   module set (first column) and whether a module owns tables (third column `yes` or `none`).
3. Catalogue: use the loader M1-02 ships in `tools/catalog` (see its README and ADR 0049) for action
   ids, durable event types, ephemeral event types and ops with their direction. Do not parse
   `actions.yaml` a second time. If `tools/catalog` exposes no importable loader, add one public
   function there with a test and no other change, and record it in the report.
4. 02: subsections titled "Module list", "Allowed dependencies", "Cross-cutting ports",
   "Actions per module" and "Event types and WebSocket ops per module". Rows start with `| \``; split
   cells on `|`; read backticked names per cell; a cell with `nothing`, `none` or no content is the
   empty set. In "Cross-cutting ports" the first column is `kernel/<package>`.
5. Diagram: node declarations `^\s*([a-z]+)\["` and edges `^\s*([a-z]+)\s*-->\s*([a-z]+)\s*$` whose
   both ends are register modules; lines with `~~~` are layout only.
6. READMEs `services/server/internal/<m>/README.md`: the line starting with
   `May use the ports of these modules:` and the backticked names of the paragraph under
   `## Tables` (`none` means the empty set).

### Checks (one finding per line, sorted, then exit 1)

| Id | Rule |
|---|---|
| C1 | Every 03 table is listed by exactly one register module, and that module equals its `Module:` line; a register table unknown to 03 is a finding. Technical tables (section "Technical tables") are not module tables. |
| C2 | Module set of 03 "Modules" equals the register; modules marked `none` list no tables and modules marked `yes` list at least one. |
| C3 | Every catalogue action is in exactly one module's `actions`; a register action unknown to the catalogue is a finding. |
| C4 | Every durable event type is in exactly one module's `events`, every ephemeral one in exactly one module's `ephemeral_events`; unknown or misplaced types are findings. |
| C5 | Every op sent by a client, agent or app (05 "WebSocket ops and action", not server-to-client) is in exactly one module's `ws_ops`; server-to-client frames never appear in the register. |
| C6 | 02 "Module list": same modules, same table set per module. |
| C7 | 02 "Allowed dependencies": same `depends_on` set per module. |
| C8 | 02 "Actions per module": same action set per module. |
| C9 | 02 "Event types and WebSocket ops per module": same durable, ephemeral and op sets per module. |
| C10 | 02 "Cross-cutting ports": package set equals `kernel.core` plus `kernel.ports`. |
| C11 | Diagram: module node set equals the register modules; edge set equals all `depends_on` pairs, in both directions (missing and extra edges). |
| C12 | Each module README: dependency line and table list equal the register. |

- Finding format: `register-check: <repo-relative path>: <rule id>: <message>`, for example
  `register-check: docs/spec/02-architecture.md: C8: module chat: action thread.move missing`.
- Success prints `register-check: ok (<n> modules, <t> tables, <a> actions, <e> event types,
  <o> ops)` and exits 0. A missing file or section exits 2 with a message naming it.
- Compare, do not generate: the tables of 02 mix machine columns with prose columns ("Responsibility",
  "Reason per edge") that people maintain, so the check compares the machine columns only. Record
  this decision in DECISIONS.md.
- Resolution rule for real differences on the merged tree: names and table owners come from the
  core contract (03, 05, `actions.yaml`); the register follows the core contract; 02, the diagram and
  the READMEs follow the register. A difference that needs a core contract change ends the ticket
  with `Verdict: BLOCKED`. An action that the catalogue has but 02 does not assign goes to the
  module that owns the tables of its resource (03); record each such assignment in DECISIONS.md.
- Typed code (mypy strict), no global state, functions small enough for the ruff pylint limits in
  `tools/pyproject.toml`; module docstring states purpose, sources and rule ids.

## Tests (write them first)

`tools/doccheck/tests/test_register_check.py`, each test builds a minimal repository in `tmp_path`
(two or three modules, a few tables, actions, events, ops, a diagram and READMEs) with one
deliberate defect and asserts the rule id and item in the output:

- `test_clean_fixture_passes`
- `test_table_from_03_without_module_fails` (C1)
- `test_table_in_two_modules_fails` (C1)
- `test_table_owner_differs_from_03_fails` (C1)
- `test_table_heading_without_owner_fails` (C1)
- `test_module_set_differs_from_03_fails` and `test_tableless_module_with_tables_fails` (C2)
- `test_action_twice_fails` and `test_action_without_module_fails` and
  `test_register_action_unknown_to_catalogue_fails` (C3)
- `test_event_type_without_producer_fails`, `test_event_type_two_producers_fails`,
  `test_ephemeral_event_listed_as_durable_fails` (C4)
- `test_client_op_without_module_fails` and `test_server_frame_in_register_fails` (C5)
- `test_module_list_row_differs_fails` (C6), `test_dependency_row_differs_fails` (C7),
  `test_actions_row_differs_fails` (C8), `test_events_row_differs_fails` (C9),
  `test_kernel_packages_differ_fails` (C10)
- `test_module_edge_missing_in_diagram_fails` and `test_extra_edge_in_diagram_fails` (C11)
- `test_readme_tables_differ_fails` and `test_readme_dependencies_differ_fails` (C12)
- `test_missing_section_exits_2`
- `test_real_repository_passes` (runs against the repository itself; the check is useless if it
  does not hold the real files)

Commands that must pass:

```sh
uv run --project tools --locked pytest tools/doccheck/tests
make register-check
make lint typecheck test coverage docs linkcheck diagrams
make check
```

## Acceptance criteria

- [ ] `make register-check` is part of `gates` and green on the merged tree.
- [ ] Each rule C1 to C12 has at least one failing fixture test; all tests pass.
- [ ] `test_real_repository_passes` runs against the real register, 02, 03, catalogue, diagram and
      READMEs.
- [ ] Tools coverage stays at or above `quality.coverage.tools_min_percent`; mypy strict, ruff and
      bandit are clean.
- [ ] `tools/README.md` and `docs/quality/gates.md` list the new gate; 02 "Other checks in the
      gate" names it.
- [ ] No file of the core contract changed.
- [ ] `make check` is green.

## Definition of done

- Code, tests and docs as above: docstrings on every public function and class, rows in
  `tools/README.md` and `docs/quality/gates.md`, DECISIONS.md entries, root `CHANGELOG.md` entry under
  `[Unreleased]` (`Added`).
- `make check` green.
- Independent review over the diff (a fresh reviewer, `make review`) and the red team gate of
  `docs/quality/red-team-gate.md` over the diff, verdict recorded in the report; fix every finding
  with a test and have the delta checked again.
- Merge locally into the base branch only if review and red team are green and `make check` passes
  on the merged state; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits
(SHA and subject), files with line counts, test results with counts (including the red output of
the first test run), the findings the check produced on the real tree and how each was resolved,
what is not proven, and follow-ups that could not be done in scope with the reason. Then write
`{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
