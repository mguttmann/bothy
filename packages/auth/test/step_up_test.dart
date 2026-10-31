// SPDX-License-Identifier: Apache-2.0

// Step-up after 08 6.2 and 14.3 and the repeat with the same Idempotency-Key (brief M4-09
// Design note 6).

import 'dart:convert';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';
import 'package:test/test.dart';

import 'support/fakes.dart';
import 'support/software_p256_signer.dart';

void main() {
  late Harness h;
  const challengeId = '0192f4a0-7b1c-7000-8000-0000000000c1';

  setUp(() async {
    h = Harness();
    await h.signIn();
    h.server.routes['POST /auth/step-up/challenge'] = (_) =>
        const FakeReply(200, {
          'challenge_id': challengeId,
          'challenge': 'Y2hhbGxlbmdl',
          'expires_at': '2026-10-15T10:05:00.000Z',
        });
    h.server.routes['POST /auth/step-up/device-key'] = (_) =>
        const FakeReply(200, {
          'step_up_at': '2026-10-15T10:00:00.000Z',
          'valid_until': '2026-10-15T10:05:00.000Z',
        });
  });

  // A write with an Idempotency-Key whose action needs step-up (revoking another device).
  Future<Response<Device>> revoke() =>
      h.session.api.getDevicesApi().revokeDevice(
        deviceId: h.deviceId,
        reasonBody: ReasonBody((b) => b..reason = 'lost'),
        extra: {
          idempotencyKeyExtra: '0192f4a0-7b1c-7000-8000-0000000000e1',
          stepUpPurposeExtra: 'Revoke the device of Sam',
        },
      );

  test(
    'step_up_required runs challenge, signature, confirmation and one repeat',
    () async {
      var calls = 0;
      h.server.routes['POST /devices/${h.deviceId}/revoke'] = (_) {
        calls++;
        return calls == 1
            ? FakeReply.problem(401, 'step_up_required')
            : FakeReply(200, deviceBody(h.deviceId, 'revoked'));
      };
      final res = await revoke();
      expect(res.data!.state, DeviceStateEnum.revoked);
      expect(h.server.requests.map((r) => r.path), [
        '/devices/${h.deviceId}/revoke',
        '/auth/step-up/challenge',
        '/auth/step-up/device-key',
        '/devices/${h.deviceId}/revoke',
      ]);
      final original = h.server.requests.first;
      final repeat = h.server.requests.last;
      expect(
        repeat.headers['idempotency-key'],
        '0192f4a0-7b1c-7000-8000-0000000000e1',
      );
      expect(
        repeat.headers['idempotency-key'],
        original.headers['idempotency-key'],
      );
      expect(repeat.body, original.body);
      expect(
        repeat.proof!.payload['jti'],
        isNot(original.proof!.payload['jti']),
      );
      expect(
        repeat.headers['authorization'],
        original.headers['authorization'],
      );
      expect(h.prompt.asked, ['Revoke the device of Sam']);
    },
  );

  test(
    'the JWS header is exactly {"alg":"ES256"} over challenge, sid and iat',
    () async {
      await h.session.stepUp.confirm(purposeText: 'Approve a device');
      final body = h.server.to('/auth/step-up/device-key').single.body! as Map;
      expect(body['challenge_id'], challengeId);
      final parts = (body['signature']! as String).split('.');
      expect(utf8.decode(b64Decode(parts[0])), '{"alg":"ES256"}');
      final payload =
          jsonDecode(utf8.decode(b64Decode(parts[1]))) as Map<String, Object?>;
      expect(payload, {
        'challenge': 'Y2hhbGxlbmdl',
        'sid': sessionId,
        'iat': h.clock.now().millisecondsSinceEpoch ~/ 1000,
      });
      final sig = b64Decode(parts[2]);
      expect(sig, hasLength(64));
      expect(
        h.stepUpKey.verify(utf8.encode('${parts[0]}.${parts[1]}'), sig),
        isTrue,
      );
      expect(
        h.dpopKey.verify(utf8.encode('${parts[0]}.${parts[1]}'), sig),
        isFalse,
      );
    },
  );

  test(
    'a cancelled prompt returns StepUpCancelled and confirms nothing',
    () async {
      h.prompt.answer = false;
      h.server.routes['POST /devices/${h.deviceId}/revoke'] = (_) =>
          FakeReply.problem(401, 'step_up_required');
      await expectLater(
        revoke(),
        throwsA(
          isA<DioException>().having(
            (e) => e.error,
            'error',
            isA<StepUpCancelled>(),
          ),
        ),
      );
      expect(h.server.to('/auth/step-up/device-key'), isEmpty);
      expect(h.stepUpKey.signCount, 0);
    },
  );

  group('a device without a step-up key (Windows, D-M4-11-2)', () {
    const passwordPath = '/auth/step-up/password';

    setUp(() async {
      h = Harness(withStepUpKey: false);
      await h.signIn();
      h.server.routes['POST /auth/step-up/challenge'] = (_) =>
          const FakeReply(200, {
            'challenge_id': challengeId,
            'challenge': 'Y2hhbGxlbmdl',
            'expires_at': '2026-10-15T10:05:00.000Z',
          });
      h.server.routes['POST $passwordPath'] = (_) => const FakeReply(200, {
        'step_up_at': '2026-10-15T10:00:00.000Z',
        'valid_until': '2026-10-15T10:05:00.000Z',
      });
    });

    test(
      'a device without a step-up key confirms with password and TOTP',
      () async {
        h.prompt.passwordAnswers.add((
          password: 'correct horse',
          totpCode: ' 123456 ',
        ));
        await h.session.stepUp.confirm(purposeText: 'Assign the runner');
        expect(h.server.requests.map((r) => r.path), [
          '/auth/step-up/challenge',
          passwordPath,
        ]);
        final sent = h.server.to(passwordPath).single;
        expect(sent.body, {
          'challenge_id': challengeId,
          'password': 'correct horse',
          'totp_code': '123456',
        });
        expect(sent.headers.containsKey('x-console-request'), isFalse);
        expect(sent.headers['authorization'], startsWith('DPoP '));
        expect(h.prompt.askedPassword, [
          (purposeText: 'Assign the runner', rejected: false),
        ]);
        expect(h.prompt.asked, isEmpty);
        expect(h.server.to('/auth/step-up/device-key'), isEmpty);
      },
    );

    test(
      'step_up_required of an action repeats it after the password step-up',
      () async {
        h.prompt.passwordAnswers.add((
          password: 'correct horse',
          totpCode: '123456',
        ));
        var calls = 0;
        h.server.routes['POST /devices/${h.deviceId}/revoke'] = (_) {
          calls++;
          return calls == 1
              ? FakeReply.problem(401, 'step_up_required')
              : FakeReply(200, deviceBody(h.deviceId, 'revoked'));
        };
        final res = await revoke();
        expect(res.data!.state, DeviceStateEnum.revoked);
        expect(h.server.requests.map((r) => r.path), [
          '/devices/${h.deviceId}/revoke',
          '/auth/step-up/challenge',
          passwordPath,
          '/devices/${h.deviceId}/revoke',
        ]);
        expect(
          h.server.requests.last.headers['idempotency-key'],
          '0192f4a0-7b1c-7000-8000-0000000000e1',
        );
      },
    );

    test('cancelling the credentials prompt throws StepUpCancelled', () async {
      h.prompt.passwordAnswers.add(null);
      await expectLater(
        h.session.stepUp.confirm(purposeText: 'x'),
        throwsA(isA<StepUpCancelled>()),
      );
      expect(h.server.to(passwordPath), isEmpty);
    });

    test('a wrong password or code asks again with a new challenge until it is accepted', () async {
      h.prompt.passwordAnswers
        ..add((password: 'wrong', totpCode: '111111'))
        ..add((password: 'correct horse', totpCode: '222222'));
      var calls = 0;
      h.server.routes['POST $passwordPath'] = (_) {
        calls++;
        return calls == 1
            ? FakeReply.problem(401, 'step_up_required')
            : const FakeReply(200, {
                'step_up_at': '2026-10-15T10:00:00.000Z',
                'valid_until': '2026-10-15T10:05:00.000Z',
              });
      };
      await h.session.stepUp.confirm(purposeText: 'x');
      expect(h.server.to('/auth/step-up/challenge'), hasLength(2));
      expect(h.server.to(passwordPath), hasLength(2));
      expect(h.prompt.askedPassword.map((a) => a.rejected), [false, true]);
    });

    test('a wrong code and then a cancel throws StepUpCancelled', () async {
      h.prompt.passwordAnswers.add((password: 'wrong', totpCode: '111111'));
      h.server.routes['POST $passwordPath'] = (_) =>
          FakeReply.problem(401, 'step_up_required');
      await expectLater(
        h.session.stepUp.confirm(purposeText: 'x'),
        throwsA(isA<StepUpCancelled>()),
      );
      expect(h.server.to(passwordPath), hasLength(1));
      expect(h.prompt.askedPassword.map((a) => a.rejected), [false, true]);
    });

    test('403 step_up_not_possible from the password endpoint throws StepUpUnavailable', () async {
      h.prompt.passwordAnswers.add((password: 'pw', totpCode: '123456'));
      h.server.routes['POST $passwordPath'] = (_) =>
          FakeReply.problem(403, 'forbidden', reason: 'step_up_not_possible');
      await expectLater(
        h.session.stepUp.confirm(purposeText: 'x'),
        throwsA(isA<StepUpUnavailable>()),
      );
      expect(h.prompt.askedPassword, hasLength(1));
    });

    test('404 from the password endpoint (local passwords off) throws StepUpUnavailable', () async {
      h.prompt.passwordAnswers.add((password: 'pw', totpCode: '123456'));
      h.server.routes.remove('POST $passwordPath');
      await expectLater(
        h.session.stepUp.confirm(purposeText: 'x'),
        throwsA(isA<StepUpUnavailable>()),
      );
      expect(h.server.to(passwordPath), hasLength(1));
    });

    test('a session revoked at the fail limit ends the step-up without asking again', () async {
      h.prompt.passwordAnswers
        ..add((password: 'wrong', totpCode: '111111'))
        ..add((password: 'wrong', totpCode: '222222'));
      h.server.routes['POST $passwordPath'] = (_) =>
          FakeReply.problem(401, 'session_revoked');
      await expectLater(
        h.session.stepUp.confirm(purposeText: 'x'),
        throwsA(isNot(isA<StepUpCancelled>())),
      );
      expect(h.server.to(passwordPath), hasLength(1));
      expect(h.prompt.askedPassword, hasLength(1));
    });
  });

  test('the fresh step-up of the terminal needs the step-up key and asks no password', () async {
    h = Harness(withStepUpKey: false);
    await h.signIn();
    await expectLater(
      h.session.stepUp.confirmFresh(purposeText: 'Open the terminal'),
      throwsA(isA<StepUpUnavailable>()),
    );
    expect(h.prompt.askedPassword, isEmpty);
    expect(h.server.to('/auth/step-up/challenge'), isEmpty);
  });

  test('the fresh step-up with a step-up key signs like confirm', () async {
    await h.session.stepUp.confirmFresh(purposeText: 'Open the terminal');
    expect(h.prompt.asked, ['Open the terminal']);
    expect(h.server.to('/auth/step-up/device-key'), hasLength(1));
  });

  test('a device with a step-up key never asks for a password (iOS, Android, macOS)', () async {
    await h.session.stepUp.confirm(purposeText: 'Approve a device');
    expect(h.prompt.askedPassword, isEmpty);
    expect(h.prompt.asked, ['Approve a device']);
    expect(h.server.to('/auth/step-up/password'), isEmpty);
    expect(h.server.to('/auth/step-up/device-key'), hasLength(1));
  });

  test(
    'a cancelled OS confirmation in the signer is StepUpCancelled too',
    () async {
      h.stepUpKey.failWith = const StepUpCancelled();
      await expectLater(
        h.session.stepUp.confirm(purposeText: 'x'),
        throwsA(isA<StepUpCancelled>()),
      );
      expect(h.server.to('/auth/step-up/device-key'), isEmpty);
    },
  );

  test(
    'a second step_up_required after confirmation is returned, never looped',
    () async {
      h.server.routes['POST /devices/${h.deviceId}/revoke'] = (_) =>
          FakeReply.problem(
            401,
            'step_up_required',
            reason: 'terminal_step_up_not_fresh',
          );
      await expectLater(
        revoke(),
        throwsA(
          isA<DioException>().having(
            (e) => ApiProblem.of(e)?.code,
            'code',
            'step_up_required',
          ),
        ),
      );
      expect(h.server.to('/devices/${h.deviceId}/revoke'), hasLength(2));
      expect(h.server.to('/auth/step-up/device-key'), hasLength(1));
    },
  );
}
