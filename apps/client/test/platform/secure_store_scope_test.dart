// SPDX-License-Identifier: Apache-2.0

// The secrets of one app identity (docs/decisions.d/FIX-app-logout.md): a build with another
// bundle identifier or application ID never reads, lists or deletes the secrets of an earlier
// one, also where the platform store is shared between apps (the login keychain of a macOS
// debug or profile build, the DPAPI file of the Windows build under one support folder).
//
// The live case: a macOS profile build under a new bundle identifier read the refresh token
// of the earlier bundle from the login keychain (5 access prompts), sent it, got 401
// `session_revoked`, wiped its data and showed the enrollment screen.

import 'dart:io';

import 'package:client/app/bootstrap.dart';
import 'package:client/platform/secure_store.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

const _oldApp = 'com.example.chat';
const _newApp = 'com.example.other';
const _token = 'instance/i-1/auth/https://chat.example.com/refresh_token';

/// One store that every app of the person shares, as the login keychain of a macOS build
/// without the Data Protection keychain does: Apple items are told apart by their service
/// (`accountName`) only, the other platforms by their key only.
class SharedStorage extends Fake implements FlutterSecureStorage {
  SharedStorage(this.platform);

  final TargetPlatform platform;

  /// The items per service (Apple) or under "" (other platforms).
  final Map<String, Map<String, String>> items = {};

  /// The service of every read, list and delete, in order: a read of another app's service
  /// would show a keychain access prompt on macOS.
  final List<String> touched = [];

  bool get _apple =>
      platform == TargetPlatform.iOS || platform == TargetPlatform.macOS;

  String _service(AppleOptions? i, AppleOptions? m) {
    if (!_apple) return '';
    final options = platform == TargetPlatform.iOS ? i : m;
    return options?.accountName ?? AppleOptions.defaultAccountName;
  }

  Map<String, String> _items(String service) =>
      items.putIfAbsent(service, () => {});

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
    _items(_service(iOptions, mOptions))[key] = value!;
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
    final service = _service(iOptions, mOptions);
    touched.add(service);
    return _items(service)[key];
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
    final service = _service(iOptions, mOptions);
    touched.add(service);
    _items(service).remove(key);
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
    final service = _service(iOptions, mOptions);
    touched.add(service);
    return Map.of(_items(service));
  }
}

/// The macOS options of a debug or profile build: the login keychain.
const _loginKeychain = MacOsOptions(
  accessibility: KeychainAccessibility.first_unlock_this_device,
  synchronizable: false,
  usesDataProtectionKeychain: false,
);

/// The macOS options of a signed release build: the Data Protection keychain.
const _dataProtection = MacOsOptions(
  accessibility: KeychainAccessibility.first_unlock_this_device,
  synchronizable: false,
);

FlutterSecureBackend _backend(
  SharedStorage storage,
  String appId, {
  MacOsOptions keychain = _loginKeychain,
  bool hasLocalData = false,
}) {
  final previous = debugDefaultTargetPlatformOverride;
  debugDefaultTargetPlatformOverride = storage.platform;
  addTearDown(() => debugDefaultTargetPlatformOverride = previous);
  return FlutterSecureBackend(
    appId: appId,
    storage: storage,
    platform: storage.platform,
    keychainOptions: keychain,
    hasLocalData: () async => hasLocalData,
    log: (_) {},
  );
}

