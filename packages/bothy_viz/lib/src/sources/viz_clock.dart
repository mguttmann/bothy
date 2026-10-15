// SPDX-License-Identifier: Apache-2.0

/// The clock of the sources: real time in the apps, a fake clock in tests and the harness.
library;

import 'dart:async';

/// A timer of a [VizClock]; [cancel] stops it before it fires.
abstract interface class VizTimer {
  /// Stops the timer; a fired or cancelled timer ignores the call.
  void cancel();
}

/// Time and one-shot timers, injected into every source so that the stale rule behaves the
/// same in every implementation and in tests (docs/spec/25-visualisierung.md 5.1).
abstract interface class VizClock {
  /// The current time.
  DateTime now();

  /// Calls [callback] once after [delay].
  VizTimer schedule(Duration delay, void Function() callback);
}

/// The clock of the apps: `DateTime.now` and `dart:async` timers.
final class SystemVizClock implements VizClock {
  /// Creates the clock.
  const SystemVizClock();

  @override
  DateTime now() => DateTime.now();

  @override
  VizTimer schedule(Duration delay, void Function() callback) =>
      _SystemTimer(Timer(delay, callback));
}

final class _SystemTimer implements VizTimer {
  _SystemTimer(this._timer);

  final Timer _timer;

  @override
  void cancel() => _timer.cancel();
}

/// A clock that moves only through [advance]: tests and the time-lapse of the harness.
///
/// ```dart
/// final clock = FakeVizClock(DateTime.utc(2026, 10, 15, 10));
/// clock.schedule(const Duration(seconds: 5), () => print('stale'));
/// clock.advance(const Duration(seconds: 5)); // prints stale
/// ```
final class FakeVizClock implements VizClock {
  /// Creates the clock at [start].
  FakeVizClock(DateTime start) : _now = start;

  DateTime _now;
  final List<_FakeTimer> _timers = [];
  int _sequence = 0;

  @override
  DateTime now() => _now;

  @override
  VizTimer schedule(Duration delay, void Function() callback) {
    final timer = _FakeTimer(_now.add(delay), _sequence++, callback, this);
    _timers.add(timer);
    return timer;
  }

  /// Moves the time forward by [by] and fires every timer due on the way, in time order (the
  /// order of scheduling for equal times); a timer scheduled by a callback fires too when it
  /// falls inside the step. Throws an [ArgumentError] for a negative [by].
  void advance(Duration by) {
    if (by.isNegative) {
      throw ArgumentError.value(by, 'by', 'must not be negative');
    }
    final end = _now.add(by);
    while (true) {
      final due = _timers.where((t) => !t.at.isAfter(end)).toList()
        ..sort((a, b) {
          final byTime = a.at.compareTo(b.at);
          return byTime != 0 ? byTime : a.sequence.compareTo(b.sequence);
        });
      if (due.isEmpty) break;
      final next = due.first;
      _timers.remove(next);
      if (next.at.isAfter(_now)) _now = next.at;
      next.callback();
    }
    _now = end;
  }

  /// Number of timers that have not fired and are not cancelled.
  int get pendingTimers => _timers.length;
}

final class _FakeTimer implements VizTimer {
  _FakeTimer(this.at, this.sequence, this.callback, this._clock);

  final DateTime at;
  final int sequence;
  final void Function() callback;
  final FakeVizClock _clock;

  @override
  void cancel() => _clock._timers.remove(this);
}
