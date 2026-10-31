// SPDX-License-Identifier: Apache-2.0

// The own devices, sessions, profile and step-up of the fake (brief M4-25 Scope "Fake server
// handlers"): `device.approve` and `device.revoke` answer `401 step_up_required` until the
// session confirmed with its step-up key, `session.revoke` never asks, and a pairing code
// without an open pairing ends the step-up of the session (08 5.3, 6.2, 10).

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart' show WorldSeed;
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;

  setUp(() async => h = await FakeWorldHarness.start());
  tearDown(() => h.stop());

  List<String> ops() => [
    for (final r in h.server.requests)
      if (r.operationId != 'loginWithPassword') '${r.operationId} ${r.status}',
  ];

  test('approving a waiting device asks for step-up once', () async {
    final alex = h.device();
    await signedIn(h, alex);
    h.seed.alex.newDevicesPending = true;
    final laptop = h.device(keySeed: 2);
    final waiting = (await laptop.signIn() as LoginPending).device;
    h.server.requests.clear();

    final approved = await alex.api.getDevicesApi().approveDevice(
      deviceId: waiting.deviceId.value,
      deviceApprove: DeviceApprove(
        (b) => b..verificationCode = waiting.verificationCode,
      ),
      idempotencyKey: '0193f2a7-0000-7000-8000-000000000001',
    );

    expect(approved.data!.state, DeviceStateEnum.active);
    expect(ops(), [
      'approveDevice 401',
      'createStepUpChallenge 200',
      'completeStepUpWithDeviceKey 200',
      'approveDevice 200',
    ]);
    // The step-up stays valid for the session: a second action asks no more.
    h.server.requests.clear();
    await alex.api.getDevicesApi().listMyDevices();
    expect(ops(), ['listMyDevices 200']);
    expect(h.server.violations, isEmpty);
  });

  test('a signature by another key does not confirm the session', () async {
    final alex = h.device();
    final s = await signedIn(h, alex);
    h.server.world.devices[s.deviceId]!.stepUpJwk = null;

    final error = await alex.api
        .getDevicesApi()
        .revokeDevice(
          deviceId: s.deviceId,
          reasonBody: ReasonBody((b) => b..reason = 'lost'),
          idempotencyKey: '0193f2a7-0000-7000-8000-000000000002',
        )
        .then<DioException?>(
          (_) => null,
          onError: (Object e) => e as DioException,
        );

    expect(ApiProblem.of(error!)?.code, 'forbidden');
    expect(ApiProblem.of(error)?.reason, 'step_up_not_possible');
    expect(h.server.world.devices[s.deviceId]!.state, 'active');
  });

  test(
    'a device without a step-up key confirms with password and TOTP',
    () async {
      final alex = h.device(stepUpKey: false);
      await signedIn(h, alex);
      final phone = h.device(keySeed: 2);
      final p = await signedIn(h, phone);
      h.server.requests.clear();

      await alex.api.getDevicesApi().revokeDevice(
        deviceId: p.deviceId,
        reasonBody: ReasonBody((b) => b..reason = 'Lost on the train'),
        idempotencyKey: '0193f2a7-0000-7000-8000-000000000009',
      );

      expect(ops(), [
        'revokeDevice 401',
        'createStepUpChallenge 200',
        'stepUpWithPassword 200',
        'revokeDevice 200',
      ]);
      expect(h.server.world.devices[p.deviceId]!.state, 'revoked');
      expect(h.server.violations, isEmpty);
    },
  );

  test(
    'a device with a step-up key cannot confirm with the password',
    () async {
      final alex = h.device();
      await signedIn(h, alex);
      final challenge =
          (await alex.api.getAuthApi().createStepUpChallenge()).data!;
      final error = await alex.api
          .getAuthApi()
          .stepUpWithPassword(
            stepUpPasswordRequest: StepUpPasswordRequest(
              (b) => b
                ..challengeId = challenge.challengeId
                ..password = WorldSeed.password
                ..totpCode = WorldSeed.totp,
            ),
          )
          .then<DioException?>(
            (_) => null,
            onError: (Object e) => e as DioException,
          );
      expect(ApiProblem.of(error!)?.reason, 'step_up_not_possible');
      expect(
        h.server.world.sessions.values.every((s) => s.stepUpAt == null),
        isTrue,
      );
    },
  );

  test('revoking a device needs step-up and ends its sessions', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final phone = h.device(keySeed: 2);
    final p = await signedIn(h, phone);
    h.server.requests.clear();

    final revoked = await alex.api.getDevicesApi().revokeDevice(
      deviceId: p.deviceId,
      reasonBody: ReasonBody((b) => b..reason = 'Lost on the train'),
      idempotencyKey: '0193f2a7-0000-7000-8000-000000000003',
    );

    expect(revoked.data!.state, DeviceStateEnum.revoked);
    expect(revoked.data!.revokeReason, DeviceRevokeReasonEnum.revokedBySelf);
    expect(ops().first, 'revokeDevice 401');
    expect(ops().last, 'revokeDevice 200');
    final sessions = h.server.world.sessions.values.where(
      (s) => s.deviceId == p.deviceId,
    );
    expect(sessions.every((s) => s.revoked), isTrue);
    final list = (await alex.api.getDevicesApi().listMyDevices()).data!.items;
    expect(
      list.firstWhere((d) => d.id == p.deviceId).state,
      DeviceStateEnum.revoked,
    );
    expect(h.server.violations, isEmpty);
  });

  test('ending a session asks for no step-up', () async {
    final alex = h.device();
    final a = await signedIn(h, alex);
    final phone = h.device(keySeed: 2);
    await signedIn(h, phone);

    final sessions =
        (await alex.api.getDevicesApi().listMySessions()).data!.items;
    expect(sessions, hasLength(2));
    final own = sessions.firstWhere((s) => s.current ?? false);
    expect(own.deviceId, a.deviceId);
    final other = sessions.firstWhere((s) => !(s.current ?? false));
    h.server.requests.clear();

    await alex.api.getDevicesApi().revokeSession(
      sessionId: other.id,
      idempotencyKey: '0193f2a7-0000-7000-8000-000000000004',
    );

    expect(ops(), ['revokeSession 204']);
    expect(h.server.world.sessions[other.id]!.revoked, isTrue);
    expect(h.server.violations, isEmpty);
  });

  test('a pairing code without a pairing ends the step-up', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final tablet = h.device(keySeed: 2);
    final ticket = await tablet.auth.login.beginPairing();
    final devices = alex.api.getDevicesApi();
    h.server.requests.clear();

    final missing = await devices
        .approvePairing(
          pairingApprove: PairingApprove((b) => b..pairingCode = 'ZZZZZZZZ'),
          idempotencyKey: '0193f2a7-0000-7000-8000-000000000005',
        )
        .then<DioException?>(
          (_) => null,
          onError: (Object e) => e as DioException,
        );
    expect(ApiProblem.of(missing!)?.status, 404);
    expect(ops(), [
      'approvePairing 401',
      'createStepUpChallenge 200',
      'completeStepUpWithDeviceKey 200',
      'approvePairing 404',
    ]);
    h.server.requests.clear();

    await devices.approvePairing(
      pairingApprove: PairingApprove((b) => b..pairingCode = ticket.code),
      idempotencyKey: '0193f2a7-0000-7000-8000-000000000006',
    );
    expect(ops().first, 'approvePairing 401');
    expect(ops().last, 'approvePairing 201');
    expect(h.server.violations, isEmpty);
  });

  test('the profile keeps language and time zone', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final me = alex.api.getMeApi();

    final user = await me.updateMyProfile(
      profileUpdate: ProfileUpdate(
        (b) => b
          ..locale = 'en'
          ..timeZone = 'Europe/Berlin',
      ),
      idempotencyKey: '0193f2a7-0000-7000-8000-000000000007',
    );

    expect(user.data!.locale, 'en');
    final read = (await me.getMe()).data!;
    expect(read.locale, 'en');
    expect(read.timeZone, 'Europe/Berlin');
    expect(h.server.violations, isEmpty);
  });
}
