// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';
import 'package:device_key/device_key.dart';
import 'package:device_key/src/generated/device_key_api.g.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

// The EC key of RFC 7517, appendix A.1.
const _x = 'MKBCTNIcKUSDii11ySs3526iDZ8AiTo7Tu6KPAqv7D4';
const _y = '4Etl6SRW2YiLUrN5vfvVHuhp7x8PxltmWWlbbM4IFyM';

Uint8List _bytes(String text) => base64Url.decode(base64Url.normalize(text));

/// A host API that records the calls and answers from fields.
class _FakeHostApi extends DeviceKeyHostApi {
  DeviceKeyInfoMessage? info;
  Uint8List signature = Uint8List(0);
  Uint8List? attestationBytes;
  Object? error;
  PromptTextsMessage? lastPrompt;
  final calls = <String>[];

  void _maybeThrow() {
    final e = error;
    if (e != null) throw e;
  }

  @override
  Future<DeviceKeyInfoMessage> ensureKey(
    String scope,
    KeyPurposeMessage purpose,
  ) async {
    calls.add('ensureKey $scope ${purpose.name}');
    _maybeThrow();
    return info!;
  }

  @override
  Future<DeviceKeyInfoMessage?> keyInfo(
    String scope,
    KeyPurposeMessage purpose,
  ) async {
    calls.add('keyInfo $scope ${purpose.name}');
    _maybeThrow();
    return info;
  }

  @override
  Future<Uint8List> sign(
    String scope,
    KeyPurposeMessage purpose,
    Uint8List data,
    PromptTextsMessage? prompt,
  ) async {
    calls.add('sign $scope ${purpose.name} ${data.length}');
    lastPrompt = prompt;
    _maybeThrow();
    return signature;
  }

  @override
  Future<Uint8List?> attestation(
    String scope,
    KeyPurposeMessage purpose,
    Uint8List challenge,
  ) async {
    calls.add('attestation $scope ${purpose.name}');
    _maybeThrow();
    return attestationBytes;
  }

  @override
  Future<void> deleteKeys(String scope) async {
    calls.add('deleteKeys $scope');
    _maybeThrow();
  }
}

DeviceKeyInfoMessage _message({bool hardware = true, bool presence = false}) =>
    DeviceKeyInfoMessage(
      x: _bytes(_x),
      y: _bytes(_y),
      hardwareBound: hardware,
      requiresUserPresence: presence,
    );

