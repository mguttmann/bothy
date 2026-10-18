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
  key fails the start; `EgressDenyAll` makes every outgoing client refuse every target before name
  resolution; `RaisedRateLimits()` gives the entries that lift every rate-limit class
  (23 R-QA-11). `Telemetry` (`NewTelemetrySinks`, M1-69) replaces the OTLP exporters with an
  in-memory span exporter, a `MemoryLogExporter` and a manual metric reader and turns
  `telemetry.enabled` on unless `Config` sets it; `otel.Setup` installs global providers, so a test
  that sets it never runs in parallel with another harness start. `SecretFiles` writes extra files
  into the secrets directory (mode 0400; the client secret of a fake OIDC provider), `OIDCRootCAs`
  is the TLS root of the OIDC provider requests, `PublicHost` (`localhost`) gives the server a
  domain name as WebAuthn relying party, and `ProofObserver` sees every DPoP proof a client sends.
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

## Deviations from the brief (rulings M1-71/5 to M1-71/7)

- `public_url` must name the port before the listener exists, so `Start` reserves a free port of
  `127.0.0.1` first and retries a start whose port was taken in between.
- `Client.Gateway` came with M1-72 (the gateway of M1-64 was not in the base of M1-71); M1-70
  adds the identify with the API token of an agent (INT-W40/1).
