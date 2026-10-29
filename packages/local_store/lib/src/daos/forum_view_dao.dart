// SPDX-License-Identifier: Apache-2.0

/// Access to `forum_view_states`, `channel_view_states` and `shell_view_state`: how forums and
/// channel groups look on this device (brief M4-19 Design note 4, docs/DECISIONS.md D-M4-19-4;
/// 01 6.8 item 7, D-SB-6), and whether the channel column next to an open thread is hidden (01
/// D1, D2; M4-42). Nothing of it is synchronized: 01 11 has no key for it.
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';

/// Data access object of the local view state of forums, channel groups and the forum overview.
///
/// Example:
/// ```dart
/// await db.forumViewDao.setDoneCollapsed(forum, collapsed: false);
/// db.forumViewDao.watchDoneCollapsed(forum).listen(print); // false
/// await db.forumViewDao.setGroupCollapsed(forum, collapsed: true);
/// db.forumViewDao.watchCollapsedGroups().listen(print); // {forum}
/// await db.forumViewDao.setOverviewPinned(pinned: true);
/// print(await db.forumViewDao.overviewPinned()); // true
/// await db.forumViewDao.setChannelColumnHidden(hidden: true);
/// print(await db.forumViewDao.channelColumnHidden()); // true
/// ```
final class ForumViewDao {
  /// Creates the DAO on a database; use `LocalDatabase.forumViewDao`.
  ForumViewDao(this._db);

  final LocalDatabase _db;

  /// Whether the Done section of [forum] is collapsed; true without a stored row (01 6.1:
  /// collapsed by default).
  Stream<bool> watchDoneCollapsed(ChannelId forum) =>
      (_db.select(_db.forumViewStates)
            ..where((t) => t.channelId.equals(forum.value)))
          .map((row) => row.doneCollapsed)
          .watchSingleOrNull()
          .map((collapsed) => collapsed ?? true);

  /// Stores whether the Done section of [forum] is [collapsed].
  Future<void> setDoneCollapsed(
    ChannelId forum, {
    required bool collapsed,
  }) async {
    await _db
        .into(_db.forumViewStates)
        .insertOnConflictUpdate(
          ForumViewStatesCompanion.insert(
            channelId: forum.value,
            doneCollapsed: collapsed,
          ),
        );
  }

  /// The channels whose thread group in the sidebar is collapsed on this device, emitted on
  /// every change; a channel without a stored row is expanded (01 6.8 item 7: expanded by
  /// default).
  Stream<Set<ChannelId>> watchCollapsedGroups() =>
      (_db.select(_db.channelViewStates)
            ..where((t) => t.groupCollapsed.equals(true)))
          .map((row) => ChannelId.parse(row.channelId))
          .watch()
          .map((channels) => channels.toSet());

  /// Stores whether the thread group of [channel] is [collapsed] on this device.
  Future<void> setGroupCollapsed(
    ChannelId channel, {
    required bool collapsed,
  }) async {
    await _db
        .into(_db.channelViewStates)
        .insertOnConflictUpdate(
          ChannelViewStatesCompanion.insert(
            channelId: channel.value,
            groupCollapsed: collapsed,
          ),
        );
  }

  /// Whether the forum overview is pinned as its own column on this device, emitted on every
  /// change; false without the stored row (01 D15: not pinned by default).
  Stream<bool> watchOverviewPinned() => _db
      .select(_db.shellViewStates)
      .map((row) => row.overviewPinned)
      .watchSingleOrNull()
      .map((pinned) => pinned ?? false);

  /// Whether the forum overview is pinned, read once (the shell reads it at start, so a restart
  /// restores the column; brief M4-40 Design note 4); false without the stored row.
  Future<bool> overviewPinned() async =>
      await _db
          .select(_db.shellViewStates)
          .map((row) => row.overviewPinned)
          .getSingleOrNull() ??
      false;

  /// Stores whether the forum overview is [pinned] as its own column on this device.
  Future<void> setOverviewPinned({required bool pinned}) async {
    await _db
        .into(_db.shellViewStates)
        .insertOnConflictUpdate(
          ShellViewStatesCompanion.insert(
            id: const Value(1),
            overviewPinned: Value(pinned),
          ),
        );
  }

  /// Whether the channel column next to an open thread is hidden on this device, emitted on
  /// every change; false without the stored row (01 D1, D2: shown by default).
  Stream<bool> watchChannelColumnHidden() => _db
      .select(_db.shellViewStates)
      .map((row) => row.channelColumnHidden)
      .watchSingleOrNull()
      .map((hidden) => hidden ?? false);

  /// Whether the channel column next to an open thread is hidden, read once; false without the
  /// stored row.
  Future<bool> channelColumnHidden() async =>
      await _db
          .select(_db.shellViewStates)
          .map((row) => row.channelColumnHidden)
          .getSingleOrNull() ??
      false;

  /// Stores whether the channel column next to an open thread is [hidden] on this device; the
  /// pinned state of the overview stays.
  Future<void> setChannelColumnHidden({required bool hidden}) async {
    await _db
        .into(_db.shellViewStates)
        .insertOnConflictUpdate(
          ShellViewStatesCompanion.insert(
            id: const Value(1),
            channelColumnHidden: Value(hidden),
          ),
        );
  }
}
