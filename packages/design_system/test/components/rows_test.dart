// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  group('thread row', () {
    testWidgets('label, bold when unread, time, waiting badge, tags, error', (
      tester,
    ) async {
      final l = await en();
      var opened = 0;
      await pumpDs(
        tester,
        DsThreadRow(
          const DsThreadRowData(
            id: 't1',
            title: 'Release notes',
            timeLabel: '10:42',
            unreadCount: 2,
            mentionCount: 1,
            runState: DsRunState.running,
            waitingForMe: true,
            operatorName: 'Robin',
            tags: [DsTagData('release', DsCategory.loch)],
            errorText: 'Project not ready',
          ),
          onOpen: () => opened++,
        ),
      );
      final title = tester.widget<Text>(find.text('\u2068Release notes\u2069'));
      expect(title.style?.fontWeight, FontWeight.w700);
      expect(find.text(l.threadRowWaitingForYou), findsOneWidget);
      expect(
        find.text(l.threadRowOperator('\u2068Robin\u2069')),
        findsOneWidget,
      );
      expect(find.text('\u2068release\u2069'), findsOneWidget);
      expect(find.text('Project not ready'), findsOneWidget);
      expect(
        tester.widgetList<DsIcon>(find.byType(DsIcon)).first.glyph,
        DsIcons.runRunning,
      );
      final label = l.threadRowLabel(
        'Release notes',
        l.liveBoxStateRunning,
        '10:42',
        l.threadRowUnread(2),
      );
      expect(
        find.bySemanticsLabel(RegExp('^${RegExp.escape(label)}')),
        findsOneWidget,
      );
      await tester.tap(find.byType(DsThreadRow));
      expect(opened, 1);
    });

    testWidgets('a read row is not bold; mention icon without run', (
      tester,
    ) async {
      await pumpDs(
        tester,
        DsThreadRow(
          const DsThreadRowData(
            id: 't1',
            title: 'Release notes',
            timeLabel: '10:42',
            mentionCount: 1,
          ),
          onOpen: () {},
        ),
      );
      final title = tester.widget<Text>(find.text('\u2068Release notes\u2069'));
      expect(title.style?.fontWeight, isNot(FontWeight.w700));
      final icon = tester.widgetList<DsIcon>(find.byType(DsIcon)).first;
      expect(icon.color, DsColors.light.attentionFg);
    });

    testWidgets('selected, done and Enter opens', (tester) async {
      final l = await en();
      var opened = 0;
      await pumpDs(
        tester,
        DsThreadRow(
          const DsThreadRowData(
            id: 't1',
            title: 'Old post',
            timeLabel: 'Yesterday',
            done: true,
          ),
          selected: true,
          onOpen: () => opened++,
        ),
      );
      final state = tester.state<DsThreadRowState>(find.byType(DsThreadRow));
      expect(state.background, DsColors.light.surfaceSelected);
      final title = tester.widget<Text>(find.text('\u2068Old post\u2069'));
      expect(title.style?.color, DsColors.light.textMuted);
      expect(
        find.bySemanticsLabel(
          RegExp(
            '^${RegExp.escape(l.threadRowLabel('Old post', l.threadRowStateDone, 'Yesterday', l.threadRowUnread(0)))}',
          ),
        ),
        findsOneWidget,
      );
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      expect(opened, 1);
    });
  });

  group('channel row', () {
    testWidgets('read, unread, mentions, run, muted, selected', (tester) async {
      final l = await en();
      final c = DsColors.light;
      Future<DsChannelRowState> show(
        DsChannelRowData data, {
        bool selected = false,
      }) async {
        await pumpDs(
          tester,
          DsChannelRow(data, selected: selected, onOpen: () {}),
        );
        return tester.state<DsChannelRowState>(find.byType(DsChannelRow));
      }

      var s = await show(const DsChannelRowData(id: 'c', name: 'general'));
      expect(s.nameColor, c.textOnSidebarSecondary);
      expect(find.text('\u2068general\u2069'), findsOneWidget);

      s = await show(
        const DsChannelRowData(id: 'c', name: 'general', unreadCount: 3),
      );
      expect(s.nameColor, c.textOnSidebar);
      expect(s.unreadBar, isTrue);

      await show(
        const DsChannelRowData(
          id: 'c',
          name: 'general',
          unreadCount: 3,
          mentionCount: 2,
          runActive: true,
        ),
      );
      expect(find.text('2'), findsOneWidget);
      expect(
        find.bySemanticsLabel(
          '${'general'}, ${l.threadRowUnread(3)}, ${l.threadRowMentions(2)}, ${l.channelRowRunActive}',
        ),
        findsOneWidget,
      );

      s = await show(
        const DsChannelRowData(id: 'c', name: 'general', muted: true),
      );
      expect(s.nameColor, c.textOnSidebarMuted);
      expect(
        tester.widgetList<DsIcon>(find.byType(DsIcon)).map((i) => i.glyph),
        contains(DsIcons.actionMute),
      );

      s = await show(
        const DsChannelRowData(id: 'c', name: 'general'),
        selected: true,
      );
      expect(s.background, c.surfaceSidebarSelected);

      await show(
        const DsChannelRowData(
          id: 'c',
          name: 'ideas',
          kind: DsChannelKind.forum,
        ),
      );
      expect(
        tester.widgetList<DsIcon>(find.byType(DsIcon)).first.glyph,
        DsIcons.channelForum,
      );
    });
  });

  group('space row', () {
    testWidgets('expanded and collapsed states, arrow keys, unread dot', (
      tester,
    ) async {
      final l = await en();
      final toggles = <bool>[];
      await pumpDs(
        tester,
        DsSpaceRow(
          const DsSpaceRowData(
            id: 's',
            name: 'Support',
            icon: 'headset',
            category: DsCategory.loch,
          ),
          onToggle: toggles.add,
        ),
      );
      expect(find.text('\u2068Support\u2069'), findsOneWidget);
      expect(
        tester.getSemantics(find.byType(DsSpaceRow)),
        isSemantics(hasExpandedState: true, isExpanded: true),
      );
      await tester.tap(find.byType(DsSpaceRow));
      expect(toggles, [false]);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.arrowLeft);
      expect(toggles, [false, false]);
      await tester.sendKeyEvent(LogicalKeyboardKey.arrowRight);
      expect(toggles, [false, false]);

      await pumpDs(
        tester,
        DsSpaceRow(
          const DsSpaceRowData(
            id: 's',
            name: 'Support',
            icon: 'headset',
            category: DsCategory.loch,
            expanded: false,
            hasUnread: true,
          ),
          onToggle: toggles.add,
        ),
      );
      expect(find.bySemanticsLabel(l.spaceRowUnread), findsOneWidget);
      expect(
        tester.getSemantics(find.byType(DsSpaceRow)),
        isSemantics(hasExpandedState: true, isExpanded: false),
      );
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.arrowRight);
      expect(toggles.last, isTrue);
    });
  });

  testWidgets('a run waiting for someone else is not "Waiting for you"', (
    tester,
  ) async {
    final l = await en();
    Future<void> show({required bool mine}) => pumpDs(
      tester,
      DsThreadRow(
        DsThreadRowData(
          id: 't',
          title: 'Deploy question',
          timeLabel: '10:00',
          runState: DsRunState.waitingInput,
          waitingForMe: mine,
        ),
        onOpen: () {},
      ),
    );
    await show(mine: false);
    expect(
      find.bySemanticsLabel(
        RegExp(
          '^${RegExp.escape(l.threadRowLabel('Deploy question', l.threadRowStateWaiting, '10:00', l.threadRowUnread(0)))}',
        ),
      ),
      findsOneWidget,
    );
    await show(mine: true);
    expect(
      find.bySemanticsLabel(
        RegExp(
          '^${RegExp.escape(l.threadRowLabel('Deploy question', l.liveBoxStateWaitingForYou, '10:00', l.threadRowUnread(0)))}',
        ),
      ),
      findsOneWidget,
    );
  });
}
