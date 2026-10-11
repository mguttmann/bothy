---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Architecture team
informed: all planning teams
---

# Go as the server language

## Context and Problem Statement

The server holds many long-lived WebSocket connections, filters every event per recipient,
writes to PostgreSQL and runs on a Mac and in Linux containers (linux/amd64, linux/arm64).
OpenAPI and AsyncAPI are normative, server stubs are generated (B2). Which language does the
server use?

## Decision Drivers

* Concurrency for many connections in one process, without an external broadcast service.
* A static binary for small multi-arch images.
* Mature libraries for PostgreSQL, WebSocket, WebAuthn, background jobs and OpenTelemetry.
* Code generation from OpenAPI 3.1.
* A language that many build sessions write safely and uniformly (formatter, vet,
  static types).

## Considered Options

* Go
* Python with FastAPI
* Elixir with Phoenix

## Decision Outcome

Chosen option: "Go", because it offers concurrency and static types in one binary and has a
maintained library for every layer needed. `go.mod` sets the older maintained line as the
`go` directive; builds use the current version (R-Stack).

### Consequences

* Good, because goroutines per connection and a hub in the process carry the fan-out without a broker.
* Good, because one binary with roles fits into a small image (B12).
* Good, because `gofmt`, `go vet` and static types enforce uniform code across many
  sessions.
* Bad, because the SDK and the Hermes adapter stay in Python; the team maintains two languages plus Dart.
* Bad, because Go versions are only maintained until two newer ones have been released; upgrades are
  mandatory (Renovate, ADR 0041).

### Confirmation

* `make typecheck` and `make test` build and test `services/server` with `go` (gate).
* The `go` directive in `services/server/go.mod` names the older maintained Go line, the
  `toolchain` directive the built version. New Go versions arrive as a Renovate proposal
  (ADR 0041); the review holds the rule. There is no gate check for this, because "maintained"
  cannot be determined without network access.
* The build produces images for linux/amd64 and linux/arm64 (18-release.md).

## Pros and Cons of the Options

### Go

* Good, because Go 1.27 and 1.26 are maintained; go1.27.2 of 2026-10-08 is current.
  [verified, Q1]
* Good, because the standard ServeMux supports method patterns and wildcards since Go 1.22.
  [verified, Q2]
* Good, because coder/websocket (ISC) uses contexts throughout and allows concurrent writes.
  [verified, Q3]
* Good, because the OpenTelemetry SDK for Go lists traces and metrics as stable. [verified, Q4]
* Good, because oapi-codegen generates servers for `net/http` with a strict interface. [verified, Q7]
* Bad, because logs in the OTel SDK for Go are only a "Release candidate". [verified, Q4]

### Python with FastAPI

* Good, because the SDK and the Hermes adapter are already Python.
* Bad, because according to the docs the WebSocket broadcast works only in the memory of one
  process; for several workers they refer to an external broadcast service. [verified, Q5]
* Bad, because a Python service is not a static binary; images and dependencies become
  larger. [derived]

### Elixir with Phoenix

* Good, because Phoenix Presence replicates process information in the cluster without a central
  point. [verified, Q6]
* Bad, because another language and runtime is added, for which less exists in the team and in the
  tools (code generation from OpenAPI, license gates). [derived]

## More Information

Sources, retrieved 2026-10-10:

* Q1 https://go.dev/doc/devel/release
* Q2 https://go.dev/blog/routing-enhancements
* Q3 https://github.com/coder/websocket (v1.8.15)
* Q4 https://opentelemetry.io/docs/languages/go/
* Q5 https://fastapi.tiangolo.com/advanced/websockets/
* Q6 https://phoenix.hexdocs.pm/presence.html
* Q7 https://github.com/oapi-codegen/oapi-codegen (v2.8.0)

Planning basis B2, R-Stack. [02-architecture.md](../spec/02-architecture.md), sections 5 and 6.
