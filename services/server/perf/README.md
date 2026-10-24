# Load tests of the server core (`services/server/perf`)

Status: built (M1-16 `event_commit`, M3-24 `push_handoff`, M1-72 everything else). Component `testsupport` (M1-05),
never imported by production code, left out of the Go coverage total.

`make perf` runs these tests and the chaos tests of `test/chaos` with the build tag `perf`, one
package after another (`go test -p 1`, so no two measurements share the machine), then
`perf/internal/report/perfreport` prints the table and decides the exit code. It is not part of
`make check` (plan P-07); `make check` only compiles, vets and lints the tagged code. The
thresholds are the `perf.*` rows of the table "Configuration values of this chapter" in chapter 02
(found by its title, English or German); no test holds a threshold as a literal.

## Profile modes (ADR 0063, point 3)

`perf.reference_profile` names a vCPU count and memory. The mode compares them with the host of
the test process and with the container runtime of the database (`docker info`, the Linux VM of
Docker on macOS):

- `match`: the host has exactly the vCPUs of the profile and at least its memory. The memory counts as
  the operating system reports it (MemTotal, the memory of the Docker VM), which leaves out what the
  kernel keeps, so a share of the profile's memory is enough (`report.hasMemory`, M1-72/16).
- `emulated`: the host is larger; the server process gets `GOMAXPROCS` equal to the vCPUs of the
  profile and the PostgreSQL container `--cpus` and `--memory` of the profile (`pgtest.Limits`).
- `below`: the host or the container runtime is smaller than the profile. The values are measured
  and printed, but the run never ends green: they are not proven on the profile.

Every measured database runs with the PostgreSQL defaults for durability (`pgtest.Durable()`);
before the first measurement each test reads `SHOW fsync` and `SHOW synchronous_commit` into the
profile. A value other than `on` ends the run not green.

## Results

`.cache/perf/results.json` at the repository root (removed at the start of every `make perf`):

```json
{"profile": {"mode", "cpus", "memory_bytes", "os", "postgres_version", "gomaxprocs", "fsync", "synchronous_commit"},
 "results": [{"name", "value", "unit", "threshold", "comparison", "pass", "samples", "test"}],
 "extra": {"<package>.<measurement>": ..., "<package>.params": {...}}}
```

Value and threshold are in the unit of the threshold (`us`, `ms`, `s`, `KiB`, `MiB`, `GiB`, or a
plain count). `comparison` is `>=` for "target (minimum)" and `<=` for "target (maximum)" and a
plain "target". Percentiles are nearest-rank over the measured window, warm-up excluded.
`perfreport` prints the mode first, then one line per target and test, then `extra`, and exits
`1` when the mode is `below`, a durability setting is off, a target of M1 has no result, or a
result misses its threshold. `extra` also holds the named parameters of each test.

Reading the table: a missed target in mode `match` or `emulated` is a finding for the code (fix it
in the ticket, never by changing a threshold, PLAN "Way of working per ticket" item 6); in mode
`below` it is "not proven" and goes into the report as such.

## Tests and their parameters

Each test seeds its world through the public API with a server process that raises every
`ratelimit.*` (`harness.StartProcess`, `harness.RaisedRateLimits`), then restarts the process on
the same database with the default configuration and measures. Simulated clients send from their
own documentation address (`2001:db8::/32`, one `/64` each) through the trusted loopback proxy
path, as clients behind the reverse proxy of a deployment do, so the per-address
`rl.public_auth` does not limit the whole load as one client.

