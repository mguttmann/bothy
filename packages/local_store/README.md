# local_store

The encrypted local database of the apps: drift on SQLite3MultipleCiphers, schema version 9
with the mirror tables and the local-only tables, data access objects that hand out the domain
types of `packages/domain`, removal and wipe, the cache cleanup and offline search
(docs/spec/15-client-architecture.md 6, ADR 0029, R-CL-08). The interface reads only from this
database (15 1 principle 2). Kind: pure Dart package, a member of the Dart workspace at the
repository root (`resolution: workspace`, `publish_to: none`).

## Dependencies

Allowed members (docs/spec/15-client-architecture.md, 3.1; `tools/arch-check/rules.yaml`): `domain` (arrows of the diagram).
Every other member is out of reach; `make arch` fails on a dependency or an import of it.
External packages: `drift`, `sqlite3`; dev: `drift_dev`, `build_runner`, `test`, `lints`.

Further rules checked by `make arch`:

- no import of `package:flutter/`, `dart:ui` (rule 1).
- no native source (rule 4).

## Opening and the key source

`openLocalStore(file:, keys:, inBackground: true)` opens the file in its own isolate
(`NativeDatabase.createInBackground`, 15 7.1). The key comes only from the port
`DatabaseKeySource.obtainKey()`, which returns 32 random bytes; the app creates them at the first
start and keeps them in secure storage (M4-18 for desktop, M5 for mobile). The first statement on
the connection is `PRAGMA key = "x'<64 hex>'"`, the raw key form that SQLite3MultipleCiphers
takes without a key derivation; then the opener checks `SELECT count(*) FROM sqlite_master`
(a wrong key raises `LocalStoreKeyError`, whose message never contains the key), that
`PRAGMA cipher` reports `chacha20` and that FTS5 is compiled in (otherwise
`LocalStoreSetupError`). The hex text exists only inside the setup call. `openInMemoryForTests()`
opens an unencrypted in-memory database with the same feature checks.

The cipher library is selected in the root `pubspec.yaml`, because build hooks read user
defines only from the workspace root:

```yaml
hooks:
  user_defines:
    sqlite3:
      source: sqlite3mc
```

## Tables (schema version 9)

