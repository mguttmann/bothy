# A1-08: Clients: New agent dialog

Status: ready | Depends on: A1-03, A1-06, A1-07 | Milestone: A1

## Goal

After this ticket an owner creates a built-in agent in the desktop app, the phone app and the web console: "New agent" offers a template, the interview or an empty form, then one editable preview of everything the agent gets, then visible steps with step-up that create the agent on a hosting runner, store its definition, set its class and billing mode and, when asked, create a channel with the agent as operator. A failed step names its cause, "Try again" resumes there and creates nothing twice, and "Close" keeps what exists. The entry is hidden on a server without the operations and when no runner hosts agents.

## Read first

- `docs/spec/01-product.md` 6.9 (the "New project" dialog this one follows) and 6.20 (this dialog), R-PR-20, R-PR-36; 5.1, 5.2, 5.3, 5.5 (where the agent list and space menu live on each platform), 8 (accessibility), 9 (languages).
- `docs/spec/26-builtin-agents.md` 3.1, 4.2, 4.4, 5.7 (what a class does not load, for the preview), 6.1 (interview and fallbacks), 7, 8 (all), 10, 12 (BA-T16), R-BA-11.
- `docs/spec/07-permissions.md` 2.13 and SR5 (step-up); `docs/spec/10-backend-policy.md` 10.3.
- `docs/spec/15-client-architecture.md` (the client foundation, dependency rules: the console uses neither the gateway nor the local database), and the design system under `docs/design/`.
- ADR 0076; design note D-A1-6, D-A1-12, D-A1-17, D-A1-22.
- The merged operations of A1-02, A1-03, A1-06 and A1-07 in `docs/api/openapi.yaml` and the generated Dart client in `packages/api_client`.
- Code on the base branch: `apps/client/lib/new_project/` (dialog, flow and port: the pattern to follow), `apps/client/lib/assist/` (a conversation with fallback notices), `packages/admin_ui/lib/src/agents/` (agent list and page), `apps/console`, `packages/l10n/lib/l10n/app_en.arb` and `app_de.arb`, `packages/sync_engine/lib/src/testing/fake_server.dart` (Dart fake server) and the contract copy it uses.

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

12. "New agent" shows only to owners and only when the server answers `listAgentTemplates` and at least one active runner reports `hosts_agents`; on an older server or without such a runner it is hidden (26 8.1).
13. The preview shows and lets the owner edit every part of 01 6.20 item 3, says what the chosen class does not load (MCP servers under `chat`, skills where 26 5.7 says so), names the secret names to add on the runner host, and offers `personal` only under the conditions of 10 10.3.
14. "Create" runs the steps of 26 8.2 in order (`agent.create`, `agent.tool_class.set` when the class is not `chat`, then `putAgentDefinition`, so a definition with a `stdio` MCP server under class `host` is accepted), each with its state and its own action; step-up prompts where an action needs one; a failure leaves exactly the earlier steps; "Try again" runs the steps whose target is not reached yet in the order of 26 8.2 (also after the owner raised the class in the preview) and creates nothing twice; "Close" keeps what exists (26 8.3, BA-T16).
15. The interview shows the builder's questions and the person's answers as a conversation, the cost estimate so far, and the fallback reason with the way to templates and the empty form.
16. Desktop, phone and console behave the same; the console talks to the server only through its backend for frontend (15).

## Scope

- `apps/client/lib/new_agent/`: dialog, flow (the step machine with resume), port (the API calls through the generated client), conversation view of the interview, preview editor; entry points in the agent list of the administration and the space menu (desktop) and the corresponding phone screens.
- `packages/admin_ui/lib/src/agents/`: the shared parts for desktop and console (preview, steps, template list) where the package rules allow; `apps/console` wiring.
- `packages/l10n/lib/l10n/app_en.arb` and `app_de.arb` with every text, `flutter gen-l10n` through `make codegen-check`.
- `packages/sync_engine/lib/src/testing/fake_server.dart` (or its merged name): the new operations for the tests, through the contract copy.
- Tests under `apps/client/test/new_agent/`, `apps/client/integration_test/`, `packages/admin_ui/test/`, `apps/console/test/`, goldens where the client keeps goldens for dialogs.
- READMEs of `apps/client` and `packages/admin_ui`; `docs/decisions.d/A1-08.md`.

