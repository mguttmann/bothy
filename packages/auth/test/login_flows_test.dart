// SPDX-License-Identifier: Apache-2.0

// The login flows against the fake API (08 2.2, 2.4, 2.5, 3.2, 5, 14.1; brief M4-09 Design
// note 5).

import 'dart:convert';

import 'package:auth/auth.dart';
import 'package:crypto/crypto.dart';
import 'package:test/test.dart';

import 'support/fakes.dart';

void main() {
  late Harness h;
  const attemptId = '0192f4a0-7b1c-7000-8000-0000000000f1';
  const providerId = '0192f4a0-7b1c-7000-8000-0000000000f2';
  const pairingId = '0192f4a0-7b1c-7000-8000-0000000000a1';

  setUp(() => h = Harness());

  FakeReply tokens({String state = 'active', String? code, String? deviceId}) =>
      FakeReply(
        201,
        tokenBody(
          access: accessToken(
            'A1',
            h.clock.now().add(const Duration(minutes: 5)),
          ),
          refresh: 'R1',
          deviceId: deviceId ?? h.deviceId,
          state: state,
          verificationCode: code,
        ),
      );

  Map<String, Object?> deviceOf(SeenRequest r) =>
      (r.body! as Map)['device']! as Map<String, Object?>;

  group('device fields', () {
    test(
      'device_id is a UUIDv7 kept per server; both public keys are sent',
      () async {
        final a = await h.session.login.registerDeviceFields();
        final b = await h.session.login.registerDeviceFields();
        expect(a.deviceId, b.deviceId);
        expect(a.deviceId, matches(RegExp(r'^[0-9a-f]{8}-[0-9a-f]{4}-7')));
        expect(h.store.values.values, contains(a.deviceId));
        expect(a.alg.name, 'ES256');
        expect(a.publicJwk.keys.toSet(), {'kty', 'crv', 'x', 'y'});
        expect(a.stepUpKey!.publicJwk['x']!.value, h.stepUpKey.publicJwk['x']);
        expect(a.platform.name, 'macos');
        expect(a.name, 'Alex laptop');
        expect(a.appVersion, '1.0.0');
      },
    );
  });

  group('OIDC with PKCE', () {
    test('client_challenge is S256 of the verifier; complete carries verifier and device', () async {
      h.server.routes['POST /auth/oidc/begin'] = (_) => const FakeReply(200, {
        'login_attempt_id': attemptId,
        'authorization_url': 'https://idp.example.com/authorize?client_id=c',
        'expires_at': '2026-10-15T10:10:00.000Z',
      });
      h.server.routes['POST /auth/oidc/complete'] = (_) => tokens();
      h.redirects.next = Uri.parse(
        'http://127.0.0.1:49152/callback?login_code=LC-canary&login_attempt_id=$attemptId',
      );
      final outcome = await h.session.login.beginOidc(providerId);
      expect(outcome, isA<LoginSignedIn>());

      final begin = h.server.to('/auth/oidc/begin').single.body! as Map;
      expect(begin['provider_id'], providerId);
      expect(begin['app_redirect_uri'], 'http://127.0.0.1:49152/callback');
      expect(
        h.server
            .to('/auth/oidc/begin')
            .single
            .headers
            .containsKey('authorization'),
        isFalse,
      );
      expect(
        h.browser.opened.single.toString(),
        'https://idp.example.com/authorize?client_id=c',
      );
      expect(h.redirects.expectedPrefix, h.redirects.uri);
      expect(h.redirects.timeout, const Duration(minutes: 10));

      final complete = h.server.to('/auth/oidc/complete').single;
      final body = complete.body! as Map;
      final verifier = body['client_verifier']! as String;
      expect(base64Url.decode(base64Url.normalize(verifier)), hasLength(32));
      final s256 = base64Url
          .encode(sha256.convert(ascii.encode(verifier)).bytes)
          .replaceAll('=', '');
      expect(begin['client_challenge'], s256);
      expect(body['login_code'], 'LC-canary');
      expect(deviceOf(complete)['device_id'], isNotEmpty);
      expect(deviceOf(complete)['public_jwk'], h.dpopKey.publicJwk);
      expect(complete.proof!.payload.containsKey('ath'), isFalse);
      expect(complete.proof!.header['jwk'], h.dpopKey.publicJwk);
      await h.settle();
      expect(h.events.whereType<SignedIn>(), hasLength(1));
      expect(h.store.values.values, contains('R1'));
    });

    test('an error redirect fails without completing', () async {
      h.server.routes['POST /auth/oidc/begin'] = (_) => const FakeReply(200, {
        'login_attempt_id': attemptId,
        'authorization_url': 'https://idp.example.com/authorize',
        'expires_at': '2026-10-15T10:10:00.000Z',
      });
      h.redirects.next = Uri.parse(
        'http://127.0.0.1:49152/callback?error=access_denied&login_attempt_id=$attemptId',
      );
      final outcome = await h.session.login.beginOidc(providerId);
      expect(
        outcome,
        isA<LoginFailed>().having((f) => f.reason, 'reason', 'access_denied'),
      );
      expect(h.server.to('/auth/oidc/complete'), isEmpty);
    });

    test('a redirect of another attempt is rejected', () async {
      h.server.routes['POST /auth/oidc/begin'] = (_) => const FakeReply(200, {
        'login_attempt_id': attemptId,
        'authorization_url': 'https://idp.example.com/authorize',
        'expires_at': '2026-10-15T10:10:00.000Z',
      });
      h.redirects.next = Uri.parse(
        'http://127.0.0.1:49152/callback?login_code=LC&login_attempt_id=0192f4a0-7b1c-7000-8000-000000000999',
      );
      final outcome = await h.session.login.beginOidc(providerId);
      expect(
        outcome,
        isA<LoginFailed>().having((f) => f.reason, 'reason', 'login_failed'),
      );
      expect(h.server.to('/auth/oidc/complete'), isEmpty);
    });

    test('no redirect before the flow expires is a timeout', () async {
      h.server.routes['POST /auth/oidc/begin'] = (_) => const FakeReply(200, {
        'login_attempt_id': attemptId,
        'authorization_url': 'https://idp.example.com/authorize',
        'expires_at': '2026-10-15T10:10:00.000Z',
      });
      final outcome = await h.session.login.beginOidc(providerId);
      expect(
        outcome,
        isA<LoginFailed>().having((f) => f.reason, 'reason', 'flow_expired'),
      );
    });

    test('a rejected begin returns the problem', () async {
      h.server.routes['POST /auth/oidc/begin'] = (_) =>
          FakeReply.problem(400, 'validation_failed');
      final outcome = await h.session.login.beginOidc(providerId);
      expect(
        outcome,
        isA<LoginFailed>().having(
          (f) => f.problem?.code,
          'code',
          'validation_failed',
        ),
      );
      expect(h.browser.opened, isEmpty);
    });
  });

  group('pending device', () {
    test(
      'a pending login emits devicePending, then polling until active signs in',
      () async {
        h.server.routes['POST /auth/password/login'] = (_) =>
            tokens(state: 'pending', code: '482913');
        final first = await h.session.login.loginWithPassword(
          loginName: 'alex',
          password: 'pw',
          totpCode: '123456',
        );
        expect(first, isA<LoginPending>());
        final pending = (first as LoginPending).device;
        expect(pending.verificationCode, '482913');
        expect(pending.deviceId.value, h.deviceId);
        await h.settle();
        expect(h.events.whereType<DevicePending>(), hasLength(1));

        var polls = 0;
        h.server.routes['GET /devices/${h.deviceId}'] = (_) {
          polls++;
          return FakeReply(
            200,
            deviceBody(h.deviceId, polls < 3 ? 'pending' : 'active'),
          );
        };
        final done = await h.session.login.pollPendingDevice(pending.deviceId);
        expect(done, isA<LoginSignedIn>());
        expect(polls, 3);
        // Backoff from client.outbox_retry_base (2 s) doubling up to client.outbox_retry_cap.
        expect(h.clock.waits, [
          const Duration(seconds: 2),
          const Duration(seconds: 4),
        ]);
        await h.settle();
        expect(h.events.whereType<SignedIn>(), hasLength(1));
        expect(
          h.server.to('/devices/${h.deviceId}').first.headers['authorization'],
          startsWith('DPoP '),
        );
      },
    );

    test(
      'polling until revoked emits signedOut and clears the store',
      () async {
        h.server.routes['POST /auth/password/login'] = (_) =>
            tokens(state: 'pending', code: '1');
        final first = await h.session.login.loginWithPassword(
          loginName: 'alex',
          password: 'pw',
          totpCode: '123456',
        ) as LoginPending;
        h.server.routes['GET /devices/${h.deviceId}'] = (_) => FakeReply(
          200,
          deviceBody(
            h.deviceId,
            'revoked',
            revokeReason: 'verification_failed',
          ),
        );
        final done = await h.session.login.pollPendingDevice(
          first.device.deviceId,
        );
        expect(
          done,
          isA<LoginFailed>().having(
            (f) => f.reason,
            'reason',
            'verification_failed',
          ),
        );
        await h.settle();
        expect(
          h.events.whereType<SignedOut>().single.reason,
          SignOutReason.deviceRevoked,
        );
        expect(h.store.values, isEmpty);
      },
    );

    test('a network error while polling keeps polling', () async {
      h.server.routes['POST /auth/password/login'] = (_) =>
          tokens(state: 'pending', code: '1');
      final first = await h.session.login.loginWithPassword(
        loginName: 'alex',
        password: 'pw',
        totpCode: '123456',
      ) as LoginPending;
      var polls = 0;
      h.server.routes['GET /devices/${h.deviceId}'] = (_) {
        polls++;
        if (polls == 1) throw NetworkDown();
        return FakeReply(200, deviceBody(h.deviceId, 'active'));
      };
      expect(
        await h.session.login.pollPendingDevice(first.device.deviceId),
        isA<LoginSignedIn>(),
      );
      expect(polls, 2);
    });
  });

  group('pending device with a transient refresh error', () {
    test('polling keeps going when a refresh fails on the network', () async {
      h.server.routes['POST /auth/password/login'] = (_) =>
          tokens(state: 'pending', code: '1');
      final first = await h.session.login.loginWithPassword(
        loginName: 'alex',
        password: 'pw',
        totpCode: '123456',
      ) as LoginPending;
      var refreshes = 0;
      h.server.routes['POST /auth/token/refresh'] = (_) {
        refreshes++;
        if (refreshes == 1) throw NetworkDown();
        return FakeReply(
          200,
          tokenBody(
            access: accessToken(
              'A2',
              h.clock.now().add(const Duration(minutes: 5)),
            ),
            refresh: 'R2',
            deviceId: h.deviceId,
          ),
        );
      };
      h.server.routes['GET /devices/${h.deviceId}'] = (_) => FakeReply(
        200,
        deviceBody(h.deviceId, refreshes >= 2 ? 'active' : 'pending'),
      );
      final done = await h.session.login.pollPendingDevice(
        first.device.deviceId,
      );
      expect(done, isA<LoginSignedIn>());
      expect(refreshes, 2);
    });
  });

  group('pairing', () {
    test(
      'beginPairing returns the code; pollPairing polls until approved',
      () async {
        h.server.routes['POST /auth/pairing/begin'] = (_) =>
            const FakeReply(201, {
              'pairing_id': pairingId,
              'pairing_code': 'K7M2Q9XA',
              'expires_at': '2026-10-15T10:10:00.000Z',
            });
        final started = await h.session.login.beginPairing();
        expect(started.code, 'K7M2Q9XA');
        expect(started.expiresAt, DateTime.utc(2026, 10, 15, 10, 10));
        final begin = h.server.to('/auth/pairing/begin').single;
        expect(deviceOf(begin)['public_jwk'], h.dpopKey.publicJwk);
        expect(begin.proof, isNotNull);

        var polls = 0;
        h.server.routes['POST /auth/pairing/complete'] = (_) {
          polls++;
          if (polls < 3) {
            return const FakeReply(202, {
              'pairing_id': pairingId,
              'status': 'requested',
              'expires_at': '2026-10-15T10:10:00.000Z',
            });
          }
          return tokens();
        };
        final done = await h.session.login.pollPairing(started.pairingId);
        expect(done, isA<LoginSignedIn>());
        expect(polls, 3);
        for (final r in h.server.to('/auth/pairing/complete')) {
          expect(r.body, {'pairing_id': pairingId});
          expect(r.proof!.header['jwk'], h.dpopKey.publicJwk);
        }
      },
    );

    test('an expired or rejected pairing (410) ends the polling', () async {
      h.server.routes['POST /auth/pairing/complete'] = (_) =>
          FakeReply.problem(410, 'gone');
      final done = await h.session.login.pollPairing(pairingId);
      expect(
        done,
        isA<LoginFailed>().having((f) => f.problem?.status, 'status', 410),
      );
    });
  });

  group('pairing with transient errors', () {
    for (final kind in ['network', '429', '503']) {
      test('pollPairing keeps polling after a transient $kind', () async {
        var polls = 0;
        h.server.routes['POST /auth/pairing/complete'] = (_) {
          polls++;
          if (polls == 1) {
            if (kind == 'network') throw NetworkDown();
            if (kind == '429') return FakeReply.problem(429, 'rate_limited');
            return FakeReply.problem(503, 'unavailable');
          }
          return tokens();
        };
        final done = await h.session.login.pollPairing(pairingId);
        expect(done, isA<LoginSignedIn>());
        expect(polls, 2);
      });
    }
  });

  group('password with TOTP', () {
    test(
      'without a code the service asks for the second factor, then signs in',
      () async {
        h.server.routes['POST /auth/password/login'] = (_) => tokens();
        final first = await h.session.login.loginWithPassword(
          loginName: 'alex',
          password: 'pw',
        );
        expect(first, isA<LoginNeedsSecondFactor>());
        expect(h.server.requests, isEmpty);
        final second = await h.session.login.loginWithPassword(
          loginName: 'alex',
          password: 'pw',
          totpCode: '123 456',
        );
        expect(second, isA<LoginSignedIn>());
        final body = h.server.to('/auth/password/login').single.body! as Map;
        expect(body['login_name'], 'alex');
        expect(body['totp_code'], '123456');
      },
    );

    test('a rejected login returns the problem and stores nothing', () async {
      h.server.routes['POST /auth/password/login'] = (_) =>
          FakeReply.problem(401, 'unauthenticated');
      final outcome = await h.session.login.loginWithPassword(
        loginName: 'alex',
        password: 'pw',
        totpCode: '1',
      );
      expect(
        outcome,
        isA<LoginFailed>().having(
          (f) => f.problem?.code,
          'code',
          'unauthenticated',
        ),
      );
      expect(
        h.store.values.keys.where((k) => k.endsWith('refresh_token')),
        isEmpty,
      );
      expect(h.events.whereType<SignedOut>(), isEmpty);
    });
  });

  group('other flows', () {
    test('proxy header login sends the device and a proof', () async {
      h.server.routes['POST /auth/trusted-header/login'] = (_) => tokens();
      final outcome = await h.session.login.loginWithTrustedHeader(
        enrollmentCode: 'en_ab-cd',
      );
      expect(outcome, isA<LoginSignedIn>());
      final r = h.server.to('/auth/trusted-header/login').single;
      expect((r.body! as Map)['enrollment_code'], 'en_ABCD');
      expect(r.proof, isNotNull);
    });

    test('passkey login goes through the authenticator port', () async {
      const challengeId = '0192f4a0-7b1c-7000-8000-0000000000b1';
      h.server.routes['POST /auth/passkey/begin'] = (_) =>
          const FakeReply(200, {
            'challenge_id': challengeId,
            'public_key': {'challenge': 'abc', 'rpId': 'chat.example.com'},
            'expires_at': '2026-10-15T10:10:00.000Z',
          });
      h.server.routes['POST /auth/passkey/complete'] = (_) => tokens();
      final outcome = await h.session.login.loginWithPasskey(loginName: 'alex');
      expect(outcome, isA<LoginSignedIn>());
      expect(h.passkeys.options.single, {
        'challenge': 'abc',
        'rpId': 'chat.example.com',
      });
      final body = h.server.to('/auth/passkey/complete').single.body! as Map;
      expect(body['challenge_id'], challengeId);
      expect(body['credential'], {'id': 'cred-1', 'type': 'public-key'});
    });

    test('enrollment code redemption with device keys', () async {
      h.server.routes['POST /auth/enroll/redeem'] = (_) => tokens();
      final outcome = await h.session.login.redeemEnrollmentCode(
        'en_ 7k2m 9qxa',
      );
      expect(outcome, isA<LoginSignedIn>());
      final body = h.server.to('/auth/enroll/redeem').single.body! as Map;
      expect(body['enrollment_code'], 'en_7K2M9QXA');
    });

    test('enrollment with a first passkey (begin and complete)', () async {
      const challengeId = '0192f4a0-7b1c-7000-8000-0000000000b2';
      h.server.routes['POST /auth/enroll/begin'] = (_) => const FakeReply(200, {
        'challenge_id': challengeId,
        'public_key': {'challenge': 'xyz'},
        'expires_at': '2026-10-15T10:10:00.000Z',
      });
      h.server.routes['POST /auth/enroll/complete'] = (_) => tokens();
      final outcome = await h.session.login.enrollWithPasskey(
        'EN_7K2M-9QXA',
        passkeyName: 'Alex laptop',
      );
      expect(outcome, isA<LoginSignedIn>());
      expect(
        (h.server.to('/auth/enroll/begin').single.body!
            as Map)['enrollment_code'],
        'en_7K2M9QXA',
      );
      final body = h.server.to('/auth/enroll/complete').single.body! as Map;
      expect(body['passkey_name'], 'Alex laptop');
      expect(body['credential'], {'id': 'cred-new', 'type': 'public-key'});
    });
  });

  group('codes', () {
    test(
      'spaces and hyphens are removed, en_ lower case, the rest upper case',
      () {
        expect(normalizeCode('en_ 7k2m 9qxa'), 'en_7K2M9QXA');
        expect(normalizeCode('EN_7K2M-9QXA'), 'en_7K2M9QXA');
        expect(normalizeCode('en_7K2M9QXA'), 'en_7K2M9QXA');
        expect(normalizeCode('k7m2-q9xa'), 'K7M2Q9XA');
        expect(normalizeCode('en_O0-il'), 'en_O0IL');
      },
    );
  });

  group('logout', () {
    test(
      'offline logout still clears the store and emits signedOut(loggedOut)',
      () async {
        await h.signIn();
        h.server.routes['POST /auth/logout'] = (_) => throw NetworkDown();
        await h.session.login.logout();
        await h.settle();
        expect(
          h.store.values.keys.where((k) => k.endsWith('refresh_token')),
          isEmpty,
        );
        expect(
          h.events.whereType<SignedOut>().single.reason,
          SignOutReason.loggedOut,
        );
        expect(h.session.session, isNull);
      },
    );

    test('online logout calls the server with the token first', () async {
      await h.signIn();
      h.server.routes['POST /auth/logout'] = (_) => const FakeReply(204);
      await h.session.login.logout();
      final r = h.server.to('/auth/logout').single;
      expect(r.headers['authorization'], startsWith('DPoP '));
      expect(h.log.last, 'delete refresh_token');
    });
  });
}
