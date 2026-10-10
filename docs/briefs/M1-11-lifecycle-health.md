# M1-11: Lifecycle, listeners, health and shutdown

Status: ready | Depends on: M1-10, M1-12 | Etappe: M1

Note on dependencies: the plan lists only M1-10. This brief also needs M1-12 because start steps
3 and 4 of 02 "Start" (database, schema version) and the readiness check "Datenbank erreichbar"
are part of the ticket. The wave does not change (M1-12 is in wave 5, this ticket in wave 6).

## Goal

After this ticket `bothy serve --role=api|worker|all` starts in the order of 02 "Start", opens the
internal listener (`/healthz`, `/readyz`) and, for `api`, the public listener, reports ready only
when the start is complete, and shuts down per role within `shutdown.total_timeout` on `SIGTERM`
or `SIGINT`; a second signal ends the process at once. `bothy healthcheck --url` lets a container
without shell probe readiness. Later tickets (instance, routes, jobs, event log, gateway) plug
their start and stop steps into this lifecycle instead of inventing their own (B1, L1, L2,
R-AR-13, R-AR-16, R-AR-17).

## Read first

- `docs/spec/02-architektur.md`: "Rollen eines Prozesses", "Befehle des Binarys", "Code-Layout",
  "Ausfall einer Abhängigkeit", "Panics", "Start", "Health", "Shutdown", "Leistungsziele"
  (`perf.startup_ready_max`, `perf.idle_rss_max`), "Entscheidungen dieses Kapitels" (R-AR-13,
  R-AR-16, R-AR-17), "Konfigurationswerte dieses Kapitels" (`http.*`, `startup.*`,
  `shutdown.*`).
- `docs/spec/16-betrieb.md`: "Bausteine einer Instanz", "Zwei Listener", "Rand und Identität"
  (readiness error `trusted_header_requires_loopback`, implemented later by M1-43), "Upgrade",
  "Konfigurationswerte dieses Kapitels" (`server.role`, `server.listen_addr`,
  `server.internal_listen_addr`).
- `docs/spec/05-aktionen.md`: R-AZ-22 (health has no action).
- `docs/spec/18-release.md` "Server-Images" (`bothy healthcheck` in a shell-less image).
- `docs/spec/19-sicherheit.md` "Logs ohne Geheimnisse und ohne Inhalte" (SEC-LOG-5).
- `infra/compose/compose.yaml` (`x-server-healthcheck`, `x-api`, `worker`),
  `docs/operations/install-compose.md` "Prüfung nach dem Start", `infra/helm/README.md`.
- `docs/PLAN.md`: "Arbeitsweise je Ticket", M1 row M1-11, contradictions W2 and W3.
- Code you build on: `internal/platform/otel` (M1-10: `Setup`, `Shutdown`, `NewCommandLogger`,
  `ServerErrorLog`), `internal/platform/pg` (M1-12: `Open`, `WaitReady`, `Ping`, `CheckSchema`,
  `RequiredSchemaVersion`, `migrations.FS`), `internal/platform/secrets` (M1-09),
  `internal/platform/config` and `cmd/bothy` (M1-07), `internal/kernel/clock` (M1-05).
- No API operation, table or action id is touched; health is outside the public API (L2).

## Fixed requirements that apply

1. No MVP. Build from scratch directly in the final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: a README per module or service, an ADR for every architecture
   decision, a CHANGELOG entry per release, the docs site kept current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: write the failing test, show it red, then make it green, then clean up.
7. No en dash or em dash characters in any file you write.
8. White label: no company, person, host or domain names except `example.com`, people Alex, Sam,
   Robin; no paths under a user home directory.
9. Configuration only through named keys listed in `docs/operations/configuration.md`.
10. L2 and 16 "Zwei Listener": `/healthz` and `/readyz` only on `server.internal_listen_addr`,
    never on the public listener; answers carry no detail except the names of failed checks.
11. 02 "Start": order config and telemetry, secrets, database within `startup.dependency_wait`,
    schema version (abort with hint on an older schema), instance (later), routes (later), role
    start, ready.
12. 02 "Health": `/healthz` means the process answers (plus later the log tail and hub for `api`,
    River for `worker`); `/readyz` means start complete, database reachable, schema fits, no
    shutdown begun.
