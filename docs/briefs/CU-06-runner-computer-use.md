# CU-06: Runner integration and end to end

Status: ready | Depends on: CU-05 | Milestone: CU

## Goal

After this ticket the Claude Code runner offers computer use: with `runner.computer_use` every `host` turn gets the per-turn MCP file for `bothy-computer-mcp`, the request folder with the run's nonce through which the runner calls `computer.desktops.list` and `computer.open` with the run's agent token and invocation token, the system prompt section `## Computer use`, and after a run that opened a computer session the end of the thread's Claude Code session with its transcript and tool result files deleted. The CLI probes of `docs/spec/27-computer-use.md` section 16 are run against the pinned CLI, and an end-to-end test in Compose drives the Linux test backend from a scripted agent through runner, MCP server, server, relay and host service.

## Read first

- `docs/spec/27-computer-use.md` sections 7, 10 (all), 13.2, 15 (CU-T21, CU-T22, CU-T24), 16, 18.
- `docs/spec/09-agents.md` 18.2 (process contract, background mode, the stage CU paragraph), 18.6, 18.9, 18.19.8 (`bothy-wake`, the request folder pattern), 4.6 (run context), 9 (agent sessions).
- Code on the base branch after CU-05: `plugins/claude-code-runner/src/claude_code_runner/` (`config.py`, `materialize.py`, `bgsession.py`, `heartbeat.py` with `PINNED_CLAUDE_CODE_VERSION`, the turn launch and the system prompt file), `plugins/claude-code-runner/tests/real_cli/` (the method against a local stand-in for the API), `services/host/cmd/bothy-computer-mcp`, `services/host/test/computer_backend`, `infra/compose/e2e.sh`, `tests/e2e/`.
- Design note `docs/design/computer-use.md` (context and evidence; where it differs from the chapters, the chapters win); ADR 0078; `docs/decisions.d/CU-01.md`; `docs/security/threat-model.md` 5.18 and 6.10.

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
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) changes in this ticket only where Scope names it.
11. Additive only: every new key defaults to off or to the behaviour before, and an installation that sets none of the new keys (`runner.computer_use`, `host_service.computer_use`) and enables no helper behaves exactly as before.
12. Full operation (owner ruling OR-1, D-CU-01-1): build no app tier, no key or app block list, no confirmation per session or action and no lock on the Bothy windows; the only protections are those of `docs/spec/27-computer-use.md` section 1 item 3.

Note on rule 1: "stages" are the milestones and the stages A1, AR and CU of the plan (PLAN P-27, P-29, P-30).

Product requirements implemented here:

