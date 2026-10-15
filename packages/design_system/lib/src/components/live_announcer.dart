// SPDX-License-Identifier: Apache-2.0

/// What the live box announces to screen readers (components.md section 2.4).
library;

/// Decides which observations of a run become a screen reader announcement.
///
/// The live box calls [observe] on every rebuild with the text of the current state and the ID
/// of the open question. Only a change of the state text or a new question ID is a candidate;
/// patches to steps and numbers keep both the same and are never announced. The first
/// observation only sets the baseline.
///
/// Throttle and merge rule (docs/DECISIONS.md D-M4-07-4): a candidate is sent at once when the
/// last announcement is at least [minInterval] (`a11y.live_announce_min_interval`) ago;
/// otherwise it waits until [pendingDue], and a later candidate replaces the waiting one, so
/// two changes closer than the interval give one announcement with the later text. A
/// candidate equal to the last announced text drops the waiting one. [flush] sends a waiting
/// candidate once it is due.
///
/// Pure Dart: [clock] gives the time, [sink] receives the text.
///
/// ```dart
/// final a = LiveAnnouncer(minInterval: interval, clock: DateTime.now, sink: speak);
/// a.observe(state: 'Running', questionId: null);
/// ```
class LiveAnnouncer {
  /// Creates the announcer.
  LiveAnnouncer({
    required this.minInterval,
    required this.clock,
    required this.sink,
  });

  /// The shortest time between two announcements.
  final Duration minInterval;

  /// The current time.
  final DateTime Function() clock;

  /// Receives each announcement.
  final void Function(String message) sink;

  bool _seen = false;
  String? _state;
  String? _questionId;
  String? _lastSent;
  DateTime? _lastSentAt;
  String? _pending;

  /// When the waiting candidate may be sent, or null when none waits.
  DateTime? get pendingDue {
    if (_pending == null) return null;
    return _lastSentAt!.add(minInterval);
  }

  /// Records the current [state] text and [questionId]; [question] is the text announced for
  /// a new question.
  void observe({
    required String state,
    required String? questionId,
    String? question,
  }) {
    if (!_seen) {
      _seen = true;
      _state = state;
      _questionId = questionId;
      return;
    }
    String? candidate;
    if (state != _state) candidate = state;
    if (questionId != null && questionId != _questionId) {
      candidate = question ?? state;
    }
    _state = state;
    _questionId = questionId;
    if (candidate == null) return;
    if (candidate == _lastSent) {
      _pending = null;
      return;
    }
    final last = _lastSentAt;
    if (last == null || !clock().isBefore(last.add(minInterval))) {
      _send(candidate);
    } else {
      _pending = candidate;
    }
  }

  /// Sends the waiting candidate when it is due; does nothing otherwise.
  void flush() {
    final due = pendingDue;
    if (due == null || clock().isBefore(due)) return;
    _send(_pending!);
  }

  void _send(String message) {
    _pending = null;
    _lastSent = message;
    _lastSentAt = clock();
    sink(message);
  }
}
