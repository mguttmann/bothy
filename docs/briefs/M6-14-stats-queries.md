# M6-14: stats: timeline, SLO traffic light and backend view

Status: ready | Depends on: M2-30, M6-05, M6-13 | Milestone: M6

## Goal

After this ticket three query operations read the own measurement with the permissions of 25 4: `getStatsTimeline` returns the minute rows of `stats_minutes` and the run intervals of a range of at most `stats.timeline_max_range` inside `retention.stats_minutes`, filtered to the caller's visible channels (rows without a channel only with `stats.read` on the instance, cost rows only with `cost.read.aggregate`), with at most `stats.timeline_max_runs` runs and `runs_truncated`; `getStatsSlo` returns one entry per SLO of the Sloth files (from `slibounds.gen.go`), computed from `stats_minutes` over the SLO period, with ratio, remaining error budget and the state `green`, `amber`, `red` or `insufficient_data` by `stats.slo_warn_budget_ratio` and `stats.slo_min_events`, labelled "own measurement", for owners only; `getStatsBackends` returns per agent of the visible channels the configured and reported billing mode, the supported modes, the switches and the I2 and I3 denials of the range, costs per mode only with `cost.read.aggregate`, and the rate-limit state that the adapter reports in its heartbeat, without any runtime name, subscription or credential (ruling T8, I5). The heartbeat stores the reported `rate_limit` in `agents.reported_details`, and the hand-written layers of the Python and TypeScript SDKs send it. VZ-T05 for REST and VZ-T09 are green. Why: the timeline, the executive traffic light and the backend view of M6-12 need these server queries, and every one of them is a place where statistics could leak channels, costs or people (T-VZ-02, T-VZ-03).

## Read first

- `docs/spec/25-visualization.md`: 1 (rules 1 to 3), 3.2 (metrics, which have a channel), 3.3 (bounds, period), 4 (permissions and rules 1 to 5), 6.2 (backend view, ruling T8), 6.4 (the traffic light table and the formula), 6.5 (timeline limits), 11 (errors), 12 (VZ-T05, VZ-T07, VZ-T09), 15 (`stats.timeline_max_range`, `stats.timeline_max_runs`, `stats.slo_warn_budget_ratio`, `stats.slo_min_events`).
- `docs/spec/09-agents.md` 4.8 (heartbeat, `agent_heartbeat.v1`, the field `rate_limit`: only state and reset time, never quotas, keys or subscription names), `docs/spec/10-backend-policy.md` 3 and 4 (configured and reported mode, I2, I3, I5).
- `docs/spec/03-data-model.md` (core contract, read only): `stats_minutes`, `agents` (`billing_mode`, `reported_billing_mode`, `reported_details` with `rate_limit`), `runs` (`queued_at`, `started_at`, `finished_at`, `state`, `channel_id`, `agent_id`).
- `docs/spec/05-actions.md` (core contract, read only): rows `stats.read`, `cost.read.aggregate`, `agent.heartbeat`.
- `docs/observability/slos.md` 1 and 2, `docs/observability/admin-statistics.md` 7.
- `docs/api/openapi.yaml`: operations `getStatsTimeline`, `getStatsSlo`, `getStatsBackends`, `sendHeartbeat`; schemas `StatsTimeline`, `StatsTimelineBucket`, `StatsTimelineRun`, `StatsSloReport`, `StatsSloEntry`, `StatsSloState`, `StatsBackendReport`, `StatsBackendAgent`, `StatsBackendCost`, `StatsBackendDenials`, `Heartbeat`, `AgentRateLimitState`.
- `docs/security/threat-model.md` 6.8 (T-VZ-01 to T-VZ-06).
- `docs/DECISIONS.md`: rulings T7, T8, D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-14-<n>` rows).
- Briefs (merged names win, D-W-100): `M6-13-stats-measure.md` (table, partitions, `measure` constants, good rule), `M6-11-viz-package.md` (Design note 6: `slibounds.gen.go`), `M6-05-stats-server.md` (authorization and channel set per query, the cache), `M2-20-heartbeat-presence.md` (heartbeat use case and `reported_details`), `M2-29-python-sdk.md` and `M2-30-typescript-sdk.md` (the hand-written layers and their tests), `M1-28-authz-core.md` (`Scope`).
- Code on the base branch: `services/server/internal/stats` (M6-05, M6-13), `internal/agents`, `internal/runs`, `sdk/python`, `sdk/typescript`.

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
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; SLO targets, thresholds and periods only from `slibounds.gen.go`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Every row and run of the timeline lies in a channel the caller can read; rows without a channel only for `stats.read` on the instance; `cost` rows and cost members only with `cost.read.aggregate` on the target (25 4 rules 1 and 2; filter mode, no error).
12. A range longer than `stats.timeline_max_range`, starting before `now - retention.stats_minutes`, empty or not on full minutes gives `400 validation_failed`; `space_id` unreadable `404`, readable without `manage` `403` (25 11).
13. The SLO traffic light needs `stats.read` on the instance; a space manager gets `403`; values only from `stats_minutes`; `measured_by = own` (25 6.4).
14. The backend view names no runtime of an instance, no subscription, no seat and no credential (ruling T8, I5); `rate_limit` carries only `state` and `resets_at`.
15. No principal ID of a person, no triggering person and no content in any response (25 1, VZ-T07).

## Scope

All paths under `services/server/` unless they start with `docs/` or `sdk/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create:

