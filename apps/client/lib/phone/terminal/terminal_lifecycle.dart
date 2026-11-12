// SPDX-License-Identifier: Apache-2.0

/// Detaching and reattaching the phone terminal with the app's life cycle and the app lock
/// (docs/spec/01-product.md 5.3 M8, 5.6 E6, R-PR-12; docs/spec/15-client-architecture.md 23.2
/// item 7; brief M5-12 Design note 5; docs/decisions.d/M5-12.md D-M5-12-4;
/// docs/decisions.d/AUD-client-phone-01.md).
///
/// A session of the phone is live only while M8 of its host shows, the app is in front and the
/// lock is open:
///
/// | Event | What happens |
/// |---|---|
/// | the app moves to the background (`hidden`, `paused`) | every live session of a shown M8 detaches (`1000`), it keeps running on the host |
/// | the app lock closes (`AppLockEvent.locked`) | the same |
/// | an opening or an attach lands while the app is in the background or the lock holds the content | the session detaches at once, as above |
/// | the app is in front again, the lock is open and M8 of the host shows | the sessions detached here attach again (`attachTerminalSession`) |
/// | M8 goes while the lock holds the content (E6 replaced it) | its sessions stay remembered and attach when M8 shows again |
/// | M8 goes any other way (back, a tapped push, a link, a share) | its sessions are forgotten: they stay detached until the person attaches them |
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
import '../../terminal/reconnect.dart' show TerminalReconnectLock;

/// What the terminal needs to know about the app lock of M5-08.
abstract interface class TerminalLockGate implements TerminalReconnectLock {
  /// Whether the lock hides the content now or is still deciding on a return; no session
  /// attaches meanwhile.
  @override
  bool get holdsContent;

  /// The `locked` and `unlocked` events of the lock.
  Stream<AppLockEvent> get events;
}

/// [TerminalLockGate] over the app lock controller.
final class TerminalAppLockGate implements TerminalLockGate {
  /// Creates the gate over [lock].
  TerminalAppLockGate(this._lock);

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

/// Detaches every live session of the tabs whose M8 shows when the app leaves the foreground or
/// the lock closes, and attaches the same sessions again once the app is in front, the lock is
/// open and their M8 shows.
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
  // The tabs whose M8 shows now.
  final Set<TerminalTabs> _tabs = {};
  // The sessions this coordinator detached, each with the tabs of its host.
  final Map<TerminalController, TerminalTabs> _detached = {};
  bool _disposed = false;

  /// M8 of [tabs] shows: its sessions follow the life cycle, and the ones this coordinator
  /// detached before attach again. Registering twice changes nothing.
  void register(TerminalTabs tabs) {
    if (_disposed || !_tabs.add(tabs)) return;
    tabs.addListener(_onTabs);
    // After the build that registers: an attach notifies the listeners of its tab at once.
    scheduleMicrotask(_evaluate);
  }

  /// M8 of [tabs] no longer shows. While the lock holds the content the screen only went
  /// behind E6, so its sessions stay remembered for its return; in every other case the person
  /// left it (back, a tapped push, a link, a share), and its sessions are forgotten: nothing
  /// attaches without the view (D-M5-12-5).
  void unregister(TerminalTabs tabs) {
    if (!_tabs.remove(tabs)) return;
    tabs.removeListener(_onTabs);
    if (_lock?.holdsContent ?? false) return;
    _detached.removeWhere((_, t) => identical(t, tabs));
  }

  /// The sessions this coordinator detached and will attach again.
  @visibleForTesting
  Set<TerminalController> get detached => Set.unmodifiable(_detached.keys);

  // Whether no session may be live now.
  bool get _held =>
      _lifecycle.current != AppActivity.foreground ||
      (_lock?.holdsContent ?? false);

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

  // An opening or an attach that was on its way when the app left the foreground or the lock
  // closed lands later: the session it made live detaches at once and attaches on the return.
  void _onTabs() {
    if (_held) _detachAll();
  }

  void _detachAll() {
    for (final tabs in _tabs) {
      for (final c in tabs.tabs) {
        final phase = c.state.phase;
        if (phase == TerminalPhase.connected || phase == TerminalPhase.exited) {
          _detached[c] = tabs;
          unawaited(c.detach());
        }
      }
    }
  }

  void _evaluate() {
    if (_disposed || _detached.isEmpty || _held) return;
    for (final MapEntry(key: c, value: tabs) in _detached.entries.toList()) {
      // Only while M8 of the host shows; until then the session stays remembered.
      if (!_tabs.contains(tabs)) continue;
      _detached.remove(c);
      // Only a session that is still detached the way this coordinator left it: "End", a
      // revocation or another device changed it meanwhile.
      if (!tabs.tabs.contains(c) ||
          c.state.phase != TerminalPhase.detached ||
          c.state.detachCause != DetachCause.person) {
        continue;
      }
      unawaited(c.attach());
    }
  }

  /// Stops listening.
  void dispose() {
    _disposed = true;
    unawaited(_activity.cancel());
    unawaited(_lockEvents?.cancel());
    _lock?.removeListener(_evaluate);
    for (final tabs in _tabs) {
      tabs.removeListener(_onTabs);
    }
    _tabs.clear();
    _detached.clear();
  }
}
