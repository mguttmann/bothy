# Package `test/harness`

Status: built (M1-71, M1-72 added `StartProcess`, `DurableDB` and the gateway client, M1-69 the
telemetry sinks and the login helpers); M1-70 extends it additively. Component `testsupport`
(M1-05), never imported by production code.

The shared full-server test harness: the real `internal/wire` graph with role `all` in the test
process against a disposable PostgreSQL 18 from `internal/platform/pg/pgtest`.

## Interface

- `Main(m)`: call from `TestMain`; the pgtest templates carry the steps of `bothy migrate up`.
- `Start(tb, Options) *Server`: database as `bothy migrate up --bootstrap` leaves it (goose, the
  wire steps and River's tables), secrets as `bothy secrets init` creates them (`secrets/stack`
  with the passwords of the container as supplied source) in `tb.TempDir()`, the first owner through
  the setup service of `bothy setup bootstrap-owner`, enrolled with fresh ES256 device keys
  (`redeemEnrollmentCode`), both listeners on `127.0.0.1`, stopped in `tb.Cleanup`.
- `Options{Config, LogWriter, Telemetry, EgressDenyAll, Clock, DurableDB}`: `DurableDB` puts the
  database on the durable pgtest container (`pgtest.Durable()`, ADR 0063); every perf and chaos
  test sets it. `Config` takes dotted keys of
  `docs/operations/configuration.md` with the precedence of an environment variable, so an unknown
  key fails the start; every server waits `testkit.Patience` for a pool connection, a statement and
  a lock, keeps an idle transaction open and runs a request up to it (`testkit.PatientDatabase()`,
  docs/DECISIONS.md PL/37, PL/52, PL/56), so a loaded runner does not turn a
  slow request into 500 or 503, and a test of one of these deadlines sets it through `Config`
  (`StartProcess` keeps the production deadlines); `EgressDenyAll` makes every outgoing client refuse every target before name
  resolution; `RaisedRateLimits()` gives the entries that lift every rate-limit class
  (23 R-QA-11). `Telemetry` (`NewTelemetrySinks`, M1-69) replaces the OTLP exporters with an
  in-memory span exporter, a `MemoryLogExporter` and a `MetricSink` (a fresh manual metric reader
  for every server start, `Collect` reads the newest, ruling PL/50) and turns
  `telemetry.enabled` on unless `Config` sets it; `otel.Setup` installs global providers, so a test
  that sets it never runs in parallel with another harness start. `SecretFiles` writes extra files
  into the secrets directory (mode 0400; the client secret of a fake OIDC provider), `OIDCRootCAs`
  is the TLS root of the OIDC provider requests, `PublicHost` (`localhost`) gives the server a
  domain name as WebAuthn relying party, `ProofObserver` sees every DPoP proof a client sends and `TokenObserver` every token pair a client
  takes over and the API token of `NewAgent`. `APNsFake` (M3-24) points the wired APNs adapter at a Fake-APNs of
  `apnstest` for both environments, trusts its certificate and appends its rule to
  `egress.allowed_hosts`; the test still enables APNs through the keys `push.apns.*`
  (`TestStartWithAPNsFake`). `DBCPUs` and `DBMemoryBytes` limit the container of a `DurableDB`
  (`pgtest.Limits`, the emulated profile of `make perf`).
- `Server{PublicURL, APIBase, GatewayURL, InternalURL, SecretsDir, SigningKeyDir, Owner}`,
  `AdminDB()` (superuser pool, assertions only), `ConfigValue(key)`, `Clock()`,
  `FlushTelemetry(ctx)` (flushes the global tracer and logger providers into the sinks),
  `NewDevice()` (a client with fresh device keys and no tokens, for login, pairing and enrollment
  flows the test drives itself).
- `NewUser(tb, UserSpec)`, `NewAgent(tb, AgentSpec)` (public API only: `createUser` with
  `local_account`, `redeemEnrollmentCode`; `createAgent`, `setAgentBillingMode`, `createToken` with
  `dpop_jkt`), `OwnerSession(ctx)` (recovery code of the setup service), `UserSession(ctx, id)`
  (`createEnrollmentCode`).