void main() {
  const all = [
    TargetPlatform.macOS,
    TargetPlatform.windows,
    TargetPlatform.iOS,
    TargetPlatform.android,
  ];

  for (final platform in all) {
    test('$platform: a new app identity never sees the session of another '
        'one in a shared store', () async {
      final storage = SharedStorage(platform);
      final old = _backend(storage, _oldApp, hasLocalData: true);
      await old.write(_token, 'r-old');
      await old.write('app/server_address', 'https://chat.example.com');

      final fresh = _backend(storage, _newApp);
      expect(await fresh.read(_token), isNull);
      expect(await fresh.read('app/server_address'), isNull);
      expect(await fresh.keys(), isEmpty);

      // Its own secrets stay its own, and the old app keeps its session.
      await fresh.write(_token, 'r-new');
      expect(await fresh.read(_token), 'r-new');
      expect(await old.read(_token), 'r-old');
      expect(await old.keys(), {_token, 'app/server_address'});
    });
  }

  group('the secrets of a build before the app scope', () {
    bool apple(SharedStorage storage) =>
        storage.platform == TargetPlatform.iOS ||
        storage.platform == TargetPlatform.macOS;

    /// The place a build before the app scope wrote to.
    Map<String, String> legacy(SharedStorage storage) =>
        storage.items.putIfAbsent(
          apple(storage) ? AppleOptions.defaultAccountName : '',
          () => {},
        );

    /// What is left of the old secrets: on Apple the whole old service, elsewhere the keys
    /// without an app prefix.
    Map<String, String> left(SharedStorage storage) => {
      for (final MapEntry(:key, :value) in legacy(storage).entries)
        if (apple(storage) ||
            FlutterSecureBackend.legacyPrefixes.any(key.startsWith))
          key: value,
    };

    for (final (platform, keychain) in [
      (TargetPlatform.iOS, _loginKeychain),
      (TargetPlatform.android, _loginKeychain),
      (TargetPlatform.macOS, _dataProtection),
    ]) {
      test('$platform: a store only this app reaches moves into the app '
          'scope, also without local data (a reinstall)', () async {
        final storage = SharedStorage(platform);
        legacy(storage).addAll({_token: 'r', 'app/instance': '{}'});
        final backend = _backend(storage, _oldApp, keychain: keychain);
        expect(await backend.read(_token), 'r');
        expect(await backend.keys(), {_token, 'app/instance'});
        expect(
          left(storage),
          isEmpty,
          reason:
              'a copy left behind would be a stale refresh token another build '
              'could send',
        );
      });
    }

    for (final (platform, keychain) in [
      (TargetPlatform.macOS, _loginKeychain),
      (TargetPlatform.windows, _loginKeychain),
    ]) {
      test('$platform: a shared store is not touched by an app without '
          'local data', () async {
        final storage = SharedStorage(platform);
        legacy(storage)[_token] = 'r-of-another-bundle';
        final backend = _backend(storage, _newApp, keychain: keychain);
        expect(await backend.read(_token), isNull);
        expect(await backend.keys(), isEmpty);
        expect(left(storage), {_token: 'r-of-another-bundle'});
        if (platform == TargetPlatform.macOS) {
          expect(
            storage.touched,
            isNot(contains(AppleOptions.defaultAccountName)),
            reason: 'every read of the old service is a keychain prompt',
          );
        }
      });

      test('$platform: a shared store moves into the scope of an app that '
          'has local data (the same app after the update)', () async {
        final storage = SharedStorage(platform);
        legacy(storage)[_token] = 'r';
        final backend = _backend(
          storage,
          _oldApp,
          keychain: keychain,
          hasLocalData: true,
        );
        expect(await backend.read(_token), 'r');
        expect(left(storage), isEmpty);
      });
    }

    test(
      'the move happens once: a later start never reads the old store',
      () async {
        final storage = SharedStorage(TargetPlatform.macOS);
        legacy(storage)[_token] = 'r';
        await _backend(storage, _oldApp, hasLocalData: true).read(_token);
        legacy(storage)['instance/i-2/device_id'] = 'late';
        storage.touched.clear();
        final next = _backend(storage, _oldApp, hasLocalData: true);
        expect(await next.keys(), {_token});
        expect(
          storage.touched,
          isNot(contains(AppleOptions.defaultAccountName)),
        );
      },
    );

    test('a scoped value is not replaced by an older one', () async {
      final storage = SharedStorage(TargetPlatform.iOS);
      final backend = _backend(storage, _oldApp);
      storage.items[FlutterSecureBackend.serviceFor(_oldApp)] = {
        _token: 'r-new',
      };
      legacy(storage)[_token] = 'r-old';
      expect(await backend.read(_token), 'r-new');
      expect(left(storage), isEmpty);
    });
  });

  test('local data is an instance folder under the data root', () async {
    final root = await Directory.systemTemp.createTemp('scope');
    addTearDown(() => root.delete(recursive: true));
    expect(await InstanceServices.hasLocalData(root), isFalse);
    await Directory('${root.path}/instances').create();
    expect(await InstanceServices.hasLocalData(root), isFalse);
    await Directory('${root.path}/instances/i-1').create();
    expect(await InstanceServices.hasLocalData(root), isTrue);
  });
}
