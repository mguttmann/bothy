---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team apps and design
consulted: Team architecture, Team API
informed: all teams
---

# Client state, local database and offline queue

## Context and Problem Statement

The apps must read and write offline, catch up without gaps after pauses, avoid duplicates and
draw live box patches smoothly ([04-sync-protocol.md](../spec/04-sync-protocol.md)). Permissions
apply at delivery time; a revocation must remove local data immediately. How do the apps hold
their state, where do they store it, and how do writes wait for the network?

## Decision Drivers

- one path for online, offline and catch-up
- a reactive user interface that redraws only what changes
- transactional application of event pages and cursor
- encrypted local data, migrations with tests
- testability without a device (fake clock, fake transport)
- maintenance of the packages and licenses

## Considered Options

State:

1. Riverpod 3 with code generation
2. BLoC (`flutter_bloc`)
3. Provider with `ChangeNotifier`

Local database:

1. drift on SQLite with SQLite3MultipleCiphers
2. Isar
3. Hive or a key-value store
4. sembast (file-based documents)

Outbox:

1. Table in the same database, FIFO per thread
2. Queue in memory with a file backup

## Decision Outcome

Chosen option: "Riverpod 3, drift with SQLite3MultipleCiphers, outbox as a table", because
drift combines relational queries with reactive streams, transactions, generated migration tests
and encryption, and Riverpod brings these streams to the user interface without a widget context,
testable and with automatic disposal. The outbox in the same database lets entry, echo and cursor
be reconciled in one transaction.

Evidence (retrieved 2026-10-10): `flutter_riverpod` 3.4.3, MIT (https://pub.dev/packages/flutter_riverpod)
[verified]; `drift` 2.35.2 for Android, iOS, macOS, Windows, Linux, web
(https://pub.dev/packages/drift) [verified]; migration workflow `make-migrations` with generated
tests (https://drift.simonbinder.eu/migrations/) [verified]; `sqlite3` bundles SQLite via build
hooks and supports `source: sqlite3mc` (https://github.com/simolus3/sqlite3.dart/blob/main/sqlite3/doc/hook.md)
[verified].

### Consequences

- Good, because the user interface reads only from the database and all network paths converge
  there.
- Good, because event page, cursor and confirmation of the outbox are atomic.
- Good, because a revocation is one transaction that deletes rows, search index and file cache
  references.
- Good, because migrations have generated tests per schema version.
- Bad, because code generation (drift, Riverpod) costs build time.
- Bad, because SQLite3MultipleCiphers has its own license terms; the license gate must know it.
- Neutral, because the web console uses no local database (ADR 0028).

### Confirmation

- Widget tests check that screens read only through providers (no direct API call from widgets;
  lint rule in the gate).
- Unit tests of the `sync_engine`: duplicates, gaps, `resync_required`, revocation, echo of the
  outbox.
- drift migration tests plus the test "outbox and drafts survive every migration".
- Integration tests with a fake server (15-client-architecture.md, section 20).

## Pros and Cons of the Options

### Riverpod 3

- Good, because dependencies without `BuildContext`, `family` per ID, `autoDispose` matching the
  focus.
- Good, because overrides in tests.
- Bad, because code generation and its own way of thinking.

### BLoC

- Good, because a clear separation of event and state, widespread.
- Bad, because a lot of boilerplate for many small, ID-bound streams (per thread, per run).

### Provider with ChangeNotifier

- Good, because simple.
- Bad, because coarse-grained redrawing and weak testability of large states.

### drift on SQLite

- Good, because relational queries (unread counter, post list by activity), streams,
  transactions, FTS5 for offline search, encryption.
- Bad, because the SQL schema must be maintained (a mirror of the server schema).

### Isar

- Good, because fast, simple objects.
- Bad, because the last stable version on pub.dev is 3.1.0+1 from 2023-04-25 (pub.dev API,
  retrieved 2026-10-10 [verified]); no relations in the SQL sense.

### Hive or key-value

- Good, because very simple; the continuation `hive_ce` is maintained (pub.dev API, retrieved
  2026-10-10 [verified]).
- Bad, because no queries for counters and lists, no transactions across several objects.

### sembast

- Good, because pure Dart, runs everywhere.
- Bad, because the whole file is in memory; unsuitable for large histories.

### Outbox as a table

- Good, because atomic with echo and cursor; survives crashes.

### Outbox in memory

- Bad, because crashes lose entries or need a second, non-transactional backup.

## More Information

- Schema mirror, encryption, cleanup: [15-client-architecture.md](../spec/15-client-architecture.md), section 6.
- Sync engine and states: ibid., section 7.
- Normative rules for the outbox and reconciliation: [04-sync-protocol.md](../spec/04-sync-protocol.md), sections 9, 13, 14.
