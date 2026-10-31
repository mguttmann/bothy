// SPDX-License-Identifier: Apache-2.0

// The device keys of one app identity (docs/decisions.d/FIX-app-logout.md FIX-AL/4): where the
// key store is shared between apps (the login keychain of a macOS debug or profile build, the
// key storage of the Windows user), a build with another bundle identifier never signs with,
// or deletes, the device keys of an earlier one.
//
// The case: after a bundle change the new app found the old app's keys of the same instance
// (scope `instance_id`), sent the old key's JWK with the enrollment and got 409 `/public_jwk`
// (a thumbprint belongs to one device for good), and a wipe of either app deleted the other
// app's keys.

import 'dart:io';

import 'package:client/app/bootstrap.dart';
import 'package:client/app/discovery.dart';
import 'package:client/platform/device_key_signer.dart';
import 'package:client/platform/secure_store.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';
import '../support/software_keys.dart';

const String _instanceId = '0193f2a7-0000-7000-8000-0000000000bb';
const String _newApp = 'com.example.other';

InstanceInfo _instance() => InstanceInfo(
  address: Uri.parse('https://chat.example.com'),
  instanceId: _instanceId,
  productName: 'Team Chat',
);

void main() {
  late Directory root;
  // One key store that every app of the person reaches, holding the old app's keys.
  late SoftwareKeyStore shared;
  late String oldThumbprint;

  setUp(() async {
    root = await Directory.systemTemp.createTemp('device_key_scope');
    shared = SoftwareKeyStore();
    oldThumbprint = (await shared.ensure(_instanceId)).dpop.thumbprint;
  });
  tearDown(() async {
    if (await root.exists()) await root.delete(recursive: true);
  });

  Future<InstanceServices> open(SecureBackend secrets) => InstanceServices.open(
    testPorts(
      dataRoot: root,
      secureBackend: secrets,
      deviceKeys: shared,
      deviceKeyScopes: const DeviceKeyScopes(appId: _newApp),
    ),
    _instance(),
  );

  test('a new app identity enrolls with keys of its own and its wipe keeps the old app keys', () async {
    final services = await open(MemorySecureBackend());
    expect(shared.scopes, {_instanceId, '$_newApp/$_instanceId'});
    final keys = await shared.ensure('$_newApp/$_instanceId');
    expect(keys.dpop.thumbprint, isNot(oldThumbprint));

    await services.wiper.wipe();

    expect(shared.scopes, {_instanceId});
    expect((await shared.ensure(_instanceId)).dpop.thumbprint, oldThumbprint);
    await services.auth.dispose();
  });

  test('the scope stays the same on the next start', () async {
    final backend = MemorySecureBackend();
    final first = await open(backend);
    await first.auth.dispose();
    await first.engine.dispose();
    await first.database.close();

    final second = await open(backend);
    expect(shared.scopes, {_instanceId, '$_newApp/$_instanceId'});
    await second.auth.dispose();
  });

  test(
    'the same app after an update keeps the keys of its session and wipes them',
    () async {
      final backend = MemorySecureBackend();
      // The secrets of the earlier build, already moved into this app's scope.
      await InstanceSecrets(
        backend,
        _instanceId,
      ).write('auth/https://chat.example.com/device_id', 'd-1');

      final services = await open(backend);
      expect(shared.scopes, {_instanceId});
      expect(
        await InstanceSecrets(
          backend,
          _instanceId,
        ).read(DeviceKeyScopes.keyName),
        _instanceId,
      );

      await services.wiper.wipe();

      expect(shared.scopes, isEmpty);
      await services.auth.dispose();
    },
  );

  test('in a store of the app alone the scope is the instance ID', () async {
    final backend = MemorySecureBackend();
    final scope = await DeviceKeyScopes(
      appId: _newApp,
      shared: () async => false,
    ).resolve(InstanceSecrets(backend, _instanceId));
    expect(scope, _instanceId);
    expect(backend.values, isEmpty);
  });

  test('without an app identity the scope is the instance ID', () async {
    final backend = MemorySecureBackend();
    final scope = await const DeviceKeyScopes().resolve(
      InstanceSecrets(backend, _instanceId),
    );
    expect(scope, _instanceId);
    expect(backend.values, isEmpty);
  });
}
