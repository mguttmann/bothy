// SPDX-License-Identifier: Apache-2.0

// The status mark of unseen agent news on sidebar rows (01 6.17, components.md 7, brief M4-52
// Design note 5): four states, each with a colour of the sidebar pairings and a shape of its own,
// a tooltip and the same words at the end of the row label; the mark stands where the counters
// stand and replaces the unread bar. Test data: Alex, Sam, Robin.

import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

Widget _onSidebar(Widget child) => Builder(
  builder: (context) =>
      ColoredBox(color: context.dsColors.surfaceSidebar, child: child),
);

/// A mark alone, at its own size.
Widget _alone(DsChatStatus status) => _onSidebar(
  Align(alignment: Alignment.topLeft, child: DsChatStatusMark(status)),
);

Finder _mark() => find.byType(DsChatStatusMark);

void main() {
  group('DsChatStatus', () {
    test('urgency: failed over waiting over working over done', () {
      final sorted = [...DsChatStatus.values]
        ..sort((a, b) => b.urgency.compareTo(a.urgency));
      expect(sorted, [
        DsChatStatus.failed,
        DsChatStatus.waiting,
        DsChatStatus.working,
        DsChatStatus.done,
      ]);
      expect(
        DsChatStatus.mostUrgent([
          DsChatStatus.done,
          DsChatStatus.waiting,
          null,
          DsChatStatus.working,
        ]),
        DsChatStatus.waiting,
      );
      expect(DsChatStatus.mostUrgent(const [null, null]), isNull);
      expect(DsChatStatus.mostUrgent(const []), isNull);
    });

    test('every state has its own words in de and en', () async {
      for (final l in [await en(), await de()]) {
        final words = {
          for (final s in DsChatStatus.values) dsChatStatusLabel(l, s),
        };
        expect(words, hasLength(DsChatStatus.values.length));
        expect(dsChatStatusLabel(l, DsChatStatus.done), l.chatStatusDone);
        expect(dsChatStatusLabel(l, DsChatStatus.working), l.chatStatusWorking);
        expect(dsChatStatusLabel(l, DsChatStatus.waiting), l.chatStatusWaiting);
        expect(dsChatStatusLabel(l, DsChatStatus.failed), l.chatStatusFailed);
      }
    });
  });

  group('DsChatStatusMark', () {
    testWidgets('colour, shape and size per state, in light and dark', (
      tester,
    ) async {
      for (final theme in DsThemeMode.values) {
        final c = theme == DsThemeMode.light ? DsColors.light : DsColors.dark;
        final expected = {
          DsChatStatus.done: (c.categoryMossOnSidebar, DsChatStatusShape.dot),
          DsChatStatus.working: (
            c.categoryGorseOnSidebar,
            DsChatStatusShape.ring,
          ),
          DsChatStatus.waiting: (
            c.categoryLochOnSidebar,
            DsChatStatusShape.diamond,
          ),
          DsChatStatus.failed: (
            c.categoryRowanOnSidebar,
            DsChatStatusShape.triangle,
          ),
        };
        for (final MapEntry(key: status, value: (color, shape))
            in expected.entries) {
          await pumpDs(tester, _alone(status), theme: theme);
          final state = tester.state<DsChatStatusMarkState>(_mark());
          expect(state.color, color, reason: '$status in $theme');
          expect(state.shape, shape, reason: '$status in $theme');
          expect(tester.getSize(_mark()), const Size.square(DsSize.statusDot));
        }
      }
    });

    testWidgets('the shapes differ, so no state rests on colour alone', (
      tester,
    ) async {
      final shapes = <DsChatStatusShape>{};
      for (final status in DsChatStatus.values) {
        await pumpDs(tester, _alone(status));
        shapes.add(tester.state<DsChatStatusMarkState>(_mark()).shape);
      }
      expect(shapes, hasLength(DsChatStatus.values.length));
    });

    testWidgets('the tooltip names the state in words; no own node', (
      tester,
    ) async {
      final l = await en();
      await pumpDs(tester, _alone(DsChatStatus.failed));
      final tip = tester.widget<Tooltip>(
        find.descendant(of: _mark(), matching: find.byType(Tooltip)),
      );
      expect(tip.message, l.chatStatusFailed);
      expect(tip.excludeFromSemantics, isTrue);
    });
  });

  group('thread row', () {
    Future<DsSidebarThreadRowState> show(
      WidgetTester tester,
      DsSidebarThreadRowData data, {
      Locale locale = const Locale('en'),
    }) async {
      await pumpDs(
        tester,
        _onSidebar(DsSidebarThreadRow(data, onOpen: () {})),
        width: 320,
        locale: locale,
      );
      return tester.state<DsSidebarThreadRowState>(
        find.byType(DsSidebarThreadRow),
      );
    }

    testWidgets('each state shows its mark and ends the label with its words', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      final l = await en();
      for (final status in DsChatStatus.values) {
        await show(
          tester,
          DsSidebarThreadRowData(
            id: 't',
            title: 'Export format',
            status: status,
          ),
        );
        expect(_mark(), findsOneWidget);
        expect(tester.widget<DsChatStatusMark>(_mark()).status, status);
        expect(
          find.bySemanticsLabel(
            'Export format, ${l.threadRowStateOpen}, '
            '${dsChatStatusLabel(l, status)}',
          ),
          findsOneWidget,
        );
      }
      expect(dsSemanticsProblems(tester), isEmpty);
      handle.dispose();
    });

    testWidgets('the mark replaces the unread bar; the mention counter stays', (
      tester,
    ) async {
      var s = await show(
        tester,
        const DsSidebarThreadRowData(
          id: 't',
          title: 'Export format',
          unreadCount: 2,
          mentionCount: 1,
          status: DsChatStatus.done,
        ),
      );
      expect(s.unreadBar, isFalse);
      // The title stays bold while unread, only the bar gives way to the mark.
      expect(s.boldTitle, isTrue);
      expect(_mark(), findsOneWidget);
      expect(find.text('1'), findsOneWidget);
      // The mark stands before the counter, at the trailing edge where the counters stand.
      expect(
        tester.getCenter(_mark()).dx,
        lessThan(tester.getCenter(find.text('1')).dx),
      );

      s = await show(
        tester,
        const DsSidebarThreadRowData(
          id: 't',
          title: 'Export format',
          unreadCount: 2,
        ),
      );
      expect(s.unreadBar, isTrue);
      expect(_mark(), findsNothing);
    });

    testWidgets('the German label ends with the German words', (tester) async {
      final handle = tester.ensureSemantics();
      final l = await de();
      await show(
        tester,
        const DsSidebarThreadRowData(
          id: 't',
          title: 'Exportformat',
          status: DsChatStatus.waiting,
        ),
        locale: const Locale('de'),
      );
      expect(
        find.bySemanticsLabel(
          'Exportformat, ${l.threadRowStateOpen}, ${l.chatStatusWaiting}',
        ),
        findsOneWidget,
      );
      handle.dispose();
    });
  });

  group('channel row', () {
    testWidgets('the aggregate shows its mark, its words and no unread bar', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      final l = await en();
      await pumpDs(
        tester,
        _onSidebar(
          DsChannelRow(
            const DsChannelRowData(
              id: 'c',
              name: 'server',
              kind: DsChannelKind.forum,
              projectBound: true,
              unreadCount: 3,
              status: DsChatStatus.failed,
            ),
            onOpen: () {},
          ),
        ),
        width: 320,
      );
      final s = tester.state<DsChannelRowState>(find.byType(DsChannelRow));
      expect(s.unreadBar, isFalse);
      expect(s.boldTitle, isTrue);
      expect(
        tester.widget<DsChatStatusMark>(_mark()).status,
        DsChatStatus.failed,
      );
      expect(
        find.bySemanticsLabel(
          'server, ${l.channelKindNameProject}, ${l.threadRowUnread(3)}, '
          '${l.chatStatusFailed}',
        ),
        findsOneWidget,
      );
      handle.dispose();
    });

    testWidgets('without a status the row keeps its unread bar', (
      tester,
    ) async {
      await pumpDs(
        tester,
        _onSidebar(
          DsChannelRow(
            const DsChannelRowData(id: 'c', name: 'general', unreadCount: 3),
            onOpen: () {},
          ),
        ),
        width: 320,
      );
      final s = tester.state<DsChannelRowState>(find.byType(DsChannelRow));
      expect(s.unreadBar, isTrue);
      expect(_mark(), findsNothing);
    });
  });
}
