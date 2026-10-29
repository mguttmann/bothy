// SPDX-License-Identifier: Apache-2.0

// "Add forums for existing folders" in the space menu for owners on the desktop (01 6.10 item 1;
// D-M4-44-7) and "Rename project" in the menu of a bound forum with `manage` (01 6.11;
// D-M4-44-6).

import 'package:client/menus/channel_menu.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:client/sidebar/sidebar_column.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/structure.dart';

Future<void> openSpaceMenu(WidgetTester tester) async {
  await tester.tap(
    find.byKey(ValueKey('sidebar.space.${spaceId(1).value}')),
    buttons: kSecondaryButton,
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('an owner gets the entry in the space menu', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(
      () => host.db.structureDao.upsertSpaceIfNewer(
        space(1, name: 'Engineering'),
      ),
    );
    await host.pump(
      tester,
      const SidebarColumn(),
      overrides: [isOwnerProvider.overrideWith((ref) async => true)],
    );
    await openSpaceMenu(tester);
    expect(find.byKey(const ValueKey('menu.addFolderForums')), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await host.dispose(tester);
  });

  testWidgets('no entry for a person who is not an owner', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(
      () => host.db.structureDao.upsertSpaceIfNewer(
        space(1, name: 'Engineering'),
      ),
    );
    await host.pump(
      tester,
      const SidebarColumn(),
      overrides: [isOwnerProvider.overrideWith((ref) async => false)],
    );
    await openSpaceMenu(tester);
    expect(find.byKey(const ValueKey('menu.addFolderForums')), findsNothing);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await host.dispose(tester);
  });

  group('Rename project', () {
    Future<void> menuOf(
      WidgetTester tester,
      Channel forum,
      List<Capability> capabilities,
      Matcher matcher,
    ) async {
      final host = await ColumnHost.open(tester);
      await tester.runAsync(() async {
        await host.db.structureDao.upsertSpaceIfNewer(space(1));
        await host.db.structureDao.upsertChannelIfNewer(forum);
        await host.db.structureDao.putChannelAccess(
          forum.id,
          AccessView(level: AccessLevel.read, capabilities: capabilities),
        );
      });
      await host.pump(
        tester,
        ChannelMenu(channel: forum, child: Text(forum.name)),
      );
      await tester.tap(find.text(forum.name), buttons: kSecondaryMouseButton);
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('menu.renameProject')), matcher);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await host.dispose(tester);
    }

    final bound = channel(
      2,
      kind: ChannelKind.forum,
      name: 'website',
    ).copyWith(projectBound: true);

    testWidgets('a bound forum with manage has the entry', (tester) async {
      await menuOf(tester, bound, [Capability.manage], findsOneWidget);
    });

    testWidgets('without manage there is none', (tester) async {
      await menuOf(tester, bound, const [], findsNothing);
    });

    testWidgets('a forum without a binding has none', (tester) async {
      await menuOf(tester, channel(3, kind: ChannelKind.forum, name: 'ideas'), [
        Capability.manage,
      ], findsNothing);
    });
  });
}
