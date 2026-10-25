// SPDX-License-Identifier: Apache-2.0

/// When the app lock of the phone applies and when it locks (docs/spec/08-identity.md 9;
/// docs/spec/01-product.md 5.6 E6; brief M5-08 Design note 2; docs/decisions.d/M5-08.md
/// D-M5-08-2 and D-M5-08-8). Pure functions over the policy, the person's switch and the times
/// of two injected clocks, tested without a device.
library;

import 'package:domain/domain.dart' show AppLockPolicy;

/// Whether the lock applies: the person turned it on on this device ([enabled]) or the owner
/// requires it (`security.app_lock_required` in [policy]).
///
/// Example: `lockApplies(enabled: false, policy: required)` is true.
bool lockApplies({required bool enabled, required AppLockPolicy? policy}) =>
    enabled || (policy?.required ?? false);

/// The time without use after which the app locks: `client.app_lock_after` of [policy].
///
/// Without a delivered policy (an instance row of an older server, before the first
/// `getInstance`) the app is strict and the time is zero: every return from the background
/// locks, because a time of its own would be a limit that no configuration key holds.
Duration lockAfter(AppLockPolicy? policy) => policy?.after ?? Duration.zero;

/// A point in time on both clocks of the lock.
///
/// [wall] is the device's clock: it keeps counting while the device sleeps, but the person can
/// set it back in the system settings without the device code. [monotonic] is the time of the
/// app process on a clock that only moves forward and cannot be set; it may stand still while
/// the device sleeps. The lock counts the larger of both, so neither a clock set back nor a
/// sleeping device shortens the time (D-M5-08-8).
final class LockInstant {
  /// Creates an instant.
  const LockInstant(this.wall, this.monotonic);

  /// The wall clock.
  final DateTime wall;

  /// The monotonic time since the app process started.
  final Duration monotonic;

  /// Whether this instant is not before [other] on the monotonic clock, which no setting of
  /// the device's clock can change.
  bool notBefore(LockInstant other) => monotonic >= other.monotonic;
}

/// The time between [since] and [now] that the lock counts: the larger of both clocks, or
/// null when the wall clock was set back before [since], which counts as reached.
Duration? _idle(LockInstant since, LockInstant now) {
  if (now.wall.isBefore(since.wall)) return null;
  final wall = now.wall.difference(since.wall);
  final monotonic = now.monotonic - since.monotonic;
  return wall > monotonic ? wall : monotonic;
}

/// Whether the time since [lastActivity] reached [after] at [now].
///
/// [lastActivity] is the last pointer or key event of the person (or the unlock); an incoming
/// message or a running upload is not activity. The same rule holds in the foreground and
/// since the app went to the background: the background does not reset the time. A wall clock
/// set back before [lastActivity] counts as reached, so changing the time never keeps the lock
/// open.
bool idleReached({
  required LockInstant lastActivity,
  required LockInstant now,
  required Duration after,
}) {
  final idle = _idle(lastActivity, now);
  return idle == null || idle >= after;
}

/// The time left until [idleReached] holds, never negative.
Duration idleLeft({
  required LockInstant lastActivity,
  required LockInstant now,
  required Duration after,
}) {
  final idle = _idle(lastActivity, now);
  if (idle == null) return Duration.zero;
  final left = after - idle;
  return left.isNegative ? Duration.zero : left;
}
