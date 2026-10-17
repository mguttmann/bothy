// SPDX-License-Identifier: Apache-2.0

/// The foreground and background state of the app for the sync engine (docs/spec/
/// 15-client-architektur.md 7.2).
library;

import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:sync_engine/sync_engine.dart';

/// [AppLifecycle] over Flutter's [AppLifecycleListener].
///
/// `resumed` and `inactive` (a desktop window without focus is still visible) count as
/// foreground; `hidden`, `paused` and `detached` as background. The desktop engine keeps its
/// connection in the background anyway (`keepConnectedInBackground`).
final class FlutterAppLifecycle implements AppLifecycle {
  /// Creates the lifecycle and starts listening; [initial] is the state at start.
  FlutterAppLifecycle({AppLifecycleState? initial})
    : _current = _activity(
        initial ??
            WidgetsBinding.instance.lifecycleState ??
            AppLifecycleState.resumed,
      ) {
    _listener = AppLifecycleListener(onStateChange: onState);
  }

  late final AppLifecycleListener _listener;
  final StreamController<AppActivity> _changes =
      StreamController<AppActivity>.broadcast();
  AppActivity _current;

  @override
  Stream<AppActivity> get changes => _changes.stream;

  @override
  AppActivity get current => _current;

  /// Records the Flutter [state]; emits a change only when the activity changes.
  @visibleForTesting
  void onState(AppLifecycleState state) {
    final next = _activity(state);
    if (next == _current) return;
    _current = next;
    _changes.add(next);
  }

  static AppActivity _activity(AppLifecycleState state) => switch (state) {
    AppLifecycleState.resumed ||
    AppLifecycleState.inactive => AppActivity.foreground,
    _ => AppActivity.background,
  };

  /// Stops listening.
  Future<void> dispose() async {
    _listener.dispose();
    await _changes.close();
  }
}
