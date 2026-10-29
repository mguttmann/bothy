// SPDX-License-Identifier: Apache-2.0

// The sidebar rows of M4-40 (components.md 7, 01 6.8): the kind of a channel (icon, tooltip,
// label; item 11), the group arrow of a channel with thread rows (item 7), the "+" action of a
// forum row (item 8), the thread rows with their tree line and markers (items 1 and 6) and the
// row "All posts" (item 5). Test data: Alex, Sam, Robin.

import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

/// The sidebar surface the rows are drawn on in the app (components.md 7).
Widget _onSidebar(Widget child) => Builder(
  builder: (context) =>
      ColoredBox(color: context.dsColors.surfaceSidebar, child: child),
);

Iterable<DsGlyph> _glyphs(WidgetTester tester) =>
    tester.widgetList<DsIcon>(find.byType(DsIcon)).map((i) => i.glyph);

Finder _icon(DsGlyph glyph) =>
    find.byWidgetPredicate((w) => w is DsIcon && identical(w.glyph, glyph));

Finder _draggable() => find.byWidgetPredicate((w) => w is Draggable);

void main() {
  group('channel kind', () {
    test('the glyph per kind; project only for a forum', () {
      expect(dsChannelGlyph(DsChannelKind.text), DsIcons.channelText);
      expect(dsChannelGlyph(DsChannelKind.forum), DsIcons.channelForum);
      expect(
        dsChannelGlyph(DsChannelKind.forum, projectBound: true),
        DsIcons.channelProject,
      );
      expect(dsChannelGlyph(DsChannelKind.dm), DsIcons.channelDm);
      expect(dsChannelGlyph(DsChannelKind.unknown), DsIcons.channelText);
      expect(
        dsChannelGlyph(DsChannelKind.text, projectBound: true),
        DsIcons.channelText,
      );
    });

    test('channel.project is the Phosphor glyph brackets-angle, regular', () {
      expect(DsIcons.channelProject.name, 'brackets-angle');
      expect(DsIcons.channelProject.weight, DsIconWeight.regular);
      expect(dsSemanticIcons['channel.project'], DsIcons.channelProject);
    });

    test('the kind sentence and the kind name per kind', () async {
      for (final l in [await en(), await de()]) {
        expect(
          dsChannelKindSentence(l, DsChannelKind.text),
          l.channelKindSentenceText,
        );
        expect(
          dsChannelKindSentence(l, DsChannelKind.forum),
          l.channelKindSentenceForum,
        );
        expect(
          dsChannelKindSentence(l, DsChannelKind.forum, projectBound: true),
          l.channelKindSentenceProject,
        );
        expect(
          dsChannelKindSentence(l, DsChannelKind.dm),
          l.channelKindSentenceText,
        );
        expect(
          dsChannelKindSentence(l, DsChannelKind.unknown),
          l.channelKindSentenceText,
        );
        expect(dsChannelKindName(l, DsChannelKind.text), l.channelKindNameText);
        expect(
          dsChannelKindName(l, DsChannelKind.forum),
          l.channelKindNameForum,
        );
        expect(
          dsChannelKindName(l, DsChannelKind.forum, projectBound: true),
          l.channelKindNameProject,
        );
        expect(dsChannelKindName(l, DsChannelKind.dm), l.channelKindNameText);
        expect(
          dsChannelKindName(l, DsChannelKind.text, projectBound: true),
          l.channelKindNameText,
        );
      }
    });
  });

  group('channel row', () {
    testWidgets('a project forum: icon, tooltip and label with the kind', (
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
              mentionCount: 2,
              runActive: true,
              muted: true,
            ),
            onOpen: () {},
          ),
        ),
      );
      expect(_glyphs(tester).first, DsIcons.channelProject);
      final tooltip = tester.widget<Tooltip>(
        find.ancestor(
          of: _icon(DsIcons.channelProject),
          matching: find.byType(Tooltip),
        ),
      );
      expect(tooltip.message, l.channelKindSentenceProject);
      final label = [
        'server',
        l.channelKindNameProject,
        l.threadRowUnread(3),
        l.threadRowMentions(2),
        l.channelRowRunActive,
        l.channelRowMuted,
      ].join(', ');
      final row = find.bySemanticsLabel(label);
      expect(row, findsOneWidget);
      // The kind sentence reaches the screen reader as the tooltip of the row (01 6.8 item 11).
      expect(
        tester.getSemantics(row),
        isSemantics(label: label, tooltip: l.channelKindSentenceProject),
      );
      expect(dsSemanticsProblems(tester), isEmpty);
      handle.dispose();
    });

    testWidgets('the tooltip names each kind', (tester) async {
      final l = await en();
      for (final (kind, project, sentence) in [
        (DsChannelKind.text, false, l.channelKindSentenceText),
        (DsChannelKind.forum, false, l.channelKindSentenceForum),
        (DsChannelKind.forum, true, l.channelKindSentenceProject),
      ]) {
        await pumpDs(
          tester,
          _onSidebar(
            DsChannelRow(
              DsChannelRowData(
                id: 'c',
                name: 'ideas',
                kind: kind,
                projectBound: project,
              ),
              onOpen: () {},
            ),
          ),
        );
        expect(find.byTooltip(sentence), findsOneWidget, reason: '$kind');
        expect(
          find.bySemanticsLabel(
            'ideas, ${dsChannelKindName(l, kind, projectBound: project)}',
          ),
          findsOneWidget,
        );
      }
    });

    testWidgets('the group arrow toggles with the requested state', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      final l = await en();
      final toggles = <bool>[];
      Future<void> show(bool? expanded) => pumpDs(
        tester,
        _onSidebar(
          DsChannelRow(
            const DsChannelRowData(id: 'c', name: 'general', unreadCount: 1),
            groupExpanded: expanded,
            onToggleGroup: toggles.add,
            onOpen: () {},
          ),
        ),
      );
      final label =
          'general, ${l.channelKindNameText}, ${l.threadRowUnread(1)}';

      await show(true);
      expect(_glyphs(tester).first, DsIcons.actionExpand);
      expect(find.byTooltip(l.sidebarHideThreads), findsOneWidget);
      expect(
        tester.getSemantics(find.bySemanticsLabel(label)),
        isSemantics(
          hasExpandedState: true,
          isExpanded: true,
          isSelected: false,
        ),
      );
      expect(
        tester.getSemantics(find.bySemanticsLabel(l.sidebarHideThreads)),
        isSemantics(isButton: true, hasTapAction: true),
      );
      await tester.tap(find.bySemanticsLabel(l.sidebarHideThreads));
      expect(toggles, [false]);
      // The row shows the state the caller passes; a tap does not flip it by itself.
      expect(find.bySemanticsLabel(l.sidebarHideThreads), findsOneWidget);

      await show(false);
      expect(_glyphs(tester).first, DsIcons.actionCollapse);
      expect(find.byTooltip(l.sidebarShowThreads), findsOneWidget);
      expect(
        tester.getSemantics(find.bySemanticsLabel(label)),
        isSemantics(hasExpandedState: true, isExpanded: false),
      );
      await tester.tap(find.bySemanticsLabel(l.sidebarShowThreads));
      expect(toggles, [false, true]);

      await show(null);
      expect(find.bySemanticsLabel(l.sidebarShowThreads), findsNothing);
      expect(find.bySemanticsLabel(l.sidebarHideThreads), findsNothing);
      expect(_icon(DsIcons.actionExpand), findsNothing);
      expect(_icon(DsIcons.actionCollapse), findsNothing);
      expect(
        tester.getSemantics(find.bySemanticsLabel(label)),
        isSemantics(hasExpandedState: false),
      );
      expect(dsSemanticsProblems(tester), isEmpty);
      handle.dispose();
    });

    testWidgets('Enter on the focused arrow toggles the group', (tester) async {
      final toggles = <bool>[];
      await pumpDs(
        tester,
        _onSidebar(
          DsChannelRow(
            const DsChannelRowData(id: 'c', name: 'general'),
            groupExpanded: true,
            onToggleGroup: toggles.add,
            onOpen: () {},
          ),
        ),
      );
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      expect(toggles, [false]);
    });

    testWidgets('the kind icon stays in its column with and without an arrow', (
      tester,
    ) async {
      for (final density in [
        DsDensityMode.comfortable,
        DsDensityMode.compact,
        DsDensityMode.touch,
      ]) {
        Future<double> iconX(bool? expanded) async {
          await pumpDs(
            tester,
            _onSidebar(
              DsChannelRow(
                const DsChannelRowData(id: 'c', name: 'general'),
                groupExpanded: expanded,
                onToggleGroup: (_) {},
                onOpen: () {},
              ),
            ),
            density: density,
          );
          return tester.getTopLeft(_icon(DsIcons.channelText)).dx;
        }

        final without = await iconX(null);
        expect(await iconX(true), without, reason: density.name);
        expect(await iconX(false), without, reason: density.name);
      }
    });

    testWidgets('"+" shows on hover, focus, selected and touch only', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      final l = await en();
      final addLabel = l.sidebarNewChatIn('server');
      var added = 0;
      Future<DsChannelRowState> show({
        bool selected = false,
        DsDensityMode density = DsDensityMode.comfortable,
      }) async {
        await pumpDs(
          tester,
          _onSidebar(
            DsChannelRow(
              const DsChannelRowData(
                id: 'c',
                name: 'server',
                kind: DsChannelKind.forum,
                projectBound: true,
              ),
              selected: selected,
              groupExpanded: true,
              onToggleGroup: (_) {},
              addLabel: addLabel,
              onAdd: () => added++,
              onOpen: () {},
            ),
          ),
          density: density,
        );
        return tester.state<DsChannelRowState>(find.byType(DsChannelRow));
      }

      // At rest the action is absent: not drawn, not in semantics, not a Tab stop.
      var state = await show();
      expect(state.addVisible, isFalse);
      expect(find.bySemanticsLabel(addLabel), findsNothing);
      expect(_icon(DsIcons.actionNew), findsNothing);

      // Hover.
      final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
      await mouse.addPointer(location: Offset.zero);
      addTearDown(mouse.removePointer);
      await mouse.moveTo(tester.getCenter(find.byType(DsChannelRow)));
      await tester.pump();
      expect(state.addVisible, isTrue);
      expect(find.bySemanticsLabel(addLabel), findsOneWidget);
      expect(find.byTooltip(addLabel), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(addLabel));
      expect(added, 1);
      await mouse.moveTo(const Offset(590, 500));
      await tester.pump();
      expect(state.addVisible, isFalse);
      expect(find.bySemanticsLabel(addLabel), findsNothing);

      // Keyboard focus on the row, then Tab reaches the action and Enter starts it.
      await tester.sendKeyEvent(LogicalKeyboardKey.tab); // the arrow
      await tester.pump();
      expect(state.addVisible, isTrue);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab); // the row
      await tester.pump();
      expect(state.addVisible, isTrue);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab); // the action
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      expect(added, 2);
      primaryFocus?.unfocus();
      await tester.pump();
      expect(state.addVisible, isFalse);

      // Selected.
      state = await show(selected: true);
      expect(state.addVisible, isTrue);
      expect(find.bySemanticsLabel(addLabel), findsOneWidget);

      // Touch: always.
      state = await show(density: DsDensityMode.touch);
      expect(state.addVisible, isTrue);
      expect(
        tester.getSemantics(find.bySemanticsLabel(addLabel)),
        isSemantics(isButton: true, hasTapAction: true),
      );
      expect(dsSemanticsProblems(tester), isEmpty);
      handle.dispose();
    });

    testWidgets('without a label there is no "+"', (tester) async {
      await pumpDs(
        tester,
        _onSidebar(
          DsChannelRow(
            const DsChannelRowData(id: 'c', name: 'general'),
            selected: true,
            onOpen: () {},
          ),
        ),
        density: DsDensityMode.touch,
      );
      expect(_icon(DsIcons.actionNew), findsNothing);
      expect(
        tester.state<DsChannelRowState>(find.byType(DsChannelRow)).addVisible,
        isFalse,
      );
    });

    testWidgets('arrow and "+" keep the row height and the tap target', (
      tester,
    ) async {
      final l = await en();
      for (final (density, tokens) in [
        (DsDensityMode.comfortable, DsDensity.comfortable),
        (DsDensityMode.compact, DsDensity.compact),
        (DsDensityMode.touch, DsDensity.touch),
      ]) {
        await pumpDs(
          tester,
          _onSidebar(
            DsChannelRow(
              const DsChannelRowData(
                id: 'c',
                name: 'requests',
                kind: DsChannelKind.forum,
              ),
              selected: true,
              groupExpanded: false,
              onToggleGroup: (_) {},
              addLabel: l.sidebarNewPostIn('requests'),
              onAdd: () {},
              onOpen: () {},
            ),
          ),
          density: density,
        );
        final height = tester.getSize(find.byType(DsChannelRow)).height;
        // touch: the row grows to density.target-min like every DsTappable (48 > 44).
        final expected = tokens.rowChannel > tokens.targetMin
            ? tokens.rowChannel
            : tokens.targetMin;
        expect(height, expected, reason: density.name);
        for (final button in [
          find.bySemanticsLabel(l.sidebarShowThreads),
          find.bySemanticsLabel(l.sidebarNewPostIn('requests')),
        ]) {
          final size = tester.getSize(button);
          expect(size.width, greaterThanOrEqualTo(tokens.targetMin));
          expect(size.height, greaterThanOrEqualTo(tokens.targetMin));
        }
      }
    });

    testWidgets('a channel row is not draggable', (tester) async {
      await pumpDs(
        tester,
        _onSidebar(
          DsChannelRow(
            const DsChannelRowData(id: 'c', name: 'requests'),
            selected: true,
            groupExpanded: true,
            onToggleGroup: (_) {},
            addLabel: 'New post in requests',
            onAdd: () {},
            onOpen: () {},
          ),
        ),
      );
      expect(_draggable(), findsNothing);
    });
  });

  group('sidebar thread row', () {
    Future<DsSidebarThreadRowState> show(
      WidgetTester tester,
      DsSidebarThreadRowData data, {
      bool selected = false,
      bool last = false,
      VoidCallback? onOpen,
      DsDensityMode density = DsDensityMode.comfortable,
    }) async {
      await pumpDs(
        tester,
        _onSidebar(
          DsSidebarThreadRow(
            data,
            selected: selected,
            last: last,
            onOpen: onOpen ?? () {},
          ),
        ),
        density: density,
      );
      return tester.state<DsSidebarThreadRowState>(
        find.byType(DsSidebarThreadRow),
      );
    }

    testWidgets('the run state icon: running, waiting, failed, else none', (
      tester,
    ) async {
      final c = DsColors.light;
      for (final (state, glyph, color) in [
        (DsRunState.queued, DsIcons.runRunning, c.categorySpruceOnSidebar),
        (DsRunState.running, DsIcons.runRunning, c.categorySpruceOnSidebar),
        (DsRunState.waitingInput, DsIcons.runWaiting, c.categoryGorseOnSidebar),
        (DsRunState.failed, DsIcons.runFailed, c.categoryRowanOnSidebar),
      ]) {
        await show(
          tester,
          DsSidebarThreadRowData(id: 't', title: 'Deploy', runState: state),
        );
        final icon = tester.widget<DsIcon>(_icon(glyph));
        expect(icon.color, color, reason: state.name);
        expect(icon.size, DsIconSizeRole.sm);
      }
      for (final state in <DsRunState?>[
        null,
        DsRunState.completed,
        DsRunState.cancelled,
        DsRunState.unknown,
      ]) {
        await show(
          tester,
          DsSidebarThreadRowData(id: 't', title: 'Deploy', runState: state),
        );
        expect(_glyphs(tester), isEmpty, reason: '$state');
      }
    });

    testWidgets('label: title, state, counts above zero, selected', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      final l = await en();
      await show(
        tester,
        const DsSidebarThreadRowData(
          id: 't',
          title: 'Gateway: resume after network change',
          runState: DsRunState.running,
          unreadCount: 2,
          mentionCount: 1,
        ),
        selected: true,
      );
      final label = [
        'Gateway: resume after network change',
        l.liveBoxStateRunning,
        l.threadRowUnread(2),
        l.threadRowMentions(1),
      ].join(', ');
      expect(
        tester.getSemantics(find.bySemanticsLabel(label)),
        isSemantics(label: label, isSelected: true, hasTapAction: true),
      );
      expect(dsSemanticsProblems(tester), isEmpty);

      for (final (data, state) in [
        (
          const DsSidebarThreadRowData(id: 't', title: 'Export format'),
          l.threadRowStateOpen,
        ),
        (
          const DsSidebarThreadRowData(
            id: 't',
            title: 'Export format',
            done: true,
          ),
          l.threadRowStateDone,
        ),
        (
          const DsSidebarThreadRowData(
            id: 't',
            title: 'Export format',
            runState: DsRunState.waitingInput,
          ),
          l.threadRowStateWaiting,
        ),
        (
          const DsSidebarThreadRowData(
            id: 't',
            title: 'Export format',
            runState: DsRunState.queued,
          ),
          l.liveBoxStateQueued,
        ),
        (
          const DsSidebarThreadRowData(
            id: 't',
            title: 'Export format',
            runState: DsRunState.failed,
          ),
          l.liveBoxStateFailed,
        ),
        // A finished run says nothing beyond the thread's own state.
        (
          const DsSidebarThreadRowData(
            id: 't',
            title: 'Export format',
            runState: DsRunState.completed,
          ),
          l.threadRowStateOpen,
        ),
      ]) {
        await show(tester, data);
        expect(
          find.bySemanticsLabel('Export format, $state'),
          findsOneWidget,
          reason: state,
        );
      }
      await show(
        tester,
        const DsSidebarThreadRowData(
          id: 't',
          title: 'Export format',
          muted: true,
        ),
      );
      expect(
        find.bySemanticsLabel(
          'Export format, ${l.threadRowStateOpen}, ${l.channelRowMuted}',
        ),
        findsOneWidget,
      );
      handle.dispose();
    });

    testWidgets('read, unread, mentions, muted, selected, done', (
      tester,
    ) async {
      final c = DsColors.light;
      var s = await show(
        tester,
        const DsSidebarThreadRowData(id: 't', title: 'Export format'),
      );
      expect(s.nameColor, c.textOnSidebarSecondary);
      expect(s.unreadBar, isFalse);
      expect(s.background, isNull);
      var title = tester.widget<Text>(find.text('\u2068Export format\u2069'));
      expect(title.style?.fontWeight, isNot(FontWeight.w700));
      expect(title.maxLines, 1);
      expect(title.overflow, TextOverflow.ellipsis);
      expect(title.style?.fontSize, DsType.bodySm.fontSize);

      s = await show(
        tester,
        const DsSidebarThreadRowData(
          id: 't',
          title: 'Export format',
          unreadCount: 4,
          mentionCount: 3,
        ),
      );
      expect(s.nameColor, c.textOnSidebar);
      expect(s.unreadBar, isTrue);
      title = tester.widget<Text>(find.text('\u2068Export format\u2069'));
      expect(title.style?.fontWeight, FontWeight.w700);
      expect(find.text('3'), findsOneWidget);
      // Unread is a mark, never a number.
      expect(find.text('4'), findsNothing);

      s = await show(
        tester,
        const DsSidebarThreadRowData(
          id: 't',
          title: 'Export format',
          unreadCount: 4,
          muted: true,
        ),
      );
      expect(s.nameColor, c.textOnSidebarMuted);
      expect(s.unreadBar, isFalse);
      expect(_glyphs(tester), contains(DsIcons.actionMute));

      s = await show(
        tester,
        const DsSidebarThreadRowData(id: 't', title: 'Export format'),
        selected: true,
      );
      expect(s.background, c.surfaceSidebarSelected);

      s = await show(
        tester,
        const DsSidebarThreadRowData(
          id: 't',
          title: 'Export format',
          done: true,
        ),
        selected: true,
      );
      expect(s.nameColor, c.textOnSidebarMuted);
    });

    testWidgets('an empty title reads "New chat" with the automatic mark', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      final l = await en();
      await show(
        tester,
        const DsSidebarThreadRowData(
          id: 't',
          title: '',
          runState: DsRunState.running,
        ),
      );
      expect(find.text('\u2068${l.sidebarNewChat}\u2069'), findsOneWidget);
      expect(find.text(l.threadTitleAutomatic), findsOneWidget);
      expect(
        find.bySemanticsLabel(
          '${l.sidebarNewChat}, ${l.threadTitleAutomatic}, ${l.liveBoxStateRunning}',
        ),
        findsOneWidget,
      );
      expect(dsSemanticsProblems(tester), isEmpty);
      handle.dispose();
    });

    testWidgets('the full title is the tooltip; a titled row has no mark', (
      tester,
    ) async {
      final l = await en();
      const long =
          'Review: grant the deploy key to the build runner of the staging host';
      await show(tester, const DsSidebarThreadRowData(id: 't', title: long));
      expect(find.byTooltip(long), findsOneWidget);
      expect(find.text(l.threadTitleAutomatic), findsNothing);
    });

    testWidgets('a tap and Enter open the thread', (tester) async {
      var opened = 0;
      await show(
        tester,
        const DsSidebarThreadRowData(id: 't', title: 'Export format'),
        onOpen: () => opened++,
      );
      await tester.tap(find.byType(DsSidebarThreadRow));
      expect(opened, 1);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      expect(opened, 2);
      expect(_draggable(), findsNothing);
    });

    testWidgets(
      'height row-channel; the title lines up with the channel name',
      (tester) async {
        for (final (density, tokens) in [
          (DsDensityMode.comfortable, DsDensity.comfortable),
          (DsDensityMode.compact, DsDensity.compact),
        ]) {
          await pumpDs(
            tester,
            _onSidebar(
              Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  DsChannelRow(
                    const DsChannelRowData(id: 'c', name: 'general'),
                    groupExpanded: true,
                    onToggleGroup: (_) {},
                    onOpen: () {},
                  ),
                  DsSidebarThreadRow(
                    const DsSidebarThreadRowData(
                      id: 't',
                      title: 'Export format',
                    ),
                    last: true,
                    onOpen: () {},
                  ),
                ],
              ),
            ),
            density: density,
          );
          expect(
            tester.getSize(find.byType(DsSidebarThreadRow)).height,
            tokens.rowChannel,
            reason: density.name,
          );
          // One step further than the channel row: the thread title starts where the channel
          // name starts, right of the tree line under the channel icon (01 D1 wireframe).
          expect(
            tester.getTopLeft(find.text('\u2068Export format\u2069')).dx,
            tester.getTopLeft(find.text('\u2068general\u2069')).dx,
            reason: density.name,
          );
          expect(
            tester.getTopLeft(find.text('\u2068Export format\u2069')).dx,
            greaterThan(tester.getTopLeft(_icon(DsIcons.channelText)).dx),
          );
        }
      },
    );
  });

  group('sidebar draft row', () {
    Future<DsSidebarDraftRowState> show(
      WidgetTester tester, {
      String title = '',
      DsDraftRowState state = DsDraftRowState.draft,
      bool selected = false,
      bool last = false,
      VoidCallback? onOpen,
      FocusNode? focusNode,
      DsDensityMode density = DsDensityMode.comfortable,
    }) async {
      await pumpDs(
        tester,
        _onSidebar(
          DsSidebarDraftRow(
            title: title,
            state: state,
            selected: selected,
            last: last,
            focusNode: focusNode,
            onOpen: onOpen ?? () {},
          ),
        ),
        density: density,
      );
      return tester.state<DsSidebarDraftRowState>(
        find.byType(DsSidebarDraftRow),
      );
    }

    testWidgets('a blank title reads "New chat", in italics, secondary color', (
      tester,
    ) async {
      final l = await en();
      final c = DsColors.light;
      for (final title in ['', '   ']) {
        final s = await show(tester, title: title);
        final text = tester.widget<Text>(
          find.text('\u2068${l.sidebarNewChat}\u2069'),
        );
        expect(text.style?.fontStyle, FontStyle.italic);
        expect(text.style?.color, c.textOnSidebarSecondary);
        expect(text.style?.fontSize, DsType.bodySm.fontSize);
        expect(text.maxLines, 1);
        expect(text.overflow, TextOverflow.ellipsis);
        expect(s.nameColor, c.textOnSidebarSecondary);
        expect(s.italic, isTrue);
        expect(s.background, isNull);
        expect(find.byTooltip(l.sidebarNewChat), findsOneWidget);
        // A draft is no automatic title: no mark, no run icon, no badge.
        expect(find.text(l.threadTitleAutomatic), findsNothing);
        expect(_glyphs(tester), isEmpty);
        expect(find.byType(DsBadge), findsNothing);
      }
    });

    testWidgets('a draft with a title shows it with the tooltip', (
      tester,
    ) async {
      const long =
          'Review: grant the deploy key to the build runner of the staging host';
      final s = await show(tester, title: long);
      final text = tester.widget<Text>(find.text('\u2068$long\u2069'));
      expect(text.style?.fontStyle, FontStyle.italic);
      expect(s.italic, isTrue);
      expect(find.byTooltip(long), findsOneWidget);
    });

    testWidgets('label per state, the selected flag, one node', (tester) async {
      final handle = tester.ensureSemantics();
      final l = await en();
      for (final (state, word) in [
        (DsDraftRowState.draft, l.sidebarDraftState),
        (DsDraftRowState.sending, l.messageSending),
        (DsDraftRowState.failed, l.sidebarDraftNotSent),
      ]) {
        for (final selected in [false, true]) {
          await show(tester, state: state, selected: selected);
          final label = '${l.sidebarNewChat}, $word';
          expect(
            tester.getSemantics(find.bySemanticsLabel(label)),
            isSemantics(label: label, isSelected: selected, hasTapAction: true),
            reason: '${state.name} $selected',
          );
          // The badge text is part of the row label, not a node of its own.
          expect(
            find.bySemanticsLabel(RegExp('^$word\$')),
            findsNothing,
            reason: state.name,
          );
          expect(dsSemanticsProblems(tester), isEmpty);
        }
      }
      await show(tester, title: 'Export format', state: DsDraftRowState.failed);
      expect(
        find.bySemanticsLabel('Export format, ${l.sidebarDraftNotSent}'),
        findsOneWidget,
      );
      handle.dispose();
    });

    testWidgets('sending shows the badge; failed the icon and the badge', (
      tester,
    ) async {
      final l = await en();
      final c = DsColors.light;
      await show(tester, state: DsDraftRowState.sending);
      var badge = tester.widget<DsBadge>(find.byType(DsBadge));
      expect(badge.text, l.messageSending);
      expect(badge.background, c.surfaceSidebarHover);
      expect(badge.foreground, c.textOnSidebarSecondary);
      expect(_glyphs(tester), isEmpty);

      await show(tester, state: DsDraftRowState.failed);
      badge = tester.widget<DsBadge>(find.byType(DsBadge));
      expect(badge.text, l.sidebarDraftNotSent);
      expect(badge.background, c.surfaceSidebarHover);
      expect(badge.foreground, c.textOnSidebarSecondary);
      final icon = tester.widget<DsIcon>(_icon(DsIcons.runFailed));
      expect(icon.color, c.categoryRowanOnSidebar);
      expect(icon.size, DsIconSizeRole.sm);
      expect(icon.decorative, isTrue);
      // The icon stands before the text, the badge after it.
      final text = find.text('\u2068${l.sidebarNewChat}\u2069');
      expect(
        tester.getTopLeft(_icon(DsIcons.runFailed)).dx,
        lessThan(tester.getTopLeft(text).dx),
      );
      expect(
        tester.getTopLeft(find.byType(DsBadge)).dx,
        greaterThan(tester.getTopRight(text).dx),
      );
    });

    testWidgets('selected background; a tap and Enter open the draft', (
      tester,
    ) async {
      final c = DsColors.light;
      var opened = 0;
      final s = await show(tester, selected: true, onOpen: () => opened++);
      expect(s.background, c.surfaceSidebarSelected);
      await tester.tap(find.byType(DsSidebarDraftRow));
      expect(opened, 1);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      expect(opened, 2);
      expect(_draggable(), findsNothing);
    });

    testWidgets('a focus node of the caller focuses the row', (tester) async {
      final node = FocusNode();
      addTearDown(node.dispose);
      var opened = 0;
      await show(tester, focusNode: node, onOpen: () => opened++);
      node.requestFocus();
      await tester.pump();
      expect(node.hasPrimaryFocus, isTrue);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      expect(opened, 1);
    });

    testWidgets('height, indent and tree line of the thread row', (
      tester,
    ) async {
      for (final (density, tokens) in [
        (DsDensityMode.comfortable, DsDensity.comfortable),
        (DsDensityMode.compact, DsDensity.compact),
      ]) {
        await pumpDs(
          tester,
          _onSidebar(
            Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                DsSidebarDraftRow(title: 'Draft title', onOpen: () {}),
                DsSidebarThreadRow(
                  const DsSidebarThreadRowData(id: 't', title: 'Export format'),
                  last: true,
                  onOpen: () {},
                ),
              ],
            ),
          ),
          density: density,
        );
        expect(
          tester.getSize(find.byType(DsSidebarDraftRow)).height,
          tokens.rowChannel,
          reason: density.name,
        );
        expect(
          tester.getTopLeft(find.text('\u2068Draft title\u2069')).dx,
          tester.getTopLeft(find.text('\u2068Export format\u2069')).dx,
          reason: density.name,
        );
        // The same tree line frame as the thread row below it.
        final paints = tester.widgetList<CustomPaint>(
          find.descendant(
            of: find.byType(DsSidebarDraftRow),
            matching: find.byType(CustomPaint),
          ),
        );
        expect(
          paints.where((p) => '${p.painter.runtimeType}' == '_TreeLinePainter'),
          hasLength(1),
        );
      }
    });
  });

  group('all posts row', () {
    testWidgets('text, marks of the posts not shown, never a total', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      final l = await en();
      final c = DsColors.light;
      var opened = 0;
      Future<DsSidebarAllPostsRowState> show({
        int unread = 0,
        int mentions = 0,
        bool selected = false,
      }) async {
        await pumpDs(
          tester,
          _onSidebar(
            DsSidebarAllPostsRow(
              unreadCount: unread,
              mentionCount: mentions,
              selected: selected,
              onOpen: () => opened++,
            ),
          ),
        );
        return tester.state<DsSidebarAllPostsRowState>(
          find.byType(DsSidebarAllPostsRow),
        );
      }

      var s = await show();
      expect(find.text(l.sidebarAllPosts), findsOneWidget);
      expect(s.nameColor, c.textOnSidebarSecondary);
      expect(s.unreadBar, isFalse);
      expect(find.bySemanticsLabel(l.sidebarAllPosts), findsOneWidget);

      s = await show(unread: 5, mentions: 2);
      expect(s.nameColor, c.textOnSidebar);
      expect(s.unreadBar, isTrue);
      final text = tester.widget<Text>(find.text(l.sidebarAllPosts));
      expect(text.style?.fontWeight, FontWeight.w700);
      expect(find.text('2'), findsOneWidget);
      expect(find.text('5'), findsNothing);
      final label = [
        l.sidebarAllPosts,
        l.threadRowUnread(5),
        l.threadRowMentions(2),
      ].join(', ');
      expect(
        tester.getSemantics(find.bySemanticsLabel(label)),
        isSemantics(label: label, isSelected: false, hasTapAction: true),
      );
      expect(dsSemanticsProblems(tester), isEmpty);
      await tester.tap(find.byType(DsSidebarAllPostsRow));
      expect(opened, 1);

      s = await show(selected: true);
      expect(s.background, c.surfaceSidebarSelected);
      expect(_draggable(), findsNothing);
      handle.dispose();
    });
  });

  group('show more row (01 6.8 item 3, M4-46)', () {
    testWidgets(
      'reads Show more or Show less with the count, toggles on tap and is no drag source',
      (tester) async {
        final handle = tester.ensureSemantics();
        final l = await en();
        final c = DsColors.light;
        var toggled = 0;
        Future<void> show({required bool expanded}) => pumpDs(
          tester,
          _onSidebar(
            DsSidebarMoreRow(
              hiddenCount: 9,
              expanded: expanded,
              onToggle: () => toggled++,
            ),
          ),
        );

        await show(expanded: false);
        expect(find.text(l.sidebarShowMore), findsOneWidget);
        final text = tester.widget<Text>(find.text(l.sidebarShowMore));
        expect(text.style?.color, c.textOnSidebarSecondary);
        final label = '${l.sidebarShowMore}, ${l.sidebarMorePosts(9)}';
        expect(
          tester.getSemantics(find.bySemanticsLabel(label)),
          isSemantics(label: label, hasTapAction: true, isButton: true),
        );
        expect(dsSemanticsProblems(tester), isEmpty);
        await tester.tap(find.byType(DsSidebarMoreRow));
        expect(toggled, 1);

        await show(expanded: true);
        expect(find.text(l.sidebarShowLess), findsOneWidget);
        expect(
          find.bySemanticsLabel(
            '${l.sidebarShowLess}, ${l.sidebarMorePosts(9)}',
          ),
          findsOneWidget,
        );
        expect(_draggable(), findsNothing);
        handle.dispose();
      },
    );

    testWidgets('reads German', (tester) async {
      final l = await de();
      await pumpDs(
        tester,
        _onSidebar(
          DsSidebarMoreRow(hiddenCount: 1, expanded: false, onToggle: () {}),
        ),
        locale: const Locale('de'),
      );
      expect(l.sidebarShowMore, 'Mehr anzeigen');
      expect(l.sidebarShowLess, 'Weniger anzeigen');
      expect(find.text('Mehr anzeigen'), findsOneWidget);
    });
  });

  testWidgets('a whole group reads cleanly in German', (tester) async {
    final handle = tester.ensureSemantics();
    final l = await de();
    await pumpDs(
      tester,
      _onSidebar(
        Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            DsChannelRow(
              const DsChannelRowData(
                id: 'c',
                name: 'server',
                kind: DsChannelKind.forum,
                projectBound: true,
              ),
              selected: true,
              groupExpanded: true,
              onToggleGroup: (_) {},
              addLabel: l.sidebarNewChatIn('server'),
              onAdd: () {},
              onOpen: () {},
            ),
            DsSidebarThreadRow(
              const DsSidebarThreadRowData(
                id: 't1',
                title: '',
                runState: DsRunState.running,
              ),
              onOpen: () {},
            ),
            DsSidebarThreadRow(
              const DsSidebarThreadRowData(
                id: 't2',
                title: 'Check migration 0042',
                runState: DsRunState.failed,
                unreadCount: 1,
              ),
              onOpen: () {},
            ),
            DsSidebarAllPostsRow(mentionCount: 1, onOpen: () {}),
          ],
        ),
      ),
      locale: const Locale('de'),
    );
    expect(find.text(l.sidebarAllPosts), findsOneWidget);
    expect(find.text(l.threadTitleAutomatic), findsOneWidget);
    expect(find.bySemanticsLabel(l.sidebarNewChatIn('server')), findsOneWidget);
    final problems = dsSemanticsProblems(tester);
    expect(problems, isEmpty, reason: problems.join('\n'));
    handle.dispose();
  });

  group('review fixes', () {
    /// [child] at text scale 2 in the width a row gets in the app's sidebar column
    /// (`layout.sidebar-width` minus the paddings of the space tree and the channel entry).
    Future<void> narrow(
      WidgetTester tester,
      Widget child, {
      Locale locale = const Locale('de'),
    }) => pumpDs(
      tester,
      Builder(
        builder: (context) => MediaQuery(
          data: MediaQuery.of(context)
              .copyWith(textScaler: const TextScaler.linear(2)),
          child: _onSidebar(child),
        ),
      ),
      width: 220,
      locale: locale,
    );

    for (final locale in [const Locale('de'), const Locale('en')]) {
      testWidgets(
        'an untitled thread row fits a narrow sidebar at text scale 2 '
        '(${locale.languageCode})',
        (tester) async {
          await narrow(
            tester,
            DsSidebarThreadRow(
              const DsSidebarThreadRowData(
                id: 't',
                title: '',
                runState: DsRunState.running,
                mentionCount: 1,
              ),
              onOpen: () {},
            ),
            locale: locale,
          );
          expect(tester.takeException(), isNull);
          final l = locale.languageCode == 'de' ? await de() : await en();
          expect(
            tester.getSize(find.text('\u2068${l.sidebarNewChat}\u2069')).width,
            greaterThan(0),
          );
        },
      );
    }

    for (final locale in [const Locale('de'), const Locale('en')]) {
      for (final state in DsDraftRowState.values) {
        testWidgets(
          'the draft row (${state.name}) fits a narrow sidebar at text scale 2 '
          '(${locale.languageCode})',
          (tester) async {
            for (final title in ['', 'Gateway: resume after network change']) {
              await narrow(
                tester,
                DsSidebarDraftRow(
                  title: title,
                  state: state,
                  selected: true,
                  onOpen: () {},
                ),
                locale: locale,
              );
              expect(tester.takeException(), isNull, reason: title);
              final l = locale.languageCode == 'de' ? await de() : await en();
              final text = title.isEmpty ? l.sidebarNewChat : title;
              expect(
                tester.getSize(find.text('\u2068$text\u2069')).width,
                greaterThan(0),
              );
            }
          },
        );
      }
    }

    testWidgets('a full channel row and thread row fit at text scale 2', (
      tester,
    ) async {
      await narrow(
        tester,
        Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            DsChannelRow(
              const DsChannelRowData(
                id: 'c',
                name: 'server',
                kind: DsChannelKind.forum,
                projectBound: true,
                unreadCount: 3,
                mentionCount: 12,
                runActive: true,
                muted: true,
              ),
              selected: true,
              groupExpanded: true,
              onToggleGroup: (_) {},
              addLabel: 'New chat in server',
              onAdd: () {},
              onOpen: () {},
            ),
            DsSidebarThreadRow(
              const DsSidebarThreadRowData(
                id: 't',
                title: 'Gateway: resume after network change',
                runState: DsRunState.waitingInput,
                unreadCount: 2,
                mentionCount: 12,
                muted: true,
              ),
              onOpen: () {},
            ),
            DsSidebarAllPostsRow(
              unreadCount: 5,
              mentionCount: 12,
              onOpen: () {},
            ),
          ],
        ),
      );
      expect(tester.takeException(), isNull);
    });

    testWidgets('a focus node of the caller focuses each row', (tester) async {
      final channel = FocusNode();
      final thread = FocusNode();
      final allPosts = FocusNode();
      addTearDown(channel.dispose);
      addTearDown(thread.dispose);
      addTearDown(allPosts.dispose);
      final opened = <String>[];
      await pumpDs(
        tester,
        _onSidebar(
          Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              DsChannelRow(
                const DsChannelRowData(
                  id: 'c',
                  name: 'server',
                  kind: DsChannelKind.forum,
                ),
                focusNode: channel,
                groupExpanded: true,
                onToggleGroup: (_) {},
                addLabel: 'New post in server',
                onAdd: () {},
                onOpen: () => opened.add('channel'),
              ),
              DsSidebarThreadRow(
                const DsSidebarThreadRowData(id: 't', title: 'Export format'),
                focusNode: thread,
                onOpen: () => opened.add('thread'),
              ),
              DsSidebarAllPostsRow(
                focusNode: allPosts,
                onOpen: () => opened.add('all'),
              ),
            ],
          ),
        ),
      );
      channel.requestFocus();
      await tester.pump();
      expect(channel.hasPrimaryFocus, isTrue);
      expect(
        tester.state<DsChannelRowState>(find.byType(DsChannelRow)).addVisible,
        isTrue,
      );
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      thread.requestFocus();
      await tester.pump();
      expect(thread.hasPrimaryFocus, isTrue);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      allPosts.requestFocus();
      await tester.pump();
      expect(allPosts.hasPrimaryFocus, isTrue);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      expect(opened, ['channel', 'thread', 'all']);
    });

    testWidgets('a click in the empty leading slot opens the channel', (
      tester,
    ) async {
      var opened = 0;
      await pumpDs(
        tester,
        _onSidebar(
          DsChannelRow(
            const DsChannelRowData(id: 'c', name: 'releases'),
            onOpen: () => opened++,
          ),
        ),
      );
      final row = tester.getRect(find.byType(DsChannelRow));
      await tester.tapAt(Offset(row.left + 4, row.center.dy));
      expect(opened, 1);
    });

    testWidgets('"+" shows on the forum of the open post (01 D1)', (
      tester,
    ) async {
      await pumpDs(
        tester,
        _onSidebar(
          DsChannelRow(
            const DsChannelRowData(
              id: 'c',
              name: 'server',
              kind: DsChannelKind.forum,
              projectBound: true,
            ),
            current: true,
            groupExpanded: true,
            onToggleGroup: (_) {},
            addLabel: 'New chat in server',
            onAdd: () {},
            onOpen: () {},
          ),
        ),
      );
      final state = tester.state<DsChannelRowState>(find.byType(DsChannelRow));
      expect(state.addVisible, isTrue);
      // The open post is the selected row; the forum row keeps its plain background.
      expect(state.background, isNull);
      expect(
        tester.getSemantics(find.bySemanticsLabel(RegExp('^server, '))),
        isSemantics(isSelected: false),
      );
    });
  });

  group('space row action (01 6.9 item 1, M4-43)', () {
    testWidgets('"+" shows on hover, focus and touch and starts the action', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      const label = 'New project in Engineering';
      var added = 0;
      Future<DsSpaceRowState> show({
        DsDensityMode density = DsDensityMode.comfortable,
        bool withAction = true,
      }) async {
        await pumpDs(
          tester,
          _onSidebar(
            DsSpaceRow(
              const DsSpaceRowData(
                id: 's',
                name: 'Engineering',
                icon: 'headset',
                category: DsCategory.loch,
              ),
              onToggle: (_) {},
              addLabel: withAction ? label : null,
              onAdd: withAction ? () => added++ : null,
            ),
          ),
          density: density,
        );
        return tester.state<DsSpaceRowState>(find.byType(DsSpaceRow));
      }

      var state = await show();
      expect(state.addVisible, isFalse);
      expect(find.bySemanticsLabel(label), findsNothing);
      expect(_icon(DsIcons.actionNew), findsNothing);

      final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
      await mouse.addPointer(location: Offset.zero);
      addTearDown(mouse.removePointer);
      await mouse.moveTo(tester.getCenter(find.byType(DsSpaceRow)));
      await tester.pump();
      expect(state.addVisible, isTrue);
      expect(find.byTooltip(label), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(label));
      expect(added, 1);
      await mouse.moveTo(const Offset(590, 500));
      await tester.pump();
      expect(state.addVisible, isFalse);

      // Keyboard: the row, then Tab reaches the action and Enter starts it.
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.pump();
      expect(state.addVisible, isTrue);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      expect(added, 2);
      primaryFocus?.unfocus();
      await tester.pump();

      state = await show(density: DsDensityMode.touch);
      expect(state.addVisible, isTrue);
      expect(
        tester.getSemantics(find.bySemanticsLabel(label)),
        isSemantics(isButton: true, hasTapAction: true),
      );

      state = await show(withAction: false, density: DsDensityMode.touch);
      expect(state.addVisible, isFalse);
      expect(dsSemanticsProblems(tester), isEmpty);
      handle.dispose();
    });
  });
}
