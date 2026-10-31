// SPDX-License-Identifier: Apache-2.0

/// Local-only tables (15 6.1): `outbox`, `sync_state`, `loaded_ranges`, `file_cache`,
/// `share_inbox`, `forum_view_states`, `recent_targets`, `host_key_pins`, `channel_view_states`,
/// `space_view_states` and `shell_view_state`. Times are milliseconds since the epoch in UTC, so ordering and the
/// cleanup compare plain integers.
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

/// How the thread group of a channel looks in the sidebar on this device (01 6.8 items 3 and 7,
/// D-SB-6, D-M4-46-2): whether it is collapsed and whether a forum group shows every open post
/// ("Show more"). Kept per device and never synchronized, because 01 11 has no key for it (a new
/// setting key would need a server change for a view preference); a channel without a row shows
/// its group expanded with the limit. The removal of a channel deletes its row (04 8.3).
@DataClassName('ChannelViewStateRow')
class ChannelViewStates extends Table {
  @override
  String get tableName => 'channel_view_states';

  /// The channel, canonical UUID text.
  TextColumn get channelId => text()();

  /// Whether the thread group under the channel row is collapsed.
  BoolColumn get groupCollapsed => boolean()();

  /// Whether the forum group shows every open post instead of the newest
  /// `ui.sidebar_forum_posts_visible` ("Show more", M4-46).
  BoolColumn get postsShownInFull =>
      boolean().withDefault(const Constant(false))();

  @override
  Set<Column<Object>> get primaryKey => {channelId};
}

/// Whether a space is collapsed in the sidebar on this device (01 6.7, R-PR-24; 04 14,
/// R-SY-29; FIX-device-mirroring-2). Kept per device and never synchronized: a person expands a
/// space on one device to open a chat there, and the other device must not follow. A space
/// without a row is expanded. Version 10 fills the table once from `user_space_states.collapsed`,
/// so an upgrade changes nothing on screen. The removal of a space deletes its row (04 8.3).
@DataClassName('SpaceViewStateRow')
class SpaceViewStates extends Table {
  @override
  String get tableName => 'space_view_states';

  /// The space, canonical UUID text.
  TextColumn get spaceId => text()();

  /// Whether the space is collapsed.
  BoolColumn get collapsed => boolean()();

  @override
  Set<Column<Object>> get primaryKey => {spaceId};
}

/// How the desktop shell looks on this device (01 6.8 item 7, D-SB-6; 01 D1, D15): whether the
/// forum overview is pinned as its own column, whether the channel column next to an open thread
/// is hidden (M4-42), whether the session column shows in either layout (M4-45) and whether
/// code blocks wrap their long lines (M4-49). One row,
/// like `sync_state`; kept per device and never synchronized, because 01 11 has no key for it;
/// without the row the overview is not pinned.
@DataClassName('ShellViewStateRow')
class ShellViewStates extends Table {
  @override
  String get tableName => 'shell_view_state';

  /// Always 1: the table holds one row.
  // drift builds the CHECK from the column itself; the getter is not called recursively.
  // ignore: recursive_getters
  IntColumn get id => integer().check(id.equals(1))();

  /// Whether the forum overview D15 is pinned as a column next to the history.
  BoolColumn get overviewPinned =>
      boolean().withDefault(const Constant(false))();

  /// Whether the channel column next to an open thread is hidden, so the thread takes the
  /// whole content width (01 D1, D2; M4-42): the main history of a text channel, or the pinned
  /// overview of a forum.
  BoolColumn get channelColumnHidden =>
      boolean().withDefault(const Constant(false))();

  /// Whether the person hid the session column where it shows by default (01 D1, D2; M4-45):
  /// next to a forum post and next to a main history alone.
  BoolColumn get sessionColumnHidden =>
      boolean().withDefault(const Constant(false))();

  /// Whether the person showed the session column next to a thread of a text channel, where it
  /// is collapsed by default (01 D2; M4-45).
  BoolColumn get textThreadSessionShown =>
      boolean().withDefault(const Constant(false))();

  /// Whether the person switched the code blocks of messages to one line per line with a
  /// horizontal scroll ("Wrap lines" off, 01 6.14 item 1; M4-49); by default long lines wrap.
  BoolColumn get codeBlocksUnwrapped =>
      boolean().withDefault(const Constant(false))();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

/// The recently visited spaces, channels and threads of the quick switcher (brief M4-23 Design
/// note 3; 01 D6). Kept per device and never synchronized, because 01 11 has no key for them;
/// the removal of a target deletes its row (04 8.3).
@DataClassName('RecentTargetRow')
class RecentTargets extends Table {
  @override
  String get tableName => 'recent_targets';

  /// `space`, `channel` or `thread` (`RecentTargetKind.code`).
  TextColumn get kind => text()();

  /// Canonical UUID text of the target.
  TextColumn get targetId => text()();

  /// Time of the last visit.
  IntColumn get visitedAt => integer()();

  @override
  Set<Column<Object>> get primaryKey => {kind, targetId};
}

/// The pinned Noise keys of the hosts on this device (15 6.1, 23.3; 24 5.5; brief M4-36 Design
/// note 2). A pin is a trust anchor: every migration keeps its rows (`MigrationRules`), the cache
/// cleanup and the removal of targets never touch it, and only the wipe of all local data
/// deletes it with the database.
@DataClassName('HostKeyPinRow')
class HostKeyPins extends Table {
  @override
  String get tableName => 'host_key_pins';

  /// The host, canonical UUID text.
  TextColumn get hostId => text()();

  /// The static X25519 key of the host, base64url without padding as in `Host`.
  TextColumn get noisePublicKey => text()();

  /// `key_generation` of the host when the pin was written.
  IntColumn get keyGeneration => integer()();

  /// `key_fingerprint` of the host when the pin was written.
  TextColumn get keyFingerprint => text()();

  /// When this device first saw the key (after the first successful handshake).
  IntColumn get firstSeenAt => integer()();

  @override
  Set<Column<Object>> get primaryKey => {hostId};
}