13. 02 "Shutdown": `SIGTERM` and `SIGINT` start it, a second signal ends at once; total budget
    `shutdown.total_timeout`; `api`: ready false, wait `shutdown.drain_delay`, HTTP shutdown with
    `shutdown.http_grace`, close connections (later), stop log tail (later), flush telemetry within
    `shutdown.telemetry_flush_timeout`, close pool; `worker`: ready false, consumers finish (later),
    River soft stop (later), telemetry, pool; `all`: api steps, then worker steps, then telemetry
    and pool. The orchestrator waits longer than the total budget (Compose `stop_grace_period`).
14. R-AR-16: `serve` never runs migrations.
15. 18 "Server-Images": `bothy healthcheck` replaces shell and curl in the image.
16. SEC-LOG-5: no client IP addresses in general logs (also not from `http.Server` error logs).

## Scope

All Go paths are relative to `services/server/`.

### `internal/platform/lifecycle`

- `type Component interface { Name() string; Start(ctx context.Context) error }` started in
  registration order; a start error stops the already started part in reverse through the
  shutdown plan and exits `1`.
- `type StopStep struct { Name string; Phase Phase; Budget time.Duration; Run func(ctx context.Context) error }`.
  `type Phase int` with the fixed order `PhaseNotReady`, `PhaseDrain`, `PhaseHTTP`,
  `PhaseConnections`, `PhaseEventTail`, `PhaseConsumers`, `PhaseJobs`, `PhaseTelemetry`,
  `PhaseDatabase`, `PhaseInternalListener`. Roles differ only in which steps they register; this
  order yields exactly the sequences of 02 "Shutdown" for `api`, `worker` and `all`.
- `type Runner` with `New(opts Options) *Runner` (`Options{ Clock, Logger, TotalTimeout, Signals <-chan os.Signal }`;
  `Signals` nil means `os/signal` for `SIGTERM` and `SIGINT`), `Add(Component)`, `OnStop(StopStep)`,
  `Fail(err error)` (a component reports a fatal runtime error, for example a panic in the log
  tail per 02 "Panics": orderly shutdown, exit `1`), `Run(ctx) int` (returns the exit code).
- Exit codes (reuse values `cmd/bothy` already defines, add what is missing): `0` graceful end,
  `1` start failure, fatal runtime error or shutdown over budget, `2` usage or configuration error,
  `3` forced by a second signal.

### `internal/platform/health`

- `type Check func(ctx context.Context) error`.
- `type Registry` with `AddLiveness(name string, c Check)`, `AddReadiness(name string, c Check)`,
  `SetStarted()`, `SetShuttingDown()`, and `Handler() http.Handler` serving `GET` and `HEAD` for
  `/healthz` and `/readyz` (other methods `405`, other paths `404`).
- Built-in readiness states: `startup` fails until `SetStarted`, `shutting_down` fails after
  `SetShuttingDown`. Names are `snake_case` and stable; they are the only detail in answers.

### `internal/platform/httpserver` (listener part only; M1-18 adds the chain)

- `func Listen(ctx context.Context, addr string) (net.Listener, error)`: the single place where the
  server opens listeners (M1-21 adds the plaintext peer check here).
- `func NewServer(cfg Config, h http.Handler, log *slog.Logger) *http.Server` with
  `ReadHeaderTimeout = http.read_header_timeout`, `IdleTimeout = http.idle_timeout`,
  `MaxHeaderBytes = http.max_header_bytes`, `ErrorLog = otel.ServerErrorLog(log)`; no
  `ReadTimeout` or `WriteTimeout` (per-route deadlines come with M1-18).
- `func Internal(reg *health.Registry) *http.ServeMux`: the internal mux; later tickets may add
  stack-internal webhook routes to it before start (16 "Zwei Listener").
- Until M1-17 and M1-18 exist, the public listener serves a handler that answers every request
  with `404` and an empty body; M1-18 replaces it.

### `internal/wire`

`func Serve(ctx context.Context, cfg *config.Config, role Role, deps Deps) (*lifecycle.Runner, error)`
(use the typed configuration type M1-07 created) assembling, in order: telemetry (`otel.Setup`),
secrets provider, internal listener with health, pool (`pg.Open`, `WaitReady` with
`startup.dependency_wait`), schema check (`pg.CheckSchema` with
`pg.RequiredSchemaVersion(migrations.FS)`, abort on `*SchemaTooOldError`), extension points for
instance (M1-15), routes (M1-17), log tail (M1-16), jobs (M1-14), then for `api` and `all` the
public listener, then `SetStarted`. Registers readiness check `database` (`pool.Ping`) and stop
steps: not ready, drain (`api`, `all`), HTTP (`api`, `all`), telemetry, database, internal
listener. Document every extension point in the README.

