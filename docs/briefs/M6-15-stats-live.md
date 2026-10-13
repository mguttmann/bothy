# M6-15: stats: live stream `stats.live`

Status: ready | Depends on: M6-13 | Milestone: M6

## Goal

After this ticket a person with `stats.read` can subscribe to live frames on the gateway and the console can poll the same frame over REST (25 5, 04 12 item 4): the ops `stats.subscribe {space_id?}` and `stats.unsubscribe` move from `gateway_ops.later` to `gateway_ops.enabled`; a subscription is checked with `authorize(principal, stats.read, instance | space)` and answered with `error` (`forbidden`, `not_found`, `feature_disabled`, `rate_limited`) without closing the connection; at most one subscription per connection, a new one replaces it, `resume` restores it. While at least one subscription exists, each api node builds exactly one base frame (`StatsLiveFrame`) per `stats.live_interval` from the runs, run steps, agents, presence and cost ports and the last full and current minute of `stats_minutes`, and filters it per connection in memory in the delivery mode of `authorize` with the receive action `stats.read` (channels, cost members, instance-only members); without a subscription the node builds nothing. A revocation (`access.changed`) ends the subscription with one `error` `forbidden` and no further frame; more than `stats.live_max_subscribers` subscriptions per node give `error` `rate_limited`; `stats.live_enabled = false` rejects both paths with `feature_disabled`. `getStatsLive` returns the most recently built base frame of the node, filtered for the caller, without rebuilding per request. A frame never exceeds `sync.event_payload_max_bytes`: the runs list is shortened and `runs_truncated` set. VZ-T05 for the stream, VZ-T06, VZ-T07, VZ-T08, VZ-T10 and DP-T12 are green. Why: the live topology and delivery views of M6-12 need a stream whose load depends on the number of intervals, not on the number of viewers (R-VZ-05), and which leaks nothing to space managers (T-VZ-02).

## Read first

