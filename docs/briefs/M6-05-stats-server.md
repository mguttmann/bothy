# M6-05: Statistics in the server

Status: ready | Depends on: M2-22, M2-26, M3-22 | Milestone: M6

## Goal

After this ticket the five statistics operations of `openapi.yaml` answer from the server's own domain tables, never from the telemetry stack (admin-statistics.md 1, R-OB-08): `getUsageStats` (messages by origin, new threads, active people, runs per space), `getRunStats` (runs by end state and duration percentiles per agent), `getToolCallStats` (tool calls by tool name, failures, median duration), `getErrorStats` (run failure rate, invocation denials, webhook failure rate, push failure rate) and `getCostStats` (valid cost sums per billing mode with the estimate flag, by space, agent or none). Every query is authorized with `stats.read` on the instance or a space, except `getCostStats`, which is authorized with `cost.read.aggregate` alone as `openapi.yaml` declares it, and filtered to the channels `authz` returns for the caller, so a space manager sees no number of a channel they cannot read; no query groups by person, and the count of active people is shown as "fewer than `privacy.stats_min_group_size`" below the minimum group (admin-statistics.md 3, DP-T11). Results are cached per caller scope for `stats.cache_ttl`, ranges are bounded by `stats.max_range`, and day and week buckets follow `stats.time_zone`. Why: the console (M6-06) and the desktop app show these numbers for the owner and the space managers, and "statistics as a surveillance tool" is a red team item of M6 (PLAN 12, T-OW-01); the numbers must follow exactly the permissions of the data they count.

## Read first

- `docs/observability/admin-statistics.md`: all of it (sections 1 to 7; the key figure table of section 2 is the list of sources, section 3 the privacy rules, section 4 the permissions, section 5 performance, cache, range and time zone).
- `docs/spec/17-observability.md`: 8 "Statistics in the admin console" and rule 7; 12 (`stats.cache_ttl`, `stats.max_range`, `stats.time_zone`).
- `docs/spec/20-privacy.md`: 9 (codetermination, row "Statistics per person"), the tests DP-T11 and DP-T12, the keys `privacy.per_person_stats_enabled`, `privacy.stats_min_group_size`.
- `docs/spec/25-visualization.md`: 1 rules 1 to 3 (no values per person, no content), 4 (permissions table and rules 1 to 5; rows without a channel only for `stats.read` on the instance; costs only with `cost.read.aggregate`), 6.4 (the executive page reads these operations).
- `docs/spec/05-actions.md` (core contract, read only): rows `stats.read` and `cost.read.aggregate`; 2.4 (filter mode).
- `docs/spec/07-permissions.md`: 2.6 (`manage`), 4.2, 7.4 (query pattern with `Scope`).
- `docs/spec/03-data-model.md` (core contract, read only): `messages` (`origin`, `author_principal_id`, `deleted_at`), `threads` (`created_at`, `done_at`), `runs` (`state`, `state_reason`, `started_at`, `finished_at`, `agent_id`, `channel_id`), `run_steps` (`kind`, `name`, `state`, durations), `cost_entries` (valid rows without a successor, `billing_mode`, `is_estimate`, `currency`, `space_id`), `messages.trigger_state_reason`, `webhook_deliveries` (`state`), `push_registrations` (`disabled_reason`, `provider`), `files` (`size_bytes`, `scope_channel_id`, `scan_status`).
- `docs/spec/10-backend-policy.md`: 12 (costs, estimate and information only in mode `personal`).
- `docs/spec/02-architecture.md`: 4.3 (row `stats`), 4.4 (tiers), 4.5, the module table and its "depends on" column.
- `docs/api/openapi.yaml`: operations `getUsageStats`, `getRunStats`, `getToolCallStats`, `getErrorStats`, `getCostStats`; schemas `StatsSeries`, `StatsBucket`, `StatsGranularity`, `CostSeries`, `CostBucket`.
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-05-<n>` rows); T-OW-01 in `docs/security/threat-model.md`.
- Briefs (merged names win, D-W-100): `M1-28-authz-core.md` (`Authorizer.Scope`, `Scope.ChannelIDs`, `InstanceWide`, filter mode), `M1-70-leak-tests.md` (SQL visibility lint and its exception categories), `M2-22-costs.md` (cost booking, valid row without a successor), `M2-26-webhooks-outgoing.md` (`webhook_deliveries`), `M3-22-push-registration.md` (`disabled_reason`), `M2-09-run-lifecycle.md` (run end states and reasons), `M3-03-upload-sessions.md` and `M3-04-clamd-scan.md` (files and scan state).
- Code on the base branch (all under `services/server/`): `internal/stats` (skeleton), `internal/messaging`, `internal/runs`, `internal/billing`, `internal/chat`, `internal/files`, `internal/apps`, `internal/notify`, `internal/authz`, `architecture/modules.yaml`, `api/operations.yaml`.

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
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket. Everything this ticket adds to the repository is English; existing text stays as it is.

Product requirements implemented here:

11. Statistics read only the server's own tables through the ports of their owning modules; no query reads the telemetry backend (admin-statistics.md 1, 6).
12. Permissions per scope (admin-statistics.md 4, 25 4): instance with `stats.read` on `instance` (owner), one space with `stats.read` on that space (`manage`); `getCostStats` with `cost.read.aggregate` on the instance or the space instead of `stats.read` (its `x-authz-action`). Without `space_id` and without owner status: `403 forbidden` (the operation description says "required without owner role"). An unreadable `space_id` gives `404` (25 11).
13. Every number counts only channels in the caller's visible set (`Scope`), intersected with the requested space; values without a channel (webhook deliveries of apps, push registrations, scans without a channel) are returned only to `stats.read` on the instance.
14. No grouping by person, no person ID, no name and no content in any response; `active_people` below `privacy.stats_min_group_size` is withheld and marked as "fewer than" (Design note 4); `privacy.per_person_stats_enabled` changes nothing here (25 rule 2).
15. Costs in mode `personal` are always marked "information only" by `billing_mode = personal` and never summed with `organization`; estimates keep `is_estimate = true` (admin-statistics.md 2, 10 12).
16. A range longer than `stats.max_range`, a range with more than `api.stats_max_buckets` buckets of the requested granularity (06 configuration table), an empty range or `from >= to` is `400 validation_failed`; results are valid for `stats.cache_ttl`; day, week and month buckets start at midnight in `stats.time_zone`.

## Scope

All paths under `services/server/` unless they start with `docs/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create:

