// SPDX-License-Identifier: Apache-2.0

/// The icon of the app in the Windows notification area with the menu Open and Quit (brief
/// M4-30; docs/spec/15-client-architektur.md 10.3).
library;

import 'dart:async';

import 'package:tray_manager/tray_manager.dart';

import 'window_behavior.dart';

/// The two entries of the tray menu.
enum TrayAction {
  /// Show the window.
  open,

  /// End the connection and the app.
  quit,
}

/// The tray icon of the platform, the port [DesktopTray] drives.
abstract interface class TrayPort {
  /// Shows the icon [iconAsset] (a Flutter asset path) with [tooltip] and the menu entries
  /// [openLabel] and [quitLabel].
  Future<void> show({
    required String iconAsset,
    required String tooltip,
    required String openLabel,
    required String quitLabel,
  });

  /// The chosen entries; a click on the icon itself counts as [TrayAction.open].
  Stream<TrayAction> get actions;

  /// Removes the icon.
  Future<void> dispose();
}

/// A tray without a platform behind it: macOS (D-M4-30-5) and tests.
final class NoTray implements TrayPort {
  /// Creates the tray.
  const NoTray();

  @override
  Future<void> show({
    required String iconAsset,
    required String tooltip,
    required String openLabel,
    required String quitLabel,
  }) async {}

  @override
  Stream<TrayAction> get actions => const Stream.empty();

  @override
  Future<void> dispose() async {}
}

/// [TrayPort] over `tray_manager` (Herausgeber leanflutter.dev, 15 10.3).
///
/// Example: `await TrayManagerTray().show(iconAsset: trayIconAsset, ...);`
final class TrayManagerTray with TrayListener implements TrayPort {
  /// Creates the tray on [manager] (the plugin's instance by default).
  TrayManagerTray({TrayManager? manager}) : _manager = manager ?? trayManager;

  final TrayManager _manager;
  final StreamController<TrayAction> _actions =
      StreamController<TrayAction>.broadcast();

  static const String _open = 'open';
  static const String _quit = 'quit';

  @override
  Future<void> show({
    required String iconAsset,
    required String tooltip,
    required String openLabel,
    required String quitLabel,
  }) async {
    _manager.addListener(this);
    await _manager.setIcon(iconAsset);
    await _manager.setToolTip(tooltip);
    await _manager.setContextMenu(
      Menu(
        items: [
          MenuItem(key: _open, label: openLabel),
          MenuItem.separator(),
          MenuItem(key: _quit, label: quitLabel),
        ],
      ),
    );
  }

  @override
  Stream<TrayAction> get actions => _actions.stream;

  void _add(TrayAction a) {
    if (!_actions.isClosed) _actions.add(a);
  }

  @override
  void onTrayIconMouseDown() => _add(TrayAction.open);

  @override
  void onTrayIconRightMouseDown() => unawaited(_manager.popUpContextMenu());

  @override
  void onTrayMenuItemClick(MenuItem menuItem) {
    switch (menuItem.key) {
      case _open:
        _add(TrayAction.open);
      case _quit:
        _add(TrayAction.quit);
    }
  }

  @override
  Future<void> dispose() async {
    _manager.removeListener(this);
    await _manager.destroy();
    await _actions.close();
  }
}

/// The Flutter asset of the tray icon: the icon of the Windows runner (`Runner.rc`), so the
/// notification area shows the same icon as the taskbar.
const String trayIconAsset = 'windows/runner/resources/app_icon.ico';

/// Connects the tray menu to the window: Open shows it, Quit ends the app.
///
/// Example:
/// ```dart
/// final tray = DesktopTray(port: TrayManagerTray(), behavior: behavior);
/// await tray.start(tooltip: 'ExampleChat', openLabel: l.trayOpen, quitLabel: l.trayQuit);
/// ```
final class DesktopTray {
  /// Creates the tray of [behavior] on [port].
  DesktopTray({required this.port, required this.behavior});

  /// The platform tray.
  final TrayPort port;

  /// The window it drives.
  final WindowBehavior behavior;

  StreamSubscription<TrayAction>? _sub;

  /// Shows the icon with [tooltip] and the menu [openLabel] and [quitLabel].
  Future<void> start({
    required String tooltip,
    required String openLabel,
    required String quitLabel,
  }) async {
    _sub ??= port.actions.listen((a) {
      switch (a) {
        case TrayAction.open:
          unawaited(behavior.open());
        case TrayAction.quit:
          unawaited(behavior.quit());
      }
    });
    await port.show(
      iconAsset: trayIconAsset,
      tooltip: tooltip,
      openLabel: openLabel,
      quitLabel: quitLabel,
    );
  }

  /// Removes the icon and stops listening.
  Future<void> dispose() async {
    await _sub?.cancel();
    await port.dispose();
  }
}
