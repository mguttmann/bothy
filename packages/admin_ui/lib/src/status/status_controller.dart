// SPDX-License-Identifier: Apache-2.0

/// The state of one status view over a [StatusSource] (brief M7-21 Design note 1 and
/// requirement 14; docs/decisions.d/M7-21.md D-M7-21-2).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as api;
import 'package:flutter/foundation.dart';

import '../data/self_repository.dart' show SelfActionFailed;
import 'status_source.dart';

/// Loads the page of [source], loads it again on every change, and decides whether the view
/// may show it.
///
/// The unreachable rule (16 12 "Limit"): the status page runs in the server, so a view that
/// cannot reach the server must not keep showing the last page as if it were current. The
/// view is [unreachable] while the source's connection state reports the server unreachable
/// or the last read got no answer at all; it then hides the page. A read that the server
/// answered with a problem is no outage: [problem] holds it. When the connection is back the
/// page is read again before it shows.
///
/// Example:
/// ```dart
/// final c = StatusController(source)..start();
/// // later
/// c.dispose();
/// ```
class StatusController extends ChangeNotifier {
  /// Creates the controller over [source]; call [start] once.
  StatusController(this.source);

  /// The source of the page.
  final StatusSource source;

  api.StatusPage? _page;
  api.ApiProblem? _problem;
  bool _loading = true;
  bool _noAnswer = false;
  bool _connectionLost = false;
  bool _disposed = false;
  int _generation = 0;
  Future<void>? _running;
  bool _again = false;
  final List<StreamSubscription<Object?>> _subs = [];

  /// The last page the server answered, or null before the first one.
  api.StatusPage? get page => _page;

  /// The problem of the last read the server refused, or null.
  api.ApiProblem? get problem => _problem;

  /// Whether the first read still runs.
  bool get loading => _loading;

  /// Whether the view shows "Server unreachable" instead of the page: the connection is lost,
  /// or the last read got no answer, or no read answered since the connection came back.
  bool get unreachable => _connectionLost || _noAnswer;

  /// Subscribes to the changes and the connection state and reads the page.
  void start() {
    _subs
      ..add(source.changes.listen((_) => unawaited(refresh())))
      ..add(
        source.unreachable.listen((lost) {
          if (lost == _connectionLost) return;
          _connectionLost = lost;
          if (lost) {
            // The page is no longer known to be current; only a new answer clears this.
            _noAnswer = true;
            _notify();
          } else {
            // Back: read the page before it shows again.
            unawaited(refresh());
          }
        }),
      );
    unawaited(refresh());
  }

  /// Reads the page; a read requested while one runs runs once after it, so the last change
  /// always reaches the view.
  Future<void> refresh() {
    if (_running != null) {
      _again = true;
      return _running!;
    }
    final run = _read();
    _running = run;
    return run;
  }

  Future<void> _read() async {
    try {
      do {
        _again = false;
        final generation = ++_generation;
        try {
          final page = await source.load();
          if (_disposed || generation != _generation) return;
          _page = page;
          _problem = null;
          _noAnswer = false;
        } on SelfActionFailed catch (e) {
          if (_disposed || generation != _generation) return;
          final problem = e.problem;
          if (problem == null) {
            _noAnswer = true;
          } else {
            _noAnswer = false;
            _problem = problem;
            _page = null;
          }
        }
        _loading = false;
        _notify();
      } while (_again && !_disposed);
    } finally {
      _running = null;
    }
  }

  void _notify() {
    if (!_disposed) notifyListeners();
  }

  @override
  void dispose() {
    _disposed = true;
    for (final s in _subs) {
      unawaited(s.cancel());
    }
    super.dispose();
  }
}
