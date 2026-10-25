// SPDX-License-Identifier: Apache-2.0

/// When the app lock of the phone applies and when it locks (docs/spec/08-identity.md 9;
/// docs/spec/01-product.md 5.6 E6; brief M5-08 Design note 2). Pure functions over the
/// policy, the person's switch and the times of an injected clock, tested without a device.
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

/// Whether the time since [lastActivity] reached [after] at [now].
///
/// [lastActivity] is the last pointer or key event of the person (or the unlock); an incoming
/// message or a running upload is not activity. The same rule holds in the foreground and
/// since the app went to the background: the background does not reset the time.
bool idleReached({
  required DateTime lastActivity,
  required DateTime now,
  required Duration after,
}) => !now.isBefore(lastActivity.add(after));

/// The time left until [idleReached] holds, never negative.
Duration idleLeft({
  required DateTime lastActivity,
  required DateTime now,
  required Duration after,
}) {
  final left = lastActivity.add(after).difference(now);
  return left.isNegative ? Duration.zero : left;
}
