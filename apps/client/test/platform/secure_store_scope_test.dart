// SPDX-License-Identifier: Apache-2.0

// The secrets of one app identity (docs/decisions.d/FIX-app-logout.md): a build with another
// bundle identifier or application ID never reads, lists or deletes the secrets of an earlier
// one, also where the platform store is shared between apps (the login keychain of a macOS
// debug or profile build, the DPAPI file of the Windows build under one support folder).
//
// The live case: a macOS profile build under a new bundle identifier read the refresh token
// of the earlier bundle from the login keychain (5 access prompts), sent it, got 401
// `session_revoked`, wiped its data and showed the enrollment screen.

import 'dart:convert';

import 'package:client/platform/secure_store.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/secure_storage_fakes.dart';

const _oldApp = 'com.example.chat';
const _newApp = 'com.example.other';
const _origin = 'https://chat.example.com';
const _token = 'instance/i-1/auth/$_origin/refresh_token';

/// The server and instance a build before the app scope remembered.
final Map<String, String> _remembered = {
  'app/server_address': _origin,
  'app/instance': jsonEncode({'address': _origin, 'instance_id': 'i-1'}),
};

FlutterSecureBackend _backend(
  SharedStorage storage,
  String appId, {
  MacOsOptions keychain = loginKeychainOptions,
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
    // The instance of [_token] has its folder under the app's data root.
    localInstances: () async => hasLocalData ? {'i-1'} : {},
    legacyKeychain: storage.legacyKeychain,
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
    Map<String, String> legacy(SharedStorage storage, [bool dp = false]) =>
        storage.at(AppleOptions.defaultAccountName, dataProtection: dp);

    /// What is left of the old secrets: on Apple the whole old service, elsewhere the keys
    /// without an app prefix.
    Map<String, String> left(SharedStorage storage, [bool dp = false]) => {
      for (final MapEntry(:key, :value) in legacy(storage, dp).entries)
        if (apple(storage) ||
            FlutterSecureBackend.legacyPrefixes.any(key.startsWith))
          key: value,
    };

    for (final (platform, keychain) in [
      (TargetPlatform.iOS, loginKeychainOptions),
      (TargetPlatform.android, loginKeychainOptions),
      (TargetPlatform.macOS, dataProtectionOptions),
    ]) {
      test('$platform: a store only this app reaches moves into the app '
          'scope, also without local data (a reinstall)', () async {
        final storage = SharedStorage(platform);
        final dp = keychain.usesDataProtectionKeychain;
        legacy(storage, dp).addAll({_token: 'r', 'app/instance': '{}'});
        final backend = _backend(storage, _oldApp, keychain: keychain);
        expect(await backend.read(_token), 'r');
        expect(await backend.keys(), {_token, 'app/instance'});
        expect(left(storage, dp), {
          _token: 'r',
          'app/instance': '{}',
        }, reason: 'the old items go after the first refresh (FIX-SSM/3)');
        await backend.write(_token, 'r2');
        expect(left(storage, dp), isEmpty);
      });
    }

    for (final (platform, keychain) in [
      (TargetPlatform.macOS, loginKeychainOptions),
      (TargetPlatform.windows, loginKeychainOptions),
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
            isNot(contains(storage.placeOf(AppleOptions.defaultAccountName))),
            reason: 'every read of the old service is a keychain prompt',
          );
        }
      });

      test('$platform: a shared store moves into the scope of an app that '
          'has local data (the same app after the update)', () async {
        final storage = SharedStorage(platform);
        legacy(storage).addAll({_token: 'r', ..._remembered});
        final backend = _backend(
          storage,
          _oldApp,
          keychain: keychain,
          hasLocalData: true,
        );
        expect(await backend.read(_token), 'r');
        expect(left(storage), {_token: 'r', ..._remembered});
        await backend.write(_token, 'r2');
        // The login keychain: `app/instance` is never read, so never moved or deleted
        // (docs/decisions.d/FIX-macos-migration-denied.md).
        expect(
          left(storage),
          platform == TargetPlatform.macOS
              ? {'app/instance': _remembered['app/instance']}
              : isEmpty,
        );
      });
    }

    test(
      'the move happens once: a later start never reads the old store',
      () async {
        final storage = SharedStorage(TargetPlatform.macOS);
        legacy(storage).addAll({_token: 'r', ..._remembered});
        await _backend(storage, _oldApp, hasLocalData: true).read(_token);
        legacy(storage)['instance/i-2/device_id'] = 'late';
        storage.touched.clear();
        final next = _backend(storage, _oldApp, hasLocalData: true);
        expect(await next.keys(), {
          _token,
          'app/server_address',
          'instance/i-1/${FlutterSecureBackend.deviceKeyScopeKey}',
        });
        expect(
          storage.touched,
          isNot(contains(storage.placeOf(AppleOptions.defaultAccountName))),
        );
      },
    );

    test('a scoped value is not replaced by an older one', () async {
      final storage = SharedStorage(TargetPlatform.iOS);
      final backend = _backend(storage, _oldApp);
      storage.at(FlutterSecureBackend.serviceFor(_oldApp))[_token] = 'r-new';
      legacy(storage)[_token] = 'r-old';
      expect(await backend.read(_token), 'r-new');
      expect(left(storage), {
        _token: 'r-old',
      }, reason: 'an instance with a session in the scope takes nothing old');
    });
  });

  test(
    'the store is shared in the login keychain and on Windows only',
    () async {
      Future<bool> shared(TargetPlatform platform, [MacOsOptions? keychain]) =>
          _backend(
            SharedStorage(platform),
            _oldApp,
            keychain: keychain ?? loginKeychainOptions,
          ).sharedStore;
      expect(await shared(TargetPlatform.macOS), isTrue);
      expect(await shared(TargetPlatform.windows), isTrue);
      expect(
        await shared(TargetPlatform.macOS, dataProtectionOptions),
        isFalse,
      );
      expect(await shared(TargetPlatform.iOS), isFalse);
      expect(await shared(TargetPlatform.android), isFalse);
    },
  );
}
