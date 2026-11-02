// SPDX-License-Identifier: Apache-2.0

/// Attaching again after sleep or a network change (FIX-terminal-session-lifecycle,
/// docs/decisions.d/FIX-terminal-session-lifecycle.md D-TSL-5): when the app is online again or
/// comes back to the front, every tab whose stream dropped while it was attached attaches again,
/// once per such change. A tab the person detached stays detached; nothing retries in a loop.
///
/// | Event | What happens |
/// |---|---|
/// | the connection of the engine becomes live after it was not | `reconnectLost` of every tab group |
/// | the app comes to the front after it was in the background | the same |
/// | either while the app lock hides the content or decides | nothing yet; once when the lock opens |
library;

import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:sync_engine/sync_engine.dart' show AppActivity, AppLifecycle;

/// The app lock as the reconnector sees it: no session attaches while it hides the content or
/// decides (docs/spec/01-product.md 5.6 E6; docs/decisions.d/M5-12.md D-M5-12-4).
abstract interface class TerminalReconnectLock implements Listenable {
  /// Whether the lock hides the content now or is still deciding on a return.
  bool get holdsContent;
}

/// What the reconnector asks of a group of tabs ([TerminalTabs] of one host).
abstract interface class TerminalReconnectTarget {
  /// Attaches every tab whose stream dropped while it was attached.
  void reconnectLost();
}

/// Calls [TerminalReconnectTarget.reconnectLost] of the registered targets when [online] turns
/// true after it was false and when [lifecycle] turns to the foreground after the background.
///
/// ```dart
/// final r = TerminalReconnector(online: engineLive, lifecycle: ports.lifecycle);
/// r.register(tabs);
/// ```
final class TerminalReconnector {
  /// Creates the reconnector; [lifecycle] null watches only [online]; [lock] null: the app has
  /// no lock.
  TerminalReconnector({
    required Stream<bool> online,
    AppLifecycle? lifecycle,
    TerminalReconnectLock? lock,
  }) : _lock = lock {
    _online = online.listen(_onOnline);
    _activity = lifecycle?.changes.listen(_onActivity);
    lock?.addListener(_onLock);
  }

  final TerminalReconnectLock? _lock;
  bool _held = false;

  late final StreamSubscription<bool> _online;
  StreamSubscription<AppActivity>? _activity;
  final Set<TerminalReconnectTarget> _targets = {};
  bool? _wasOnline;
  bool _wasBackground = false;

  /// Adds [target]; registering twice changes nothing.
  void register(TerminalReconnectTarget target) => _targets.add(target);

  /// Removes [target].
  void unregister(TerminalReconnectTarget target) => _targets.remove(target);

  void _onOnline(bool online) {
    final back = online && _wasOnline == false;
    _wasOnline = online;
    if (back) _reconnect();
  }

  void _onActivity(AppActivity activity) {
    if (activity == AppActivity.background) {
      _wasBackground = true;
      return;
    }
    if (!_wasBackground) return;
    _wasBackground = false;
    _reconnect();
  }

  void _onLock() {
    if (!_held || (_lock?.holdsContent ?? false)) return;
    _held = false;
    _reconnect();
  }

  void _reconnect() {
    if (_lock?.holdsContent ?? false) {
      _held = true;
      return;
    }
    for (final t in List.of(_targets)) {
      t.reconnectLost();
    }
  }

  /// Stops listening.
  void dispose() {
    _lock?.removeListener(_onLock);
    unawaited(_online.cancel());
    unawaited(_activity?.cancel());
    _targets.clear();
  }
}
