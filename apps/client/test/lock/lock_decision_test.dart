// SPDX-License-Identifier: Apache-2.0

// When the app lock closes (brief M5-08 Design note 2; 08 9; 01 5.6 E6): at a cold start when
// on, never when off and not required, after `client.app_lock_after` without use in the
// foreground and in the background; activity resets the time, nothing else does. A fake clock,
// no device.

import 'package:client/lock/app_lock_controller.dart';
import 'package:client/lock/lock_decision.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart' show AppActivity;

import 'support/lock_fakes.dart';

void main() {
  const after = Duration(minutes: 5);
  const second = Duration(seconds: 1);

  group('the pure rules', () {
    test('the lock applies when on or required', () {
      expect(lockApplies(enabled: false, policy: null), isFalse);
      expect(lockApplies(enabled: false, policy: policy()), isFalse);
      expect(lockApplies(enabled: true, policy: null), isTrue);
      expect(
        lockApplies(enabled: false, policy: policy(required: true)),
        isTrue,
      );
    });

    test('the time comes from the policy; without one it is zero', () {
      expect(lockAfter(policy(after: after)), after);
      expect(lockAfter(null), Duration.zero);
    });

    test('the idle time is reached exactly at the configured time', () {
      LockInstant at(Duration d) => LockInstant(lockStart.add(d), d);
      final last = at(Duration.zero);
      expect(
        idleReached(lastActivity: last, now: at(after - second), after: after),
        isFalse,
      );
      expect(
        idleReached(lastActivity: last, now: at(after), after: after),
        isTrue,
      );
      expect(
        idleLeft(lastActivity: last, now: at(second), after: after),
        after - second,
      );
      expect(
        idleLeft(lastActivity: last, now: at(after * 2), after: after),
        Duration.zero,
      );
    });

    test(
      'the larger clock counts; a wall clock set back counts as reached',
      () {
        final last = LockInstant(lockStart, Duration.zero);
        // The device slept: the wall clock moved, the monotonic clock stood still.
        expect(
          idleReached(
            lastActivity: last,
            now: LockInstant(lockStart.add(after), second),
            after: after,
          ),
          isTrue,
        );
        // The clock was set forward by less than the time: the monotonic clock counts.
        expect(
          idleReached(
            lastActivity: last,
            now: LockInstant(lockStart.add(second), after),
            after: after,
          ),
          isTrue,
        );
        // The clock was set back: reached at once, whatever the monotonic clock says.
        final back = LockInstant(lockStart.subtract(second), second);
        expect(
          idleReached(lastActivity: last, now: back, after: after),
          isTrue,
        );
        expect(
          idleLeft(lastActivity: last, now: back, after: after),
          Duration.zero,
        );
      },
    );
  });

  group('the controller', () {
    late LockRig rig;
    tearDown(() => rig.dispose());

    test('a cold start locks when the person turned the lock on', () async {
      rig = LockRig();
      expect(rig.controller.status, AppLockStatus.checking);
      await rig.settle();
      expect(rig.controller.status, AppLockStatus.locked);
    });

    test('a cold start locks when the owner requires the lock', () async {
      rig = LockRig(on: false, initial: policy(required: true));
      await rig.settle();
      expect(rig.controller.status, AppLockStatus.locked);
    });

    test('off and not required never locks', () async {
      rig = LockRig(on: false);
      await rig.settle();
      expect(rig.controller.status, AppLockStatus.unlocked);
      rig.advance(after * 10);
      await rig.controller.check();
      rig.lifecycle
        ..set(AppActivity.background)
        ..set(AppActivity.foreground);
      await rig.controller.check();
      expect(rig.controller.status, AppLockStatus.unlocked);
    });

    test('a fresh login does not lock at once', () async {
      rig = LockRig(lockAtStart: false);
      await rig.settle();
      expect(rig.controller.status, AppLockStatus.unlocked);
    });

    test(
      'the foreground locks after the configured time without use',
      () async {
        rig = LockRig(lockAtStart: false);
        await rig.settle();
        rig.advance(after - second);
        await rig.controller.check();
        expect(rig.controller.status, AppLockStatus.unlocked);
        rig.advance(second);
        await rig.controller.check();
        expect(rig.controller.status, AppLockStatus.locked);
      },
    );

    test('the background counts: the return after the time locks', () async {
      rig = LockRig(lockAtStart: false);
      await rig.settle();
      rig.lifecycle.set(AppActivity.background);
      rig.advance(after);
      rig.lifecycle.set(AppActivity.foreground);
      await rig.controller.check();
      expect(rig.controller.status, AppLockStatus.locked);
    });

    test('a short trip to the background does not lock', () async {
      rig = LockRig(lockAtStart: false);
      await rig.settle();
      rig.lifecycle.set(AppActivity.background);
      rig.advance(after - second);
      rig.lifecycle.set(AppActivity.foreground);
      await rig.controller.check();
      expect(rig.controller.status, AppLockStatus.unlocked);
    });

    test('activity resets the time', () async {
      rig = LockRig(lockAtStart: false);
      await rig.settle();
      rig.advance(after - second);
      rig.controller.recordActivity();
      rig.advance(after - second);
      await rig.controller.check();
      expect(rig.controller.status, AppLockStatus.unlocked);
      rig.advance(second);
      await rig.controller.check();
      expect(rig.controller.status, AppLockStatus.locked);
    });

    test(
      'an incoming message is no activity: only recordActivity resets',
      () async {
        rig = LockRig(lockAtStart: false);
        await rig.settle();
        // The engine storing a message, a running upload or a new policy row does not call
        // recordActivity; the time runs on.
        rig.advance(after - second);
        rig.deliver(policy());
        await Future<void>.delayed(Duration.zero);
        rig.advance(second);
        await rig.controller.check();
        expect(rig.controller.status, AppLockStatus.locked);
      },
    );

    test('a later policy shortens the time', () async {
      rig = LockRig(lockAtStart: false);
      await rig.settle();
      rig.deliver(policy(after: const Duration(seconds: 30)));
      await Future<void>.delayed(Duration.zero);
      rig.advance(const Duration(seconds: 30));
      await rig.controller.check();
      expect(rig.controller.status, AppLockStatus.locked);
    });

    test(
      'without a policy only the return from the background locks',
      () async {
        rig = LockRig(lockAtStart: false);
        rig.deliver(null);
        await rig.settle();
        await Future<void>.delayed(Duration.zero);
        rig.advance(after * 10);
        await rig.controller.check();
        expect(rig.controller.status, AppLockStatus.unlocked, reason: 'in use');
        rig.lifecycle.set(AppActivity.background);
        rig.advance(second);
        rig.lifecycle.set(AppActivity.foreground);
        await rig.controller.check();
        expect(rig.controller.status, AppLockStatus.locked);
      },
    );

    test(
      'setting the clock back in the background does not keep the lock open',
      () async {
        rig = LockRig(lockAtStart: false);
        await rig.settle();
        rig.lifecycle.set(AppActivity.background);
        rig.advance(const Duration(hours: 1));
        rig.setWallBack(const Duration(days: 1));
        rig.lifecycle.set(AppActivity.foreground);
        await rig.controller.check();
        expect(rig.controller.status, AppLockStatus.locked);
      },
    );

    test(
      'setting the clock back by less than the time away still locks',
      () async {
        rig = LockRig(lockAtStart: false);
        await rig.settle();
        rig.lifecycle.set(AppActivity.background);
        rig.advance(after);
        // The wall clock now says only one second passed; the monotonic clock says five minutes.
        rig.setWallBack(after - second);
        rig.lifecycle.set(AppActivity.foreground);
        await rig.controller.check();
        expect(rig.controller.status, AppLockStatus.locked);
      },
    );

    test('the time the device slept counts', () async {
      rig = LockRig(lockAtStart: false);
      await rig.settle();
      rig.lifecycle.set(AppActivity.background);
      rig.sleep(after);
      rig.lifecycle.set(AppActivity.foreground);
      await rig.controller.check();
      expect(rig.controller.status, AppLockStatus.locked);
    });

    test('activity while locked does not open the lock', () async {
      rig = LockRig();
      await rig.settle();
      rig.controller.recordActivity();
      await rig.controller.check();
      expect(rig.controller.status, AppLockStatus.locked);
    });
  });
}