## Out of scope

- Editing, versions, re-hosting and disabling (A1-09); server and runner changes (earlier tickets); a dialog for templates management.
- Any change of the "New project" dialog beyond sharing a widget without behaviour change.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/A1-08-new-agent-clients`; one writer per worktree.
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
- Screenshots, goldens and test data use the example persons and `example.com`; no real installation.
- No GUI test on a host the launcher does not name for it; integration tests run where `make e2e-desktop`, `make e2e-mobile` and `make console-e2e` run them.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that the base has A1-03 (`hosted_by` on `createAgent`, `hosts_agents`), A1-06 (`listAgentTemplates`) and A1-07 (`interviewAgent`), else `Verdict: BLOCKED dependency <ticket> missing` naming the missing one.

2. **Visibility.** The client reads the agent list it already has; a runner hosts agents when its reported capabilities carry `hosts_agents`. `listAgentTemplates` answering `404` or a missing operation in the server's discovery hides the entry (an older server).

3. **Step machine.** Steps: create agent, set class (at the first run only above `chat`; on "Try again" whenever the target differs from the agent's current class, also back to `chat`), store definition, set billing (only `personal`), create channel, set operator (only when a channel is asked for). The class step comes before the definition step because the server refuses a `stdio` server below `host` (26 4.3 item 7). Each step stores its result (agent ID, class, definition version, channel ID) in the flow state; "Try again" compares each step's target from the edited preview with that state and runs the steps not reached, in this order, so a class raised after a refused definition is set before the definition is sent again; idempotency keys per step come from the outbox rules of the client. A step-up prompt appears where the action needs one; a refused step-up fails that step with its sentence.

4. **Preview.** One page, sections for identity, instructions (a Markdown editor), skills (list with add, edit, remove), MCP servers (transport, URL or command and package, secret names), settings (model, class with an explanation of what it allows and does not load, billing, cost cap), channel. Field errors of the server (from the interview's draft or from `putAgentDefinition`) show at their field by JSON Pointer.

5. **Interview.** One request per turn; the answer replaces a spinner with the builder's questions or the draft; "Use what we have" sets `want_draft`; a fallback shows its reason and the buttons "From a template" and "Empty form". The cost estimate sums the turns.

6. **Accessibility and languages.** Every control has a label, the step list is announced on change at most as often as `a11y.live_announce_min_interval` allows, German and English texts.

7. **Rulings for `docs/decisions.d/A1-08.md`**: the entry points per platform, the step texts, the preview layout on the phone.

## Tests (write them first)

- Widget tests (desktop, phone, console): entry hidden without a hosting runner, hidden on an older server, shown otherwise; template, interview and form paths lead to the same preview; preview edits reach the request; `personal` offered only under 10 10.3.
- Flow tests (BA-T16): a failure at each step after `agent.create` leaves exactly the earlier steps; "Try again" resumes without a second create; "Close" keeps the agent, which the list shows as "Not set up". A template with a `stdio` MCP server and class `host`: the fake server sees `agent.tool_class.set` before `putAgentDefinition` and stores the definition (the fake server refuses a `stdio` definition below `host` with `tool_class_too_low` like the real one). A definition refused with `tool_class_too_low`, the class raised in the preview, "Try again": the class step runs, then the definition step, nothing is created twice. A definition step that fails after the class was set to `host`, the class changed to `chat` in the preview, "Try again": the class step sets `chat`, the agent does not stay at `host`.
- Interview tests: questions, draft with field errors, each fallback.
- Integration tests with the fake server on desktop, phone and console; goldens of the dialog in light and dark.

```sh
make apps-platform
make e2e-desktop
make console-e2e
make check
```

## Acceptance criteria

- [ ] BA-T16 is green; every path of 01 6.20 has a widget test on each platform.
- [ ] The entry is hidden where 26 8.1 says.
- [ ] Texts in English and German; accessibility labels present.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `apps/client` and `packages/admin_ui` describe the dialog.
- An entry `changelog.d/A1-<nn>.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/<ticket>.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0076 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: only owners see and use the dialog; every step goes through its own action with step-up where required; a failure never leaves a half-created step hidden; the preview shows everything that will run on the host; the console uses no gateway.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: the dialog on platforms the build host cannot run), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
