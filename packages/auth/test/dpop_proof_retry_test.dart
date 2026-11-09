// SPDX-License-Identifier: Apache-2.0

// A stale or rejected DPoP proof never ends the session: `401 invalid_dpop_proof` is answered
// with one repeat under a fresh proof, and a refresh that still fails keeps the stored refresh
// token (06 problem `invalid_dpop_proof`, 15 8). A refresh request carries timeouts that end it
// well before its proof would be too old (08 7.3, `auth.dpop_proof_max_age`).

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';
import 'package:test/test.dart';

import 'support/fakes.dart';

void main() {
  late Harness h;

  setUp(() async {
    h = Harness();
    await h.signIn(access: 'A1', refresh: 'R1');
  });

  FakeReply staleProof() => FakeReply.problem(
    401,
    'invalid_dpop_proof',
    headers: {
      'DPoP-Nonce': 'current',
      'WWW-Authenticate': 'DPoP error="invalid_dpop_proof"',
    },
  );

  FakeReply tokens({String access = 'A2', String refresh = 'R2'}) => FakeReply(
    200,
    tokenBody(
      access: accessToken(
        access,
        h.clock.now().add(const Duration(minutes: 5)),
      ),
      refresh: refresh,
      deviceId: h.deviceId,
    ),
  );

  String? storedRefreshToken() => h.store.values.entries
      .where((e) => e.key.endsWith('refresh_token'))
      .map((e) => e.value)
      .singleOrNull;

  group('refresh answered with 401 invalid_dpop_proof', () {
    test('repeats once with a fresh proof and adopts the answer', () async {
      var calls = 0;
      h.server.routes['POST /auth/token/refresh'] = (_) {
        calls++;
        if (calls == 1) {
          // The proof was signed before the device slept; time passed until it arrived.
          h.clock.advance(const Duration(minutes: 8));
          return staleProof();
        }
        return tokens();
      };
      final token = await h.session.refresh.refreshNow();
      await h.settle();
      expect(token, contains('.'));
      final sent = h.server.to('/auth/token/refresh');
      expect(sent, hasLength(2));
      expect(sent[1].body, {'refresh_token': 'R1'});
      final first = sent[0].proof!.payload;
      final second = sent[1].proof!.payload;
      expect(second['jti'], isNot(first['jti']));
      expect(second['iat'] as int, greaterThan(first['iat'] as int));
      expect(second['nonce'], 'current');
      expect(storedRefreshToken(), 'R2');
      expect(h.events.whereType<SignedOut>(), isEmpty);
      expect(h.session.session, isNotNull);
    });

    test('a second rejection keeps the stored refresh token and reports '
        'RefreshUnavailable', () async {
      h.server.routes['POST /auth/token/refresh'] = (_) => staleProof();
      await expectLater(
        h.session.refresh.refreshNow(),
        throwsA(
          isA<RefreshUnavailable>().having((e) => e.status, 'status', 401),
        ),
      );
      await h.settle();
      expect(h.server.to('/auth/token/refresh'), hasLength(2));
      expect(storedRefreshToken(), 'R1');
      expect(h.session.session, isNotNull);
      expect(h.events.whereType<SignedOut>(), isEmpty);
      expect(h.events.whereType<RefreshFailed>().single.status, 401);
    });

    test('during a request with an expired token keeps the session', () async {
      h.server.routes['POST /auth/token/refresh'] = (_) => staleProof();
      h.clock.advance(const Duration(minutes: 6));
      await expectLater(
        h.session.api.getDevicesApi().getDevice(deviceId: h.deviceId),
        throwsA(
          isA<DioException>().having(
            (e) => e.error,
            'error',
            isA<RefreshUnavailable>(),
          ),
        ),
      );
      await h.settle();
      expect(storedRefreshToken(), 'R1');
      expect(h.events.whereType<SignedOut>(), isEmpty);
      // A later refresh with a good proof succeeds with the kept token.
      h.server.routes['POST /auth/token/refresh'] = (_) => tokens();
      await h.session.refresh.refreshNow();
      expect(h.server.to('/auth/token/refresh').last.body, {
        'refresh_token': 'R1',
      });
      expect(storedRefreshToken(), 'R2');
    });
  });

  group('refresh rejections that still end the session', () {
    for (final (code, reason) in [
      ('session_revoked', SignOutReason.sessionRevoked),
      ('unauthenticated', SignOutReason.refreshRejected),
      ('token_expired', SignOutReason.refreshRejected),
    ]) {
      test('401 $code signs out with ${reason.name}', () async {
        h.server.routes['POST /auth/token/refresh'] = (_) =>
            FakeReply.problem(401, code);
        await expectLater(
          h.session.refresh.refreshNow(),
          throwsA(
            isA<SessionEnded>().having((e) => e.reason, 'reason', reason),
          ),
        );
        await h.settle();
        expect(h.server.to('/auth/token/refresh'), hasLength(1));
        expect(h.events.whereType<SignedOut>().single.reason, reason);
        expect(storedRefreshToken(), isNull);
        expect(h.session.session, isNull);
      });
    }
  });

  group('a request answered with 401 invalid_dpop_proof', () {
    test('repeats once with a fresh proof', () async {
      var calls = 0;
      h.server.routes['GET /devices/${h.deviceId}'] = (_) {
        calls++;
        if (calls == 1) {
          h.clock.advance(const Duration(seconds: 2));
          return staleProof();
        }
        return FakeReply(200, deviceBody(h.deviceId, 'active'));
      };
      final res = await h.session.api.getDevicesApi().getDevice(
        deviceId: h.deviceId,
      );
      expect(res.data!.state, DeviceStateEnum.active);
      final sent = h.server.to('/devices/${h.deviceId}');
      expect(sent, hasLength(2));
      final first = sent[0].proof!.payload;
      final second = sent[1].proof!.payload;
      expect(second['jti'], isNot(first['jti']));
      expect(second['iat'] as int, greaterThan(first['iat'] as int));
      expect(second['nonce'], 'current');
      expect(second['ath'], first['ath']);
    });

    test('a second rejection goes to the caller and ends nothing', () async {
      h.server.routes['GET /devices/${h.deviceId}'] = (_) => staleProof();
      await expectLater(
        h.session.api.getDevicesApi().getDevice(deviceId: h.deviceId),
        throwsA(
          isA<DioException>().having(
            (e) => ApiProblem.of(e)?.code,
            'code',
            'invalid_dpop_proof',
          ),
        ),
      );
      await h.settle();
      expect(h.server.to('/devices/${h.deviceId}'), hasLength(2));
      expect(h.server.to('/auth/token/refresh'), isEmpty);
      expect(h.events.whereType<SignedOut>(), isEmpty);
      expect(storedRefreshToken(), 'R1');
    });

    test('a nonce retry and a proof retry each happen at most once', () async {
      var calls = 0;
      h.server.routes['GET /devices/${h.deviceId}'] = (_) {
        calls++;
        return switch (calls) {
          1 => FakeReply.problem(
            401,
            'use_dpop_nonce',
            headers: {'DPoP-Nonce': 'n1'},
          ),
          2 => staleProof(),
          _ => FakeReply(200, deviceBody(h.deviceId, 'active')),
        };
      };
      await h.session.api.getDevicesApi().getDevice(deviceId: h.deviceId);
      expect(h.server.to('/devices/${h.deviceId}'), hasLength(3));
    });
  });

  test('a refresh request carries timeouts shorter than the proof lifetime '
      '(auth.dpop_proof_max_age, 60 s)', () async {
    h.server.routes['POST /auth/token/refresh'] = (_) => tokens();
    await h.session.refresh.refreshNow();
    final sent = h.server.to('/auth/token/refresh').single;
    final connect = sent.connectTimeout;
    final send = sent.sendTimeout;
    final receive = sent.receiveTimeout;
    expect(connect, isNotNull);
    expect(send, isNotNull);
    expect(receive, isNotNull);
    expect(
      connect! + send! + receive!,
      lessThanOrEqualTo(const Duration(seconds: 60)),
    );
  });
}