- `internal/stats/app/timeline.go`, `slo.go`, `backends.go`: the three use cases (Design notes 1 to 3).
- `internal/stats/domain/slo.go`: ratio, remaining budget `1 - (1 - ratio) / (1 - objective/100)` and the state decision (Design note 2).
- `internal/stats/adapters/postgres/`: sqlc queries over `stats_minutes` (range and metric filter, channel filter `channel_id = ANY($n)` plus the instance rows when allowed, sums over the SLO period per `sli.*` metric).
- Source ports in the owning modules where they do not exist yet: `runs.TimelineSource` (run intervals in a range for a channel set, ordered and limited, without trigger person) and `agents.BackendSource` (configured and reported mode, supported modes and the stored `rate_limit` for a set of agent IDs).
- `internal/stats/adapters/httpapi/`: the three handlers.
- Tests named under "Tests".

Change:

- `internal/agents`: the heartbeat use case stores `rate_limit` (validated against `AgentRateLimitState`: closed `state`, optional `resets_at` not further in the future than you can justify; reject anything else) in `agents.reported_details.rate_limit`; an absent field clears a stored one (the heartbeat is a full state report, 09 4.8). The stored value is never audited unless the mode or tool class changes (catalogue row `agent.heartbeat`).
- `api/operations.yaml`: enable `getStatsTimeline`, `getStatsSlo`, `getStatsBackends`; regenerate.
- `docs/api/openapi.yaml`: descriptions only where the build sharpens them (for example the order of runs and the meaning of `runs_truncated`); `make api-lint`, `make api-checks`, `make codegen-check`.
- `internal/wire`: wiring; configuration keys `stats.timeline_max_range`, `stats.timeline_max_runs`, `stats.slo_warn_budget_ratio`, `stats.slo_min_events` enter the schema with this ticket as their first user (`make config-reference`, `make config-doc-check`).
- `sdk/python` and `sdk/typescript` hand-written layers: the heartbeat helper accepts and sends `rate_limit` (`state`, optional `resets_at`), with a test each; their READMEs and CHANGELOGs.
- Docs: `docs/spec/25-visualization.md` 6.2, 6.4, 6.5 only where the build sharpens a rule (spec first), `internal/stats/README.md`, `internal/agents/README.md`, `docs/observability/admin-statistics.md` 7 item 2 (the three queries are built); `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M6-14-1` to `D-M6-14-6`.

This ticket adds no table, no migration, no action, no event and no top-level Makefile target.

## Out of scope

- The live frame, the subscription and `getStatsLive` (M6-15).
- The views (M6-12) and their hosts (M6-16).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-14-stats-queries`; one writer per worktree.
- Conventional Commits with the module or component as scope (for example `feat(stats): ...`, `feat(agents): ...`, `feat(sdk-python): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; never publish an SDK package; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. Logs of this ticket carry no person, channel or agent ID. No network calls except package downloads for pinned tools, Go modules and SDK dependencies.
- Time only through `kernel/clock`; tests use a fake clock and write `stats_minutes` rows through the M6-13 flush or as fixture rows through `pgtest`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Timeline** (25 6.5; Ruling for docs/DECISIONS.md). Authorization and channel set exactly as M6-05 Design note 1 (reuse that function). Rows: `stats_minutes` in `[from, to)` for the requested metrics (absent `metric` means all the caller may see), channel rows in the set, instance rows (`channel_id IS NULL`) only for instance scope with `stats.read` on the instance, `cost` rows only with `cost.read.aggregate`; agent rows of channels outside the set never appear because every agent metric with a channel carries the channel. Runs: `runs.TimelineSource` returns runs whose interval intersects the range, in the channel set, ordered by `queued_at`, at most `stats.timeline_max_runs` plus one to detect truncation; no trigger person, no title. Validation: `from` and `to` on full minutes (the operation says so), `to - from <= stats.timeline_max_range`, `from >= now - retention.stats_minutes` (25 11), `from < to`.

2. **SLO traffic light** (25 6.4; VZ-T09; Ruling for docs/DECISIONS.md). For each entry of `slibounds.gen.go`: sum `count` and `good` of its metric over `[now - period, now)` across all rows (instance values without a channel; SLIs have no channel except `sli.run_start` and `sli.turn_duration`, which sum over all channels because the owner sees the instance); `ratio = good / total`; `budget_remaining` by the formula; state: `insufficient_data` when `total < stats.slo_min_events`, else `red` when `budget_remaining <= 0`, else `amber` when `budget_remaining < stats.slo_warn_budget_ratio`, else `green`. `threshold_ms` from the bound when present; `objective` and `period_days` from the bound. The response is cached for `stats.cache_ttl` (the cache of M6-05) because it scans the period.

