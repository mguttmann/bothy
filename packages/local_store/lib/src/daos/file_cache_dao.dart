// SPDX-License-Identifier: Apache-2.0

/// Access to the metadata of the file cache (15 6.1 `file_cache`); M4-22 writes and reads the
/// bytes and deletes the paths this package returns.
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../mappers/codec.dart';
import '../models.dart';

/// Data access object of the file cache rows.
final class FileCacheDao {
  /// Creates the DAO on [db]; use `LocalDatabase.fileCacheDao`.
  FileCacheDao(this._db);

  final LocalDatabase _db;

  /// Stores [entry], replacing the row of the same file and variant.
  Future<void> put(FileCacheEntry entry) async {
    await _db
        .into(_db.fileCache)
        .insertOnConflictUpdate(
          FileCacheCompanion.insert(
            fileId: entry.fileId.value,
            variant: entry.variant,
            path: entry.path,
            sizeBytes: entry.sizeBytes,
            lastAccessedAt: toMillis(entry.lastAccessedAt),
          ),
        );
  }

  /// The row of [fileId] and [variant], or null.
  Future<FileCacheEntry?> entry(FileId fileId, String variant) async {
    final row =
        await (_db.select(_db.fileCache)..where(
              (t) => t.fileId.equals(fileId.value) & t.variant.equals(variant),
            ))
            .getSingleOrNull();
    return row == null ? null : _fromRow(row);
  }

  /// Records an access to [fileId] and [variant] now (15 6.3).
  Future<void> touch(FileId fileId, String variant) async {
    await (_db.update(_db.fileCache)..where(
          (t) => t.fileId.equals(fileId.value) & t.variant.equals(variant),
        ))
        .write(
          FileCacheCompanion(lastAccessedAt: Value(toMillis(_db.clock()))),
        );
  }

  /// Deletes the row of [fileId] and [variant] and returns its path, or null.
  Future<String?> delete(FileId fileId, String variant) =>
      _db.transaction(() async {
        final found = await entry(fileId, variant);
        if (found == null) return null;
        await (_db.delete(_db.fileCache)..where(
              (t) => t.fileId.equals(fileId.value) & t.variant.equals(variant),
            ))
            .go();
        return found.path;
      });

  /// The sum of `size_bytes` of all rows.
  Future<int> totalBytes() async {
    final sum = _db.fileCache.sizeBytes.sum();
    final row = await (_db.selectOnly(
      _db.fileCache,
    )..addColumns([sum])).getSingle();
    return row.read(sum) ?? 0;
  }

  static FileCacheEntry _fromRow(FileCacheRow row) => FileCacheEntry(
    fileId: FileId.parse(row.fileId),
    variant: row.variant,
    path: row.path,
    sizeBytes: row.sizeBytes,
    lastAccessedAt: fromMillis(row.lastAccessedAt),
  );
}
