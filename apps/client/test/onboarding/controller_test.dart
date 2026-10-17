// SPDX-License-Identifier: Apache-2.0

// The onboarding flows against the fake server (brief M4-18 Design notes 2 and 3; 08 5.2, 5.3;
// R-SY-14): a waiting device that is confirmed, rejected or expires; pairing with
// confirmation, a wrong code, expiry and a new code; logout and 4012 delete every local datum.

import 'dart:io';

import 'package:client/app/account_wiper.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:domain/domain.dart' show AppConfigValues;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import '../support/fakes.dart' show fastConfig;
import '../support/other_device.dart';

void main() {
  // flutter_test answers every HttpClient request with 400; these tests talk to the fake.
  HttpOverrides? saved;
  late FakeInstance f;
  late ProviderContainer c;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    f = await FakeInstance.start();
    c = appContainer(f);
  });

  tearDown(() async {
    c.dispose();
    await f.stop();
    HttpOverrides.global = saved;
  });

  OnboardingState state() => c.read(onboardingControllerProvider);

  Future<void> loginAsAlex(OnboardingController control) => control.loginWithPassword(
    WorldSeed.alexLogin,
    WorldSeed.password,
    WorldSeed.totp,
  );

  String deviceId() => state().services!.auth.session!.deviceId.value;

  test('password login without a pending rule goes straight to the main view', () async {
    final control = await connected(c, f);
    expect(state().step, OnboardingStep.login);
    expect(state().instance?.productName, f.server.world.productName);
    await loginAsAlex(control);
    expect(state().step, OnboardingStep.signedIn);
    await eventually('live', () => state().services?.engine.state.phase == ConnectionPhase.live);
    expect(f.server.violations, isEmpty);
  });

  test('a wrong password shows the failure text and stays on E2', () async {
    final control = await connected(c, f);
    await control.loginWithPassword(WorldSeed.alexLogin, 'wrong', WorldSeed.totp);
    expect(state().step, OnboardingStep.login);
    expect(state().notice, LoginNotice.failed);
  });

  group('waiting device (E4)', () {
    late OtherDevice other;

    setUp(() async {
      other = OtherDevice(f.server);
      await other.signIn();
      f.seed.alex.newDevicesPending = true;
    });

    tearDown(() => other.dispose());

    test('confirmed on the other device: the main view', () async {
      final control = await connected(c, f);
      await loginAsAlex(control);
      expect(state().step, OnboardingStep.pending);
      final code = state().code!;
      expect(code, isNotEmpty);
      final created = DateTime.parse(
        f.server.world.devices[deviceId()]!.createdAt,
      );
      expect(state().deadline, created.add(f.ports.config.devicePendingTtl));
      await other.confirm(deviceId(), code);
      await eventually('signed in', () => state().step == OnboardingStep.signedIn);
      await eventually('live', () => state().services?.engine.state.phase == ConnectionPhase.live);
    });

    test('a wrong code on the other device rejects it: E7 and a wiped store', () async {
      final control = await connected(c, f);
      await loginAsAlex(control);
      expect(state().step, OnboardingStep.pending);
      await other.confirm(deviceId(), '000000');
      await eventually('E7', () => state().step == OnboardingStep.signedOut);
      expect(state().cause, SignOutCause.rejected);
      expect(await f.wiped(), isTrue);
    });

    test('expiry: E7 with its reason and a wiped store', () async {
      // A short auth.device_pending_ttl, so the deadline of E4 passes in the test.
      await other.dispose();
      c.dispose();
      await f.stop();
      f = await FakeInstance.start(
        config: {...fastConfig, 'auth.device_pending_ttl': '1s'},
      );
      c = appContainer(f);
      other = OtherDevice(f.server);
      await other.signIn();
      f.seed.alex.newDevicesPending = true;
      final control = await connected(c, f);
      await loginAsAlex(control);
      await eventually(
        'deadline passed',
        () => !DateTime.now().toUtc().isBefore(state().deadline!),
      );
      f.server.world.expirePendingDevice(deviceId());
      await eventually('E7', () => state().step == OnboardingStep.signedOut);
      expect(state().cause, SignOutCause.pendingExpired);
      expect(await f.wiped(), isTrue);
    });

    test('"Cancel" logs out and deletes the keys', () async {
      final control = await connected(c, f);
      await loginAsAlex(control);
      expect(state().step, OnboardingStep.pending);
      await control.cancelPending();
      expect(state().step, OnboardingStep.signedOut);
      expect(state().cause, SignOutCause.loggedOut);
      expect(await f.wiped(), isTrue);
      await control.loginAgain();
      expect(state().step, OnboardingStep.login);
      expect(f.keys.scopes, {f.server.world.instanceId});
    });
  });

  group('pairing (E5)', () {
    late OtherDevice other;

    setUp(() async {
      other = OtherDevice(f.server);
      await other.signIn();
    });

    tearDown(() => other.dispose());

    test('confirmed on the other device: the new device is active', () async {
      final control = await connected(c, f);
      final pairing = control.openPairing();
      await eventually('code', () => state().code != null);
      expect(state().step, OnboardingStep.pair);
      expect(state().deadline, isNotNull);
      await other.approvePairing(state().code!);
      await pairing;
      expect(state().step, OnboardingStep.signedIn);
    });

    test('a wrong code keeps waiting; expiry shows its text; "New code" starts again', () async {
      final control = await connected(c, f);
      final first = control.openPairing();
      await eventually('code', () => state().code != null);
      final code = state().code!;
      await expectLater(other.approvePairing('ZZZZZZZZ'), throwsA(anything));
      expect(state().step, OnboardingStep.pair);
      expect(state().pairingExpired, isFalse);
      f.server.world.expirePairing(f.server.world.pairings.keys.single);
      await first;
      expect(state().pairingExpired, isTrue);
      final second = control.openPairing();
      await eventually('new code', () => state().code != null);
      expect(state().code, isNot(code));
      control.backToLogin();
      await second;
      expect(state().step, OnboardingStep.login);
    });
  });

  test('logout deletes database, file cache, keys and tokens (R-SY-14)', () async {
    final control = await connected(c, f);
    await loginAsAlex(control);
    await eventually('live', () => state().services?.engine.state.phase == ConnectionPhase.live);
    await File('${f.instanceFolder.path}/files/a.bin').create(recursive: true);
    expect(File('${f.instanceFolder.path}/local.db').existsSync(), isTrue);
    await control.logout();
    expect(state().step, OnboardingStep.signedOut);
    expect(state().cause, SignOutCause.loggedOut);
    expect(await f.wiped(), isTrue);
    expect(f.server.requestsOf('logout'), hasLength(1));
  });

  test('4012 from the fake does the same', () async {
    final control = await connected(c, f);
    await loginAsAlex(control);
    await eventually('live', () => state().services?.engine.state.phase == ConnectionPhase.live);
    f.server.faults.revoke(deviceId: deviceId());
    await eventually('E7', () => state().step == OnboardingStep.signedOut);
    expect(state().cause, SignOutCause.sessionRevoked);
    expect(await f.wiped(), isTrue);
  });

  test('a stored session restarts signed in; a revoked one ends in E7', () async {
    final control = await connected(c, f);
    await loginAsAlex(control);
    await eventually('live', () => state().services?.engine.state.phase == ConnectionPhase.live);
    final id = deviceId();
    c.dispose();
    c = appContainer(f);
    c.read(onboardingControllerProvider);
    await eventually('signed in again', () => state().step == OnboardingStep.signedIn);
    await eventually('live again', () => state().services?.engine.state.phase == ConnectionPhase.live);
    f.server.world.revokeDevice(id);
    await eventually('E7', () => state().step == OnboardingStep.signedOut);
    expect(await f.wiped(), isTrue);
  });
}