13. Keys `runner.computer_use` (default `false`), `runner.computer_mcp_bin` (required with it; an absolute path to an existing file not writable by group or others, else the runner refuses to start) and `runner.computer_request_timeout`, in the runner configuration and its README.
14. Per-turn file `<runner.state_dir>/turns/<run_id>.computer.mcp.json` (mode `0600`, removed at the turn's end and swept at start) with the one server of 27 10.1; `--mcp-config <file>` directly after `--permission-prompts none` in print mode and directly after the flags of the class in background mode, without `--strict-mcp-config`; only under `host`; below `host` never (CU-T21). Without the key the argument list and environment equal the fixtures of before.
15. The file contract of 27 10.2 on the runner side and the four variables of 27 10.1 (`BOTHY_COMPUTER_DIR`, `BOTHY_COMPUTER_NONCE`, `BOTHY_COMPUTER_TIMEOUT_MS`, `BOTHY_COMPUTER_PINS`) in the MCP file's `env`; a fresh nonce per run; the runner calls the two operations with the agent token, DPoP and `Invocation-Token`; no token or key in the turn's environment, files or system prompt (CU-T21).
16. The system prompt section `## Computer use` with the fixed text of 27 10.3, only in turns with the tool.
17. End of a computer run (27 10.5): the runner records that a run opened a session (from the answers it relayed); after the run, once the process is gone or the background session is stopped and removed, it deletes the session's transcript and tool result files in the CLI's configuration folder; the server has ended the agent session with `computer_use` (CU-04), so the next run gets `session.mode = new`; a failure logs `computer_transcript_left` and names it in the next run's live box (CU-T22).
18. The probes of 27 16 for the CLI with the pinned version against the local stand-in: `--mcp-config` with a `stdio` server and its `env` in a background session, and the transcript and tool result layout after a turn whose MCP tool returned an image; the results recorded in 27 16 and the runner README, the fallback applied where a probe fails.
19. End to end (CU-T24): Compose stack with server, PostgreSQL, the host service with `host_service.computer_use`, the Linux test backend in Xvfb with the fixture app, the runner with a scripted fake Claude that calls the MCP tools; the test enables the desktop, opens, clicks a button, types, zooms, checks pixels of the returned images, simulates a lock (pause and resume), stops from the API and checks that no key stays pressed and that the server holds no image byte; it runs in CI through the existing end-to-end entry point.
## Scope

Create:

- In `plugins/claude-code-runner/src/claude_code_runner/`: the computer module (per-turn file, request folder service, end of a computer run) with tests; `plugins/claude-code-runner/tests/real_cli/` probe tests for 27 16.
- The Compose end-to-end test under `tests/e2e/` and its services in `infra/compose/` (a profile used only by the test).
- `docs/decisions.d/CU-06.md`.

Change:

- `config.py`, the turn launch for print and background mode, the system prompt builder, `materialize.py` (sweep), `plugins/claude-code-runner/README.md`, the runner fixtures where the key is on, `infra/compose/e2e.sh`, `docs/spec/27-computer-use.md` section 16 and 10 where the probe or the code needs a precise name, `docs/spec/09-agents.md` 18.2 only if a name changes, `docs/operations/configuration.md` through `make config-reference`.

## Out of scope

- The Hermes adapter (CU-09), unless loading the tool there turns out trivial; then a separate ruling and test.
- The MCP server itself (CU-05), the host service (CU-03), the server (CU-04), the clients (CU-07).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/CU-06-runner-computer-use`, created from the branch of the stage that holds CU-01; one writer per worktree.
- Conventional Commits with scope and a DCO sign-off on every commit (`git commit -s`), no co-author trailers.
- Do not push and do not open a pull request: the stage lands as one pull request with CU-01 to CU-07 and one CI round (PLAN P-30, `AGENTS.md`); the coordinator merges your branch into the stage branch. Never push to `main`, never tag, never create remote repositories.
- Do not modify files outside the scope above, except the CHANGELOG fragments, README and `docs/decisions.d/` entries named in this brief, each listed in the report.
- Follow the host rules supplied by the launcher (ports, machine rules); no GUI or input test on a maintainer's machine and no run with a real model; native GUI tests run only in CI jobs.
- Tests bind only `127.0.0.1` and port 0; PostgreSQL and other services run only as throwaway containers. Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, `t.TempDir()` or `tmp_path`, removed at the end.
- Never read or print secrets. Tokens, attach tokens, DPoP keys, signing material and screenshots never appear in logs, test output, failure messages, audit entries or files outside the places this brief names.
- No real model call and nothing that costs money; a check of the pinned CLI runs only without a login against a local stand-in for the API on `127.0.0.1`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs beyond what Scope names, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before its dependencies were built, under the rule of D-W-100: where the merged code of a dependency names a package, type, function, file, operation or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from `docs/spec/27-computer-use.md`, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that CU-05 is merged into the stage branch (`bothy-computer-mcp` builds), else `Verdict: BLOCKED dependency CU-05 not merged`.
2. **Same pattern as `bothy-wake`.** Reuse the request folder, nonce and answer file code of 09 18.19.8 where it is built; where it is not built yet, write the computer part so that both can share it later and record that in the rulings.
3. **Deleting the transcript.** Only after the turn's process is gone (print) or after `claude stop` and `claude rm` (background); read the exact paths from the recorded layout of the probe, never by guessing a folder name; refuse to delete anything outside the CLI configuration folder of the turn.
4. **The scripted agent** is the fake Claude of the runner tests extended to emit MCP tool calls; no model runs anywhere.
5. **Rulings for `docs/decisions.d/CU-06.md`:** the probe results, the fallback taken, how the runner learns that a session was opened, every name that differs from chapter 27.

## Tests (write them first)

- Python tests: the command line per class and mode with and without the key (CU-T21); the per-turn file mode and removal; the nonce check; no token in environment or files; the end of a computer run against the recorded layout (CU-T22); the start refusal for a bad `runner.computer_mcp_bin`.
- The probes of 27 16 under `tests/real_cli/` against the local stand-in (no login, no cost).
- The Compose end-to-end test CU-T24.

```sh
make runner-test
make config-reference && make config-doc-check
make check
```

## Acceptance criteria

- [ ] CU-T21, CU-T22 and CU-T24 are green; without the key the runner equals its fixtures of before.
- [ ] The probe results stand in 27 section 16 and the runner README.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- An entry `changelog.d/CU-06.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/CU-06.md`, one row each with date, ruling and reason; every name that differs from chapter 27 is a ruling and a change of chapter 27 in the same commit.
- An ADR only if you take an architecture decision beyond ADR 0078 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green for the components you changed.
- An independent review of your diff by a fresh reviewer (one follow-up round at most); record its verdict. The red team of the stage runs once over the whole stage diff before the pull request; it checks at least, for this ticket: no credential of the runner reaches the MCP server or the turn; the tool never loads below `host`; a request without the run's nonce is refused; no screenshot stays on the runner host after a computer run; the end-to-end test proves that the server stores no image byte.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. The review verdict and the findings fixed.
4. What is not proven (at least: a run with a real model on a real Mac or Windows desktop, which the owner evaluates by hand after the merge), each probe of `docs/spec/27-computer-use.md` section 16 you ran with its result, and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
