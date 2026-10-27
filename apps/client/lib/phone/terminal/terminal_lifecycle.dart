// SPDX-License-Identifier: Apache-2.0

/// Detaching and reattaching the phone terminal with the app's life cycle and the app lock
/// (docs/spec/01-product.md 5.3 M8, 5.6 E6, R-PR-12; docs/spec/15-client-architecture.md 23.2
/// item 7; brief M5-12 Design note 5; docs/decisions.d/M5-12.md D-M5-12-4).
///
/// | Event | What happens |
/// |---|---|
/// | the app moves to the background (`hidden`, `paused`) | every live session detaches (`1000`), it keeps running on the host |
/// | the app lock closes (`AppLockEvent.locked`) | the same |
/// | the app is in front again and the lock is open | the sessions detached here attach again (`attachTerminalSession`) |
///
/// The client never decides the reattach window: within `terminal.quick_reattach_window` the
/// server accepts the attach without a step-up, after it it answers `401 step_up_required` and
/// the controller runs the step-up through the device key with biometrics once and repeats
/// (`TerminalController.attach`). A failed attach shows its reason and "Reattach"; nothing
/// retries in a loop.
library;

import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:sync_engine/sync_engine.dart' show AppActivity, AppLifecycle;
import 'package:terminal_ui/terminal_ui.dart';

import '../../lock/app_lock_controller.dart';
import '../../terminal/providers.dart' show TerminalTabs;

/// What the terminal needs to know about the app lock of M5-08.
abstract interface class TerminalLockGate implements Listenable {
  /// Whether the lock hides the content now or is still deciding on a return; no session
  /// attaches meanwhile.
  bool get holdsContent;

  /// The `locked` and `unlocked` events of the lock.
  Stream<AppLockEvent> get events;
}

/// [TerminalLockGate] over the app lock controller.
final class AppLockGate implements TerminalLockGate {
  /// Creates the gate over [lock].
  AppLockGate(this._lock);

  final AppLockController _lock;

  @override
  bool get holdsContent => _lock.closed || _lock.deciding;

  @override
  Stream<AppLockEvent> get events => _lock.events;

  @override
  void addListener(VoidCallback listener) => _lock.addListener(listener);

  @override
  void removeListener(VoidCallback listener) => _lock.removeListener(listener);
}

/// Detaches every live session of the registered tabs when the app leaves the foreground or
/// the lock closes, and attaches the same sessions again once the app is in front and the lock
/// is open.
///
/// ```dart
/// final coordinator = PhoneTerminalCoordinator(lifecycle: ports.lifecycle, lock: gate);
/// coordinator.register(tabs);
/// ```
final class PhoneTerminalCoordinator {
  /// Creates the coordinator over [lifecycle] and [lock] (null: the app has no lock).
  PhoneTerminalCoordinator({
    required AppLifecycle lifecycle,
    TerminalLockGate? lock,
  }) : _lifecycle = lifecycle,
       _lock = lock {
    _activity = lifecycle.changes.listen(_onActivity);
    _lockEvents = lock?.events.listen(_onLock);
    lock?.addListener(_evaluate);
  }

  final AppLifecycle _lifecycle;
  final TerminalLockGate? _lock;
  late final StreamSubscription<AppActivity> _activity;
  StreamSubscription<AppLockEvent>? _lockEvents;
  final Set<TerminalTabs> _tabs = {};
  final Set<TerminalController> _detached = {};
  bool _disposed = false;

  /// The tabs whose sessions follow the life cycle; registering twice changes nothing.
  void register(TerminalTabs tabs) => _tabs.add(tabs);

  /// Forgets [tabs] (the login ended and disposed them).
  void unregister(TerminalTabs tabs) {
    _tabs.remove(tabs);
    _detached.removeWhere((c) => !_alive(c));
  }

  /// The sessions this coordinator detached and will attach again.
  @visibleForTesting
  Set<TerminalController> get detached => Set.unmodifiable(_detached);

  void _onActivity(AppActivity activity) {
    if (activity == AppActivity.background) {
      _detachAll();
      return;
    }
    // The app lock answers the same event in the same delivery: closing, or "deciding" while
    // it reads the boot clock. Deciding after that delivery sees its answer.
    scheduleMicrotask(_evaluate);
  }

  void _onLock(AppLockEvent event) {
    if (event == AppLockEvent.locked) {
      _detachAll();
    } else {
      _evaluate();
    }
  }

  void _detachAll() {
    for (final tabs in _tabs) {
      for (final c in tabs.tabs) {
        final phase = c.state.phase;
        if (phase == TerminalPhase.connected || phase == TerminalPhase.exited) {
          _detached.add(c);
          unawaited(c.detach());
        }
      }
    }
  }

  void _evaluate() {
    if (_disposed || _detached.isEmpty) return;
    if (_lifecycle.current != AppActivity.foreground) return;
    if (_lock?.holdsContent ?? false) return;
    final again = List.of(_detached);
    _detached.clear();
    for (final c in again) {
      // Only a session that is still detached the way this coordinator left it: "End", a
      // revocation or another device changed it meanwhile.
      if (!_alive(c) ||
          c.state.phase != TerminalPhase.detached ||
          c.state.detachCause != DetachCause.person) {
        continue;
      }
      unawaited(c.attach());
    }
  }

  bool _alive(TerminalController c) => _tabs.any((t) => t.tabs.contains(c));

  /// Stops listening.
  void dispose() {
    _disposed = true;
    unawaited(_activity.cancel());
    unawaited(_lockEvents?.cancel());
    _lock?.removeListener(_evaluate);
    _tabs.clear();
    _detached.clear();
  }
}
