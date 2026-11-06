# AR-02: Runner: orderly stop never orphans a turn

Status: ready | Depends on: none (specification merged) | Milestone: AR

## Goal

After this ticket an orderly stop of the Claude Code runner (SIGTERM, SIGINT) never leaves an orphaned turn. Today the cancellation arrives inside `_run_process`, whose `finally` has already set `self.process = None`, so the `except CancelledError` branch of `RunTurn.run` skips `terminate` and the outer `finally` deletes the turn record and the run files while the CLI lives on in its own session, holds the thread and session locks and loses its background work at the CLI's ceiling (design note 2.2, reproduced). The fix keeps the process handle until the turn is known to be gone, stops every live turn as 09 18.6 says, finishes its run `cancelled` with `runner_stopped`, and removes record and run files only after the process exited (09 18.19.2, R-AG-56). No key: this is today's behaviour corrected, and it may land before the rest of the stage.

## Read first

- `docs/spec/09-agents.md` 18.2 (row "Detachment"), 18.6, 18.9 (crash of the runner, adoption), 18.19.2, R-AG-35, R-AG-56; AR-T01 in 18.19.13.
- `docs/spec/16-operations.md` 10.1 item 4 (restarts while no turn runs until the handover is on).
- Code on the base branch: `plugins/claude-code-runner/src/claude_code_runner` (`__main__.py`, `app.py`, `runturn.py` (`RunTurn.run`, `_run_process`, the `except CancelledError` branch and the outer `finally`), `stop.py`, `turn.py`, `causes.py`), `sdk/python/src` (`adapter.py`: the cancellation of run tasks at SIGTERM), `tests/scenarios/test_crash.py`, `tests/fake_claude/`.
- Design note `docs/design/autonomous-runs.md` (context and evidence; where it differs from the chapters, the chapters win); ADR 0077; `docs/decisions.d/AR-01.md`.

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

12. On SIGTERM or SIGINT of the runner, every live turn is stopped by the 18.6 escalation (SIGINT, SIGTERM after `runner.stop_grace`, SIGKILL after `runner.kill_grace`, each escalation logged `stop_escalated`), and only after `os.killpg(pgid, 0)` raises `ProcessLookupError` is its run finished `cancelled` with the summary code `runner_stopped` (R-AG-35 form) and its record and run files removed.
13. The handle the cancel path uses is never cleared before the turn is gone; a cancellation in any await of `_run_process` reaches the stop.
14. The runner's own exit waits for these stops, bounded by `runner.stop_grace` plus `runner.kill_grace` per turn in parallel, then exits 0 as today. A turn whose group is still present at that bound is not finished: its record stays, `stop_escalated` names the remaining process, and the server ends its run as 09 5.5 says (`interrupted` or `stale`, 18.19.2); a test holds this with a process that ignores the escalation until after the bound.
15. A turn whose run the server already ended is stopped the same way without a finish (18.6).

## Scope

All paths under `plugins/claude-code-runner/` unless they start with `docs/` or `sdk/`.

Change:

- `src/claude_code_runner/runturn.py`: keep the process handle through the cancel path; stop, then finish, then remove.
- `src/claude_code_runner/app.py`, `__main__.py`: wait for the stops at shutdown; log `runner_stopped` once per turn.
- `src/claude_code_runner/causes.py`: the summary of `runner_stopped`.
- `tests/scenarios/test_orderly_stop.py` (new): AR-T01 with a real runner process and the fake CLI.
- `README.md` of the runner (stop behaviour), `docs/spec/09-agents.md` only if a sentence of 18.19.2 must name a merged function differently.

## Out of scope

- The handover, the keeper and everything behind `runner.turn_handover` (AR-03).
- Any change to the server or the SDK beyond what the stop needs (the SDK keeps cancelling its run tasks; the runner's task handles the cancellation).

## Boundaries

- Work in a dedicated git worktree on the branch `fix/AR-02-runner-orderly-stop`; one writer per worktree.
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

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that `docs/spec/09-agents.md` has 18.19.2, else `Verdict: BLOCKED specification missing`.

2. **Reproduce first.** Write the scenario test of AR-T01 before any change: a real runner process with a live turn of the fake CLI, then SIGTERM; today it shows the turn alive after the runner exited, the record gone and no `stop_escalated` (design note 2.2 probe). Keep that red output for the report.

3. **Where to stop.** The stop runs in the task of the turn (shielded from a second cancellation), so its locks are released only after the group is gone; a second SIGTERM during the stop changes nothing.

4. **Finish.** `finishRun` with `cancelled` and the summary of `runner_stopped`; when the server cannot be reached the decided end stays in the record as today (18.9) and the next start repeats it.

5. **Rulings for `docs/decisions.d/AR-02.md`**: the bound of the shutdown wait, the summary text.

## Tests (write them first)

- `tests/scenarios/test_orderly_stop.py` (AR-T01): SIGTERM and SIGINT to a runner with one and with two live turns (one with a running background job of the fake CLI); asserted: `stop_escalated` per escalation, every run finished `cancelled` with `runner_stopped`, record and run files removed only after the group is gone (checked by polling during the stop), no process of any turn group left, the locks free for a new runner.
- A turn whose run the server already ended: stopped, not finished.
- The existing crash scenarios (`tests/scenarios/test_crash.py`) stay green.

```sh
(cd plugins/claude-code-runner && uv run --locked pytest -q)
make check
```

## Acceptance criteria

- [ ] AR-T01 was red before the change and is green after it.
- [ ] No orphan, no record removed before its process exited, every run ends `cancelled` with `runner_stopped`.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- The runner README describes the orderly stop.
- An entry `changelog.d/AR-02.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/AR-02.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0077 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: no path of an orderly stop leaves a live process of a turn group; no record or run file is removed before its process exited; a stop never finishes a run the server already ended.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: the behaviour under a real service manager (AR-09)), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
