# tests/load

Status: built (M7-18). The capacity load test of
[docs/spec/16-operations.md](../../docs/spec/16-operations.md) section 13: it installs the full
stack from the images by the installation path of the portability test (16 11.1, M7-14), ramps
people, gateway connections and the message rate in steps, measures every row of the capacity
table and writes a report. The capacity gate of `make perf` holds the sustained throughput as the
named value `perf.capacity_min_messages_per_s`.

## Files

| Path | Content |
|---|---|
| [`../../services/server/test/load`](../../services/server/test/load/doc.go) | The Go package of the scenario: the seeded generator of people and texts, the plan of a step, the driver through the public API and the gateway, the ramp and the result file. A black box of the server like `test/smoke` (`TestBlackBoxImports`). |
| [`../../services/server/test/load/cmd/bothy-load`](../../services/server/test/load/cmd/bothy-load/main.go) | The command `bothy-load ramp`: seeds, waits for the runner, runs the ramp, writes `result.json`. |
| [`../../tools/load/capacity_run.py`](../../tools/load/capacity_run.py) | The runner behind `make load-capacity`: installation, profile, driver, server metrics, report. |
| [`../../services/server/perf/capacity`](../../services/server/perf/capacity/doc.go) | The capacity gate of `make perf` (`TestPerf_CapacitySteadyThroughput`). |

The runner lives in `tools/load`, because the Python tooling of the repository (ruff, mypy,
bandit, pytest with coverage) runs there, as for the portability test (D-M7-14-6); the Go code
lives in the server module, because it uses the generated client `api/rest`.

## Scenario

Every value below is a named constant of `services/server/test/load/params.go` (the one place);
the result file repeats them.

- **People.** Sam, Robin, then the numbered test people `person-<n>` with
  `person-<n>@example.com`; the owner Alex comes from the setup wizard. Every person has a local
  account, its own device with its own DPoP key and its own client address (a `/64` of the
  documentation range `2001:db8::/32`), as distinct clients behind the reverse proxy of a
  deployment.
- **Groups.** `GroupSize` people share one space with one text channel and write into its main
  thread, so every message reaches the connected members of its group: the fan-out per message.
  The first `ObserversPerGroup` member of every group decodes its dispatches and measures commit
  to dispatch (`occurred_at` of the event to the receipt; the same clock, the driver runs on the
  host of the stack); the other connections read and drop their frames.
- **Load.** Open loop: message `k` of a step is due at `start + k / rate` and is sent in its own
  goroutine, so a slow answer never delays the next one; the latency runs from the due time to
  the answer (no coordinated omission). Each sender keeps within `Headroom` of the default of
  `ratelimit.rl.message.per_minute`, so a step at rate `r` has `r / (Headroom x per_minute / 60)`
  senders, each with one gateway connection. Texts are `MinContentRunes` to `MaxContentRunes`
  runes of plain words from a fixed vocabulary, derived from `Seed`, the step and `k`: no
  mention, link or markup.
- **Steps.** `RampRates` in rising order; each step renews the tokens of its senders, connects
  them, sends for `Warmup` plus `Window` and waits up to `Drain` for the dispatches. Only the sends
  due in the window count.
- **Stop.** A step misses when the p99 of `createMessage` exceeds `perf.message_create_p99`, the
  p99 of commit to dispatch exceeds `perf.fanout_local_p99` (02 section 17), any send fails (a
  `429` would mean the plan broke a rate limit), an observer misses a dispatch or a connection
  fails. The ramp stops at the first missed step; that step and the one before are the result.
  The sustained throughput is the goodput of the last step that held: the sends of its window
  answered `201` within `perf.message_create_p99`, per second.
- **Seeding.** The owner makes two administrative calls per person, so the runner starts the
  server with raised rate limits for the seeding and restarts it with its defaults before the
  ramp.

## Run on the full stack

