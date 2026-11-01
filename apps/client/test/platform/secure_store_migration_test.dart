// SPDX-License-Identifier: Apache-2.0

// The update of an installation from a build before the app scope
// (docs/decisions.d/FIX-secure-store-migration.md): the secrets and the device key reference of
// an instance move from the old place (the keychain service `flutter_secure_storage_service`,
// the keys without the app prefix) into the scope of the app identity, the old items stay until
// the first successful refresh with them, and a build of another app identity never takes them.
//
// The live case of 2026-10-31: after the update a macOS build without the Data Protection
// keychain (login keychain) started on the connect screen although its device was active on
// the server. `readAll` of the login keychain fails with -50 (errSecParam), so the move of the
// old secrets threw before the first call and every read of the store failed with it.

import 'dart:convert';
import 'dart:io';

import 'package:client/app/bootstrap.dart';
import 'package:client/app/discovery.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/lock/app_lock_controller.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/platform/device_key_signer.dart';
import 'package:client/platform/push_android.dart';
import 'package:client/platform/push_apple.dart';
import 'package:client/platform/secure_store.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart' show ConnectionPhase;
import 'package:sync_engine/testing.dart' show WorldSeed;

import '../support/fake_instance.dart';
import '../support/fakes.dart';
import '../support/secure_storage_fakes.dart';

const String _app = 'com.example.chat';
const String _otherApp = 'com.example.other';
const String _i1 = '0193f2a7-0000-7000-8000-0000000000a1';
const String _i2 = '0193f2a7-0000-7000-8000-0000000000a2';
const String _origin = 'https://chat.example.com';
const String _token = 'instance/$_i1/auth/$_origin/refresh_token';

/// One build of the app on one platform store.
typedef Build = ({
  String name,
  TargetPlatform platform,
  bool entitled,
  MacOsOptions keychain,
  bool legacyDataProtection,
});

const List<Build> _builds = [
  // The live case: a release build with an ad-hoc signature keeps its secrets in the login
  // keychain (docs/decisions.d/FIX-macos-keychain.md).
  (
    name: 'macOS without keychain entitlements',
    platform: TargetPlatform.macOS,
    entitled: false,
    keychain: dataProtectionOptions,
    legacyDataProtection: false,
  ),
  (
    name: 'macOS debug or profile build',
    platform: TargetPlatform.macOS,
    entitled: true,
    keychain: loginKeychainOptions,
    legacyDataProtection: false,
  ),
  (
    name: 'macOS signed release build',
    platform: TargetPlatform.macOS,
    entitled: true,
    keychain: dataProtectionOptions,
    legacyDataProtection: true,
  ),
  (
    name: 'iOS',
    platform: TargetPlatform.iOS,
    entitled: true,
    keychain: dataProtectionOptions,
    legacyDataProtection: false,
  ),
  (
    name: 'Android',
    platform: TargetPlatform.android,
    entitled: true,
    keychain: dataProtectionOptions,
    legacyDataProtection: false,
  ),
  (
    name: 'Windows',
    platform: TargetPlatform.windows,
    entitled: true,
    keychain: dataProtectionOptions,
    legacyDataProtection: false,
  ),
];

/// Whether [platform] keeps the device keys in a store every app of the person reaches, as
/// `productionPorts` decides (macOS and Windows, when the secrets store is shared).
bool _sharedKeys(TargetPlatform platform) =>
    platform == TargetPlatform.macOS || platform == TargetPlatform.windows;

/// Whether the secrets of [build] lie in a store every app of the person reaches: the macOS
/// login keychain, the Windows file.
bool _sharedStore(Build build) =>
    build.platform == TargetPlatform.windows ||
    (build.platform == TargetPlatform.macOS && !build.legacyDataProtection);

/// Whether [build] reads the old secrets by their names (the macOS login keychain).
bool _byName(Build build) =>
    build.platform == TargetPlatform.macOS && !build.legacyDataProtection;

/// The old items a refresh leaves: in the login keychain `app/instance`, which the move never
/// reads (docs/decisions.d/FIX-macos-migration-denied.md FIX-MMD/1).
Map<String, String> _afterRefresh(Build build, Map<String, String> items) => {
  for (final MapEntry(:key, :value) in items.entries)
    if (!_byName(build) || key != 'app/instance') key: value,
};

FlutterSecureBackend _updated(
  SharedStorage storage,
  Build build, {
  String appId = _app,
  required Future<Set<String>> Function() localInstances,
  List<String>? logs,
}) => FlutterSecureBackend(
  appId: appId,
  storage: storage,
  platform: build.platform,
  keychainOptions: build.keychain,
  localInstances: localInstances,
  legacyKeychain: storage.legacyKeychain,
  log: logs == null ? (_) {} : logs.add,
);

