// SPDX-License-Identifier: Apache-2.0

/// The macOS menu bar with the actions of the shortcut table (brief M4-24 Scope;
/// docs/spec/01-product.md 7).
///
/// The menus offer the same actions as the keys, by pointer and to VoiceOver, and run them
/// through the same [KeyboardRegistry], so an action that has no target where the focus is
/// does nothing from the menu either. The items carry no key equivalent: the key belongs to
/// the shortcut layer alone, which works the same on Windows and in tests, and a second
/// binding in the menu would run an action twice (docs/decisions.d/M4-24.md). The
/// application menu keeps the system items (About, Hide with Cmd+H, Quit with Cmd+Q, Window
/// with Cmd+M), so no system assignment is lost.
library;

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';

import 'app_shortcuts.dart';
import 'bindings.dart';
import 'intents.dart';
import 'shortcut_list.dart';

/// The menus over [child] on macOS; on every other platform [child] alone.
///
/// [appName] is the title of the application menu (macOS shows the bundle name there anyway).
/// It sits above the router, so there is one menu bar for the whole app; its items run in
/// [KeyboardRegistry.current], the shell on screen.
class AppMenuBar extends StatelessWidget {
  /// Creates the menu bar.
  const AppMenuBar({super.key, required this.appName, required this.child});

  /// The title of the application menu.
  final String appName;

  /// The shell.
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final l = Localizations.of<AppLocalizations>(context, AppLocalizations);
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.macOS || l == null) {
      return child;
    }
    return PlatformMenuBar(
      menus: appMenus(l, () => KeyboardRegistry.current, appName),
      child: child,
    );
  }
}

/// The menus of the app in the language of [l].
///
/// Every item of an action of the table runs [KeyboardRegistry.invoke] on the registry that
/// [registry] returns when the item is chosen (nothing while no shell is built); "Open hosts
/// and terminal" is in the menu Go since M4-36 added its route.
List<PlatformMenuItem> appMenus(
  AppLocalizations l,
  KeyboardRegistry? Function() registry,
  String appName,
) {
  final labels = {
    for (final row in shortcutRows)
      for (final action in row.actions) action: shortcutRowLabel(l, row),
  };
  PlatformMenuItem item(ShortcutAction action, [String? label]) =>
      PlatformMenuItem(
        label: label ?? labels[action]!,
        onSelected: () => registry()?.invoke(action),
      );
  return [
    PlatformMenu(
      label: appName,
      menus: [
        const PlatformMenuItemGroup(
          members: [
            PlatformProvidedMenuItem(type: PlatformProvidedMenuItemType.about),
          ],
        ),
        PlatformMenuItemGroup(members: [item(ShortcutAction.settings)]),
        const PlatformMenuItemGroup(
          members: [
            PlatformProvidedMenuItem(
              type: PlatformProvidedMenuItemType.servicesSubmenu,
            ),
          ],
        ),
        const PlatformMenuItemGroup(
          members: [
            PlatformProvidedMenuItem(type: PlatformProvidedMenuItemType.hide),
            PlatformProvidedMenuItem(
              type: PlatformProvidedMenuItemType.hideOtherApplications,
            ),
            PlatformProvidedMenuItem(
              type: PlatformProvidedMenuItemType.showAllApplications,
            ),
          ],
        ),
        const PlatformMenuItemGroup(
          members: [
            PlatformProvidedMenuItem(type: PlatformProvidedMenuItemType.quit),
          ],
        ),
      ],
    ),
    PlatformMenu(
      label: l.menuGo,
      menus: [
        PlatformMenuItemGroup(
          members: [
            item(ShortcutAction.quickSwitcher),
            item(ShortcutAction.search),
            item(ShortcutAction.runningNow),
            item(ShortcutAction.mentions),
            item(ShortcutAction.hostsTerminal),
          ],
        ),
        PlatformMenuItemGroup(
          members: [
            item(ShortcutAction.nextChannel, l.shortcutChannel),
            item(ShortcutAction.nextUnreadChannel, l.shortcutUnreadChannel),
            item(ShortcutAction.nextColumn, l.shortcutColumn),
          ],
        ),
      ],
    ),
    PlatformMenu(
      label: l.menuThread,
      menus: [
        PlatformMenuItemGroup(
          members: [
            item(ShortcutAction.newPost),
            item(ShortcutAction.searchInThread),
            item(ShortcutAction.focusInput),
            item(ShortcutAction.markRead),
            item(ShortcutAction.toggleDone),
          ],
        ),
        PlatformMenuItemGroup(
          members: [
            item(ShortcutAction.stopRun),
            item(ShortcutAction.toggleLiveBox),
            item(ShortcutAction.toggleSessionColumn),
          ],
        ),
      ],
    ),
    PlatformMenu(
      label: l.menuWindow,
      menus: const [
        PlatformProvidedMenuItem(
          type: PlatformProvidedMenuItemType.minimizeWindow,
        ),
        PlatformProvidedMenuItem(type: PlatformProvidedMenuItemType.zoomWindow),
        PlatformProvidedMenuItem(
          type: PlatformProvidedMenuItemType.toggleFullScreen,
        ),
        PlatformProvidedMenuItem(
          type: PlatformProvidedMenuItemType.arrangeWindowsInFront,
        ),
      ],
    ),
    PlatformMenu(label: l.menuHelp, menus: [item(ShortcutAction.shortcutList)]),
  ];
}
