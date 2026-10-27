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
| `routes.go` | `Role`, `Handlers`, `MountRoutes` (start step 7, every route through the HTTP chain, CORS preflights; optional `RouteOption`s adjust the chain operation of a route before it is wrapped), `RouteOptions`, `HTTPOperation` (sets `AnonymousCredentials` for the operations with action `webhook_in.post`, M2-27), `RouteOption`, `WebhookBodyLimit` |
| `jobs.go` | `JobsConfig`, `JobRole`, `NewJobRegistry` (one job registry for all roles), `MigrateUpSteps` (River before goose, then the after-step `instance`) |
| `instance.go` | `MigrationSteps` (the after-step `instance`), `InstanceCounters` and `loadInstance`, the start step `instance` |
| `authz.go` | catalogue start check (M1-25): `BuildRegistry` over the module `Register` functions (`Deps.Registrations` replaces them in tests), `RouteBindings`, `NewAuthzCatalog` (closed catalogue with `bothy.authz.errors`); every role checks routes and registrations before the routes are mounted, one ERROR line `registration check failed` per violation; `moduleRegistrations` holds `authz.Register` (`access.changed`, M1-29); `NewRightsHook` with `RightsSources` builds the rights hooks of authz (M1-29) |
| `incoming.go` | `InstallWebhookToken` (the token check of the incoming webhooks at `httpserver.StageWebhookToken`) and `mountInternalHooks` (their routes on the internal listener, M2-27, below) |
| `maintenance.go` | maintenance mode (M7-08, ADR 0065): `Maintenance` and `MaintenanceFrom` (`server.maintenance_mode`, `server.maintenance_retry_after`, read in `bind`), `InstallMaintenance` (the stage at `httpserver.StageMaintenance` in `mountAPI` after the webhook token, over the effect of the catalogue, `runs.RunningRuns` and the threads of chat; `Deps.MaintenanceWrites` replaces the effect in tests), `MaintenanceJobs` (the job runtime of the mode: queues `delivery` and `maintenance`, the kinds of `delivery` and `runs.cancel_timeout`), `server.unitConfig` (`pg.UnitConfig.ReadOnly` for the units of work of `bothy serve`, never of the commands) and the patcher of the gateway op `run.update`; the consumers (`NewSync`) and the presence tracker (`BindPresence`) run over `txn.ExemptInMaintenance`, and the own measurement neither records nor flushes in the mode |
| `ratelimit.go`, `configkeys_ratelimit.go` | `RateLimitConfig`, `InstallRateLimits` and the start check (M1-19); the keys `ratelimit.*` |
| `serve.go` | `Serve`, `ServeConfig`, `Deps`: the process steps (`processWiring`) and stop steps of `bothy serve`. `Deps.APNsEndpoints` and `Deps.APNsRootCAs` point the APNs adapter at Fake-APNs for in-process tests (M3-24), `Deps.FCMEndpoints` the FCM adapter at Fake-FCM (M5-11); no configuration key reaches them, the release build of `cmd/bothy` leaves them unset (`TestProductionWiringHasNoTestOverrides`, `TestFCMDepsCarryOnlyTestOverrides`), and only its build with the Go tag `e2e` sets `APNsEndpoints` and `FCMEndpoints` from two environment variables (`cmd/bothy/internal/e2epush`, the mobile E2E test) |
| `wiring.go` | `moduleWiring` and the list `moduleWirings`: how each module plugs into `bothy serve` (PL/28) |
| `module_<name>.go` | one file per wired module of `architecture/modules.yaml`: its state in `server`, its start parts with their order keys, handlers, route bindings, mounts and jobs |
| `setup.go` | `InstallSetupGate` and `RouteActions` (the setup gate over the instance row, M1-37); `Setup` (`NewSetup`, `BootstrapOwner`, `IssueRecoveryCode`, `Close`): the parts of the server `bothy setup` needs, as `bothy_app` |
| `restore.go` | `RestoreFinalize` (`NewRestoreFinalize`, `Finalize`, `Close`): `bothy restore finalize` as `database.user` (the schema owner) over the modules of a server of the internal role `restore` (`restore_modules.go`, M7-07: the start parts from the secrets to privacy without listeners, River work, consumers or measurement, an insert-only job runtime, `lifecycle.Runner.Shutdown` on `Close`), so the deletion journal applies through the normal deletion use cases; `NewRestoreService` (with the port `instanceapp.DeletionJournal`, `Privacy.RestoreJournal`), `RestoreCursors` (the cursor ports of agents, apps and sync, in this order), `ParseRelocationManifest` (`backup_manifest.v1`); the start lock `checkRestoreGuard` with the readiness check `restore_not_finalized` and `database.restore_guard`; while it holds, `whileFinalized` skips the steps `log_tail`, `agent_presence`, `jobs`, `event_consumers` and `public_listener`, and the terminal reconcile of `routes` waits too (M7-04, R-DM-50, D-M7-04-7) |
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
| `instance` | loads the instance row through `instance.State` (missing: abort with the hint to run `bothy migrate up`), logs the setup state, compares the own version with `instance.min_server_version`; below it the readiness check `schema_requires_newer_server` fails for good (M1-15); unless `database.restore_guard` is `off`, a database identity that differs from the stored one registers `restore_not_finalized`, which fails for good (M7-04) | all |
| `audit` | resolves `privacy.audit_ip_kek_ref` once (fail fast), builds the audit module with the unit of work of the role (`NewAudit`) and starts the aggregator of denials (`Audit.Start`), registering the stop step `audit_denials` (M1-24) | all |
| `identity` | after `audit`, `sync` and `authz`: resolves `auth.token_pepper_ref` once (32 bytes as hex, fail fast), builds `Identity` over the unit of work of `audit` and the Authorizer, starts the sweeper of the jti store with the stop step `dpop_replay_sweeper` in `PhaseHTTP`; `routes` mounts its device handlers unless `Deps.Handlers` names one (M1-33) | all |
| `agents` | after `identity`: builds `Agents` (`NewAgents`) over the unit of work, recorder and rights hook of `audit`, the appender of `sync`, the Authorizer and the principal writer, directory and token revocation of identity; `routes` mounts its handlers unless `Deps.Handlers` names one (M1-51) | all |
| `routes` | catalogue check of route bindings and registrations (all roles, M1-25); then `NewChain` (validator over `api/rest.Spec`, the Authenticator of identity from the step `sessions`, `actor.AnonymousOnly` without it), `InstallRateLimits` (start check of the classes, limiter in `StageRateLimit`, sweeper with the stop step `ratelimit_sweeper` in `PhaseHTTP`, M1-19), `InstallWebhookToken` (M2-27) and `MountRoutes` on the public mux with the `authz` catalogue and `WebhookBodyLimit`; then, in every role, `mountInternalHooks` (M2-27, below) | api, all (check and internal webhook routes: every role) |
| `jobs` | `jobs.ValidateMigrations` (missing River migrations abort with the hint to run `bothy migrate up`), `jobs.New` for the role, `Start` of River (02 "Start" step 8; no-op for api and while no kind is registered), `Runner.FailOn` on `Failed` | all |
| `event_consumers` | after `jobs`: registers the handlers of `consumerHandlers` (`webhooks`, `push`, `privacy.journal` since M7-07), inserts their cursor rows at the head (`Ensure`), registers the stop step `event_consumers` in `PhaseConsumers` and starts the runner with its own `LISTEN` connection; an unexpected end of the runner calls `Runner.FailOn` (M2-01) | worker, all |
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
PrincipalSource of identity, the agent policy source of agents (`NewAgentPolicySource`, M1-51)
and the empty structure source until M1-54) and registers `Begin` for phase 1 and `BeforeCommit` for phase 3.1; `NewTxnRegistry`
refuses Modules without them. The hook needs the appender of sync and the recorder of audit,
which need the unit of work themselves, so the start step `authz` binds them
(`Audit.RightsHook().Bind`) before any route or job runs; an unbound hook fails every rights
change with kind internal (the registries of `bothy migrate up` and of the job registry check
never run one). `authorize.AccessChanged` has no production subscriber in M1.

