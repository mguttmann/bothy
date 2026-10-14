# M7-18: Capacity load test

Status: ready | Depends on: M7-14 | Milestone: M7

## Goal

After this ticket the capacity table of 16 13 holds measured numbers instead of start values: a reproducible load test against the full stack from the images (the same installation path as the portability test) measures throughput and the lock time at the counter `event_seq` (`bothy.db.event_seq.lock_wait`), connections and memory per api node, fan-out lag, and the share of the indexes in the database size, on `perf.reference_profile`, and writes a report. A gate holds a minimum throughput as a named value: when a later change drops the measured throughput below it, the load test fails. M1-72 already confirmed or changed `database.max_conns`; this ticket fills the remaining rows and the inputs of the disk space formula. Why: instance operators size hosts from 16 13, and the serial section at the counter is the known limit of the architecture (02 16); numbers from a test replace guesses.

## Read first

- `docs/spec/16-operations.md`: 13 (capacity table, levers, disk space formula, `bothy.storage.volume.free`), 11.1 (the installation path), 14.
- `docs/spec/02-architecture.md`: 16 (the limit of `event_seq`), 17 (performance targets and their tests), 22 (named values); `docs/spec/04-sync-protocol.md` 3.2 (counter pattern).
- `docs/adr/0063-performance-gate-for-the-event-log.md` (when and on which environment), `docs/observability/signals.md` (the metrics named in 16 13).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-18-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-72-performance-gate.md` (`make perf`, `perf.reference_profile`, the existing perf tests and their output format), `M7-14-portability-test.md` (installation from a trial release, `make portability-trial`).
- Code on the base branch: `services/server/perf/`, `tests/portability/`, `Makefile` (`perf`, `portability-trial`).

This ticket touches no table, no action and no event.

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
8. White label: generated load data uses the people Alex, Sam, Robin and numbered test people `person-<n>@example.com`; no company, host or personal names.
9. Thresholds only as named values (02 22, `docs/operations/configuration.md`); the report and 16 13 cite numbers only as measured results with profile, commit and date, never as rules.
10. Specification first: 16 13 is updated in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): the test runs on the build host or a GitHub-hosted runner; no cloud load generator.

Product requirements implemented here:

12. The run is reproducible: fixed seed, fixed duration and rates as named values, the stack installed from the trial release with the documented options, the profile recorded (`perf.reference_profile` or the emulated mode of ADR 0063).
13. The gate value `perf.capacity_min_messages_per_s` (name recorded in the ruling) fails `make perf` when the measured sustained throughput falls below it; it is set from this ticket's measurement with a documented margin.
14. The report lists every row of 16 13 with metric, measured value, lever and the conditions; the disk space formula gets the measured means (message size, event payload, index share).

## Scope

- `tests/load/` (scenario, data generator, driver through the public API and the gateway, result collection from the metrics), with a README.
- `services/server/perf/` or `tests/load/`: the gate test wired into `make perf` (Design note 2).
- Configuration reference: the new named value(s) of requirement 13.
- Docs: `docs/spec/16-operations.md` 13 (the table filled, with the profile, commit and date as the source of the numbers), `docs/spec/02-architecture.md` 22 if a value is added there, `docs/quality/gates.md` (the load test and the gate), `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-18-<n>`.

## Out of scope

- Changing other `perf.*` targets or `perf.reference_profile` (M1-72 owns them; a needed change is a report item).
- Multi-node capacity beyond the target of M7-17.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-18-capacity-load-test`; one writer per worktree.
- Conventional Commits with the component as scope (for example `test(load): ...`, `docs(operations): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). The stack publishes only on `127.0.0.1`.
- Stop only processes and containers you started, by PID or container ID; the installation is removed with its volumes at the end. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Scenario** (16 13). Ramp people, connections and message rate in steps; at each step hold for a named duration and record throughput, `bothy.db.event_seq.lock_wait`, `postgresql.backends`, `bothy.gateway.connections.active`, `bothy.event.fanout.lag`, process memory and database size by table and index. Stop the ramp at the first step where a performance target of 02 17 is violated; that step and the one before are the result.

2. **Gate** (P-07, ADR 0063). The gate runs in `make perf`, not in `make check`; it reads the result of a short steady run at the documented rate and fails below the named minimum. Record the margin rule in the ruling.

3. **Rulings for `docs/DECISIONS.md`** (`D-M7-18-<n>`): scenario and profile (note 1), gate value and margin (note 2).

## Tests (write them first)

- Unit tests of the data generator (seeded, deterministic) and of the result parser.
- The gate test: a synthetic result below the minimum fails, one above passes.
- One full run on the build host with the report.

Commands that must pass, from the repository root:

- the unit tests of `tests/load` (in `make test` through the tool chain the build chooses)
- `make perf` including the capacity gate, on the build host with the profile recorded
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] A reproducible run produces a report with every row of 16 13.
- [ ] 16 13 holds measured numbers with profile, commit and date.
- [ ] The minimum throughput gate fails below the named value.
- [ ] `gates.md`, the configuration reference, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `tests/load/README.md` written in English.
- Rulings of Design note 3 recorded in `docs/DECISIONS.md` section 10.
- `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make perf`.
3. Review and red team verdicts and the findings fixed.
4. The capacity table as measured, with the profile and whether it matched `perf.reference_profile`.
5. What is not proven (larger hosts, production data shapes).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