```sh
# the trial release of the product on the build host (D-W-127), served until stopped
uv run --project tools python tools/portability/portability_trial.py --cosign <path> --syft <path> --hold
# with the PORTABILITY_* variables it prints:
make load-capacity [LOAD_REPORT_DIR=.cache/load]
```

The interface is the one of `make portability` (`PORTABILITY_TAG`, `PORTABILITY_INSTALL_SOURCE`,
`PORTABILITY_RELEASE_DIR`, `GITHUB_REPOSITORY`, `PORTABILITY_TRIAL_PUBLIC_KEY`); on a
GitHub-hosted runner a published tag with the source `url` works the same way (no cloud load
generator, D-W-125). Needs Docker Engine 26 with Compose 2.26, `uv` and `go`.

The runner, in this order:

1. reads `perf.reference_profile`, `perf.message_create_p99`, `perf.fanout_local_p99` (02
   section 22) and the default of `ratelimit.rl.message.per_minute` (configuration reference);
2. builds `bothy-load` for Linux on the architecture of the Docker runtime, pulls and verifies the
   images, runs `install.sh` with `--storage fs` and the options of
   `tests/portability/install-options`, and starts the overlay `local` (Prometheus) with a metric
   export every 5 s;
3. decides the profile mode of ADR 0063 from the vCPUs and memory of the Docker runtime and gives
   `api` and `worker` `GOMAXPROCS` and `cpus` of the profile, PostgreSQL `cpus` and `mem_limit`
   (a Compose override; the installation's files stay as `install.sh` wrote them). `api` trusts
   the forwarding headers of the network of its published port, as behind a reverse proxy;
4. runs `bothy-load ramp` in a container in the network of the Docker host next to the stack:
   seeding with raised rate limits, the restart with the defaults, the ramp;
5. per step reads from Prometheus the counter lock wait (`bothy.db.event_seq.lock_wait`, p99 and
   mean), the events per second (`bothy.event.appended`), the database backends
   (`postgresql.backends`) and the acquired pool connections, the gateway connections
   (`bothy.gateway.connections.active`) and the fan-out lag (`bothy.event.fanout.lag`), and from
   `docker stats` the memory of `api`, `worker`, `postgres` and `clamav`; at the end the size of
   every table with its indexes, the mean message row, the mean event payload and the index share
   of the database;
6. writes `capacity-report.json` (the report and the driver's result) and `capacity-report.md`
   into `LOAD_REPORT_DIR`, then removes the installation with its volumes and the temporary
   directory, also after a failure (after printing `docker compose ps --all` and the logs).

`LOAD_RATES` (comma-separated rising rates, for example `20,40`) runs a shorter ramp by hand to
try the path; the measurement runs the ramp of `params.go`.

Exit codes: 0 the report was written (whatever the ramp measured), 1 a step failed, 2 an invalid
interface value or a missing precondition.

## The capacity gate

`TestPerf_CapacitySteadyThroughput` in `make perf` runs the groups, the headroom and the gateway
connections of the scenario (every sender connected, no observer, the short texts of
`perfkit.Send` instead of the generator's) as one steady step of `load.Warmup` plus `load.SteadyWindow` at
`perf.capacity_steady_messages_per_s` against the server process of the perf harness, in the
profile mode of the other load tests. Its goodput must reach `perf.capacity_min_messages_per_s`;
below it `make perf` ends not green. Both values and the margin between them are rulings of
`docs/decisions.d/M7-18.md`.

## Tests

```sh
cd services/server && go test -race ./test/load/...        # generator, plan, parser, fan-out
cd tools && uv run pytest load                              # the runner and the report
cd services/server && go test ./perf/internal/report/        # the gate below and above the minimum
```

## Not proven here

- Hosts larger than the reference profile, several api nodes (M7-17) and a database on its own
  host.
- Production data shapes: long threads, attachments, mentions, agents and search share the
  database with the messages of this scenario only in their sizes, not in their load.
