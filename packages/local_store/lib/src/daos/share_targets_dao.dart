// SPDX-License-Identifier: Apache-2.0

/// The reads of the share sheet (brief M5-06 Design notes 1 and 7; 01 14
/// `client.share.recent_targets`): the threads the person last wrote into and the writable
/// channels, over the mirror tables with bound variables, so a schema change breaks here and
/// in the tests of this package, not at run time in the app.
///
/// A channel counts as writable when the server reported it so: the stored level of the
/// caller is `write` or `trigger`, it is not read-only, and it is a text or forum channel
/// (direct messages are no share target). No permission logic of the app's own (15 1
/// principle 1).
library;

import 'package:drift/drift.dart';

import '../database.dart';

/// The SQL condition of a writable text or forum channel `c`; constant text, no input.
const String _writable =
    "c.level IN ('write', 'trigger') AND c.read_only = 0 "
    "AND c.kind IN ('text', 'forum')";

/// A thread the person wrote into, with its writable channel.
typedef ShareRecentRow = ({
  String threadId,
  String channelId,
  String channelName,
  String? mainThreadId,
  String? title,
});

/// A writable channel.
typedef ShareChannelRow = ({
  String id,
  String name,
  String kind,
  String? mainThreadId,
});

/// Data access object of the share sheet's targets.
///
/// Example:
/// ```dart
/// final rows = await db.shareTargetsDao.recentWrites(me: me.value, max: 8);
/// ```
final class ShareTargetsDao {
  /// Creates the DAO on a database; use `LocalDatabase.shareTargetsDao`.
  ShareTargetsDao(this._db);

  final LocalDatabase _db;

  /// The last [max] threads [me] wrote a message into, newest first, one row per thread; the
  /// channel is the thread's (also after a move) and writable.
  Future<List<ShareRecentRow>> recentWrites({
    required String me,
    required int max,
  }) async {
    if (max <= 0) return const [];
    final rows = await _db
        .customSelect(
          'SELECT m.thread_id AS thread_id, c.id AS channel_id, '
          'c.name AS channel_name, c.main_thread_id AS main_thread_id, '
          't.title AS title, MAX(m.created_at) AS sent_at, MAX(m.id) AS last_id '
          'FROM messages m '
          'LEFT JOIN threads t ON t.id = m.thread_id '
          'JOIN channels c ON c.id = COALESCE(t.channel_id, m.channel_id) '
          'WHERE m.author_principal_id = ? AND $_writable '
          'GROUP BY m.thread_id '
          'ORDER BY sent_at DESC, last_id DESC '
          'LIMIT ?',
          variables: [Variable<String>(me), Variable<int>(max)],
          readsFrom: {_db.messages, _db.threads, _db.channels},
        )
        .get();
    return [
      for (final r in rows)
        (
          threadId: r.read<String>('thread_id'),
          channelId: r.read<String>('channel_id'),
          channelName: r.read<String>('channel_name'),
          mainThreadId: r.read<String?>('main_thread_id'),
          title: r.read<String?>('title'),
        ),
    ];
  }

  /// The writable channels by name (case-insensitive), then by ID.
  Future<List<ShareChannelRow>> writableChannels() async {
    final rows = await _db
        .customSelect(
          'SELECT c.id AS id, c.name AS name, c.kind AS kind, '
          'c.main_thread_id AS main_thread_id '
          'FROM channels c WHERE $_writable ORDER BY lower(c.name), c.id',
          readsFrom: {_db.channels},
        )
        .get();
    return [
      for (final r in rows)
        (
          id: r.read<String>('id'),
          name: r.read<String>('name'),
          kind: r.read<String>('kind'),
          mainThreadId: r.read<String?>('main_thread_id'),
        ),
    ];
  }

  /// One value on listen and after every change of the stored channels: their names, levels
  /// and `read_only` decide the targets (04 8.3).
  Stream<void> watchChannels() => _db
      .customSelect(
        'SELECT c.id, c.name, c.level, c.read_only, c.kind, c.main_thread_id '
        'FROM channels c',
        readsFrom: {_db.channels},
      )
      .watch()
      .map((_) {});
}
