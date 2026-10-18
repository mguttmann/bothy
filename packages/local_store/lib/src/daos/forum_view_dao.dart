// SPDX-License-Identifier: Apache-2.0

/// Access to `forum_view_states`: how a forum looks on this device (brief M4-19 Design note 4;
/// docs/DECISIONS.md D-M4-19-4).
library;

import 'package:domain/domain.dart';

import '../database.dart';

/// Data access object of the local view state of forums.
///
/// Example:
/// ```dart
/// await db.forumViewDao.setDoneCollapsed(forum, collapsed: false);
/// db.forumViewDao.watchDoneCollapsed(forum).listen(print); // false
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
}