The subscribers of sync (M1-61): `NewModules` builds `syncapp.Subscribers` over the personal
repository and `NewTxnRegistry` subscribes `messaging.MessagePosted -> sync` (drafts with
`clear_draft`); the start step `authz` and `bothy setup` bind its appender
(`Audit.SyncSubscribers().Bind`). `personal.go` builds the personal data use cases and handlers
of sync (`NewPersonal`, `PersonalConfig`) in the chat step after messaging.

Thread moves (M1-62): `NewModules` also builds `messagingapp.Subscribers` (required,
`Modules.MessagingSubscribers`), and `NewTxnRegistry` subscribes `chat.ThreadMoved -> messaging`
and then `chat.ThreadMoved -> sync`, which change `messages.channel_id` and
`read_states.channel_id` and `drafts.channel_id` of the moved thread in its transaction.

Incoming webhooks (M2-27): `NewModules` also builds `appsapp.NewArchiveSubscriber` over the apps
repository (required, `Modules.AppsArchive`; `NewTxnRegistry` refuses Modules without it), and
`NewTxnRegistry` subscribes `chat.ChannelArchived -> apps` after `messaging.MessagePosted -> runs`:
`ArchiveSubscriber.OnChannelArchived` revokes the incoming webhooks of the archived channel in its
transaction (docs/DECISIONS.md D-M2-27-21).

## Incoming webhooks (M2-27)

- Configuration: `configkeys_apps.go` declares `webhook_in.max_body_bytes` and
  `webhook_in.formats_enabled` (chapter 11); `IncomingConfig` binds `appsapp.IncomingConfig`
  (`message.max_content_bytes`, `webhook_in.formats_enabled`, `identity.last_active_granularity`,
  `server.public_url`) in the step `apps`, which now needs chat and messaging (`AppsDeps.Chat`,
  `AppsDeps.Messages`). The start check `webhook_in_formats` (`configchecks.go`) requires every
  entry of `webhook_in.formats_enabled` to be `native` or `alertmanager` and to appear once; an
  empty list switches every format off.
