# M7-09: Status page with automatic checks

Status: ready | Depends on: M6-13, M7-02, M7-05 | Milestone: M7

## Goal

After this ticket the server runs its own status page (R-OP-15): the components `api` (messages and sync), `push`, `files` (files and scan), `search` and one component per agent exist from setup on; the job `status.probe` in the `worker` checks the `health_source` of every component every `status.check_interval` and sets the state automatically: a check fails without an answer or when an error ratio exceeds `status.error_ratio_degraded`; failing longer than `status.degrade_after` gives `degraded`, an error ratio above `status.error_ratio_outage` or no answer gives `major_outage` (16 12). A `major_outage` longer than `status.incident_after` opens an incident automatically (`created_by` NULL), which is resolved after `status.resolve_after` of health; owner texts (`status.manage`) stay untouched by the automation. Agent components follow `agents.reported_runtime_state`. People with `status.read` read it (`getStatus`), only agents with `agent.read` appear, and without login it is readable only with `status.public_enabled` (`getPublicStatus`). Every change appends `status.updated`. The job `retention.ops` removes old rows of the status tables and of `backup_runs` per `retention.ops`. Why: users need to know whether a problem is theirs or the instance's, without the optional observability stack; the checks come from inside the server, not from Prometheus.

## Read first

- `docs/spec/16-operations.md`: 12 (both tables, the limit "Server unreachable"), 14 (`status.*`), 15 (R-OP-15).
- `docs/spec/03-data-model.md` (core contract, read only): `status_components`, `status_incidents`, `status_incident_updates`, `status_incident_components`, `backup_runs` (retention `ops`), `agents.reported_runtime_state`, `stats_minutes`.
- `docs/spec/05-actions.md` and `actions.yaml` (core contract, read only): `status.read`, `status.manage`, `status.public.read`, event `status.updated`.
- `docs/spec/06-api.md` and `docs/api/openapi.yaml`: `getStatus`, `getPublicStatus`, `createStatusComponent`, `updateStatusComponent`, `deleteStatusComponent`, `createStatusIncident`, `addStatusIncidentUpdate`, `onStatusUpdatedEvent`.
- `docs/spec/25-visualization.md` 3.2 (the `sli.*` metrics that the checks read), `docs/spec/20-privacy.md` 4.1 (class `ops`), `docs/spec/02-architecture.md` 12.3 (`status.probe`, `retention.<class>`).
- `docs/operations/configuration.md` (rows `status.*`, `retention.ops`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-09-<n>` rows).
- Briefs (merged names win, D-W-100): `M6-13-stats-measure.md` (`stats_minutes`), `M2-20-heartbeat-presence.md`, `M3-20-apns-adapter.md` and `M3-21-fcm-adapter.md` (provider results), `M3-01-object-storage-fs.md` (storage port), `M3-04-clamd-scan.md` (`PING`), `M3-14-search-coordinator.md`, `M7-02-backup-service.md` (`backup_runs`), `M7-05-retention-per-class.md` (participant port, coverage test exceptions), `M1-37-identity-bootstrap.md` (setup).
- Code on the base branch: `services/server/internal/status`, `internal/stats`, `internal/agents`, `internal/files`, `internal/search`, `internal/notify`, `internal/wire`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Intervals, ratios and periods only through the named keys `status.*` and `retention.ops`; code comments and docs name the key, never the value.
10. Specification first: a change to behaviour, API or configuration updates `docs/spec/16-operations.md` or `docs/api/` in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125).

Product requirements implemented here:

12. The checks read sources inside the instance only: `api.http` and `sync.fanout` and `push.apns`, `push.fcm` from the own measurement in `stats_minutes` (`sli.api_availability`, `sli.delivery_latency`, `sli.push_acceptance` per provider, invalid tokens already excluded there), `files.storage` with a probe object (write, read, delete), `files.scan` with `PING` to clamd, `search.query` with a test query, `agent.presence` from the heartbeat state.
13. The state machine of 16 12 with the named thresholds; automatic incidents are created and resolved only by the job and never touch incidents or texts of the owner.
14. Visibility: `status.read` for logged-in principals, agent components only with `agent.read` on that agent, `getPublicStatus` only with `status.public_enabled`, otherwise `404`; backup and restore test are never components (16 12).
15. Every change of a component state or incident appends `status.updated`; a check round without a change appends nothing.
16. `retention.ops` removes rows older than `retention.ops` from the status tables and `backup_runs` through the participant port of M7-05; the latest successful `backup_runs` row per kind is always kept (the backup requirement of `migrate` reads it).

## Scope

Server module (paths under `services/server/` unless they start with `docs/`; every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`):