- `internal/stats/domain/`: bucket arithmetic (granularity, time zone, range checks), the minimum group rule, percentile helpers over sorted samples (for `duration_ms_p50` and `p95` when a source returns samples), the merge of per-channel aggregates into buckets and groups (Design notes 2 to 5).
- `internal/stats/app/queries.go`: one use case per operation (`UsageStats`, `RunStats`, `ToolCallStats`, `ErrorStats`, `CostStats`), each: authorize, build the channel set, call the source ports, merge, cache (Design notes 1, 2, 6).
- `internal/stats/app/cache.go`: the result cache keyed by operation, parameters, caller scope fingerprint (`PermissionVersion` and the channel set) and expiring after `stats.cache_ttl` (Design note 6).
- `internal/stats/adapters/httpapi/`: the five handlers.
- Source ports in the owning modules (root packages, implemented in their `app` with sqlc queries in `adapters/postgres`), each taking `(from, to, granularity, tz, channelIDs []ids.ID)` and returning aggregates per bucket and group key, never rows: `messaging.StatsSource` (messages by origin, threads created and done, distinct human authors per space and requested bucket (week or month) as a count, computed inside `messaging` for exactly that bucket, because a monthly distinct count cannot be built from weekly counts, invocation denials from `trigger_state_reason`), `runs.StatsSource` (runs by end state and agent, durations, tool steps by name with failures and durations, `billing_mode_mismatch` failures), `billing.StatsSource` (valid cost sums by mode, currency, estimate flag, space or agent), `files.StatsSource` (storage per space, scans `error` and `infected` per day), `apps.StatsSource` (webhook deliveries `failed` and `dead` per app and day), `notify.StatsSource` (push registrations disabled per provider). Design note 2 lists the exact metrics.
- Tests named under "Tests".

Change:

