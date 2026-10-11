---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Architecture team
informed: all planning teams
---

# Modular monolith instead of microservices

## Context and Problem Statement

Bothy is a self-hosted team chat in which agents are participants. The first instance runs
on a single host, later instances in a cloud. Every write request must store its
change and its event atomically (B8), every delivery checks permissions (B7), and one person
should be able to operate the instance. What basic shape does the server have?

## Decision Drivers

* Change and event in one transaction (commit-ordered event log, ADR 0009).
* Operation by one person: few parts, one backup, one upgrade step.
* Low latency between writing, permission check and delivery.
* Later growth without a rewrite (more connections, more jobs).
* Clear boundaries, so that many build sessions can work in parallel in their own worktrees.

## Considered Options

* Modular monolith: one binary, strict modules, roles `api`, `worker`, `all`
* Microservices per domain with their own database and a message broker
* Monolith without checked module boundaries
* Extend an existing chat server (for example a Matrix homeserver, Mattermost or Zulip)

## Decision Outcome

Chosen option: "Modular monolith", because it keeps change and event in one
PostgreSQL transaction, needs only one server container and still scales separately through
roles. The modules are the boundaries for parallel work and for a later split
(02-architecture.md, section 16).

### Consequences

* Good, because one deployment, one database and one backup are enough.
* Good, because roles (`api`, `worker`) scale separately without splitting the code.
* Good, because modules with ports and a dependency graph without cycles can later move behind a
  process boundary.
* Bad, because a memory leak or crash hits all modules of a process. Countermeasure:
  separation of the roles, recover per request, job and connection (02, 14.5).
* Bad, because the global counter `event_seq` is a shared bottleneck of all modules. Countermeasure:
  load test gate on `perf.event_commit_min_per_s` (02, section 17).
* Bad, because module boundaries only hold if a tool checks them (ADR 0008).

### Confirmation

* `go-arch-lint check` in the local gate (ADR 0008, 0038) holds the module boundaries.
* A role test starts `bothy serve` with every role and checks: `api` runs no jobs,
  `worker` opens no public listener, `all` does both.
* The release builds exactly one server image (18-release.md).

## Pros and Cons of the Options

### Modular monolith

* Good, because transactions across modules are possible without a distributed transaction.
* Good, because in-process fan-out works without a broker (R-Stack).
* Neutral, because the roles use the same image; the instance operator chooses only one parameter.
* Bad, because discipline with dependencies is needed (a tool instead of a convention).

### Microservices per domain

* Good, because parts can be deployed and scaled independently.
* Bad, because change and event across services stay consistent only with an outbox and a broker;
  the global, commit-ordered log from B8 would be a distributed bottleneck. [derived]
* Bad, because every permission check would go over the network or permissions would have to be
  replicated (B7 requires one single decision point). [derived]
* Bad, because operation on one host needs many containers, networking and backups.

### Monolith without checked module boundaries

* Good, because fastest at the start.
* Bad, because parallel build sessions without boundaries write into the same packages and read
  tables across modules; a later split becomes expensive. [derived]

### Extend an existing chat server

* Good, because basic chat functions already exist.
* Bad, because the data model, permissions and sync of the third-party system apply. The core
  objects of this product (run and live box, invocation token, invariant I1, one operator per
  channel) would be extensions outside the core, and permissions would have to be right in two
  places. [derived]
* Bad, because the apps (Flutter, B5) would have to speak a third-party client contract.

## More Information

* Planning basis B1, B3, B6, B7, B8, B12.
* [02-architecture.md](../spec/02-architecture.md), sections 3, 4 and 16.
* Related: ADR 0005 (Go), ADR 0008 (module rules), ADR 0009 (event log).
