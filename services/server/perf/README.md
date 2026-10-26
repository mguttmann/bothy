# Load tests of the server core (`services/server/perf`)

Status: built (M1-16 `event_commit`, M2-36 `invocation` and `run_patch`, M3-24 `push_handoff`, M3-16 `search`, M7-18 `capacity`, M1-72 everything else). Component `testsupport` (M1-05),
never imported by production code, left out of the Go coverage total.

`make perf` runs these tests and the chaos tests of `test/chaos` with the build tag `perf`, one
package after another (`go test -p 1`, so no two measurements share the machine), then
`perf/internal/report/perfreport` prints the table and decides the exit code. It is not part of
`make check` (plan P-07); `make check` only compiles, vets and lints the tagged code. The
thresholds are the rows with place `Load test` of the tables "Configuration values of this chapter"
in chapter 02 (`perf.*`) and chapter 14 (`search.*`), each found by its title (`report.LoadAll`); no
test holds a threshold as a literal.

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
 "results": [{"name", "value", "unit", "threshold", "comparison", "pass", "samples", "test", "note"}],
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
| `invocation` | `TestPerf_InvocationDeliveryP99` | `perf.invocation_delivery_p99` under `perf.sustained_messages_per_s` | the constants of `perf/internal/agentpath/constants.go` (section `invocation` and `run_patch`) |
| `run_patch` | `TestPerf_RunPatchFanoutP99` | `perf.run_patch_fanout_p99` under `perf.sustained_messages_per_s` | the same constants |
| `push_handoff` | `TestPerf_PushHandoffP95` | `perf.push_handoff_p95` under `perf.sustained_messages_per_s` | `BudgetShare`, `SendersPerChannel`, `RecipientsPerChannel`, `MentionEvery`, `MentionsPerMessage`, `Warmup`, `Window`, `Drain`, `MinSamples`, `LagSampleEvery` (section `push_handoff`) |
| `search` | `TestPerf_Search` | `search.p95_target` at `search.benchmark_messages`, `search.prefix_p95_target` | the constants of `perf/internal/searchbench/constants.go` (section `search`) |
| `capacity` | `TestPerf_CapacitySteadyThroughput` | `perf.capacity_min_messages_per_s` at `perf.capacity_steady_messages_per_s` | the constants of `test/load/params.go` (`GroupSize`, `Headroom`, `Warmup`, `SteadyWindow`) |
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
- `capacity`: the capacity gate (M7-18): the scenario of the capacity load test
  ([tests/load/README.md](../../../tests/load/README.md)) as one steady step: one space with one
  text channel per `GroupSize` people, each sender within `Headroom` of
  `ratelimit.rl.message.per_minute` and connected to the gateway, open-loop arrivals at
  `perf.capacity_steady_messages_per_s` for `Warmup` plus `SteadyWindow` (the short texts of
  `perfkit.Send`, no fan-out observer). The value is the goodput:
  the sends of the window answered `201` within `perf.message_create_p99`, per second
  (`load.SummarizeSends`); it must reach `perf.capacity_min_messages_per_s`. A gateway connection
  that ends during the run fails the target. `extra`: achieved rate, failed sends, p99.

## `invocation` and `run_patch`

`TestPerf_InvocationDeliveryP99` and `TestPerf_RunPatchFanoutP99` (M2-36) measure the two targets
of the agent path against a server process (`harness.StartProcess`, durable database, the limits of
the profile mode) that runs with the default configuration after a seeding process with raised
rate limits. The measuring point on the adapter side is the Go fake adapter of
`perf/internal/fakeadapter` ([09-agents.md](../../../docs/spec/09-agents.md) section 16.1 "Fake
adapter (load)"), one per agent: consumer `perf` at the gateway (`wstest`, the only measured
transport), cumulative `ack` (right after an invocation passed its token check, at half of
`agent.max_inflight_events`, or after `agent.ack_persist_interval`, so the window never fills),
every token checked with `runstest.ReferenceVerifier` against `getJwks`, `getRunContext`,
`startRun` with `organization` and `chat`, heartbeats through `sendHeartbeat` every
`HeartbeatAck.interval_ms` with `details.active_runs`. It has no runtime and no model.

