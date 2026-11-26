// SPDX-License-Identifier: Apache-2.0

/// The icon of the app in the Windows notification area with the menu Open and Quit (brief
/// M4-30; docs/spec/15-client-architecture.md 10.3), and the entry "Stop computer control"
/// while a computer session of the owner is live, on Windows and macOS (brief CU-07; docs/spec/
/// 27-computer-use.md 11 item 4; ruling D-CU-07-4).
library;

import 'dart:async';

import 'package:tray_manager/tray_manager.dart';

/// The entries of the tray menu.
enum TrayAction {
  /// Show the window.
  open,

  /// End the connection and the app.
  quit,

  /// Stop the live computer sessions of the owner (27 8.3 `stopped_by_user`).
  stopComputer,
}

/// The window the tray drives: `WindowBehavior` of the app.
abstract interface class TrayWindow {
  /// Shows the window.
  Future<void> open();

  /// Ends the connection and the app.
  Future<void> quit();
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

  /// Adds the entry [label] ([TrayAction.stopComputer]) to the menu, or removes it with null.
  Future<void> setComputerEntry(String? label);

  /// Removes the icon until the next [show]; the port stays usable.
  Future<void> hide();

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
  Future<void> setComputerEntry(String? label) async {}

  @override
  Future<void> hide() async {}

  @override
  Stream<TrayAction> get actions => const Stream.empty();

  @override
  Future<void> dispose() async {}
}

/// [TrayPort] over `tray_manager` (publisher leanflutter.dev, 15 10.3).
///
/// Example: `await TrayManagerTray().show(iconAsset: trayIconAsset, ...);`
final class TrayManagerTray with TrayListener implements TrayPort {
  /// Creates the tray on [manager] (the plugin's instance by default).
  TrayManagerTray({TrayManager? manager}) : _manager = manager ?? trayManager;

  final TrayManager _manager;
  final StreamController<TrayAction> _actions =
      StreamController<TrayAction>.broadcast();

  bool _listening = false;

  /// The labels of the last [show] and the computer entry, so either change rebuilds the menu.
  String _openLabel = '';
  String _quitLabel = '';
  String? _computerLabel;

  static const String _open = 'open';
  static const String _quit = 'quit';
  static const String _stopComputer = 'stop_computer';

  @override
  Future<void> show({
    required String iconAsset,
    required String tooltip,
    required String openLabel,
    required String quitLabel,
  }) async {
    // A second call only changes the texts (the language changed); one listener stays.
    if (!_listening) {
      _manager.addListener(this);
      _listening = true;
    }
    _openLabel = openLabel;
    _quitLabel = quitLabel;
    await _manager.setIcon(iconAsset);
    await _manager.setToolTip(tooltip);
    await _menu();
  }

  @override
  Future<void> setComputerEntry(String? label) async {
    _computerLabel = label;
    await _menu();
  }

  @override
  Future<void> hide() => _manager.destroy();

  Future<void> _menu() => _manager.setContextMenu(
    Menu(
      items: [
        if (_computerLabel case final stop?) ...[
          MenuItem(key: _stopComputer, label: stop),
          MenuItem.separator(),
        ],
        MenuItem(key: _open, label: _openLabel),
        MenuItem.separator(),
        MenuItem(key: _quit, label: _quitLabel),
      ],
    ),
  );

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
      case _stopComputer:
        _add(TrayAction.stopComputer);
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

/// Connects the tray menu to the window: Open shows it, Quit ends the app, and the computer
/// entry stops the owner's live computer sessions.
///
/// A [persistent] tray (Windows) shows its icon from [start] on; a tray that is not (macOS,
/// D-CU-07-4) shows the icon only while the computer entry is set.
///
/// Example:
/// ```dart
/// final tray = DesktopTray(port: TrayManagerTray(), behavior: behavior);
/// await tray.start(tooltip: 'ExampleChat', openLabel: l.trayOpen, quitLabel: l.trayQuit);
/// await tray.setComputerEntry(l.trayComputerStop('Desk'), onStop: stopAll);
/// ```
final class DesktopTray {
  /// Creates the tray of [behavior] on [port].
  DesktopTray({
    required this.port,
    required this.behavior,
    this.persistent = true,
  });

  /// The platform tray.
  final TrayPort port;

  /// The window it drives.
  final TrayWindow behavior;

  /// Whether the icon stays while no computer session is live.
  final bool persistent;

  StreamSubscription<TrayAction>? _sub;
  ({String tooltip, String openLabel, String quitLabel})? _labels;
  String? _computerLabel;
  void Function()? _onStop;
  bool _shown = false;

  /// Sets the icon's [tooltip] and the menu [openLabel] and [quitLabel], and shows the icon
  /// when the tray is [persistent] or the computer entry is set; a later call changes the
  /// texts (the language of the app changed).
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
        case TrayAction.stopComputer:
          // A click that raced the removal of the entry stops nothing.
          if (_computerLabel != null) _onStop?.call();
      }
    });
    _labels = (tooltip: tooltip, openLabel: openLabel, quitLabel: quitLabel);
    if (persistent || _computerLabel != null) await _show();
  }

  /// Sets the computer entry [label] with [onStop], or removes it with null; a tray that is
  /// not [persistent] shows its icon with the entry and hides it without.
  Future<void> setComputerEntry(
    String? label, {
    void Function()? onStop,
  }) async {
    _computerLabel = label;
    _onStop = label == null ? null : onStop;
    if (label != null && !_shown && _labels != null) await _show();
    if (_shown) await port.setComputerEntry(label);
    if (label == null && !persistent && _shown) {
      _shown = false;
      await port.hide();
    }
  }

  Future<void> _show() async {
    final labels = _labels!;
    _shown = true;
    await port.show(
      iconAsset: trayIconAsset,
      tooltip: labels.tooltip,
      openLabel: labels.openLabel,
      quitLabel: labels.quitLabel,
    );
  }

  /// Removes the icon and stops listening.
  Future<void> dispose() async {
    await _sub?.cancel();
    await port.dispose();
  }
}
