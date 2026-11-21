// SPDX-License-Identifier: Apache-2.0

// The database key source never replaces the key of an existing database
// (docs/decisions.d/FIX-client-db-key.md): "not found" creates a key only while no database
// file exists, a refused read (the iOS keychain of a locked device) is never read as "not
// found", and every path logs a fixed reason code without the key.

import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:client/onboarding/start_route.dart';
import 'package:client/platform/secure_store.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/fakes.dart';

void main() {
  late Directory dir;
  late File db;
  late MemorySecureBackend backend;
  late List<String> log;
  const stored = 'instance/instance-a/${SecureDatabaseKeySource.keyName}';

  SecureDatabaseKeySource source() => SecureDatabaseKeySource(
    InstanceSecrets(backend, 'instance-a'),
    database: db,
    random: Random(1),
    log: log.add,
  );

  setUp(() {
    dir = Directory.systemTemp.createTempSync('db-key-source');
    db = File('${dir.path}/local.db');
    backend = MemorySecureBackend();
    log = [];
  });
  tearDown(() => dir.deleteSync(recursive: true));

  test('not found and no database: creates, stores and logs a key', () async {
    final key = await source().obtainKey();
    expect(key, hasLength(databaseKeyBytes));
    expect(backend.values[stored], base64Encode(key));
    expect(log, ['db_key.created']);
  });

  test('present: returns the stored key and writes nothing', () async {
    final key = List<int>.filled(databaseKeyBytes, 9);
    backend.values[stored] = base64Encode(key);
    db.writeAsBytesSync([1]);
    expect(await source().obtainKey(), key);
    expect(backend.values[stored], base64Encode(key));
    expect(log, isEmpty);
  });

  test('not found while the database exists: no new key, LocalStoreKeyError keyMissing', () async {
    db.writeAsBytesSync([1]);
    Object? error;
    try {
      await source().obtainKey();
    } on Object catch (e) {
      error = e;
    }
    expect(
      error,
      isA<LocalStoreKeyError>().having(
        (e) => e.reason,
        'reason',
        LocalStoreKeyReason.keyMissing,
      ),
    );
    expect(backend.values, isEmpty, reason: 'no key is stored');
    expect(log, ['db_key.missing_with_database: no new key is created']);
    expect(openFailureStatus(error!), ServerStatus.localDataUnreadable);
  });

  for (final exists in [false, true]) {
    test('a locked keychain (errSecInteractionNotAllowed) is no "not found" '
        '(database exists: $exists)', () async {
      if (exists) db.writeAsBytesSync([1]);
      backend.lockedKeys.add(stored);
      Object? error;
      try {
        await source().obtainKey();
      } on Object catch (e) {
        error = e;
      }
      expect(error, isA<PlatformException>());
      expect(backend.values, isEmpty, reason: 'no key is stored');
      expect(log, ['db_key.read_failed: PlatformException -25308']);
      expect(openFailureStatus(error!), ServerStatus.keychainUnavailable);
    });
  }

  test(
    'a directory at the database path counts as an existing database',
    () async {
      Directory(db.path).createSync();
      await expectLater(
        source().obtainKey(),
        throwsA(isA<LocalStoreKeyError>()),
      );
      expect(backend.values, isEmpty);
    },
  );

  test(
    'a folder whose entries cannot be read is no proof of absence: no key',
    () async {
      db.writeAsBytesSync([1]);
      Process.runSync('chmod', ['000', dir.path]);
      addTearDown(() => Process.runSync('chmod', ['700', dir.path]));
      await expectLater(source().obtainKey(), throwsA(anything));
      expect(backend.values, isEmpty, reason: 'no key is stored');
    },
    testOn: 'mac-os || linux',
  );

  test('a read failure logs only a numeric keychain status', () async {
    final failing = _ThrowingBackend(
      PlatformException(code: 'x', details: 'java.lang.Exception: at a.b(C)'),
    );
    final lines = <String>[];
    await expectLater(
      SecureDatabaseKeySource(
        InstanceSecrets(failing, 'instance-a'),
        database: db,
        log: lines.add,
      ).obtainKey(),
      throwsA(isA<PlatformException>()),
    );
    expect(lines, ['db_key.read_failed: PlatformException']);
  });

  test('a database the device cannot read at the moment is no reset case', () {
    expect(
      openFailureStatus(const LocalStoreUnavailableError(14)),
      ServerStatus.openFailed,
    );
    expect(
      openFailureStatus(const LocalStoreKeyError()),
      ServerStatus.localDataUnreadable,
    );
  });
}

class _ThrowingBackend extends MemorySecureBackend {
  _ThrowingBackend(this.error);

  final Object error;

  @override
  Future<String?> read(String key) async => throw error;
}
