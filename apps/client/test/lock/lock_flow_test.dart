// SPDX-License-Identifier: Apache-2.0

// The app lock over the full chain (brief M5-08 Design notes 1 to 4): a phone build against
// the fake server of M4-17 takes the policy of `getInstance` into the instance row, a fresh
// login does not lock, a cold start with the stored session locks, the return after
// `client.app_lock_after` locks, and "Log out" from the lock deletes every local datum and
// shows E7.

import 'dart:io';

import 'package:client/app/account_wiper.dart' show SignOutCause;
import 'package:client/lock/app_lock_controller.dart';
import 'package:client/lock/providers.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:domain/domain.dart' show AppLockPolicy;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart' show AppActivity, ConnectionPhase;
import 'package:sync_engine/testing.dart' show WorldSeed;

import '../support/fake_instance.dart';
import 'support/lock_fakes.dart';

void main() {
  // flutter_test answers every HttpClient request with 400; these tests talk to the fake.
  HttpOverrides? saved;
  late FakeInstance f;
  late ProviderContainer c;
  late FakeAppLockPort port;
  late ManualLifecycle lifecycle;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    port = FakeAppLockPort();
    lifecycle = ManualLifecycle();
    f = await FakeInstance.start(
      platform: 'ios',
      appLock: port,
      lifecycle: lifecycle,
    );
    c = appContainer(f);
  });

  tearDown(() async {
    c.dispose();
    await f.stop();
    HttpOverrides.global = saved;
  });

  OnboardingState state() => c.read(onboardingControllerProvider);

  Future<AppLockController> signIn() async {
    final control = await connected(c, f);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await eventually(
      'live',
      () => state().services?.engine.state.phase == ConnectionPhase.live,
    );
    final lock = c.read(appLockProvider)!;
    await lock.decided;
    return lock;
  }

  test(
    'the policy of getInstance reaches the lock; a login does not lock',
    () async {
      f.server.world
        ..appLockRequired = true
        ..appLockAfterMs = 120000;
      final lock = await signIn();
      await eventually('policy', () => lock.policy != null);
      expect(lock.policy, const AppLockPolicy(required: true, afterMs: 120000));
      expect(
        (await state().services!.database.instanceDao.get())?.appLock,
        lock.policy,
        reason: 'cached with the instance row',
      );
      expect(lock.status, AppLockStatus.unlocked);
      expect(lock.required, isTrue);
    },
  );

  test('the return after the time locks; a cold start locks; log out wipes and shows E7', () async {
    // The shortest time the fake can report, so the return needs no long wait; the clock of
    // the app is the system clock of the full chain.
    f.server.world.appLockAfterMs = 1;
    var lock = await signIn();
    await eventually('policy', () => lock.policy?.afterMs == 1);
    // The lock is turned on while the app is in the background, so no timer of the
    // foreground runs and only the return can lock.
    lifecycle.set(AppActivity.background);
    expect(
      await lock.setEnabled(true, reason: 'Unlock'),
      AppLockSwitchResult.changed,
    );
    expect(port.prompts, ['Unlock'], reason: 'turning on asks once');
    final start = DateTime.now();
    await eventually(
      'the time passed',
      () => DateTime.now().difference(start) > const Duration(milliseconds: 2),
    );
    expect(
      lock.status,
      AppLockStatus.unlocked,
      reason: 'nothing locks in the background',
    );
    lifecycle.set(AppActivity.foreground);
    await eventually('locked', () => lock.status == AppLockStatus.locked);

    // A cold start: a new app process over the same storage resumes the session and locks
    // before any content.
    c.dispose();
    c = appContainer(f);
    // The controller connects to the remembered address by itself, as at a real start.
    c.read(onboardingControllerProvider);
    await eventually('resumed', () => state().step == OnboardingStep.signedIn);
    expect(state().restored, isTrue);
    lock = c.read(appLockProvider)!;
    expect(lock.status, AppLockStatus.checking);
    await lock.decided;
    expect(lock.status, AppLockStatus.locked);

    await c.read(onboardingControllerProvider.notifier).logout();
    expect(state().step, OnboardingStep.signedOut);
    expect(state().cause, SignOutCause.loggedOut);
    expect(
      await f.wiped(),
      isTrue,
      reason: 'database, files, keys and secrets',
    );
    expect(c.read(appLockProvider), isNull);
  });
}
