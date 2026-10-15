// SPDX-License-Identifier: Apache-2.0

// Encryption at rest (15 6.3, R-CL-08, T-DV-01): a file in a temporary directory, the fixed
// test keys of support/fixtures.dart.

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late Directory dir;
  late File dbFile;
  setUp(() async {
    dir = await tempDir();
    dbFile = File('${dir.path}/store.db');
  });
  tearDown(() => dir.delete(recursive: true));

  Future<void> writeOneEntry(bool inBackground) async {
    final db = await openLocalStore(
      file: dbFile,
      keys: FixedKeySource(testKeyA),
      inBackground: inBackground,
    );
    await db.outboxDao.enqueue(id: uuid(1), kind: 'postMessage', payload: '{}');
    await db.close();
  }

  for (final inBackground in [true, false]) {
    test(
      'reopening with key A shows the rows (inBackground: $inBackground)',
      () async {
        await writeOneEntry(inBackground);
        final db = await openLocalStore(
          file: dbFile,
          keys: FixedKeySource(testKeyA),
          inBackground: inBackground,
        );
        expect((await db.outboxDao.byId(uuid(1)))?.kind, 'postMessage');
        await db.close();
      },
    );

    test(
      'key B fails with LocalStoreKeyError without the key (inBackground: $inBackground)',
      () async {
        await writeOneEntry(inBackground);
        Object? error;
        try {
          await openLocalStore(
            file: dbFile,
            keys: FixedKeySource(testKeyB),
            inBackground: inBackground,
          );
        } on Object catch (e) {
          error = e;
        }
        expect(error, isA<LocalStoreKeyError>());
        final text = error.toString().toLowerCase();
        for (final key in [testKeyA, testKeyB]) {
          final hex = key
              .map((b) => b.toRadixString(16).padLeft(2, '0'))
              .join();
          expect(text, isNot(contains(hex)));
          expect(text, isNot(contains(base64Encode(key).toLowerCase())));
        }
      },
    );
  }

  test('the file is not readable as plain SQLite', () async {
    await writeOneEntry(false);
    final header = (await dbFile.openRead(0, 16).first);
    expect(
      ascii.decode(header, allowInvalid: true),
      isNot(startsWith('SQLite format 3')),
    );
  });

  test('the cipher is chacha20 and FTS5 is compiled in', () async {
    final db = await openLocalStore(
      file: dbFile,
      keys: FixedKeySource(testKeyA),
      inBackground: false,
    );
    final cipher = await db.customSelect('PRAGMA cipher').getSingle();
    expect(cipher.data.values.single, expectedCipher);
    final fts5 = await db
        .customSelect("SELECT sqlite_compileoption_used('ENABLE_FTS5') AS on_")
        .getSingle();
    expect(fts5.read<int>('on_'), 1);
    await db.close();
  });

  test(
    'a key of the wrong length is rejected before the file is opened',
    () async {
      expect(
        () => openLocalStore(
          file: dbFile,
          keys: FixedKeySource(Uint8List(16)),
          inBackground: false,
        ),
        throwsA(isA<LocalStoreSetupError>()),
      );
      expect(dbFile.existsSync(), isFalse);
    },
  );

  test('wipe deletes the database file and its -wal and -shm files', () async {
    final db = await openLocalStore(
      file: dbFile,
      keys: FixedKeySource(testKeyA),
      inBackground: false,
    );
    await db.outboxDao.enqueue(id: uuid(2), kind: 'postMessage', payload: '{}');
    expect(dbFile.existsSync(), isTrue);
    await db.wipe();
    for (final suffix in ['', '-wal', '-shm']) {
      expect(
        File('${dbFile.path}$suffix').existsSync(),
        isFalse,
        reason: suffix,
      );
    }
  });

  test('the error types name no key', () {
    expect(
      const LocalStoreKeyError().toString(),
      contains('key does not open'),
    );
    expect(
      const LocalStoreSetupError('x').toString(),
      'LocalStoreSetupError: x',
    );
  });
}
