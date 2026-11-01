// SPDX-License-Identifier: Apache-2.0

// The move of the old secrets against the real login keychain of macOS
// (FIX-macos-legacy-keychain-prompt, FIX-macos-migration-denied; run by
// tool/run_legacy_keychain_test.sh).
//
// The storage fakes of test/platform answer every read at once, so the move passed its tests while
// the real app stopped at its start on a keychain dialog: whether a read shows that dialog is
// decided by securityd from the item's access list and the app's code signature, which no fake
// has. Here the app's own runner channel (`bothy/legacy_keychain`) reads a real old item whose
// access list trusts only another program, and one this app wrote itself, the way a debug build
// keeps its secrets (the login keychain). The owner's case of 2026-11-01 runs as well: the session
// the build before the app scope wrote, an `app/instance` only another app may read, and the final
// marker a failed move left in the scope; the session must move without any dialog.

import 'dart:convert';

import 'package:client/platform/secure_store.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

/// The instance whose old database key the script wrote with a foreign access list.
const String _foreignInstance = String.fromEnvironment(
  'BOTHY_FOREIGN_INSTANCE',
);

/// Whether the script put the foreign `app/instance` of the owner's case into the login keychain
/// (it does so only when no such item exists yet).
const bool _ownerState = bool.fromEnvironment('BOTHY_OWNER_STATE');

/// The login keychain, as a debug or profile build keeps its secrets.
const MacOsOptions _login = MacOsOptions(
  accessibility: KeychainAccessibility.first_unlock_this_device,
  synchronizable: false,
  usesDataProtectionKeychain: false,
);

/// The time a start may take for the store's first call.
const Duration _start = Duration(seconds: 5);

const FlutterSecureStorage _storage = FlutterSecureStorage();

/// The app scope of [appId] in the login keychain.
MacOsOptions _scopeOf(String appId) => MacOsOptions(
  accountName: FlutterSecureBackend.serviceFor(appId),
  accessibility: _login.accessibility,
  synchronizable: false,
  usesDataProtectionKeychain: false,
);

/// Deletes what [backend] of [appId] wrote into its scope.
Future<void> _cleanUp(FlutterSecureBackend backend, String appId) async {
  for (final key in await backend.keys()) {
    await backend.delete(key);
  }
  final scope = _scopeOf(appId);
  for (final key in [
    FlutterSecureBackend.scopeMarkerKey,
    FlutterSecureBackend.indexKey,
    FlutterSecureBackend.legacyPendingKey,
  ]) {
    await _storage.delete(key: key, mOptions: scope);
  }
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('an old item another app wrote: no dialog, the start goes on', (
    tester,
  ) async {
    if (_foreignInstance.isEmpty) {
      markTestSkipped('needs tool/run_legacy_keychain_test.sh');
      return;
    }
    const appId = 'com.example.legacykeychain.foreign';
    final logs = <String>[];
    final backend = FlutterSecureBackend(
      appId: appId,
      keychainOptions: _login,
      localInstances: () async => {_foreignInstance},
      log: logs.add,
    );
    try {
      final watch = Stopwatch()..start();
      final value = await backend
          .read('instance/$_foreignInstance/database_key')
          .timeout(_start, onTimeout: () => fail('the start waits'));
      expect(watch.elapsed, lessThan(_start));
      expect(value, isNull);
      expect(logs.join('\n'), contains('refuses'));
      expect(
        await _storage.read(
          key: FlutterSecureBackend.scopeMarkerKey,
          mOptions: _scopeOf(appId),
        ),
        isNull,
        reason: 'a refusal is never final: the next start tries again',
      );
    } finally {
      await _cleanUp(backend, appId);
    }
  });

  testWidgets('an old item this app wrote moves into the scope', (
    tester,
  ) async {
    const appId = 'com.example.legacykeychain.own';
    final instance =
        'legacy-keychain-own-${DateTime.now().microsecondsSinceEpoch}';
    final key = 'instance/$instance/database_key';
    // The build before the app scope: the old service, the login keychain.
    await _storage.write(key: key, value: 'old-key', mOptions: _login);
    final backend = FlutterSecureBackend(
      appId: appId,
      keychainOptions: _login,
      localInstances: () async => {instance},
    );
    try {
      final watch = Stopwatch()..start();
      final value = await backend
          .read(key)
          .timeout(_start, onTimeout: () => fail('the start waits'));
      expect(watch.elapsed, lessThan(_start));
      expect(value, 'old-key');
    } finally {
      await _storage.delete(key: key, mOptions: _login);
      await _cleanUp(backend, appId);
    }
  });

  testWidgets('the owner\'s update after a failed move: the old session '
      'moves, app/instance is not read, the marker does not block', (
    tester,
  ) async {
    if (!_ownerState) {
      markTestSkipped(
        'needs tool/run_legacy_keychain_test.sh on a keychain '
        'without app/instance and app/server_address',
      );
      return;
    }
    const appId = 'com.example.legacykeychain.owner';
    final instance =
        'legacy-keychain-owner-${DateTime.now().microsecondsSinceEpoch}';
    const origin = 'https://owner.example.com';
    final dbKey = 'instance/$instance/database_key';
    final deviceId = 'instance/$instance/auth/$origin/device_id';
    final token = 'instance/$instance/auth/$origin/refresh_token';
    final keyScope =
        'instance/$instance/${FlutterSecureBackend.deviceKeyScopeKey}';
    // The build before the app scope: the plugin, the old service, the login keychain.
    final old = {
      'app/server_address': origin,
      dbKey: 'k-old',
      deviceId: 'd-old',
      token: 'r-old',
    };
    for (final MapEntry(:key, :value) in old.entries) {
      await _storage.write(key: key, value: value, mOptions: _login);
    }
    // What the failed move left in the scope: the final marker, a new database key and device
    // key scope, the server and instance of its discovery, no session.
    final scope = _scopeOf(appId);
    final leftovers = {
      FlutterSecureBackend.scopeMarkerKey: appId,
      FlutterSecureBackend.indexKey: jsonEncode([
        keyScope,
        dbKey,
        'app/server_address',
        'app/instance',
      ]),
      keyScope: '$appId/$instance',
      dbKey: 'k-new',
      'app/server_address': origin,
      'app/instance': jsonEncode({'instance_id': instance, 'address': origin}),
    };
    for (final MapEntry(:key, :value) in leftovers.entries) {
      await _storage.write(key: key, value: value, mOptions: scope);
    }
    final logs = <String>[];
    final backend = FlutterSecureBackend(
      appId: appId,
      keychainOptions: _login,
      localInstances: () async => {instance},
      log: logs.add,
    );
    try {
      final watch = Stopwatch()..start();
      final value = await backend
          .read(token)
          .timeout(_start, onTimeout: () => fail('the start waits'));
      expect(watch.elapsed, lessThan(_start));
      expect(value, 'r-old', reason: 'logs: $logs');
      expect(await backend.read(deviceId), 'd-old');
      expect(await backend.read(dbKey), 'k-old');
      expect(await backend.read(keyScope), instance);
      expect(logs.join('\n'), isNot(contains('refuses')));
      for (final MapEntry(:key, :value) in old.entries) {
        expect(
          await _storage.read(key: key, mOptions: _login),
          value,
          reason: 'the old items stay until the first refresh',
        );
      }
    } finally {
      for (final key in old.keys) {
        await _storage.delete(key: key, mOptions: _login);
      }
      await _cleanUp(backend, appId);
    }
  });
}
