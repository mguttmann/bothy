# A1-04: Runner: host many agents

Status: ready | Depends on: A1-03 | Milestone: A1

## Goal

After this ticket one Claude Code runner process can host the built-in agents whose `hosted_by` is its own agent, behind the new key `runner.hosted_agents` (default off). For each hosted agent it fetches a key-bound token into memory, opens its own gateway connection with its own cursor, channel cache, heartbeat and state folder, and shares the runner's slots; it reports `hosts_agents` with its class ceiling. Runs of a hosted agent end with the cause `definition_missing` until A1-05 runs the definition. Without the key the runner is byte for byte the program it was before: the command line, the heartbeat and the served channels equal fixtures recorded from the base before any change.

## Read first

- `docs/spec/26-builtin-agents.md`: 1, 3 (all), 5.6 item 4, 11, 12 (BA-T09, BA-T20), 13 (R-BA-02, R-BA-03), 14 (`runner.hosted_agents`, `runner.hosted_agents_root`, `runner.hosted_max_tool_class`).
- `docs/spec/09-agents.md` 2.2 (the row `hosts_agents`), 4.3, 4.5, 4.8, 5.2.1, 18.1, 18.2 (process contract, slots, environment), 18.3, 18.9, 18.18; R-AG-35 (summary causes), R-AG-41 (wait reasons), R-AG-43.
- `docs/spec/04-sync-protocol.md` 15.1 and 15.2.
- ADR 0076; `docs/decisions.d/A1-01.md` D-A1-01-4, D-A1-01-13; design note D-A1-3, D-A1-4, D-A1-22 to D-A1-24.
- The merged result of A1-03 (`listHostedAgents`, `issueHostedAgentToken`, `agent.hosting_changed`, the heartbeat capability) in `docs/api/openapi.yaml` and the generated Python client of `sdk/python`.
- Code on the base branch: `plugins/claude-code-runner` (`src/claude_code_runner/app.py`, `config.py`, `channels.py`, `heartbeat.py`, `statedir.py`, `locks.py`, `toolclass.py`, `causes.py`, `secrets.py`, `README.md`, `config.example.toml`, `tests/unit/test_commandline.py`, `tests/unit/test_heartbeat.py`, `tests/scenarios/`, `tests/realserver/`), `sdk/python/src/bothy_sdk` (`client.py`, `gateway.py`, `auth.py`, `adapter.py`).

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

12. `runner.hosted_agents` defaults to `false`; with it off the runner reads no hosted list, opens no other connection, and its command line per tool class, its heartbeat bodies and its served channel list equal the fixtures of the base (BA-T20, 26 11).
13. With it on: one connection, cursor, channel cache, heartbeat and state folder `<runner.state_dir>/hosted/<agent_id>/` (mode `0700`) per hosted agent; slots of `runner.max_parallel_runs` shared across the runner's own agent and every hosted agent (26 3.2).
14. Hosted tokens only in memory: never in a file, a log line, a turn record, an environment or a crash dump the runner writes; refreshed before `expires_at` and after a restart (26 3.3 item 2, BA-T09).
15. The runner's heartbeat reports `hosts_agents` with `runner.hosted_max_tool_class` only when `runner.hosted_agents` is on; a hosted agent's heartbeat reports its own `reported_tool_class` (the lower of its configured class and the ceiling) and never `hosts_agents`.
16. Every run of a hosted agent ends `failed` with `adapter_error` and the cause `definition_missing` in the summary until A1-05 (26 5.6 item 4).

## Scope

All paths under `plugins/claude-code-runner/` unless they start with `docs/` or `sdk/`.

Create first, before any other change (design note 2):

- `tests/fixtures/baseline/`: the command line per tool class, the heartbeat bodies (start, with a run, degraded) and the served channel list, recorded from the unchanged base with fake Claude and the fake server, with a `README.md` saying how they were recorded and that they change only with a ruling.
- `tests/unit/test_byte_equal_baseline.py` comparing the runner without the new keys against them, also for channels that carry `channel_skill_bindings` of both forms (`claude-code:<name>` and a reference of another adapter).

Then create:

- `src/claude_code_runner/hosting.py`: the hosted set (list at start, on `agent.hosting_changed`, every `runner.channels_resync_interval`), per agent the token in memory, an SDK client and gateway connection with the runner's DPoP key, the per-agent channel cache, heartbeat and state folder, and the clean stop of an agent that left the set (connection closed, its turns stopped by 18.6 with the reason of an offline agent).
- `tests/unit/test_hosting.py`, `tests/scenarios/test_hosted_agents.py`, and a case in `tests/realserver/` against the real server.

Change:

