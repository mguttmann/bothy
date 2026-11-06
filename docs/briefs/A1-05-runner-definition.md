# A1-05: Runner: run the definition

Status: ready | Depends on: A1-02, A1-04 | Milestone: A1

## Goal

After this ticket a hosted agent's turns run from its definition. The runner checks `runner.hosted_agents_root`, writes the agent folder and the control folder from the current definition (never copying a file from it), builds the command line of 26 5.3 per tool class with its own restrictive settings, the skills plugin, the per-turn MCP file and the cost cap, rebuilds every control file and removes every foreign one before each turn, and refuses a turn whose definition breaks the closed schema or would start a `stdio` server below `host`. It probes every open point of 26 5.7 against the pinned CLI behind the real CLI switch and applies the fallback where a probe fails, and it loads channel skills only behind `runner.channel_skills_dir` and only in `host` turns. Without the new keys the runner stays byte-equal to the fixtures of A1-04.

## Read first

- `docs/spec/26-builtin-agents.md`: 1, 4 (all), 5 (all), 11, 12 (BA-T10 to BA-T14, BA-T19, BA-T20, BA-T22), 13 (R-BA-04 to R-BA-08, R-BA-13), 14 (runner keys).
- `docs/spec/09-agents.md` 18.1 (`allowed_tools` intersection), 18.2 (argument list, environment, tool class flags), 18.3, 18.7, 18.9, 18.10 (roots), 18.18; R-AG-35, R-AG-43.
- `docs/spec/07-permissions.md` 2.11, 2.13, SR2.
- `docs/security/threat-model.md` 5.17 and T-BA-02 to T-BA-04.
- ADR 0076; design note D-A1-7 to D-A1-9, D-A1-18 (with the surface table and "to verify in A1-05"), D-A1-20; `docs/decisions.d/A1-01.md` D-A1-01-5, D-A1-01-6, D-A1-01-10 to D-A1-01-12.
- The corpus `tests/agent-definitions/` and its README (A1-02); the hosting of A1-04 (`hosting.py`, the baseline fixtures).
- Code on the base branch: `plugins/claude-code-runner/src/claude_code_runner` (`commandline.py`, `toolclass.py`, `environment.py`, `turn.py`, `runturn.py`, `secrets.py`, `causes.py`, `workspace.py`, `prompt.py`, `config.py`), `tests/fake_claude/`, `tests/real_cli/test_pinned_cli.py`, `tests/unit/test_commandline.py`.

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