- Module `status`: migrations of the four status tables (03 4.12); `domain` (state machine, thresholds, incident automation), `app` (the seven operations, `status.probe`, read model with visibility, `retention.ops` participant), `adapters` (PostgreSQL, the check sources through the ports of `stats`, `files`, `search`, `agents`), handlers.
- Setup: the four standard components and one per agent created idempotently (Design note 2); a disabled agent's component follows `agents.AgentDisabled`.
- `internal/wire`: `status.probe` on queue `maintenance` in `worker` and `all`; `retention.ops` registered with the participant port.
- Configuration: the keys `status.*` and `retention.ops` become active.
- Docs: `docs/spec/16-operations.md` 12 (as built: the measurement window of each check), the README of `status`, `services/server/CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-09-<n>`; M7-05's coverage test drops the exception for the status tables and `backup_runs`.

## Out of scope

- The views of the status page in apps and console (M7-21); the maintenance display (M7-08 adds it to this read model).
- Alerts to the outside when the whole server is down (M7-20, watchdog).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-09-status-page`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(status): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). Tests bind only `127.0.0.1` and port 0; PostgreSQL, clamd and SeaweedFS only as throwaway containers on random ports (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Time only through `kernel/clock`; tests move a fake clock and call `status.probe` directly; outages are simulated with fakes of the sources.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Measurement window** (16 12; Ruling for docs/DECISIONS.md). `stats_minutes` holds minute rows; a check reads the minutes that end within the last `status.check_interval`, at least the last complete minute. Error ratio of `api.http`: `1 - good/count` of `sli.api_availability`; `push.*`: `1 - good/count` of `sli.push_acceptance` per provider; `sync.fanout`: mean delivery latency `sum/count` of `sli.delivery_latency` against `status.fanout_lag_degraded`. No traffic in the window means no evidence, not a failure.

2. **Standard components** (16 12 "created during setup"; Ruling for docs/DECISIONS.md). `status.probe` creates every standard component whose `health_source` no component carries (`ON CONFLICT (lower(name)) DO NOTHING`), and one component per active agent without one; an owner may rename, describe and reorder them; a deleted standard component comes back at the next round, because the four sources are the minimum of 16 12 (the runbook and the README say so).

3. **Automatic incidents** (16 12). One open automatic incident per component; its updates name the state change without content; resolution after `status.resolve_after` of continuous health.

4. **Rulings for `docs/DECISIONS.md`** (`D-M7-09-<n>`): measurement window (note 1), standard components (note 2), automatic incidents (note 3), the kept latest `backup_runs` rows.

## Tests (write them first)

Unit:

- `status/domain`: `TestStateMachine` (healthy, failing below `status.degrade_after`, `degraded`, outage ratio, no answer gives `major_outage`), `TestIncidentAutomation` (open after `status.incident_after`, resolve after `status.resolve_after`, owner texts untouched).

Integration (`pgtest`, real modules through `internal/wire`, fakes for the sources):

- `TestProbe_SimulatedOutages`: each `health_source` failing and recovering gives the expected states, one automatic incident and its resolution.
- `TestVisibility`: a person without `agent.read` sees no agent component; `getPublicStatus` gives `404` without `status.public_enabled` and the page with it; owners manage components and incidents with `status.manage`.
- `TestStatusUpdatedOnlyOnChange`.
- `TestRetentionOps` with a fake clock, keeping the latest successful `backup_runs` row per kind.
- Handler tests for the seven operations.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/status/... ./internal/stats/... ./internal/wire/... ./cmd/...`
- `make migrations-lint`, `make migrations-lock`, `make codegen-check`, `make arch`, `make register-check`, `make config-doc-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `degraded` after `status.degrade_after`, `major_outage`, automatic incident and resolution with simulated outages; owner texts remain.
- [ ] Visibility rules of 16 12 hold; `status.updated` on every change.
- [ ] `retention.ops` keeps the classes of 20 for the status tables and `backup_runs`.
- [ ] 16, the README, the configuration reference, the CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `status` updated in English.
- Rulings of Design note 4 recorded in `docs/DECISIONS.md` section 10.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The measurement window per check as built.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
