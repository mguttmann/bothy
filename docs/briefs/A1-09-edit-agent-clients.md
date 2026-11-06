# A1-09: Clients: edit and remove

Status: ready | Depends on: A1-08 | Milestone: A1

## Goal

After this ticket an owner edits a built-in agent in the desktop app, the phone app and the web console with the same preview as the creation dialog, saved as a new definition version with step-up; sees the versions and undoes a change by saving an older version again; changes the hosting runner; disables the agent; and finishes the setup of an agent that is "Not set up". Server refusals (stale version, class below a `stdio` server, host ceiling, billing holder) show as plain sentences at the right place.

## Read first

- `docs/spec/01-product.md` 6.20 items 6 and 1; `docs/spec/26-builtin-agents.md` 3.1, 3.3 item 3, 4.1, 4.3 item 7, 4.4 items 4 and 5, 8.2 to 8.4, 10, 12 (BA-T16 edit part), R-BA-06, R-BA-11; `docs/spec/07-permissions.md` 2.13; `docs/spec/10-backend-policy.md` 10.3.
- `docs/spec/06-api.md` 8 (`If-Match`, `412 version_conflict`).
- The dialog of A1-08 (`apps/client/lib/new_agent/`, the shared parts in `packages/admin_ui`), the agent page of `packages/admin_ui/lib/src/agents/agent_page.dart`.
- The operations `getAgentDefinition`, `listAgentDefinitionVersions`, `putAgentDefinition`, `setAgentHost` in `docs/api/openapi.yaml` and the generated Dart client.

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
8. White label: examples, templates and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no internal script names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) changes in this ticket only where Scope names it: the catalogue rows of `docs/spec/actions.yaml` that R-AZ-36 assigns to this ticket and their generated copies (`make catalog`).
11. Additive only (26 section 11): every new column is nullable or defaults to "as before", every key defaults to off, and an installation that sets none of the new keys behaves exactly as before.

Note on rule 1: "stages" are the milestones and the stage A1 of the plan (PLAN P-27).

Product requirements implemented here:

12. "Edit" opens the preview of A1-08 with the current definition and saves with `putAgentDefinition`, `If-Match` and step-up; a `412` shows that another device changed the agent and offers to reload (26 4.1). When the edit also changes the class, a raised class is set with `agent.tool_class.set` before `putAgentDefinition` (so an added `stdio` server is accepted) and a lowered class after it (so a removed `stdio` server no longer blocks the downgrade), each a visible step with "Try again" as in 26 8.3 (26 8.4, BA-T16).
13. "Versions" lists the versions newest first; "Restore" saves the chosen version's content as a new version (26 4.1 item 2).
14. "Change runner" offers the runners that report `hosts_agents`, says that the agent's connections restart, and shows `host_tool_class_ceiling` and `hosted_holder_mismatch` as sentences (26 3.1, 10 10.3).
15. Lowering the class below `host` with a `stdio` server shows `definition_needs_host` with the way to remove the server first (R-BA-06).
16. "Disable" uses `agent.disable`; "Continue setup" resumes the steps of 26 8.2 for an agent without a definition, in their order (class before definition).

## Scope

- `apps/client/lib/new_agent/` (shared editor and flow, extended for edit and resume), `packages/admin_ui/lib/src/agents/` (agent page actions: Edit, Versions, Change runner, Disable, Continue setup), `apps/console` wiring.
- `packages/l10n` texts in English and German.
- The Dart fake server learns the edit operations if A1-08 did not add them.
- Tests under `apps/client/test/new_agent/`, `packages/admin_ui/test/`, `apps/console/test/`, `apps/client/integration_test/`.
- READMEs; `docs/decisions.d/A1-09.md`.

## Out of scope

- Server or runner behaviour (earlier tickets); deleting an agent beyond `agent.disable` (09 2.1 has no delete of agent principals).
- Templates management.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/A1-09-edit-agent-clients`; one writer per worktree.
- Conventional Commits with scope and a DCO sign-off on every commit (`git commit -s`), no co-author trailers; squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG fragments, README and `docs/decisions.d/` entries named in this brief, each listed in the report.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL and other services run only as throwaway containers (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, `t.TempDir()` or `tmp_path`, removed at the end.
- Never read or print secrets. Tokens, DPoP keys, secret values and secret references resolved to values never appear in logs, test output, failure messages, audit entries or files outside the places this brief names.
- No real model call and nothing that costs money, except a test behind the explicit switch of the runner's real CLI tests when this brief names one and the launcher allows it.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs beyond what Scope names, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before its dependencies were built, under the rule of D-W-100: where the merged code of a dependency names a package, type, function, file, operation or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from `docs/spec/26-builtin-agents.md`, stop with `Verdict: BLOCKED <reason>`.
- Screenshots, goldens and test data use the example persons and `example.com`.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that the base has A1-08 (`apps/client/lib/new_agent/`), else `Verdict: BLOCKED dependency A1-08 missing`.

2. **One editor.** Edit uses the preview editor of A1-08 with a mode flag; no second form. The diff to the current version is shown before saving (changed parts named), because a definition change takes effect at the next turn.

3. **Versions.** The list shows version, time, author and origin; "Show" opens a version read-only in the editor; "Restore" writes it as a new version through the same save path.

4. **Errors.** Every server reason of requirements 12 to 15 maps to a sentence and an action (reload, remove the server, choose another runner, switch billing).

5. **Rulings for `docs/decisions.d/A1-09.md`**: the placement of the actions per platform and the diff presentation.

## Tests (write them first)

- Widget tests per platform: edit and save with step-up; `412` with reload; versions list and restore; change runner with each refusal; class lowering with `definition_needs_host`; an edit that adds a `stdio` server and raises the class to `host` (the fake server sees `agent.tool_class.set` before `putAgentDefinition` and stores it) and one that removes it and lowers the class (`putAgentDefinition` first, then the class, no `definition_needs_host`) (BA-T16 edit part); disable; continue setup resumes at the first missing step.
- Integration tests with the fake server on desktop, phone and console.

```sh
make apps-platform
make e2e-desktop
make console-e2e
make check
```

## Acceptance criteria

- [ ] Every requirement 12 to 16 has a widget test on each platform.
- [ ] Texts in English and German; accessibility labels present.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `apps/client` and `packages/admin_ui` describe editing.
- An entry `changelog.d/A1-<nn>.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/<ticket>.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0076 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: edits always go through `putAgentDefinition` with step-up and `If-Match`; no path writes a definition the server would refuse without showing the refusal; the console uses no gateway.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: the screens on platforms the build host cannot run), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
