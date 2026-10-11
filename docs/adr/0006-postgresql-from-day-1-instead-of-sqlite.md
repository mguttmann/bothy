---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Architecture team
informed: all planning teams
---

# PostgreSQL from day 1 instead of SQLite

## Context and Problem Statement

The first instance runs on one host; there an embedded database would be the simplest.
The product, however, should also run in clouds, needs full-text search with a permission filter, a
commit-ordered event log with row locks (ADR 0009), notifications to other processes
and background jobs. Which database does the server use, and one or two?

## Decision Drivers

* One database path in the code and in the tests.
* Concurrent writers (messages, run patches, jobs, heartbeats).
* Full-text search, partitioning, row locks and `LISTEN/NOTIFY`.
* Portability: managed services in every cloud.
* A small installation on one host must still stay simple.

## Considered Options

* Only PostgreSQL
* Only SQLite
* Both behind a repository layer

## Decision Outcome

Chosen option: "Only PostgreSQL", because the event log, the permission filters and the search
need features that only PostgreSQL offers in one form, and because two database paths double every
query and every test. The target version is PostgreSQL 18; PostgreSQL 19 only after GA and its own test.

### Consequences

* Good, because one SQL dialect, one migration path, one test environment.
* Good, because `LISTEN/NOTIFY` serves as a wake-up signal without a broker (02, section 11).
* Good, because every cloud offers managed PostgreSQL.
* Bad, because even the smallest installation needs a PostgreSQL container. Countermeasure:
  Compose as a one-command installation (ADR 0012); in the Compose network PostgreSQL is not
  published to the outside, the port is configuration.
* Bad, because backup and upgrade of the database are tasks of the instance operator (16-operations.md).

### Confirmation

* Integration tests always run against a real PostgreSQL 18 (testcontainers-go), never against
  a substitute.
* The list of allowed third-party libraries (`modules.yaml`, ADR 0008) contains no SQLite driver.
* The product's Compose file publishes no PostgreSQL port on the host (16-operations.md).

## Pros and Cons of the Options

### Only PostgreSQL

* Good, because PostgreSQL 18 is current and maintained until 2030-11-14. [verified, Q1]
* Good, because `NOTIFY` delivers only at commit and follows the commit order. [verified, Q2]
* Good, because `websearch_to_tsquery` accepts raw input without syntax errors. [verified, Q3]
* Bad, because operation needs one more database service than a file.

### Only SQLite

* Good, because no separate database installation is needed.
* Bad, because SQLite allows any number of readers, but only one writer at any point in time.
  [verified, Q4]
* Bad, because for databases shared over the network the docs recommend a client-server database;
  several server nodes would thus be ruled out. [verified, Q4]
* Bad, because a second process (worker) would have no `LISTEN/NOTIFY`. [derived]

### Both behind a repository layer

* Good, because small installations could run without PostgreSQL.
* Neutral, because River supports both databases. [verified, Q5]
* Bad, because every query, every migration and every test would exist twice and the event log
  would need two locking models. [derived]

## More Information

Sources, retrieved 2026-10-10:

* Q1 https://www.postgresql.org/support/versioning/
* Q2 https://www.postgresql.org/docs/current/sql-notify.html
* Q3 https://www.postgresql.org/docs/current/textsearch-controls.html
* Q4 https://sqlite.org/whentouse.html
* Q5 https://github.com/riverqueue/river

Planning basis B3. [02-architecture.md](../spec/02-architecture.md), sections 8 and 11.
Related: ADR 0007, ADR 0009.
