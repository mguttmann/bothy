# AR-07: Runner: end state

Status: ready | Depends on: AR-05 | Milestone: AR

## Goal

After this ticket, with `runner.end_state`, every answer of a runner turn ends with `STATE: done`, `STATE: waiting` or `STATE: blocked`, the runner strips and stores it, nudges once per waiting phase when nothing will wake the thread and then posts a note, reports the idle flag of each autonomous input and notification turn, and passes `runner.cli_bg_wait_ceiling` to the CLI when set (09 18.19.11, 18.19.12, 5.8.6).

## Read first

- `docs/spec/09-agents.md` 5.8.6, 18.5, 18.19.7, 18.19.11, 18.19.12, R-AG-55; AR-T12 to AR-T15 in 18.19.13; 21 (`runner.end_state`, `runner.cli_bg_wait_ceiling`).
- Design note backlog item 23 (idle count and agents that wait for several CI runs).
- Code on the base branch: `plugins/claude-code-runner/src/claude_code_runner` (`title.py` and the `TITLE:` handling, `processstate.py`, `runturn.py`, `followups.py`, `environment.py`, `prompt.py`, `prompts/runner_prompt.md`, `wakes/`).
- Design note `docs/design/autonomous-runs.md` (context and evidence; where it differs from the chapters, the chapters win); ADR 0077; `docs/decisions.d/AR-01.md`.
- `docs/decisions.d/FIX-RUNNER-LIMIT-KEEPS-RUN.md` and 09 18.9 "Account errors" (added 2026-11-13, after this brief was written): after an account error of a held process the runner writes an input line of its own, the retry line (`account.py` `retry_input`, `RunTurn._account_retry`). Its echo counts in `ProcessState._input_taken` like an input of a person: it clears the nudge mark and sets the count of autonomous inputs to zero. Where this ticket reads or writes those marks, flag the retry line as neither a person's nor an autonomous input.

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
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) changes in this ticket only where Scope names it: the catalogue rows of `docs/spec/actions.yaml` that R-AZ-37 assigns to this ticket and their generated copies (`make catalog`).
11. Additive only (09 18.19.1, ADR 0077): every new column is nullable or defaults to "as before", every key defaults to off, and an installation that sets none of the new keys behaves exactly as before.

Note on rule 1: "stages" are the milestones and the stages A1 and AR of the plan (PLAN P-27, P-29).

Product requirements implemented here:

12. The marker handled like `TITLE:`: removed from the posted text, stored as `end_state` with `wake_pending_until` in the run's summary and metrics (03 `runs.metrics`); missing or malformed is `waiting`; never posted.
13. The nudge of 18.19.11 once per waiting phase, the note with `continuation_without_wake`, no nudge for `chat`, the nudge mark in `ProcessState` and its replay.
14. The idle flag per autonomous input and per notification turn as a patch of `/metrics/autonomy_last`; stdin closes at the `result` whose patch answer carries `autonomy_cap_reached`, with the note.
15. Decide with a test whether reading a background task's output is a tool call that makes a turn count as work (design note backlog item 23), and record the decision.
16. `runner.cli_bg_wait_ceiling` reaches the environment as `CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS` only when set; the 18.2 allow list names it.

## Scope

All paths under `plugins/claude-code-runner/` unless they start with `docs/`.

Create: `src/claude_code_runner/endstate.py`, its tests, `tests/scenarios/test_end_state.py`, `docs/decisions.d/AR-07.md`.

Change: `title.py` (shared marker parsing), `processstate.py`, `runturn.py`, `environment.py`, `prompts/runner_prompt.md`, `config.py`, `config.example.toml`, `README.md`; `docs/spec/09-agents.md` 5.8.6 only if the decision of requirement 15 changes a sentence.

## Out of scope

- The server side of the caps (AR-04); `bothy-wake` (AR-05).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/AR-07-runner-end-state`; one writer per worktree.
- Conventional Commits with scope and a DCO sign-off on every commit (`git commit -s`), no co-author trailers; squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG fragments, README and `docs/decisions.d/` entries named in this brief, each listed in the report.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL and other services run only as throwaway containers (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, `t.TempDir()` or `tmp_path`, removed at the end.
- Never read or print secrets. Tokens, DPoP keys, secret values and secret references resolved to values never appear in logs, test output, failure messages, audit entries or files outside the places this brief names.
- No real model call and nothing that costs money; a check of the pinned CLI runs only without a login against a local stand-in for the API on `127.0.0.1` (the method of `plugins/claude-code-runner/tests/real_cli/captured`).
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs beyond what Scope names, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before its dependencies were built, under the rule of D-W-100: where the merged code of a dependency names a package, type, function, file, operation or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from `docs/spec/09-agents.md` 5.8 or 18.19, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that the base has AR-05 (`wakes/` with the pending list), else `Verdict: BLOCKED dependency AR-05 missing`.

2. **Pending.** "Nothing pending" means no pending wake-up of the thread in the runner's list of 18.19.8 and no running background task; the list is the one `run.wake.list_pending` and `run.wake_changed` keep.

3. **Rulings for `docs/decisions.d/AR-07.md`**: the marker grammar, the nudge text, the decision of requirement 15.

## Tests (write them first)

- `tests/scenarios/test_end_state.py`: AR-T12 with every case; AR-T15; the runner parts of AR-T13 and AR-T14 against the fake server, and against the server of AR-04 in `tests/realserver`.

```sh
(cd plugins/claude-code-runner && uv run --locked pytest -q)
make adapters-e2e
make check
```

## Acceptance criteria

- [ ] AR-T12 and AR-T15 are green; AR-T13 and AR-T14 are green with the server.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- The runner README describes the end state, the nudge and the ceiling key.
- An entry `changelog.d/AR-07.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/AR-07.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0077 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: the marker is never posted; one nudge per waiting phase; a `waiting` end with nothing pending is always visible; the idle cap stops a self-polling held process; without the keys the fixtures are byte-equal.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: how often real agents omit the marker), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
