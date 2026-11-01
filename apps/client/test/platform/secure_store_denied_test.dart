// SPDX-License-Identifier: Apache-2.0

// A refused old item never loses the session (docs/decisions.d/FIX-macos-migration-denied.md).
//
// The live case of 2026-11-01: the macOS app with the move into the app scope (FIX-secure-store-
// migration, FIX-macos-legacy-keychain-prompt) did not take over the owner's session. The build
// before the scope reads `app/server_address`, the database key and the session of the
// remembered server at every start, so the keychain gives them to it; `app/instance` it only
// writes after discovery and reads only when discovery fails, so that item still trusted only the
// app of an older bundle identifier. The move read `app/instance` as well, the keychain refused it
// (user interaction off), one refusal made the whole move "not migratable", the move wrote its
// final marker and the app started with enrollment. Every later start skipped the move because of
// the marker. The tests below hold the class: the move reads only what the build before the scope
// read, a refusal is never final, and a marker without a session in the scope does not block a
// retry.

import 'dart:convert';

import 'package:client/platform/secure_store.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart' show PlatformException;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/secure_storage_fakes.dart';

const String _app = 'com.example.chat';
const String _i1 = '0193f2a7-0000-7000-8000-0000000000a1';
const String _origin = 'https://chat.example.com';
const String _token = 'instance/$_i1/auth/$_origin/refresh_token';
const String _deviceId = 'instance/$_i1/auth/$_origin/device_id';
const String _dbKey = 'instance/$_i1/database_key';
const String _keyScope =
    'instance/$_i1/${FlutterSecureBackend.deviceKeyScopeKey}';
const String _marker = FlutterSecureBackend.scopeMarkerKey;

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

final String _instanceJson = jsonEncode({
  'instance_id': _i1,
  'address': _origin,
});

/// The old place of [storage] (the login keychain on macOS).
Map<String, String> _old(SharedStorage storage) =>
    storage.at(AppleOptions.defaultAccountName);

/// The app scope of [storage] in the login keychain.
Map<String, String> _scope(SharedStorage storage) =>
    storage.at(FlutterSecureBackend.serviceFor(_app));

/// The secrets the build before the app scope left in the login keychain, written the way it
/// wrote them; `app/instance` trusts only the app of an older bundle identifier ([foreign]).
void _ownerOldItems(SharedStorage storage) {
  _old(storage)
    ..['app/server_address'] = _origin
    ..['app/instance'] = _instanceJson
    ..[_dbKey] = 'k-old'
    ..[_deviceId] = 'd-old'
    ..[_token] = 'r-old';
  storage.foreign.add('app/instance');
}

/// What the build of the failed move left in the scope: the final marker, a new database key and
/// device key scope, the server and instance of its discovery, no session.
void _failedMoveLeftovers(SharedStorage storage) {
  const keys = [_keyScope, _dbKey, 'app/server_address', 'app/instance'];
  _scope(storage)
    ..[_marker] = _app
    ..[FlutterSecureBackend.indexKey] = jsonEncode(keys)
    ..[_keyScope] = '$_app/$_i1'
    ..[_dbKey] = 'k-new'
    ..['app/server_address'] = _origin
    ..['app/instance'] = _instanceJson;
}