### `cmd/bothy`

- `bothy serve [--role=api|worker|all] [--config <file>]`: loads config (flag overrides
  `server.role`), builds the runner with `wire.Serve`, exits with `Runner.Run`.
- `bothy healthcheck --url <url> [--timeout <duration>]` (D5).

### Configuration

Register in the schema if M1-07 did not, with type, default and doc from the chapter tables:
`server.role`, `server.listen_addr`, `server.internal_listen_addr`, `http.read_header_timeout`,
`http.idle_timeout`, `http.max_header_bytes`, `shutdown.drain_delay`, `shutdown.http_grace`,
`shutdown.telemetry_flush_timeout`, `shutdown.total_timeout` (new default, D3). Typed into the
`Config` structs of the packages above in `internal/wire`.

### Other files

- `infra/compose/compose.yaml`: `stop_grace_period: 75s` in the `x-api` anchor and for `worker`
  (D3).
- `docs/spec/02-architektur.md`, `docs/spec/16-betrieb.md`, `services/server/README.md` as listed
  under "Definition of done".

## Out of scope

- Routes, the HTTP middleware chain and problem format (M1-17, M1-18); transport checks (M1-21).
- Instance record and `schema_requires_newer_server` (M1-15), log tail (M1-16), River (M1-14),
  gateway close with `4013` (M1-65), worker consumers (M2), `trusted_header_requires_loopback`
  (M1-43), object storage readiness (M3).
- The server image and its `HEALTHCHECK` (M1-23); the startup and memory performance targets
  are measured by M1-72.
- Removing `getLiveness` and `getReadiness` from `docs/api/openapi.yaml` (M1-17).

## Boundaries

- Work in a dedicated git worktree on branch `feat/m1-11-lifecycle-health`.
- Conventional Commits with scope (`feat(lifecycle): ...`, `docs(spec): ...`) and DCO sign-off;
  squash before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and the docs named in "Definition of done".
- Follow the host rules supplied by the launcher. Tests bind only `127.0.0.1` port 0. Stop only
  processes you started, by PID (keep the PID of every `bothy` process you exec in a test and
  signal only that PID).
- Temporary directories only via `mktemp -d` or `t.TempDir()`, removed at the end.
- Never read or print real secrets. No network calls except package downloads for pinned modules,
  tools and the pinned PostgreSQL image used by `pgtest`.
- A missing core-contract piece means `Verdict: BLOCKED`, not a deviation.

## Design notes

D1. Start sequence and listeners.
- The internal listener opens right after telemetry and secrets, before the database wait, so an
  orchestrator sees `/healthz` `200` and `/readyz` `503` with `startup` while the process starts.
- The public listener opens only in the last role step (`api`, `all`); `worker` has no public
  listener.
- After both listeners are open the process logs `internal listener started` and
  `public listener started` with `server.address` and `server.port` (tests read the bound port 0
  from these lines), then `server ready` with `bothy.server.role`.
- Database down at start: `WaitReady` retries until `startup.dependency_wait`, then exit `1`.
  Database lost later: readiness check `database` fails (`503`), the process keeps running and
  recovers without intervention (02 "Ausfall einer Abhängigkeit").
- Older schema: exit `1` with the hint text from M1-12, no listener left open.

D2. Health answers (write into 16 "Zwei Listener").
- `Content-Type: application/json`, `Cache-Control: no-store`.
- Success: `200` with `{"status":"ok"}`.
- Failure: `503` with `{"status":"unavailable","failed":["<check>", ...]}`, names sorted.
- Checks run concurrently with the request context; each readiness check that talks to a
  dependency bounds itself (`database` uses `database.acquire_timeout` through `pool.Ping`).
- No authentication, no action (R-AZ-22); the internal listener is reachable only inside the
  stack.

D3. Shutdown budget.
- Each step runs with `min(step budget, remaining total)`; when the total is spent, remaining
  steps are skipped, a log line `shutdown budget exceeded` names the phase, exit `1`.
