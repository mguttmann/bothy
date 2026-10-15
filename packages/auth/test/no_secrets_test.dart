// SPDX-License-Identifier: Apache-2.0

// No token, refresh token, proof, code, verifier, password or TOTP in any toString, exception
// or log line (15 16, 19 SEC-LOG; brief M4-09 Design note 7).

import 'package:auth/auth.dart';
import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support/fakes.dart';

const _canaries = [
  'CANARY-ACCESS',
  'CANARY-REFRESH',
  'CANARY-PASSWORD',
  'CANARY-TOTP',
  'CANARY-CODE',
  'CANARY-LOGIN-CODE',
  'CANARY-VERIFY',
];

void _clean(Object? value) {
  final text = '$value';
  for (final c in _canaries) {
    expect(
      text,
      isNot(contains(c)),
      reason: 'leaked $c in ${value.runtimeType}',
    );
  }
  // A proof or token is a JWS: three base64url segments.
  expect(text, isNot(matches(RegExp(r'ey[A-Za-z0-9_-]{8,}\.'))));
}

void main() {
  test('public value types redact their secrets', () {
    final session = SessionInfo(
      sid: sessionId,
      deviceId: DeviceId.parse('0192f4a0-7b1c-7000-8000-0000000000d1'),
      accessExpiresAt: DateTime.utc(2026),
    );
    final state = TokenState(
      accessToken: 'CANARY-ACCESS',
      refreshToken: 'CANARY-REFRESH',
      session: session,
    );
    _clean(state);
    expect('$state', contains('13'));
    _clean(session);
    _clean(LoginSignedIn(state));
    _clean(
      PendingDevice(
        deviceId: DeviceId.parse('0192f4a0-7b1c-7000-8000-0000000000d1'),
        verificationCode: 'CANARY-VERIFY',
      ),
    );
    _clean(
      LoginPending(
        PendingDevice(
          deviceId: DeviceId.parse('0192f4a0-7b1c-7000-8000-0000000000d1'),
          verificationCode: 'CANARY-VERIFY',
        ),
      ),
    );
    _clean(
      PairingTicket(
        pairingId: '0192f4a0-7b1c-7000-8000-0000000000a1',
        code: 'CANARY-CODE',
        expiresAt: DateTime.utc(2026),
      ),
    );
    _clean(const LoginNeedsSecondFactor());
    _clean(const LoginFailed(reason: 'login_failed'));
    _clean(SignedIn(session));
    _clean(
      DevicePending(
        PendingDevice(
          deviceId: DeviceId.parse('0192f4a0-7b1c-7000-8000-0000000000d1'),
          verificationCode: 'CANARY-VERIFY',
        ),
      ),
    );
    _clean(const SignedOut(SignOutReason.loggedOut));
    _clean(const RefreshFailed());
    _clean(const NotSignedIn());
    _clean(const SessionEnded(SignOutReason.sessionRevoked));
    _clean(const RefreshUnavailable());
    _clean(const StepUpCancelled());
    _clean(redacted('access token', 'CANARY-ACCESS'));
  });

  test(
    'flows that fail print no secret in outcomes, errors or log lines',
    () async {
      final h = Harness();
      final lines = <String>[];
      h.session.logHook = lines.add;
      final failures = <Object>[];

      h.server.routes['POST /auth/password/login'] = (_) =>
          FakeReply.problem(401, 'unauthenticated');
      failures.add(
        await h.session.login.loginWithPassword(
          loginName: 'alex',
          password: 'CANARY-PASSWORD',
          totpCode: 'CANARY-TOTP',
        ),
      );

      h.server.routes['POST /auth/enroll/redeem'] = (_) => throw NetworkDown();
      failures.add(await h.session.login.redeemEnrollmentCode('CANARY-CODE'));

      h.server.routes['POST /auth/oidc/begin'] = (_) => const FakeReply(200, {
        'login_attempt_id': '0192f4a0-7b1c-7000-8000-0000000000f1',
        'authorization_url': 'https://idp.example.com/authorize',
        'expires_at': '2026-10-15T10:10:00.000Z',
      });
      h.server.routes['POST /auth/oidc/complete'] = (_) =>
          FakeReply.problem(401, 'unauthenticated');
      h.redirects.next = Uri.parse(
        'http://127.0.0.1:49152/callback?login_code=CANARY-LOGIN-CODE&login_attempt_id=0192f4a0-7b1c-7000-8000-0000000000f1',
      );
      failures.add(
        await h.session.login.beginOidc('0192f4a0-7b1c-7000-8000-0000000000f2'),
      );

      // A signed-in session whose refresh is rejected, and a request after it.
      h.server.routes['POST /auth/password/login'] = (_) => FakeReply(
        201,
        tokenBody(
          access: 'CANARY-ACCESS',
          refresh: 'CANARY-REFRESH',
          deviceId: h.deviceId,
        ),
      );
      await h.session.login.loginWithPassword(
        loginName: 'alex',
        password: 'CANARY-PASSWORD',
        totpCode: 'CANARY-TOTP',
      );
      h.server.routes['POST /auth/token/refresh'] = (_) => throw NetworkDown();
      try {
        await h.session.refresh.refreshNow();
      } on Object catch (e) {
        failures.add(e);
      }
      h.server.routes['POST /auth/token/refresh'] = (_) =>
          FakeReply.problem(401, 'session_revoked');
      try {
        await h.session.refresh.refreshNow();
      } on Object catch (e) {
        failures.add(e);
      }
      try {
        await h.session.api.getDevicesApi().getDevice(deviceId: h.deviceId);
      } on Object catch (e) {
        failures.add(e);
      }
      await h.settle();

      expect(failures, hasLength(6));
      for (final f in [...failures, ...h.events, ...lines]) {
        _clean(f);
      }
      expect(lines, isNotEmpty);
    },
  );
}
