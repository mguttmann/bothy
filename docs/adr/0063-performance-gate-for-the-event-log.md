---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team A (architecture), Team H (quality and repo)
informed: Team F (operations)
---

# Performance gate for the event log

## Context and Problem Statement

The event log assigns numbers through a counter row with a row lock (ADR 0009). All
transactions with durable events therefore serialize at the end of their execution; the `NOTIFY`
at commit lengthens this section ([04-sync-protocol.md](../spec/04-sync-protocol.md),
section 3.2; [02-architecture.md](../spec/02-architecture.md), section 11.7). This is the deliberate
limit of the architecture: if the throughput is not enough, a new decision is needed (02, section
16). How is the throughput checked so that a regression shows up before it hits users, and
so that the number means something?

## Decision Drivers

- No number without a test: the target is a named value with a test that holds it (R-AR-20).
- Measurements must be repeatable; performance measurements on arbitrary hardware are not.
- The local gate stays fast and usable for contributors without special hardware (ADR 0038).
- The measurement checks the real pattern with real durability, not a faster shortcut.
- A miss triggers an architecture decision, not a silent change of the threshold.

## Considered Options

- Absolute target `perf.event_commit_min_per_s` on `perf.reference_profile`, measured by
  `perf/event_commit`, checked with `make perf` at the end of a milestone, in the release and in
  tickets that touch the write path
- The same measurement in every `make check` and in CI
- Relative benchmark: comparison with the state before the change on the same machine
- Monitoring in operations only (`bothy.db.event_seq.lock_wait` with an alert)
- No gate; capacity estimated

## Decision Outcome

Chosen option: "Absolute target on the reference profile with `make perf`", because only an absolute
number says whether the limit of the architecture is enough, and only a fixed profile makes it
repeatable. Basis: [02-architecture.md](../spec/02-architecture.md), section 17, and
[PLAN.md](../PLAN.md), P-07.

1. `perf/event_commit` has many parallel writers run realistic transactions
   (domain change, several events, counter step last, `pg_notify`) and measures durable
   events per second as well as the wait time at the counter lock.
2. Measurement happens only on `perf.reference_profile` with the durability settings of operations
   (`fsync` and `synchronous_commit` on). Reason: without durability the commit would be shorter and
   the number worthless.
3. `make perf` writes, per target, the measured value, the threshold and the measured environment
   and fails when the value is below the threshold. The environment gets a mode from the vCPU count
   and the memory of `perf.reference_profile`: `match` (exactly as many logical CPUs, at least as
   much memory), `emulated` (larger machine, limited to the profile: `GOMAXPROCS` equal to the vCPU
   count, PostgreSQL container with `--cpus` and `--memory` from the profile) or `below` (smaller
   machine). Only `match` and `emulated` count as evidence; a run with `below` or without the
   durability settings from item 2 does not end green, but shows the measurement. Reason for
   `emulated`: a dedicated machine exactly matching the profile would be a question of cost; limiting
   to the profile measures the same parallelism that a CPU count in the profile means.
4. `make perf` runs at the end of every milestone, in the release and in every ticket that touches a
   performance target ([PLAN.md](../PLAN.md), section 3, rule 6); not in the pre-push.
5. No ticket lowers the threshold unless its brief says so explicitly (PLAN, section 3,
   rule 6). If the target is missed, a new ADR decides on the pattern (for example sequences
   per space, 02 section 16).

### Consequences

- Good, because the capacity limit of the design is measured instead of estimated.
- Good, because the local gate stays fast and contributors need no reference system.
- Good, because the measurement also answers whether River creates additional `NOTIFY` load and
  `PollOnly` is needed (02, section 21).
- Bad, because a regression between two milestones shows up only at the end of the milestone if the
  ticket did not recognize that it touches the performance target.
- Bad, because a reference system must be available; parts of the profile (for example SSD) can
  be checked only to a limited extent.
- Neutral, because a separate test holds the correctness of the pattern (concurrency test from
  ADR 0009); this gate checks only the throughput.

### Confirmation

- Build ticket M1-16 in [PLAN.md](../PLAN.md): `make perf` measures `perf.event_commit_min_per_s` on
  `perf.reference_profile` and fails when the value is below the threshold (built:
  `TestEventCommitThroughput` in `services/server/perf/event_commit` on a database from
  `pgtest.Durable()`; it fails when `fsync` or `synchronous_commit` is not `on` and below the
  threshold, which `perf/internal/report` reads from the table of 02, `TestReport_FailsBelowThreshold`).
- Build ticket M1-72: `make perf` writes the measured value and the threshold per target; chaos tests
  of the event chain (loss of the `LISTEN` connection, database restart, SIGKILL during commit)
  leave no gap.
- Test of the tool: the modes `match`, `emulated` and `below` result from fixed
  example machines; a run in mode `below` or with `synchronous_commit = off` does not end green;
  `results.json` names the mode (build ticket M1-72).
