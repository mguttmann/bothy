# Package `test/harness`

Status: built (M1-71); M1-69, M1-70 and M1-72 extend it additively. Component `testsupport`
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
- `Options{Config, LogWriter, Telemetry, EgressDenyAll, Clock}`: `Config` takes dotted keys of
  `docs/operations/configuration.md` with the precedence of an environment variable, so an unknown
  key fails the start; `EgressDenyAll` makes every outgoing client refuse every target before name
  resolution; `RaisedRateLimits()` gives the entries that lift every rate-limit class
  (23 R-QA-11). `Telemetry` is refused until M1-69 adds the sinks.
- `Server{PublicURL, APIBase, GatewayURL, InternalURL, SecretsDir, Owner}`, `AdminDB()`
  (superuser pool, assertions only), `ConfigValue(key)`, `Clock()`.
- `NewUser(tb, UserSpec)`, `NewAgent(tb, AgentSpec)` (public API only: `createUser` with
  `local_account`, `redeemEnrollmentCode`; `createAgent`, `setAgentBillingMode`, `createToken` with
  `dpop_jkt`), `OwnerSession(ctx)` (recovery code of the setup service), `UserSession(ctx, id)`
  (`createEnrollmentCode`).
- `Client.Do(ctx, Request) Response`: DPoP proof per RFC 9449 with the newest `DPoP-Nonce`, one
  retry on `use_dpop_nonce`, refresh on `token_expired`, a UUIDv7 `Idempotency-Key` on POST and
  PATCH except the operations with `x-idempotency-exempt`, optional `Observer`; `StepUp(ctx)`
  (`createStepUpChallenge`, `completeStepUpWithDeviceKey`); `MustDo` and `DoWithStepUp`.
- `BuildMatrixWorld(tb, s) *World`: the M1 subset of 07 "Examples" and "Test matrix" (spaces,
  channels, Alex, Sam, Robin, Lotse, Texter, Runner, Assistent, group Leser, grants G-1 to G-5,
  threads and a forum post); expiry times relative to the harness clock.

## Deviations from the brief (rulings M1-71/5 to M1-71/7)

- `public_url` must name the port before the listener exists, so `Start` reserves a free port of
  `127.0.0.1` first and retries a start whose port was taken in between.
- `Client.Gateway` is not built: the gateway (M1-64) is not in this base; M1-64 adds it.
- `StartProcess` (M1-72) is not created here, as the brief says.