- World (`perf/internal/agentworld`, design note 5 of the brief): one space; per agent
  `ChannelsPerAgent` text channels with the default switches (`require_mention` and `auto_thread`
  on) and the agent as operator; `ThreadsPerChannel` background threads per channel, created
  before the operators are set, so the seeding starts no run; trigger persons with `trigger`,
  background senders with `write` and viewers with `read` on the space, one device each; handles
  `agent-01`, `trigger-001`, `sender-001`, `viewer-001` at `example.com`.
- Load: open-loop human messages (`perfkit.Schedule`) at `perf.sustained_messages_per_s` with the
  default `ratelimit.*`. Background messages go round robin from the senders into the background
  threads: the operator is a candidate there, but a sender lacks `trigger`, so nothing triggers,
  while every message still reaches the agents' consumers (RK-19). The viewers are connected in both
  tests and read every frame.
- Plan (`agentpath.NewPlan`, from the defaults of the keys through the server schema, 02 for
  `perf.sustained_messages_per_s`, 04 for `agent.run_update_min_interval` and 09 for the adapter
  keys of the verifier): trigger rate = rate / `TriggerEvery`; agents A = the smallest count with
  (trigger rate / A) x `CallsPerRun` + 1 / `sync.heartbeat_interval` <= `BudgetShare` x
  `ratelimit.rl.agent_runtime.per_minute` / 60; patch runs per agent R = the largest count with
  R / `agent.run_update_min_interval` + 1 / `sync.heartbeat_interval` within the same budget;
  trigger persons = ceil(trigger rate / (`BudgetShare` x min(`rl.message`, `rl.trigger`) per
  second)); senders = ceil(rate / (`BudgetShare` x `rl.message` per second)); viewers = A x R x
  `ViewersPerRun`. `results.json` holds the counts under `invocation.params` and `run_patch.params`.
- Parameters: `TriggerEvery` 10, `BudgetShare` 0.5, `ChannelsPerAgent` 2, `ThreadsPerChannel` 20,
  `ViewersPerRun` 4, `StepEvery` 4, `CallsPerRun` 4, `Warmup` 30 s, `Window` 3 min, `Drain` 30 s,
  `MinSamples` 1000, `ConsumerName` `perf`.
- `invocation`: every `TriggerEvery`-th scheduled message is a trigger: `createMessage` into the
  main thread of a channel, round robin over trigger persons and channels, mentioning the operator
  by handle; the answer must be `201` with `trigger_state` `received`; with `auto_thread` each
  trigger opens a new thread and run. The rest of the rate are background messages. The adapters
  check, acknowledge, start and finish every run (`completed`). Sample: the receive time of the
  `agent.invocation` frame at the adapter (taken as soon as `wstest` hands the frame over, before
  the adapter decodes it) minus `occurred_at` of its envelope (the transaction of the triggering
  message, at or before the commit, so an upper bound), corrected by the clock offset between
  database and host (`report.ClockOffset`); one sample per trigger scheduled in the window, matched
  by `message_id`; the value is the nearest-rank p99.
- `run_patch`: A x R setup triggers, evenly spaced at the trigger rate, each agent R of them; the
  adapters start the runs; the test waits until every run is `running`, then each viewer focuses
  its run's thread and waits for the snapshot that focusing sends. Then the background load at the
  full rate starts with the patch schedule: per run one `run.update` every
  `agent.run_update_min_interval`, the runs of an adapter phase-shifted, `base_version` from the
  `startRun` answer rising by one per patch; ops `replace /status_line`, a growing
  `/last_text_excerpt`, `/metrics/tokens_out`, and every `StepEvery`-th patch a step that the next
  patch completes. Sample, per version and viewer: from the scheduled send time of the patch that
  produced the version (a lagging writer keeps it) to the receipt of the first `run.updated` that
  lifts the viewer's version to it or above (client rule of 04 section 11.3 point 4), both on the
  test process clock. After `Drain` the test reads `getRun` for every run, then finishes the runs.
- Failure conditions besides the threshold: an answer other than `201` to a human message (among
  them `429 rate_limited`) or a trigger without `trigger_state` `received`; a trigger without
  exactly one `agent.invocation`; an invocation for an unknown message; a `redelivered` envelope; a
  token the reference verifier rejects; any other status of the adapters' calls; any `error` or
  close frame on an adapter or viewer connection; a run that does not end `completed`; a gap or a
  late snapshot at a viewer; a viewer whose version differs from `getRun` after `Drain`; a version
  of the window that did not reach every viewer; fewer than `MinSamples` samples. Messages name
  counts, run and message IDs, never tokens.
