# AR-03: Runner: turn handover

Status: ready | Depends on: AR-02, AR-04 | Milestone: AR

## Goal

After this ticket, with `runner.turn_handover`, a restart of the Claude Code runner hands every live turn over instead of ending it, and the next runner adopts it with full rights: the keeper leads the turn's own session and holds the write end of the turn's named pipe, every attempt of a run starts through the launch handshake, the turn record carries the run's phase and `process_start`, the adopter replays `ProcessState` from the stream and takes follow-ups, wake inputs, the stdin rule and the stop after an error, and the end of an adopted process is handled like an own one (09 18.19.3 to 18.19.7). Without the key the runner stays byte-equal to today's fixtures.

## Read first

- `docs/spec/09-agents.md` 4.8 (closed capabilities), 4.9, 5.5, 18.2, 18.5, 18.6, 18.9, 18.18, 18.19.1 and 18.19.3 to 18.19.7, R-AG-45 to R-AG-48; AR-T02, AR-T03, AR-T04, AR-T16 in 18.19.13.
- `docs/spec/16-operations.md` 10.1.
- `docs/decisions.d/M2-35.md` (D-M2-35-3 and its current state, D-M2-35-11, D-M2-35-28).
- Design note sections 2.2, 2.4 (the keeper probe), 2.5 (the inventory of per-turn state and both replay probes) and backlog items 3, 6, 7, 8, 9 and 12. The line numbers of section 2.5 are hints only and some are off (GitHub issue 556 item 1: `background.apply` is called at `runturn.py` 749, `BackgroundWork()` is created at 686, `_held_logged` is reset at 687, `_idle` spans 799 to 840); follow the code.
- Code on the base branch: `plugins/claude-code-runner/src/claude_code_runner` (`runturn.py`, `turn.py`, `adoption.py`, `app.py`, `ledger.py`, `background.py`, `followups.py`, `streamreader.py`, `recovery.py`, `failures.py`, `locks.py`, `stop.py`, `heartbeat.py`, `config.py`), `sdk/python/src` (`adapter.py`: the cancellation at SIGTERM and the inbox restart), `tests/scenarios/`, `tests/fake_claude/`, `tests/real_cli/`.
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

12. `runner.turn_handover` (default off) turns on everything below and the heartbeat capability `turn_handover: true` (only while on); without it AR-T16 holds byte for byte.
13. The keeper `python -m claude_code_runner.keeper` behaves exactly as 18.19.4 items 1 to 5 say (session leader, named pipe, identity files, release of the CLI after `<launch>.cli`, the initial inputs written without blocking and `<launch>.fed` only after the last byte, signals ignored, wait status to `<launch>.exit`, exit code of the CLI, SIGUSR1 closes its write end, no lock descriptor kept).
14. Every attempt (first, compaction, retry, `ResumeSameReference`, `NewSession`) starts through the launch handshake of 18.19.5 with the record written before each step; the next runner handles a record in `launching` by the four cases of 18.19.5, reading `<launch>.cli` once more after the keeper's group is gone.
15. The record carries the phases of 18.19.6, the recovery flags, the highest outage number reached before the attempt, `inputs_written`, `go_sent` and `stdin_closed`; `process_start` lives in the ledger's part of the record and is kept by every adopter.
16. `ProcessState` holds every value of 18.19.7 and is the only writer of them; the adopter replays the lines from `process_start` to `offset` through it, then reads on; the replay posts, patches and plans nothing.
17. SIGTERM hands over (18.19.3): no new runs, no further heartbeat, live turns left running, in-flight input lines finished or recorded with their byte count, `turn_handed_over` per turn, exit 75 when at least one turn was handed over; `claude-code-runner stop --turns` stops every turn as in 18.6 and exits 0.
18. The adoption rule and the launch check of 18.19.6: an adopted end is classified and recovered like an own one within the once-per-run limits of the record; before any launch of a run it did not accept, the runner calls `run.launch.check` and launches only on allow.
19. An adopter takes stdin only for a record with a named pipe, `stdin_closed = false` and a process that is no compaction; every close records `stdin_closed = true` first (issue 556 item 2); a record without a named pipe is adopted as 18.9 says (issue 556 item 4).

## Scope

All paths under `plugins/claude-code-runner/` unless they start with `docs/` or `sdk/`.

Create:

- `src/claude_code_runner/keeper.py` (the keeper program), `src/claude_code_runner/launch.py` (the handshake and the cases of a `launching` record), `src/claude_code_runner/processstate.py` (`ProcessState` and its replay), `src/claude_code_runner/handover.py` (SIGTERM handover and `stop --turns`).
- Tests: `tests/unit/test_keeper.py`, `tests/unit/test_launch.py`, `tests/unit/test_processstate.py`, `tests/scenarios/test_handover.py`, `tests/scenarios/test_launch_crash_points.py`, `tests/real_cli/test_named_pipe.py` (the pinned CLI with a named pipe against the local API stand-in).

Change:

- `runturn.py`, `turn.py`, `adoption.py`, `app.py`, `ledger.py` (`process_start` in `to_record` and `from_record`), `background.py`, `followups.py`, `recovery.py`, `stop.py`, `heartbeat.py` (capability), `config.py` and `config.example.toml` (the key, off), `__main__.py` (`stop --turns`, exit 75).
- `sdk/python/src` (`adapter.py`): a hook that lets the runner hand over at SIGTERM instead of cancelling its run tasks, with a test in the SDK; nothing changes for an adapter that does not use it.
- `README.md` of the runner and of the SDK; `docs/spec/09-agents.md` 18.19.12 row "Named pipe as stdin" with the result of the capture.

