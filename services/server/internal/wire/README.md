# Package `internal/wire`

Normative is [02-architecture.md](../../../../docs/spec/02-architecture.md) ("Code layout",
"Configuration loading", "Start", "Shutdown"); the chapter wins on any difference.

## Purpose

Builds the object graph of each process role (`api`, `worker`, `all`): the configuration schema
of the server, the `Config` structs of the platform packages, the route mounting (M1-17) and
the assembly of `bothy serve` (M1-11). Only `cmd` imports it.

## Code

| File | Content |
|---|---|
| `config.go`, `configkeys_*.go`, `configchecks.go`, `configchapters.go` | configuration schema and cross-checks |
| `database.go`, `egress.go`, `secrets.go`, `telemetry.go` | binding of the platform `Config` structs, the secret provider |
| `audit.go` | `Audit` (`NewAudit`, `Start`, `UoW`, `Recorder`, `RightsHook`), `AuditConfig`, `SecretsKEK`: the audit module of a role (M1-24) |
| `txn.go` | `Modules`, `NewModules`, `NewTxnRegistry` (the one list of hooks and subscribers, golden file `testdata/txn_registrations.golden`), `UnitConfig` |
| `identity.go` | `Identity` (`NewIdentity`, `Start`), `IdentityConfig`, `TokenPepper`: devices, the DPoP verifier with its jti store and the device handlers (M1-33) |
| `routes.go` | `Role`, `Handlers`, `MountRoutes` (start step 7, every route through the HTTP chain, CORS preflights), `RouteOptions`, `HTTPOperation` |
| `jobs.go` | `JobsConfig`, `JobRole`, `NewJobRegistry` (one job registry for all roles), `MigrateUpSteps` (River before goose, then the after-step `instance`) |
| `instance.go` | `MigrationSteps` (the after-step `instance`), `InstanceCounters` and `loadInstance`, the start step `instance` |
| `authz.go` | catalogue start check (M1-25): `BuildRegistry` over the module `Register` functions (`Deps.Registrations` replaces them in tests), `RouteBindings`, `NewAuthzCatalog` (closed catalogue with `bothy.authz.errors`); every role checks routes and registrations before the routes are mounted, one ERROR line `registration check failed` per violation; `moduleRegistrations` holds `authz.Register` (`access.changed`, M1-29); `NewRightsHook` with `RightsSources` builds the rights hooks of authz (M1-29) |
| `ratelimit.go`, `configkeys_ratelimit.go` | `RateLimitConfig`, `InstallRateLimits` and the start check (M1-19); the keys `ratelimit.*` |
| `serve.go` | `Serve`, `ServeConfig`, `Deps`: start order and stop steps of `bothy serve` |
| `switchlog.go` | the process logger that follows the switch from stdout JSON to the telemetry logger |

## Serve assembly

`Serve(ctx, values, role, deps)` returns a `lifecycle.Runner`; it starts nothing itself. The
components, in start order:

| Component | Does | Roles |
|---|---|---|
| `telemetry` | `otel.Setup`; logs switch to the server logger | all |
| `secrets` | resolves `database.password_ref` once (fail fast) | all |
| `internal_listener` | opens `server.internal_listen_addr` with `/healthz` and `/readyz`, logs `internal listener started` | all |
| `database` | `pg.Open`, `WaitReady` within `startup.dependency_wait`; readiness check `database` | all |
| `schema` | `pg.CheckSchema` against the highest embedded version; older aborts with the hint to run `bothy migrate up` | all |
| `instance` | loads the instance row through `instance.State` (missing: abort with the hint to run `bothy migrate up`), logs the setup state, compares the own version with `instance.min_server_version`; below it the readiness check `schema_requires_newer_server` fails for good (M1-15) | all |
| `audit` | resolves `privacy.audit_ip_kek_ref` once (fail fast), builds the audit module with the unit of work of the role (`NewAudit`) and starts the aggregator of denials (`Audit.Start`), registering the stop step `audit_denials` (M1-24) | all |
| `identity` | after `audit`, `sync` and `authz`: resolves `auth.token_pepper_ref` once (32 bytes as hex, fail fast), builds `Identity` over the unit of work of `audit` and the Authorizer, starts the sweeper of the jti store with the stop step `dpop_replay_sweeper` in `PhaseHTTP`; `routes` mounts its device handlers unless `Deps.Handlers` names one (M1-33) | all |
| `routes` | catalogue check of route bindings and registrations (all roles, M1-25); then `NewChain` (validator over `api/rest.Spec`, `actor.AnonymousOnly`), `InstallRateLimits` (start check of the classes, limiter in `StageRateLimit`, sweeper with the stop step `ratelimit_sweeper` in `PhaseHTTP`, M1-19) and `MountRoutes` on the public mux with the `authz` catalogue | api, all (check: every role) |
| `jobs` | `jobs.ValidateMigrations` (missing River migrations abort with the hint to run `bothy migrate up`), `jobs.New` for the role, `Start` of River (02 "Start" step 8; no-op for api and while no kind is registered), `Runner.FailOn` on `Failed` | all |
| `event_consumers` | after `jobs`: registers the handlers of `consumerHandlers` (none yet), inserts their cursor rows at the head (`Ensure`), registers the stop step `event_consumers` in `PhaseConsumers` and starts the runner with its own `LISTEN` connection; an unexpected end of the runner calls `Runner.FailOn` (M2-01) | worker, all |
| `public_listener` | opens `server.listen_addr`, logs `public listener started` | api, all |
| `ready` | readiness passes `startup`, logs `server ready` | all |