void main() {
  const scope = 'https://chat.example.com';

  group('DeviceKeyInfo', () {
    test(
      'publicJwk has the members of RFC 7638 in base64url without padding',
      () {
        final info = DeviceKeyInfo(
          x: _bytes(_x),
          y: _bytes(_y),
          hardwareBound: true,
          requiresUserPresence: false,
        );
        expect(info.publicJwk, {'crv': 'P-256', 'kty': 'EC', 'x': _x, 'y': _y});
        expect(info.publicJwk.keys.toList(), ['crv', 'kty', 'x', 'y']);
        expect(info.publicJwk['x'], isNot(contains('=')));
      },
    );

    test('thumbprint follows RFC 7638 for the EC key of RFC 7517 A.1', () {
      final info = DeviceKeyInfo(
        x: _bytes(_x),
        y: _bytes(_y),
        hardwareBound: false,
        requiresUserPresence: false,
      );
      // Computed here from the canonical JSON, independently of the library.
      const canonical = '{"crv":"P-256","kty":"EC","x":"$_x","y":"$_y"}';
      final expected = base64Url
          .encode(sha256.convert(utf8.encode(canonical)).bytes)
          .replaceAll('=', '');
      expect(info.thumbprint, expected);
      // The same value computed with a second tool (Python hashlib).
      expect(info.thumbprint, 'cn-I_WNMClehiVp51i_0VpOENW1upEerA8sEam5hn-s');
    });

    test('equal keys are equal and toString carries no key material', () {
      final a = DeviceKeyInfo(
        x: _bytes(_x),
        y: _bytes(_y),
        hardwareBound: true,
        requiresUserPresence: true,
      );
      final b = DeviceKeyInfo(
        x: _bytes(_x),
        y: _bytes(_y),
        hardwareBound: true,
        requiresUserPresence: true,
      );
      expect(a, b);
      expect(a.hashCode, b.hashCode);
      expect(a.toString(), isNot(contains(_x)));
      expect(a.toString(), contains(a.thumbprint));
    });
  });

  group('DeviceKeyClient', () {
    test(
      'ensure passes scope and purpose and returns the native key',
      () async {
        final api = _FakeHostApi()..info = _message(presence: true);
        final client = DeviceKeyClient(api: api);
        final info = await client.ensure(scope, KeyPurpose.stepUp);
        expect(api.calls, ['ensureKey $scope stepUp']);
        expect(info.x, _bytes(_x));
        expect(info.y, _bytes(_y));
        expect(info.hardwareBound, isTrue);
        expect(info.requiresUserPresence, isTrue);
      },
    );

    test('keyInfo returns null when there is no key', () async {
      final api = _FakeHostApi();
      final client = DeviceKeyClient(api: api);
      expect(await client.keyInfo(scope, KeyPurpose.dpop), isNull);
      api.info = _message(hardware: false);
      final info = await client.keyInfo(scope, KeyPurpose.dpop);
      expect(info!.hardwareBound, isFalse);
      expect(api.calls, ['keyInfo $scope dpop', 'keyInfo $scope dpop']);
    });

    test('sign returns the native bytes unchanged', () async {
      final raw = Uint8List.fromList(List<int>.generate(64, (i) => i));
      final api = _FakeHostApi()..signature = raw;
      final client = DeviceKeyClient(api: api);
      final data = Uint8List.fromList(utf8.encode('header.payload'));
      expect(await client.sign(scope, KeyPurpose.dpop, data), same(raw));
      expect(api.calls, ['sign $scope dpop 14']);
    });

    test('the step-up prompt texts pass through to the native side', () async {
      final api = _FakeHostApi()..signature = Uint8List(64);
      final client = DeviceKeyClient(api: api);
      await client.sign(
        scope,
        KeyPurpose.stepUp,
        Uint8List(1),
        prompt: const StepUpPrompt(
          title: 'Confirm as Alex',
          subtitle: 'chat.example.com',
          negativeButton: 'Cancel',
        ),
      );
      expect(api.lastPrompt!.title, 'Confirm as Alex');
      expect(api.lastPrompt!.subtitle, 'chat.example.com');
      expect(api.lastPrompt!.negativeButton, 'Cancel');
      await client.sign(
        scope,
        KeyPurpose.stepUp,
        Uint8List(1),
        prompt: const StepUpPrompt(title: 'Confirm', negativeButton: 'Cancel'),
      );
      expect(api.lastPrompt!.subtitle, isNull);
      await client.sign(scope, KeyPurpose.dpop, Uint8List(1));
      expect(api.lastPrompt, isNull);
    });

    test('a signature that is not 64 bytes is an internal error', () async {
      final api = _FakeHostApi()..signature = Uint8List(70);
      final client = DeviceKeyClient(api: api);
      await expectLater(
        client.sign(scope, KeyPurpose.dpop, Uint8List(1)),
        throwsA(
          isA<DeviceKeyException>().having(
            (e) => e.code,
            'code',
            DeviceKeyError.internal,
          ),
        ),
      );
    });

    test(
      'a key with coordinates that are not 32 bytes is an internal error',
      () async {
        final api = _FakeHostApi()
          ..info = DeviceKeyInfoMessage(
            x: Uint8List(31),
            y: Uint8List(32),
            hardwareBound: true,
            requiresUserPresence: false,
          );
        final client = DeviceKeyClient(api: api);
        await expectLater(
          client.ensure(scope, KeyPurpose.dpop),
          throwsA(isA<DeviceKeyException>()),
        );
      },
    );

    test('attestation and delete reach the native side', () async {
      final api = _FakeHostApi()..attestationBytes = Uint8List.fromList([1]);
      final client = DeviceKeyClient(api: api);
      expect(await client.attestation(scope, KeyPurpose.dpop, Uint8List(16)), [
        1,
      ]);
      await client.delete(scope);
      expect(api.calls, ['attestation $scope dpop', 'deleteKeys $scope']);
    });

    final codes = {
      'user_cancelled': DeviceKeyError.userCancelled,
      'key_invalidated': DeviceKeyError.keyInvalidated,
      'not_available': DeviceKeyError.notAvailable,
      'internal': DeviceKeyError.internal,
      // Pigeon's code when no native handler answers (a platform without the plugin).
      'channel-error': DeviceKeyError.notAvailable,
      'something_else': DeviceKeyError.internal,
    };
    for (final entry in codes.entries) {
      test('native code ${entry.key} maps to ${entry.value}', () async {
        final api = _FakeHostApi()
          ..error = PlatformException(
            code: entry.key,
            message: 'secret $_x',
            details: _y,
          );
        final client = DeviceKeyClient(api: api);
        Object? caught;
        try {
          await client.sign(scope, KeyPurpose.stepUp, Uint8List(1));
        } on Object catch (e) {
          caught = e;
        }
        expect(caught, isA<DeviceKeyException>());
        final error = caught! as DeviceKeyException;
        expect(error.code, entry.value);
        // The platform message is dropped: no key material reaches logs.
        expect(error.toString(), isNot(contains(_x)));
        expect(error.toString(), isNot(contains(_y)));
      });
    }

    test('codes are the wire values of the native side', () {
      expect(DeviceKeyError.values.map((e) => e.wire), [
        'user_cancelled',
        'key_invalidated',
        'not_available',
        'internal',
      ]);
    });

    test('the Android core sends the same codes (DeviceKeyError.kt)', () {
      final kotlin = File(
        'android/devicekeycore/src/main/kotlin/devicekey/core/DeviceKeyError.kt',
      ).readAsStringSync();
      final wire = RegExp(
        r'^    [A-Z_]+\("([a-z_]+)", ',
        multiLine: true,
      ).allMatches(kotlin).map((m) => m.group(1));
      expect(wire, DeviceKeyError.values.map((e) => e.wire));
    });

    test('Windows sends the same codes as iOS and macOS', () {
      // The native sides spell the shared codes themselves; this keeps the Windows store
      // (M4-11) and the Swift core (M4-10) on the constants of DeviceKeyError.
      final windows = File('windows/core/device_key_store.cpp')
          .readAsStringSync();
      final darwin = File(
        'darwin/device_key/DeviceKeyCore/Sources/DeviceKeyCore/DeviceKeyError.swift',
      ).readAsStringSync();
      for (final code in DeviceKeyError.values) {
        expect(windows, contains('return "${code.wire}";'), reason: code.wire);
        expect(darwin, contains('return "${code.wire}"'), reason: code.wire);
      }
    });

    test('a platform without the plugin reports not_available', () async {
      final api = _FakeHostApi()..error = MissingPluginException('none');
      final client = DeviceKeyClient(api: api);
      await expectLater(
        client.delete(scope),
        throwsA(
          isA<DeviceKeyException>().having(
            (e) => e.code,
            'code',
            DeviceKeyError.notAvailable,
          ),
        ),
      );
    });

    test('the default constructor uses the generated host API', () {
      TestWidgetsFlutterBinding.ensureInitialized();
      expect(DeviceKeyClient(), isA<DeviceKeyClient>());
    });
  });
}
