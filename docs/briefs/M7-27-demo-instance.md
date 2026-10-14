# M7-27: Demo instance with made-up data (`bothy setup --demo`)

Status: ready | Depends on: M2-48, M6-14, M6-15, M7-14 | Milestone: M7

## Goal

After this ticket one command turns an empty instance into a demo that shows everything visible with made-up, white-label data (P-22, P-26): the people Alex, Sam and Robin, the spaces Support, Marketing, Engineering and Research with channels and threads, agents with ongoing runs driven by the test double of the Python SDK (`sdk/python/testing`), costs, a statistics history for the views of 25, and a host with the host service in a container so a terminal opens without any real model call. The command is `bothy setup demo` (a subcommand next to `bootstrap-owner`, 02 3.3; the plan writes it `bothy setup --demo`, Design note 1) with the Compose overlay `infra/compose/demo.yaml` (demo agent, host service container) and the data set in `infra/demo/`. Specification first: a new section "Demo instance" in 16. Every record comes into being through public operations of the API with `authorize` and audit, never through SQL; a run on an empty instance succeeds and is repeatable with the same seed (same content, same structure); on an instance with data it aborts without writing; `make denylist` and `make gitleaks` over the data set find nothing. Why: the showcase (M7-28) is produced from this instance, and a demo that bypassed the permission model would show behaviour the product does not have.

## Read first

- `docs/PLAN.md` decisions P-22 and P-26; `docs/spec/16-operations.md` (where the new section "Demo instance" goes, 3.1, 4.4 for the host service), `docs/spec/02-architecture.md` 3.3 (command register, exit codes), `docs/spec/08-identity.md` 4 (setup, bootstrap owner, enrollment), `docs/spec/01-product.md` (product description, screens the demo must fill).
- `docs/spec/25-visualization.md` 6 (all six views and what data they need), 3 (own measurement: what produces history), `docs/spec/24-terminal.md` (host pairing and a session), `docs/spec/09-agents.md` (runs, live box, approvals).
- `docs/operations/environments-and-flags.md` (`bothy dev seed` only in `dev`; the demo is a different thing, Design note 1), `docs/quality/gates.md` (`denylist`, `gitleaks`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-27-<n>` rows).
- Briefs (merged names win, D-W-100): `M2-29-python-sdk.md` (`sdk/python/testing`, the test double), `M2-48-terminal-conformance.md` (host service in a Linux container, test overlay), `M6-15-stats-live.md` and `M6-13-stats-measure.md` (live frame and `stats_minutes`), `M1-37-identity-bootstrap.md` (`setup bootstrap-owner`), `M7-14-portability-test.md` (installation from the trial release, the stack the demo runs on).
- Code on the base branch: `services/server/cmd/bothy` (`setup`), the use cases of `chat`, `messaging`, `agents`, `runs`, `billing`, `terminal`, `identity`; `infra/compose/`; `sdk/python/testing`.

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
8. White label: only the people Alex, Sam and Robin, the four spaces, made-up agent names, `example.com` addresses and made-up content; no company, product of a third party, real person, host or place.
9. Sizes of the data set (counts, days of history) are named values of the demo or fields of the data set, never literals in code.
10. Specification first: the section "Demo instance" in 16 and the row of the command in 02 3.3 come first. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): no real model call, no external service; agents are the SDK test double.
12. Never tag, never publish, never deploy a public demo (D-W-125).

Product requirements implemented here:

13. Only public operations of the API with `authorize` and audit create data, through the sessions of Design note 5; a test fails if the demo package imports a SQL adapter, a repository or a use case of a module directly.
14. Empty instance: success; same seed twice on two empty instances: the same content and structure (IDs and timestamps may differ; the comparison ignores them); an instance with data (any non-setup row): abort with a non-zero exit code before the first write.
15. The demo shows every view of 25 (live topology with ongoing runs, backend view, delivery and sync live, executive page with the traffic light, the replayable timeline with history, TV mode) and a terminal session on the demo host.
16. `make denylist` and `make gitleaks` over `infra/demo/` pass; the data set has no secret, no real name and no real address.

## Scope

- `services/server/cmd/bothy`: `setup demo [--seed N] [--data <dir>]` (exit codes per 02 3.3; record the code for "instance not empty").
- A small client package in `cmd/bothy` (or `internal/demo`, recorded) that drives the public API of the running instance (Design note 5); no module code changes.
- `infra/demo/`: the data set (people, spaces, channels, threads, messages, agents, prices, the host) as YAML or JSON with a schema and a README.
- `infra/compose/demo.yaml`: the demo agent from `sdk/python/testing` (scripted runs with tool steps, approvals, long-running runs for the live views) and the host service container (from the overlay of M2-48, outside the installation bundle rules: the file lives under `infra/compose/` but is excluded from the bundle like the test overlays; record the rule).
- Docs: `docs/spec/16-operations.md` new section "Demo instance", `docs/spec/02-architecture.md` 3.3 row, `infra/demo/README.md`, `docs/operations/` guide "Run the demo", `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-27-<n>`.

## Out of scope

- Images, video and landing pages (M7-28); a hosted public demo.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-27-demo-instance`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(bothy): ...`, `feat(compose): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). The demo stack publishes only on `127.0.0.1`.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; the enrollment code of the demo owner appears only on the terminal.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Command and environment** (02 3.3; Ruling for docs/DECISIONS.md). `bothy dev seed` exists only for `instance.environment = dev`; the demo is a product feature for any environment, so it is `setup demo` and refuses a non-empty instance instead. The plan's spelling `bothy setup --demo` maps to this subcommand; record it.

