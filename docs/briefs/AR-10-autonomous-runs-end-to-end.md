# AR-10: End to end and rollout

Status: ready | Depends on: AR-06, AR-07, AR-08, AR-09 | Milestone: AR

## Goal

After this ticket the demo stack with the fake Claude proves the stage end to end: an agent waits for the checks of a fake pull request across a runner restart, wakes and finishes; the chain cap is reached and a reply continues; a usage-window end continues at the reset. The rollout steps 4 to 6 of the design note section 9 stand in the service guide.

## Read first

- `docs/spec/09-agents.md` 5.8, 18.19; `docs/spec/16-operations.md` 10.1; `docs/spec/01-product.md` 6.21.
- Design note section 9; `docs/operations/claude-code-runner.md` (AR-09).
- Code on the base branch: `tests/e2e/`, `infra/compose/demo.yaml`, the demo instance of 16 section 16.
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

12. An end-to-end test on the demo stack with the fake Claude and a fake `gh`: `bothy-wake --pr` in a turn, a runner restart while the wake-up is pending, the checks turn green, the wake run resumes the session and ends `done`.
13. An end-to-end test of the chain cap with a small `run.autonomous_chain_max`: the note appears, a reply of the person continues.
14. An end-to-end test of a usage-window end with a synthetic frame: the run ends `usage_limited`, the wake-up fires after the reset and the thread continues.
15. The service guide gets the rollout steps 4 to 6 (keys `runner.wakes`, `runner.end_state`, raising `run.autonomous_chain_max`).

## Scope

Change: `tests/e2e/`, `infra/compose/demo.yaml` (keys on for the demo runner), `docs/operations/claude-code-runner.md`, `docs/decisions.d/AR-10.md`.

## Out of scope

- Any behaviour change in server, runner or clients; a real model or a real GitHub.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/AR-10-autonomous-runs-end-to-end`; one writer per worktree.
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

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that the base has AR-06, AR-07, AR-08 and AR-09, else `Verdict: BLOCKED dependency <ticket> missing` naming the missing one.

2. **Rulings for `docs/decisions.d/AR-10.md`**: the demo keys and the fake `gh`.

## Tests (write them first)

- The three end-to-end tests of requirements 12 to 14.

```sh
make image
make demo-e2e
make docs
make check
```

## Acceptance criteria

- [ ] The three end-to-end tests are green.
- [ ] The guide carries the rollout steps 4 to 6.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- The demo README names the keys of stage AR it turns on.
- An entry `changelog.d/AR-10.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/AR-10.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0077 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: the tests use only fakes; the restart in the test is a real runner restart with the handover; the stage's red team list of PLAN section 22 holds end to end.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: a real account, a real GitHub and a real model), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