12. `runner.hosted_agents_root` is refused at start when it is equal to, inside or above `runner.state_dir` or `runner.projects_root`, when it or a folder above it holds a `CLAUDE.md`, or when group or others may write it (26 5.1, BA-T10).
13. The files of 26 5.2 are written by the runner from the definition: `SKILL.md` frontmatter by a YAML serializer with exactly `name` and `description`, extra files flat under their validated names, `plugin.json` with name and version only, the settings of 26 5.4 exactly, secret values only in the per-turn MCP file (mode `0600`, deleted after the turn) and never in any environment (BA-T11, R-BA-08).
14. Before every turn: the runner's own check of 26 4.3 against the corpus, refusal with `tool_class_mismatch` before `run.start` for a failing definition or a `stdio` server under a token class below `host`, rewrite of every owned file and removal of every foreign surface file of 26 5.5 in the agent and control folders, the agent's other files kept (26 5.6, BA-T12, BA-T13).
15. The command line per class is exactly 26 5.3; the tool list of the definition only narrows `runner.scoped_allowed_tools` (BA-T13).
16. `--max-budget-usd` from the definition; a run stopped by it ends `failed` with `runtime_error` and the cause `cost_cap_reached` (26 5.8, BA-T19).
17. Channel skills only with `runner.channel_skills_dir`, the `claude-code:<name>` resolution of 26 5.9 and only in turns of class `host` (the runner's own agent and hosted agents alike); a `chat` or `scoped` turn gets no `--plugin-dir` for a channel skill and its live box names the skipped reference (R-BA-13, BA-T22); without the key the baseline fixtures of A1-04 still match, also for channels with bindings of both forms (BA-T20).
18. Every point of 26 5.7 is probed against the pinned CLI behind the real CLI switch; the result and the applied fallback stand in 26 5.7 and the runner README (BA-T14).

## Scope

All paths under `plugins/claude-code-runner/` unless they start with `docs/` or `tests/agent-definitions/`.

Create:

- `src/claude_code_runner/definition.py`: the runner's own check of 26 4.2 and 4.3 (a Python port of the rules of A1-02, held by the corpus), the class requirement and the intersection of 26 4.4 item 2. It is the one place of the rules in the runner: if the base already has it (the builder of the interview uses the same module), use and extend it; never write a second copy.
- `src/claude_code_runner/materialize.py`: the agent and control folders, every file of 26 5.2, the per-turn MCP file with resolved secrets through `secrets.py`, the rebuild and the removal of foreign files of 26 5.6.
- `src/claude_code_runner/channel_skills.py`: 26 5.9.
- Tests: `tests/unit/test_definition.py` (runs the whole corpus), `tests/unit/test_materialize.py`, `tests/unit/test_hosted_commandline.py`, `tests/unit/test_channel_skills.py`, `tests/scenarios/test_hosted_turns.py`, and in `tests/real_cli/` a probe module `test_hosted_probe.py` behind the existing switch of the real CLI tests.

Change:

- `config.py`, `config.example.toml`: `runner.hosted_agents_root` checks, `runner.mcp_stdio_runners`, `runner.channel_skills_dir`.
- `commandline.py`, `toolclass.py`: the hosted additions of 26 5.3 in their fixed order; the existing classes and the runner's own agent unchanged.
- `turn.py`, `runturn.py`, `hosting.py`: fetch the definition with the hosted token (`getAgentDefinition` as agent-self), materialize on start, on `agent.definition_changed` and before every turn, then run the turn in the agent folder; replace the `definition_missing` end of A1-04 by a turn when a definition exists.
- `causes.py`: `cost_cap_reached`; the summary of `tool_class_mismatch` for a refused definition names the field code, never the content.
- `README.md` of the runner (folders, files, settings, the probe results), `docs/spec/26-builtin-agents.md` 5.7 (results) and, only if the probe forces a fallback, the sentences of 5.3 that the fallback changes.

## Out of scope

- The server (A1-02, A1-03); templates (A1-06); the builder (A1-07); clients.
- Approval cards for `scoped`; a monthly budget; built-in agents in project forums.
- Any change to the runner's own agent or to the argument list of 09 18.2 without the new keys.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/A1-05-runner-definition`; one writer per worktree.
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
- The probe of the real CLI runs only behind the existing switch of `tests/real_cli` and only when the launcher allows a real model call; its turns are as short as the point needs (one turn per point, `haiku` or the cheapest model the switch allows). Without the switch the probe is skipped and BA-T14 is reported as not proven.
- Planted test skills, hooks and settings live only in temporary folders of the test.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that the base has A1-02 (`tests/agent-definitions/` and `getAgentDefinition`), else `Verdict: BLOCKED dependency A1-02 missing`, and A1-04 (`hosting.py` and `tests/fixtures/baseline/`), else `Verdict: BLOCKED dependency A1-04 missing`.

2. **Order of a hosted turn.** Inbox and token check as for every run (09 4.3, 4.5); fetch the definition; the runner's own check (refusal before `run.start`); rebuild; billing pre-check of 18.3; `run.start`; resolve secrets into the per-turn MCP file; start the turn with the argument list of 26 5.3 and the environment of 09 18.2 (nothing added for MCP); after the end delete the per-turn file in a `finally`, also on stop and crash adoption (the sweep at start removes leftovers of a crashed runner).

3. **Frontmatter.** Use the YAML library the runner already depends on, with `default_style='"'` or the equivalent that quotes every scalar, so a line break or a colon in a value never starts a key; the schema check refuses a line break in a description anyway, and the test proves the serializer alone also holds (BA-T11).

4. **Rebuild.** Walk the agent folder without following symbolic links; a symbolic link anywhere in the agent or control folder is removed and logged (`hosted_link_removed`), because a link could point the CLI at a file outside the folder. Remove the surfaces of 26 5.5 by name at any depth; keep every other file. The control folder holds only what the runner wrote; anything else in it is removed.

5. **Probe.** For each row of 26 5.7, one test in `tests/real_cli/test_hosted_probe.py` that plants the case in a temporary agent folder and checks the observable result (a hook writes a marker file, `Bash` would write a marker, inline shell would print a marker, the placeholder "[shell command execution disabled by policy]" appears). Record the CLI version and the outcome per row in the runner README and in 26 5.7; where the expected result does not hold, implement the fallback of the table and test it with fake Claude.

6. **Settings deny rules.** Write the deny rules in the rule syntax of the pinned CLI (`Edit(<pattern>)` and `Write(<pattern>)` as the CLI documents it); the probe confirms that an edit of `CLAUDE.md` in the agent folder is refused under `scoped`.

7. **Cost cap.** Map the `result` the pinned CLI sends when `--max-budget-usd` stops a turn (find its `subtype` in the probe or in the recorded strings) to `cost_cap_reached`; when the probe cannot run, any end of a turn with a set cap and a cost at or above it maps to the same cause.

8. **Channel skills.** Read `channel_skill_bindings` through `agent.config.read` only when `runner.channel_skills_dir` is set; decide by the tool class the turn runs with (the class whose flags of 09 18.2 and 26 5.3 the command line carries), and add a channel skill's `--plugin-dir` only for `host`; below `host` skip every `claude-code:` reference and name the skipped ones in the live box status line once per run; a `claude-code:` reference whose name fails the pattern is skipped like a foreign one; a missing folder is named in the live box status line once per run.

9. **Rulings for `docs/decisions.d/A1-05.md`**: the probe results and every fallback taken; the deny rule syntax; the cost cap mapping.

## Tests (write them first)

- `tests/unit/test_definition.py`: every fixture of `tests/agent-definitions/` gives its expected findings in the runner's check (the same corpus as the server).
- `tests/unit/test_materialize.py` (BA-T11, BA-T12): the files of 26 5.2 for a full definition; a description with a line break and a colon; file names with `/`, `..` and `SKILL.md` refused before writing; settings file content equal to 26 5.4; planted files of every surface of 26 5.5 (also a symbolic link and a `bin/` folder in the plugin) removed, other files kept; the per-turn MCP file has mode `0600`, holds the resolved value and is gone after the turn; no secret value in the environment of the turn.
- `tests/unit/test_hosted_commandline.py` (BA-T13): the argument list per class equal to 26 5.3 (with and without skills, with and without MCP servers, with a cap); the intersection keeps the order of `runner.scoped_allowed_tools`; a `stdio` server under a token class below `host` and a definition failing the check end with `tool_class_mismatch` before `run.start`.
- `tests/unit/test_config.py` (BA-T10): every refused placement of `runner.hosted_agents_root`.
- `tests/scenarios/test_hosted_turns.py` with fake Claude and the fake server: a hosted turn answers in its folder; a definition change between turns is applied before the next; the cost cap end (BA-T19); channel skills with and without the key.
- `tests/unit/test_channel_skills.py` (BA-T22): a channel skill folder with `allowed-tools: Bash`, `hooks`, both inline shell forms and a `hooks/` folder, bound to channels of a `chat`, a `scoped` and a `host` turn, for the runner's own agent and for a hosted agent: the folder reaches only the `host` command lines; the others carry no `--plugin-dir` for it and name the skipped reference in the live box.
- `tests/unit/test_byte_equal_baseline.py` of A1-04 stays green (BA-T20).
- `tests/real_cli/test_hosted_probe.py` (BA-T14), behind the switch.

```sh
cd plugins/claude-code-runner && uv run --locked pytest -q
make adapters-e2e
make docs
make check
```

## Acceptance criteria

- [ ] The corpus passes in the runner's own check.
- [ ] BA-T10 to BA-T13, BA-T19, BA-T20 and BA-T22 are green with fake Claude.
- [ ] BA-T14 ran against the pinned CLI behind the switch, or is reported as not proven; 26 5.7 and the README carry the results and the fallbacks taken.
- [ ] No secret value in any environment, log, turn record or file outside the per-turn MCP file.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- The runner README describes the folders, the files, the settings, the rebuild, the probe results and the keys; `config.example.toml` shows the keys off.
- An entry `changelog.d/A1-<nn>.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/<ticket>.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0076 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: no content of a definition reaches a surface of 26 5.5 below `host`; no channel skill reaches a turn below `host`; the settings file holds nothing beyond 26 5.4; foreign files are removed before every turn; secrets stay out of every environment; the tool list only narrows the runner's; without the keys the baseline fixtures match.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: each row of 26 5.7 that the probe could not run; the behaviour of a later CLI version), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
