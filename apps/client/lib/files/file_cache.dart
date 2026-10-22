// SPDX-License-Identifier: Apache-2.0

/// The local file cache (15 6.1 `file_cache`, 15 6.3; brief M4-22 Design note 5): the bytes
/// of downloaded files and thumbnails under the instance's folder, one row per file and
/// variant in `file_cache` with `last_accessed_at`. The rows live in the encrypted database;
/// the cleaner of M4-12, the access removal of M4-14 (`FileCacheEraser`) and the wipe of
/// M4-18 delete the bytes with them.
library;

import 'dart:io';

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

/// The bytes of cached files at `<root>/<file id>/<variant>`, where `<root>` is
/// `<application support>/instances/<instance id>/files` (D-M4-22-5); the original of a file
/// with a name lies at `<root>/<file id>/original/<name>`, so the system opens it with the app
/// of its extension.
///
/// It is the [FileCacheEraser] of the engine: when the dispatcher removes the rows of a target
/// (`access.changed` with `removed`), it hands their paths here. [clean] runs the
/// [CacheCleaner] of the database and deletes the paths it returns. The account wipe deletes
/// the instance folder and so the whole cache.
///
/// Example:
/// ```dart
/// final cache = FileCache(db: database, root: Directory('$folder/files'));
/// final hit = await cache.lookup(file.id, 'small');
/// ```
final class FileCache implements FileCacheEraser {
  /// Creates the cache on [db] with the bytes below [root].
  FileCache({required LocalDatabase db, required this.root})
    : _db = db; // ignore: prefer_initializing_formals

  final LocalDatabase _db;

  /// The folder of the bytes.
  final Directory root;

  /// Where the bytes of [variant] of [fileId] lie; [name] (the original's file name) adds a
  /// last segment.
  File fileOf(FileId fileId, String variant, {String? name}) {
    final sep = Platform.pathSeparator;
    final base = '${root.path}$sep${fileId.value}$sep$variant';
    final safe = name == null ? null : safeFileName(name);
    return File(safe == null ? base : '$base$sep$safe');
  }

  /// [name] as one path segment on every desktop system: without separators, control
  /// characters, leading dots and spaces or trailing dots and spaces; a Windows device name
  /// (`CON`, `lpt1.txt`) gets the prefix `_`; `file` when nothing is left.
  static String safeFileName(String name) {
    final cleaned = name
        .replaceAll(RegExp(r'[/\\:*?"<>|\x00-\x1f\x7f]'), '_')
        .replaceFirst(RegExp(r'^[\s.]+'), '')
        .replaceFirst(RegExp(r'[\s.]+$'), '');
    if (cleaned.isEmpty) return 'file';
    return _windowsDevice.hasMatch(cleaned) ? '_$cleaned' : cleaned;
  }

  static final RegExp _windowsDevice = RegExp(
    r'^(con|prn|aux|nul|com[0-9]|lpt[0-9])(\.|$)',
    caseSensitive: false,
  );

  /// A new temporary file for a download in progress, in the cache's own folder so the final
  /// rename stays on one file system.
  Future<File> partial(FileId fileId, String variant) async {
    final dir = Directory('${root.path}${Platform.pathSeparator}.partial');
    await dir.create(recursive: true);
    return File(
      '${dir.path}${Platform.pathSeparator}${fileId.value}.$variant.'
      '${DateTime.now().microsecondsSinceEpoch}',
    );
  }

  /// The cached bytes of [variant] of [fileId], or null; a hit records the access (15 6.3). A
  /// row whose bytes are gone is dropped.
  Future<File?> lookup(FileId fileId, String variant) async {
    final entry = await _db.fileCacheDao.entry(fileId, variant);
    if (entry == null) return null;
    final file = File(entry.path);
    if (!file.existsSync()) {
      await _db.fileCacheDao.delete(fileId, variant);
      return null;
    }
    await _db.fileCacheDao.touch(fileId, variant);
    return file;
  }

  /// Moves the checked download [partialFile] to its place and records the row.
  Future<File> store(
    FileId fileId,
    String variant,
    File partialFile, {
    String? name,
  }) async {
    final target = fileOf(fileId, variant, name: name);
    await target.parent.create(recursive: true);
    final moved = await partialFile.rename(target.path);
    await _db.fileCacheDao.put(
      FileCacheEntry(
        fileId: fileId,
        variant: variant,
        path: moved.path,
        sizeBytes: await moved.length(),
        lastAccessedAt: _db.clock(),
      ),
    );
    return moved;
  }

  /// Deletes the row and the bytes of [variant] of [fileId] (a damaged download).
  Future<void> drop(FileId fileId, String variant) async {
    final path = await _db.fileCacheDao.delete(fileId, variant);
    await erase([?path]);
  }

  @override
  Future<void> erase(List<String> paths) async {
    for (final path in paths) {
      final file = File(path);
      try {
        if (file.existsSync()) await file.delete();
        // The empty folders of the file go too, up to the root.
        var dir = file.parent;
        while (dir.path.startsWith(root.path) &&
            dir.path != root.path &&
            dir.existsSync() &&
            dir.listSync().isEmpty) {
          await dir.delete();
          dir = dir.parent;
        }
      } on FileSystemException {
        // A file another process holds open goes with the next cleanup or the wipe.
      }
    }
  }

  /// Runs [cleaner] (15 6.3: `client.cache_max_age`, `client.cache_max_bytes`) and deletes the
  /// bytes of the rows it removed; returns how many paths went.
  Future<int> clean(CacheCleaner cleaner) async {
    final paths = await cleaner.run();
    await erase(paths);
    return paths.length;
  }
}
