// SPDX-License-Identifier: Apache-2.0

// Accessibility of the sidebar (brief M4-32 Design note 1; 01 8; components.md 7): spaces are
// tree nodes with their expanded state, channels say unread, mentions, a running agent and
// muted in words, the selected channel is selected, and every control is a labelled button.

import 'package:client/sidebar/sidebar_column.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/sidebar/sidebar_golden_test.dart' as golden;
import '../support/structure.dart';

Widget sidebar() => Builder(
  builder: (context) => SizedBox.fromSize(
    size: golden.column,
    child: SidebarView(
      productName: 'Team Chat',
      nodes: golden.nodes(),
      ownName: 'Alex',
      selectedChannel: channelId(6),
      onToggle: (_, _) {},
      onMove: (_, _) {},
      onOpenChannel: (_) {},
      onSettings: () {},
      onLogout: () {},
    ),
  ),
);

void main() {
  dsAccessibilityTests(
    'the sidebar',
    build: sidebar,
    semantics: (tester, l) {
      expect(
        tester.getSemantics(find.bySemanticsLabel(RegExp('^Product'))),
        isSemantics(hasExpandedState: true, isExpanded: true),
      );
      expect(
        tester.getSemantics(find.bySemanticsLabel(RegExp('^Operations'))),
        isSemantics(hasExpandedState: true, isExpanded: false),
      );
      // State in words, never by color alone (01 8 "Color").
      expect(
        find.bySemanticsLabel(RegExp('^ideas.*${l.channelRowRunActive}')),
        findsOneWidget,
      );
      expect(
        find.bySemanticsLabel(RegExp('^random.*${l.channelRowMuted}')),
        findsOneWidget,
      );
      expect(
        tester.getSemantics(find.bySemanticsLabel(RegExp('^design'))),
        isSemantics(isSelected: true),
      );
    },
    reachable: (l) => [
      find.byKey(const ValueKey('sidebar.jumpTo')),
      find.byKey(const ValueKey('sidebar.runningNow')),
      find.byKey(const ValueKey('sidebar.settings')),
    ],
  );
}
