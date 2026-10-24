// SPDX-License-Identifier: Apache-2.0

/// Access to `recent_targets`: the recently visited spaces, channels and threads that the quick
/// switcher ranks first (brief M4-23 Design note 3; 01 D6). The list is kept per device and never
/// synchronized, because 01 11 has no key for it.
library;

import 'package:drift/drift.dart';

import '../database.dart';
import '../models.dart';

/// Data access object of the recently visited targets.
///
/// Example:
/// ```dart
/// await db.recentTargetsDao.visit(RecentTargetKind.thread, thread.id.value,
///     at: clock.now(), keep: 50);
/// db.recentTargetsDao.watchRecent(limit: 10).listen(print);
/// ```
final class RecentTargetsDao {
  /// Creates the DAO on a database; use `LocalDatabase.recentTargetsDao`.
  RecentTargetsDao(this._db);

  final LocalDatabase _db;

  /// Records a visit of [targetId] of [kind] at [at] and keeps only the newest [keep] rows.
  ///
  /// A target visited before moves to the top with the new time; the oldest rows beyond [keep]
  /// are deleted in the same transaction, so the table never grows past the caller's limit.
  Future<void> visit(
    RecentTargetKind kind,
    String targetId, {
    required DateTime at,
    required int keep,
  }) => _db.transaction(() async {
    await _db
        .into(_db.recentTargets)
        .insertOnConflictUpdate(
          RecentTargetsCompanion.insert(
            kind: kind.code,
            targetId: targetId,
            visitedAt: at.toUtc().millisecondsSinceEpoch,
          ),
        );
    // The same order as [watchRecent], so the rows kept are the rows shown.
    await _db.customUpdate(
      'DELETE FROM recent_targets WHERE rowid NOT IN '
      '(SELECT rowid FROM recent_targets ORDER BY visited_at DESC, rowid DESC LIMIT ?)',
      variables: [Variable<int>(keep)],
      updates: {_db.recentTargets},
      updateKind: UpdateKind.delete,
    );
  });

  /// The recent targets, newest first, at most [limit].
  Stream<List<RecentTarget>> watchRecent({required int limit}) => _db
      .customSelect(
        'SELECT kind, target_id, visited_at FROM recent_targets '
        'ORDER BY visited_at DESC, rowid DESC LIMIT ?',
        variables: [Variable<int>(limit)],
        readsFrom: {_db.recentTargets},
      )
      .watch()
      .map(
        (rows) => [
          for (final r in rows)
            RecentTarget(
              kind: RecentTargetKind.fromCode(r.read<String>('kind')),
              targetId: r.read<String>('target_id'),
              visitedAt: DateTime.fromMillisecondsSinceEpoch(
                r.read<int>('visited_at'),
                isUtc: true,
              ),
            ),
        ],
      );
}
