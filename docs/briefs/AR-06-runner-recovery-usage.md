# AR-06: Runner: recovery and usage-window wake-ups

Status: ready | Depends on: AR-05 | Milestone: AR

## Goal

After this ticket lost background work leads to a recovery run that lists it, or to a visible note, for every case of 09 18.19.9, and the usage window of the account marks a run usage-limited, keeps its background work and wakes the thread at the reset with one wake-up per outage (18.19.10). The frame and the error `result` of a refused request are captured from the pinned CLI against a local stand-in for the API.

## Read first

- `docs/spec/09-agents.md` 5.2 (`usage_limited`), 5.8.2 (`run.wake.recover`), 18.5, 18.9, 18.19.7, 18.19.9, 18.19.10, 18.19.12 (usage rows), R-AG-17, R-AG-53, R-AG-54; AR-T03 (keeper kill), AR-T05, AR-T11 in 18.19.13.
- Design note sections 2.3 and backlog item 18.
- Code on the base branch: `plugins/claude-code-runner/src/claude_code_runner` (`streamreader.py` `KNOWN_TYPES`, `processstate.py`, `runturn.py`, `recovery.py`, `failures.py`, `causes.py`, `adoption.py`, `wakes/`), `tests/real_cli/captured/`.
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

12. Lost-task detection for each case of 18.19.9 with `run.wake.recover` (reason `recovery`) within the autonomy rules, or the note; for a run the server still shows `running` with no `result` or decided end, the runner first finishes it `failed` with `runtime_error` and the summary code `turn_lost`.
13. The recovery run's first input lists each lost task (id, kind, description from `ProcessState`, how it ended) and says nothing was run again.
14. `rate_limit_event` in `KNOWN_TYPES`; the usage mark exactly as 18.19.10 (only `rejected` without overage sets it, unknown `status` logged and allowed, usage errors kept apart from the failure mark, no stop after an error while or after the mark, one wake-up per outage: `run.wake.create` keyed by its number, repeated with the same key when a later `rejected` frame moves `resetsAt`, which moves the pending wake-up's `not_before`, cancel with `usage_cleared` when the mark clears, `usage_limited` end, `run.wake.recover` with `usage_limit` after a restart).
15. The live box and the thread show the texts of 18.19.10.
16. `tests/real_cli/captured/usage_limit_<pinned version>.jsonl` holds the lines the pinned CLI writes for a refused request (`rate_limit_event` with `rejected`, and the error `result`), captured against a local stand-in that answers `429` with the headers of a used-up window (no login, no cost); `test_failures.py` classifies them, and 18.19.12 records the order of frame and `result`.
17. A live turn that adoption finds for a run the server ended `cancelled` or `completed` (states `run.wake.recover` does not continue) is stopped as in 18.9, and the runner still calls `run.wake.recover` with reason `recovery` and a label naming the lost tasks: the server stores that wake-up `refused` with `run_not_failed` (merged with issue 567), so the thread gets the note of 5.8.4 instead of a recovery run (18.19.9); the runner logs `recovery_wake` with the reason.
18. After a restart, `run.wake.recover` with `usage_limit` while the outage's wake-up from `run.wake.create` is pending changes nothing (the server answers that wake-up, issue 567); the runner keeps that one wake-up, cancels it with `usage_cleared` when the mark clears, and logs every end of it with its own code (`wake_cancelled`, `wake_expired`, `wake_fired`, `wake_refused`; 17, R-OB-11).
19. The usage error that set the mark stays exempt from the stop of 18.5 after the mark clears; a new error `result` read after the mark cleared is an ordinary error and leads to the stop of 18.5 (18.19.10, issue 567).

## Scope

All paths under `plugins/claude-code-runner/` unless they start with `docs/`.

Create: `src/claude_code_runner/usage.py` is taken (session usage); use `src/claude_code_runner/usagelimit.py` for the mark, `src/claude_code_runner/lostwork.py`, their tests, `tests/scenarios/test_recovery_runs.py`, `tests/scenarios/test_usage_window.py`, the capture file and its script under `tests/real_cli/`, `docs/decisions.d/AR-06.md`.

Change: `streamreader.py`, `processstate.py`, `runturn.py`, `recovery.py`, `failures.py`, `causes.py`, `adoption.py`, `README.md`; `docs/spec/09-agents.md` 18.19.12 rows on the usage frame with the capture.

## Out of scope

- The server (AR-04); `bothy-wake` (AR-05); the end-state check (AR-07).
- Polling the usage window; a usage-limit path without a `rejected` frame beyond today's `runtime_error`.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/AR-06-runner-recovery-usage`; one writer per worktree.
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

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that the base has AR-05 (`wakes/` and `run.wake.create` from the runner), else `Verdict: BLOCKED dependency AR-05 missing`.

2. **Stand-in.** Extend the local API stand-in of `tests/real_cli/captured` with an answer `429` carrying `anthropic-ratelimit-unified-status: rejected`, `anthropic-ratelimit-unified-representative-claim: five_hour`, `anthropic-ratelimit-unified-reset` and `retry-after`; run the pinned CLI with a fake key and an empty `HOME`, capture stdout, and keep only the lines the runner reads.

3. **Rulings for `docs/decisions.d/AR-06.md`**: the recovery text, the capture result, the summary of `turn_lost`.

## Tests (write them first)

- `tests/scenarios/test_recovery_runs.py`: each case of 18.19.9, with and without permission; AR-T05; the keeper-kill part of AR-T03; a live turn of a run the server ended `cancelled` and one it ended `completed`, each giving the note of `run_not_failed` and no recovery run (requirement 17).
- `tests/scenarios/test_usage_window.py`: AR-T11 with every variant, against synthetic frames and the captured lines; a restart while the outage's wake-up is pending leaves one wake-up (requirement 18); an error `result` while the mark is set, then a clearing frame, then a second error `result`: no stop for the first, the stop of 18.5 for the second (requirement 19).

```sh
(cd plugins/claude-code-runner && uv run --locked pytest -q)
make check
```

## Acceptance criteria

- [ ] AR-T03 (keeper kill), AR-T05 and AR-T11 are green.
- [ ] The capture of the refused request is committed and classified, or reported as not proven with the reason.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- The runner README describes recovery runs and the usage window.
- An entry `changelog.d/AR-06.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/AR-06.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0077 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: no recovery run repeats a lost task; a usage end never stops running background tasks, also not after the mark clears; one wake-up per outage; every lost-work case ends in a recovery run or a note.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: the frame order of a refused request with a real account), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