Stop steps: `readiness` (not ready), `drain` and `public_listener` (api, all, budgets
`shutdown.drain_delay` and `shutdown.http_grace`), `jobs` and then `jobs_wait` (worker, all: River's
soft stop within `shutdown.jobs_soft_timeout`, then for the cancelled jobs to end what
`shutdown.total_timeout` leaves after the fixed budgets of the role, left out when nothing is
left; docs/DECISIONS.md M1-14/4), `telemetry`
(`shutdown.telemetry_flush_timeout`), `audit_denials` and `pool` (phase `database`; the audit
step is registered later, so it runs first and flushes the open aggregations of denials before
the pool closes), `internal_listener`.

Every unit of work of a role runs on a registry from `NewTxnRegistry(NewModules(pool, log))`;
`NewTxnRegistry` refuses `Modules` without the audit hook, so no unit can skip the chain.

The rights hooks (M1-29): `NewModules` builds `authzapp.RightsHook` (`NewRightsHook` with the
PrincipalSource of identity and the empty structure and agent policy sources until M1-54 and
M1-51) and registers `Begin` for phase 1 and `BeforeCommit` for phase 3.1; `NewTxnRegistry`
refuses Modules without them. The hook needs the appender of sync and the recorder of audit,
which need the unit of work themselves, so the start step `authz` binds them
(`Audit.RightsHook().Bind`) before any route or job runs; an unbound hook fails every rights
change with kind internal (the registries of `bothy migrate up` and of the job registry check
never run one). `authorize.AccessChanged` has no production subscriber in M1.

## Instance and counters

`MigrationSteps(pool, fsys)` gives `bothy migrate up` (and `pgtest.WithSteps` in tests) the
after-step `instance`: in one unit of work on the migrate pool it reads the goose version and,
when that equals the newest migration of `fsys`, records it with `migrations.MinServerVersion`
through `instance.State.RecordMigration`. `InstanceCounters(pool)` builds `instance.Counters`; only
the constructors of the phase hooks of authz, sync and audit may receive it, and
`TestCountersOnlyUsedByPhaseHooks` (with the negative fixtures under `testdata/countersuser`, `testdata/countersflow`, `testdata/countersshape` and
`testdata/countersgeneric`) fails when another package names the port or `instance.CounterName`,
calls the port, has an expression whose type contains it or instantiates a generic with it.
Below a permanently failing readiness check the `ready` step logs `server started, not ready`
instead of `server ready`.

The event log (M1-16, `sync.go`): `NewModules` builds the sync phase hook (phase 3.2) next to the
audit hook, and `NewTxnRegistry` refuses Modules without either. The start step `sync` builds
`Sync` over the unit of work of the audit step (the appender as `eventlog.Appender` for the
modules, the partition maintenance); the step `log_tail` (api and all, after `routes`) starts the
log tail, waits until it has read the head and registers the stop step `event_tail` in
`lifecycle.PhaseEventTail`; a tail that fails later calls `Runner.Fail` (exit code 1). The step
`jobs` builds the job registry with `sync.partition_maintenance` (`NewJobRegistry(&JobModules{})`,
queue `maintenance`, every `sync.partition_maintenance_interval`, `RunOnStart`).
`MigrateUpSteps(pool, fsys, MigrateUpConfig{}, result)` adds the after-step
`event_log_partitions` (the same maintenance with the configured values, upgrade.md rule M10);
`MigrationSteps`, which `pgtest.WithSteps` uses, does not, so test databases have no partitions.
`SyncConfigFrom` binds the `sync.*` keys of `configkeys_sync.go`.

## Extension points

A later ticket adds its part in `components` of `serve.go` (the start sequence `register`
adds), at the marked place, with its stop step:

| Ticket | Component | Place | Stop phase |
|---|---|---|---|
| M1-32 | signing keys, readiness `signing_keys` | after the instance | none |
| M1-17 | module handlers in `Deps.Handlers` | `routes` | none |
| M1-28, M1-65 | sinks of the log tail (rights cache, then hub) in `tailSinks` | `sync.go` | none |
| M1-65 | gateway connections | with the log tail | `connections` |
| M1-43 | readiness `trusted_header_requires_loopback` | before `ready` | none |
| M1-18 | middleware chain around the public mux | `public_listener` | none |

Stack-internal webhooks (Alertmanager) go on the mux of `httpserver.Internal` before
`internal_listener` starts.
