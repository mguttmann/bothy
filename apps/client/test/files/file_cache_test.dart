// SPDX-License-Identifier: Apache-2.0

// The file cache (brief M4-22; 12 9, 15 6.3): the original name of another person becomes one
// valid segment on every desktop system, a row whose bytes are gone is dropped, and erasing a
// file takes its empty folders up to the root.

import 'dart:io';

import 'package:client/files/file_cache.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

void main() {
  group('storage', () {
    late Directory folder;
    late LocalDatabase db;
    late FileCache cache;
    final id = FileId.parse('0192f4a0-7b1c-78f9-8074-000000000501');

    setUp(() {
      folder = Directory.systemTemp.createTempSync('file_cache');
      db = openInMemoryForTests();
      cache = FileCache(db: db, root: Directory('${folder.path}/files'));
    });
    tearDown(() async {
      await db.close();
      folder.deleteSync(recursive: true);
    });

    Future<File> stored() async {
      final partial = await cache.partial(id, 'original');
      await partial.writeAsBytes([1, 2, 3], flush: true);
      return cache.store(id, 'original', partial, name: 'report.txt');
    }

    test('a stored file is found until its bytes are gone', () async {
      final file = await stored();
      expect((await cache.lookup(id, 'original'))?.path, file.path);
      file.deleteSync();
      expect(await cache.lookup(id, 'original'), isNull);
      expect(await db.fileCacheDao.entry(id, 'original'), isNull);
    });

    test('drop deletes the row, the bytes and the empty folders', () async {
      final file = await stored();
      await cache.drop(id, 'original');
      expect(await db.fileCacheDao.entry(id, 'original'), isNull);
      expect(file.existsSync(), isFalse);
      expect(Directory('${cache.root.path}/${id.value}').existsSync(), isFalse);
      expect(cache.root.existsSync(), isTrue);
    });

    test('erase keeps a folder that still holds another file', () async {
      final file = await stored();
      final other = File('${file.parent.path}/other.txt')
        ..writeAsStringSync('x');
      await cache.erase([file.path, '${folder.path}/missing.bin']);
      expect(file.existsSync(), isFalse);
      expect(other.existsSync(), isTrue);
    });
  });

  test('a name of dots and spaces becomes file', () {
    for (final name in [' .. ', '..', '.', '   ', ' . . ']) {
      expect(FileCache.safeFileName(name), 'file', reason: '"$name"');
    }
  });

  test('leading dots go after trimming, trailing dots and spaces go too', () {
    expect(FileCache.safeFileName(' .hidden.txt '), 'hidden.txt');
    expect(FileCache.safeFileName('report.txt. .'), 'report.txt');
    expect(FileCache.safeFileName('a/b\\c.txt'), 'a_b_c.txt');
  });

  test('Windows device names get a prefix, with or without an extension', () {
    expect(FileCache.safeFileName('CON'), '_CON');
    expect(FileCache.safeFileName('con.txt'), '_con.txt');
    expect(FileCache.safeFileName('Lpt1.tar.gz'), '_Lpt1.tar.gz');
    expect(FileCache.safeFileName('console.txt'), 'console.txt');
  });
}
