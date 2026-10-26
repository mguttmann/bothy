// SPDX-License-Identifier: Apache-2.0

/// The cache cleanup of 15 6.3: threads and file cache rows without access since
/// `client.cache_max_age` go, then the least recently used until the database plus the file
/// cache fit into `client.cache_max_bytes`. Outbox, drafts, share inbox and the pinned host keys
/// (`host_key_pins`, M4-36: a lost pin would prevent the warning on a changed key) are never
/// touched; structure rows (spaces, channels, principals, commands, instance) stay for the
/// sidebar.
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
  Future<List<String>> run() async {
    final paths = <String>[];
    final cutoff = toMillis(_now().subtract(maxAge));
    final old = await _db
        .customSelect(
          'SELECT id FROM threads WHERE last_accessed_at < ?',
          variables: [Variable<int>(cutoff)],
        )
        .get();
    for (final row in old) {
      paths.addAll(await _deleteThread(row.read<String>('id')));
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
