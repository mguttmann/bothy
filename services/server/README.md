# services/server

Status: skeleton (M1-05) with configuration (M1-07) and secret provider (M1-09). The Go module
builds; every module of 02 exists as a port package, the kernel packages `ids`, `clock` and
`errs`, the platform package `config` with `bothy config check|print` and the platform package
[`secrets`](internal/platform/secrets/README.md) with `bothy secrets init|export` are complete,
and `make arch` enforces the module boundaries from the register.
Since M1-11 `bothy serve` starts every role with the internal listener, health, the database
and schema check and the public listener, shuts down within `shutdown.total_timeout`, and
`bothy healthcheck` probes a loopback health endpoint.
Since M1-23 `make image` builds the multi-arch image `bothy-server` (section [Image](#image)).
The normative source is [docs/spec/02-architecture.md](../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

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
| `bothy config check\|print` | check the configuration (with `--secrets` also resolve every reference) and print it redacted |
| `bothy import discord ...` | import and read mirror through the API (22-import-migration.md); code under `cmd/bothy/importtool` (D-W-124), Discord read client: [`cmd/bothy/importtool/discord`](cmd/bothy/importtool/discord/README.md) |
| `bothy restore finalize` | after a restore: new sync epoch (16-operations.md) |
| `bothy secrets init\|export` | create and back up secrets (16-operations.md) |
| `bothy healthcheck --url` | health check of a loopback URL of the internal listener (16-operations.md) |
| `bothy files rescan` | have files checked again (12-files.md) |
| `bothy telemetry pseudonym` | pseudonym of a principal for debugging |
| `bothy dev seed` | test data, only in the environment `dev` |

Health: `/healthz` (the process is alive) and `/readyz` (ready) only on the internal listener
`server.internal_listen_addr`, outside the public API and without an action (ruling L2,
[16-operations.md](../../docs/spec/16-operations.md)).

Health (W2 resolved in 02 "Roles of a process", R-AR-13): `health` means `/healthz` and
`/readyz` on the internal listener only; metrics leave the process only over OTLP. The public
listener answers both paths with `404`. Answer format and check names:
[`internal/platform/health`](internal/platform/health/README.md). Exit codes of `bothy serve`:
`0` graceful end, `1` failed start, fatal runtime error or shutdown over
`shutdown.total_timeout`, `2` usage or configuration error, `3` forced by a second signal
([`internal/platform/lifecycle`](internal/platform/lifecycle/README.md)).

## Structure

```text
cmd/bothy/            entry point, command table of the subcommands
api/                  generated from docs/api/ (REST server, WebSocket types, route metadata)
architecture/         modules.yaml: module register, source for go-arch-lint and checks;
                      config-schema.json: exported configuration schema (generated)
migrations/           goose SQL, embedded
internal/kernel/      cross-cutting ports
internal/platform/    technology without domain logic
internal/wire/        wiring per role, registration of the in-tx subscribers
internal/<module>/    one module per directory (table below)
test/, perf/          test code across module boundaries and load tests (component testsupport)
tools/archgen/        generator: architecture/modules.yaml -> .go-arch-lint.yml (go generate)
tools/configschema/   generator: configuration schema -> architecture/config-schema.json (go generate)
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
GO_ARCH_LINT="$(uv run --project ../../tools --locked --quiet python ../../tools/bin/fetch_tools.py path go-arch-lint)" go test -race -tags contract_validate ./...
```

Contract tests (M1-71): the tag `contract_validate` turns on G8, the validation of every response of
the public listener against `docs/api/openapi.yaml`
([`contractcheck`](internal/platform/httpserver/contractcheck/README.md); a violation ends the test
binary with exit code 3). `make contract-test` runs G7, Schemathesis against the server of this tree
([`test/contract`](test/contract/README.md)). Full-server tests build on the shared harness
[`test/harness`](test/harness/README.md). The leak tests behind threat T-AZ-02 (M1-70: AZ-P4 404
equality and timing per route, AZ-P5 property tests of lists, counters, snapshot and catch-up on
random worlds, AZ-M52) live in [`test/leak`](test/leak/README.md); a failing AZ-P5 seed is
replayed with `LEAK_SEED=<seed>`.

Load and chaos tests (M1-16, M1-72): `make perf` runs the packages under [`perf`](perf/README.md)
and [`test/chaos`](test/chaos/README.md) with the build tag `perf`, one after another, and prints the
table of every performance target with value, threshold and the profile mode of ADR 0063; it is not
part of `make check`, which only vets and lints the tagged code. The thresholds come from the table
of chapter 02, never from a literal. The production validator compiles request schemas only; the
test build with `contract_validate` keeps the responses for G8.

## Image

The image `bothy-server` (ADR 0012, 18-release.md "Server images") is built from
[`Dockerfile`](Dockerfile) with the repository root as build context; the ignore file
[`Dockerfile.dockerignore`](Dockerfile.dockerignore) lets only the server sources into the
context, without tests and test data.

```sh
make image        # linux/amd64 and linux/arm64 into .cache/image/bothy-server.oci.tar,
                  # host platform loaded as bothy-server:<VERSION>-local
make image-test   # structure tests of that image (services/server/test/image, tag imagetest)
make trivy-image IMAGE=bothy-server:<VERSION>-local
```

- The builder stage runs on the build platform and cross compiles `bothy` for the target
  platform without cgo (`CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`, `-ldflags "-s -w"`);
  the final stage only copies, so no step runs under emulation.
- The binary is `/usr/local/bin/bothy`, owned by `0:0` with mode `0555`. The image runs as the
  numeric user `65532:65532` (`nonroot` of distroless), has no shell and no package manager, and
  declares no `VOLUME`; it runs with a read-only root file system, without capabilities and
  with `no-new-privileges` (Compose `x-hardening`, `/tmp` as tmpfs). `secrets-init` overrides the
  user with `0:0`.
- `/var/lib/bothy/signing-keys` (`secrets.signing_key_dir`) exists empty with owner `65532:65532`
  and mode `0700`, so a named volume mounted there takes that owner.
- Health check: `/usr/local/bin/bothy healthcheck --url http://127.0.0.1:8081/readyz`, interval
  `15s`, timeout `5s`, start period `30s`, `4` retries, equal to `x-server-healthcheck` in
  `infra/compose/compose.yaml` (a test of `make image-test` compares both).
- Build arguments: `VERSION` (required, must equal [VERSION](VERSION)), `REVISION` (commit SHA;
  a build from a dirty tree still labels `HEAD`) and `SOURCE_URL` (default
  `https://github.com/OWNER/bothy`). They set the OCI labels `org.opencontainers.image.version`,
  `.revision` and `.source`; `.title` is `bothy-server`, `.licenses` is `Apache-2.0`.
- Local tags are `<VERSION>-local`, never `latest`. The release workflow pushes by digest and
  tags only after the vulnerability scan.

Base images, pinned by digest (`make lint` rejects any `FROM` without one; Renovate updates the
digests, and the golang tag follows the `toolchain` of [go.mod](go.mod)):

| Stage | Tag | Digest (image index) |
|---|---|---|
| build | `golang:1.27.2-trixie` | `sha256:e58d6f83b3416618d8bcac2b3dde1b7f7e3c4a77d25e88637f8bbae81536c48d` |
| final | `gcr.io/distroless/static-debian13:nonroot` | `sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3` |

`make image` builds several platforms at once. Docker Engine with the containerd image store does
that with its default builder; otherwise create a builder once and pass it:

```sh
docker buildx create --name bothy-multiarch --driver docker-container
make image IMAGE_BUILDER=bothy-multiarch
```

Variables: `IMAGE_NAME`, `IMAGE_TAG`, `IMAGE_PLATFORMS`, `IMAGE_OCI`, `IMAGE_BUILDER`; `make
image-test IMAGE=<ref>` tests another loaded image.

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
| [`pg`](internal/platform/pg/README.md) | pool, Unit of Work with lock order, `LISTEN` connection, retry on deadlock |
| [`httpserver`](internal/platform/httpserver/README.md) | server, middleware chain, `problem+json`, proof check of `authorize` (02, section 9); listener part built in M1-11, chain in M1-18, transport protection in M1-21 (plaintext only from loopback and `server.trusted_proxies`, own TLS from 1.2 with rotation, HSTS on both listeners; 19, SEC-TLS-1 to SEC-TLS-3) |
| `ratelimit` | token buckets per class from 05 |
| [`jobs`](internal/platform/jobs/README.md) | the only place with River: client per role, queues, schema `jobs.schema`, migrations in `bothy migrate`; built in M1-14 |
| `otel` | telemetry setup, encapsulated `gen_ai.*` attributes (R-Telemetrie) |
| [`health`](internal/platform/health/README.md) | `/healthz` and `/readyz` on the internal listener `server.internal_listen_addr` (L2); built in M1-11 |
| [`lifecycle`](internal/platform/lifecycle/README.md) | start order and shutdown per role (02, section 15); built in M1-11 |
| [`egress`](internal/platform/egress/README.md) | the only client for outgoing HTTP connections: target allow list, IP check at connection setup, checked redirects, deadlines and size limits, loopback client for the health check (19-security.md, section 8; built in M1-22) |
| `buildinfo` | version of the server as a constant equal to `VERSION` (18-release.md) |

## Rules that the gate enforces

- Modules import only ports of allowed modules; third-party libraries only in approved adapters
  (go-arch-lint, ADR 0008).
- The SQL of a module names only its own tables (02, 7.3).
- Every operation has an action from 05; otherwise the server does not start.
- Generated code (`api/`, sqlc, `.go-arch-lint.yml`) matches its source.
- Every reading sqlc query on a content table carries `channel_id = ANY(...)` or
  `space_id = ANY(...)` with the visible lists of `Authorizer.Scope`, or stands with a category
  and a reason on the reviewed exception list `tools/lint/sql_visibility.yaml` (`make
  sql-visibility`, AZ-P12, 07 7.4 rule 1).
- Register, core contract (03, `actions.yaml`), module tables in 02, component diagram and
  module READMEs agree (`make register-check`, ADR 0050).

## Name-bound locations

The product name appears in code only here: Go module path in `go.mod`, binary `bothy` (`cmd/bothy`), prefix
`BOTHY_` of the environment variables, image name and the default of the displayed product name,
the constant `DefaultProductName` in `internal/instance/domain` (setting
`branding.product_name`, ADR 0044). 02-architecture.md, section 6, keeps the complete list by kind;
`tools/ci/tests/test_product_name_places.py` holds it.