- `extra`: `invocation.*`: achieved trigger and background rates, invocations received, p50 and
  maximum, the p99 of the `startRun` and `finishRun` round trips, planned and highest observed
  `rl.agent_runtime` calls per agent and second, the clock offset; `run_patch.*`: achieved patches
  and background messages per second, viewer connections, `run.updated` frames per version and
  viewer (below one when coalescing within `sync.run_patch_coalesce_interval` joins versions), late
  snapshots and gaps, p50 and maximum, planned and highest observed calls per agent and second.
  A high invocation value with a low `finishRun` round trip points at the delivery path (log tail,
  consumer runner, token); a high patch value with few frames per version at the coalescer.

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

## `search`

`TestPerf_Search` (M3-16, [14-search.md](../../../docs/spec/14-search.md) section 7) measures the p95
of `GET /api/v1/search` (type `messages`) against `search.p95_target` on a corpus of exactly
`search.benchmark_messages` messages, and the p95 of `GET /api/v1/search/thread-titles` against
`search.prefix_p95_target` on the same world. `make perf` runs it with every other package;
`make bench-search` runs it alone and prints the table of its two targets.

- Measuring point: from the scheduled send time to the fully read response body over loopback in
  the test process, an upper bound of the server time without coordinated omission; the client's
  DPoP proof lies inside the interval. The value is the nearest-rank p95 of the window.
- Sequence: a seeding process (`harness.StartProcess` on a durable database, every `ratelimit.*`
  raised) builds the world through the public API (`searchworld.Seed`) and stops; the bulk loader
  (`searchbench.Load`) writes the posts, the messages and the counters of every thread through the
  admin pool and runs `VACUUM (ANALYZE)`; the process starts again on the same database with the
  default configuration and the limits of the profile mode; `searchworld.CheckThreads` compares
  `ConsistencySamples` threads with `getThread` and `listThreadMessages`; the phases `messages`,
  `prefix` and `probe` run one after the other (both endpoints share `rl.search`);
  `searchworld.CheckWrites` writes one message and one post per language and compares
  `thread_seq` and `search_language` with the loader; the process stops; phase `rewrite` runs.
  Before each phase every person renews its token pair, so no sample pays a renewal.
- World: `Spaces` spaces, each with `TextChannelsPerSpace` text and `ForumChannelsPerSpace` forum
  channels, `ForumPostsPerChannel` posts per forum; `Authors` people (first half `de`, second half
  `en`) with `write` on every space; `Searchers` people (`de` and `en` alternating), searcher k with
  `read` on the spaces k mod `Spaces` and (k + `Spaces`/2) mod `Spaces`, so each sees a quarter of
  the channels. The last space is the probe space. Handles `space-01`, `text-01-1`, `forum-01-1`,
  `author-01`, `searcher-01`, addresses at `example.com`.
- Corpus (pure functions of `Seed` and an index): per language about forty function words and
  `ContentWords` generated stems of four to ten letters (German ones with a, o, u with diaeresis
  and the sharp s), inflected per occurrence; a word is a function word with probability
  `FunctionWordShare`, else a stem drawn by Zipf with exponent `ZipfS`. Messages have
  `MessageWordsMin` to `MessageWordsMax` words in the author's language, titles `TitleWordsMin` to
  `TitleWordsMax` content words; message i is created at load time minus `CorpusSpan` plus
  (i + 1) x `CorpusSpan` / N; the first messages open the posts; `DeletedShare` of the others are
  deleted; in the probe space `ProbeTermShare` of the messages carry the probe term, and the absent
  term occurs nowhere.
- Load: every searcher sends open loop at `BudgetShare` of `ratelimit.rl.search.per_minute`, the
  searchers phase-shifted evenly (`perfkit.Schedule`); `Warmup` without samples, `Window` with
  samples, then up to `Drain` for open requests; no background writes.
- Queries of phase `messages`: `GET /search?q=` with the defaults of `openapi.yaml`; German or
  English with equal chance; one word (`ShareOneWord`), two words (`ShareTwoWords`), three words
  (`ShareThreeWords`), a phrase of two adjacent words of a visible live message (`SharePhrase`) or
  `a -b` (`ShareNegation`); the words of a query come from one band chosen by share: frequent (ranks
  1 to `BandFrequentMax`, `ShareFrequent`), medium (up to `BandMediumMax`, `ShareMedium`), rare
  (`ShareRare`). Phase `prefix`: one word (`SharePrefixOneWord`) or two of the title of a visible
  post, each cut to between `search.prefix_min_chars` and its full length.
