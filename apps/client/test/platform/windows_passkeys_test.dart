// SPDX-License-Identifier: Apache-2.0

// Passkeys of the Windows app against a mocked `bothy/webauthn` channel: the arguments the
// runner gets, the client data the adapter writes, the credential JSON the server parses, a
// cancelled Windows Hello prompt, and the factory without Windows Hello or without a runner.

import 'dart:convert';

import 'package:client/platform/windows_passkeys.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

String b64(List<int> bytes) => base64Url.encode(bytes).replaceAll('=', '');

Uint8List bytes(String b64url) =>
    base64Url.decode(b64url.padRight((b64url.length + 3) & ~3, '='));

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  const channel = MethodChannel('bothy/webauthn');
  final calls = <MethodCall>[];
  Object? Function(MethodCall call) answer = (_) => null;

  setUp(() {
    calls.clear();
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
          calls.add(call);
          return answer(call);
        });
  });

  tearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, null);
  });

  WindowsPasskeys passkeys() =>
      WindowsPasskeys(origin: 'https://chat.example.com:8443/some/path');

  final credentialId = Uint8List.fromList([1, 2, 3, 250]);
  final userId = Uint8List.fromList([9, 8, 7]);

  Map<String, Object?> creation({Map<String, Object?>? selection}) => {
    'rp': {'id': 'chat.example.com', 'name': 'Team Chat'},
    'user': {'id': b64(userId), 'name': 'ada', 'displayName': 'Ada L.'},
    'challenge': 'Y2hhbGxlbmdlLWNyZWF0ZQ',
    'pubKeyCredParams': [
      {'type': 'public-key', 'alg': -8},
      {'type': 'public-key', 'alg': -7},
      {'type': 'public-key', 'alg': -257},
    ],
    'timeout': 120000,
    'excludeCredentials': [
      {
        'type': 'public-key',
        'id': b64([5, 6]),
      },
      {
        'type': 'public-key',
        'id': b64([7, 255, 254]),
      },
    ],
    'authenticatorSelection':
        selection ??
        {'residentKey': 'required', 'userVerification': 'required'},
    'attestation': 'none',
  };

  test('makeCredential gets the creation options in Win32 form', () async {
    answer = (_) => {
      'credentialId': credentialId,
      'attestationObject': Uint8List.fromList([0xa3, 1]),
      'transports': ['internal'],
    };
    final json = await passkeys().createCredential(creation());

    expect(calls.single.method, 'makeCredential');
    final args = calls.single.arguments as Map;
    expect(args['rpId'], 'chat.example.com');
    expect(args['rpName'], 'Team Chat');
    expect(args['userId'], userId);
    expect(args['userName'], 'ada');
    expect(args['userDisplayName'], 'Ada L.');
    expect(args['algs'], [-8, -7, -257]);
    expect(args['excludeIds'], [
      [5, 6],
      [7, 255, 254],
    ]);
    expect(args['timeoutMs'], 120000);
    expect(args['residentKeyRequired'], isTrue);
    expect(args['userVerification'], 'required');
    expect(args['platformOnly'], isTrue);

    final clientData = args['clientData'] as Uint8List;
    expect(
      utf8.decode(clientData),
      '{"type":"webauthn.create","challenge":"Y2hhbGxlbmdlLWNyZWF0ZQ",'
      '"origin":"https://chat.example.com:8443","crossOrigin":false}',
    );

    expect(json, {
      'id': b64(credentialId),
      'rawId': b64(credentialId),
      'type': 'public-key',
      'authenticatorAttachment': 'platform',
      'clientExtensionResults': <String, Object?>{},
      'response': {
        'clientDataJSON': b64(clientData),
        'attestationObject': b64([0xa3, 1]),
        'transports': ['internal'],
      },
    });
    expect(bytes(json['id']! as String), credentialId);
  });

  test(
    'makeCredential defaults: timeout, user verification, algorithms',
    () async {
      answer = (_) => {
        'credentialId': credentialId,
        'attestationObject': Uint8List.fromList([1]),
      };
      final options = creation(selection: {'residentKey': 'preferred'})
        ..remove('timeout')
        ..remove('pubKeyCredParams');
      final json = await passkeys().createCredential(options);

      final args = calls.single.arguments as Map;
      expect(args['timeoutMs'], defaultPasskeyTimeoutMs);
      expect(args['userVerification'], 'preferred');
      expect(args['residentKeyRequired'], isFalse);
      expect(args['algs'], [-7, -257]);
      expect((json['response']! as Map).containsKey('transports'), isFalse);
    },
  );

  test('getAssertion gets the request options in Win32 form', () async {
    answer = (_) => {
      'credentialId': credentialId,
      'authenticatorData': Uint8List.fromList([10, 11]),
      'signature': Uint8List.fromList([12, 13]),
      'userHandle': userId,
    };
    final json = await passkeys().getAssertion({
      'challenge': 'c3RlcC11cA',
      'rpId': 'example.com',
      'allowCredentials': [
        {'type': 'public-key', 'id': b64(credentialId)},
      ],
      'userVerification': 'required',
    });

    expect(calls.single.method, 'getAssertion');
    final args = calls.single.arguments as Map;
    expect(args['rpId'], 'example.com', reason: 'a domain above the host');
    expect(args['allowIds'], [credentialId]);
    expect(args['timeoutMs'], defaultPasskeyTimeoutMs);
    expect(args['userVerification'], 'required');
    final clientData = args['clientData'] as Uint8List;
    expect(
      utf8.decode(clientData),
      '{"type":"webauthn.get","challenge":"c3RlcC11cA",'
      '"origin":"https://chat.example.com:8443","crossOrigin":false}',
    );

    expect(json, {
      'id': b64(credentialId),
      'rawId': b64(credentialId),
      'type': 'public-key',
      'authenticatorAttachment': 'platform',
      'clientExtensionResults': <String, Object?>{},
      'response': {
        'clientDataJSON': b64(clientData),
        'authenticatorData': b64([10, 11]),
        'signature': b64([12, 13]),
        'userHandle': b64(userId),
      },
    });
  });

  test(
    'getAssertion without rpId uses the host and omits a null userHandle',
    () async {
      answer = (_) => {
        'credentialId': credentialId,
        'authenticatorData': Uint8List.fromList([1]),
        'signature': Uint8List.fromList([2]),
        'userHandle': null,
      };
      final json = await passkeys().getAssertion({'challenge': 'YQ'});
      final args = calls.single.arguments as Map;
      expect(args['rpId'], 'chat.example.com');
      expect(args['allowIds'], isEmpty);
      expect(args['userVerification'], 'preferred');
      expect((json['response']! as Map).containsKey('userHandle'), isFalse);
    },
  );

  test(
    'a relying party outside the origin is refused before Windows',
    () async {
      await expectLater(
        passkeys().getAssertion({'challenge': 'YQ', 'rpId': 'evil.example'}),
        throwsStateError,
      );
      await expectLater(
        passkeys().getAssertion({
          'challenge': 'YQ',
          'rpId': 'xchat.example.com',
        }),
        throwsStateError,
      );
      expect(calls, isEmpty);
    },
  );

  test(
    'a cancelled Windows Hello prompt throws PlatformException cancelled',
    () async {
      answer = (_) => throw PlatformException(code: 'cancelled');
      await expectLater(
        passkeys().getAssertion({'challenge': 'YQ'}),
        throwsA(
          isA<PlatformException>().having((e) => e.code, 'code', 'cancelled'),
        ),
      );
      await expectLater(
        passkeys().createCredential(creation()),
        throwsA(
          isA<PlatformException>().having((e) => e.code, 'code', 'cancelled'),
        ),
      );
    },
  );

  test('create returns the adapter only when Windows Hello is there', () async {
    answer = (_) => true;
    final available = await WindowsPasskeys.create(
      origin: 'https://chat.example.com',
    );
    expect(available, isNotNull);
    expect(available!.origin, 'https://chat.example.com');
    expect(calls.single.method, 'available');

    answer = (_) => false;
    expect(
      await WindowsPasskeys.create(origin: 'https://chat.example.com'),
      isNull,
    );
  });

  test('create returns null without the runner channel', () async {
    expect(
      await WindowsPasskeys.create(
        origin: 'https://chat.example.com',
        channel: const MethodChannel('bothy/webauthn-missing'),
      ),
      isNull,
    );
  });
}
