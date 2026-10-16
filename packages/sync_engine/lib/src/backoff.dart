// SPDX-License-Identifier: Apache-2.0

/// Full-jitter backoff of 04 6.5.
library;

import 'ports.dart';

/// The wait before reconnect attempt `n`: uniform in `[0, min(cap, base * 2^n)]` (04 6.5),
/// with `base` from `sync.reconnect_backoff_base` and `cap` from `sync.reconnect_backoff_cap`
/// (D-W-115).
///
/// [next] draws a wait and raises `n`; [reset] after `ready` or `resumed` starts at 0 again.
/// An immediate attempt on a network change or the foreground does not call [next], so it
/// does not raise `n`.
///
/// Example:
/// ```dart
/// final backoff = ReconnectBackoff(
///   base: config.reconnectBackoffBase,
///   cap: config.reconnectBackoffCap,
///   random: SystemRandomSource(),
/// );
/// await clock.delay(backoff.next());
/// ```
final class ReconnectBackoff {
  /// Creates the backoff with [base], [cap] and the [random] source of the jitter.
  ReconnectBackoff({
    required this.base,
    required this.cap,
    required this._random,
  });

  /// `sync.reconnect_backoff_base`.
  final Duration base;

  /// `sync.reconnect_backoff_cap`.
  final Duration cap;

  final RandomSource _random;

  int _attempt = 0;

  /// The number `n` of the next attempt.
  int get attempt => _attempt;

  /// The upper bound `min(cap, base * 2^n)` for attempt [n].
  Duration bound(int n) {
    var bound = base;
    for (var i = 0; i < n && bound < cap; i++) {
      bound *= 2;
    }
    return bound > cap ? cap : bound;
  }

  /// Draws the wait before the next attempt and raises `n`.
  Duration next() {
    final b = bound(_attempt);
    _attempt++;
    return b * _random.nextDouble();
  }

  /// Starts at attempt 0 again (after `ready` or `resumed`).
  void reset() => _attempt = 0;
}
