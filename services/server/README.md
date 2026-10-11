# services/server

Status: skeleton (M1-05) with configuration (M1-07). The Go module builds; every module of 02
exists as a port package, the kernel packages `ids`, `clock` and `errs` and the platform package
`config` with `bothy config check|print` are complete, and `make arch` enforces the module
boundaries from the register. The normative source is
[docs/spec/02-architecture.md](../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

Module path until V1: `github.com/OWNER/bothy/services/server` (PLAN P-05). `go.mod` names the
older maintained Go line as the `go` directive and the built version as `toolchain` (ADR 0005).
Version: [VERSION](VERSION), changes: [CHANGELOG.md](CHANGELOG.md).

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
| `bothy healthcheck --url` | health check of a loopback URL of the internal listener (16-operations.md) |
| `bothy files rescan` | have files checked again (12-files.md) |
| `bothy telemetry pseudonym` | pseudonym of a principal for debugging |
| `bothy dev seed` | test data, only in the environment `dev` |

Health: `/healthz` (the process is alive) and `/readyz` (ready) only on the internal listener
`server.internal_listen_addr`, outside the public API and without an action (ruling L2,
[16-operations.md](../../docs/spec/16-operations.md)).

## Structure

```text
cmd/bothy/            entry point, command table of the subcommands
api/                  generated from docs/api/ (REST server, WebSocket types, route metadata)
architecture/         modules.yaml: module register, source for go-arch-lint and checks
migrations/           goose SQL, embedded
internal/kernel/      cross-cutting ports
internal/platform/    technology without domain logic
internal/wire/        wiring per role, registration of the in-tx subscribers
internal/<module>/    one module per directory (table below)
test/, perf/          test code across module boundaries and load tests (component testsupport)
tools/archgen/        generator: architecture/modules.yaml -> .go-arch-lint.yml (go generate)
sqlc.yaml             one entry per module
.go-arch-lint.yml     generated from architecture/modules.yaml, never edited by hand
.golangci.yml         golangci-lint v2 configuration of the module
```

The register `architecture/modules.yaml` is the only source of the module boundaries (ADR 0050).
When an entry changes, `go generate ./tools/archgen` regenerates the configuration;
`make codegen-check` reports an outdated file, `make arch` every package without a component.

Every module has the same layers: port in the root package, `domain/`, `app/`, `adapters/<kind>/`
(02-architecture.md, section 5).

## Tests

`make test` in the repository root tests the module and sets `GO_ARCH_LINT` while doing so. The
tests of `tools/archgen` check the generated configuration with the pinned go-arch-lint and fail
without this variable. By hand, from `services/server`:

```sh
GO_ARCH_LINT="$(uv run --project ../../tools --locked --quiet python ../../tools/bin/fetch_tools.py path go-arch-lint)" go test -race ./...
```

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
| [terminal](internal/terminal/README.md) | hosts, pairing, terminal sessions, relay (from M2) |

## `internal/kernel`

Cross-cutting ports that every module may use. `kernel` depends on nothing in the project.

| Package | Content | Implemented by |
|---|---|---|
| `ids`, `clock`, `errs` | UUIDv7, time source, error kinds with code | kernel |
| `textcheck` | check of display texts (SEC-IN-4) | kernel |
| `reqmeta` | `correlation_id` and client IP in the context | kernel; set by `platform/httpserver` |
| `jose` | compact JWS and JWK with the standard library (ADR 0067) | kernel |
| `designkeys` | allowed keys of the design system, generated from the tokens | generator |
| `actor` | principal of a request, `Authenticator` | `identity` |
| `authorize` | actions (generated from 05), resource, `Decision`, `Authorizer`, in-tx event `AccessChanged` | `authz` |
| `txn` | Unit of Work, phases before the commit, in-tx event bus | `platform/pg` |
| `eventlog` | `Appender` (durable), `Redactor` (redacts the payloads of a deleted message), `EphemeralPublisher` (ephemeral) | `sync`, `gateway` |
| `auditing` | `Recorder` | `audit` |
| `idempotency` | `Store` for `Idempotency-Key` | `sync` |
| `personaldata` | `Participant` for export and deletion | modules with tables that hold personal data |
| `measure` | `Recorder` of the own measurement, in-memory counters only | `stats` |

`ids`, `clock`, `errs`, `textcheck`, `reqmeta`, `jose` and `designkeys` are listed in the register
under `kernel.core` and may be used by every package, including `domain`; the others are listed
under `kernel.ports` and are forbidden for `domain` (02, section 5). The register is authoritative.

## `internal/platform`

Technology without domain logic; only `adapters/`, `wire` and `cmd` may import it.

| Package | Content |
|---|---|
| [`config`](internal/platform/config/README.md) | schema, loading and checking of the configuration, `Bind` to the `Config` structs (02, section 13; built in M1-07) |
| `secrets` | secret provider (ADR 0011) |
| `pg` | pool, Unit of Work with lock order, `LISTEN` connection, retry on deadlock |
| `httpserver` | server, middleware chain, `problem+json`, proof check of `authorize` (02, section 9) |
| `ratelimit` | token buckets per class from 05 |
| `jobs` | the only place with River: client per role, queues, schema `jobs.schema` |
| `otel` | telemetry setup, encapsulated `gen_ai.*` attributes (R-Telemetrie) |
| `health` | `/healthz` and `/readyz` on the internal listener `server.internal_listen_addr` (L2) |
| `lifecycle` | start order and shutdown per role (02, section 15) |
| [`egress`](internal/platform/egress/README.md) | the only client for outgoing HTTP connections: target allow list, IP check at connection setup, checked redirects, deadlines and size limits, loopback client for the health check (19-security.md, section 8; built in M1-22) |
| `buildinfo` | version of the server as a constant equal to `VERSION` (18-release.md) |

## Rules that the gate enforces

- Modules import only ports of allowed modules; third-party libraries only in approved adapters
  (go-arch-lint, ADR 0008).
- The SQL of a module names only its own tables (02, 7.3).
- Every operation has an action from 05; otherwise the server does not start.
- Generated code (`api/`, sqlc, `.go-arch-lint.yml`) matches its source.
- Register, core contract (03, `actions.yaml`), module tables in 02, component diagram and
  module READMEs agree (`make register-check`, ADR 0050).

## Name-bound locations

The product name appears in code only here: Go module path in `go.mod`, binary `bothy` (`cmd/bothy`), prefix
`BOTHY_` of the environment variables, image name. The list is kept in 18-release.md.