- `Client.Do(ctx, Request) Response`: DPoP proof per RFC 9449 with the newest `DPoP-Nonce`, one
  retry on `use_dpop_nonce`, refresh on `token_expired`, a UUIDv7 `Idempotency-Key` on POST and
  PATCH except the operations with `x-idempotency-exempt`, optional `Observer`; `StepUp(ctx)`
  (`createStepUpChallenge`, `completeStepUpWithDeviceKey`); `MustDo` and `DoWithStepUp`;
  `DoAnonymous` (the DPoP proof of the client's key without the access token: refresh, logins),
  `Adopt(body)` (the token pair of a login answer) and `Registration(name)` (the
  `DeviceRegistration` of the client's keys).
- `StartProcess(tb, ProcessOptions{Config, Env, GOMAXPROCS, DBCPUs, DBMemoryBytes}) *Process`:
  builds `./cmd/bothy` once per test binary into a temp directory (removed by `Main`), runs
  `bothy secrets init`, `bothy migrate up --bootstrap` and `bothy setup bootstrap-owner` against a
  durable pgtest database (with `pgtest.Limits` when `DBCPUs` or `DBMemoryBytes` is set), then
  `bothy serve --role=all` with both listeners on free `127.0.0.1` ports picked just before the
  start (retried on a bind conflict) and the log in `LogPath`. `Process` embeds `*Server` (URLs,
  enrolled `Owner`, `NewUser`, `AdminDB`), and offers `DB`, `PID()`, `RSSBytes()` (Linux
  `/proc/<pid>/status` `VmRSS`, macOS `ps -o rss=`), `WaitReady(ctx)`, `ReadyAfter()` (exec to
  the first `200` of `/readyz`), `Signal(sig)`, `Exited()`, `Restart(ctx)` and
  `RestartWith(ctx, config)` (same ports, new configuration: seeding with raised limits, then
  the measured server with the defaults). Cleanup stops it by PID (SIGTERM, then SIGKILL).
- `Client.Gateway(ctx)` and `Client.GatewayFrom(ctx, after)`: a gateway connection through
  `identify` (access token and device of a person or, M1-70, the API token of an agent; DPoP proof
  with the HTTPS `htu` of the public origin) to `ready`, optionally with a cursor; `GatewayConn`
  sends heartbeats every `heartbeat_interval_ms` of `hello` (first after a random share), answers
  `reauth_required` of a person with a refreshed token and `reauth`, counts the acknowledgements
  (`Heartbeats()`), and hands every other frame with its receive time to `Next`.
  `Client.GatewayResume(ctx, session, cursor)` resumes a gateway session (op `resume`) and
  `GatewayConn.Reauth()` sends `reauth` without waiting for `reauth_required` (M1-69).
  `Client.ClientIP` sends `X-Forwarded-For` through the trusted loopback proxy path, so load tests
  can give each simulated client its own address.
- `Server.CreateUser(ctx, UserSpec)`: `NewUser` without a `testing.TB`, for seeding from several
  goroutines.
- `BuildMatrixWorld(tb, s) *World`: the M1 subset of 07 "Examples" and "Test matrix" (spaces,
  channels, Alex, Sam, Robin, Lotse, Texter, Runner, Assistent, group Leser, grants G-1 to G-5,
  threads and a forum post); expiry times relative to the harness clock. `World.Owner` is the owner
  that built it (M1-70).

- Additions of the agent conformance suite (M2-31, additive): `Options.ManualJobs` keeps the job
  runtime from working the kinds `runs.queued_expiry`, `runs.cancel_timeout`, `runs.stall_check`
  and `instance.key_rotation` (River runs no-ops for them, because River schedules by the wall
  clock while the server follows `Clock`); `Server.RunDueJobs(tb, kind)` runs every job of a
  per-run kind that is due on the harness clock once through the real handler, in `id` order, or a
  periodic kind once, and returns how many ran (`ManualJobKinds()` lists the kinds).
  `Options.ProofsOnClock` makes every client take the `iat` of its DPoP and step-up proofs from
  `Clock`, so a server on a fake clock that a test moves forward accepts them.
  `Options.PublicURL` (M2-47) is the stable origin in front of the server (the brief's
  `PublicOrigin`); `Server.ListenURL` is the own listener. `Server.SnapshotCursors(tb)` keeps the
  stored rows of `agent_consumer_cursors`; `Server.Restart(tb, RestartOptions{RestoreCursors})`
  stops the wire graph gracefully and starts it again on the same database, secrets, key store,
  file root, configuration and clock with a new listener (it needs `Options.PublicURL`, which
  stays `server.public_url`); with `RestoreCursors` it writes the snapshot back between stop and
  start, the state of a process that died before its later acknowledgements were stored;
  `RestartOptions.Config` (M7-08) replaces configuration entries for this and every later
  generation (an empty value removes one), so a test switches `server.maintenance_mode` on and off,
  and `RestartOptions.Telemetry` gives the new generation fresh (empty) telemetry sinks. `Options.MaintenanceWrites` passes
  `wire.Deps.MaintenanceWrites`, with which the test of the second line marks a writing action as
  read. `RestartOptions.WhileStopped` (M7-07) runs between stop and start: with `Server.DB()` and
  `pgtest.DB.Snapshot` a test takes a backup of the database there and later restores it, and
  `Server.FinalizeRestore(ctx)` runs `bothy restore finalize` (`wire.NewRestoreFinalize`) on it with
  the configuration of the server as the schema owner, once no `bothy_app` session is left.
  `Client.AgentCredentials()` returns the API token and the private DPoP JWK of a client made by
  `NewAgent` (test-only; the type never prints them; a person's client fails the test).

- The job `status.probe` (M7-09) runs no round by default (ruling PL/46): River runs a no-op in
  its place, so no `status.updated` lands in the event log behind a test. The test's own
  `JobModules` hook still sees the real use case (`test/status` runs the rounds itself);
  `Options.StatusProbe` runs the real job as in production.

- The jobs `files.purge_objects` and `files.cleanup_pending` run with their real use cases (ruling
  PL/54): either removes the row of a deleted file at any moment after the delete (the purge job
  right after it, the catch-up of the cleanup on its start and every `file.gc_interval`), and some
  suites wait for that (`test/leak`, `test/canary`). A test world that reads the files row of a
  deleted file gives River no-ops for `JobModules.FilesPurge` and `JobModules.FilesCleanup` in its
  `JobModules` hook (`test/voice`, `test/privacy`, `test/filecleanup`); the harness does not
  enforce this.

## Deviations from the brief (rulings M1-71/5 to M1-71/7)

- `public_url` must name the port before the listener exists, so `Start` reserves a free port of
  `127.0.0.1` first and retries a start whose port was taken in between.
- `Client.Gateway` came with M1-72 (the gateway of M1-64 was not in the base of M1-71); M1-70
  adds the identify with the API token of an agent (INT-W40/1).
