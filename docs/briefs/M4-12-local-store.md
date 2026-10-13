# M4-12: Local database schema v1 (ADR 0029)

Status: ready | Depends on: M4-08 | Milestone: M4

## Goal

After this ticket `packages/local_store` is the encrypted local database of the apps (15 6, ADR 0029, R-CL-08): a drift database on SQLite3MultipleCiphers (package `sqlite3` with `source: sqlite3mc`, never the end-of-life `sqlcipher_flutter_libs`), schema version 1 with the mirror tables of 15 6.1 (only the columns a client sees, plus `version`) and the local-only tables `outbox`, `sync_state`, `loaded_ranges`, `file_cache`, `search_index` (FTS5) and `share_inbox`, data access objects that hand out the domain types of M4-08 and apply the version rule on every upsert, one-transaction deletion of removed targets (04 8.3) and of everything on revocation (`4012`, R-SY-14), the cleanup of 15 6.3 (`client.cache_max_age`, `client.cache_max_bytes`; never outbox, drafts or the share inbox), the drift migration workflow with stored schema files and generated migration tests, and the rule "outbox, drafts and share inbox survive every migration" as a test that every later schema version must pass. The database key is 32 random bytes from a key source port (secure storage lives in the app, 15 6.3). FTS5 and the license of the encryption library are checked and noted. Why: the interface reads only from this database (15 1 principle 2, R-CL-03), and a lost device must reveal nothing (T-DV-01).

## Read first

- `docs/PLAN.md`: section 3; section 10 (Risks: "license and FTS5 of the encryption library of the local database"; rows M4-12 to M4-16, M4-23, M4-36).
- `docs/spec/15-client-architecture.md`: 1 (principles 2, 3, 5), 3.1 (rule 1: `local_store` imports no Flutter; it depends only on `domain`), 4 (drift queries with `watch` feed the providers), 6 "Local database" with 6.1 "Schema mirror" (both tables), 6.2 "Migrations", 6.3 "Encryption and cleanup", 7.1 (the database runs in its own isolate), 22 (`client.cache_max_bytes`), 21 (R-CL-03, R-CL-08).
- `docs/spec/03-data-model.md`: every server table named in 15 6.1 (columns, enums, `version` columns); `user_settings`, `read_states`, `drafts` (`updated_by_device_id`, `version`), 6.5 (redaction: tombstones keep `thread_seq`).
- `docs/spec/04-sync-protocol.md`: 4 (gaps in `thread_seq`), 7.1 (cursor), 7.3 and 7.5 (`sync_epoch` stored with the cursor), 8.3 (what the client deletes on removal), 8.5, 9 (version rule and the types without `resource.version`), 13.1, 13.2 (outbox entry fields), 14 (personal data rules), 18 (`client.cache_max_age`).
- `docs/spec/14-search.md`: 6 "Offline search in the client" (FTS5 in `.drift` files, `bm25`, `highlight`, `snippet`; index content; deletion).
- `docs/spec/20-privacy.md`: the row "Client caches" of the deletion table.
- `docs/security/threat-model.md`: T-DV-01.
- ADR `0029-client-state-local-database-and-offline-queue.md`.
- `docs/DECISIONS.md`: D-W-100, D-W-114, D-W-115 (the client-side reading of `client.cache_max_age`); section 10.
- Briefs: `docs/briefs/M4-08-domain.md` (entities, `ResourceVersion`, `decideApply`, `LoadedRanges`, `AppConfig`), `docs/briefs/M4-01-workspace-arch.md` (generated code excluded from lint and coverage, `codegen-check`).
- drift documentation for `make-migrations`, `stepByStep`, `.drift` files with FTS5, `NativeDatabase.createInBackground`; the `sqlite3` package documentation of build hooks (`hooks: user_defines: sqlite3: source: sqlite3mc`) as cited in 15 6.

This ticket touches no API operation, no table of chapter 03, no action, no event and no configuration key.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. Limits only through named keys: `client.cache_max_bytes` and `client.cache_max_age` come from `AppConfig` (M4-08, D-W-115); no literal limit in code.
10. Specification first: 15 6.1 lists `host_key_pins`; this schema version leaves it out because M4-36 adds it with its own schema version (Out of scope); 15 6 gets the tokenizer choice of Design note 5.

