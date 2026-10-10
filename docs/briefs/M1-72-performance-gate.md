# M1-72: Load tests and performance gate of the server core (ADR 0063)

Status: ready | Depends on: M1-57, M1-65 | Etappe: M1

## Goal

After this ticket `make perf` measures every performance target of the server core on a recorded
host profile, compares each value with its named threshold from 02 "Leistungsziele", writes value
and threshold per target and fails when one is missed. Chaos tests prove that losing the `LISTEN`
connection, restarting the database during the log tail and killing the server during commits leave
no gap and no partial change. This closes exit criterion 7 of M1 and turns the capacity start
values of 16 into measured facts.

## Read first

- `docs/spec/02-architektur.md`: "Ereignis-Pfad" ("Log-Tail je api-Prozess", "Hub und Zustellung"),
  "Start und Graceful Shutdown" ("Health"), "Skalierungspfad", "Leistungsziele", "Offen",
  "Konfigurationswerte dieses Kapitels" (all `perf.*` rows; "Leistungsziele (`perf.*`) sind
  Schwellen für Tests, keine Laufzeit-Konfiguration").
- `docs/spec/04-sync-protokoll.md`: "Globale, commit-geordnete Sequenz" ("Muster mit
  Zähler-Zeile"), "Lebenszyklus", "Nachholen (Catch-up)", "Zustellgarantie", "Reihenfolge".
- `docs/spec/03-datenmodell.md`: "Transaktionsregeln", tables `event_log`, `sequence_counters`,
  `messages`, `threads`, `idempotency_keys`, `audit_events`.
- `docs/spec/07-rechte.md`: "Abfrage-Muster" rule 5 (large visible sets).
- `docs/spec/16-betrieb.md`: "Zwei Listener", "Datenbank" ("Rollen"), "Kapazität und Ressourcen",
  "Konfigurationswerte dieses Kapitels" (`database.max_conns`, start value until the load test).
- `docs/observability/signals.md`: `bothy.delivery.duration`, `bothy.db.event_seq.lock_wait`,
  `bothy.db.pool.connections`.
- ADRs 0009 (Confirmation), 0004; ADR 0063 "Leistungs-Gate für das Ereignis-Log" if M1-16 wrote it.
- `docs/PLAN.md`: "Arbeitsweise je Ticket" item 6, "Abschluss einer Etappe" item 3, M1
  "Exit-Kriterien" item 7, rows M1-16, M1-28 and M1-72, decisions P-07, P-09, P-10, P-12.
- Operations: `createMessage`, `listEvents`, `getSnapshot`, `verifyAuditChain`, `createSpace`,
  `createChannel`, `createThread`, `createGrant`, `createUser`, `createEnrollmentCode`,
  `completeEnrollment`, `refreshToken`; gateway ops `hello`, `identify`, `ready`, `heartbeat`,
  `heartbeat_ack`, `dispatch`, `reauth`.
- Code: `perf/event_commit` and the `perf` target from M1-16, `BenchmarkAuthorizeCached` from
  M1-28, `internal/platform/pg/pgtest` (M1-12), `internal/sync` log tail and `LISTEN` (M1-16),
  `cmd/bothy` (`serve`, `migrate`, `setup`, `secrets`).

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a
   CHANGELOG entry per release, docs site kept current. A change without matching documentation is
   not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every unit test is written and seen failing before the code that makes it pass.
7. No en dash and no em dash in any file; white label.
8. Configuration only through named keys from `docs/operations/configuration.md`; thresholds only
   by their `perf.*` names, never as literals in code or docs.
9. 02 "Leistungsziele": `make perf` measures `perf.message_create_p99` under
   `perf.sustained_messages_per_s`, `perf.fanout_local_p99`, `perf.catchup_page_p99`,
   `perf.authorize_cached_p99`, `perf.ws_connections_per_node`, `perf.ws_idle_conn_mem_max`,
   `perf.idle_rss_max`, `perf.startup_ready_max` and keeps `perf.event_commit_min_per_s` from M1-16.
10. P-07: `make perf` runs at the etappe close and the release, not in the pre-push gate; values
    are valid on `perf.reference_profile` (ADR 0063).
11. 04 "Muster mit Zähler-Zeile" and "Zustellgarantie": no gap in `seq`, no lost or duplicated
    delivery after deduplication, order equals commit order.
12. PLAN "Arbeitsweise je Ticket" item 6: no ticket lowers a threshold.

## Scope

1. `services/server/perf/` (build constraint `//go:build perf`, or the tag M1-16 chose; one tag for
   all perf and chaos code):
   - `load_messages/`, `gateway_fanout/`, `catchup/`, `ws_soak/`, `startup/`: one test package each.
   - `internal/report/` (untagged, unit tested in `make test`): threshold loader, units, percentile,
     profile detection, results writer. Move M1-16's own reporting of `perf/event_commit` onto it
     so there is one results file and one table.
   - `README.md`: what each test measures, how, its parameters (named constants), how to read the
     results, profile modes.
2. `services/server/test/chaos/` (same tag): `listen_loss_test.go`, `db_restart_test.go`,
   `sigkill_commit_test.go`, `invariants.go`, `README.md`.
3. `BenchmarkAuthorizeCached` (M1-28): if it does not report a p99, extend it with a minimal change
   in its test file to record per-operation durations and `b.ReportMetric(p99, "p99-ns")`.
4. `internal/platform/pg/pgtest`: additive options for perf and chaos: resource limits for the
   container (`--cpus`, `--memory`) and a restart that keeps the host port (pick a free loopback
   port first, publish it explicitly as `127.0.0.1:<port>:5432`, retry on a bind conflict).
5. Shared test harness `services/server/test/harness` (create or extend, text below); this ticket
   adds `StartProcess`.
6. `Makefile` target `perf` (keep M1-01's frame): builds nothing into the repository; runs
   `cd services/server && go test -tags perf -count=1 -timeout $(PERF_TIMEOUT) ./perf/... ./test/chaos/...`
   without `-race`; prints the result table; exits non-zero when any target fails or any chaos test
   fails. `make check` compiles and vets the tagged code (`go vet -tags perf ./perf/...
   ./test/chaos/...` in `lint`, golangci-lint with the tag) so it cannot rot between etappen.
7. ADR 0063: if `docs/adr/0063-*.md` exists, extend its "Confirmation" and "More Information" with
   the M1-72 targets and chaos tests without changing the decision; otherwise write it (P-12) as
   `docs/adr/0063-leistungs-gate-fuer-das-ereignis-log.md` (Design notes) and run `make adr-index`.
8. Documentation: `docs/quality/gates.md` (a section "Nicht in `make check`" with `perf` and
   `portability`), `docs/spec/16-betrieb.md` "Kapazität und Ressourcen" and the `database.max_conns`
   row (Design notes), `sonar-project.properties` (`services/server/perf/**` in
   `sonar.test.inclusions`), `services/server/README.md`, `services/server/CHANGELOG.md` under
   "Unreleased".

### Shared test harness (identical text in the briefs M1-69 to M1-72)

If `services/server/test/harness` exists on the base branch, use it and extend it only additively.
If it does not exist, first search for an existing full-server test helper
(`grep -rn "pgtest" services/server/internal/wire services/server/test`); wrap it if it builds the
real `internal/wire` graph, otherwise create the package with exactly this interface. Parallel
tickets build the same interface, so merges converge.

- Package `harness`, non-test Go files, architecture component `testsupport` (Design notes).
- `func Start(tb testing.TB, opts Options) *Server`: the real `internal/wire` graph with role `all`
  in this process; PostgreSQL 18 from `internal/platform/pg/pgtest`; migrations as
  `bothy migrate up --bootstrap` runs them; secrets created as `bothy secrets init` creates them, in
  `tb.TempDir()`; public and internal listener on `127.0.0.1:0`; the first owner created through the
  same application service that `bothy setup bootstrap-owner` calls (08 "Erstinstallation"), then
  enrolled with its enrollment code and fresh ES256 device keys (08 "Einladungs-Code",
  "Registrierung"); everything stopped in `tb.Cleanup` in reverse order.
- `type Options struct { Config map[string]string; LogWriter io.Writer; Telemetry *TelemetrySinks;
  EgressDenyAll bool; Clock clock.Clock }`. `Config` takes dotted keys from
  `docs/operations/configuration.md` and is applied through `platform/config` with flag precedence,
  so an unknown key still fails.
- `type Server struct { PublicURL, APIBase, GatewayURL, InternalURL, SecretsDir string; Owner *Client }`
  with `APIBase = PublicURL + "/api/v1"` and `func (s *Server) AdminDB() *pgxpool.Pool` (superuser
  of the disposable container; assertions only).
- `func (s *Server) NewUser(tb, UserSpec{Handle, DisplayName, Email string}) *Client` and
  `func (s *Server) NewAgent(tb, AgentSpec{Handle, DisplayName, BillingMode, CredentialHolder string}) *Client`,
  only through the public API (`createUser`, `createEnrollmentCode`, `completeEnrollment`;
  `createAgent`, `createToken`), owner step-up via `createStepUpChallenge` and
  `completeStepUpWithDeviceKey`.
- `type Client struct { PrincipalID, DeviceID string; ... }` with `Do(ctx, Request) Response`
  (DPoP proof per RFC 9449 with the latest `DPoP-Nonce`, one retry on `use_dpop_nonce`, refresh on
  `token_expired`, `Idempotency-Key` UUIDv7 on POST and PATCH except the list in 06 "Idempotenz",
  optional `Observer func(Request, Response)`), `StepUp(ctx)`, `Gateway(ctx) *GatewayConn`
  (`hello`, `identify`, `ready`, `Next`, `Send`, `Close`, automatic `heartbeat` and `reauth`).
- `type Response struct { Status int; Header http.Header; Body []byte; Elapsed time.Duration }`.
- `func BuildMatrixWorld(tb, s *Server) *World`: the M1 subset of 07 "Beispiele" and
  "Testmatrix": spaces Support, Marketing, Engineering, Research; channels Support/Allgemein,
  Support/Eskalation, Marketing/Kampagnen, Engineering/Projekt-A (forum, no project binding),
  Engineering/Werkzeuge, Research/Notizen; people Alex, Sam, Robin; agents Lotse (organization,
  operator of Support), Texter (organization, operator of Marketing), Runner (organization, operator
  of Engineering/Werkzeuge), Assistent (personal, credential holder the owner, operator of Research);
  group Leser with member Sam (membership with expiry); grants G-1 (Alex, `trigger`, `upload` and
  `approve_actions`, space Support), G-2 (Sam, `trigger`, `upload`, space Marketing), G-3 (Robin,
  `trigger`, `upload`, channel Engineering/Projekt-A, with expiry), G-4 (group Leser, `read`,
  Support/Allgemein), G-5 (Lotse, `write`, `upload`, space Support); one thread with messages per
  text channel and one forum post in Projekt-A. Left out until their etappe: app Tracker and G-6
  (M2), the DM Alex-Sam (V3), project binding and tool class `host` (M2). Expiry times are relative
  to the harness clock, never calendar dates.
- `func StartProcess(tb, ProcessOptions) *Process`: builds `./cmd/bothy` once per test binary into
  a temp directory; runs `bothy secrets init`, `bothy migrate up --bootstrap`, `bothy setup
  bootstrap-owner` against a pgtest container, then `bothy serve --role=all` with
  `server.listen_addr` and `server.internal_listen_addr` on free loopback ports picked just before
  start (retry on bind conflict); `ProcessOptions{Config map[string]string; Env []string;
  GOMAXPROCS int}`; `Process` offers `PID()`, `RSSBytes()` (Linux `/proc/<pid>/status` `VmRSS`,
  macOS `ps -o rss= -p <pid>`), `WaitReady(ctx)` (polls `/readyz` on the internal listener),
  `Signal(sig)`, `Restart(ctx)`, `APIBase`, `GatewayURL`; log output into a file in the temp
  directory; stopped by PID in `tb.Cleanup`.
- Unit tests for every part you add (at least: DPoP proof claims and the nonce retry; RSS parsing).

## Out of scope

- Targets of later etappen: `perf.fanout_cross_node_p99` (M7 multi-node),
  `perf.invocation_delivery_p99`, `perf.run_patch_fanout_p99` (M2), `perf.push_handoff_p95` (M3).
- Optimisations beyond what a missed M1 target requires.
- Changing any `perf.*` threshold or `perf.reference_profile`.
- The etappe-close run on the reference host itself (owner operation after merge).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-72-performance-gate`.
- Conventional Commits with scope (`test(perf): ...`, `test(chaos): ...`, `perf(<module>): ...`,
  `docs(adr): ...`) and DCO sign-off; description language as in the existing `git log`.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside this scope, except the docs and changelog entries named here, the
  shared harness, pgtest options, and fixes in M1 code needed to meet a target or to remove a chaos
  defect, each with a test and listed in the report.
- Follow the host rules supplied by the launcher. Stop only processes and containers you started,
  by PID or container ID. Temporary directories only via `mktemp` or `t.TempDir()`, removed at the
  end.
- Never read or print secrets. No network calls except package and image downloads for pinned tools;
  bind only `127.0.0.1`; PostgreSQL as a disposable container.
- Never lower a threshold, disable a gate or delete a test. A missed target on a host below the
  reference profile is reported as not proven, not "fixed" by changing numbers.

## Design notes

Thresholds and units (`perf/internal/report`):

1. Source: the rows `perf.*` of the table in section "Konfigurationswerte dieses Kapitels" of
   `docs/spec/02-architektur.md` (Ruling L4: chapter tables are the source; 02 says `perf.*` are not
   runtime configuration, so they are not read from the config schema). Find the section by its
   title, not its number; a missing row or an unparseable value fails the run.
2. Units: `us`, `ms`, `s` for durations, `KiB`, `MiB`, `GiB` for sizes, plain integers for counts
   and rates. Comparison by the meaning column: "Ziel (Mindestwert)" means value >= threshold,
   "Ziel (Höchstwert)" and plain "Ziel" mean value <= threshold.
3. Percentiles: nearest-rank over all samples of the measured window; warm-up samples excluded.
4. Results: `.cache/perf/results.json` at the repository root (directory already used for coverage),
   schema `{"profile":{"mode","cpus","memory_bytes","os","postgres_version","gomaxprocs"},
   "results":[{"name","value","unit","threshold","comparison","pass","samples","test"}],
   "extra":{...}}`, plus a table on standard output. `extra` holds supporting measurements: pool
   acquire wait p99, `bothy.db.event_seq.lock_wait` p99, database and index size per thousand
   messages, DB clock skew.
5. Profile: parse vCPU count and memory from `perf.reference_profile`. Mode `match` when the host
   has exactly that many logical CPUs and at least that memory; `emulated` when the host is larger:
   the server process gets `GOMAXPROCS` equal to the reference vCPU count and the PostgreSQL
   container `--cpus` and `--memory` from the profile; `below` when the host is smaller. Targets are
   enforced in every mode; the mode is printed first and stored. Which modes count for the etappe
   close is written in ADR 0063.

Measurements (parameters such as durations, warm-ups, sender and receiver counts are named
constants in each package, documented in `perf/README.md`, and copied into `results.json`):

1. Seeding at scale goes through the public API, never raw SQL: a seeding server
   (`harness.StartProcess` with every `ratelimit.*` raised) creates users, devices, spaces,
   channels, threads and grants, then stops; the measured server starts afterwards on the same
   database with default configuration. Reason: real flows, default limits during measurement.
2. `load_messages` (`perf.message_create_p99`): open-loop arrivals at
   `perf.sustained_messages_per_s` spread over enough senders that each stays within
   `ratelimit.rl.message.*`, into shared threads, while gateway receivers are connected. Latency of
   one send is measured from its scheduled send time (avoids coordinated omission) to the response
   on loopback, which bounds the server time from above. Any non-2xx response fails the test.
3. `gateway_fanout` (`perf.fanout_local_p99`): connected receivers with read on the channel, sender
   at the sustained rate. Latency per event and receiver = receive time minus envelope
   `occurred_at` (transaction time, at or before commit, so an upper bound), corrected by the
   measured clock offset between the database and the host (bracketed `SELECT clock_timestamp()`
   samples). Every receiver must get every event exactly once in `seq` order.
4. `catchup` (`perf.catchup_page_p99`): a log with many events across many channels; the principal
   sees a fraction. Measure `listEvents` with `limit` = `sync.catchup_page_size` per full page.
   Second variant: the principal sees a large number of channels (07 "Abfrage-Muster" rule 5); the
   same threshold holds.
5. `authorize_cached` (`perf.authorize_cached_p99`): run `BenchmarkAuthorizeCached` with
   `-run '^$' -bench '^BenchmarkAuthorizeCached$'` and read `p99-ns`.
6. `ws_soak` (`perf.ws_connections_per_node`, `perf.ws_idle_conn_mem_max`): server subprocess; open
   `perf.ws_connections_per_node` connections, each a distinct seeded device (one device holds one
   connection, 04 "Endpunkt und Version"), each through `identify` to `ready`; keep them idle with
   heartbeats for at least two `sync.heartbeat_interval`. Pass: all connected, no close frame, every
   heartbeat acknowledged; memory per connection = (RSS with all connections after settling minus RSS
   after ready) divided by the count. Check `RLIMIT_NOFILE` of the test process first and fail with
   a clear message if the hard limit is too low.
7. `startup` (`perf.startup_ready_max`, `perf.idle_rss_max`): on a migrated database, repeated starts
   of `bothy serve --role=all`; time from exec to the first `200` of `/readyz`; the maximum over the
   repetitions is compared; RSS after an idle settle period, maximum compared.
8. `event_commit` (`perf.event_commit_min_per_s`): M1-16's test, reported through the shared report.
9. `database.max_conns`: record pool acquire wait p99 and saturation during `load_messages`. If the
   default holds the targets without saturation, remove "(Startwert bis zum Lasttest)" from the 16
   row and point "Kapazität und Ressourcen" to `make perf` and `perf/README.md` (no numbers in the
   chapter). If another default is needed, change the 16 row and the config schema in this ticket
   with the measurement in the report.

Chaos tests (`test/chaos`, all under load from several writers and with gateway receivers):

1. `TestChaos_ListenConnectionLoss`: terminate the `LISTEN` backend with `pg_terminate_backend`
   (identify it by `application_name`; if M1-16 set none, set `bothy_event_listen` in
   `internal/sync` in this ticket). Receivers still get every event in order; delivery resumes within
   the poll and backoff bounds (`sync.log_poll_interval`, `database.listen_reconnect_backoff_cap`).
2. `TestChaos_DatabaseRestartDuringTail`: restart the pgtest container (stable port) while writers
   and the tail run. Writers retry failed sends with the same `client_msg_id` until success.
3. `TestChaos_SigkillDuringCommit`: server subprocess under write load, `SIGKILL` at random moments
   (repetitions as a named constant), restart, writers retry unacknowledged sends with the same
   `client_msg_id`.
4. Invariants after each chaos test (`invariants.go`, read-only queries through `AdminDB()`):
   `seq` in `event_log` is gapless and the `event_seq` counter equals its maximum; every message has
   exactly one `message.created` event and every such event has its message; per thread
   `thread_seq` is gapless and `threads.last_message_seq` equals its maximum; every acknowledged
   send exists exactly once; no `idempotency_keys` row is left `in_progress`; `verifyAuditChain`
   reports a valid chain; every receiver saw a gapless, duplicate-free (after `seq` dedupe) stream
   in `seq` order.

ADR 0063 (only if missing; MADR 4, headings English, text German, status `accepted`): context is the
global serialisation point of ADR 0009 and that numbers only mean something on a fixed profile.
Decision drivers: reproducibility, no flaky pre-push gate, one source per threshold. Options: (a)
separate `make perf` at etappe close and release with thresholds from 02 and a recorded profile
(chosen); (b) load tests inside `make check` (rejected: hardware-dependent, slow, flaky); (c)
relative benchmarks against a stored baseline (rejected: a stored baseline is a value that must be
re-derived with every hardware change, and the product promise is absolute); (d) an external
benchmarking service (rejected: external account and cost). Confirmation: `make perf` with every
target of M1 including `perf.event_commit_min_per_s`, the chaos tests, and the rule that the etappe
close needs mode `match` or `emulated`.

Architecture register and coverage (shared with M1-70 and M1-71): if M1-05's generator has no
component for `test/**` and `perf/**`, add `testsupport` with the go-arch-lint v3 keys
`anyProjectDeps: true` and `anyVendorDeps: true` (verify against the pinned version) and a generator
test that no production component may depend on it; exclude `services/server/test/**` and
`services/server/perf/**` from the Go coverage total.

## Tests (write them first)

Unit tests (untagged, in `make test`):

1. `perf/internal/report`: `TestThresholdsFromChapterTable` (all M1 names found in a fixture copy
   and in the real 02), `TestThresholdMissingFails`, `TestUnitParsing` (`100us`, `150ms`, `5s`,
   `64KiB`, `256MiB`, plain counts), `TestComparisonFromMeaning`, `TestNearestRankPercentile`,
   `TestProfileModes` (`match`, `emulated`, `below` from fixture hosts), `TestResultsJSONSchema`,
   `TestClockSkewCorrection`.
2. Harness: `TestRSSParsing` (Linux and macOS formats), `TestStartProcessPicksLoopbackPorts`.
3. pgtest: `TestRestartKeepsHostPort`.

Tagged tests (in `make perf`): `TestPerf_MessageCreateP99`, `TestPerf_FanoutLocalP99`,
`TestPerf_CatchupPageP99`, `TestPerf_CatchupPageP99LargeVisibleSet`, `TestPerf_AuthorizeCachedP99`,
`TestPerf_WSConnectionsPerNode` (also yields `perf.ws_idle_conn_mem_max`),
`TestPerf_StartupReadyAndIdleRSS`, M1-16's event commit test, `TestChaos_ListenConnectionLoss`,
`TestChaos_DatabaseRestartDuringTail`, `TestChaos_SigkillDuringCommit`.

Commands that must pass:

```sh
cd services/server && go test -race -count=1 ./perf/internal/... ./test/harness/... ./internal/platform/pg/...
cd services/server && go vet -tags perf ./perf/... ./test/chaos/...
make perf
make check
```

## Acceptance criteria

- [ ] `make perf` measures every target named in requirement 9, writes `.cache/perf/results.json`
      and the table with value, threshold, comparison and pass per target, and the profile mode.
- [ ] `make perf` exits non-zero when one target is missed (proved by a unit test of the report
      exit logic with a failing fixture).
- [ ] Thresholds come only from the 02 table; no `perf.*` value appears as a literal in code.
- [ ] The three chaos tests pass and check every invariant listed.
- [ ] `database.max_conns` is confirmed or changed with evidence; 16 updated accordingly.
- [ ] ADR 0063 exists and names the M1 targets and the profile rule; `make docs` passes.
- [ ] Tagged code compiles and passes vet and lint inside `make check`.
- [ ] On the build host every target passes; if the host mode is `below`, the report lists missed
      targets as not proven instead of claiming them.
- [ ] `make check` is green.

## Definition of done

- Code and tests as above; README in `services/server/perf/`, `services/server/test/chaos/` and the
  harness (if created); doc comment on every exported symbol.
- ADR 0063 written or extended (P-12).
- CHANGELOG entry under "Unreleased" in `services/server/CHANGELOG.md`.
- `make check` green and `make perf` run with its table in the report.
- Independent review and red team over the diff (`docs/quality/red-team-gate.md`) with verdict
  recorded in the report; merge locally into the base branch only if both are green, then
  `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits,
files with line counts, unit test counts, the `make perf` table with profile mode and host data, the
chaos results, the `extra` measurements (pool, lock wait, index share), fixes made to meet targets,
what is not proven (at least: values on the reference profile if the mode was not `match` or
`emulated`), follow-ups that could not be done in scope with reason. Then write `{{DONE_PATH}}` with
`exit=0 sha=<sha>` or `exit=1 <reason>`.
