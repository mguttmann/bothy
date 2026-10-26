// SPDX-License-Identifier: Apache-2.0

// The macOS menu bar (brief M4-24 Scope): its items are the actions of the table, they run in
// the shell's registry, carry no key equivalent (the shortcut layer owns the keys) and the
// application menu keeps the system items.

import 'package:client/keyboard/app_shortcuts.dart';
import 'package:client/keyboard/intents.dart';
import 'package:client/keyboard/menu_bar.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/widgets.dart';

Iterable<PlatformMenuItem> flatten(Iterable<PlatformMenuItem> items) sync* {
  for (final item in items) {
    yield item;
    if (item is PlatformMenu) yield* flatten(item.menus);
    if (item is PlatformMenuItemGroup) yield* flatten(item.members);
  }
}

void main() {
  testWidgets('menu items run the actions of the shell on screen', (
    tester,
  ) async {
    final ran = <ShortcutAction>[];
    await tester.pumpWidget(
      hostScreen(
        AppShortcuts(
          handlers: {
            for (final a in ShortcutAction.values)
              if (a != ShortcutAction.shortcutList)
                a: KeyboardHandler(() => ran.add(a)),
          },
          child: const SizedBox(),
        ),
      ),
    );
    final menus = appMenus(en, () => KeyboardRegistry.current, 'Example');
    final items = flatten(menus).toList();
    final provided = items.whereType<PlatformProvidedMenuItem>().map(
      (i) => i.type,
    );
    expect(
      provided,
      containsAll([
        PlatformProvidedMenuItemType.quit,
        PlatformProvidedMenuItemType.hide,
        PlatformProvidedMenuItemType.minimizeWindow,
      ]),
    );
    final actions = [
      for (final i in items)
        if (i is! PlatformMenu &&
            i is! PlatformMenuItemGroup &&
            i is! PlatformProvidedMenuItem)
          i,
    ];
    expect(actions, isNotEmpty);
    for (final item in actions) {
      expect(item.shortcut, isNull, reason: item.label);
      item.onSelected!();
    }
    expect(ran, contains(ShortcutAction.quickSwitcher));
    expect(ran, contains(ShortcutAction.stopRun));
    // M4-36: the hosts route exists, so the menu Go offers "Open hosts and terminal".
    expect(ran, contains(ShortcutAction.hostsTerminal));
  });

  testWidgets('the menu bar is built on macOS only', (tester) async {
    final calls = <MethodCall>[];
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.menu,
      (call) async {
        calls.add(call);
        return null;
      },
    );
    addTearDown(
      () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.menu,
        null,
      ),
    );
    await tester.pumpWidget(
      hostScreen(const AppMenuBar(appName: 'Example', child: SizedBox())),
    );
    expect(find.byType(PlatformMenuBar), findsNothing);
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    await tester.pumpWidget(
      hostScreen(const AppMenuBar(appName: 'Example', child: SizedBox())),
    );
    expect(find.byType(PlatformMenuBar), findsOneWidget);
    await tester.pumpWidget(const SizedBox());
    debugDefaultTargetPlatformOverride = null;
    expect(calls.map((c) => c.method), contains('Menu.setMenus'));
  });
}
