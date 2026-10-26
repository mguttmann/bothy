// SPDX-License-Identifier: Apache-2.0

// The sidebar group "Live and statistics" of the desktop (brief M6-16 Scope; 01 4, D12): with
// both entries it shows below the head and each entry calls its target; without them (the
// phone) there is no group.

import 'package:client/sidebar/sidebar_column.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/widgets.dart';

Widget _sidebar({VoidCallback? onLive, VoidCallback? onStatistics}) =>
    hostScreen(
      SidebarView(
        productName: 'Team Chat',
        nodes: const [],
        onToggle: (_, _) {},
        onMove: (_, _) {},
        onOpenChannel: (_) {},
        onSettings: () {},
        onLogout: () {},
        onLive: onLive,
        onStatistics: onStatistics,
      ),
    );

void main() {
  testWidgets('the group opens Live and Statistics', (tester) async {
    desktopWindow(tester);
    final opened = <String>[];
    await tester.pumpWidget(
      _sidebar(
        onLive: () => opened.add('live'),
        onStatistics: () => opened.add('statistics'),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text(en.sidebarLiveGroup), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('sidebar.live')));
    await tester.tap(find.byKey(const ValueKey('sidebar.statistics')));
    expect(opened, ['live', 'statistics']);
    expect(find.bySemanticsLabel(en.sidebarLive), findsOneWidget);
  });

  testWidgets('without the entries there is no group', (tester) async {
    desktopWindow(tester);
    await tester.pumpWidget(_sidebar());
    await tester.pumpAndSettle();
    expect(find.text(en.sidebarLiveGroup), findsNothing);
    expect(find.byKey(const ValueKey('sidebar.live')), findsNothing);
  });
}
