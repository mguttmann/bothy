// SPDX-License-Identifier: Apache-2.0

// Nonce adoption and the single retry on use_dpop_nonce (RFC 9449 8, brief M4-09 Design note 3).

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';
import 'package:test/test.dart';

import 'support/fakes.dart';

void main() {
  late Harness h;

  setUp(() async {
    h = Harness();
    await h.signIn();
  });

  Future<Response<Device>> getDevice() =>
      h.session.api.getDevicesApi().getDevice(deviceId: h.deviceId);

  test(
    'every request carries Authorization DPoP and a proof bound to the token',
    () async {
      h.server.routes['GET /devices/${h.deviceId}'] = (_) =>
          FakeReply(200, deviceBody(h.deviceId, 'active'));
      await getDevice();
      final r = h.server.requests.single;
      expect(r.headers['authorization'], startsWith('DPoP '));
      final token = r.headers['authorization']!.substring(5);
      expect(r.proof!.payload['ath'], accessTokenHash(token));
      expect(
        r.proof!.payload['htu'],
        'https://chat.example.com/api/v1/devices/${h.deviceId}',
      );
      expect(r.proof!.payload['htm'], 'GET');
    },
  );

  test(
    'the newest DPoP-Nonce of any response is used in the next proof',
    () async {
      var n = 0;
      h.server.routes['GET /devices/${h.deviceId}'] = (_) => FakeReply(
        200,
        deviceBody(h.deviceId, 'active'),
        {'DPoP-Nonce': 'nonce-${++n}'},
      );
      await getDevice();
      await getDevice();
      await getDevice();
      expect(h.server.requests[0].proof!.payload.containsKey('nonce'), isFalse);
      expect(h.server.requests[1].proof!.payload['nonce'], 'nonce-1');
      expect(h.server.requests[2].proof!.payload['nonce'], 'nonce-2');
      expect(h.session.nonces.current, 'nonce-3');
    },
  );

  test(
    '401 use_dpop_nonce repeats once with the new nonce and a fresh proof',
    () async {
      var calls = 0;
      h.server.routes['GET /devices/${h.deviceId}'] = (_) {
        calls++;
        if (calls == 1) {
          return FakeReply.problem(
            401,
            'use_dpop_nonce',
            headers: {
              'DPoP-Nonce': 'fresh',
              'WWW-Authenticate': 'DPoP error="use_dpop_nonce"',
            },
          );
        }
        return FakeReply(200, deviceBody(h.deviceId, 'active'));
      };
      final res = await getDevice();
      expect(res.data!.state, DeviceStateEnum.active);
      expect(h.server.requests, hasLength(2));
      final first = h.server.requests[0].proof!.payload;
      final second = h.server.requests[1].proof!.payload;
      expect(second['nonce'], 'fresh');
      expect(second['jti'], isNot(first['jti']));
    },
  );

  test('a second use_dpop_nonce is returned to the caller', () async {
    h.server.routes['GET /devices/${h.deviceId}'] = (_) => FakeReply.problem(
      401,
      'use_dpop_nonce',
      headers: {'DPoP-Nonce': 'again'},
    );
    await expectLater(
      getDevice(),
      throwsA(
        isA<DioException>().having(
          (e) => ApiProblem.of(e)?.code,
          'code',
          'use_dpop_nonce',
        ),
      ),
    );
    expect(h.server.requests, hasLength(2));
  });

  test('a login call without a token gets the nonce retry as well', () async {
    var calls = 0;
    h.server.routes['POST /auth/pairing/begin'] = (_) {
      calls++;
      if (calls == 1) {
        return FakeReply.problem(
          401,
          'use_dpop_nonce',
          headers: {'DPoP-Nonce': 'p1'},
        );
      }
      return const FakeReply(201, {
        'pairing_id': '0192f4a0-7b1c-7000-8000-0000000000a1',
        'pairing_code': 'K7M2Q9XA',
        'expires_at': '2026-10-15T10:10:00.000Z',
      });
    };
    final started = await h.session.login.beginPairing();
    expect(started.pairingId, '0192f4a0-7b1c-7000-8000-0000000000a1');
    final second = h.server.requests[1];
    expect(second.headers.containsKey('authorization'), isFalse);
    expect(second.proof!.payload['nonce'], 'p1');
    expect(second.proof!.payload.containsKey('ath'), isFalse);
  });
}
