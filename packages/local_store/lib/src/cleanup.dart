// SPDX-License-Identifier: Apache-2.0

/// The cache cleanup of 15 6.3: threads and file cache rows without access since
/// `client.cache_max_age` go, then the least recently used until the database plus the file
/// cache fit into `client.cache_max_bytes`. Outbox, drafts, share inbox and the pinned host keys
/// (`host_key_pins`, M4-36: a lost pin would prevent the warning on a changed key) are never
/// touched; structure rows (spaces, channels, principals, commands, instance) stay for the
/// sidebar, and so does an open thread without access for `client.cache_max_age` with what the
/// sidebar reads of it; only its content goes: the sidebar lists the open threads the device
/// holds (01 6.8 item 2), and a connection that continues from a cursor brings none of them
/// back.
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import 'cascade.dart';
import 'database.dart';
import 'mappers/codec.dart';

/// Removes old and excess cache entries from a [LocalDatabase].
final class CacheCleaner {
  /// Creates a cleaner with the limits [maxAge] (`client.cache_max_age`) and [maxBytes]
  /// (`client.cache_max_bytes`) and the clock [now].
  CacheCleaner(
    this._db, {
    required this.maxAge,
    required this.maxBytes,
    required DateTime Function() now,
  }) : _now = now; // ignore: prefer_initializing_formals

  /// A cleaner with the limits of [config] (D-W-115: both keys come from `AppConfig`).
  factory CacheCleaner.fromConfig(
    LocalDatabase db,
    AppConfig config, {
    required DateTime Function() now,
  }) => CacheCleaner(
    db,
    maxAge: config.cacheMaxAge,
    maxBytes: config.cacheMaxBytes,
    now: now,
  );

  final LocalDatabase _db;
  final DateTime Function() _now;

  /// Entries without access for longer are removed.
  final Duration maxAge;

  /// Upper limit of the database plus the file cache in bytes.
  final int maxBytes;

  /// Runs the cleanup and returns the paths of the deleted file cache rows; the caller deletes
  /// the bytes (M4-22). Each deletion is its own transaction, so a long cleanup never blocks the
  /// sync for long.
  ///
  /// A thread counts as accessed while the person has it on screen (`ThreadsDao.touch`) and
  /// when its row reaches the device. Without access for [maxAge] a done thread goes with
  /// everything below it. An open thread loses its content (messages with their reactions,
  /// components, mentions, attachments and files, wake-ups, loaded ranges and their search
  /// rows) and keeps its row with what the sidebar reads of it (tags, read state, seen mark,
  /// notification preference, runs, the title in the search index); its age then starts
  /// again, like that of a row that just reached the device. The byte limit afterwards
  /// removes whole threads, open ones too.
  Future<List<String>> run() async {
    final paths = <String>[];
    final cutoff = toMillis(_now().subtract(maxAge));
    final old = await _db
        .customSelect(
          'SELECT id, state FROM threads WHERE last_accessed_at < ?',
          variables: [Variable<int>(cutoff)],
        )
        .get();
    for (final row in old) {
      final id = row.read<String>('id');
      paths.addAll(
        row.read<String>('state') == ThreadState.open.code
            ? await _emptyThread(id)
            : await _deleteThread(id),
      );
    }
    paths.addAll(
      await _db.transaction(() async {
        final rows = await _db
            .customSelect(
              'SELECT path FROM file_cache WHERE last_accessed_at < ?',
              variables: [Variable<int>(cutoff)],
            )
            .get();
        await _db.customUpdate(
          'DELETE FROM file_cache WHERE last_accessed_at < ?',
          variables: [Variable<int>(cutoff)],
          updates: {_db.fileCache},
          updateKind: UpdateKind.delete,
        );
        return [for (final r in rows) r.read<String>('path')];
      }),
    );
    while (await _usedBytes() > maxBytes) {
      final oldest = await _db
          .customSelect(
            "SELECT 'thread' AS kind, id AS a, '' AS b, last_accessed_at AS t FROM threads "
            "UNION ALL SELECT 'file', file_id, variant, last_accessed_at FROM file_cache "
            'ORDER BY t, kind, a, b LIMIT 1',
          )
          .getSingleOrNull();
      if (oldest == null) break;
      final a = oldest.read<String>('a');
      if (oldest.read<String>('kind') == 'thread') {
        paths.addAll(await _deleteThread(a));
      } else {
        final path = await _db.fileCacheDao.delete(
          FileId.parse(a),
          oldest.read<String>('b'),
        );
        if (path != null) paths.add(path);
      }
    }
    return paths;
  }

  Future<List<String>> _deleteThread(String id) =>
      _db.transaction(() => deleteThreads(_db, [id], withDrafts: false));

  /// Deletes the content of thread [id], keeps its row and starts its age again.
  Future<List<String>> _emptyThread(String id) => _db.transaction(() async {
    final paths = await deleteThreads(
      _db,
      [id],
      withDrafts: false,
      contentOnly: true,
    );
    // No watcher of the threads is woken: the access time is in no value they hand out.
    await _db.customUpdate(
      'UPDATE threads SET last_accessed_at = ? WHERE id = ?',
      variables: [Variable<int>(toMillis(_now())), Variable<String>(id)],
    );
    return paths;
  });

  /// Used pages of the database plus the sum of the file cache rows.
  Future<int> _usedBytes() async {
    final pages = await _db
        .customSelect(
          'SELECT (SELECT page_count FROM pragma_page_count()) - '
          '(SELECT freelist_count FROM pragma_freelist_count()) AS used, '
          '(SELECT page_size FROM pragma_page_size()) AS size',
        )
        .getSingle();
    final cached = await _db.fileCacheDao.totalBytes();
    return pages.read<int>('used') * pages.read<int>('size') + cached;
  }
}
