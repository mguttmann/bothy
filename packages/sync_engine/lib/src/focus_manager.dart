// SPDX-License-Identifier: Apache-2.0

/// The focus of the gateway connection (04 6.2 `focus`, 04 11.3 item 2; brief M4-13 Design
/// note 6).
library;

import 'dart:async';

import 'package:api_client/api_client.dart';
import 'package:built_collection/built_collection.dart';
import 'package:domain/domain.dart';

import 'frames.dart';

/// Reports the visible threads to the gateway.
///
/// Each change sends one `focus` frame with the complete set and `idle`; changes within one
/// microtask are coalesced. At most `sync.max_focused_threads` IDs are sent: the most recently
/// shown win (a thread counts as shown when it first appears in a call; IDs new in one call
/// count in the iteration order of the set), and the rest stays unfocused, which
/// [setVisible] returns. After `ready` the current set is sent again (a new gateway session
/// has no focus); after `resumed` only a change made while disconnected is sent, because the
/// server restores the focus on `resume` (04 6.3 item 7). An `error` with `validation_failed`
/// for `focus` keeps the previous set, as the server does.
///
/// Example:
/// ```dart
/// final unfocused = connection.focus.setVisible({threadA, threadB}, idle: false);
/// ```
final class FocusManager {
  /// Creates the manager; [limit] is `sync.max_focused_threads`, [trySend] sends a frame on
  /// the identified connection and returns `false` without one.
  FocusManager({required this.limit, required this._trySend});

  /// `sync.max_focused_threads`.
  final int limit;

  final bool Function(GatewayFrame) _trySend;

  /// Visible threads, the oldest shown first.
  final List<ThreadId> _shown = [];
  bool _idle = false;

  /// The set and idle flag the server holds, as far as the client knows.
  Set<ThreadId> _sent = const {};
  bool _sentIdle = false;
  Set<ThreadId> _previousSent = const {};
  bool _previousSentIdle = false;
  bool _flushScheduled = false;

  /// The focused set the server holds, as far as the client knows.
  Set<ThreadId> get current => Set.unmodifiable(_sent);

  /// Sets the visible threads [ids] and whether the person is [idle]; returns the IDs that
  /// stay unfocused because of the limit.
  Set<ThreadId> setVisible(Set<ThreadId> ids, {required bool idle}) {
    _shown.removeWhere((id) => !ids.contains(id));
    for (final id in ids) {
      if (!_shown.contains(id)) _shown.add(id);
    }
    _idle = idle;
    if (!_flushScheduled) {
      _flushScheduled = true;
      scheduleMicrotask(() {
        _flushScheduled = false;
        _flush(force: false);
      });
    }
    final cut = _shown.length - limit;
    return cut <= 0 ? <ThreadId>{} : _shown.take(cut).toSet();
  }

  /// A new gateway session is ready: sends the current set when it is not empty.
  void onReady() {
    _sent = const {};
    _sentIdle = false;
    _flush(force: _focused.isNotEmpty);
  }

  /// The gateway session resumed: sends only a change made while disconnected.
  void onResumed() => _flush(force: false);

  /// The server answered the last `focus` with `validation_failed`: the previous set stays.
  void onRejected() {
    _sent = _previousSent;
    _sentIdle = _previousSentIdle;
  }

  List<ThreadId> get _focused =>
      _shown.length <= limit ? _shown : _shown.sublist(_shown.length - limit);

  void _flush({required bool force}) {
    final focused = _focused;
    final set = focused.toSet();
    final changed =
        set.length != _sent.length ||
        !set.containsAll(_sent) ||
        _idle != _sentIdle;
    if (!force && !changed) return;
    final frame = FocusFrame(
      GatewayFocus(
        (b) => b
          ..threadIds = ListBuilder<String>([
            for (final id in focused) id.value,
          ])
          ..idle = _idle,
      ),
    );
    if (!_trySend(frame)) return;
    _previousSent = _sent;
    _previousSentIdle = _sentIdle;
    _sent = set;
    _sentIdle = _idle;
  }
}