void main() {
  group('an installation with only the old secrets starts signed in after '
      'the update', () {
    late FakeInstance f;
    late ProviderContainer c;

    HttpOverrides? saved;

    setUp(() async {
      // The fake server runs on 127.0.0.1; the test binding's HTTP override would answer 400.
      saved = HttpOverrides.current;
      HttpOverrides.global = null;
      f = await FakeInstance.start();
    });
    tearDown(() async {
      c.dispose();
      await f.stop();
      HttpOverrides.global = saved;
    });

    AppPorts ports(SecureBackend secrets, DeviceKeyScopes scopes) => testPorts(
      dataRoot: f.root,
      secureBackend: secrets,
      deviceKeys: f.keys,
      deviceKeyScopes: scopes,
      discovery: ApiDiscoveryClient(
        platform: 'macos',
        appVersion: testBranding.appVersion,
        allowPlainHttp: true,
      ),
    );

    ProviderContainer container(AppPorts p) =>
        ProviderContainer(overrides: [appPortsProvider.overrideWithValue(p)]);

    OnboardingState state() => c.read(onboardingControllerProvider);

    for (final build in _builds) {
      test(build.name, () async {
        final storage = SharedStorage(build.platform, entitled: build.entitled);
        final old = LegacyBackend(
          storage,
          dataProtection: build.legacyDataProtection,
        );
        final instanceId = f.server.world.instanceId;
        final token =
            'instance/$instanceId/auth/${f.server.baseUri.origin}/refresh_token';

        // The build before the app scope signs in and quits.
        c = container(ports(old, const DeviceKeyScopes()));
        final control = await connected(c, f);
        await control.loginWithPassword(
          WorldSeed.alexLogin,
          WorldSeed.password,
          WorldSeed.totp,
        );
        await eventually(
          'live before the update',
          () => state().services?.engine.state.phase == ConnectionPhase.live,
        );
        expect(old.legacyItems, contains(token));
        c.dispose();

        // The update starts on the same platform store and data root.
        final logs = <String>[];
        final backend = _updated(
          storage,
          build,
          localInstances: () => InstanceServices.localInstances(f.root),
          logs: logs,
        );
        c = container(
          ports(
            backend,
            DeviceKeyScopes(
              appId: _sharedKeys(build.platform) ? _app : null,
              shared: () => backend.sharedStore,
            ),
          ),
        );
        c.read(onboardingControllerProvider);
        await eventually(
          'signed in after the update (logs: $logs)',
          () => state().step == OnboardingStep.signedIn,
        );
        await eventually(
          'live after the update',
          () => state().services?.engine.state.phase == ConnectionPhase.live,
        );
        expect(f.keys.scopes, {
          instanceId,
        }, reason: 'the session signs with the device key of the old build');
        // The first refresh with the old session succeeded, so the old place is empty.
        await eventually(
          'the old secrets removed after the first refresh',
          () => _afterRefresh(build, old.legacyItems).isEmpty,
        );
        expect(await backend.read(token), isNotNull);
      });
    }

    for (final build in _builds.where(_sharedStore)) {
      test('${build.name}: another app identity ignores the old secrets in '
          'the shared store', () async {
        final storage = SharedStorage(build.platform, entitled: build.entitled);
        final old = LegacyBackend(storage);
        c = container(ports(old, const DeviceKeyScopes()));
        final control = await connected(c, f);
        await control.loginWithPassword(
          WorldSeed.alexLogin,
          WorldSeed.password,
          WorldSeed.totp,
        );
        await eventually(
          'live',
          () => state().services?.engine.state.phase == ConnectionPhase.live,
        );
        c.dispose();
        final before = old.legacyItems;

        // Another bundle identifier: its own data root, the same platform store.
        final otherRoot = await Directory.systemTemp.createTemp('other-app');
        addTearDown(() => otherRoot.delete(recursive: true));
        storage.touched.clear();
        final other = _updated(
          storage,
          build,
          appId: _otherApp,
          localInstances: () => InstanceServices.localInstances(otherRoot),
        );
        c = container(
          testPorts(
            dataRoot: otherRoot,
            secureBackend: other,
            discovery: ApiDiscoveryClient(
              platform: 'macos',
              appVersion: testBranding.appVersion,
              allowPlainHttp: true,
            ),
          ),
        );
        c.read(onboardingControllerProvider);
        await Future<void>.delayed(const Duration(milliseconds: 300));
        expect(state().step, OnboardingStep.server);
        expect(state().services, isNull);
        expect(
          old.legacyItems,
          before,
          reason: 'the old app keeps its session',
        );
        expect(await other.keys(), isEmpty);
        if (build.platform != TargetPlatform.macOS) return;
        expect(
          storage.touched,
          isNot(contains(storage.placeOf(AppleOptions.defaultAccountName))),
          reason:
              'every read of the old service in the login keychain is an '
              'access prompt',
        );
      });
    }
  });

  group('the move of the old secrets', () {
    for (final build in _builds) {
      // The old place: on Apple the old service, elsewhere the keys without the app prefix.
      Map<String, String> legacyOf(SharedStorage storage) => Map.fromEntries(
        storage
            .at(
              AppleOptions.defaultAccountName,
              dataProtection: build.legacyDataProtection,
            )
            .entries
            .where((e) => !e.key.startsWith('$_app/')),
      );
      void addLegacy(SharedStorage storage, Map<String, String> items) =>
          storage
              .at(
                AppleOptions.defaultAccountName,
                dataProtection: build.legacyDataProtection,
              )
              .addAll(items);

      test('${build.name}: the old items stay until the first successful '
          'refresh', () async {
        final storage = SharedStorage(build.platform, entitled: build.entitled);
        addLegacy(storage, {
          _token: 'r-old',
          'instance/$_i1/auth/$_origin/device_id': 'd-1',
          'instance/$_i1/${SecureDatabaseKeySource.keyName}': 'k',
          'app/server_address': _origin,
          'app/instance': jsonEncode({'address': _origin, 'instance_id': _i1}),
        });
        final logs = <String>[];
        final backend = _updated(
          storage,
          build,
          localInstances: () async => {_i1},
          logs: logs,
        );
        expect(await backend.read(_token), 'r-old');
        expect(
          await backend.read(
            'instance/$_i1/${SecureDatabaseKeySource.keyName}',
          ),
          'k',
        );
        expect(await backend.read('app/server_address'), _origin);
        expect(legacyOf(storage), hasLength(5), reason: 'not refreshed yet');

        // Another write is no refresh.
        await backend.write('instance/$_i1/app_lock/enabled', 'true');
        expect(legacyOf(storage), hasLength(5));

        // The refresh with the old session adopts a new refresh token.
        await backend.write(_token, 'r-new');
        expect(_afterRefresh(build, legacyOf(storage)), isEmpty);
        expect(await backend.read(_token), 'r-new');
        expect(logs.join('\n'), contains('removed'));

        // A later start neither reads nor moves anything again.
        storage.touched.clear();
        addLegacy(storage, {_token: 'r-stale'});
        final next = _updated(
          storage,
          build,
          localInstances: () async => {_i1},
        );
        expect(await next.read(_token), 'r-new');
        if (build.platform == TargetPlatform.android ||
            build.platform == TargetPlatform.windows) {
          return; // one place for old and new keys there
        }
        expect(
          storage.touched,
          isNot(
            contains(
              storage.placeOf(
                AppleOptions.defaultAccountName,
                dataProtection: build.legacyDataProtection,
              ),
            ),
          ),
        );
      });

      test('${build.name}: the device key of the old build stays the key of '
          'the instance', () async {
        final storage = SharedStorage(build.platform, entitled: build.entitled);
        addLegacy(storage, {
          _token: 'r',
          'app/server_address': _origin,
          'app/instance': jsonEncode({'address': _origin, 'instance_id': _i1}),
        });
        final backend = _updated(
          storage,
          build,
          localInstances: () async => {_i1},
        );
        final scope = await DeviceKeyScopes(
          appId: _sharedKeys(build.platform) ? _app : null,
          shared: () => backend.sharedStore,
        ).resolve(InstanceSecrets(backend, _i1));
        expect(scope, _i1);
      });
    }

    for (final build in _builds.where(_sharedStore)) {
      test('${build.name}: only the instances under the app\'s own data root '
          'move out of a shared store', () async {
        final storage = SharedStorage(build.platform, entitled: build.entitled);
        final legacy = storage.at(AppleOptions.defaultAccountName);
        legacy.addAll({
          _token: 'r-1',
          'instance/$_i2/auth/$_origin/refresh_token': 'r-2',
          'app/server_address': _origin,
          'app/instance': jsonEncode({'address': _origin, 'instance_id': _i2}),
        });
        final backend = _updated(
          storage,
          build,
          localInstances: () async => {_i1},
        );
        expect(await backend.read(_token), 'r-1');
        expect(
          await backend.read('instance/$_i2/auth/$_origin/refresh_token'),
          isNull,
          reason: 'not an instance of this app',
        );
        expect(
          await backend.read('app/instance'),
          isNull,
          reason: 'the remembered instance is not one of this app',
        );
        // The login keychain never reads `app/instance`: there the remembered server belongs to
        // the instance of this app whose session lies under it (FIX-MMD/1).
        final serverMoves = _byName(build);
        expect(await backend.keys(), {
          _token,
          'instance/$_i1/${FlutterSecureBackend.deviceKeyScopeKey}',
          if (serverMoves) 'app/server_address',
        });
        await backend.write(_token, 'r-1b');
        expect(
          {
            for (final MapEntry(:key, :value) in legacy.entries)
              if (!key.startsWith('$_app/')) key: value,
          },
          {
            'instance/$_i2/auth/$_origin/refresh_token': 'r-2',
            if (!serverMoves) 'app/server_address': _origin,
            'app/instance': jsonEncode({
              'address': _origin,
              'instance_id': _i2,
            }),
          },
        );
      });
    }

    test(
      'an instance that already has a session in the scope keeps it',
      () async {
        final storage = SharedStorage(TargetPlatform.android);
        storage.at('')
          ..['$_app/$_token'] = 'r-scoped'
          ..[_token] = 'r-old'
          ..['instance/$_i1/database_key'] = 'k-old';
        final backend = _updated(
          storage,
          _builds[4],
          localInstances: () async => {},
        );
        expect(await backend.read(_token), 'r-scoped');
        expect(
          await backend.read('instance/$_i1/database_key'),
          isNull,
          reason: 'the new session has its own data',
        );
      },
    );

    test('a failed move falls back to enrollment, is logged and tried again '
        'at the next start', () async {
      final storage = SharedStorage(TargetPlatform.android)
        ..failReadAll = StateError('store unreadable');
      storage.at('')[_token] = 'r';
      final logs = <String>[];
      final backend = _updated(
        storage,
        _builds[4],
        localInstances: () async => {},
        logs: logs,
      );
      expect(await backend.read(_token), isNull);
      await backend.write('app/server_address', _origin);
      expect(logs.join('\n'), contains('store unreadable'));
      expect(storage.at(''), containsPair(_token, 'r'));

      storage.failReadAll = null;
      final next = _updated(
        storage,
        _builds[4],
        localInstances: () async => {},
      );
      expect(await next.read(_token), 'r');
    });
  });

  group('the login keychain cannot list its items', () {
    test(
      'the scope keeps an index, so keys, isEmpty and the wipe work',
      () async {
        final storage = SharedStorage(TargetPlatform.macOS);
        final backend = _updated(
          storage,
          _builds[1],
          localInstances: () async => {},
        );
        final secrets = InstanceSecrets(backend, _i1);
        expect(await secrets.isEmpty(), isTrue);
        await secrets.write('device_id', 'd');
        await InstanceSecrets(backend, _i2).write('device_id', 'e');
        await AppPreferences(backend).setServerAddress(Uri.parse(_origin));
        expect(await backend.keys(), {
          'instance/$_i1/device_id',
          'instance/$_i2/device_id',
          'app/server_address',
        });
        expect(await secrets.isEmpty(), isFalse);
        await secrets.deleteAll();
        expect(await secrets.read('device_id'), isNull);
        expect(await backend.keys(), {
          'instance/$_i2/device_id',
          'app/server_address',
        });
        // A new process reads the index back.
        final next = _updated(
          storage,
          _builds[1],
          localInstances: () async => {},
        );
        expect(await next.keys(), {
          'instance/$_i2/device_id',
          'app/server_address',
        });
      },
    );
  });

  test('the instance keys of a build before the app scope are the keys the '
      'app writes', () {
    expect(FlutterSecureBackend.legacyInstanceKeys, {
      SecureDatabaseKeySource.keyName,
      SecretAppLockSwitch.key,
      ApplePush.apnsTokenKey,
      AndroidPush.fcmTokenKey,
      OnboardingController.pendingKey,
    });
    expect(DeviceKeyScopes.keyName, FlutterSecureBackend.deviceKeyScopeKey);
  });

  test(
    'local instances are the instance folders under the data root',
    () async {
      final root = await Directory.systemTemp.createTemp('scope');
      addTearDown(() => root.delete(recursive: true));
      expect(await InstanceServices.localInstances(root), isEmpty);
      await Directory('${root.path}/instances').create();
      expect(await InstanceServices.localInstances(root), isEmpty);
      await Directory('${root.path}/instances/$_i1').create();
      await File('${root.path}/instances/stray.txt').create();
      expect(await InstanceServices.localInstances(root), {_i1});
    },
  );
}