- Built by M1-72: `make perf` measures every M1 target of 02 section 17 on the profile of
  point 3: `perf.message_create_p99` under `perf.sustained_messages_per_s`
  (`TestPerf_MessageCreateP99`), `perf.fanout_local_p99` (`TestPerf_FanoutLocalP99`),
  `perf.catchup_page_p99` with a fractional and a large visible set (`TestPerf_CatchupPageP99`,
  `TestPerf_CatchupPageP99LargeVisibleSet`), `perf.authorize_cached_p99`
  (`TestPerf_AuthorizeCachedP99` over `BenchmarkAuthorizeCached`), `perf.ws_connections_per_node`
  and `perf.ws_idle_conn_mem_max` (`TestPerf_WSConnectionsPerNode`), `perf.startup_ready_max` and
  `perf.idle_rss_max` (`TestPerf_StartupReadyAndIdleRSS`), and `perf.event_commit_min_per_s`
  (`TestEventCommitThroughput`). One results file `.cache/perf/results.json` holds value,
  threshold, comparison and pass per target, the profile with its mode and the durability settings;
  `perfreport` prints the table and exits non-zero on a miss (`TestExitFailsOnMissedTarget`).
- The modes come from fixture hosts in `TestProfileModes`; the container runtime of the database
  counts as well, so a large host whose Docker VM is below the profile is `below`.
  `TestProfileRejectsNonDurable` shows that a run with `fsync = off` or `synchronous_commit = off`
  does not end green although every target passes.
- Chaos tests of the event path (M1-72, `services/server/test/chaos`): `TestChaos_ListenConnectionLoss`,
  `TestChaos_DatabaseRestartDuringTail` and `TestChaos_SigkillDuringCommit` check under load that
  `seq` stays gapless, the counter equals its maximum, every acknowledged message exists once with
  one `message.created`, no idempotency key stays `in_progress`, the audit chain is valid and every
  receiver gets every event once in order.
- Build ticket M2-36: `make perf` measures both targets of the agent path,
  `perf.invocation_delivery_p99` (`TestPerf_InvocationDeliveryP99` in
  `services/server/perf/invocation`) and `perf.run_patch_fanout_p99` (`TestPerf_RunPatchFanoutP99`
  in `services/server/perf/run_patch`), against a server process with the default configuration on
  a durable database under `perf.sustained_messages_per_s`, with the Go fake adapter of
  `perf/internal/fakeadapter` as the measuring point, in the profile modes, and fails when one is
  exceeded or a check of the path fails (`TestReport_AgentPathTargetsFailAboveThreshold`).
- Build ticket M3-24: `make perf` measures `perf.push_handoff_p95` against Fake-APNs on a durable
  database in the profile modes and fails when it is exceeded (`TestPerf_PushHandoffP95` in
  `services/server/perf/push_handoff`, `TestReport_PushHandoffFailsAboveThreshold`).
- Build ticket M3-16: `make perf` measures both search targets of 14 section 7,
  `search.p95_target` on a corpus of `search.benchmark_messages` messages and
  `search.prefix_p95_target`, against a server process with the default configuration on a durable
  database in the profile modes, and fails when one is missed or a check of the path fails
  (`TestPerf_Search` in `services/server/perf/search`, `TestReport_SearchTargetsFailAboveThreshold`;
  `make bench-search` runs it alone).
- Exit criterion of every milestone ([PLAN.md](../PLAN.md), section 4, item 3).

## Pros and Cons of the Options

### Absolute target on the reference profile

- Good, because the number is a statement about capacity.
- Bad, because it holds only on the profile.

### Measurement in every `make check`

- Good, because regressions would show up immediately.
- Bad, because laptops, runners and containers differ in speed; the gate would turn red at random
  or would have to be so generous that it no longer checks anything.

### Relative benchmark

- Good, because it finds regressions against the previous state on any machine.
- Bad, because it does not say whether the throughput is enough; two small regressions below the
  noise threshold add up unnoticed.
- Neutral, because it remains useful as a supplement in tickets on the write path.

### Monitoring in operations only

- Good, because real load is measured.
- Bad, because a bottleneck shows up only for users; the metric stays in the dashboard anyway
  ([16-operations.md](../spec/16-operations.md), section 13).

### No gate

- Bad, because the central limit of the architecture would remain unchecked.

## More Information

- Related: ADR 0004 (modular monolith), ADR 0009 (event log), ADR 0038 (local gates).
- Further performance targets of the server core (`perf.*` in 02, section 22) run in the same
  `make perf` (build ticket M1-72); this ADR justifies the form with its most important target.
- M1-72 measures every target of M1 in `make perf` with the same rules: thresholds only from the
  table of 02, one tag `perf` for load and chaos tests, packages one after another, durable
  databases only. The modes `match` and `emulated` are evidence; `below` prints the values as not
  proven. `services/server/perf/README.md` names each test, its parameters and how to read the
  table; the milestone-close run on the reference host is an owner operation.
- The targets of the agent path (M2-36) run in the same `make perf`: `perf.invocation_delivery_p99`
  from `occurred_at` of the triggering transaction to the receipt of `agent.invocation` at the fake
  adapter, and `perf.run_patch_fanout_p99` from the scheduled `run.update` of the fake adapter to
  the first `run.updated` with that version at every focused viewer, both under
  `perf.sustained_messages_per_s` with the default rate limits and the same profile modes and
  durability rules; a value counts only when the path behaved correctly while it was taken.
- The push target `perf.push_handoff_p95` (M3-24) runs in the same `make perf`: commit of a
  mention to the `200` of Fake-APNs under `perf.sustained_messages_per_s`, with the server in the
  test process and the same profile modes and durability rules.
- The search targets of [14-search.md](../spec/14-search.md) section 7 (`search.p95_target`,
  `search.prefix_p95_target`, with the corpus size `search.benchmark_messages`; M3-16) have the
  place `Load test` in the table of 14 section 15 and run in the same `make perf` with the same
  profile modes and durability rules; `perf/internal/report` reads the tables of 02 and 14.