- `architecture/modules.yaml`: `stats.depends_on` gains `files`, `apps`, `notify` (for their `StatsSource` ports); `make register-check`, `make arch`; `docs/spec/02-architecture.md` module table row `stats` and its reason in the same change.
- `api/operations.yaml`: enable the five operations; regenerate.
- `docs/api/openapi.yaml` (Design notes 2 and 4): `getCostStats` gains the optional query parameter `currency` and the error code `currency_required`; `StatsBucket` gains the optional member `below_min_group` (array of metric names withheld by the minimum group) and descriptions; the descriptions of the five operations name the grouping keys, the permission filter and the cache; `make api-lint`, `make api-checks`, `make codegen-check`.
- `internal/wire`: wiring of the source ports and the use cases in roles `api` and `all`; configuration keys `stats.cache_ttl`, `stats.max_range`, `stats.time_zone`, `api.stats_max_buckets`, `stats.cache_max_entries` (new, 17 11 first), `privacy.stats_min_group_size`, `privacy.per_person_stats_enabled` enter the schema with this ticket as their first user (status `planned (M6)` becomes active; `make config-reference`, `make config-doc-check`).
- Docs (all new text in English, existing text stays as it is): `docs/spec/17-observability.md` 11 (`stats.cache_max_entries`), `docs/observability/admin-statistics.md` (section 2 marks which source port serves each row; section 5 the cache key), `internal/stats/README.md` and the READMEs of the modules that gain a `StatsSource`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M6-05-1` to `D-M6-05-6`.

This ticket adds no table, no migration, no action, no event and no top-level Makefile target. It does not write `stats_minutes` (M6-13).

## Out of scope

- The own measurement, `stats_minutes`, the live frame, timeline, traffic light and backend view (M6-13 to M6-15).
- The daily aggregate table `stats_daily` of admin-statistics.md 5 item 3: it is created only when a load test requires it, as an additive migration with a core contract change first; this ticket measures (Tests) and reports.
- CSV export of statistics (admin-statistics.md 3 item 4): a later ticket with its own operation; not in `openapi.yaml` today.
- The views (M6-06).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-05-stats-server`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(stats): ...`, `feat(runs): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. Logs of this ticket carry no person, channel or space ID. No network calls except package downloads for pinned tools and Go modules.
- Time only through `kernel/clock`; tests use a fake clock.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Authorization and channel set** (25 4; 07 7.4; Ruling for docs/DECISIONS.md). Each use case first calls `authorize(p, stats.read, instance)` when `space_id` is absent, else `authorize(p, stats.read, space)`; `getCostStats` uses `cost.read.aggregate` on the same target instead. A space that the caller cannot read gives `404` (the decision of `authorize`); readable without `manage` gives `403`. Then `Scope(ctx, p, message.read)` gives the visible channels; with `space_id` keep only the channels of that space (`chat` port for the channel list of a space). The channel set is passed to every source port; a source never widens it. Rows without a channel are fetched only when the target is the instance and the decision was an owner decision (`Scope.InstanceWide`).

2. **Metrics and sources** (admin-statistics.md 2; the operation descriptions in `openapi.yaml` are the list of names):
    - `getUsageStats`, group key space ID: `messages` (count of messages not deleted, by origin as separate metrics `messages_human`, `messages_agent`, `messages_app`, `messages_import` and their sum `messages`), `threads_created`, `threads_done`, `active_people` (distinct authors of kind human per space and bucket, only for granularity `week` or `month`; for `hour` and `day` the metric is absent because a day of one space names almost a person), `runs`, `storage_bytes` (current sum of `files.size_bytes` per space, in the last bucket only). Add the names that are not yet in the operation description to it (spec first).
    - `getRunStats`, group key agent ID: `runs`, `completed`, `failed`, `cancelled`, `duration_ms_p50`, `duration_ms_p95` (from `started_at` to `finished_at` of finished runs), and the top failure reasons as metrics `failed_reason_<code>` for each `state_reason` code that occurs (codes are a closed list of 09 5.2, never free text).
    - `getToolCallStats`, group key tool name (`run_steps.name`; names only, 25 rule 3): `calls`, `failed`, `duration_ms_p50`.
    - `getErrorStats`, group key none (instance) or space: `run_failure_rate` (failed per finished runs), `invocation_denials` (messages whose `trigger_state_reason` is `billing_mode_personal_not_holder` plus runs failed with `billing_mode_mismatch`), and, only for the instance target, `webhook_failure_rate` (`failed` and `dead` per finished deliveries still within retention), `push_failure_rate` (registrations disabled for provider reasons per active and disabled registrations in the range, by provider as separate metrics `push_failure_rate_<provider>`), `scan_errors` and `scan_infected`.
    - `getCostStats`: `CostSeries` with one currency per response: add the optional query parameter `currency` (ISO 4217) to `getCostStats` in `openapi.yaml` first; without it, a range whose cost rows have one currency answers in that currency, and a range with several currencies answers `400 validation_failed` with `errors[].code` `currency_required` and the currencies found, so no amounts of different currencies are ever mixed or dropped, buckets per `billing_mode`, `is_estimate` true when any contributing row is an estimate.
    Percentiles are computed in SQL with `percentile_cont` inside the owning module (no sample transfer) when possible; document the choice.

