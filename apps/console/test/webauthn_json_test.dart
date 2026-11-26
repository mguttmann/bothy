// SPDX-License-Identifier: Apache-2.0

import 'package:console/console.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('request options of the server become bytes', () {
    final r = PasskeyRequest.fromJson({
      'challenge': 'Zm9vYmFy',
      'rpId': 'chat.example.com',
      'timeout': 60000,
      'userVerification': 'preferred',
      'allowCredentials': [
        {
          'type': 'public-key',
          'id': 'Y3JlZA',
          'transports': ['internal'],
        },
        {'type': 'public-key'},
      ],
    });
    expect(String.fromCharCodes(r.challenge), 'foobar');
    expect(r.rpId, 'chat.example.com');
    expect(r.timeoutMs, 60000);
    expect(r.allowCredentials, hasLength(1));
    expect(String.fromCharCodes(r.allowCredentials.single.id), 'cred');
    expect(r.allowCredentials.single.transports, ['internal']);
  });

  test('options without a challenge are refused', () {
    expect(() => PasskeyRequest.fromJson({}), throwsFormatException);
  });

  test('an assertion becomes the JSON form of the server', () {
    final json = assertionJson(
      id: 'Y3JlZA',
      rawId: 'cred'.codeUnits,
      clientDataJson: '{}'.codeUnits,
      authenticatorData: 'auth'.codeUnits,
      signature: 'sig'.codeUnits,
      userHandle: 'u1'.codeUnits,
      authenticatorAttachment: 'platform',
    );
    expect(json, {
      'id': 'Y3JlZA',
      'rawId': 'Y3JlZA',
      'type': 'public-key',
      'authenticatorAttachment': 'platform',
      'response': {
        'clientDataJSON': 'e30',
        'authenticatorData': 'YXV0aA',
        'signature': 'c2ln',
        'userHandle': 'dTE',
      },
      'clientExtensionResults': <String, Object?>{},
    });
    expect(base64UrlBytes('e30'), '{}'.codeUnits);
  });

  test('creation options of the server in binary form (V1-14)', () {
    final o = PasskeyCreation.fromJson({
      'challenge': 'Zm9v',
      'rp': {'id': 'chat.example.com', 'name': 'Team Chat'},
      'user': {'id': 'dTE', 'name': 'alex', 'displayName': 'Alex'},
      'pubKeyCredParams': [
        {'type': 'public-key', 'alg': -7},
        {'type': 'public-key', 'alg': -257},
        {'type': 'public-key'},
      ],
      'excludeCredentials': [
        {
          'type': 'public-key',
          'id': 'Y3JlZA',
          'transports': ['internal'],
        },
      ],
      'authenticatorSelection': {
        'residentKey': 'preferred',
        'userVerification': 'discouraged',
      },
      'timeout': 300000,
    });
    expect(o.challenge, 'foo'.codeUnits);
    expect(o.rpId, 'chat.example.com');
    expect(o.rpName, 'Team Chat');
    expect(o.userId, 'u1'.codeUnits);
    expect(o.userName, 'alex');
    expect(o.userDisplayName, 'Alex');
    expect(o.parameters.map((p) => p.alg), [-7, -257]);
    expect(o.excludeCredentials.single.id, 'cred'.codeUnits);
    expect(o.excludeCredentials.single.transports, ['internal']);
    expect(o.residentKey, 'preferred');
    expect(o.timeoutMs, 300000);
    expect(
      () => PasskeyCreation.fromJson({'challenge': 'Zm9v'}),
      throwsFormatException,
    );
    expect(
      () => PasskeyCreation.fromJson({
        'user': {'id': 'dTE'},
      }),
      throwsFormatException,
    );
  });

  test('a new passkey in the JSON form of completePasskeyRegistration', () {
    expect(
      attestationJson(
        id: 'bmV3',
        rawId: 'new'.codeUnits,
        clientDataJson: '{}'.codeUnits,
        attestationObject: 'att'.codeUnits,
        transports: ['internal', 'hybrid'],
        authenticatorAttachment: 'platform',
      ),
      {
        'id': 'bmV3',
        'rawId': 'bmV3',
        'type': 'public-key',
        'authenticatorAttachment': 'platform',
        'response': {
          'clientDataJSON': 'e30',
          'attestationObject': 'YXR0',
          'transports': ['internal', 'hybrid'],
        },
        'clientExtensionResults': <String, Object?>{},
      },
    );
  });
}