| Package | Test | Target | Parameters (named constants) |
|---|---|---|---|
| `load_messages` | `TestPerf_MessageCreateP99` | `perf.message_create_p99` under `perf.sustained_messages_per_s` | `warmup`, `window`, `threads`, `receivers`, `headroom` |
| `gateway_fanout` | `TestPerf_FanoutLocalP99` | `perf.fanout_local_p99` | `warmup`, `window`, `receivers`, `threads`, `headroom`, `settle` |
| `catchup` | `TestPerf_CatchupPageP99`, `TestPerf_CatchupPageP99LargeVisibleSet` | `perf.catchup_page_p99` | `channels`, `messagesPerChannel`, `visibleChannels`, `readers`, `largeChannels`, `largeMessagesPerChannel`, `largeReaders`, `walks` |
| `authorize_cached` | `TestPerf_AuthorizeCachedP99` | `perf.authorize_cached_p99` | `benchtime` |
| `ws_soak` | `TestPerf_WSConnectionsPerNode` | `perf.ws_connections_per_node`, `perf.ws_idle_conn_mem_max` | `settle`, `intervals`, `ackGrace`, `fdReserve` |
| `startup` | `TestPerf_StartupReadyAndIdleRSS` | `perf.startup_ready_max`, `perf.idle_rss_max` | `starts`, `idleSettle` |
| `event_commit` | `TestEventCommitThroughput` | `perf.event_commit_min_per_s` | flags `perf.writers`, `perf.duration` |
| `push_handoff` | `TestPerf_PushHandoffP95` | `perf.push_handoff_p95` under `perf.sustained_messages_per_s` | `BudgetShare`, `SendersPerChannel`, `RecipientsPerChannel`, `MentionEvery`, `MentionsPerMessage`, `Warmup`, `Window`, `Drain`, `MinSamples`, `LagSampleEvery` (section `push_handoff`) |
| `stats_queries` | `TestPerf_StatsQueries` | none yet: records the answer time of each statistics operation over `stats.max_range` and the plans of the message and run reads (M6-05) | `channels`, `agents`, `messages`, `runsPerChannel`, `samples` |

- `load_messages`: open-loop arrivals at `perf.sustained_messages_per_s`, spread over as many
  senders as keep each one at `headroom` of `ratelimit.rl.message.per_minute`, into the main
  threads of `threads` channels, while `receivers` of the senders hold gateway connections. The
  latency of a send runs from its scheduled time (no coordinated omission) to the answer on
  loopback, an upper bound of the server time. Any non-2xx answer fails the target. `extra`: pool
  occupancy and lock waiters sampled from `pg_stat_activity` (in use p99 and maximum, share of
  samples with every connection of `database.max_conns` in use), and the growth of the database,
  `messages` and `event_log` with their indexes per thousand messages.
- `gateway_fanout`: `receivers` people with read on a space hold gateway connections while senders
  write into its channels at the sustained rate. Latency per event and receiver is the receive
  time minus `occurred_at` (transaction time, so an upper bound), corrected by the clock offset
  between database and host (`report.ClockOffset` over bracketed `SELECT clock_timestamp()`).
  Every receiver must get every message exactly once, every dispatch above the seq before it.
- `catchup`: `listEvents` with `limit` = `sync.catchup_page_size`; only full pages are samples.
  The first variant has readers that see `visibleChannels` of `channels`; the second has readers
  with an own grant on each of `largeChannels` channels (07 "Query pattern" rule 5).
- `authorize_cached`: runs `BenchmarkAuthorizeCached` of `internal/authz/app` with
  `-run '^$' -bench '^BenchmarkAuthorizeCached$'` (`-cpu` of the profile in emulated mode) and
  reads its `p99-ns`.
- `ws_soak`: `perf.ws_connections_per_node` people, one device and one connection each, through
  `identify` to `ready`, idle with heartbeats for `intervals` times `sync.heartbeat_interval`.
  Passes when all stay connected and every heartbeat is acknowledged; memory per connection is
  (RSS after the connections settled minus RSS after ready) divided by the count. It checks the
  hard `RLIMIT_NOFILE` first and raises the soft limit, which the server process inherits.
- `startup`: `starts` starts of `bothy serve --role=all` on a migrated database; time from exec to
  the first `200` of `/readyz` and RSS after `idleSettle`; the maximum is compared.
- `event_commit`: durable events per second with the counter pattern in this process (M1-16); in
  emulated mode with `GOMAXPROCS` and container limits of the profile. The second subtest with an
  idle River worker only stores its rate in `extra`.

## `push_handoff`

`TestPerf_PushHandoffP95` (M3-24) measures "commit to handoff to the push service (fake)": the
time from the commit of a message that mentions a person to the moment Fake-APNs accepts the push
for that person's device with `200`.