Product requirements implemented here:

11. The interface reads only from this database; every write of the network lands here in a transaction (15 1 principle 2, 15 5).
12. Mirror tables carry only what a client may see: no channel `prompt`, no skills, no `client_payload_sha256`, nothing readable only with `manage` (15 6.1).
13. A durable change is stored only when its `resource.version` is greater than the stored one; types without `resource.version` are applied in `seq` order (04 9, 15 5).
14. The cursor and the sync epoch are stored in the same transaction as the page they belong to (15 7.3 item 1, 04 7.5).
15. Encrypted at rest with a 256-bit key that never lies in plain text on disk (15 6.3, R-CL-08).
16. Removed targets are deleted in one transaction, including their files in the cache, their search index rows and drafts (04 8.3, 14 6); `4012` deletes the whole database (R-SY-14).
17. Cleanup never touches outbox, drafts and share inbox (15 6.3); migrations keep them without loss (15 6.2).

## Scope

Create in `packages/local_store/`:

- `lib/src/database.dart` (`LocalDatabase` drift class, schema version 1), `lib/src/tables/` one file per table group (`structure.dart`, `threads_messages.dart`, `files.dart`, `runs_agents.dart`, `personal.dart`, `instance.dart`, `local_only.dart`), `lib/src/search.drift` (FTS5 table and queries, Design note 5).
- `lib/src/open.dart`: `openLocalStore({required File file, required DatabaseKeySource keys, bool inBackground = true})` and `openInMemoryForTests()`; `DatabaseKeySource` port (Design note 2).
- `lib/src/daos/`: `structure_dao.dart`, `threads_dao.dart`, `messages_dao.dart`, `runs_dao.dart`, `principals_dao.dart`, `commands_dao.dart`, `personal_dao.dart`, `instance_dao.dart`, `outbox_dao.dart`, `sync_state_dao.dart`, `file_cache_dao.dart`, `search_dao.dart`, `share_inbox_dao.dart` (Design note 3); `lib/src/removal.dart` (`removeTargets`, `wipe`), `lib/src/cleanup.dart` (`CacheCleaner`).
- `lib/src/mappers/` (rows to domain entities and back).
- `drift_schemas/local_store/drift_schema_v1.json` and the generated migration test scaffold from `dart run drift_dev make-migrations` (Design note 4); `build.yaml` with the drift options.
- `lib/local_store.dart` exports; tests under `test/` (Tests).

Change:

- `packages/local_store/pubspec.yaml` (`drift`, `sqlite3`, `domain`; dev: `drift_dev`, `build_runner`, `test`), `README.md` (tables, DAOs, key source, cleanup, migration rules, the license and FTS5 notes).
- Root `pubspec.yaml`: `hooks: user_defines: sqlite3: source: sqlite3mc` (Design note 2).
- Root `Makefile` `codegen-check`: the drift generation (`build_runner`) of `local_store` compared in a temporary copy, as M4-01 set up for generated Dart code.
- `docs/spec/15-client-architecture.md` 6 (the tokenizer, the cipher, where the user define lives), `docs/quality/licenses.md` (SQLite3MultipleCiphers: license and how it is bundled, if the file lists bundled native code).
- `docs/DECISIONS.md` section 10: rows `D-M4-12-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- The table `host_key_pins` (M4-36 adds it in a later schema version with its migration test).
- Applying pages and events, the dispatcher, the outbox sender and the run documents (M4-14, M4-15, M4-16); this ticket offers the storage they call.
- The secure storage implementation of the key source (`flutter_secure_storage`) and the file protection classes of iOS and Android (M4-18 for desktop, M5 for mobile).
- The file cache bytes on disk (M4-22 writes and reads them; this ticket keeps the metadata rows and returns the paths to delete).
- Search queries of the app and the merge with server results (M4-23).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-12-local-store`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `build(gates): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server unless a section below names a container. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH` (and Go and Docker where a section names them); without them the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout" ("Places that carry the product name"); the app shows `branding.product_name` from the instance, never a literal.
- No permission logic in the client (15 1 principle 1): controls follow server responses, snapshots and events, never an own derivation from grants.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M4 by D-W-114). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- Tests use an in-memory database where the test does not need a file; the encryption tests use a file in a temporary directory that the test deletes. No test opens a database outside a temporary directory.
- Never print or log the database key; tests use a fixed test key that is not a secret and is named as such.

