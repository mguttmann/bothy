// SPDX-License-Identifier: Apache-2.0

// One refresh per family at a time, persist before publish, revocation and network errors
// (08 7.4, 15 8, R-CL-07; brief M4-09 Design note 4).

import 'dart:async';

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

  void deviceRoute() {
    h.server.routes['GET /devices/${h.deviceId}'] = (_) =>
        FakeReply(200, deviceBody(h.deviceId, 'active'));
  }

  void refreshRoute({String access = 'A2', String refresh = 'R2'}) {
    h.server.routes['POST /auth/token/refresh'] = (_) async {
      // Hold the answer a little, so every caller is waiting while it runs.
      await Future<void>.delayed(const Duration(milliseconds: 5));
      return FakeReply(
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
    };
  }

  test(
    'ten parallel requests with an expired token cause one refresh',
    () async {
      deviceRoute();
      refreshRoute();
      h.clock.advance(const Duration(minutes: 6));
      final results = await Future.wait([
        for (var i = 0; i < 10; i++)
          h.session.api.getDevicesApi().getDevice(deviceId: h.deviceId),
      ]);
      expect(results, hasLength(10));
      expect(h.server.to('/auth/token/refresh'), hasLength(1));
      final refresh = h.server.to('/auth/token/refresh').single;
      expect(refresh.body, {'refresh_token': 'R1'});
      expect(refresh.headers.containsKey('authorization'), isFalse);
      expect(refresh.proof!.payload.containsKey('ath'), isFalse);
      expect(refresh.proof!.header['jwk'], h.dpopKey.publicJwk);
      final sent = h.server.to('/devices/${h.deviceId}');
      expect(sent, hasLength(10));
      for (final r in sent) {
        expect(
          r.headers['authorization'],
          contains(
            accessToken(
              'A2',
              h.clock.now().add(const Duration(minutes: 5)),
            ).split('.')[1],
          ),
        );
      }
    },
  );

  test(
    'the new refresh token is written before the first repeated request',
    () async {
      deviceRoute();
      refreshRoute();
      h.store.writeDelay = const Duration(milliseconds: 10);
      h.clock.advance(const Duration(minutes: 6));
      await Future.wait([
        for (var i = 0; i < 3; i++)
          h.session.api.getDevicesApi().getDevice(deviceId: h.deviceId),
      ]);
      final write = h.log.indexOf('write refresh_token');
      final firstUse = h.log.indexOf('send /devices/${h.deviceId}');
      expect(write, greaterThanOrEqualTo(0));
      expect(write, lessThan(firstUse));
      expect(h.log.indexOf('send /auth/token/refresh'), lessThan(write));
      expect(h.store.values.values, contains('R2'));
    },
  );

  test(
    '401 token_expired refreshes once and repeats the request once',
    () async {
      refreshRoute();
      var calls = 0;
      h.server.routes['GET /devices/${h.deviceId}'] = (_) {
        calls++;
        return calls == 1
            ? FakeReply.problem(401, 'token_expired')
            : FakeReply(200, deviceBody(h.deviceId, 'active'));
      };
      await h.session.api.getDevicesApi().getDevice(deviceId: h.deviceId);
      expect(h.server.to('/auth/token/refresh'), hasLength(1));
      expect(h.server.to('/devices/${h.deviceId}'), hasLength(2));
    },
  );

  test('a second 401 unauthenticated after the refresh is returned', () async {
    refreshRoute();
    h.server.routes['GET /devices/${h.deviceId}'] = (_) =>
        FakeReply.problem(401, 'unauthenticated');
    await expectLater(
      h.session.api.getDevicesApi().getDevice(deviceId: h.deviceId),
      throwsA(isA<DioException>()),
    );
    expect(h.server.to('/auth/token/refresh'), hasLength(1));
    expect(h.server.to('/devices/${h.deviceId}'), hasLength(2));
  });

  test(
    'refresh answered with 401 session_revoked signs out and clears the store',
    () async {
      deviceRoute();
      h.server.routes['POST /auth/token/refresh'] = (_) =>
          FakeReply.problem(401, 'session_revoked');
      h.clock.advance(const Duration(minutes: 6));
      await expectLater(
        h.session.api.getDevicesApi().getDevice(deviceId: h.deviceId),
        throwsA(
          isA<DioException>().having(
            (e) => e.error,
            'error',
            isA<SessionEnded>().having(
              (e) => e.reason,
              'reason',
              SignOutReason.sessionRevoked,
            ),
          ),
        ),
      );
      await h.settle();
      expect(
        h.events.whereType<SignedOut>().single.reason,
        SignOutReason.sessionRevoked,
      );
      expect(h.store.values, isEmpty);
      expect(h.session.session, isNull);
      expect(h.server.to('/devices/${h.deviceId}'), isEmpty);
    },
  );

  test(
    'a network error keeps the old token and reports refreshFailed',
    () async {
      h.server.routes['POST /auth/token/refresh'] = (_) => throw NetworkDown();
      final before = Map.of(h.store.values);
      await expectLater(
        h.session.refresh.refreshNow(),
        throwsA(isA<RefreshUnavailable>()),
      );
      await h.settle();
      expect(h.store.values, before);
      expect(h.session.session, isNotNull);
      expect(h.events.whereType<RefreshFailed>(), hasLength(1));
      expect(h.events.whereType<SignedOut>(), isEmpty);
    },
  );

  test('ensureFresh returns the current token while it is valid', () async {
    final token = await h.session.refresh.ensureFresh();
    expect(token, contains('.'));
    expect(h.server.requests, isEmpty);
  });

  test('any 401 session_revoked of a request signs out (R-SY-14)', () async {
    h.server.routes['GET /devices/${h.deviceId}'] = (_) =>
        FakeReply.problem(401, 'session_revoked');
    await expectLater(
      h.session.api.getDevicesApi().getDevice(deviceId: h.deviceId),
      throwsA(isA<DioException>()),
    );
    await h.settle();
    expect(
      h.events.whereType<SignedOut>().single.reason,
      SignOutReason.sessionRevoked,
    );
    expect(h.store.values, isEmpty);
  });

  test('the gateway close 4012 signs out through sessionRevoked', () async {
    await h.session.sessionRevoked();
    await h.settle();
    expect(
      h.events.whereType<SignedOut>().single.reason,
      SignOutReason.sessionRevoked,
    );
    expect(h.store.values, isEmpty);
    await expectLater(
      h.session.refresh.ensureFresh(),
      throwsA(isA<NotSignedIn>()),
    );
  });

  test(
    'after a restart the stored refresh token restores the session',
    () async {
      final again = Harness();
      again.store.values.addAll(h.store.values);
      again.server.routes['POST /auth/token/refresh'] = (_) => FakeReply(
        200,
        tokenBody(
          access: accessToken(
            'A9',
            again.clock.now().add(const Duration(minutes: 5)),
          ),
          refresh: 'R9',
          deviceId: h.deviceId,
        ),
      );
      again.server.routes['GET /devices/${h.deviceId}'] = (_) =>
          FakeReply(200, deviceBody(h.deviceId, 'active'));
      await again.session.api.getDevicesApi().getDevice(deviceId: h.deviceId);
      expect(again.server.to('/auth/token/refresh').single.body, {
        'refresh_token': 'R1',
      });
      expect(again.session.session!.sid, sessionId);
    },
  );

  test(
    'a late session_revoked of an older session keeps the new one',
    () async {
      final h = Harness();
      await h.signIn(access: 'A1', refresh: 'R1');
      final gate = Completer<void>();
      h.server.routes['GET /devices/${h.deviceId}'] = (_) async {
        await gate.future;
        return FakeReply.problem(401, 'session_revoked');
      };
      h.server.routes['POST /auth/logout'] = (_) => const FakeReply(204);
      final pending = h.session.api.getDevicesApi().getDevice(
        deviceId: h.deviceId,
      );
      final caught = expectLater(pending, throwsA(isA<DioException>()));
      await h.settle();
      await h.session.login.logout();
      await h.signIn(access: 'B1', refresh: 'RB');
      gate.complete();
      await caught;
      await h.settle();
      expect(h.events.whereType<SignedOut>(), isEmpty);
      expect(h.session.session, isNotNull);
      expect(h.store.values.values, contains('RB'));
    },
  );

  for (final relogin in [true, false]) {
    test('a late refresh rejection of an older session ends nothing '
        '(${relogin ? 'after a new login' : 'after a logout'})', () async {
      final h = Harness();
      await h.signIn(access: 'A1', refresh: 'R1');
      final gate = Completer<void>();
      h.server.routes['POST /auth/token/refresh'] = (_) async {
        await gate.future;
        return FakeReply.problem(401, 'session_revoked');
      };
      h.server.routes['POST /auth/logout'] = (_) => const FakeReply(204);
      final late = expectLater(
        h.session.refresh.refreshNow(),
        throwsA(isA<SessionEnded>()),
      );
      await h.settle();
      await h.session.login.logout();
      if (relogin) await h.signIn(access: 'B1', refresh: 'RB');
      await h.settle();
      h.events.clear();
      gate.complete();
      await late;
      await h.settle();
      expect(h.events, isEmpty);
      expect(h.store.values.keys.any((k) => k.endsWith('device_id')), isTrue);
      if (relogin) {
        expect(h.session.session, isNotNull);
        expect(h.store.values.values, contains('RB'));
      }
    });
  }
}
