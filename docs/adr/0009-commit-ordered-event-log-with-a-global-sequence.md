---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team Architecture
informed: all planning teams
---

# Commit-ordered event log with a global sequence

## Context and Problem Statement

After a pause, clients and agent adapters catch up on "everything from number X" (B8). A reader
must never skip an event in the process. A PostgreSQL sequence (`bigserial`) assigns numbers at the
call, not at the commit, and leaves gaps on rollback; a reader cannot tell a gap from a transaction
that is still open (04, 3.1). How is a number created whose order matches the order of
visibility?

## Decision Drivers

* No event is lost during catch-up.
* Simple reader: `WHERE seq > cursor ORDER BY seq`.
* Gapless numbers, so that a client detects missing data.
* Change and event atomic in one transaction.
* No additional system in operations.

## Considered Options

* Counter row with a row lock in the same transaction (pattern from 04, 3.2)
* `bigserial` plus a visibility boundary via transaction IDs (`pg_current_xact_id`, `pg_snapshot_xmin`)
* Logical Decoding (change data stream from the WAL)
* External broker with an outbox table

## Decision Outcome

Chosen option: "Counter row with a row lock", because it delivers gapless numbers in commit order,
needs only PostgreSQL and uses the same mechanism for `audit_seq` and `permission_version`
(03, R-DM-05). The counter step always comes last before the commit; the unit of work enforces this
(02, section 8).

### Consequences

* Good, because a reader sees all smaller numbers after every commit; a rollback leaves no
  gap (04, 3.2).
* Good, because the cursor of a client is a single number.
* Bad, because all transactions with events are serialized at the end; throughput is
  limited. Mitigation: counter step last, no external calls in transactions,
  load test gate `perf.event_commit_min_per_s`.
* Bad, because writing transactions with events must run in `READ COMMITTED`.
* Bad, because `NOTIFY` additionally takes an instance-wide lock at commit [verified, Q3]; it falls
  into the same serialized section.

### Confirmation

* Concurrency test: many parallel writers with random delays before the commit and
  random rollbacks; a reader reading along sees only gaplessly increasing numbers and misses
  none.
* Load test `perf/event_commit` holds `perf.event_commit_min_per_s` (02, section 17).
* The unit of work sets the isolation level explicitly; a test checks that write transactions
  with events never run in `REPEATABLE READ` or `SERIALIZABLE`.

## Pros and Cons of the Options

### Counter row with a row lock

* Good, because visibility and number order coincide: PostgreSQL makes a transaction
  visible before it releases its locks (04, 3.2).
* Bad, because a global serialization point arises.

### `bigserial` plus visibility boundary

* Good, because no lock serializes the commits.
* Good, because PostgreSQL 18 has the necessary functions: `pg_current_xact_id()` returns the
  transaction ID, `pg_snapshot_xmin()` the smallest one still open. [verified, Q1]
* Bad, because readers may only read up to the oldest open transaction of the whole database;
  a long transaction anywhere (maintenance, job) holds up every delivery. [derived]
* Bad, because the cursor becomes a pair of transaction ID and number, and numbers have gaps; 04
  requires gapless numbers. [derived]

### Logical Decoding

* Good, because changes are read without touching the write paths.
* Bad, because replication slots retain WAL and catalog rows, even without a connected reader;
  in the extreme case a shutdown due to transaction ID wraparound looms. [verified, Q2]
* Bad, because changes can arrive twice after a crash. [verified, Q2]
* Bad, because permission filters and payloads still need their own event table.
  [derived]

### External broker with an outbox table

* Good, because brokers are built for high throughput.
* Bad, because the outbox does not solve the ordering question but only moves it; a second
  system is added to operations and backup. [derived]

## More Information

Sources, retrieved 2026-10-10:

* Q1 https://www.postgresql.org/docs/18/functions-info.html (transaction ID and snapshot functions)
* Q2 https://www.postgresql.org/docs/current/logicaldecoding-explanation.html
* Q3 https://recall.ai/blog/postgres-listen-notify-does-not-scale

Planning basis B8. [04-sync-protocol.md](../spec/04-sync-protocol.md), section 3;
[03-data-model.md](../spec/03-data-model.md), section 6; [02-architecture.md](../spec/02-architecture.md),
sections 8 and 11.
