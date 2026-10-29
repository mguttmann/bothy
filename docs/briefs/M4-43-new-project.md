# M4-43: New project from the app

Status: ready | Depends on: M2-34, M4-41 | Milestone: M4

## Goal

After this ticket an owner creates a project from the desktop app (01 6.9, R-PR-20): "New project" at a space row and in the space menu opens a dialog with four modes (an empty project, a clone of an `https` repository, an existing folder under the runner's projects root, an idea). The dialog creates the forum, assigns the runner when it is not the space's default operator, binds the project with the new binding member `provision` (`init`, `clone` or `none`), waits until the runner reports the project ready, and for an idea posts it as the first chat, so the agent answers there at once. The serving runner creates the folder under `runner.projects_root` (09 18.12) and reports a readiness reason (`ProjectReadiness.reason`, stored as `project_bindings.ready_reason`), so the person sees each step and, on a failure, why. Why: owner feedback of 2026-10-28, a space had no way to add a project ([design note](../design/new-project.md), D-M4-43-1 to D-M4-43-12).

## Read first

- `docs/design/new-project.md` (all), `docs/decisions.d/M4-43.md`.
- `docs/spec/01-product.md`: 6.8 item 8 (new post draft), 6.9 (all), 13 R-PR-20, 14 (`ui.new_project_*`).
- `docs/spec/03-data-model.md`: `project_bindings` (`provision`, `ready_reason`), R-DM-54, R-DM-55.
- `docs/spec/09-agents.md`: 4.8 (`project_readiness` with `reason`, the code form), 18.1, 18.2 (environment allow list), 18.12 (all), 21 (`runner.git_bin`, `runner.provision_timeout`, `runner.provision_retry_after`), R-AG-29.
- `docs/spec/05-actions.md`: 4.5 rows `channel.create`, `channel.operator.set`, `channel.project.bind`, `channel.behavior.read`; `thread.create`.
- `docs/api/openapi.yaml`: `ProjectBinding`, `ProjectBindingPut`, `ProjectProvision`, `ProjectReadiness`; operations `createChannel`, `setChannelOperator`, `putProjectBinding`, `getChannelBehavior`, `listAgents`, `createThread`, `putChannelDraft`.
- Briefs and rulings of M2-03 (binding), M2-20 (heartbeat), M2-34 (runner), M4-41 (new chat, channel draft); merged code wins on names.

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

10. Server: migration adding `provision` (CHECK `none`, `init`, `clone`, default `none`) and `ready_reason` to `project_bindings`; `putProjectBinding` takes `provision` with the checks of 03 (one segment for `init` and `clone`, `https` URL for `clone`), counts it in the equality of a PUT and clears `ready_reason` on every effective change and every reset; the heartbeat applies `reason` (code form, only with `ready` false) and a change of the reason alone like a change of `ready`; `ProjectBinding` carries `provision` always and `ready_reason` while set.
11. Runner: the folder creation of 09 18.12 with its record, checks, process rules, outcome codes, retry window and immediate heartbeat; readiness entries carry the reason.
12. Desktop: the entry and the dialog of 01 6.9 for owners only, the step list, the failure sentences, the resume, the wait hint, the idea through the forum's new post draft, opening the forum or the new chat at the end.

## Scope

- `services/server`: `migrations/` (new migration and its test), `internal/chat` (domain, app, postgres adapter, sqlc queries), `internal/agents` (heartbeat readiness with reason), the REST mapping of the binding; README of `internal/chat`.
- `plugins/claude-code-runner`: a provisioning module, `channels.py`, `app.py`, `config.py`, `config.example.toml`, README.
- Generated: `services/server/api`, `sdk/python`, `sdk/typescript`, `packages/api_client`, `packages/sync_engine` contract copy, `packages/domain` configuration keys.
- `apps/client/lib`: the new project dialog and its controller, the sidebar space row action, the space menu entry, the API adapter; `packages/l10n` texts.

Fragments: `docs/decisions.d/M4-43.md`; changelog fragments `services/server/changelog.d/M4-43.md`, `plugins/claude-code-runner/changelog.d/M4-43.md`, `changelog.d/M4-43.md` (app), and the SDK components that have a `changelog.d/`.

## Out of scope

- The phone (D-M4-43-12); browsing the runner's folders; `ssh` clones; deleting folders.
- A new action or permission (D-M4-43-1).

## Boundaries

- Work in a dedicated git worktree; one writer per worktree.
- Conventional Commits with a DCO sign-off; no co-author trailers.
- Tests bind only `127.0.0.1` with port 0 and make no network call; the runner tests use a fake `git` through `runner.git_bin` for clones and may use the installed `git` for `init` in a temporary folder.
- Never read or print secrets; never lower a threshold or disable a gate.
- This brief was written after M4-41 merged, in the manner of D-W-100: where the merged code names a package, type, function, file or provider differently from this brief, follow the code and list each difference in the report.

## Design notes

1. **Order of the steps** (D-M4-43-1): forum, operator, binding, readiness, idea. The operator comes before the binding, because a later operator change resets readiness (09 18.1).
2. **Readiness polling**: a readiness report writes no event; the dialog reads `getChannelBehavior` every `ui.new_project_poll_interval` until `ready`, a failure reason, or `ui.new_project_wait`.
3. **Failure reasons that end the wait**: every code of 09 18.12 except `provisioning` and, while the runner has not reported yet, an absent reason.
4. **Resume**: the controller keeps the IDs and idempotency keys of done steps; "Try again" runs from the failed step with the current fields.

## Tests (write them first)

- Server: domain tests of the checks and the PUT plan with `provision`; app tests of `ready_reason` (stored, cleared on a change, on `ready` true and on a reset); heartbeat test of a reason-only change; migration test.
- Runner: `init` creates a Git work tree and is ready; `clone` runs the fake `git` with the exact argument list and environment and renames; an existing folder is `workspace_exists`; refused names and URLs never start a process; a time limit kills and reports `provision_timeout`; a failure removes only its own folder; the retry window; the record survives a restart; a heartbeat follows the end of a creation.
- Client: name suggestion; the step machine with each failure and the resume; the dialog shows only for an owner; widget tests of the dialog states.

Commands for the touched components: `go test` of the touched server packages, `uv run --locked pytest` in the runner, `flutter test` of the new client tests, the linters and the codegen checks of those components.

## Acceptance criteria

- [ ] An owner creates a project in each of the four modes from the space row or menu; the forum is bound, the folder exists under the projects root, and for an idea the first chat is posted when ready.
- [ ] Each failure shows the step and one sentence; "Try again" resumes without duplicates.
- [ ] No path outside the projects root is written; refused names and URLs never start `git`.

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
