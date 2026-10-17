// SPDX-License-Identifier: Apache-2.0

// The one wiper of every sign-out cause (brief M4-18 Design note 2, R-SY-14): after `wipe` no
// file of the instance and no secret of the instance remains; the server address stays.

import 'dart:io';

import 'package:client/app/account_wiper.dart';
import 'package:client/app/bootstrap.dart';
import 'package:client/app/discovery.dart';
import 'package:client/platform/secure_store.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';
import '../support/software_keys.dart';

const String instanceId = '0193f2a7-0000-7000-8000-0000000000aa';

InstanceInfo instance() => InstanceInfo(
  address: Uri.parse('https://chat.example.com'),
  instanceId: instanceId,
  productName: 'Team Chat',
);

void main() {
  late Directory root;

  setUp(() async => root = await Directory.systemTemp.createTemp('wiper'));
  tearDown(() async {
    if (await root.exists()) await root.delete(recursive: true);
  });

  test('after wipe no file and no secret of the instance remains; the address stays', () async {
    final backend = MemorySecureBackend();
    final keys = SoftwareKeyStore();
    final ports = testPorts(
      dataRoot: root,
      secureBackend: backend,
      deviceKeys: keys,
    );
    await AppPreferences(backend)
        .setServerAddress(Uri.parse('https://chat.example.com'));
    final services = await InstanceServices.open(ports, instance());
    await services.secrets.write(
      'auth/https://chat.example.com/refresh_token',
      'rt',
    );
    final folder = Directory('${root.path}/instances/$instanceId');
    await File('${folder.path}/files/cached.bin').create(recursive: true);
    expect(File('${folder.path}/local.db').existsSync(), isTrue);
    expect(keys.scopes, {instanceId});
    expect(
      backend.values.keys.where((k) => k.contains(instanceId)),
      isNotEmpty,
    );

    await services.wiper.wipe();

    expect(folder.existsSync(), isFalse);
    expect(keys.scopes, isEmpty);
    expect(backend.values.keys.where((k) => k.contains(instanceId)), isEmpty);
    expect(
      await AppPreferences(backend).serverAddress(),
      Uri.parse('https://chat.example.com'),
    );
    await services.auth.dispose();
  });

  test('a failing step does not stop the others and is reported', () async {
    final backend = MemorySecureBackend();
    final keys = SoftwareKeyStore();
    final secrets = InstanceSecrets(backend, instanceId);
    await secrets.write('device_id', 'd');
    await keys.ensure(instanceId);
    final folder = await Directory('${root.path}/instances/$instanceId')
        .create(recursive: true);
    final wiper = AccountWiper(
      instanceId: instanceId,
      folder: folder,
      stop: () async => throw StateError('engine'),
      wipeDatabase: () async => throw const FileSystemException('db'),
      keys: keys,
      secrets: secrets,
    );
    await expectLater(wiper.wipe(), throwsStateError);
    expect(folder.existsSync(), isFalse);
    expect(keys.scopes, isEmpty);
    expect(backend.values, isEmpty);
  });

  test('a second wipe waits for the first and deletes nothing twice', () async {
    final keys = SoftwareKeyStore();
    final folder = await Directory('${root.path}/x').create();
    final wiper = AccountWiper(
      instanceId: instanceId,
      folder: folder,
      stop: () async {},
      wipeDatabase: () async {},
      keys: keys,
      secrets: InstanceSecrets(MemorySecureBackend(), instanceId),
    );
    await Future.wait([wiper.wipe(), wiper.wipe()]);
    expect(keys.deletes, 1);
  });

  test('every revoke reason has its cause; an unknown one counts as a revoked session', () {
    expect(
      SignOutCause.ofRevokeReason('revoked_by_self'),
      SignOutCause.revokedBySelf,
    );
    expect(
      SignOutCause.ofRevokeReason('revoked_by_owner'),
      SignOutCause.revokedByOwner,
    );
    expect(
      SignOutCause.ofRevokeReason('user_disabled'),
      SignOutCause.userDisabled,
    );
    expect(SignOutCause.ofRevokeReason('inactive'), SignOutCause.inactive);
    expect(
      SignOutCause.ofRevokeReason('verification_failed'),
      SignOutCause.rejected,
    );
    expect(
      SignOutCause.ofRevokeReason('pending_expired'),
      SignOutCause.pendingExpired,
    );
    expect(
      SignOutCause.ofRevokeReason('something_new'),
      SignOutCause.sessionRevoked,
    );
    expect(SignOutCause.ofRevokeReason(null), SignOutCause.sessionRevoked);
  });

  test('the remembered instance survives its JSON; garbage gives null', () {
    final info = InstanceInfo(
      address: Uri.parse('https://chat.example.com'),
      instanceId: instanceId,
      productName: 'Team Chat',
      accentColor: 'loch',
      providers: const [LoginProvider(id: 'p', kind: 'oidc', name: 'SSO')],
      localEnabled: true,
      passwordEnabled: true,
    );
    final back = InstanceInfo.fromJson(info.toJson())!;
    expect(back.toJson(), info.toJson());
    expect(InstanceInfo.fromJson('x'), isNull);
    expect(InstanceInfo.fromJson({'address': 'https://a.example.com'}), isNull);
  });
}