- `config.py` and `config.example.toml`: `runner.hosted_agents`, `runner.hosted_agents_root` (only read here; its checks belong to A1-05), `runner.hosted_max_tool_class` (one of `chat`, `scoped`, `host`, default `scoped`).
- `app.py`, `heartbeat.py`, `channels.py`, `statedir.py`, `locks.py`: the shared slots and per-agent state; `run_slots` and `waiting_runs` per agent with the shared totals (09 5.2.1).
- `causes.py`: the cause `definition_missing` with its fixed sentence.
- `sdk/python` only where the SDK cannot hold several clients in one process yet (for example a module-level singleton); additive, with tests and a changelog fragment of `sdk/python`.
- `README.md` of the runner: hosting, the keys, what lives where, that tokens stay in memory; `docs/spec/09-agents.md` 18.18 only where the build settles a detail.

## Out of scope

- Running a definition, the agent and control folders, the command line of a hosted turn and the checks of `runner.hosted_agents_root` (A1-05).
- The builder (A1-07); channel skills (A1-05).
- Any change of the runner's behaviour without the new keys.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/A1-04-runner-hosting`; one writer per worktree.
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
- The adapter for another agent runtime (`plugins/hermes-adapter` or its merged name) gets no change in this ticket.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that the base has A1-03 (`issueHostedAgentToken` and `listHostedAgents` in `docs/api/openapi.yaml` and in the generated Python client), else `Verdict: BLOCKED dependency A1-03 missing`.

2. **Fixtures first.** Record the baseline before touching the runner: run the unchanged runner in the scenario harness with fake Claude for each tool class and capture the exact argument list, the heartbeat JSON bodies (with stable clocks and IDs from the harness) and `listServedChannels` as the runner caches it. Commit the fixtures and the comparing test in their own commit, red-free, so the report can show that the later commits keep them green. Never regenerate them in this ticket.

3. **Connections.** Each hosted agent is a consumer `default` of its own principal. The runner's DPoP key signs the proofs of all its connections, because the hosted tokens are bound to it (A1-03). A token is fetched at start and when it is within a margin of its `expires_at` (a fraction of `agents.hosted_token_ttl` named as a constant in code with its reason); on `4012` the runner drops the agent until the next hosted list says it is back.

4. **Slots.** One semaphore of `runner.max_parallel_runs` for every turn of the process; locks per thread and session stay per agent (their paths under the agent's state folder for hosted agents). `waiting_runs` names the agent's own waiting runs; `run_slots.used` counts every agent's turns, so the person sees that another agent holds the slots.

5. **Class.** The loaded class of a hosted agent is the lower of the `tool_class` claim of its token and `runner.hosted_max_tool_class` (26 3.1 item 3); its heartbeat reports that as `reported_tool_class`, read from `agent.read` of itself at start and on `agent.updated`.

6. **Until A1-05.** A hosted run is accepted into the inbox and ended at once with `run.finish` `failed`, `adapter_error` and the summary of `definition_missing`, without `run.start` and without a turn.

7. **Logs.** One log line per hosted agent joining or leaving (agent ID, never a token), and `hosted_token_refreshed` with the agent ID; a failed issue logs the problem code and retries with the SDK's backoff.

## Tests (write them first)

- `tests/unit/test_byte_equal_baseline.py` (design note 2): without `runner.hosted_agents` every fixture matches byte for byte (BA-T20 hosting part).
- `tests/unit/test_hosting.py`: the hosted set follows list, event and resync; a token is never written (scan `runner.state_dir` and the hosted state folders for the token after a scenario, and the log records); refresh before expiry; drop on `4012`.
- `tests/scenarios/test_hosted_agents.py` with the fake server: two hosted agents and the runner's own agent; separate cursors and state folders; shared slots (a third concurrent turn waits with `slots_full` while two other agents hold the slots); each hosted run ends with `definition_missing`; the runner's heartbeat carries `hosts_agents`, the hosted heartbeats do not (BA-T09).
- `tests/realserver/`: the runner hosts one built-in agent against the real server: token issued, connection open, a message to the agent ends with `definition_missing`.
- `tests/unit/test_config.py`: the new keys, their defaults and the refusal of an unknown class.

```sh
(cd plugins/claude-code-runner && uv run --locked pytest -q)
make adapters-e2e
make check
```

## Acceptance criteria

- [ ] The baseline fixtures were recorded from the base before any change and match after it (BA-T20 hosting part).
- [ ] BA-T09 is green: separate connections, cursors and state folders, shared slots, no token in any file or log.
- [ ] `hosts_agents` with the ceiling in the runner's heartbeat only with `runner.hosted_agents`.
- [ ] Hosted runs end with `definition_missing`.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- The runner README and `config.example.toml` describe hosting; `docs/spec/09-agents.md` 18.18 matches what was built.
- An entry `changelog.d/A1-<nn>.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/<ticket>.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0076 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: without the key the runner is byte-equal to the base; no hosted token ever reaches a file, log, environment or turn; one agent's state is separate from another's; the slots are shared and visible.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: a crash of the runner while several hosted agents run (each comes back with a fresh token after the restart); the behaviour with a real CLI (A1-05)), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
