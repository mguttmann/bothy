// SPDX-License-Identifier: Apache-2.0

// Accessibility of the channel menu and its level submenus (brief M4-32 Design note 1; 01 8;
// brief M4-33): the menu has the channel's name, every entry is a labelled button, the chosen
// level says it is selected (the check mark alone is not read), and the menus meet contrast,
// labelled targets and the desktop target size.

import 'package:client/menus/channel_menu.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:client/sync/notify_ops.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';

const String rowText = 'the row';

void main() {
  late ColumnHost host;
  var ids = 0;
  final general = channel(1, name: 'general');

  Future<void> openMenu(WidgetTester tester, {NotificationLevel? own}) async {
    host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      final db = host.db;
      await db.structureDao.upsertSpaceIfNewer(space(1, name: 'Team'));
      await db.structureDao.upsertChannelIfNewer(general);
      if (own != null) {
        await db.personalDao.upsertNotificationPreferenceIfNewer(
          NotificationPreference(
            targetKind: NotificationTargetKind.channel,
            targetId: channelId(1).value,
            level: own,
            version: const ResourceVersion(1),
          ),
        );
      }
    });
    await host.pump(
      tester,
      ChannelMenu(channel: general, child: const Text(rowText)),
      overrides: [
        notifyWriterProvider.overrideWithValue(
          NotifyWriter(
            host.engine,
            host.db,
            () =>
                '0192f4a0-7b1c-7000-8000-${(++ids).toRadixString(16).padLeft(12, '0')}',
          ),
        ),
      ],
    );
    await tester.tap(find.text(rowText), buttons: kSecondaryMouseButton);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('menu.notifications')));
    await tester.pumpAndSettle();
  }

  testWidgets('the channel menu: roles, labels and states for screen readers', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    await openMenu(tester, own: NotificationLevel.mentions);
    final problems = dsSemanticsProblems(tester);
    expect(problems, isEmpty, reason: problems.join('\n'));
    expect(find.bySemanticsLabel(RegExp('general')), findsWidgets);
    expect(
      tester.getSemantics(find.byKey(const ValueKey('menu.level.mentions'))),
      isSemantics(
        label: 'Mentions',
        isButton: true,
        hasSelectedState: true,
        isSelected: true,
        hasEnabledState: true,
        isEnabled: true,
        isFocusable: true,
        hasTapAction: true,
        hasFocusAction: true,
      ),
    );
    expect(
      tester.getSemantics(find.byKey(const ValueKey('menu.level.all'))),
      isSemantics(
        label: 'All',
        isButton: true,
        hasSelectedState: true,
        hasEnabledState: true,
        isEnabled: true,
        isFocusable: true,
        hasTapAction: true,
        hasFocusAction: true,
      ),
    );
    await host.dispose(tester);
    handle.dispose();
  });

  testWidgets(
    'the channel menu: contrast, labelled targets and target size in light',
    (tester) async {
      final handle = tester.ensureSemantics();
      await openMenu(tester);
      await dsExpectAccessible(tester);
      await host.dispose(tester);
      handle.dispose();
    },
  );
}