- `docs/spec/25-visualization.md`: 1, 4 (permissions, rules 1 to 5), 5 (all of it: 5.1 flow items 1 to 6 and the console paragraph, 5.2 the frame table and the size rule), 11, 12 (VZ-T05 to VZ-T08, VZ-T10), 14 (R-VZ-05, R-VZ-06, R-VZ-07), 15 (`stats.live_enabled`, `stats.live_interval`, `stats.live_max_runs`, `stats.live_max_subscribers`, `stats.live_sli_window`).
- `docs/spec/04-sync-protocol.md` (core contract, read only): 2.1 and 2.2 (ephemeral events: no `seq`, not in the log, `truncated` never set here), 6.2 (ops table rows `stats.subscribe`, `stats.unsubscribe`), 6.3 (`resume` restores subscriptions), 6.4 (`error` frames), 7.2 item 6 (no ephemeral event before `catchup.complete`, dropped on a full queue), 8 (revocation), 12 items 2 to 4.
- `docs/spec/05-actions.md` (core contract, read only): `stats.read`, `cost.read.aggregate`, section 6 (ephemeral events table, row `stats.live`; ops table rows `stats.subscribe`, `stats.unsubscribe`).
- `docs/spec/20-privacy.md` 9 (codetermination, `stats.live_enabled`), DP-T12.
- `docs/spec/02-architecture.md`: 10 "Gateway", 11.4, 11.6 (ephemeral events across nodes; the live frame is built per node and needs no cross-node path), 16.
- `docs/api/openapi.yaml`: `getStatsLive`, `StatsLiveFrame` and its members (`StatsLiveRun`, `StatsLiveAgent`, `StatsLiveRate`, `StatsLiveSli`, `StatsLiveConnections`, `StatsLiveStep`, `StatsScope`), `GatewayStatsSubscribe`. `docs/api/asyncapi.yaml`: `op.stats.subscribe`, `op.stats.unsubscribe`, `event.stats.live`, `receive.stats.subscribe`.
- `docs/observability/signals.md` 4.3b (`bothy.stats.live.subscribers`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-15-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-64-gateway-connection.md` (`gateway_ops`, dispatcher, op handlers, the proof that an op called `authorize`), `M1-65-gateway-delivery.md` (hub indices, `sync.Admit`, `authorize.Delivery(ctx)`, delivery of ephemeral events, phases), `M1-66-gateway-focus-typing.md` (`EphemeralEvent`, `EphemeralPublisher`, per-connection state that `resume` restores), `M2-20-heartbeat-presence.md` (presence store), `M2-09-run-lifecycle.md`, `M2-15-components-interactions.md` (approval cards), `M2-22-costs.md` (cost so far of a run), `M6-13-stats-measure.md` (table and minutes). The frame computes its own SLI window from `stats_minutes`; it does not use the queries of M6-14.
- Code on the base branch (all under `services/server/`): `internal/gateway`, `internal/sync` (`Admit`), `internal/stats`, `internal/runs`, `internal/agents`, `internal/billing`, `api/operations.yaml`.

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
9. Limits, windows and intervals only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. A subscription is authorized at `stats.subscribe` and every frame passes the delivery mode with `stats.read` and is filtered to the visible channels of the receiver; agents and apps cannot subscribe (04 12 item 4).
12. One base frame per node and `stats.live_interval`, only while a subscription exists; per-connection filtering in memory; the load does not grow with the number of viewers beyond the filtering (25 5.1 item 4).
13. The frame contains no person, no triggering person and no content: runs carry IDs, state, billing mode, step kind and name, approval flag and cost; nothing else (25 5.2, SEC-VZ-3).
14. Cost members only with `cost.read.aggregate` on the target; `sli` and `connections` only for instance scope; `connections` only with `cluster.mode = single` (25 5.2).
15. A frame never exceeds `sync.event_payload_max_bytes`; `runs` is shortened and `runs_truncated` set; `truncated` of the envelope is never set (25 5.2).
16. Revocation ends the subscription with exactly one `error` `forbidden`; `stats.live_enabled = false` gives `feature_disabled` for the op and `403 feature_disabled` for `getStatsLive`; over `stats.live_max_subscribers` on the node gives `error` `rate_limited` (25 5.1, 11).
17. `getStatsLive` returns the most recently built base frame (built within the last `stats.live_interval`), filtered for the caller; it never triggers a rebuild per request (25 5.1, R-VZ-06).

## Scope

All paths under `services/server/` unless they start with `docs/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create:

- `internal/stats/domain/live.go`: the frame filter (channel set, cost permission, scope rules), the size cut (Design note 4).
- `internal/stats/app/live.go`: the `Live` port of the stats README (subscribe, unsubscribe, current frame), the builder component that runs one loop per node while subscriptions exist (Design notes 1 to 3).
- `internal/stats/adapters/httpapi/live.go`: `getStatsLive`.
- `internal/gateway/app/statsops.go`: the op handlers `stats.subscribe` and `stats.unsubscribe`, per-connection subscription state restored by `resume`, and delivery of each filtered frame as ephemeral event `stats.live` to that connection only (Design note 5).
- Source ports where missing: `runs.LiveSource` (active runs in a channel set with current step kind and name, approval pending flag, started time; ordered newest first; limited), `billing.RunCostSource` (cost so far per active run with currency and estimate flag), `agents.LiveSource` (presence, configured and reported mode, `rate_limit` for a set of agents).
- Tests named under "Tests".

Change:

- `api/operations.yaml`: `stats.subscribe` and `stats.unsubscribe` from `later` to `enabled` (M1-64 note: "M6, ticket M6-15"); enable `getStatsLive`; regenerate.
- `internal/sync` (`Admit`): the case of `stats.live` if the generic path does not already apply `stats.read` on the frame's scope resource (instance or space) to the connection's actor; with a unit test.
- `internal/wire`: wiring of the builder in roles `api` and `all`; configuration keys `stats.live_enabled`, `stats.live_interval`, `stats.live_max_runs`, `stats.live_max_subscribers`, `stats.live_sli_window` enter the schema (first user; `make config-reference`, `make config-doc-check`); metric `bothy.stats.live.subscribers` registered.
- `docs/api/openapi.yaml` and `asyncapi.yaml`: descriptions only where the build sharpens them; `make api-lint`, `make api-checks`, `make codegen-check`.
- Docs: `docs/spec/25-visualization.md` 5 only where the build sharpens a rule; `docs/spec/02-architecture.md` 10 (the op handlers) and 11.6 (a row `stats.live`: built per node, no cross-node path); `internal/stats/README.md`, `internal/gateway/README.md`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M6-15-1` to `D-M6-15-7`.

This ticket adds no table, no migration, no action, no event type and no top-level Makefile target.

## Out of scope

- The client sources (`sync_engine` subscription, console polling) and the views (M6-12, M6-16).
- Cross-node presence and connection counts with `cluster.mode = multi` (M7-17; the frame omits `connections` there).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-15-stats-live`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(stats): ...`, `feat(gateway): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. Logs of this ticket carry no person, channel or agent ID. No network calls except package downloads for pinned tools and Go modules.
- Time only through `kernel/clock`; tests advance a fake clock and drive the builder tick, they never sleep for an interval.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Subscription** (25 5.1 items 1, 3, 6; 04 12 item 4; Ruling for docs/DECISIONS.md). The op handler validates `GatewayStatsSubscribe`, then: `stats.live_enabled` false gives `error` `feature_disabled`; `authorize(actor, stats.read, instance)` without `space_id`, else `authorize(actor, stats.read, space)` with `not_found` for an unreadable space and `forbidden` otherwise; the node-wide count over `stats.live_max_subscribers` gives `error` `rate_limited`. On success the connection's state holds `{scope, space_id}` (replacing an earlier subscription) and the count gauge rises. `stats.unsubscribe` clears it (no error without a subscription). The state is part of what `resume` restores (M1-66 per-connection state); a restored subscription is re-authorized at `resume`. A closed connection decrements the gauge.

2. **Builder** (25 5.1 items 4 and 5; R-VZ-05; Ruling for docs/DECISIONS.md). A lifecycle component per api node: when the first subscription appears it starts a ticker of `stats.live_interval`; when the last one ends it stops (a test counts builds). Each tick builds one base frame for the instance: active runs of all channels (`runs.LiveSource`, newest first; keep enough runs that every receiver's filtered list can still hold `stats.live_max_runs`, and document the bound you choose), cost so far per run, agents with a run or an operator role (`agents.LiveSource`), rates per metric of 25 3.2 without `sli.*` from `stats_minutes` for the last full minute and the current minute (`dimension` summed and per dimension where the views need it, currency for `cost`), SLI totals over `stats.live_sli_window`, and `connections` from the hub when `cluster.mode = single`. The base frame keeps per-channel and per-run attribution internally so the filter can drop what a receiver must not see; it is never sent unfiltered. The builder keeps the last base frame with its build time for `getStatsLive`.

3. **Rates per receiver** (25 5.2 row `rates`). Rates must cover only visible channels, so the base frame holds rates per channel (and the instance rows without a channel separately); the filter sums the channels of the receiver's set and adds instance rows only for instance scope. Cost rates only with `cost.read.aggregate`.

4. **Filter and size** (25 4, 5.2; VZ-T08; Ruling for docs/DECISIONS.md). Per receiver and frame: channel set from `Scope` cached per connection and invalidated by the permission version (as the delivery mode does); space scope keeps only that space's channels; drop runs and rates outside the set; drop agents without a run or operator role in the set; drop cost members without `cost.read.aggregate` on the target; drop `sli` and `connections` for space scope. Encode; if the payload exceeds `sync.event_payload_max_bytes`, drop the oldest runs until it fits and set `runs_truncated`; never set the envelope's `truncated`. `interval_ms` = `stats.live_interval`, `at` the build time, `scope` from the subscription.

5. **Delivery** (04 2.1, 7.2 item 6; 25 5.1 item 3; Ruling for docs/DECISIONS.md). Each filtered frame is an `EphemeralEvent` `stats.live` with the receiver as audience and the scope resource (instance or space), published to that one connection only through the hub, after `sync.Admit` passes it with `stats.read` in the delivery mode (`authorize.Delivery(ctx)`, rights in log order). A connection that is not live gets nothing; a full queue drops the frame (the next one replaces it). When `Admit` denies `stats.read` (revocation applied by the tail), the handler ends the subscription and sends one `error` with code `forbidden` and a reference to `stats.subscribe` (`op` member), then nothing more. The same happens when the per-connection channel set becomes empty for a space scope because the space became unreadable (send `not_found`).

6. **`getStatsLive`** (25 5.1 last paragraph; R-VZ-06). Authorize as Design note 1 (REST errors `403 forbidden`, `404`, `403 feature_disabled`); a `getStatsLive` request counts as a subscription of the node for `stats.live_interval` (record this in 25 5.1 item 4, which otherwise says a node builds only with a subscription, and which 5.1's last paragraph needs for the console), so the builder runs while polls arrive and stops one interval after the last poll; the first poll on an idle node builds once in a single flight shared by all callers; then filter for the caller exactly as Design note 4. A burst of polls from many consoles therefore builds at most one frame per interval.

7. **Rulings for `docs/DECISIONS.md`** (section 10): `D-M6-15-1` subscription checks, replacement, `resume` (note 1); `D-M6-15-2` one builder per node, start and stop with subscriptions (note 2); `D-M6-15-3` per-channel rates in the base frame (note 3); `D-M6-15-4` filter and size cut (note 4); `D-M6-15-5` delivery and ending on revocation (note 5); `D-M6-15-6` on-demand build for `getStatsLive` with single flight (note 6); `D-M6-15-7` `connections` only with `cluster.mode = single`.

## Tests (write them first)

Test world: 07 10 and 11.1; Sam with `manage` on Support (and therefore `cost.read.aggregate` there, 05); the Owner Alex; runs of support-agent in Support and dev-runner in Engineering created through the runs use cases, an approval card pending on one; cost bookings through billing; `stats_minutes` rows for the current and the last minute written by the M6-13 flush. Gateway clients are `wstest` of M1-64 against the real server on `127.0.0.1:0`; "live" means `catchup.complete` was received.

Unit, `internal/stats/domain`:

- `TestFilter`: space scope drops other spaces, instance members and costs without permission; owner keeps everything; agents without a run or operator role in the set disappear.
- `TestSizeCut` (VZ-T08): a frame with more than `stats.live_max_runs` runs and a payload over `sync.event_payload_max_bytes` (small value in the test configuration) is cut to fit with `runs_truncated = true`; the envelope has no `truncated`.

Integration (`pgtest`, full chain, role `all`):

- `TestLive_SpaceManager` (VZ-T05 for the stream): Sam subscribes with `space_id = Support` and receives frames every `stats.live_interval` of the fake clock with only Support channels and their costs, no `sli`, no `connections`; the path without `cost.read.aggregate` is the unit test `TestFilter` with a stubbed decision; Alex subscribes without `space_id` and receives everything including costs and `sli`.
- `TestLive_Revocation` (VZ-T06): Sam's `manage` on Support is removed during the subscription; Sam receives exactly one `error` `forbidden` and no further `stats.live`.
- `TestLive_NoContentNoPeople` (VZ-T07, DP-T12): canary values in the message, thread title, step title and step detail of an active run appear in no frame and no `getStatsLive` response; no principal ID of a person appears; `privacy.per_person_stats_enabled = true` changes nothing.
- `TestLive_Disabled` (VZ-T10): `stats.live_enabled = false` gives `error` `feature_disabled` for the op and `403 feature_disabled` for `getStatsLive`.
- `TestLive_MaxSubscribers`: with `stats.live_max_subscribers` set to two in the test configuration, the third subscription gets `error` `rate_limited` and its connection stays open.
- `TestLive_NoBuildWithoutSubscription`: the build counter stays 0 without subscriptions, rises by one per interval with three subscribers (not three), and stops after the last unsubscribe.
- `TestLive_ResumeRestores`: a connection resumes within `sync.resume_window` and keeps receiving frames without a new `stats.subscribe`; `identify` instead of `resume` has no subscription.
- `TestLive_AgentCannotSubscribe`: an agent connection gets `error` `forbidden`.
- `TestGetStatsLive_NoRebuildPerRequest`: ten requests within one interval cause at most one build; the response is filtered per caller like the stream.
- `TestGatewayOpsEnabled`: `stats.subscribe` and `stats.unsubscribe` are in `gateway_ops.enabled`; the M1-64 test of the enable list passes.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/stats/... ./internal/gateway/... ./internal/sync/... ./internal/runs/... ./internal/agents/... ./internal/billing/... ./internal/wire/...`
- `make codegen-check`, `make api-lint`, `make api-checks`, `make arch`, `make register-check`, `make config-doc-check`, `make signal-check`
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] VZ-T05 for the stream, VZ-T06, VZ-T07, VZ-T08, VZ-T10 and DP-T12 are green.
- [ ] More than `stats.live_max_subscribers` subscriptions per node give `error` `rate_limited`; without a subscription the node builds no frame (counter in the test).
- [ ] `getStatsLive` returns the most recently built frame, filtered per caller, with at most one build per interval.
- [ ] The ops are enabled; the READMEs, 02 10 and 11.6, the CHANGELOG and the rows `D-M6-15-1` to `D-M6-15-7` are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `stats` and `gateway` updated in English.
- Rulings of Design note 7 recorded in `docs/DECISIONS.md` section 10.
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
4. The measured build time of one base frame with `stats.live_max_runs` runs and the filter time per receiver.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
