// SPDX-License-Identifier: Apache-2.0

// Encryption at rest (15 6.3, R-CL-08, T-DV-01): a file in a temporary directory, the fixed
// test keys of support/fixtures.dart.

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:local_store/local_store.dart';
import 'package:sqlite3/sqlite3.dart' show SqliteException;
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

  // FIX-client-db-key: only a file the key does not decrypt is a key error. A file the device
  // cannot read at the moment (the iOS file protection of a locked device, a busy or locked
  // file, an I/O error) is transient and never "unreadable", so nothing offers or runs a
  // reset for it.
  for (final inBackground in [true, false]) {
    test(
      'a file without read permission fails as unavailable, not as a key error '
      '(inBackground: $inBackground)',
      () async {
        await writeOneEntry(inBackground);
        // The -wal and -shm files without read permission: the file opens, and the key check
        // fails with SQLITE_CANTOPEN, as the files of a locked iPhone do (15 6.3).
        for (final suffix in ['-wal', '-shm']) {
          final f = File('${dbFile.path}$suffix')..writeAsBytesSync([]);
          await Process.run('chmod', ['000', f.path]);
        }
        Object? error;
        try {
          await openLocalStore(
            file: dbFile,
            keys: FixedKeySource(testKeyA),
            inBackground: inBackground,
          );
        } on Object catch (e) {
          error = e;
        } finally {
          for (final suffix in ['', '-wal', '-shm']) {
            final f = File('${dbFile.path}$suffix');
            if (f.existsSync()) await Process.run('chmod', ['600', f.path]);
          }
        }
        expect(error, isA<LocalStoreUnavailableError>());
        expect(error, isNot(isA<LocalStoreKeyError>()));
      },
      testOn: 'mac-os || linux',
    );
  }

  group('openErrorOf', () {
    SqliteException code(int extended) =>
        SqliteException(extendedResultCode: extended, message: 'm');

    test('SQLITE_NOTADB and SQLITE_CORRUPT are key errors', () {
      expect(openErrorOf(code(26)), isA<LocalStoreKeyError>());
      expect(openErrorOf(code(11)), isA<LocalStoreKeyError>());
      // An extended code keeps its primary code.
      expect(openErrorOf(code(11 | (1 << 8))), isA<LocalStoreKeyError>());
    });

    test(
      'busy, locked, I/O, cannot open, permission and auth are transient',
      () {
        for (final primary in [3, 5, 6, 10, 14, 23]) {
          final error = openErrorOf(code(primary));
          expect(error, isA<LocalStoreUnavailableError>(), reason: '$primary');
          expect(
            (error as LocalStoreUnavailableError).resultCode,
            primary,
            reason: '$primary',
          );
        }
        // SQLITE_IOERR_READ (266) is an I/O error.
        expect(openErrorOf(code(266)), isA<LocalStoreUnavailableError>());
      },
    );

    test('an unknown code is transient: it never offers a reset', () {
      expect(openErrorOf(code(99)), isA<LocalStoreUnavailableError>());
    });

    test('the reason codes are fixed and name no key', () {
      expect(const LocalStoreKeyError().reason, LocalStoreKeyReason.wrongKey);
      expect(
        const LocalStoreKeyError(LocalStoreKeyReason.keyMissing).toString(),
        contains('key_missing'),
      );
      expect(
        const LocalStoreUnavailableError(14).toString(),
        'LocalStoreUnavailableError: sqlite 14 (cantopen), try again later',
      );
    });
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