## Design notes

1. **Tables.** Mirror tables named as in 15 6.1 with `lowerCamelCase` Dart getters and `snake_case` SQL columns equal to 03; primary key the server ID; a `version` integer column where the server table has one; no column 15 6.1 excludes. `channels` carries the effective operator from the API field `operator` as a column (`effective_operator_agent_id`, 15 6.1 "effective operator as a column") and, like `spaces`, the caller's `level` and `capabilities` from `ChannelView` and `SpaceView` of the snapshot and of `getChannel` and `getSpace` (the server's answer that the interface shows controls by, never derived from grants, 15 1 principle 1), `messages` carries `trigger_state`, `thread_seq`, `deleted_at` and the tombstone fields, `runs` only the summary from `run.state_changed`. Local columns: `last_accessed_at` on `threads` and `file_cache` (for the cleanup), `local_state` on `messages` only if the outbox display needs it (otherwise the outbox table alone carries pending messages). Local-only tables:
    - `outbox`: `id` (UUIDv7, the `client_msg_id`, `client_interaction_id` or `Idempotency-Key`), `kind` (operation name), `thread_id` (nullable for writes outside a thread), `depends_on` (nullable outbox ID, an upload before its message), `payload` (JSON text), `expected_version` (nullable), `state` (`pending`, `sending`, `waiting`, `failed`, `confirmed`), `attempts`, `next_attempt_at`, `last_error_code`, `last_error_reason`, `conflict_current` (JSON of the `current` revision of a `412`, nullable), `created_at`, `state_changed_at`; index on `(thread_id, created_at)`.
    - `sync_state`: one row: `cursor`, `sync_epoch`, `head_seq`, `permission_version`, `gateway_session_id`, `disconnected_at`, `last_snapshot_at`.
    - `loaded_ranges`: `thread_id`, `from_seq`, `to_seq` (merged on write through `LoadedRanges` of M4-08).
    - `file_cache`: `file_id`, `variant` (`original` or a thumbnail size), `path`, `size_bytes`, `last_accessed_at`.
    - `share_inbox`: `id` (the `client_msg_id` the extension created), `target_thread_id`, `payload`, `file_paths`, `created_at`.
    The schema has no foreign keys to server-owned rows that the server may delete first (the version rule and the removal handle consistency); within the local tables, cascade only where a parent is local.

2. **Encryption and opening.** `abstract interface class DatabaseKeySource { Future<Uint8List> obtainKey(); }` returns 32 bytes; the app creates them at the first start and keeps them in secure storage (M4-18). `openLocalStore` opens the file with `NativeDatabase.createInBackground` (own isolate, 15 7.1) and a `setup` callback that runs `PRAGMA key = "x'<64 hex>'"` as the first statement, then checks with `SELECT count(*) FROM sqlite_master` that the key opens the file, then `PRAGMA cipher` reports `chacha20` (the SQLite3MultipleCiphers default; record the cipher as a ruling); verify in the SQLite3MultipleCiphers documentation that the raw hex key form `x'<64 hex>'` applies to that cipher without a key derivation step, and use the form the documentation names if it differs; a wrong key raises `LocalStoreKeyError` without the key in the message. `hooks: user_defines: sqlite3: source: sqlite3mc` goes into the root `pubspec.yaml` because build hooks read user defines from the workspace root; verify that with the pinned SDK and record it. The hex form of the key exists only in the setup call and is not kept in a field.

