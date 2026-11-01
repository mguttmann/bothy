// SPDX-License-Identifier: Apache-2.0

// No read of the old secrets shows a keychain dialog or holds the start
// (docs/decisions.d/FIX-macos-legacy-keychain-prompt.md).
//
// The live case of 2026-11-01: after the update into the app scope a macOS build that keeps its
// secrets in the login keychain stopped at its start on a keychain access dialog. The old items
// were written by a build of another bundle identifier, so their access list trusts only that
// app; the plugin read of the move shows the dialog and waits for it, nobody could click, and
// the app never reached the server. The fake store models that dialog as a plugin call that
// never completes; the runner's reader (`LegacyKeychain`) answers such an item with
// `errSecAuthFailed`, as macos/LegacyKeychainTests measures against a real keychain.

import 'dart:async';
import 'dart:convert';

import 'package:client/platform/secure_store.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/secure_storage_fakes.dart';

const String _app = 'com.example.chat';
const String _i1 = '0193f2a7-0000-7000-8000-0000000000a1';
const String _origin = 'https://chat.example.com';
const String _token = 'instance/$_i1/auth/$_origin/refresh_token';
const String _deviceId = 'instance/$_i1/auth/$_origin/device_id';
const String _dbKey = 'instance/$_i1/database_key';

/// The macOS builds that keep their secrets in the login keychain.
const Map<String, ({bool entitled, MacOsOptions keychain})> _loginBuilds = {
  'macOS without keychain entitlements': (
    entitled: false,
    keychain: dataProtectionOptions,
  ),
  'macOS debug or profile build': (
    entitled: true,
    keychain: loginKeychainOptions,
  ),
};

/// The old secrets of one signed-in instance at the old place of [storage].
void _oldSession(SharedStorage storage, {bool dataProtection = false}) {
  storage.at(AppleOptions.defaultAccountName, dataProtection: dataProtection)
    ..['app/server_address'] = _origin
    ..['app/instance'] = jsonEncode({'instance_id': _i1, 'address': _origin})
    ..[_dbKey] = 'k'
    ..[_deviceId] = 'd'
    ..[_token] = 'r';
}

FlutterSecureBackend _backend(
  SharedStorage storage,
  MacOsOptions keychain, {
  List<String>? logs,
  Duration legacyDeadline = const Duration(seconds: 5),
  Duration callDeadline = const Duration(seconds: 15),
}) => FlutterSecureBackend(
  appId: _app,
  storage: storage,
  platform: storage.platform,
  keychainOptions: keychain,
  localInstances: () async => {_i1},
  legacyKeychain: storage.legacyKeychain,
  legacyDeadline: legacyDeadline,
  callDeadline: callDeadline,
  log: logs == null ? (_) {} : logs.add,
);

/// The first call of a start, which every start awaits; fails the test instead of hanging.
Future<String?> _firstRead(FlutterSecureBackend backend) => backend
    .read('app/server_address')
    .timeout(
      const Duration(seconds: 2),
      onTimeout: () => fail('the start waits for a keychain dialog'),
    );