- Readiness turns to `503 shutting_down` as the first step and stays so until the internal listener
  closes as the very last step; `/healthz` stays `200` until then.
- `http.Server.Shutdown` with the HTTP step budget; hijacked connections are left to
  `PhaseConnections` (M1-65).
- At wiring time the sum of all registered step budgets must not exceed `shutdown.total_timeout`;
  otherwise the start fails with exit `2` naming both values. The current defaults violate this
  as soon as M1-14 registers `shutdown.jobs_soft_timeout`: the defaults of
  `shutdown.drain_delay`, `shutdown.http_grace`, `shutdown.jobs_soft_timeout` and
  `shutdown.telemetry_flush_timeout` add up to more than the default of
  `shutdown.total_timeout`. Change the default of `shutdown.total_timeout` to `60s` in 02
  "Konfigurationswerte dieses Kapitels" and the schema, and add `stop_grace_period: 75s` to the
  Compose services `api`, `api-tailscale` (anchor `x-api`) and `worker` (today Compose stops after
  Docker's default grace and kills the process mid-shutdown). A unit test reads
  `infra/compose/compose.yaml` and fails if `stop_grace_period` of these services is not greater
  than the schema default of `shutdown.total_timeout`. Record in DECISIONS.md.

D4. Signals. The runner listens for `SIGTERM` and `SIGINT` with a buffer of two. First signal:
orderly shutdown. Second signal at any time: log `forced shutdown`, exit `3` immediately without
running further steps.

D5. `bothy healthcheck`.
- Needs no configuration file, no secrets and no database (the container may have only the
  environment).
- Accepts only `http://` URLs whose host is a loopback address (`127.0.0.0/8`, `::1`) or
  `localhost`; anything else is a usage error (exit `2`). Reason: the command exists for the local
  container probe; it must not become a general HTTP client (M1-22 forbids those outside
  `platform/egress`, and this restriction is the documented exception).
- Own `http.Client` with timeout `--timeout` (default `5s`, matching the Compose healthcheck
  timeout; a test holds the default), no redirects, no proxy from the environment.
- Exit `0` for `2xx`, else `1` with one line on stderr naming the status code or the error class.

D6. W2 and W3.
- W2: 16 is authoritative. In 02 replace the row "Ops-Listener (Health, optional Metriken)" of
  "Rollen eines Prozesses" with "interner Listener (Health, Eingangs-Webhooks aus dem Stack)",
  adjust R-AR-13 to "Health auf dem internen Listener außerhalb der öffentlichen API" (the server
  exports metrics only over OTLP, ADR 0034), and in `services/server/README.md` describe `health`
  as `/healthz` and `/readyz` on the internal listener. Record W2 in DECISIONS.md.
- W3: if the W3 entry from M1-09 is missing in DECISIONS.md, add it (L1 extended by `secrets`,
  `healthcheck`, `restore`; `setup` replaces `admin`).

D7. Logging. `http.Server.ErrorLog` uses `otel.ServerErrorLog`, so TLS and connection errors with
client addresses never reach the logs (SEC-LOG-5).

## Tests (write them first)

Unit tests in `internal/platform/lifecycle` and `internal/platform/health` (fake clock, injected
signal channel):

- `TestStartOrderAndRollback`: components start in order; a failing third component stops the
  first two in reverse through the plan; exit `1`.
- `TestStopPhasesPerRole`: registered steps for `api`, `worker` and `all` run in exactly the
  order of 02 "Shutdown".
- `TestStepBudgetIsMinOfOwnAndRemaining` and `TestShutdownOverBudgetExits1`.
- `TestSecondSignalForcesExit3`.
- `TestFailTriggersShutdownExit1`.
- `TestBudgetSumAboveTotalFailsAtWiring` (exit `2`).
- `TestReadyzStartupThenOkThenShuttingDown`: `503 startup` before `SetStarted`, `200` after,
  `503 shutting_down` after the first signal while `/healthz` stays `200`.
- `TestHealthAnswerFormat`: JSON bodies, sorted failed names, `Cache-Control`, `405` for `POST`,
  `404` for other paths.
- `TestComposeStopGraceAboveTotal` (reads `infra/compose/compose.yaml`).

Command and integration tests (build the binary once in `TestMain` into a `mktemp` directory;
database from `pgtest`, password file at mode `0600` in a temp secrets directory; both listeners on
`127.0.0.1:0`):

- `TestServeHealthOnlyOnInternalListener`: `/healthz` and `/readyz` on the public listener give
  `404`; on the internal listener `200` once ready.
- `TestServeReadyAfterStart`: `/readyz` reaches `200` and the log line `server ready` appears.
- `TestServeSigtermExitsZeroWithinTotal`: send `SIGTERM` to the child PID; `/readyz` answers `503`
  during `shutdown.drain_delay` (set to a short value through the environment); the process exits
  `0` before `shutdown.total_timeout`.
- `TestServeSecondSignalExitsImmediately`: long `shutdown.drain_delay`, two `SIGTERM`; exit `3`
  well before the drain delay ends.
- `TestServeWorkerHasNoPublicListener`.
- `TestServeAbortsOnOldSchema`: database migrated from a test FS whose highest version is below
  the highest embedded version; exit `1`, stderr contains `bothy migrate up`, no listener stays
  open.
- `TestServeDatabaseUnreachableExits1AfterWait`: closed port, short `startup.dependency_wait`.
- `TestReadyzReportsDatabaseLoss`: health registry with the `database` check on a pool opened by
  `pg.Open` against a `pgtest` database; `/readyz` is `200`; then `DROP DATABASE <name> WITH (FORCE)`
  through `SuperConn`; `/readyz` answers `503` with `["database"]` within
  `database.acquire_timeout` plus the request.
- `TestHealthcheckCommand`: `0` against a ready server, `1` against `503`, `1` on timeout, `2` for a
  non-loopback host and for `https://example.com`.
- `TestServerErrorLogHasNoClientAddress`: a server from `NewServer` with a test handler that
  panics (the standard library then logs `http: panic serving <client address>` to `ErrorLog`);
  the captured logs contain `http server error` and no client address.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/platform/lifecycle/... ./internal/platform/health/... ./internal/platform/httpserver/... ./internal/wire/... ./cmd/bothy/...
make arch
make check
```

## Acceptance criteria

- [ ] `bothy serve` starts with each role in the order of 02 "Start"; migrations never run.
- [ ] `/readyz` is `503` before the start completes and during shutdown, `200` in between; health
      on the public listener is `404`.
- [ ] `SIGTERM` ends within `shutdown.total_timeout` with exit `0`; a second signal ends at once
      with exit `3`.
- [ ] An older schema aborts the start with the hint to `bothy migrate up`.
- [ ] `bothy healthcheck` returns `0` or `1` and refuses non-loopback URLs.
- [ ] Shutdown budgets cannot exceed the total; Compose waits longer than the total.
- [ ] 02 and 16 agree on health (W2); DECISIONS.md has W2, W3 and the shutdown budget entry.
- [ ] `make check` green; module coverage at or above `quality.coverage.go_min_percent`.

## Definition of done

- Code and tests; doc comment on every exported symbol.
- READMEs: `internal/platform/lifecycle/README.md` (phases, budgets, exit codes, how later
  tickets add components and stop steps), `internal/platform/health/README.md` (check names,
  answer format, how to add checks), `internal/platform/httpserver/README.md` (listener part;
  M1-18 extends it), `internal/wire/README.md` (serve assembly and extension points).
- Spec and docs in the same change: 02 "Rollen eines Prozesses", "Befehle des Binarys"
  (`healthcheck` flags, exit codes), "Start", "Health", "Shutdown" (budgets, exit codes, forced
  exit), R-AR-13, "Konfigurationswerte dieses Kapitels" (`shutdown.total_timeout`); 16
  "Zwei Listener" (answer format); `services/server/README.md`; `infra/compose/compose.yaml`.
- `docs/DECISIONS.md`: W2, W3 (if missing), D2, D3, D4, D5.
- Configuration reference regenerated without diff.
- `services/server/CHANGELOG.md` entry under "Unreleased".
- No new ADR: the rulings refine 02 and go to DECISIONS.md.
- `make check` green; independent review and red team (`docs/quality/red-team-gate.md`) with
  verdict `JA` recorded; merge locally into the base branch only if both are green, then
  `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then
commits, files with line counts, test results with counts (including the red runs), what is not
proven (for example behaviour under a real orchestrator), follow-ups that could not be done in
scope with the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or
`exit=1 <reason>`.
