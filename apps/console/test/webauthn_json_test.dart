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
}
