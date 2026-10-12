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
| `routes.go` | `Role`, `Handlers`, `MountRoutes` (start step 7) |
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
| `routes` | `MountRoutes` on the public mux | api, all |
| `public_listener` | opens `server.listen_addr`, logs `public listener started` | api, all |
| `ready` | readiness passes `startup`, logs `server ready` | all |

Stop steps: `readiness` (not ready), `drain` and `public_listener` (api, all, budgets
`shutdown.drain_delay` and `shutdown.http_grace`), `telemetry`
(`shutdown.telemetry_flush_timeout`), `pool`, `internal_listener`.

## Extension points

A later ticket adds its part in `register` of `serve.go`, at the marked place, with its stop
step:

| Ticket | Component | Place | Stop phase |
|---|---|---|---|
| M1-15 | instance row, readiness `schema_requires_newer_server` | after `schema` | none |
| M1-32 | signing keys, readiness `signing_keys` | after the instance | none |
| M1-17 | module handlers in `Deps.Handlers` | `routes` | none |
| M1-16 | log tail and hub (api, all); a panic calls `Runner.Fail` | after `routes` | `event_tail` |
| M1-65 | gateway connections | with the log tail | `connections` |
| M1-14 | River client (worker, all), `shutdown.jobs_soft_timeout` | after the log tail | `jobs` |
| M2 | worker consumers | after River | `consumers` |
| M1-43 | readiness `trusted_header_requires_loopback` | before `ready` | none |
| M1-18 | middleware chain around the public mux | `public_listener` | none |

Stack-internal webhooks (Alertmanager) go on the mux of `httpserver.Internal` before
`internal_listener` starts.