- Measuring points (13 section 9): t0 is `occurred_at` of the `message.created` event of the
  mention, the time of its transaction, read after the run in one read-only query; it lies at or
  before the commit, so each sample is an upper bound. t1 is `apnstest.Request.AcceptedAt`, taken
  after the Fake read the request and verified the JWT, right before it writes `200`. Each sample
  is t1 minus t0, corrected by the clock offset between database and host
  (`report.ClockOffset`); one sample per mention message scheduled in the window and per mentioned
  recipient; the value is the nearest-rank p95.
- The server runs in the test process (`harness.Start` with `Options.APNsFake` and
  `Options.DurableDB`), because only Go values (`wire.Deps.APNsEndpoints`, `wire.Deps.APNsRootCAs`)
  point the wired APNs adapter at the Fake; APNs is enabled through `push.apns.team_id`,
  `push.apns.key_id`, `push.apns.private_key_ref` (a P-256 key generated per run) and
  `push.apns.topics`. In mode `emulated` the test sets `GOMAXPROCS` of the profile and the
  container limits; the load generator and Fake-APNs share the CPUs with the server, so the value
  is an upper bound of the server's own time (`extra` says so).
- World (`perf/internal/pushworld`): one space, `channels` text channels, `senders` with `write`
  and `recipients` with `read` on their channel, the level `mentions` per recipient on its channel
  (`putNotificationPreference`) and one `apns` registration per recipient, written as a fixture
  row because `registerPush` is not part of the measured path. Every `ratelimit.*` key is raised
  for the seeding except `ratelimit.rl.message.per_minute` and `.burst`, which keep their defaults;
  the seeding sends no message, and the measured path after the commit uses no rate-limit class.
  No recipient opens a gateway connection or reads.
- Load (`perf/internal/pushhandoff`): open-loop sends (`perfkit.Schedule`) at
  `perf.sustained_messages_per_s`, round robin over the senders, each into the main thread of its
  channel; every `MentionEvery`-th message mentions `MentionsPerMessage` recipients of its channel,
  round robin per channel, by handle in `content_md` and in `mention_ids`. The plan derives
  senders = ceil(rate / (`BudgetShare` x `ratelimit.rl.message.per_minute` / 60)), channels =
  ceil(senders / `SendersPerChannel`), recipients = channels x `RecipientsPerChannel`.
- Parameters: `BudgetShare` 0.5, `SendersPerChannel` 10, `RecipientsPerChannel` 4,
  `MentionEvery` 10, `MentionsPerMessage` 2, `Warmup` 30 s, `Window` 3 min, `Drain` 30 s,
  `MinSamples` 1000, `LagSampleEvery` 1 s. At their defaults the value holds for
  `consumer.batch_size`, `consumer.poll_interval`, `jobs.queue.delivery.max_workers`,
  `push.max_concurrent_requests`, `push.max_age`, `push.ttl`, `push.apns.jwt_refresh_interval`,
  `notify.coalesce_window` and `egress.timeout`.
- Failure conditions besides the threshold (`pushhandoff.Check`): an answer of `createMessage`
  other than `201`; an expected pair of the window without a request after `Drain`; two requests
  for one pair or with one `nid`; a request that does not decode strictly, whose `b.m` is no
  mention message of the run, whose token belongs to no seeded registration, whose recipient was
  not mentioned, whose `b.k` is not `mention`, whose `b.t` or `apns-collapse-id` is not the main
  thread of the message, or whose `aps.badge` is missing or below one; a badge sync request; the
  per-run canary of `content_md` in any body or header; a Fake answer other than `200`; a
  registration with `disabled_at` or `failure_count` above zero after `Drain`; fewer than
  `MinSamples` samples. Messages name counts, message and registration IDs, never tokens.
- `extra`: `push_handoff.params` (the constants and plan counts), achieved message and mention
  rates, expected, received and matched pushes, p50, p99 and maximum in ms, the largest and the
  mean lag of the consumer `push` (`max(event_log.seq)` minus its `last_seq`, sampled every
  `LagSampleEvery`), `GOMAXPROCS`, the clock offset and the note on shared CPUs. A large lag
  points at the consumer round, a small lag with high values at the job and the adapter.

## `database.max_conns`

`load_messages` records the pool occupancy under the sustained load. The default stays until a run
in mode `match` or `emulated` shows saturation; see "Capacity and
resources" (chapter 16, `docs/spec/16-operations.md`).
