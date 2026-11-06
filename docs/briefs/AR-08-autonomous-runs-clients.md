# AR-08: Clients: autonomous runs

Status: ready | Depends on: AR-04 | Milestone: AR

## Goal

After this ticket the desktop, the phone and the web console show a thread's pending wake-ups with cancel, the end state of each finished run, wake runs and recovery runs, the notes of wake-up ends and caps, the usage-limit notice, and the channel switch "Let agents continue on their own" (01 6.21). Wake-up controls appear only for an agent that reports `wakes`.

## Read first

- `docs/spec/01-product.md` 6.2, 6.21, R-PR-37; `docs/spec/09-agents.md` 5.8, 18.19.10, 18.19.11; `docs/spec/07-permissions.md` 2.14; `docs/spec/04-sync-protocol.md` 15.9; `docs/spec/15-client-architecture.md` (local database, outbox).
- Code on the base branch: `apps/client`, `packages/admin_ui`, `apps/console`, the Dart client generated from `openapi.yaml`, `packages/sync_engine`, `packages/l10n`.
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

12. Pending wake-ups above the composer with label, condition, earliest fire, deadline and cancel for the persons of 07 2.14 item 5; live through `run.wake_changed`.
13. The collapsed run line shows `end_state` and the next wake-up; wake runs show "Woke up: <label>", recovery runs "Continued after lost work".
14. The notes of 09 5.8.4, 5.8.6, 18.19.10 and 18.19.11 render as notes of the thread, translated, never as messages of a person.
15. The channel switch for persons with `manage`, with the sentence of 01 6.21 item 5; hidden on an older server.

## Scope

Change: `apps/client`, `packages/admin_ui`, `apps/console`, `packages/sync_engine` (local table of wake-ups), `packages/l10n` (strings in every shipped language), the Dart client (regenerated), their tests and READMEs; `docs/decisions.d/AR-08.md`.

## Out of scope

- The server (AR-04); any runner change.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/AR-08-autonomous-runs-clients`; one writer per worktree.
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

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that the base has AR-04 (`run.wake.read`, `run.wake_changed`), else `Verdict: BLOCKED dependency AR-04 missing`.

2. **Rulings for `docs/decisions.d/AR-08.md`**: the layout of the pending list, the grouping of a nudge answer with the run's answer (design note backlog item 15).

## Tests (write them first)

- Widget and integration tests per platform against the Dart fake server: pending list, cancel, live change, end state, wake and recovery runs, every note, the channel switch, hidden controls without `wakes` and on an older server.

```sh
make apps-platform
make e2e-desktop
make console-e2e
make check
```

## Acceptance criteria

- [ ] Every item of 01 6.21 is shown and tested on desktop, phone and console.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- The READMEs of the changed packages describe the new views.
- An entry `changelog.d/AR-08.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/AR-08.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0077 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: cancel is offered only to the persons of 07 2.14; notes never look like a person's message; nothing is shown for an agent without `wakes`.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: the look on a real device beyond the integration tests), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
