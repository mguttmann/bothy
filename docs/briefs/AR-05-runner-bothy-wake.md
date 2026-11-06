# AR-05: Runner: bothy-wake

Status: ready | Depends on: A1-05, AR-03 | Milestone: AR

## Goal

After this ticket, with `runner.wakes`, an agent in a runner turn asks for a wake-up with `bothy-wake` (`--after`, `--at`, `--pr`, `--until-success`, `--list`, `--cancel`), the runner sends it to the server with the run's token, evaluates `pr_checks` and `command` wake-ups with `run.wake.check` before every execution, fires them, takes wake inputs into a held turn after their checks, disallows the CLI's own timers and reports the capability `wakes` (09 18.19.8).

## Read first

- `docs/spec/09-agents.md` 5.8 (all), 18.2, 18.5, 18.18, 18.19.1, 18.19.7, 18.19.8, 18.19.12 (rows on crons, `--disallowedTools` and the environment prefix), R-AG-49 to R-AG-52; AR-T06 to AR-T10 in 18.19.13; 21 (runner keys `runner.wakes` and `runner.wake_*`).
- `docs/spec/26-builtin-agents.md` 4.4 (the intersection of hosted tools) and 5.3.
- Code on the base branch: `plugins/claude-code-runner/src/claude_code_runner` (`commandline.py`, `toolclass.py`, `environment.py`, `followups.py`, `runturn.py`, `processstate.py`, `hosting.py`, `heartbeat.py`, `prompt.py`, `prompts/runner_prompt.md`, `config.py`), `sdk/python/src` (outbox, inbox), `tests/`.
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

12. `runner.wakes` (default off) needs `runner.turn_handover` (the runner refuses to start with only `runner.wakes`); it reports `wakes: true` only while on; AR-T10 without the key holds byte for byte.
13. The command folder first on the cleaned `PATH` (owned by the runner user, not writable by group or others), `Bash(bothy-wake:*)` in `--allowedTools` under `scoped` (for a hosted agent after the intersection), the five timer tools in `--disallowedTools`.
14. The request folder `<runner.state_dir>/wake-requests/<run_id>/` with the per-run nonce of `BOTHY_WAKE_NONCE`; requests with another nonce are refused and logged; the answer file and the exit codes of 18.19.8; "accepted, not confirmed" when the request waits in the SDK outbox, which sends it later with `deferred: true` (18.19.8).
15. Condition checks exactly as 18.19.8: `run.wake.check` before every execution, the `command` environment, folder, group, timeout, output cut, backoff, `runner.wake_check_failures`, the head-commit rule of `pr_checks`, the stop of running checks at a handover.
16. `run.wake.list_pending` at start and after every reconnect, per hosted agent connection too; `run.wake_changed` between reads.
17. Wake inputs written with the checks of 09 5.8.3 like follow-ups, a skip reported to the server; a wake input's `result` leaves stdin open while a task runs (18.19.7).
18. The runner prompt section "Background work" says that a promise to continue needs a running background task or a wake-up and names `bothy-wake` with its forms.
19. The durable cron file of 18.19.8: logged `durable_cron_present` once per run and named in the live box; a test against the local API stand-in decides whether the pinned CLI fires such a file in a print turn, and the result goes into 18.19.12.

## Scope

All paths under `plugins/claude-code-runner/` unless they start with `docs/`.

Create:

- `src/claude_code_runner/wakes/` (the command entry point `bothy_wake.py` installed into the command folder, `requests.py`, `checks.py`, `inputs.py`), their tests, `tests/scenarios/test_wakes.py`, `tests/real_cli/test_timer_tools.py` (the tool list of `system/init` with the five tools disallowed under `bypassPermissions`, against the local API stand-in).
- `docs/decisions.d/AR-05.md`.

Change:

- `commandline.py`, `toolclass.py`, `environment.py` (the two variables), `hosting.py`, `heartbeat.py`, `followups.py`, `runturn.py`, `prompts/runner_prompt.md`, `config.py`, `config.example.toml`, `README.md`; `docs/spec/09-agents.md` 18.19.12 rows with the captured results.

## Out of scope

- Recovery and usage-window wake-ups (AR-06); the end-state check (AR-07); the server (AR-04); clients (AR-08).
- `bothy-wake` for tool class `chat`.
- Switching `runner.wakes` on in an installation: it stays off until AR-06 (recovery) and AR-07 (end state and idle count) are merged too, because without them lost work or idle notification turns could end without the note or the cap of 5.8.6; the service guide (AR-09) says so.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/AR-05-runner-bothy-wake`; one writer per worktree.
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

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that the base has AR-03 (the keeper and `ProcessState`) and A1-05 (the hosted command line with its intersection), else `Verdict: BLOCKED dependency <ticket> missing` naming the missing one.

2. **Command.** A small Python entry point without third-party imports beyond the standard library, so it starts fast in the turn's environment; it parses the forms, writes the request atomically, waits for the answer file at most `runner.wake_request_timeout` and prints one line.

3. **Checks.** One asyncio task per due check, bounded by `runner.wake_checks_max_parallel`; `gh` and `/bin/sh` from the cleaned `PATH`; the result text cut at a UTF-8 boundary and cleaned of control and bidi characters before it leaves the host.

4. **Rulings for `docs/decisions.d/AR-05.md`**: the request and answer file format, the `--at` parsing, the result text format, the durable cron result.

## Tests (write them first)

- Unit tests per form of the command, the nonce check, the answer and exit codes, the environment of a check, the timeout, the backoff, the result cut.
- `tests/scenarios/test_wakes.py`: AR-T06, AR-T07, the runner parts of AR-T08 and AR-T09, AR-T10, against the fake server; a hosted agent gets the entry after the intersection.
- `tests/real_cli/test_timer_tools.py` against the local API stand-in.

```sh
(cd plugins/claude-code-runner && uv run --locked pytest -q)
make adapters-e2e
make check
```

## Acceptance criteria

- [ ] AR-T06 to AR-T10 are green; a `command` never runs without an allow of `run.wake.check` in the test logs.
- [ ] The captures of 18.19.12 are recorded, or reported as not proven.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- The runner README describes `bothy-wake`, the request folder, the checks and the keys; `config.example.toml` shows the keys off.
- An entry `changelog.d/AR-05.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/AR-05.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0077 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: no `command` or `gh` read runs without an allow of `run.wake.check` and a matching actual tool class; a turn cannot file a request for another run; the token never reaches the turn; the timer tools are disallowed with the key; without the key the command line is byte-equal.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: the behaviour of a real `gh` against GitHub (fake `gh` in tests)), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
