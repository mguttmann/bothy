// SPDX-License-Identifier: Apache-2.0

/// Access to threads and their tags (15 6.1 `threads`, `thread_tags`).
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../mappers/codec.dart';
import '../mappers/threads_messages.dart';
import 'upsert.dart';

/// Data access object of threads; hands out domain types only.
final class ThreadsDao {
  /// Creates the DAO on a database; use `LocalDatabase.threadsDao`.
  ThreadsDao(this._db);

  final LocalDatabase _db;

  /// Stores [thread] and its tags if its version is newer, in one transaction; returns whether
  /// it applied. A new row starts with `last_accessed_at` = now, so the cleanup counts its age
  /// from the time it reached the device.
  Future<bool> upsertIfNewer(Thread thread) => _db.transaction(() async {
    final insert = threadToCompanion(thread, _db.clock());
    final applied = await upsertIfNewerVersion(
      _db,
      _db.threads,
      insert,
      update: insert.copyWith(lastAccessedAt: const Value.absent()),
      version: (t) => t.version,
    );
    if (applied) {
      await (_db.delete(
        _db.threadTags,
      )..where((t) => t.threadId.equals(thread.id.value))).go();
      await _db.batch(
        (b) => b.insertAll(_db.threadTags, [
          for (final tag in thread.tagIds)
            ThreadTagsCompanion.insert(
              threadId: thread.id.value,
              tagId: tag.value,
            ),
        ]),
      );
    }
    return applied;
  });

  /// The thread [id] with its tags, or null.
  Future<Thread?> byId(ThreadId id) async {
    final row = await (_db.select(
      _db.threads,
    )..where((t) => t.id.equals(id.value))).getSingleOrNull();
    return row == null ? null : threadFromRow(row, await _tags(row.id));
  }

  /// The thread [id], emitted on every change of the thread or its tags.
  Stream<Thread?> watchThread(ThreadId id) => _db
      .customSelect('SELECT 1', readsFrom: {_db.threads, _db.threadTags})
      .watch()
      .asyncMap((_) => byId(id));

  /// The threads of [channelId], latest activity first (post list, 03 `threads`).
  Stream<List<Thread>> watchForChannel(ChannelId channelId) => _db
      .customSelect('SELECT 1', readsFrom: {_db.threads, _db.threadTags})
      .watch()
      .asyncMap((_) async {
        final rows =
            await (_db.select(_db.threads)
                  ..where((t) => t.channelId.equals(channelId.value))
                  ..orderBy([
                    (t) => OrderingTerm(
                      expression: t.lastActivityAt,
                      mode: OrderingMode.desc,
                    ),
                  ]))
                .get();
        return [
          for (final row in rows) threadFromRow(row, await _tags(row.id)),
        ];
      });

  /// Records that the person opened thread [id] now; the cleanup keeps recently opened threads
  /// (15 6.3).
  Future<void> touch(ThreadId id) async {
    await (_db.update(_db.threads)..where((t) => t.id.equals(id.value))).write(
      ThreadsCompanion(lastAccessedAt: Value(toMillis(_db.clock()))),
    );
  }

  Future<List<ForumTagId>> _tags(String threadId) async {
    final rows =
        await (_db.select(_db.threadTags)
              ..where((t) => t.threadId.equals(threadId))
              ..orderBy([(t) => OrderingTerm(expression: t.tagId)]))
            .get();
    return [for (final row in rows) ForumTagId.parse(row.tagId)];
  }
}
