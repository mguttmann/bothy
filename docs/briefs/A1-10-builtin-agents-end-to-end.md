# A1-10: End to end and documentation

Status: ready | Depends on: A1-05, A1-09 | Milestone: A1

## Goal

After this ticket the stage is proven end to end and documented. On the demo stack with fake Claude, an owner creates one agent from a template and one through an interview with a fake builder, chats with both in their new channels, sees their costs, and after `runner.hosted_agents` is turned off the runner behaves as before and the hosted agents are offline with their definitions kept. An operator guide explains how to turn hosting on, and the README gets a section for visitors. The exit criteria of PLAN section 21 are checked and RISKS.md is updated.

## Read first

- `docs/PLAN.md` section 21 (Goal, Exit criteria, Test strategy, Red team) and section 4 (closing); `docs/spec/26-builtin-agents.md` (all), in particular 8, 11 and 12 (BA-T21).
- `docs/spec/16-operations.md` (installation, configuration, the demo instance of M7-27), `docs/operations/configuration.md` (the keys of 26 14).
- `docs/security/threat-model.md` 5.17, 6.9 and section 9 (maintenance rule); `docs/RISKS.md`.
- Code on the base branch: `infra/compose/demo.yaml`, `infra/compose/demo.sh`, `infra/compose/e2e.sh` and `infra/compose/test/`, the Makefile targets `demo`, `demo-e2e` and `adapters-e2e`, `plugins/claude-code-runner/tests/fake_claude/`, `README.md`, `docs/index.md`.

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

12. BA-T21: on the demo stack with fake Claude an agent from a template and one from an interview are created through the public API with step-up, answer in their channels, and show their costs; with `runner.hosted_agents` off the runner's served channels and heartbeat equal the A1-04 baseline and the hosted agents are offline with their definitions kept (26 11 item 4).
13. An operator guide `docs/operations/builtin-agents.md`: turning hosting on (the runner keys, `runner.hosted_agents_root` placement rules, the class ceiling, the builder, channel skills), secrets on the runner host, private templates, turning it off, what the preview does and does not isolate (26 5.7 results).
14. A README section for visitors: what a built-in agent is and how to try one on the demo, generic and without a description of any real installation.
15. The exit criteria of PLAN section 21 are checked one by one in the report; RISKS.md gets the risks of the stage; the threat model carries "as of stage A1" with the results of the probe of A1-05.

## Scope

- An end-to-end test of the demo stack (a Compose overlay under `infra/compose/test/` with the Claude Code runner in hosting mode, fake Claude as its CLI and a fake builder, and a test driver in `tools/e2e/` or the merged place of the demo integration test), reachable through a Makefile target in the pattern of `demo-e2e`.
- `docs/operations/builtin-agents.md` and its entry in `mkdocs.yml`; `README.md` section; `docs/index.md` if it lists features.
- `docs/RISKS.md`, `docs/security/threat-model.md` (status and results), `docs/roadmap.md` (stage status).
- `docs/decisions.d/A1-10.md`.

## Out of scope

- New behaviour in server, runner or clients; a fix found here is a fix with a test in the module that owns it, listed in the report.
- The showcase images (V1-11) and the public texts beyond the README section (V1-12).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/A1-10-builtin-agents-end-to-end`; one writer per worktree.
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
- The demo data and the guide use the example persons, `example.com` and generic agents only.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that the base has A1-05 (`materialize.py` in the runner) and A1-09 (the edit actions in `packages/admin_ui`), else `Verdict: BLOCKED dependency <ticket> missing` naming the missing one.

2. **Stack.** The runner container gets `runner.hosted_agents`, a `runner.hosted_agents_root` outside its state folder, `runner.agent_builder` for a second runner agent that acts as the builder (or the same runner, when the configuration allows both), and fake Claude as `runner.claude_bin`. Fake Claude answers a turn with a fixed reply and a fixed `result` usage, and the builder call with a fixed questions answer, then a fixed draft.

3. **Driver.** Through the public API only (P-26): owner login with step-up as the demo does, `listAgentTemplates`, `createAgent` with `hosted_by`, `agent.tool_class.set` when the class is not `chat`, `putAgentDefinition` (the order of 26 8.2), `channel.create`, `channel.operator.set`, a message, the reply, the run's costs; then `interviewAgent` twice and the same creation path with the draft. Then restart the runner with `runner.hosted_agents` off and compare its served channels and heartbeat with the A1-04 baseline fixtures; read both definitions back.

4. **Rulings for `docs/decisions.d/A1-10.md`**: the stack layout and the fake builder answers.

## Tests (write them first)

- The end-to-end test of design notes 2 and 3 (BA-T21), red before the overlay and driver exist.
- `make docs` and `make linkcheck` with the new guide.

```sh
make image
make demo-e2e
make docs
make linkcheck
make check
```

## Acceptance criteria

- [ ] BA-T21 is green.
- [ ] The operator guide and the README section exist, generic, linked from the docs site.
- [ ] Every exit criterion of PLAN section 21 is checked in the report with its evidence.
- [ ] RISKS.md and the threat model are updated for the stage.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- The guide, README, RISKS.md, threat model and roadmap describe the stage as built.
- An entry `changelog.d/A1-<nn>.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/<ticket>.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0076 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: the end-to-end test proves creation from a template and an interview, chat, costs and the rollback; the guide names every key and the limits of isolation honestly; no real installation is described.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: anything of the exit criteria that only a real CLI or a real host shows), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
