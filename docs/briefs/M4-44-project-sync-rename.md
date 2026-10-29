# M4-44: Forums for existing folders and renaming a project

Status: ready | Depends on: M4-43 | Milestone: M4

## Goal

After this ticket an owner adds a forum for every Git folder of a runner's projects root that has none yet, and renames a project forum, from the desktop app (01 6.10, 6.11, R-PR-21). "Add forums for existing folders" in the space menu lists the folders the runner reported (`Heartbeat.project_folders`, stored as `agents.reported_project_folders`) that no forum is bound to (`listAgentProjectFolders`, action `agent.project_folders.read`), with a checkbox each; "Add forums" creates per checked folder a forum named like the folder, assigns the runner when needed and binds the folder with `provision` `none`, then waits for readiness. A folder with a forum is never offered and never gets a second one. "Rename project" in the menu of a bound forum renames the forum through `updateChannel`; the folder stays. Why: the owner's former setup had a folder sync and a rename of project forums ([design note](../design/new-project.md) section 6, D-M4-44-1 to D-M4-44-7).

## Read first

- `docs/design/new-project.md` (all, section 6 for this ticket), `docs/decisions.d/M4-43.md`, `docs/decisions.d/M4-44.md`.
- `docs/spec/01-product.md`: 6.9, 6.10, 6.11, 13 R-PR-21, 14 (`ui.new_project_*`).
- `docs/spec/03-data-model.md`: `agents` (`reported_project_folders`), `project_bindings` (unique `lower(workspace_ref)`), R-DM-56.
- `docs/spec/05-actions.md`: rows `agent.project_folders.read`, `channel.update`, `channel.create`, `channel.operator.set`, `channel.project.bind`.
- `docs/spec/09-agents.md`: 4.8 (`project_folders`), 18.1, 18.13, 21 (`runner.project_folders_max`), R-AG-30.
- `docs/api/openapi.yaml`: `Heartbeat.project_folders`, `ProjectFolderName`, `ProjectFolderList`; operations `listAgentProjectFolders`, `updateChannel`, `putProjectBinding`.
- Brief and code of M4-43 (new project dialog, flow and port); merged code wins on names.

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
7. No en dash and no em dash in any file you write. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`.
9. Limits and windows only through the named configuration keys of 01 14 and 09 21.

Product requirements implemented here:

10. Server: migration adding `agents.reported_project_folders`; the heartbeat decodes `project_folders` (distinct names of the form of `ProjectFolderName`) and writes the column in its own unit of work when the list differs from the last one this process wrote; `listAgentProjectFolders` for the owner answers the stored names without those that a binding names (compared without case) and `reported`; the new action in the catalogue, the requirement table and the route list.
11. Runner: the folder report of 09 18.13 in every heartbeat, with its filter, order, cap and log codes; the configuration key `runner.project_folders_max`.
12. SDK: the Python adapter passes `project_folders` of the runtime status into the heartbeat.
13. Desktop: the space menu entry and the dialog of 01 6.10 for owners only (runner field, checked list, empty and not reported states, per row steps, failure sentences, resume, wait hint); the menu entry and the dialog of 01 6.11.

## Scope

- `services/server`: `migrations/` (new migration and its test), `internal/agents` (domain decode, app apply and list, postgres adapter, sqlc queries, REST handler), `internal/chat` (bound names for the readiness port), `internal/authz` (requirement table), generated route and REST code; READMEs of `internal/agents` and `internal/chat`.
- `plugins/claude-code-runner`: `workspace.py` (folder list), `channels.py`, `heartbeat.py`, `app.py`, `config.py`, `config.example.toml`, README.
- `sdk/python`: `adapter.py`, `heartbeat.py`, generated models.
- Generated: `services/server/api`, `sdk/typescript`, `packages/api_client`, `packages/sync_engine` contract copy.
- `apps/client/lib`: the project sync dialog and its flow, the rename dialog, the space and channel menus, the API adapter; `packages/l10n` texts.

Fragments: `docs/decisions.d/M4-44.md`; changelog fragments `services/server/changelog.d/M4-44.md`, `plugins/claude-code-runner/changelog.d/M4-44.md`, `sdk/python/changelog.d/M4-44.md`, `sdk/typescript/changelog.d/M4-44.md`, `changelog.d/M4-44.md` (app).

## Out of scope

- The phone (D-M4-44-7); folders deeper than one level or without `.git`; removing forums of vanished folders; renaming folders.

## Boundaries

- Work in a dedicated git worktree; one writer per worktree.
- Conventional Commits with a DCO sign-off; no co-author trailers.
- Tests bind only `127.0.0.1` with port 0 and make no network call; runner tests use temporary folders.
- Never read or print secrets; never lower a threshold or disable a gate.
- This brief was written after M4-43 merged, in the manner of D-W-100: where the merged code names a package, type, function, file or provider differently from this brief, follow the code and list each difference in the report.

## Design notes

1. **Write on change** (D-M4-44-1): the tracker keeps the last folder list it wrote per agent next to the readiness list and writes the column only when the list differs; absent means unchanged.
2. **Bound names**: agents asks chat through the readiness port for the names among a list that some binding names (`lower(workspace_ref)`), so it never reads `project_bindings` itself.
3. **One folder after the other** (D-M4-44-5): forum, runner, binding per folder in list order, so the first step-up covers the rest; then readiness of all forums every `ui.new_project_poll_interval`, until ready, a failure reason or `ui.new_project_wait`.
4. **Rename**: `getChannel` gives the version for `If-Match`; on `version_conflict` the dialog reads the forum again and keeps the typed name for one more press.

## Tests (write them first)

- Server: domain decode of `project_folders` (form, duplicates, absent); tracker writes on a change only; list without bound names, case folding, `reported` false before a report, owner only; migration test of the column.
- Runner: reported folders (Git work trees only, no hidden, no `.provision-*`, no files, no links out of the root), code point order, the cap with its log code, an unreadable root; the heartbeat carries the list.
- Client: the sync flow with every row failure and the resume without a second forum, a runner change reloads, empty and not reported lists; widget tests of the sync dialog and the rename dialog; menu entries only for owners or with `manage` on a bound forum.

Commands for the touched components: `go test` of the touched server packages, `uv run --locked pytest` in the runner and the Python SDK, `flutter test` of the new client tests, the linters and the codegen checks of those components.

## Acceptance criteria

- [ ] An owner adds forums for the checked folders without a binding of a runner; each becomes a ready project forum, and a folder with a forum is never offered or bound twice.
- [ ] Each failure shows the folder and one sentence; "Try again" resumes without duplicates.
- [ ] "Rename project" renames the forum; the binding and the folder stay.

## Definition of done

- Code, tests, READMEs, rulings and changelog fragments as in Scope.
- An independent review and the red team over the diff; the full suite runs once in the pull request CI.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits, test results (red before, green after), the checks run.
3. Review and red team verdicts.
4. What is not proven.
5. Follow-ups with reasons and differences to the names of this brief.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
