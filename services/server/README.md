# services/server

Status: planned (M0). No code yet. The normative source is
[docs/spec/02-architecture.md](../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

The server is a modular monolith in Go (ADR 0004, 0005): one binary `bothy`, one
PostgreSQL database (ADR 0006), one object store. Agents, plugins and apps never run in this
process; they use the public API (B6, B10).

## Roles and commands

The register of all subcommands is in 02-architecture.md, section 3.3 (ruling L1); here only
the overview.

| Command | Purpose |
|---|---|
| `bothy serve --role=api` | REST, gateway WebSocket, log tail and fan-out |
| `bothy serve --role=worker` | River jobs, event consumers (push, webhooks), file processing, maintenance |
| `bothy serve --role=all` | both in one process (default) |
| `bothy migrate up\|status\|validate` | migrations (goose, rivermigrate); never implicitly at startup |
| `bothy setup bootstrap-owner\|issue-enrollment-code` | setup, first owner, enrollment code (08-identity.md) |
| `bothy config check\|print` | check the configuration and print it redacted |
| `bothy import discord ...` | import and read mirror through the API (22-import-migration.md) |
| `bothy restore finalize` | after a restore: new sync epoch (16-operations.md) |
| `bothy secrets init\|export` | create and back up secrets (16-operations.md) |
| `bothy healthcheck --url` | health check against the internal listener (16-operations.md) |
| `bothy files rescan` | have files checked again (12-files.md) |
| `bothy telemetry pseudonym` | pseudonym of a principal for debugging |
| `bothy dev seed` | test data, only in the environment `dev` |

## Structure

```text
cmd/bothy/            entry point, subcommands
api/                  generated from docs/api/ (REST server, WebSocket types, route metadata)
architecture/         modules.yaml: module register, source for go-arch-lint and checks
migrations/           goose SQL, embedded
internal/kernel/      cross-cutting ports
internal/platform/    technology without domain logic
internal/wire/        wiring per role, registration of the in-tx subscribers
internal/<module>/    one module per directory (table below)
sqlc.yaml             one entry per module
.go-arch-lint.yml     generated from architecture/modules.yaml
```

Every module has the same layers: port in the root package, `domain/`, `app/`, `adapters/<kind>/`
(02-architecture.md, section 5).

## Modules

| Module | Summary |
|---|---|
| [instance](internal/instance/README.md) | settings, flags, signing keys, counters |
| [audit](internal/audit/README.md) | audit log with hash chain |
| [authz](internal/authz/README.md) | `authorize`, groups, grants, I1 |
| [billing](internal/billing/README.md) | costs and prices |
| [identity](internal/identity/README.md) | principals, devices, sessions, tokens |
| [chat](internal/chat/README.md) | spaces, channels, threads |
| [agents](internal/agents/README.md) | agents, backend policy, sessions, commands |
| [files](internal/files/README.md) | files, scan, preview |
| [messaging](internal/messaging/README.md) | messages, reactions, pins |
| [runs](internal/runs/README.md) | triggering, runs, live box, components |
| [sync](internal/sync/README.md) | event log, catch-up, personal sync data |
| [notify](internal/notify/README.md) | notification, push |
| [gateway](internal/gateway/README.md) | WebSocket, fan-out |
| [search](internal/search/README.md) | full-text search |
| [stats](internal/stats/README.md) | statistics for the console |
| [apps](internal/apps/README.md) | apps, webhooks |
| [importer](internal/importer/README.md) | import |
| [privacy](internal/privacy/README.md) | privacy requests |
| [status](internal/status/README.md) | status page |

## `internal/kernel`

Cross-cutting ports that every module may use. `kernel` depends on nothing in the project.

| Package | Content | Implemented by |
|---|---|---|
| `ids`, `clock`, `errs` | UUIDv7, time source, error kinds with code | kernel |
| `actor` | principal of a request, `Authenticator` | `identity` |
| `authorize` | actions (generated from 05), resource, `Decision`, `Authorizer`, in-tx event `AccessChanged` | `authz` |
| `txn` | Unit of Work, phases before the commit, in-tx event bus | `platform/pg` |
| `eventlog` | `Appender` (durable), `EphemeralPublisher` (ephemeral) | `sync`, `gateway` |
| `auditing` | `Recorder` | `audit` |
| `idempotency` | `Store` for `Idempotency-Key` | `sync` |
| `personaldata` | `Participant` for export and deletion | modules with tables that hold personal data |

## `internal/platform`

Technology without domain logic; only `adapters/`, `wire` and `cmd` may import it.

| Package | Content |
|---|---|
| `config` | loading and checking the configuration (02, section 13) |
| `secrets` | secret provider (ADR 0011) |
| `pg` | pool, Unit of Work with lock order, `LISTEN` connection, retry on deadlock |
| `httpserver` | server, middleware chain, `problem+json`, proof check of `authorize` (02, section 9) |
| `ratelimit` | token buckets per class from 05 |
| `jobs` | the only place with River: client per role, queues, schema `jobs.schema` |
| `otel` | telemetry setup, encapsulated `gen_ai.*` attributes (R-Telemetrie) |
| `health` | ops listener with `live` and `ready` |
| `lifecycle` | start order and shutdown per role (02, section 15) |

## Rules that the gate enforces

- Modules import only ports of allowed modules; third-party libraries only in approved adapters
  (go-arch-lint, ADR 0008).
- The SQL of a module names only its own tables (02, 7.3).
- Every operation has an action from 05; otherwise the server does not start.
- Generated code (`api/`, sqlc, `.go-arch-lint.yml`) matches its source.

## Name-bound locations

The product name appears in code only here: Go module path in `go.mod`, binary `bothy` (`cmd/bothy`), prefix
`BOTHY_` of the environment variables, image name. The list is kept in 18-release.md.