3. **DAOs.** Each DAO returns domain types of M4-08, never drift row classes, and offers `watch...` streams for the providers of the app. Writes:
    - `upsertIfNewer(entity)` per mirror table: one statement that inserts or updates only when the incoming `version` is greater than the stored one (`INSERT ... ON CONFLICT DO UPDATE ... WHERE excluded.version > <table>.version`), returning whether it applied; for rows without a version the caller decides through `decideApply` of M4-08 and calls `put`.
    - `MessagesDao`: `pageBefore(threadId, ThreadSeq? before, int limit)`, `watchThread(threadId, {int limit})` ordered by `thread_seq`, `watchContiguous(threadId, {int limit})` that emits only the contiguous range of `loaded_ranges` ending at the newest loaded message plus the first gap (M4-14 shows a loading row there), `applyReactionSummary` and `applyPin` with absolute values (04 9 item 3), `tombstone(messageId)`.
    - `SyncStateDao.commitPage(Future<void> Function(LocalTxn) apply, {required GlobalSeq cursor, String? syncEpoch})`: runs `apply` and the cursor update in one transaction (15 7.3 item 1); `saveSession({gatewaySessionId, permissionVersion, headSeq})` and `markDisconnected(DateTime at)` store the session fields of M4-13 (each its own short transaction; neither touches the cursor or the epoch); `clearMirror(LocalTxn)` empties every mirror table and `loaded_ranges` and keeps `outbox`, `drafts`, `share_inbox` and `sync_state` (the resync of 04 7.3; drafts are personal data the snapshot does not replace).
    - `OutboxDao`: `enqueue`, `nextForThread(threadId)` (oldest non-final entry), `markState`, `updatePayload(id, payload)`, `delete(id)`, `recordFailure(id, {code, reason, conflictCurrent})`, `pendingThreads()`, `entriesFor(threadId)` for the display, `replaceWithServer(id)` on the echo; FIFO order by `created_at` then `id`; a new ID is always a new row (`enqueue`), never an update of the key.
    - `PersonalDao`: read state through `mergeReadState` of M4-08 (monotonic), drafts and settings with the rules of 04 14 (own echo by `updated_by_device_id` ignored by the caller; the DAO stores what it is given).
    - `SearchDao`: `indexMessage`, `indexThreadTitle`, `indexTranscript`, `removeForTargets`, `search(String query, {int limit})` with `bm25` and `snippet` (Design note 5).

4. **Migrations.** drift `make-migrations` with `build.yaml` (`databases: local_store: lib/src/database.dart`, `schema_dir: drift_schemas/`, `test_dir: test/drift/`); schema version 1 stored; `onUpgrade` uses `stepByStep`. Rule of 15 6.2 as a reusable helper `MigrationRules`: mirror tables may be dropped and recreated on any step, followed by `sync_state.cursor` reset so the next connection does a resync; `outbox`, `drafts`, `share_inbox` (and later `host_key_pins`) are migrated row by row. `test/drift/survival_test.dart` runs every pair of stored versions (`from < to`) with fixture rows in `outbox`, `drafts` and `share_inbox` and asserts that they are equal after the migration; with only version 1 it runs the round trip of version 1 (create, fill, close, reopen with the key, compare) and is the test M4-36 extends. Ruling for docs/DECISIONS.md.

5. **Offline search.** `search.drift` defines `search_index` as an FTS5 virtual table with columns `kind` (`message`, `thread_title`, `transcript`), `target_id`, `thread_id`, `channel_id`, `space_id`, `body`, all but `body` `UNINDEXED`, tokenizer `unicode61 remove_diacritics 2` (German and English without a stemmer, 14 6 accepts differing word stems); queries with `bm25(search_index)` ordering and `snippet`. A start check runs `SELECT sqlite_compileoption_used('ENABLE_FTS5')` and fails the open with a clear error if FTS5 is missing. Ruling for docs/DECISIONS.md: the tokenizer and the column set; 15 6 states it.

6. **Removal and wipe.** `removeTargets(List<TargetRef> removed)` (kinds `space` and `channel` as `AccessTargetKind` of `access.changed` names them, plus `thread` for `thread.moved_out` of M4-14; `host` entries are ignored here, M4-36 handles them): in one transaction delete the target rows and everything below them (channels of a space, threads of a channel, messages, attachments, reactions summaries, mentions, components, runs, drafts, read states, loaded ranges, search rows, file cache rows), and return the list of file cache paths so the caller deletes the bytes. `wipe()` closes the database and deletes the file and its `-wal` and `-shm` files; the app deletes keys and tokens (M4-18).