3. **Backend view** (25 6.2; ruling T8; Ruling for docs/DECISIONS.md). Agents: those with a run in a visible channel in the range or an operator role on a visible channel (the agents port lists them for the set). Per agent: `billing_mode` and `reported_billing_mode` from `agents`, `billing_modes_supported` from the stored heartbeat, `switches` and `invocations_denied` (`i2_denied`, `billing_mode_mismatch`) summed from `stats_minutes` in the range (switches have no channel: count them only for agents in the result), costs per mode summed from `stats_minutes` `cost` rows in the visible channels only with `cost.read.aggregate`, `rate_limit` from `reported_details`. Never `adapter_kind`, never `runtime_version`, never `billing_evidence` (the schema has no member for them; a test asserts that the encoded response has only the schema members).

4. **Heartbeat `rate_limit`** (09 4.8). Validation in the agents domain: `state` from the closed list, `resets_at` optional; the stored JSON is exactly `{state, resets_at?}`; any extra member in the request is already rejected by the closed request schema (M1-18). An adapter that never sends it leaves `rate_limit` absent in the backend view, which the view shows as "unknown" (M6-12).

5. **SDKs.** Python and TypeScript heartbeat helpers gain an optional `rate_limit` argument with the two members, typed from the generated models of M2-23, and a unit test against the SDKs' fake server of M2-29 and M2-30 that the request body carries it and nothing else new.

6. **Rulings for `docs/DECISIONS.md`** (section 10): `D-M6-14-1` timeline rows, runs and validation (note 1); `D-M6-14-2` the traffic light computation and its caching (note 2); `D-M6-14-3` which agents appear in the backend view (note 3); `D-M6-14-4` members never sent by the backend view (note 3); `D-M6-14-5` storing and clearing `rate_limit` (note 4); `D-M6-14-6` the SDK helpers (note 5).

## Tests (write them first)

Test world: 07 10 and 11.1 (Sam with `manage` on Support, which gives him `cost.read.aggregate` there by 05; Robin with `read` only; the Owner Alex); `stats_minutes` rows for both spaces, instance SLI rows and cost rows written as fixture rows through `pgtest` over several hours and one full SLO period; runs created through the runs use cases.

Unit:

- `internal/stats/domain`: `TestSloState` (VZ-T09): generated minutes with remaining budget above and below `stats.slo_warn_budget_ratio`, exactly 0, negative, and fewer than `stats.slo_min_events` events give `green`, `amber`, `red`, `red`, `insufficient_data`; the formula against hand-computed values.
- `internal/agents`: `TestHeartbeatRateLimit` (stored, cleared when absent, invalid state rejected).

Integration (`pgtest`, full chain):

- `TestTimeline_SpaceManager` (VZ-T05 for REST): Sam with `space_id = Support` gets only rows and runs of Support channels, no instance rows, and `cost` rows of Support only; without `space_id` `403`. The path without `cost.read.aggregate` (no `cost` rows, no cost members) is proven in a unit test of the use case with a stubbed decision, because 05 gives the action to every manager.
- `TestTimeline_Owner`: Alex gets channel rows, instance rows and costs; runs over `stats.timeline_max_runs` give `runs_truncated = true` and exactly the limit.
- `TestTimeline_Range` (25 11): over `stats.timeline_max_range`, before `retention.stats_minutes`, not on a full minute, empty: `400`.
- `TestSlo_OwnerOnly`: Alex gets six entries with `measured_by = own`; Sam gets `403`.
- `TestBackends`: Sam sees support-agent, not dev-runner (only in Engineering); costs of Support present (the path without `cost.read.aggregate` in the unit test above); `rate_limit` from the last heartbeat; the encoded response contains no `adapter_kind`, no runtime version and no evidence; switches and denials match the rows.
- `TestNoPeopleNoContent` (VZ-T07 part): canary message text, thread title and step title in the world never appear in any of the three responses; no principal ID of a person appears.
- SDK tests: `cd sdk/python && ...` and `cd sdk/typescript && ...` as their READMEs prescribe.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/stats/... ./internal/agents/... ./internal/runs/... ./internal/wire/...`
- the SDK test commands of `sdk/python/README.md` and `sdk/typescript/README.md`
- `make codegen-check`, `make api-lint`, `make api-checks`, `make arch`, `make register-check`, `make config-doc-check`
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] VZ-T05 for REST and VZ-T09 are green.
- [ ] A range over `stats.timeline_max_range` or outside `retention.stats_minutes` gives `400`; cost fields are missing without `cost.read.aggregate`.
- [ ] The traffic light is owner only, computed from `stats_minutes` with the bounds of the Sloth files, labelled own measurement.
- [ ] The backend view names no runtime of an instance, no subscription and no credential; `rate_limit` comes from the heartbeat, which the SDK helpers send.
- [ ] The READMEs, the CHANGELOGs and the rows `D-M6-14-1` to `D-M6-14-6` are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `stats`, `agents`, `sdk/python` and `sdk/typescript` updated in English.
- Rulings of Design note 6 recorded in `docs/DECISIONS.md` section 10.
- `services/server/CHANGELOG.md` and the SDK CHANGELOGs under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