- `NewApps` builds `appsapp.NewIncoming` (`Apps.Incoming`) and its handlers (`Apps.IncomingHandlers`,
  `Handlers.Incoming`): `listIncomingWebhooks`, `createIncomingWebhook`, `revokeIncomingWebhook`,
  `postIncomingWebhook` and `postIncomingWebhookWithBearer`.
- `HTTPOperation` sets `httpserver.Operation.AnonymousCredentials` for the operations with action
  `webhook_in.post`, so their token never reaches the authenticator; `WebhookBodyLimit(limit)`
  sets `BodyLimit` and `BodyLimitKey` (`webhook_in.max_body_bytes`) on them; `mountAPI` passes it to
  `MountRoutes` with the value of `webhook_in.max_body_bytes`.
- `InstallWebhookToken(chain, apps, limiter)` installs `appshttp.TokenStage` at
  `httpserver.StageWebhookToken` with `limiter.FailedCredentialGuard` (the failed-credential
  token of `rl.public_auth`); `mountAPI` calls it after `InstallRateLimits`, before the routes.
- `mountInternalHooks` (end of the step `routes`, every role, 16 section 4.2): the step
  `internal_listener` keeps its mux (`server.internalMux`), and the two post operations are mounted
  there through a chain of their own: the chain configuration of the public listener with empty
  `server.trusted_proxies` and CORS origins (the client IP is the peer address, forwarded headers
  are never read, R-SE-09), `actor.AnonymousOnly` as the authenticator, a request validator of only the two post operations (`internalHooksValidator`, so a worker role does not compile the validator of the whole API before it is ready), its own rate limiter over
  only these two operations (so `rl.webhook_in` and `rl.public_auth` count per listener; its
  sweeper registers a second stop step `ratelimit_sweeper`), `InstallWebhookToken` and
  `WebhookBodyLimit`. Nothing else of the API is mounted there (docs/DECISIONS.md D-M2-27-18).

## Instance and counters

`MigrationSteps(pool, fsys)` gives `bothy migrate up` (and `pgtest.WithSteps` in tests) the
after-step `instance`: in one unit of work on the migrate pool it reads the goose version and,
when that equals the newest migration of `fsys`, records it with `migrations.MinServerVersion`
through `instance.State.RecordMigration`. `InstanceCounters(pool)` builds `instance.Counters`; only
the constructors of the phase hooks of authz, sync and audit may receive it, and
`TestCountersOnlyUsedByPhaseHooks` in `test/countersscan` (with the negative fixtures under its `testdata/countersuser`, `testdata/countersflow`, `testdata/countersshape` and
`testdata/countersgeneric`) fails when another package names the port or `instance.CounterName`,
calls the port, has an expression whose type contains it or instantiates a generic with it.
The scan checks the packages of the module from source in import order, one after the other, and
reads only packages outside the module from export data, so it never compiles the module a second
time (docs/DECISIONS.md PL/30, `TestCountersScanBuildsNothingOfTheModule`). The scans run in
`make arch` without the race detector and skip in the race builds of `make test` (PL/42).
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

Each module wires itself in its own file `module_<name>.go` (PL/28): a function `<name>Wiring`
returns its `moduleWiring`, listed once in `moduleWirings` (`wiring.go`). A part names its start
step and an order key; parts of one step with adjacent keys form one start component (the step
`chat` runs the parts of chat, messaging, runs, sync, agents, notify and stats), and `serve.go`
only iterates. New wiring code goes into the file of its module, not into `serve.go`; a new
module adds its file and one line in `moduleWirings` and leaves `notWiredYet` of
`wiring_internal_test.go`, which checks that every module of the register is wired exactly once.
`TestStartSequenceUnchanged` pins the start components of each role. The table names the earlier
extension points:

| Ticket | Component | Place | Stop phase |
|---|---|---|---|
| M1-32 (built) | signing keys (`startSigningKeys`: `EnsureActive`, readiness `signing_keys`), key store check in `startSecrets`, job `instance.key_rotation` in `NewJobRegistry` | step `signing_keys` after `identity` | none |
| M1-34 (built) | session part of identity (`startSessions`: `Identity.BindSessions` with the signing keys and the instance id; the Authenticator of the chain, the session handlers) | step `sessions` after `signing_keys`, before `routes` | none |
| M1-17 | module handlers in `Deps.Handlers` | `routes` | none |
| M1-28, M1-65 (built) | sink of the log tail `DeliverySink` (`Authorizer.Observe`, then `Hub.Dispatch`, one goroutine), liveness check `event_tail` | step `log_tail` (`startLogTail`, `gateway.go`, `sync.go`) | `event_tail` |
| M1-65 (built) | gateway connections: `Gateway.StopConnections` closes every connection with 4013 | `mountGateway` | `connections` |
| M1-43 | readiness `trusted_header_requires_loopback` | before `ready` | none |
| M1-18 | middleware chain around the public mux | `public_listener` | none |

Stack-internal webhooks (Alertmanager) go on the mux of `httpserver.Internal`; since M2-27 the
step `routes` adds the two post operations of the incoming webhooks to it after
`internal_listener` has started (`mountInternalHooks`, see "Incoming webhooks").