7. **Cleanup.** `CacheCleaner(LocalDatabase db, {required Duration maxAge, required int maxBytes, required DateTime Function() now})`: `run()` deletes threads whose `last_accessed_at` is older than `maxAge` with everything below them (as in note 6, but never drafts, outbox entries or share inbox rows), then, while the database file size plus the `file_cache` sum exceeds `maxBytes`, deletes the least recently accessed threads and file cache rows; returns the file paths to delete. Structure rows (spaces, channels, principals, commands, instance) are never cleaned because the sidebar needs them.

8. **License and FTS5 notes.** Check the license of SQLite3MultipleCiphers (MIT according to its repository) in the bundled sources the `sqlite3` hook builds and note that SQLite itself is in the public domain (bundled source, not a dependency the gate lists), run `make licenses` (the gate checks the Dart packages of the lock file; MIT is on `quality/licenses/allowed.txt`), and write the result and the FTS5 check into the README and, if it lists bundled native code, `docs/quality/licenses.md`.

9. **Rulings for docs/DECISIONS.md** (`D-M4-12-<n>`): the cipher and the key form (note 2); the user define in the root `pubspec.yaml` (note 2); conditional upsert in SQL (note 3); the migration helper and the survival test (note 4); the tokenizer (note 5); `host_key_pins` deferred to M4-36 (Specification first).

## Tests (write them first)

`packages/local_store/test/` (in-memory database unless noted; fixed test key):

- `schema_test.dart`: every table of 15 6.1 except `host_key_pins` exists with the documented columns; a column list per mirror table is compared with a list in the test that names the 03 source; no column named `prompt`, `skills` or `client_payload_sha256` exists.
- `encryption_test.dart` (file in a temporary directory): a database written with key A cannot be opened with key B (`LocalStoreKeyError`) nor read as plain SQLite (the header is not `SQLite format 3`); reopening with key A shows the rows; the error message does not contain the key.
- `upsert_version_test.dart`: an older or equal version does not overwrite; a newer one does; reactions and pins apply absolute values without touching `messages.version`.
- `commit_page_test.dart`: a page whose `apply` throws leaves neither rows nor the cursor changed; a successful page stores both; `clearMirror` empties the mirror and `loaded_ranges` and keeps outbox, drafts, share inbox and `sync_state`.
- `contiguous_test.dart`: `watchContiguous` hides messages after a gap until the range is loaded.
- `outbox_dao_test.dart`: FIFO per thread across interleaved threads; `depends_on` order; `updatePayload`, `delete` and `recordFailure` with a `412` revision; entries survive close and reopen.
- `removal_test.dart`: removing a channel deletes its threads, messages, search rows, drafts and file cache rows in one transaction and returns the paths; other channels stay; `wipe` deletes the files.
- `cleanup_test.dart` (fake clock): old threads go, accessed ones stay; the byte limit removes the least recently used first; outbox, drafts and share inbox are never touched.
- `search_test.dart`: FTS5 present; German words with umlauts and diacritics found without them; `snippet` marks the hit; removed targets are no longer found.
- `drift/` generated migration tests plus `survival_test.dart` (Design note 4).

Commands that must pass, from the repository root:

- `cd packages/local_store && dart test --coverage=coverage` (or `flutter test --coverage` if M4-01 runs pure Dart members that way)
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`, `make osv`
- `make check`

## Acceptance criteria

- [ ] Schema version 1 with every table of 15 6.1 except `host_key_pins`, mirror columns as visible to a client, stored schema file and generated migration test.
- [ ] The database is encrypted with SQLite3MultipleCiphers; a wrong key fails without leaking it; the key comes only from the key source port.
- [ ] Upserts respect the version rule; the cursor is stored in the page's transaction.
- [ ] Removal and wipe delete everything of the targets in one transaction; cleanup follows 15 6.3 and never touches outbox, drafts or share inbox; the survival test exists for every later version.
- [ ] FTS5 works with the chosen tokenizer; the license of the encryption library and the FTS5 check are noted; `make licenses` green.
- [ ] `local_store` imports no Flutter and only `domain` among the members (`make arch`); rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite the spec section.
- `packages/local_store/README.md` (English) as in Scope.
- 15 6 updated; rulings of Design note 9 in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The cipher and the tokenizer in use, the result of the FTS5 check and of the license check (with versions of `sqlite3`, SQLite and SQLite3MultipleCiphers), and the size of the empty encrypted database file.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