3. **Buckets and time zone** (admin-statistics.md 5 item 4). `hour` buckets in UTC; `day`, `week` (ISO 8601 week starting Monday) and `month` start at local midnight of `stats.time_zone` and are returned as UTC timestamps of that instant. A bucket that crosses a clock change keeps its local start. `from` and `to` are aligned down and up to the granularity before the query; the response covers the aligned range.

4. **Minimum group** (admin-statistics.md 3 item 2; DP-T11; Ruling for docs/DECISIONS.md). For `active_people` a bucket value below `privacy.stats_min_group_size` is removed from `metrics` and its name is added to the new member `below_min_group`, so the client shows "fewer than `privacy.stats_min_group_size`" (M6-06); the threshold value is not sent, the client reads it from the instance limits it already has or shows the text without the number. The same rule applies if a later metric counts people. No other metric is affected, because none counts people.

5. **No people, no content** (25 1; SEC-VZ-3). The source ports return counts and sums only; no person ID leaves a module, `active_people` is counted with `COUNT(DISTINCT author_principal_id)` inside `messaging`. Group keys are space IDs, agent IDs or tool names; never thread titles, message text or file names.

6. **Cache** (admin-statistics.md 5 item 2; Ruling for docs/DECISIONS.md). An in-memory cache per api process, keyed by operation, normalized parameters, the caller's `PermissionVersion` and a hash of the channel set, value expiring after `stats.cache_ttl` by the clock, bounded by the new configuration key `stats.cache_max_entries` (add it first to 17 11 and `docs/operations/configuration.md` with a default and its reason; arbitrary ranges can produce unbounded distinct keys), evicting the least recently used entry when full. A permission change gives a new `PermissionVersion` and therefore a cache miss, so a revoked manager never sees cached numbers of their old scope.

7. **Visibility lint and indexes** (07 7.4; M1-70). Every new query that reads a table with channel visibility takes `channel_id = ANY($n)` from the passed set; where the SQL visibility lint of M1-70 flags an aggregate query, add one exception with category `system_aggregate` only if the query is filtered by the passed set in another way, with a reason; never widen an existing entry. Use the indexes admin-statistics.md 5 item 1 names; if `EXPLAIN` of a query over `stats.max_range` on the load fixture (Tests) shows a sequential scan of a large table, add the index in a migration of the owning module and record it.

8. **Rulings for `docs/DECISIONS.md`** (section 10): `D-M6-05-1` authorization and channel set per query (note 1); `D-M6-05-2` the metric names and sources (note 2); `D-M6-05-3` `active_people` only for week and month (note 2); `D-M6-05-4` `below_min_group` (note 4); `D-M6-05-5` the cache key with the permission version (note 6); `D-M6-05-6` the new module edges of `stats` (Scope).

## Tests (write them first)

Test world: 07 10 and 11.1 (space Support with channels Allgemein and Eskalation; space Engineering with channel server; Sam with `manage` on Support, Robin with `read` on Support/Allgemein only; the Owner Alex; agents support-agent and dev-runner). Data written through the use cases of the owning modules where they exist (messages, runs, steps, costs), as fixture rows through `pgtest` otherwise (webhook deliveries, push registrations, scans), spread over two days around a clock change of `Europe/Berlin` for the time zone tests.

Unit, `internal/stats/domain`:

