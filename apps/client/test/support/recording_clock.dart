// SPDX-License-Identifier: Apache-2.0

// A fake clock of a recording (brief M5-07 Design note 1): the time moves only by [advance],
// which fires every timer that falls due in order, so the tests of the limit, the notice and
// the interval rule never wait for real time.

import 'dart:async';

import 'package:client/phone/compose/voice_recorder.dart';

/// A [RecordingClock] the test moves by hand.
final class FakeRecordingClock implements RecordingClock {
  /// Creates the clock at [start].
  FakeRecordingClock([DateTime? start])
    : _now = start ?? DateTime.utc(2026, 10, 25, 9);

  DateTime _now;
  final List<_FakeTimer> _timers = [];

  @override
  DateTime now() => _now;

  @override
  Timer after(Duration delay, void Function() callback) =>
      _add(_FakeTimer(this, _now.add(delay), null, callback));

  @override
  Timer every(Duration period, void Function() callback) =>
      _add(_FakeTimer(this, _now.add(period), period, callback));

  Timer _add(_FakeTimer t) {
    _timers.add(t);
    return t;
  }

  /// The timers that still wait.
  int get pending => _timers.where((t) => t.isActive).length;

  /// Moves the time by [by] and fires each timer that falls due, earliest first; a periodic
  /// timer fires once per period that passes.
  void advance(Duration by) {
    final end = _now.add(by);
    while (true) {
      _FakeTimer? next;
      for (final t in _timers) {
        if (!t.isActive || t.due.isAfter(end)) continue;
        if (next == null || t.due.isBefore(next.due)) next = t;
      }
      if (next == null) break;
      _now = next.due;
      next.fire();
    }
    _now = end;
    _timers.removeWhere((t) => !t.isActive);
  }
}

final class _FakeTimer implements Timer {
  _FakeTimer(this._clock, this.due, this._period, this._callback);

  final FakeRecordingClock _clock;
  DateTime due;
  final Duration? _period;
  final void Function() _callback;
  bool _active = true;
  int _ticks = 0;

  void fire() {
    _ticks++;
    final period = _period;
    if (period == null) {
      _active = false;
    } else {
      due = due.add(period);
    }
    _callback();
  }

  @override
  void cancel() => _active = false;

  @override
  bool get isActive => _active;

  @override
  int get tick => _ticks;

  @override
  String toString() => '_FakeTimer(due: $due, clock: ${_clock.now()})';
}
