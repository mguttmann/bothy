# Load tests of the server core (`services/server/perf`)

Status: built (M1-16 `event_commit`, M1-72 everything else). Component `testsupport` (M1-05),
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

## `database.max_conns`

`load_messages` records the pool occupancy under the sustained load. The default stays until a run
in mode `match` or `emulated` shows saturation; see "Capacity and
resources" (chapter 16, `docs/spec/16-operations.md`).
