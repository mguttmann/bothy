// SPDX-License-Identifier: Apache-2.0

// "New agent" in the space menu of the desktop and the phone (brief A1-08 Tests; 01 6.20 item 1;
// 26 8.1): shown only with an offer (an owner, a server with the operations and a runner that
// hosts agents), hidden otherwise; the entry opens the dialog, full screen on the phone.

import 'package:admin_ui/admin_ui.dart';
import 'package:client/new_agent/new_agent_entry.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:client/sidebar/sidebar_column.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';

const NewAgentOffer _offer = NewAgentOffer(
  templates: [],
  runners: [
    HostingRunner(
      id: 'runner',
      name: 'Dev runner',
      ceiling: AgentToolClass.host,
    ),
  ],
);

Future<void> _openSpaceMenu(WidgetTester tester) async {
  await tester.tap(
    find.byKey(ValueKey('sidebar.space.${spaceId(1).value}')),
    buttons: kSecondaryButton,
  );
  await tester.pumpAndSettle();
}

void main() {
  Future<ColumnHost> column(
    WidgetTester tester, {
    required NewAgentOffer? offer,
    bool owner = true,
    String platform = 'macos',
  }) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(
      () => host.db.structureDao.upsertSpaceIfNewer(space(1, name: 'Team')),
    );
    await host.pump(
      tester,
      SidebarColumn(phone: platform != 'macos'),
      platform: platform,
      overrides: [
        isOwnerProvider.overrideWith((ref) async => owner),
        newAgentOfferProvider.overrideWith((ref) async => offer),
      ],
    );
    return host;
  }

  for (final platform in ['macos', 'ios']) {
    testWidgets('$platform: the space menu has New agent with an offer', (
      tester,
    ) async {
      final host = await column(tester, offer: _offer, platform: platform);
      await _openSpaceMenu(tester);
      expect(find.byKey(const ValueKey('menu.newAgent')), findsOneWidget);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await host.dispose(tester);
    });

    testWidgets('$platform: no entry without an offer', (tester) async {
      final host = await column(tester, offer: null, platform: platform);
      await _openSpaceMenu(tester);
      expect(find.byKey(const ValueKey('menu.newAgent')), findsNothing);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await host.dispose(tester);
    });
  }
}
