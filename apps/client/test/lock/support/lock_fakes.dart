// SPDX-License-Identifier: Apache-2.0

// Fakes of the app lock tests (brief M5-08 Tests): the system authentication, the switch, the
// lifecycle and the cached policy, all driven by the test with a fake clock.

import 'dart:async';

import 'package:client/lock/app_lock_controller.dart';
import 'package:client/lock/app_lock_port.dart';
import 'package:domain/domain.dart' show AppLockPolicy;
import 'package:sync_engine/sync_engine.dart' show AppActivity, AppLifecycle;

import '../../support/fakes.dart';

/// The system authentication of a test: [credential] says whether the device has one, and
/// [results] are the answers of the next prompts (success when empty).
class FakeAppLockPort implements AppLockPort {
  /// Whether the device has a device credential.
  bool credential = true;

  /// The answers of the next prompts, first first.
  final List<AppLockAuthResult> results = [];

  /// The reasons of every prompt shown.
  final List<String> prompts = [];

  /// When set, a prompt waits for it before answering.
  Completer<void>? hold;

  @override
  Future<bool> hasDeviceCredential() async => credential;

  @override
  Future<AppLockAuthResult> authenticate(String reason) async {
    prompts.add(reason);
    await hold?.future;
    if (!credential) return AppLockAuthResult.noCredential;
    return results.isEmpty ? AppLockAuthResult.success : results.removeAt(0);
  }
}

/// The switch in memory.
class MemorySwitch implements AppLockSwitchStore {
  /// Creates the switch at [on].
  MemorySwitch([this.on = false]);

  /// The stored value.
  bool on;

  @override
  Future<bool> read() async => on;

  @override
  Future<void> write(bool value) async => on = value;
}

/// A lifecycle the test moves between foreground and background.
class ManualLifecycle implements AppLifecycle {
  final StreamController<AppActivity> _changes =
      StreamController<AppActivity>.broadcast(sync: true);
  AppActivity _current = AppActivity.foreground;

  @override
  Stream<AppActivity> get changes => _changes.stream;

  @override
  AppActivity get current => _current;

  /// Moves the app to [activity].
  void set(AppActivity activity) {
    _current = activity;
    _changes.add(activity);
  }
}

/// A monotonic clock the test moves.
class FakeMonotonic implements MonotonicClock {
  @override
  Duration elapsed = Duration.zero;
}

/// The start of the fake clock.
final DateTime lockStart = DateTime.utc(2026, 10, 25, 9);

/// A policy with [after] and [required].
AppLockPolicy policy({
  Duration after = const Duration(minutes: 5),
  bool required = false,
}) => AppLockPolicy(required: required, afterMs: after.inMilliseconds);

/// A controller with its fakes.
class LockRig {
  /// Builds the controller: a cold start when [lockAtStart], the switch at [on], the cached
  /// [initial] policy.
  LockRig({
    bool lockAtStart = true,
    bool on = true,
    AppLockPolicy? initial,
    bool credential = true,
  }) : store = MemorySwitch(on) {
    port.credential = credential;
    controller = AppLockController(
      port: port,
      clock: clock,
      lifecycle: lifecycle,
      store: store,
      policy: _policies.stream,
      lockAtStart: lockAtStart,
      monotonic: monotonic,
    );
    _policies.add(initial ?? policy());
  }

  /// The fake wall clock.
  final TestClock clock = TestClock(lockStart);

  /// The fake monotonic clock.
  final FakeMonotonic monotonic = FakeMonotonic();

  /// The system authentication.
  final FakeAppLockPort port = FakeAppLockPort();

  /// The lifecycle.
  final ManualLifecycle lifecycle = ManualLifecycle();

  /// The switch.
  final MemorySwitch store;

  final StreamController<AppLockPolicy?> _policies =
      StreamController<AppLockPolicy?>.broadcast();

  /// The controller.
  late final AppLockController controller;

  /// Delivers a new cached policy.
  void deliver(AppLockPolicy? next) => _policies.add(next);

  /// Lets [by] pass: both clocks move.
  void advance(Duration by) {
    clock.start = clock.start.add(by);
    monotonic.elapsed += by;
  }

  /// Sets the wall clock back by [by], as the person can in the system settings; the
  /// monotonic clock does not move.
  void setWallBack(Duration by) => clock.start = clock.start.subtract(by);

  /// Lets [by] pass on the wall clock only, as while the device sleeps.
  void sleep(Duration by) => clock.start = clock.start.add(by);

  /// Waits until the controller finished reading the switch and the policy.
  Future<void> settle() => controller.decided;

  bool _disposed = false;

  /// Stops the controller; a second call does nothing. A widget test calls it at the end of
  /// its body, so no timer of the controller outlives the test.
  Future<void> dispose() async {
    if (_disposed) return;
    _disposed = true;
    controller.dispose();
    await _policies.close();
  }
}