FlutterSecureBackend _backend(
  SharedStorage storage,
  MacOsOptions keychain, {
  List<String>? logs,
  Future<Set<String>> Function()? localInstances,
}) => FlutterSecureBackend(
  appId: _app,
  storage: storage,
  platform: storage.platform,
  keychainOptions: keychain,
  localInstances: localInstances ?? () async => {_i1},
  legacyKeychain: storage.legacyKeychain,
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
  test('the app keys the move reads in the login keychain are the ones the '
      'build before the scope read at every start', () {
    expect(FlutterSecureBackend.legacyReadAppKeys, [
      'app/server_address',
      'app/update_notice_dismissed',
    ]);
    expect(
      FlutterSecureBackend.legacyReadAppKeys,
      isNot(contains('app/instance')),
      reason: 'written after discovery, read only when discovery fails',
    );
  });

  for (final MapEntry(key: name, value: build) in _loginBuilds.entries) {
    group(name, () {
      test('the owner\'s update: the session moves although app/instance '
          'trusts only another app, and app/instance is never read', () async {
        final storage = SharedStorage(
          TargetPlatform.macOS,
          entitled: build.entitled,
        );
        _ownerOldItems(storage);
        final logs = <String>[];

        final backend = _backend(storage, build.keychain, logs: logs);
        expect(await _firstRead(backend), _origin);
        expect(await backend.read(_token), 'r-old');
        expect(await backend.read(_deviceId), 'd-old');
        expect(await backend.read(_dbKey), 'k-old');
        expect(await backend.read(_keyScope), _i1);
        expect(storage.legacyKeychain.calls, isNot(contains('app/instance')));
        expect(storage.pluginLegacyCalls, isEmpty);
        expect(logs.join('\n'), isNot(contains('refuses')));
        expect(await backend.read(_marker), _app);
        expect(_old(storage), hasLength(5), reason: 'not refreshed yet');

        // The first refresh removes the copied old items; the one never read stays.
        await backend.write(_token, 'r-new');
        expect(_old(storage), {'app/instance': _instanceJson});
      });

      test('the owner\'s Mac after the failed move: the marker without a '
          'session in the scope does not block the retry, the old secrets '
          'replace the leftovers', () async {
        final storage = SharedStorage(
          TargetPlatform.macOS,
          entitled: build.entitled,
        );
        _ownerOldItems(storage);
        _failedMoveLeftovers(storage);

        final backend = _backend(storage, build.keychain);
        expect(await _firstRead(backend), _origin);
        expect(await backend.read(_token), 'r-old');
        expect(await backend.read(_deviceId), 'd-old');
        expect(
          await backend.read(_dbKey),
          'k-old',
          reason: 'the local database of the instance has the old key',
        );
        expect(
          await backend.read(_keyScope),
          _i1,
          reason: 'the session signs with the old build\'s device key',
        );
        expect(await backend.keys(), containsAll([_token, _deviceId]));
        expect(_old(storage), hasLength(5), reason: 'not refreshed yet');

        await backend.write(_token, 'r-new');
        expect(_old(storage), {'app/instance': _instanceJson});
      });

      test('a refused session item: the instance is not copied, no marker, '
          'logged, and the next start tries again', () async {
        final storage = SharedStorage(
          TargetPlatform.macOS,
          entitled: build.entitled,
        );
        _ownerOldItems(storage);
        storage.foreign.add(_token);
        final logs = <String>[];

        final backend = _backend(storage, build.keychain, logs: logs);
        expect(await _firstRead(backend), isNull);
        expect(await backend.keys(), isEmpty, reason: 'no half session');
        expect(await backend.read(_marker), isNull);
        expect(
          logs.where((l) => l.contains('refuses')),
          hasLength(1),
          reason: '$logs',
        );
        expect(logs.join('\n'), contains('the next start tries again'));
        expect(logs.join('\n'), isNot(contains('not migratable')));
        expect(_old(storage), hasLength(5), reason: 'nothing deleted');

        // The keychain gives the item out at the next start (an unlock, an access granted).
        storage.foreign.remove(_token);
        final next = _backend(storage, build.keychain);
        expect(await _firstRead(next), _origin);
        expect(await next.read(_token), 'r-old');
        expect(await next.read(_marker), _app);
      });

      test('every refusal status leaves the move to the next start', () async {
        for (final status in [-25293, -25308, -25243, -128, -25244, -1]) {
          final storage = SharedStorage(
            TargetPlatform.macOS,
            entitled: build.entitled,
          );
          _ownerOldItems(storage);
          storage.foreign.add(_dbKey);
          storage.legacyKeychain.foreignReadStatus = status;

          final backend = _backend(storage, build.keychain);
          await _firstRead(backend);
          expect(await backend.read(_token), isNull, reason: '$status');
          expect(await backend.read(_marker), isNull, reason: '$status');
          expect(_old(storage), hasLength(5), reason: '$status');
        }
      });

      test('a refused item besides the session: the session moves, no '
          'marker until nothing is left to read', () async {
        final storage = SharedStorage(
          TargetPlatform.macOS,
          entitled: build.entitled,
        );
        _ownerOldItems(storage);
        _old(storage)['app/update_notice_dismissed'] = '1.2.0';
        storage.foreign.add('app/update_notice_dismissed');

        final backend = _backend(storage, build.keychain);
        expect(await _firstRead(backend), _origin);
        expect(await backend.read(_token), 'r-old');
        expect(await backend.read('app/update_notice_dismissed'), isNull);
        expect(await backend.read(_marker), isNull);

        // The session is in the scope now: the next start reads nothing old and settles.
        storage.legacyKeychain.calls.clear();
        final next = _backend(storage, build.keychain);
        expect(await _firstRead(next), _origin);
        expect(storage.legacyKeychain.calls, isEmpty);
        expect(await next.read(_marker), _app);
      });

      test('a marker with a session in the scope: the old place is not '
          'read', () async {
        final storage = SharedStorage(
          TargetPlatform.macOS,
          entitled: build.entitled,
        );
        _ownerOldItems(storage);
        _failedMoveLeftovers(storage);
        final scope = _scope(storage);
        scope[_token] = 'r-enrolled';
        scope[FlutterSecureBackend.indexKey] = jsonEncode([
          _keyScope,
          _dbKey,
          _token,
          'app/server_address',
          'app/instance',
        ]);

        final backend = _backend(storage, build.keychain);
        expect(await backend.read(_token), 'r-enrolled');
        expect(await backend.read(_dbKey), 'k-new');
        expect(storage.legacyKeychain.calls, isEmpty);
        expect(_old(storage), hasLength(5));
      });

      test('the server address goes with the own instance whose session lies '
          'under it; without such a session it stays', () async {
        final storage = SharedStorage(
          TargetPlatform.macOS,
          entitled: build.entitled,
        );
        _old(storage)
          ..['app/server_address'] = _origin
          ..[_dbKey] = 'k-old';

        final backend = _backend(storage, build.keychain);
        expect(await backend.read(_dbKey), 'k-old');
        expect(
          await backend.read('app/server_address'),
          isNull,
          reason: 'no session of this app under the remembered server',
        );
      });

      test('a later move keeps the pending list of an earlier one', () async {
        const i2 = '0193f2a7-0000-7000-8000-0000000000a2';
        const token2 = 'instance/$i2/auth/$_origin/refresh_token';
        final storage = SharedStorage(
          TargetPlatform.macOS,
          entitled: build.entitled,
        );
        _ownerOldItems(storage);
        _old(storage)['instance/$i2/database_key'] = 'k2';
        _old(storage)[token2] = 'r2';
        storage.foreign.add(token2);

        final first = _backend(
          storage,
          build.keychain,
          localInstances: () async => {_i1, i2},
        );
        expect(await first.read(_token), 'r-old');
        expect(await first.read(token2), isNull);

        storage.foreign.remove(token2);
        final second = _backend(
          storage,
          build.keychain,
          localInstances: () async => {_i1, i2},
        );
        expect(await second.read(token2), 'r2');
        expect(await second.read(_token), 'r-old', reason: 'not copied twice');
        final pending = jsonDecode(
          _scope(storage)[FlutterSecureBackend.legacyPendingKey]!,
        ) as Map<String, Object?>;
        expect(pending.keys, containsAll([_i1, i2]));

        await second.write(_token, 'r-new');
        expect(_old(storage), isNot(contains(_token)));
        expect(_old(storage), contains(token2), reason: 'i2 not refreshed');
      });

      test('a marker and the session of one instance in the scope: another '
          'own instance with a refused session still moves at a later '
          'start', () async {
        const i2 = '0193f2a7-0000-7000-8000-0000000000a2';
        const token2 = 'instance/$i2/auth/$_origin/refresh_token';
        final storage = SharedStorage(
          TargetPlatform.macOS,
          entitled: build.entitled,
        );
        _ownerOldItems(storage);
        _failedMoveLeftovers(storage);
        _old(storage)['instance/$i2/database_key'] = 'k2';
        _old(storage)[token2] = 'r2';
        storage.foreign.add(token2);
        Future<Set<String>> both() async => {_i1, i2};

        final first = _backend(storage, build.keychain, localInstances: both);
        expect(await first.read(_token), 'r-old');
        expect(await first.read(token2), isNull);
        expect(await first.read(_marker), _app, reason: 'the old marker');

        storage.foreign.remove(token2);
        final second = _backend(storage, build.keychain, localInstances: both);
        expect(await second.read(token2), 'r2');
        expect(await second.read('instance/$i2/database_key'), 'k2');
      });
    });
  }

  group('the shared rules on the other platforms', () {
    test('iOS: a refused list of the old items writes no marker and the next '
        'start tries again', () async {
      final storage = SharedStorage(TargetPlatform.iOS)
        ..failReadAll = PlatformException(
          code: 'Unexpected security result code',
          details: -25308,
        );
      storage.at(AppleOptions.defaultAccountName)
        ..[_token] = 'r'
        ..[_dbKey] = 'k';
      final logs = <String>[];
      final backend = _backend(storage, dataProtectionOptions, logs: logs);
      expect(await backend.read(_token), isNull);
      expect(await backend.read(_marker), isNull);
      expect(logs.join('\n'), contains('the next start tries again'));

      storage.failReadAll = null;
      final next = _backend(storage, dataProtectionOptions);
      expect(await next.read(_token), 'r');
      expect(await next.read(_marker), _app);
    });

    for (final platform in [
      TargetPlatform.iOS,
      TargetPlatform.android,
      TargetPlatform.windows,
    ]) {
      final prefix = platform == TargetPlatform.iOS ? '' : '$_app/';
      Map<String, String> scope(SharedStorage s) =>
          platform == TargetPlatform.iOS
          ? s.at(FlutterSecureBackend.serviceFor(_app))
          : s.at('');
      Map<String, String> old(SharedStorage s) =>
          s.at(AppleOptions.defaultAccountName);

      test('$platform: a marker without a session and a half copy do not '
          'block the move', () async {
        final storage = SharedStorage(platform);
        old(storage)
          ..[_token] = 'r'
          ..[_dbKey] = 'k-old';
        scope(storage)
          ..['$prefix$_marker'] = _app
          ..['$prefix$_dbKey'] = 'k-half';

        final backend = _backend(storage, dataProtectionOptions);
        expect(await backend.read(_token), 'r');
        expect(await backend.read(_dbKey), 'k-old');
      });

      test('$platform: a marker with a session keeps the scope', () async {
        final storage = SharedStorage(platform);
        old(storage)
          ..[_token] = 'r-old'
          ..[_dbKey] = 'k-old';
        scope(storage)
          ..['$prefix$_marker'] = _app
          ..['$prefix$_token'] = 'r-new'
          ..['$prefix$_dbKey'] = 'k-new';

        final backend = _backend(storage, dataProtectionOptions);
        expect(await backend.read(_token), 'r-new');
        expect(await backend.read(_dbKey), 'k-new');
      });
    }
  });
}
