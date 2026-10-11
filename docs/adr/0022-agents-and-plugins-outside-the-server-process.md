---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team agent platform
---

# Agents and plugins outside the server process

## Context and Problem Statement

The product should integrate AI agents of different origins (Hermes Agent, Claude Code, custom
agents) and integrations (code hosting, tickets, monitoring). Agent runtimes are large, written
in other languages, start tools with file system and network access and hold their own
credentials. Where do they run, and how do they talk to the server?

## Decision Drivers

- Security: A bug in or a compromise of an extension must not reach the server, its database and
  its keys (threat model, Baseline B6).
- Credentials: The server must not store Claude.ai credentials or subscription tokens (B10, I5).
- Permissions: Every action of an extension must go through `authorize` (B7).
- Free choice of language and location for extensions (Python for Hermes, Node.js for
  integrations).
- The public API should be used by real clients from day one.
- Operations: A restart of the server must not kill an agent turn, and vice versa.

## Considered Options

1. Separate processes that use only the public API (REST, gateway WebSocket, signed
   webhooks), as principals with a token, grants and scopes.
2. Plugins in the server process (Go plugins or an embedded WebAssembly runtime with
   host functions).
3. Sidecar processes with a private RPC protocol (for example gRPC) next to the public API.

## Decision Outcome

Chosen: option 1, because it is the only one that meets all drivers: a hard process boundary, no
credentials in the server, every action through `authorize`, free choice of language, and the
public API is tested by the reference adapters themselves.

### Consequences

- Good, because a compromised extension can do only as much as its token with grants and
  scopes.
- Good, because Hermes, the Claude Code runner and third-party agents use the same contract
  (docs/spec/09-agents.md); the Hermes adapter is one client among several.
- Good, because the server and the runtimes restart separately.
- Bad, because every call goes over the network (latency, retries, idempotency needed). The
  protocol therefore carries cursors, `ack`, client IDs and compare-and-swap (04).
- Bad, because live box patches need throttling and a batching window.

### Confirmation

- Architecture check in the gate: The server module imports neither the Go package `plugin` nor a
  WebAssembly runtime; otherwise the dependency rule (ADR 0008) fails.
- The contract tests (docs/spec/09-agents.md section 16) run with a fake agent, fake Hermes and
  fake Claude exclusively against the public API of the running server.
- A test counts all routes and gateway ops and requires one action from the catalogue for each
  (05).

## Pros and Cons of the Options

### Option 1: Separate processes through the public API

- Good, because the process boundary also separates memory, file system and network.
- Good, because extensions are possible in any language.
- Neutral, because the instance operator runs more processes (Compose services or separate
  hosts).
- Bad, because network errors and duplicate deliveries must be handled.

### Option 2: Plugins in the server process

- Good, because calls run without network and without serialization.
- Bad, because Go plugins are tied to compiler and dependency versions and offer no
  memory protection.
- Bad, because a WebAssembly sandbox cannot carry agent runtimes with tools, subprocesses and
  large Python ecosystems; every feature would need a host function.
- Bad, because credentials of the runtime would then be in the server process (B10).

### Option 3: Sidecar processes with private RPC

- Good, because the process boundary remains.
- Bad, because a second protocol emerges next to the public API that needs its own
  authorization, versioning and tests.
- Bad, because the public API is then not used by the most important clients, and
  gaps go unnoticed.

## More Information

- Baseline B6, B7, B10; specification docs/spec/09-agents.md, docs/spec/11-extensions.md.
- Multi-user isolation of dev runtimes is a separate decision (ADR 0046).