2. **History** (25 3). The live views fill from the running demo agent. For the timeline and the executive page the demo needs past minutes: the public API cannot backdate (Design note 5), so the demo agent runs its scripted runs for a warm-up period (a named value of the demo) before the views are read; if a view cannot show history honestly, it shows what the running demo produces and the report says so.

3. **Determinism** (requirement 14). The seed drives every choice (texts from the data set, order, run scripts); the comparison test exports both instances through the public API and compares them without IDs and timestamps.

4. **Rulings for `docs/DECISIONS.md`** (`D-M7-27-<n>`): command (note 1), history (note 2), determinism (note 3), sessions (note 5), the bundle rule for `demo.yaml`.

5. **Sessions** (07 SR5, 08 4 and 12). Owner actions such as `user.create`, `agent.create`, `price.manage`, `app.install` and `grant.create` need an owner with step-up, which only a person's session has; a server command has neither, and no system action covers the demo. So `setup demo` is a client of the public API of the running instance: it creates the owner with the system action `instance.bootstrap` of `setup bootstrap-owner` (M1-37), enrolls a demo device with the enrollment code it receives (never printed), keeps the device key in memory, logs in and steps up with the device key (`createStepUpChallenge`, `completeStepUpWithDeviceKey`, M1-35) before each owner action, creates Alex, Sam and Robin as local accounts (M1-39) and writes their messages in their own sessions. No new action and no change to the core contract.

## Tests (write them first)

- Go: `TestDemoRefusesNonEmpty` (abort before the first write), `TestDemoOnlyUseCases` (architecture test: no SQL or foreign repository import), `TestDemoDeterministic` (two empty instances, same seed, equal export).
- Integration with the demo overlay: after `setup demo`, every view of 25 has data through its query (`getStatsLive`, `getStatsTimeline`, `getStatsSlo`, `getStatsBackends`), at least one run is `running` with a live box, an approval card waits, and a terminal session opens on the demo host.
- Data set: schema validation; `make denylist` and `make gitleaks` pass over `infra/demo/`.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./cmd/... ./internal/wire/...`
- the demo integration test (the target the build adds, outside `check` if it needs the stack; record it in `docs/quality/gates.md`)
- `make denylist`, `make gitleaks`, `make arch`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] A run on an empty instance succeeds and is repeatable with the same seed; on an instance with data it aborts without writing.
- [ ] All data is created through public operations of the API with `authorize` and audit, never through SQL.
- [ ] `make denylist` and `make gitleaks` over the data set have no hit.
- [ ] The demo shows every view of 25 and a terminal without a real model call.
- [ ] 16, 02, the guide, READMEs, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `infra/demo/README.md` written in English.
- Rulings of Design note 4 recorded in `docs/DECISIONS.md` section 10.
- `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of the demo integration test.
3. Review and red team verdicts and the findings fixed.
4. Which views show history and which only live data.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
