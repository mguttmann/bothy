// SPDX-License-Identifier: Apache-2.0

/// Local-only tables (15 6.1): `outbox`, `sync_state`, `loaded_ranges`, `file_cache`,
/// `share_inbox` and `forum_view_states`. Times are milliseconds since the epoch in UTC, so ordering and the cleanup
/// compare plain integers.
library;

import 'package:drift/drift.dart';

/// Entries of the outbox (15 7.5, 04 13.2). `outbox` survives every migration (15 6.2).
@DataClassName('OutboxRow')
@TableIndex(name: 'outbox_thread_created', columns: {#threadId, #createdAt})
class Outbox extends Table {
  @override
  String get tableName => 'outbox';

  /// UUIDv7: the `client_msg_id`, `client_interaction_id` or `Idempotency-Key` (04 5).
  TextColumn get id => text()();

  /// Name of the operation, for example `postMessage`.
  TextColumn get kind => text()();

  /// Thread of the write; null for a write outside a thread.
  TextColumn get threadId => text().nullable()();

  /// Outbox entry that must be confirmed first (an upload before its message, 15 7.5).
  TextColumn get dependsOn => text().nullable()();

  /// Request body as JSON text.
  TextColumn get payload => text()();

  /// `If-Match` version of an edit, if any.
  IntColumn get expectedVersion => integer().nullable()();

  /// `pending`, `sending`, `waiting`, `failed` or `confirmed` (15 7.5).
  TextColumn get state => text()();

  /// Send attempts so far.
  IntColumn get attempts => integer().withDefault(const Constant(0))();

  /// Earliest next attempt after a backoff.
  IntColumn get nextAttemptAt => integer().nullable()();

  /// Problem code of the last failure.
  TextColumn get lastErrorCode => text().nullable()();

  /// Reason of the last failure.
  TextColumn get lastErrorReason => text().nullable()();

  /// JSON of the `current` revision of a `412` (04 13.3).
  TextColumn get conflictCurrent => text().nullable()();

  /// Creation time; FIFO order with `id` (15 7.5).
  IntColumn get createdAt => integer()();

  /// Time of the last state change.
  IntColumn get stateChangedAt => integer()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

/// The one row of the sync state (04 7.1, 7.5, 6.3).
@DataClassName('SyncStateRow')
class SyncStateTable extends Table {
  @override
  String get tableName => 'sync_state';

  /// Always 1: the table holds one row.
  // drift builds the CHECK from the column itself; the getter is not called recursively.
  // ignore: recursive_getters
  IntColumn get id => integer().check(id.equals(1))();

  /// Global `seq` of the last applied page (04 7.1); null before the first sync.
  IntColumn get cursor => integer().nullable()();

  /// Sync epoch stored with the cursor (04 7.5).
  TextColumn get syncEpoch => text().nullable()();

  /// `head_seq` of the last `ready` (04 6.3).
  IntColumn get headSeq => integer().nullable()();

  /// `permission_version` of the last `ready`.
  IntColumn get permissionVersion => integer().nullable()();

  /// Gateway session for `resume` (04 6.5).
  TextColumn get gatewaySessionId => text().nullable()();

  /// Time the connection was lost.
  IntColumn get disconnectedAt => integer().nullable()();

  /// Time of the last snapshot (04 7.3).
  IntColumn get lastSnapshotAt => integer().nullable()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

/// Loaded ranges of `thread_seq` per thread (04 4, 15 7.3 item 4).
@DataClassName('LoadedRangeRow')
class LoadedRangesTable extends Table {
  @override
  String get tableName => 'loaded_ranges';

  /// Thread.
  TextColumn get threadId => text()();

  /// First loaded position.
  IntColumn get fromSeq => integer()();

  /// Last loaded position.
  IntColumn get toSeq => integer()();

  @override
  Set<Column<Object>> get primaryKey => {threadId, fromSeq};
}

/// Metadata of the cached file bytes (15 6.1 `file_cache`); M4-22 writes the bytes.
@DataClassName('FileCacheRow')
@TableIndex(name: 'file_cache_last_accessed', columns: {#lastAccessedAt})
class FileCache extends Table {
  @override
  String get tableName => 'file_cache';

  /// The file.
  TextColumn get fileId => text()();

  /// `original` or a thumbnail variant.
  TextColumn get variant => text()();

  /// Path of the bytes in the file cache.
  TextColumn get path => text()();

  /// Size of the bytes on disk.
  IntColumn get sizeBytes => integer()();

  /// Last access, for the cleanup (15 6.3).
  IntColumn get lastAccessedAt => integer()();

  @override
  Set<Column<Object>> get primaryKey => {fileId, variant};
}

/// Share entries taken over from the extensions (15 12); survive every migration (15 6.2).
@DataClassName('ShareInboxRow')
class ShareInbox extends Table {
  @override
  String get tableName => 'share_inbox';

  /// The `client_msg_id` the extension created.
  TextColumn get id => text()();

  /// Thread the person chose, if any.
  TextColumn get targetThreadId => text().nullable()();

  /// Shared text and metadata as JSON text.
  TextColumn get payload => text()();

  /// Paths of the shared files as a JSON array.
  TextColumn get filePaths => text()();

  /// Time the extension created the entry.
  IntColumn get createdAt => integer()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

/// How a forum looks on this device (brief M4-19 Design note 4): whether its Done section is
/// collapsed. Kept per device because 01 11 has no personal setting for it (01 6.1); a forum
/// without a row shows the section collapsed.
@DataClassName('ForumViewStateRow')
class ForumViewStates extends Table {
  @override
  String get tableName => 'forum_view_states';

  /// The forum channel.
  TextColumn get channelId => text()();

  /// Whether the section "Done (n)" is collapsed.
  BoolColumn get doneCollapsed => boolean()();

  @override
  Set<Column<Object>> get primaryKey => {channelId};
}