- Phase `probe`: the searchers without the probe space send the probe term and the absent term
  alternating at the same rate, `ProbeWarmup` of each unmeasured, then `ProbeQueries` of each
  measured. No threshold (14 4.3 accepts the residual risk); RK-10 is rated on the values.
- Phase `rewrite`: after the process stopped, the sizes, then in one transaction `DROP COLUMN
  search_vector` and `ADD COLUMN ... GENERATED ALWAYS AS (<expression from the catalog>) STORED`,
  then each saved index definition without `CONCURRENTLY`; expression, index definitions and the
  messages of a fixed phrase must be unchanged afterwards.
- Parameters: `Seed` 1; `Spaces` 8, `TextChannelsPerSpace` 4, `ForumChannelsPerSpace` 4,
  `ForumPostsPerChannel` 600, `Authors` 32, `Searchers` 24; `ContentWords` 50000, `ZipfS` 1.07,
  `FunctionWordShare` 0.4, `MessageWordsMin` 3, `MessageWordsMax` 40, `TitleWordsMin` 2,
  `TitleWordsMax` 6, `CorpusSpan` 365 days, `DeletedShare` 0.01, `ProbeTermShare` 0.2,
  `LoadBatch` 100000, `ConsistencySamples` 50; `BudgetShare` 0.5, `Warmup` 30 s, `Window` 3 min,
  `Drain` 30 s, `MinSamples` 1000, `ProbeWarmup` 20, `ProbeQueries` 200; `ShareOneWord` 0.40,
  `ShareTwoWords` 0.25, `ShareThreeWords` 0.15, `SharePhrase` 0.10, `ShareNegation` 0.10,
  `BandFrequentMax` 100, `BandMediumMax` 5000, `ShareFrequent` 0.2, `ShareMedium` 0.5,
  `ShareRare` 0.3, `SharePrefixOneWord` 0.7. `searchbench.NewPlan` fails before any work when the
  searchers' rate times `Window` stays below `MinSamples`. PostgreSQL runs with the defaults of its
  image and the durability of `pgtest.Durable()`, without tuning.
- Failure conditions besides the thresholds: a status other than `200` (among them `429 rate_limited` and `503 unavailable`
  with `reason` `search_timeout`); a body that does not decode strictly into `SearchHitPage` or
  `ThreadTitleHitList`; a hit of another kind than `message` or from a channel the searcher may
  not read; a phrase or prefix query without a hit; a probe answer with an item, with
  `candidates_capped` or with `next_cursor`; a failed load, product or rewrite check; a corpus
  whose count differs from `search.benchmark_messages`; fewer than `MinSamples` samples in a phase.
  Any of them fails both targets. Messages name phase, class, query, counts and ids, never content.
- `extra.search`: `corpus` (counts, visible share, planned rates), `load` (rows and the durations of
  `COPY` and `VACUUM (ANALYZE)`), `phase_messages` and `phase_prefix` (p50, p95, p99 and maximum,
  per class and per band, the share of answers with `candidates_capped`, planned and achieved
  rates), `probe` (p50 and p95 of both terms and their differences), `sizes` (bytes and bytes per
  million messages of the table, the indexes and the database, and the share of the search
  indexes), `rewrite` (`rewrite_seconds`, `rewrite_seconds_per_million`, `index_build_seconds`,
  `index_build_seconds_per_million`, the commit and `maintenance_work_mem`), `postgres`
  (`shared_buffers`, `work_mem`, `maintenance_work_mem`), `defaults` (the keys the plan read),
  `params` and `steps` (the duration of each step). A slow class or band points at the query to
  examine with `EXPLAIN (ANALYZE, BUFFERS)`; the upgrade guide (section 4) turns the rewrite values
  into the maintenance window of a release that changes a search vector expression.

## `database.max_conns`

`load_messages` records the pool occupancy under the sustained load. The default stays until a run
in mode `match` or `emulated` shows saturation; see "Capacity and
resources" (chapter 16, `docs/spec/16-operations.md`).