| Group | File | Tables |
|---|---|---|
| Structure | `lib/src/tables/structure.dart` | `spaces`, `user_space_states`, `channels` (effective operator, caller's `level` and `capabilities`, the derived `project_bound` of the API `Channel` since version 6, M4-40, R-DM-54; no `prompt`), `channel_operators`, `forum_tags` |
| Threads and messages | `lib/src/tables/threads_messages.dart` | `threads` (local `last_accessed_at`), `thread_tags`, `messages` (`trigger_state`, `thread_seq`, tombstone fields, no `client_payload_sha256`), `reactions_summary`, `my_mentions`, `message_components` |
| Files | `lib/src/tables/files.dart` | `files`, `message_attachments` |
| Runs and agents | `lib/src/tables/runs_agents.dart` | `runs` (summary of `run.state_changed`), `agents`, `principals` |
| Personal | `lib/src/tables/personal.dart` | `read_states`, `drafts`, `user_settings`, `notification_preferences`, `quiet_hours` |
| Instance | `lib/src/tables/instance.dart` | `instance` (with the app lock policy of `getInstance` in `app_lock` since version 5, M5-08; null in a row of an earlier version until the next `getInstance`), `commands` |
| Local only | `lib/src/tables/local_only.dart` | `outbox`, `sync_state`, `loaded_ranges`, `file_cache`, `share_inbox`, `forum_view_states` (version 2, M4-19: the Done section per forum), `recent_targets` (version 3, M4-23: the recently visited spaces, channels and threads of the quick switcher, per device, never synchronized), `host_key_pins` (version 4, M4-36: the pinned Noise keys of the hosts, per device, a trust anchor), `channel_view_states` and `shell_view_state` (version 6, M4-40: the collapsed thread group per channel and the one row with the pinned forum overview, per device, never synchronized; 01 6.8 item 7, D-SB-6; version 7, M4-42: `shell_view_state.channel_column_hidden`, whether the channel column next to an open thread is hidden, 01 D1, D2; version 8, M4-45: `shell_view_state.session_column_hidden` and `text_thread_session_shown`, whether the session column is hidden beside a forum post or main history and shown beside a thread of a text channel, 01 D1, D2; version 9, M4-46: `channel_view_states.posts_shown_in_full`, whether a forum group lists every open post after "Show more", 01 6.8 item 3) |
| Search | `lib/src/search.drift` | `search_index` (FTS5) |

`host_key_pins` of 15 6.1 came with version 4 (M4-36); from there on it is a preserved table of
`MigrationRules` like `outbox`, `drafts` and `share_inbox` (D-M4-36-2). The
schema has no foreign keys to server-owned rows; the version rule and the removal keep it
consistent. `test/schema_test.dart` compares the columns of every mirror table with chapter 03.

## Data access objects

Each DAO hands out domain types, never drift rows, and offers `watch...` streams for the
providers. Mirror writes are `upsert...IfNewer`: one statement
`INSERT ... ON CONFLICT DO UPDATE ... WHERE excluded.version > <table>.version` that reports
whether it applied (04 9); reactions and pins carry absolute values and leave
`messages.version` alone.

| DAO | Purpose |
|---|---|
| `StructureDao`, `ThreadsDao`, `MessagesDao`, `RunsDao`, `PrincipalsDao`, `CommandsDao`, `InstanceDao` | mirror tables; `MessagesDao.watchContiguous` emits only the contiguous range of `loaded_ranges` that ends at the newest loaded message plus the first gap |
| `PersonalDao` | read states through `mergeReadState` (monotonic), drafts, settings, notification preferences, quiet hours |
| `SyncStateDao` | `commitPage` stores a page and the cursor with the sync epoch in one transaction; `saveSession`, `markDisconnected`, `markSnapshot`, `resetCursor`; `clearMirror` for a resync |
| `OutboxDao` | FIFO per thread by `created_at`, then `id`; `depends_on`; failures with the `current` revision of a `412` |
| `FileCacheDao`, `ShareInboxDao`, `SearchDao` | file cache metadata, share inbox, offline search; `SearchDao.watchTranscriptsToIndex` lists the done transcripts of attached voice files without an index row (the app fetches the text), `pruneTranscripts` drops the rows of files no stored message attaches any more |
| `HostKeyPinsDao` | the pin of a host (`read`, `write` replaces, `all`); nothing here deletes a pin (M4-36) |
| `ForumViewDao`, `RecentTargetsDao` | local view state per device: the Done section of a forum, the collapsed thread groups (`watchCollapsedGroups`, `setGroupCollapsed`; expanded without a row) and the pinned forum overview (`watchOverviewPinned`, `overviewPinned`, `setOverviewPinned`; not pinned without the row); the recently visited targets (`visit` keeps the newest `keep` rows, `watchRecent` newest first) |

## Removal, wipe and cleanup

- `removeTargets(List<TargetRef>)` deletes spaces, channels and threads with everything below
  them (messages, attachments, reactions, mentions, components, runs, drafts, read states, loaded
  ranges, search rows, file cache rows, recently visited rows, the collapsed thread group of a
  channel) in one transaction and returns the file cache paths, so
  the caller deletes the bytes (04 8.3). Outbox entries stay; they fail with `404`. Host targets
  are ignored and the pins of `host_key_pins` stay.
- `wipe()` closes the database and deletes the file with its `-wal` and `-shm` files (`4012`,
  R-SY-14); the app deletes keys and tokens.
- `CacheCleaner` (limits `client.cache_max_age` and `client.cache_max_bytes` from `AppConfig`)
  deletes threads and file cache rows without access since the maximum age, then the least
  recently used until the database plus the file cache fit the byte limit. It never touches
  outbox, drafts, the share inbox or the pinned host keys, and never the structure rows the
  sidebar needs (15 6.3). Only `wipe()` deletes the pins, with the database.

## Migrations

Code generation: `dart run build_runner build` writes `lib/src/database.g.dart`;
`dart run drift_dev make-migrations` (options in `build.yaml`) stores the schema of each version
under `drift_schemas/local_store/` and the schema helpers under `test/drift/local_store/generated/`.
`make codegen-check` runs both and fails when a file differs from a fresh generation, so a table
change without a new schema version fails. drift writes the schema file without a final newline;
the end-of-file hook excludes `drift_schemas/`.

Rules for every later version (15 6.2): `onUpgrade` uses `stepByStep` with the steps from
`MigrationRules`. A mirror table may be discarded (`MigrationRules.discardCache` drops and
recreates it and resets the cursor, so the next connection loads a snapshot; a step that adds a
mirror column only the snapshot fills calls `MigrationRules.resetCursor` alone, as version 6 does
for `channels.project_bound`); `outbox`, `drafts`
and `share_inbox` are migrated row by row (`MigrationRules.keepRows`). `test/drift/survival_test.dart`
migrates every pair of stored versions with fixture rows in those tables and compares them; a new
version adds its fixture rows there.

## Encryption library, license and FTS5

Measured with the pinned SDK: package `sqlite3` 3.7.1 loads SQLite 3.54.0 as
SQLite3MultipleCiphers 2.6.1 with the default cipher `chacha20` (ChaCha20-Poly1305).
The `sqlite3` build hook downloads a prebuilt library from the releases of the `sqlite3`
package and checks it against the sha256 hashes in its Dart sources; no C source is compiled in
the workspace. SQLite3MultipleCiphers is under the MIT license (its repository's `LICENSE`);
SQLite itself is in the public domain. Neither is a package of the lock file, so `make licenses`
does not list them; see `docs/quality/licenses.md` "Bundled native code". FTS5 is compiled in
(`sqlite_compileoption_used('ENABLE_FTS5')` is 1); the opener fails without it. `search_index`
uses the tokenizer `unicode61 remove_diacritics 2` without a stemmer, so German and English words
match without their diacritics (14 6).

## Layout

| Path | Content |
|---|---|
| `lib/local_store.dart` | the public library; every public symbol has a doc comment (`make lint`, `public_member_api_docs`) |
| `lib/src/` | database, tables, DAOs, mappers, removal, cleanup, migration rules, opening |
| `drift_schemas/` | stored schema files, one per version |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`); `test/drift/` the migration tests |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