- `TestBuckets`: hour, day, week, month alignment in `UTC` and in `Europe/Berlin` across both clock changes; a range over `stats.max_range`, a range with more than `api.stats_max_buckets` buckets and an empty range fail.
- `TestMinGroup`: values below, at and above `privacy.stats_min_group_size`.
- `TestMerge`: per-channel aggregates of two spaces merge into the right group keys; channels outside the set are dropped even if a fake source returns them (defence in depth).

Integration (`pgtest`, full chain through `internal/wire`):

- `TestStats_SpaceManagerSeesOnlyOwnChannels` (PLAN row, VZ-T05 in spirit): Sam with `space_id = Support` gets numbers of Allgemein and Eskalation only; with `space_id = Engineering` `404`; without `space_id` `403`; a message in Engineering never changes Sam's numbers.
- `TestStats_ReadOnlyMemberIsForbidden`: Robin (`read` without `manage`) gets `403` on Support.
- `TestStats_OwnerInstanceWide`: Alex without `space_id` gets all spaces and the instance-only metrics (webhook, push, scans); Sam never gets them.
- `TestStats_MinGroup` (DP-T11): with `privacy.stats_min_group_size` read from the test configuration as `n`, `n - 1` distinct authors in a week give no `active_people` and `below_min_group: ["active_people"]`; `n` give the number; `privacy.per_person_stats_enabled = true` changes nothing; no response of any operation contains a person ID (scan the JSON for every principal ID of the world).
- `TestStats_NoPersonGrouping`: no operation accepts a grouping by person; an unknown query member is `400` (closed request schema of M1-18).
- `TestStats_Costs`: `organization` and `personal` stay separate buckets; an estimate row marks the bucket; a superseded cost row (with a successor) is not counted; Sam (manager of Support, who holds `cost.read.aggregate` there by 05) sees the costs of Support only; Robin gets `403`. The path "no `cost.read.aggregate`" cannot be reached by a person with `stats.read` today (05 gives both to owners and managers); prove it in a unit test of the use case with a stubbed decision.
- `TestStats_RunsAndTools`: end states, percentiles against a hand-computed fixture, failure reason metrics only for codes that occur; tool names appear, step details never.
- `TestStats_ErrorRates`: run failure rate, invocation denials from both sources, webhook and push rates for the owner only.
- `TestStats_CacheRespectsPermissionVersion`: Sam's result is cached; removing Sam's `manage` gives `403` on the next call within `stats.cache_ttl`; a new message within the TTL does not change the cached result, after the TTL it does.
- `TestStats_TimeZone`: daily buckets of `Europe/Berlin` start at local midnight and the day of the clock change has 23 or 25 hours of data.
- `TestStats_Range`: `400` for a range over `stats.max_range`, for more than `api.stats_max_buckets` buckets (for example `hour` over many weeks) and for `from >= to`.
- Load fixture (not part of the default run, behind the `perf` build tag used by `make perf`): 10 channels, 10 agents and one million messages over `stats.max_range`; each operation answers within the budget you measure and document in the report; `EXPLAIN` shows index use (Design note 7).

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/stats/... ./internal/messaging/... ./internal/runs/... ./internal/billing/... ./internal/files/... ./internal/apps/... ./internal/notify/... ./internal/wire/...`
- `make codegen-check`, `make api-lint`, `make api-checks`, `make arch`, `make register-check`, `make config-doc-check`
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] The five operations answer from the domain tables through the source ports of the owning modules, authorized with `stats.read` or `cost.read.aggregate` on the instance or a space.
- [ ] Space managers see no numbers of other channels; values without a channel only for the owner.
- [ ] `active_people` below `privacy.stats_min_group_size` is withheld and listed in `below_min_group`; no response groups by person or names a person (DP-T11).
- [ ] Costs separate the modes and mark estimates; ranges, buckets and time zone follow `stats.max_range` and `stats.time_zone`; the cache follows `stats.cache_ttl` and the permission version.
- [ ] `openapi.yaml`, 02, admin-statistics.md, `modules.yaml`, the READMEs, the CHANGELOG and the rows `D-M6-05-1` to `D-M6-05-6` are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `stats` and of every module with a new `StatsSource` updated in English.
- Rulings of Design note 8 recorded in `docs/DECISIONS.md`.
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
4. The measured answer times of the load fixture and whether `stats_daily` would be needed.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
