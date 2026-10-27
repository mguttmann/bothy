// SPDX-License-Identifier: Apache-2.0

// The keychain choice of the app secrets on macOS (docs/decisions.d/FIX-macos-keychain.md): an
// app without the keychain entitlements (ad-hoc signature, no team) gets -34018 from every
// write to the Data Protection keychain; the secrets then go to the login keychain.

import 'package:client/platform/secure_store.dart';
import 'package:flutter/services.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

/// A keychain that answers like the system's: without the entitlements every write to the Data
/// Protection keychain fails with -34018 and every read of it finds nothing.
class FakeKeychainStorage extends Fake implements FlutterSecureStorage {
  FakeKeychainStorage({required this.missingEntitlement});

  final bool missingEntitlement;

  /// The items per keychain: `true` the Data Protection keychain, `false` the login keychain.
  final Map<bool, Map<String, String>> items = {true: {}, false: {}};

  /// The keychain of every write, in order.
  final List<bool> writes = [];

  static bool _dataProtection(MacOsOptions? options) =>
      (options ?? MacOsOptions.defaultOptions).usesDataProtectionKeychain;

  @override
  Future<void> write({
    required String key,
    required String? value,
    AppleOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
  }) async {
    final dp = _dataProtection(mOptions as MacOsOptions?);
    writes.add(dp);
    if (dp && missingEntitlement) {
      throw PlatformException(
        code: 'Unexpected security result code',
        message:
            'Code: -34018, Message: A required entitlement isn\'t present.',
        details: -34018,
      );
    }
    items[dp]![key] = value!;
  }

  @override
  Future<String?> read({
    required String key,
    AppleOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
  }) async {
    final dp = _dataProtection(mOptions as MacOsOptions?);
    if (dp && missingEntitlement) return null;
    return items[dp]![key];
  }

  @override
  Future<void> delete({
    required String key,
    AppleOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
  }) async {
    items[_dataProtection(mOptions as MacOsOptions?)]!.remove(key);
  }

  @override
  Future<Map<String, String>> readAll({
    AppleOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
  }) async {
    final dp = _dataProtection(mOptions as MacOsOptions?);
    if (dp && missingEntitlement) return {};
    return Map.of(items[dp]!);
  }
}

void main() {
  const signed = MacOsOptions(
    accessibility: KeychainAccessibility.first_unlock_this_device,
    synchronizable: false,
  );

  test(
    'without the entitlement the secrets go to the login keychain',
    () async {
      final keychain = FakeKeychainStorage(missingEntitlement: true);
      final logs = <String>[];
      final backend = FlutterSecureBackend(
        storage: keychain,
        macOS: true,
        keychainOptions: signed,
        log: logs.add,
      );
      await backend.write('instance/a/device_id', '1');
      expect(await backend.read('instance/a/device_id'), '1');
      expect(keychain.items[false], {'instance/a/device_id': '1'});
      expect(await backend.keys(), {'instance/a/device_id'});
      await backend.delete('instance/a/device_id');
      expect(await backend.read('instance/a/device_id'), isNull);
      expect(await backend.downgraded, isTrue);
      // One try of the Data Protection keychain, then only the login keychain; logged once.
      expect(keychain.writes.where((dp) => dp), hasLength(1));
      expect(logs, hasLength(1));
      expect(logs.single, contains('login keychain'));
    },
  );

  test('an entitled app keeps the Data Protection keychain', () async {
    final keychain = FakeKeychainStorage(missingEntitlement: false);
    final logs = <String>[];
    final backend = FlutterSecureBackend(
      storage: keychain,
      macOS: true,
      keychainOptions: signed,
      log: logs.add,
    );
    await backend.write('app/server_address', 'https://chat.example.com');
    expect(keychain.items[true], {
      'app/server_address': 'https://chat.example.com',
    });
    expect(keychain.items[false], isEmpty);
    expect(await backend.downgraded, isFalse);
    expect(logs, isEmpty);
  });

  test('reads before any write already use the login keychain', () async {
    // A restart of an unentitled app: the secrets of the last run sit in the login keychain,
    // and the Data Protection keychain answers every read with nothing.
    final keychain = FakeKeychainStorage(missingEntitlement: true);
    keychain.items[false]!['app/server_address'] = 'https://chat.example.com';
    final backend = FlutterSecureBackend(
      storage: keychain,
      macOS: true,
      keychainOptions: signed,
      log: (_) {},
    );
    expect(
      await backend.read('app/server_address'),
      'https://chat.example.com',
    );
  });

  test('other platforms and the login keychain builds never probe', () async {
    for (final (macOS, options) in [
      (false, signed),
      (true, const MacOsOptions(usesDataProtectionKeychain: false)),
    ]) {
      final keychain = FakeKeychainStorage(missingEntitlement: true);
      final backend = FlutterSecureBackend(
        storage: keychain,
        macOS: macOS,
        keychainOptions: options,
        log: (_) {},
      );
      expect(await backend.read('x'), isNull);
      expect(keychain.writes, isEmpty);
      expect(await backend.downgraded, isFalse);
    }
  });

  test('only -34018 counts as a missing entitlement', () {
    expect(
      FlutterSecureBackend.isMissingEntitlement(
        PlatformException(
          code: 'Unexpected security result code',
          details: -34018,
        ),
      ),
      isTrue,
    );
    expect(
      FlutterSecureBackend.isMissingEntitlement(
        PlatformException(
          code: 'Unexpected security result code',
          details: -25308,
        ),
      ),
      isFalse,
    );
    expect(FlutterSecureBackend.isMissingEntitlement(StateError('x')), isFalse);
  });

  test('another keychain failure of the probe is not hidden', () async {
    final backend = FlutterSecureBackend(
      storage: _FailingStorage(),
      macOS: true,
      keychainOptions: signed,
      log: (_) {},
    );
    await expectLater(backend.read('x'), throwsA(isA<PlatformException>()));
  });
}

class _FailingStorage extends Fake implements FlutterSecureStorage {
  @override
  Future<void> write({
    required String key,
    required String? value,
    AppleOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
  }) async => throw PlatformException(
    code: 'Unexpected security result code',
    details: -25308,
  );
}
