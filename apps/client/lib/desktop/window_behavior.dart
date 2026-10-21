// SPDX-License-Identifier: Apache-2.0

/// The desktop window in the notification area (brief M4-30; docs/spec/15-client-architektur.md
/// 10 and 10.3): closing hides the window on Windows while the app keeps its gateway
/// connection, Quit ends the connection and the app, and the window state decides `idle`.
library;

import 'dart:async';

import 'package:window_manager/window_manager.dart';

/// What the window reports (the events of `window_manager`).
enum DesktopWindowEvent {
  /// The person asked to close the window (only reported while closing is prevented).
  close,

  /// The window was hidden.
  hide,

  /// The window was shown.
  show,

  /// The window was minimized.
  minimize,

  /// The window came back from minimized.
  restore,

  /// The window became the foreground window of the system.
  focus,

  /// Another window became the foreground window.
  blur,
}

/// The window of the app, the port [WindowBehavior] drives.
abstract interface class DesktopWindow {
  /// Prepares the window; with [preventClose] a close only reports [DesktopWindowEvent.close].
  Future<void> start({required bool preventClose});

  /// The events of the window.
  Stream<DesktopWindowEvent> get events;

  /// Hides the window; the app keeps running.
  Future<void> hide();

  /// Shows the window and brings it to the front.
  Future<void> show();

  /// Ends the window and with it the app.
  Future<void> destroy();
}

/// A window without a platform behind it: tests and hosts without `window_manager`.
final class NoDesktopWindow implements DesktopWindow {
  /// Creates the window.
  const NoDesktopWindow();

  @override
  Future<void> start({required bool preventClose}) async {}

  @override
  Stream<DesktopWindowEvent> get events => const Stream.empty();

  @override
  Future<void> hide() async {}

  @override
  Future<void> show() async {}

  @override
  Future<void> destroy() async {}
}

/// [DesktopWindow] over `window_manager` (Herausgeber leanflutter.dev, 15 10.3).
///
/// Example: `final window = WindowManagerWindow(); await window.start(preventClose: true);`
final class WindowManagerWindow with WindowListener implements DesktopWindow {
  /// Creates the window on [manager] (the plugin's instance by default).
  WindowManagerWindow({WindowManager? manager})
    : _manager = manager ?? windowManager;

  final WindowManager _manager;
  final StreamController<DesktopWindowEvent> _events =
      StreamController<DesktopWindowEvent>.broadcast();

  @override
  Future<void> start({required bool preventClose}) async {
    await _manager.ensureInitialized();
    _manager.addListener(this);
    await _manager.setPreventClose(preventClose);
  }

  @override
  Stream<DesktopWindowEvent> get events => _events.stream;

  @override
  Future<void> hide() => _manager.hide();

  @override
  Future<void> show() async {
    if (await _manager.isMinimized()) await _manager.restore();
    await _manager.show();
    await _manager.focus();
  }

  @override
  Future<void> destroy() async {
    _manager.removeListener(this);
    await _manager.destroy();
  }

  void _add(DesktopWindowEvent e) {
    if (!_events.isClosed) _events.add(e);
  }

  @override
  void onWindowClose() => _add(DesktopWindowEvent.close);

  @override
  void onWindowFocus() => _add(DesktopWindowEvent.focus);

  @override
  void onWindowBlur() => _add(DesktopWindowEvent.blur);

  @override
  void onWindowMinimize() => _add(DesktopWindowEvent.minimize);

  @override
  void onWindowRestore() => _add(DesktopWindowEvent.restore);

  @override
  void onWindowEvent(String eventName) {
    switch (eventName) {
      case 'hide':
        _add(DesktopWindowEvent.hide);
      case 'show':
        _add(DesktopWindowEvent.show);
    }
  }
}

/// Close, Open and Quit of the desktop window, and whether the window is active.
///
/// With [hideOnClose] (Windows, 15 10.3) a close hides the window and nothing else happens:
/// the sync engine stays connected in the notification area. Without it (macOS) a close is
/// left to the system. [quit] runs [shutdown] (the engine and the database close, so the
/// device has no gateway connection any more and the server creates no notification for it,
/// 13 4.6 item 3), then ends the window.
///
/// [active] is the window part of `idle` (D-M4-30-3): the window is active while it is shown,
/// not minimized and the foreground window of the system; hidden to the notification area,
/// minimized or behind another window it is not, and the app reports `idle: true`.
///
/// Example:
/// ```dart
/// final behavior = WindowBehavior(window: window, hideOnClose: true, shutdown: services.close);
/// await behavior.start();
/// behavior.activeChanges.listen((active) => focus.setVisible(threads, idle: !active));
/// ```
final class WindowBehavior {
  /// Creates the behavior of [window].
  WindowBehavior({
    required this.window,
    required this.hideOnClose,
    required this._shutdown,
  });

  /// The window.
  final DesktopWindow window;

  /// Whether a close hides the window instead of ending the app.
  final bool hideOnClose;

  final Future<void> Function() _shutdown;
  final StreamController<bool> _changes = StreamController<bool>.broadcast();
  StreamSubscription<DesktopWindowEvent>? _sub;
  bool _visible = true;
  bool _minimized = false;
  bool _focused = true;
  Future<void>? _quit;

  /// Whether the window is shown, not minimized and in the foreground.
  bool get active => _visible && !_minimized && _focused;

  /// Every change of [active].
  Stream<bool> get activeChanges => _changes.stream;

  /// Starts listening to the window and prevents the close where it hides.
  Future<void> start() async {
    _sub ??= window.events.listen(_on);
    await window.start(preventClose: hideOnClose);
  }

  void _on(DesktopWindowEvent e) {
    final before = active;
    switch (e) {
      case DesktopWindowEvent.close:
        if (hideOnClose && _quit == null) unawaited(window.hide());
      case DesktopWindowEvent.hide:
        _visible = false;
      case DesktopWindowEvent.show:
        _visible = true;
      case DesktopWindowEvent.minimize:
        _minimized = true;
      case DesktopWindowEvent.restore:
        _minimized = false;
      case DesktopWindowEvent.focus:
        // A window that receives the focus is shown and not minimized.
        _visible = true;
        _minimized = false;
        _focused = true;
      case DesktopWindowEvent.blur:
        _focused = false;
    }
    if (active != before && !_changes.isClosed) _changes.add(active);
  }

  /// Shows the window from the notification area and brings it to the front.
  Future<void> open() => window.show();

  /// Ends the connection and the app; a second call waits for the first.
  Future<void> quit() => _quit ??= () async {
    try {
      await _shutdown();
    } finally {
      await window.destroy();
    }
  }();

  /// Stops listening.
  Future<void> dispose() async {
    await _sub?.cancel();
    await _changes.close();
  }
}