## Out of scope

- `bothy-wake`, condition checks and wake-up creation (AR-05); recovery and usage-window wake-ups (AR-06); the end-state check (AR-07). The runner takes wake inputs here only as far as 18.19.7 needs (written like follow-ups after their own checks of 5.8.3).
- The service guide and the launchd test (AR-09).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/AR-03-runner-turn-handover`; one writer per worktree.
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
- Crash points for tests are a test-only hook (an environment variable read only by the test build of the runner or an injected callable); they are never reachable in a normal start.
- Never send a signal to a process group that the record does not name and `process_matches` does not confirm.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that the base has AR-02 (the orderly stop of 18.19.2) and AR-04 (`run.launch.check` and the heartbeat capability `turn_handover` on the server), else `Verdict: BLOCKED dependency <ticket> missing` naming the missing one.

2. **Keeper.** A small module without imports of the runner's async code: `os.open` of the named pipe (`O_RDONLY | O_NONBLOCK`, then `O_WRONLY`, then clear `O_NONBLOCK` on the read end), identity file by write and rename, `os.fork`, a release pipe for the child, `os.execvpe` with the argument list and environment the runner passed in a file next to the record (mode `0600`), `signal.SIG_IGN` for INT, TERM, HUP and PIPE in the keeper and `SIG_DFL` in the child before `exec`, a `SIGCHLD` wake-up with `waitpid(WNOHANG)` in the feed loop, `SIGUSR1` closes the write end.

3. **ProcessState.** Move every stream-derived value that `_read` and `_idle` read into the object (the inventory of design note 2.5: `BackgroundWork`, `last_result_at`, `_failed_at` and `_lost_tasks`, task descriptions, usage mark and usage error mark with the outage number, nudge mark, idle flag); `_idle` and the stop rule read only it, the record's follow-ups, the run's stop state and the configuration. The replay calls the same `apply` with the adoption time.

4. **Class test (issue 556 item 3).** For every recorded fake stream of the scenarios and every checkpoint position in it, run the old runner's read and the adopter's replay plus read side by side on a fake clock and compare the sequence of stdin and stop decisions (hold, close, stop, with their times shifted by the adoption); a value kept outside `ProcessState` changes the sequence and fails the test.

5. **In-flight writes (design note backlog item 6).** Record the line before writing, mark it written after the last byte; at a handover wait up to `runner.kill_grace`, then record `partial_input` with the byte count; the adopter writes the rest first; a line marked started without a count after a crash closes stdin (18.19.3).

6. **Outage number (issue 556 comment).** The `launching` record of each attempt carries the highest outage number reached before it; `ProcessState` of the attempt starts from it in the live read and in the replay. AR-06 builds the usage mark; this ticket carries the number through record and replay and tests it with a synthetic frame.

7. **Real CLI.** `tests/real_cli/test_named_pipe.py` runs the pinned CLI with a named pipe as stdin against the local API stand-in of `tests/real_cli/captured` (no login, no model call): two input lines written through the pipe, one before and one after a reopen of the write end by a second process, each answered by the stand-in; the result goes into 09 18.19.12.

8. **Lock descriptors (design note backlog item 3).** The keeper closes its copies after the fork; a test asserts that the keeper's `/dev/fd` (or `lsof` equivalent) holds no lock file and that a new runner can take the locks once the CLI exited.

9. **Rulings for `docs/decisions.d/AR-03.md`**: the file layout of a launch, the SDK hook, the replay comparison, the crash-point hook, every name that differs from 18.19.

## Tests (write them first)

- `tests/unit/test_keeper.py`: every item of 18.19.4, also a CLI that stops reading inside an initial input larger than the pipe buffer and exits (no block, no `<launch>.fed`, `<launch>.exit` written), a launch pipe closed without a go byte (`not_started`, no CLI), SIGUSR1, a CLI killed by a signal (exit 128 plus the number).
- `tests/unit/test_processstate.py`: the replay against the live read for the cases of design note 2.5 (quiet window, offset past the result, error result before the checkpoint, task running) and the class test of design note 4.
- `tests/scenarios/test_handover.py`: AR-T02 portable with variants (a) to (d), AR-T03.
- `tests/scenarios/test_launch_crash_points.py`: AR-T04 with every crash point, every attempt kind, the outage-number variant, the keeper killed before `<launch>.fed`, and the launch check refusals and an unreachable server.
- `tests/unit/test_byte_equal_baseline.py` (or the existing fixture tests): AR-T16 without the key.

```sh
(cd plugins/claude-code-runner && uv run --locked pytest -q)
(cd sdk/python && uv run --locked pytest -q)
make adapters-e2e
make check
```

## Acceptance criteria

- [ ] AR-T02 (portable), AR-T03, AR-T04 and AR-T16 are green.
- [ ] The class test compares the decision sequences for every recorded stream and checkpoint position.
- [ ] The capture of the named pipe with the pinned CLI is recorded in 09 18.19.12, or reported as not proven.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- The runner README describes the keeper, the launch files, the phases, the handover, `stop --turns` and the key; `config.example.toml` shows the key off; the SDK README describes the hook.
- An entry `changelog.d/AR-03.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/AR-03.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0077 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: a runner crash at any point of any attempt leaves one recorded CLI or none and never an input answered twice; no CLI runs that no record names; an adopter never writes into a closed stdin or a compaction; the replay posts nothing; the launch check runs before every launch of a run the runner did not accept; without the key the fixtures are byte-equal.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: the macOS launchd variant (AR-09); systemd with `KillMode=process`), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
