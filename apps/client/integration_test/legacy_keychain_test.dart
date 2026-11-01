// SPDX-License-Identifier: Apache-2.0

// The move of the old secrets against the real login keychain of macOS
// (FIX-macos-legacy-keychain-prompt; run by tool/run_legacy_keychain_test.sh).
//
// The storage fakes of test/platform answer every read at once, so the move passed its tests while
// the real app stopped at its start on a keychain dialog: whether a read shows that dialog is
// decided by securityd from the item's access list and the app's code signature, which no fake
// has. Here the app's own runner channel (`bothy/legacy_keychain`) reads a real old item whose
// access list trusts only another program, and one this app wrote itself, the way a debug build
// keeps its secrets (the login keychain).

import 'package:client/platform/secure_store.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

/// The instance whose old database key the script wrote with a foreign access list.
const String _foreignInstance = String.fromEnvironment(
  'BOTHY_FOREIGN_INSTANCE',
);

/// The login keychain, as a debug or profile build keeps its secrets.
const MacOsOptions _login = MacOsOptions(
  accessibility: KeychainAccessibility.first_unlock_this_device,
  synchronizable: false,
  usesDataProtectionKeychain: false,
);

/// The time a start may take for the store's first call.
const Duration _start = Duration(seconds: 5);

const FlutterSecureStorage _storage = FlutterSecureStorage();

/// Deletes what [backend] of [appId] wrote into its scope.
Future<void> _cleanUp(FlutterSecureBackend backend, String appId) async {
  for (final key in await backend.keys()) {
    await backend.delete(key);
  }
  final scope = MacOsOptions(
    accountName: FlutterSecureBackend.serviceFor(appId),
    accessibility: _login.accessibility,
    synchronizable: false,
    usesDataProtectionKeychain: false,
  );
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
      expect(logs.join('\n'), contains('not migratable'));
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
}
