// SPDX-License-Identifier: Apache-2.0

// The fake host of `package:device_key/testing.dart`: a real `DeviceKeyClient` reaches it
// through the generated channel code and its codec, and it loses keys the way the platforms do
// (docs/spec/08-identity.md 5.4).

import 'dart:typed_data';

import 'package:device_key/device_key.dart';
import 'package:device_key/testing.dart';
import 'package:flutter_test/flutter_test.dart';

/// A key without a curve: the generation is its point and its signature.
class _Key implements FakeHostKey {
  _Key(this.generation);

  final int generation;

  @override
  Uint8List get x => Uint8List(32)..[31] = generation + 1;

  @override
  Uint8List get y => Uint8List(32)..[0] = generation + 1;

  @override
  Future<Uint8List> sign(Uint8List data) async =>
      Uint8List(64)..[0] = generation + 1;
}

Matcher _fails(DeviceKeyError code) =>
    throwsA(isA<DeviceKeyException>().having((e) => e.code, 'code', code));

void main() {
  const scope = 'https://chat.example.com';
  late FakeDeviceKeyHost host;
  late DeviceKeyClient client;

  setUp(() {
    host = FakeDeviceKeyHost(newKey: (_, _, generation) => _Key(generation));
    client = host.client;
  });

  test('ensure creates a key once and keyInfo never creates one', () async {
    expect(await client.keyInfo(scope, KeyPurpose.dpop), isNull);
    expect(host.has(scope, KeyPurpose.dpop), isFalse);
    final created = await client.ensure(scope, KeyPurpose.dpop);
    expect(await client.ensure(scope, KeyPurpose.dpop), created);
    expect(await client.keyInfo(scope, KeyPurpose.dpop), created);
    expect(created.requiresUserPresence, isFalse);
    final stepUp = await client.ensure(scope, KeyPurpose.stepUp);
    expect(stepUp.requiresUserPresence, isTrue);
    expect(stepUp.thumbprint, isNot(created.thumbprint));
    expect(host.calls, [
      'keyInfo dpop',
      'ensureKey dpop',
      'ensureKey dpop',
      'keyInfo dpop',
      'ensureKey stepUp',
    ]);
  });

  test('sign, attestation and delete pass through the channel', () async {
    await expectLater(
      client.sign(scope, KeyPurpose.dpop, Uint8List(3)),
      _fails(DeviceKeyError.notAvailable),
    );
    await expectLater(
      client.attestation(scope, KeyPurpose.dpop, Uint8List(16)),
      _fails(DeviceKeyError.notAvailable),
    );
    await client.ensure(scope, KeyPurpose.dpop);
    final signature = await client.sign(scope, KeyPurpose.dpop, Uint8List(3));
    expect(signature, hasLength(64));
    expect(
      await client.attestation(scope, KeyPurpose.dpop, Uint8List(16)),
      isNull,
    );
    await client.delete(scope);
    expect(host.has(scope, KeyPurpose.dpop), isFalse);
    expect(host.calls.last, 'deleteKeys');
  });

  test('a removed key is gone and a replaced key is another key', () async {
    final first = await client.ensure(scope, KeyPurpose.stepUp);
    host.remove(scope, KeyPurpose.stepUp);
    expect(await client.keyInfo(scope, KeyPurpose.stepUp), isNull);
    final second = await client.ensure(scope, KeyPurpose.stepUp);
    expect(second.thumbprint, isNot(first.thumbprint));
    host.replace(scope, KeyPurpose.stepUp);
    final third = await client.keyInfo(scope, KeyPurpose.stepUp);
    expect(third!.thumbprint, isNot(second.thumbprint));
  });

  test('a marker without its key answers key_invalidated until the keys are '
      'deleted', () async {
    await client.ensure(scope, KeyPurpose.dpop);
    host.loseKeepingMarker(scope, KeyPurpose.dpop);
    await expectLater(
      client.ensure(scope, KeyPurpose.dpop),
      _fails(DeviceKeyError.keyInvalidated),
    );
    await expectLater(
      client.keyInfo(scope, KeyPurpose.dpop),
      _fails(DeviceKeyError.keyInvalidated),
    );
    await expectLater(
      client.sign(scope, KeyPurpose.dpop, Uint8List(1)),
      _fails(DeviceKeyError.keyInvalidated),
    );
    await client.delete(scope);
    expect(await client.keyInfo(scope, KeyPurpose.dpop), isNull);
    await client.ensure(scope, KeyPurpose.dpop);
  });

  test('an invalidated key stays but no longer signs', () async {
    final info = await client.ensure(scope, KeyPurpose.stepUp);
    host.invalidate(scope, KeyPurpose.stepUp);
    expect(await client.keyInfo(scope, KeyPurpose.stepUp), info);
    await expectLater(
      client.sign(scope, KeyPurpose.stepUp, Uint8List(1)),
      _fails(DeviceKeyError.keyInvalidated),
    );
  });

  test('a storage that cannot answer fails every call for the key', () async {
    await client.ensure(scope, KeyPurpose.stepUp);
    host.failWith(scope, KeyPurpose.stepUp, DeviceKeyError.notAvailable);
    await expectLater(
      client.keyInfo(scope, KeyPurpose.stepUp),
      _fails(DeviceKeyError.notAvailable),
    );
    host.failWith(scope, KeyPurpose.stepUp, null);
    expect(await client.keyInfo(scope, KeyPurpose.stepUp), isNotNull);
  });

  test('a storage that refuses the delete keeps its keys', () async {
    await client.ensure(scope, KeyPurpose.dpop);
    host.deleteError = DeviceKeyError.internal;
    await expectLater(client.delete(scope), _fails(DeviceKeyError.internal));
    expect(host.has(scope, KeyPurpose.dpop), isTrue);
    host.deleteError = null;
    await client.delete(scope);
    expect(host.has(scope, KeyPurpose.dpop), isFalse);
  });

  test('a platform without a step-up key refuses to create one', () async {
    final windows = FakeDeviceKeyHost(
      newKey: (_, _, generation) => _Key(generation),
      stepUpKey: false,
    );
    await expectLater(
      windows.client.ensure(scope, KeyPurpose.stepUp),
      _fails(DeviceKeyError.notAvailable),
    );
    expect(await windows.client.keyInfo(scope, KeyPurpose.stepUp), isNull);
    await windows.client.ensure(scope, KeyPurpose.dpop);
  });
}
