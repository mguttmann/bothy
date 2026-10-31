// SPDX-License-Identifier: Apache-2.0

// The database key of the app lock's residual protection (brief M5-08 Design note 6; 15 6.3,
// 15 9; T-DV-01): a copy of the database file is unreadable without the key from secure
// storage, and the key item is stored for this device only and, on iOS, only while the device
// is unlocked (`WhenUnlockedThisDeviceOnly`), unlike the tokens (`AfterFirstUnlock`).

import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:client/platform/secure_store.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart'
    show KeychainAccessibility;
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/fakes.dart';

/// A key source with a fixed key.
class _FixedKey implements DatabaseKeySource {
  _FixedKey(this.key);

  final Uint8List key;

  @override
  Future<Uint8List> obtainKey() async => key;
}

const String _marker = 'Please include expired threads.';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('a copy of the database file', () {
    late Directory dir;
    late File copy;
    late SecureDatabaseKeySource keys;

    setUp(() async {
      dir = Directory.systemTemp.createTempSync('db-key');
      keys = SecureDatabaseKeySource(
        InstanceSecrets(MemorySecureBackend(), 'instance-a'),
      );
      final file = File('${dir.path}/local.db');
      final db = await openLocalStore(
        file: file,
        keys: keys,
        inBackground: false,
      );
      await db.outboxDao.enqueue(
        id: '0192f4a0-7b1c-7000-8000-000000000001',
        kind: 'postMessage',
        payload: jsonEncode({'content': _marker}),
      );
      await db.close();
      copy = file.copySync('${dir.path}/copied.db');
    });
    tearDown(() => dir.deleteSync(recursive: true));

    test('holds no readable SQLite header and no plain text', () {
      final bytes = copy.readAsBytesSync();
      expect(
        latin1.decode(bytes.sublist(0, 16), allowInvalid: true),
        isNot(startsWith('SQLite format 3')),
      );
      expect(
        latin1.decode(bytes, allowInvalid: true),
        isNot(contains(_marker)),
      );
    });

    test('does not open with a wrong key', () async {
      final random = Random(7);
      final wrong = Uint8List.fromList(
        List<int>.generate(databaseKeyBytes, (_) => random.nextInt(256)),
      );
      await expectLater(
        openLocalStore(file: copy, keys: _FixedKey(wrong), inBackground: false),
        throwsA(anything),
      );
    });

    test('does not open on a device whose secure storage lacks the key', () async {
      // Another device: its own, empty secure storage creates a key of its own.
      final elsewhere = SecureDatabaseKeySource(
        InstanceSecrets(MemorySecureBackend(), 'instance-a'),
      );
      await expectLater(
        openLocalStore(file: copy, keys: elsewhere, inBackground: false),
        throwsA(anything),
      );
    });

    test(
      'opens with the key from the secure storage it was made with',
      () async {
        final db = await openLocalStore(
          file: copy,
          keys: keys,
          inBackground: false,
        );
        addTearDown(db.close);
        final entry = await db.outboxDao.byId(
          '0192f4a0-7b1c-7000-8000-000000000001',
        );
        expect(entry?.payload, contains(_marker));
      },
    );
  });

  group('the key item in the secure storage', () {
    const channel = MethodChannel(
      'plugins.it_nomads.com/flutter_secure_storage',
    );
    final writes = <String, Map<Object?, Object?>>{};
    final store = <String, String>{};

    setUp(() {
      writes.clear();
      store.clear();
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, (call) async {
            final args = (call.arguments as Map?) ?? const {};
            final key = args['key'] as String?;
            switch (call.method) {
              case 'write':
                writes[key!] = args['options']! as Map<Object?, Object?>;
                store[key] = args['value']! as String;
                return null;
              case 'read':
                return store[key];
              case 'readAll':
                return Map.of(store);
              case 'delete':
                store.remove(key);
                return null;
            }
            return null;
          });
    });
    tearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
      debugDefaultTargetPlatformOverride = null;
    });

    Future<void> writeSecrets() async {
      final secrets = InstanceSecrets(
        FlutterSecureBackend(appId: 'com.example.chat'),
        'instance-a',
      );
      await SecureDatabaseKeySource(secrets).obtainKey();
      await secrets.write('auth/https://chat.example.com/refresh_token', 'r');
    }

    test('on iOS: WhenUnlockedThisDeviceOnly, never synchronized', () async {
      debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
      await writeSecrets();
      final key = writes['instance/instance-a/database_key']!;
      expect(
        key['accessibility'],
        KeychainAccessibility.unlocked_this_device.name,
      );
      expect(key['synchronizable'], 'false');
      final token =
          writes['instance/instance-a/auth/https://chat.example.com/refresh_token']!;
      expect(
        token['accessibility'],
        KeychainAccessibility.first_unlock_this_device.name,
        reason: 'the tokens stay readable after the first unlock',
      );
    });

    test('on Android: the Keystore-wrapped store of this device', () async {
      debugDefaultTargetPlatformOverride = TargetPlatform.android;
      await writeSecrets();
      expect(
        writes['com.example.chat/instance/instance-a/database_key'],
        FlutterSecureBackend.androidOptions.toMap(),
      );
    });
  });
}