void main() {
  test('the deadlines the decisions name (FIX-MLKP/3)', () {
    final backend = FlutterSecureBackend(appId: _app);
    expect(backend.legacyDeadline, const Duration(seconds: 5));
    expect(backend.callDeadline, const Duration(seconds: 15));
  });

  for (final MapEntry(key: name, value: build) in _loginBuilds.entries) {
    group(name, () {
      test('old items another app wrote are not migratable: no dialog, the '
          'start goes on with enrollment and logs it once', () async {
        final storage = SharedStorage(
          TargetPlatform.macOS,
          entitled: build.entitled,
        );
        _oldSession(storage);
        // The live keychain: every old item trusts only the old bundle identifier.
        storage.foreign.addAll(
          storage.at(AppleOptions.defaultAccountName).keys,
        );
        final logs = <String>[];

        final backend = _backend(storage, build.keychain, logs: logs);
        expect(await _firstRead(backend), isNull);
        expect(
          await InstanceSecrets(
            backend,
            _i1,
          ).read('auth/$_origin/refresh_token'),
          isNull,
        );
        expect(
          storage.pluginLegacyCalls,
          isEmpty,
          reason: 'the plugin can show a dialog',
        );
        expect(logs.where((l) => l.contains('not migratable')), hasLength(1));
        // Enrollment writes the new session; nothing waits for the old items.
        await InstanceSecrets(backend, _i1).write('device_id', 'new');
        expect(await backend.keys(), {'instance/$_i1/device_id'});

        // The next start does not ask the keychain again and logs nothing.
        storage.legacyKeychain.calls.clear();
        final nextLogs = <String>[];
        final next = _backend(storage, build.keychain, logs: nextLogs);
        expect(await _firstRead(next), isNull);
        expect(storage.legacyKeychain.calls, isEmpty);
        expect(
          nextLogs.where((l) => l.contains('build before the app scope')),
          isEmpty,
        );
      });

      test(
        'one refused item keeps the whole old session out of the scope',
        () async {
          final storage = SharedStorage(
            TargetPlatform.macOS,
            entitled: build.entitled,
          );
          _oldSession(storage);
          storage.foreign.add(_token);

          final backend = _backend(storage, build.keychain);
          expect(await _firstRead(backend), isNull);
          expect(await backend.keys(), isEmpty, reason: 'no half session');
        },
      );

      test('old items this app may read move without the plugin', () async {
        final storage = SharedStorage(
          TargetPlatform.macOS,
          entitled: build.entitled,
        );
        _oldSession(storage);

        final backend = _backend(storage, build.keychain);
        expect(await _firstRead(backend), _origin);
        expect(await backend.read(_token), 'r');
        expect(await backend.read(_dbKey), 'k');
        expect(storage.pluginLegacyCalls, isEmpty);

        // The first refresh removes the old items through the reader as well.
        await backend.write(_token, 'r2');
        expect(storage.at(AppleOptions.defaultAccountName), isEmpty);
        expect(storage.pluginLegacyCalls, isEmpty);
      });

      test('a reader that never answers holds the start only for the '
          'deadline, and the abandoned copy writes nothing later', () async {
        final storage = SharedStorage(
          TargetPlatform.macOS,
          entitled: build.entitled,
        );
        _oldSession(storage);
        storage.legacyKeychain.hang = true;
        final logs = <String>[];

        final backend = _backend(
          storage,
          build.keychain,
          logs: logs,
          legacyDeadline: const Duration(milliseconds: 100),
        );
        expect(await _firstRead(backend), isNull);
        expect(logs.join('\n'), contains('did not finish'));
        expect(await backend.keys(), isEmpty);
        expect(
          await backend.read(FlutterSecureBackend.scopeMarkerKey),
          isNull,
          reason: 'the next start tries again',
        );
      });
    });
  }

  test('a platform store that never answers fails the call after the '
      'deadline instead of holding the start', () async {
    final logs = <String>[];
    final storage = _DialogStorage();
    final backend = FlutterSecureBackend(
      appId: _app,
      storage: storage,
      platform: TargetPlatform.macOS,
      keychainOptions: loginKeychainOptions,
      localInstances: () async => {},
      legacyKeychain: storage.legacyKeychain,
      callDeadline: const Duration(milliseconds: 200),
      log: logs.add,
    );
    await expectLater(
      backend.read('app/server_address'),
      throwsA(isA<TimeoutException>()),
    );
    expect(logs.join('\n'), contains('did not answer read'));
  });

  test('the old reads of iOS and the Data Protection keychain never allow a '
      'dialog', () async {
    for (final (platform, dp) in [
      (TargetPlatform.iOS, false),
      (TargetPlatform.macOS, true),
    ]) {
      final storage = SharedStorage(platform);
      _oldSession(storage, dataProtection: dp);
      final backend = FlutterSecureBackend(
        appId: _app,
        storage: storage,
        platform: platform,
        keychainOptions: dataProtectionOptions,
        legacyKeychain: storage.legacyKeychain,
        log: (_) {},
      );
      expect(await backend.read(_token), 'r');
      await backend.write(_token, 'r2');
      expect(storage.legacyOptions, isNotEmpty, reason: '$platform');
      for (final options in storage.legacyOptions) {
        expect(
          options?.authenticationUIBehavior,
          FlutterSecureBackend.noAuthenticationUI,
          reason: '$platform',
        );
      }
      expect(storage.legacyKeychain.calls, isEmpty, reason: '$platform');
    }
  });
}

/// A login keychain whose every read shows a dialog nobody answers (an item a build signed
/// differently wrote into the app's own service).
final class _DialogStorage extends SharedStorage {
  _DialogStorage() : super(TargetPlatform.macOS);

  @override
  Future<String?> read({
    required String key,
    AppleOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
  }) => Completer<String?>().future;
}
