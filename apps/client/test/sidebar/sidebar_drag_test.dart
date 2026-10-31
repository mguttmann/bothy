// SPDX-License-Identifier: Apache-2.0

// Dragging in the sidebar (brief M4-51; 01 6.15; D-M4-51-6, D-M4-51-7): on the desktop a mouse
// drag moves a space, a channel within its space and an open post within its forum, with a line
// where the row lands; on a touch screen a long press lifts the row and a swipe does not; a row
// never lands in another parent; at an automatic level nothing moves and a note says why; the
// row menus and the screen reader actions move one place up or down.

import 'package:client/sidebar/sidebar_arrange.dart';
import 'package:client/sidebar/sidebar_column.dart';
import 'package:client/sidebar/sidebar_model.dart';
import 'package:client/sidebar/sidebar_order.dart';
import 'package:client/sidebar/sidebar_threads.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';

SidebarChannel entry(Channel c, {List<Thread> posts = const []}) =>
    SidebarChannel(
      channel: c,
      counts: ThreadCounts.zero,
      runActive: false,
      muted: false,
      group: posts.isEmpty
          ? null
          : SidebarThreadGroup(
              rows: [
                for (final p in posts)
                  SidebarThread(thread: p, counts: ThreadCounts.zero),
              ],
              candidates: [
                for (final p in posts)
                  SidebarThread(thread: p, counts: ThreadCounts.zero),
              ],
              channelCounts: ThreadCounts.zero,
              allPostsCounts: null,
            ),
    );

/// Space 1 with the text channel 1 and the forums 2 (posts 1 to 3) and 3; space 2 with channel 4.
List<SidebarSpaceNode> nodes() => [
  SidebarSpaceNode(
    entry: SidebarSpace(space: space(1)),
    channels: [
      entry(channel(1, name: 'general')),
      entry(
        channel(2, kind: ChannelKind.forum, name: 'server'),
        posts: [
          thread(1, channelN: 2, title: 'First'),
          thread(2, channelN: 2, title: 'Second'),
          thread(3, channelN: 2, title: 'Third'),
        ],
      ),
      entry(channel(3, kind: ChannelKind.forum, name: 'client')),
    ],
  ),
  SidebarSpaceNode(
    entry: SidebarSpace(space: space(2)),
    channels: [entry(channel(4, spaceN: 2, name: 'ops'))],
  ),
];

final class Moves {
  Moves(this.host);

  /// The host of the view; disposed at the end of the test.
  final ColumnHost host;

  final List<(SpaceId, int)> spaces = [];
  final List<(SpaceId, ChannelId, int)> channels = [];
  final List<(ChannelId, ThreadId, int)> threads = [];
}

Future<Moves> pumpSidebar(
  WidgetTester tester, {
  SidebarArrangement arrangement = const SidebarArrangement(autoChats: false),
  SidebarHoldMode holdMode = SidebarHoldMode.pointer,
  Locale locale = const Locale('en'),
}) async {
  final moves = Moves(await ColumnHost.open(tester));
  await moves.host.pump(
    tester,
    SidebarView(
      productName: 'Team Chat',
      nodes: nodes(),
      onToggle: (_, _) {},
      onMove: (s, to) => moves.spaces.add((s, to)),
      onOpenChannel: (_) {},
      onSettings: () {},
      onLogout: () {},
      onOpenThread: (_) {},
      onToggleGroup: (_, _) {},
      holdMode: holdMode,
      arrangement: arrangement,
      onMoveChannel: (s, c, to) => moves.channels.add((s, c, to)),
      onMoveThread: (f, t, to) => moves.threads.add((f, t, to)),
    ),
    locale: locale,
  );
  return moves;
}

Finder channelRow(int n) =>
    find.byKey(ValueKey('sidebar.channel.${channelId(n).value}'));
Finder postRow(int n) =>
    find.byKey(ValueKey('sidebar.thread.${threadId(n).value}'));
Finder spaceRow(int n) =>
    find.byKey(ValueKey('sidebar.space.${spaceId(n).value}'));

/// Drags [from] with the mouse to the lower quarter of [to] (it lands below [to]) and returns
/// the gesture before the button goes up, so a test can look at the drop line.
Future<TestGesture> mouseDragOnto(
  WidgetTester tester,
  Finder from,
  Finder to, {
  bool below = true,
}) async {
  final g = await tester.startGesture(
    tester.getCenter(from),
    kind: PointerDeviceKind.mouse,
  );
  await tester.pump(const Duration(milliseconds: 20));
  final rect = tester.getRect(to);
  final target = Offset(
    rect.center.dx,
    below ? rect.bottom - rect.height / 4 : rect.top + rect.height / 4,
  );
  final steps = 10;
  final start = tester.getCenter(from);
  for (var i = 1; i <= steps; i++) {
    await g.moveTo(Offset.lerp(start, target, i / steps)!);
    await tester.pump(const Duration(milliseconds: 16));
  }
  return g;
}

void main() {
  testWidgets('desktop: a mouse drag moves a channel within its space, with a '
      'drop line', (tester) async {
    final moves = await pumpSidebar(tester);
    final g = await mouseDragOnto(tester, channelRow(1), channelRow(3));
    expect(
      find.byKey(ValueKey('sidebar.dropLine.${channelId(3).value}.below')),
      findsOneWidget,
    );
    await g.up();
    await tester.pumpAndSettle();
    expect(moves.channels, [(spaceId(1), channelId(1), 2)]);
    expect(
      find.byKey(ValueKey('sidebar.dropLine.${channelId(3).value}.below')),
      findsNothing,
    );
    await moves.host.dispose(tester);
  });

  testWidgets('desktop: dropped above a row lands before it', (tester) async {
    final moves = await pumpSidebar(tester);
    final g = await mouseDragOnto(
      tester,
      channelRow(3),
      channelRow(1),
      below: false,
    );
    expect(
      find.byKey(ValueKey('sidebar.dropLine.${channelId(1).value}.above')),
      findsOneWidget,
    );
    await g.up();
    await tester.pumpAndSettle();
    expect(moves.channels, [(spaceId(1), channelId(3), 0)]);
    await moves.host.dispose(tester);
  });

  testWidgets('desktop: a mouse drag moves an open post within its forum', (
    tester,
  ) async {
    final moves = await pumpSidebar(tester);
    final g = await mouseDragOnto(tester, postRow(3), postRow(1), below: false);
    await g.up();
    await tester.pumpAndSettle();
    expect(moves.threads, [(channelId(2), threadId(3), 0)]);
    expect(moves.channels, isEmpty, reason: 'the forum itself stays');
    await moves.host.dispose(tester);
  });

  testWidgets('a channel never lands in another space, a post never in a '
      'channel', (tester) async {
    final moves = await pumpSidebar(tester);
    var g = await mouseDragOnto(tester, channelRow(1), channelRow(4));
    await g.up();
    await tester.pumpAndSettle();
    g = await mouseDragOnto(tester, postRow(1), channelRow(3));
    await g.up();
    await tester.pumpAndSettle();
    expect(moves.channels, isEmpty);
    expect(moves.threads, isEmpty);
    await moves.host.dispose(tester);
  });

  testWidgets('desktop: a mouse drag still moves a space', (tester) async {
    final moves = await pumpSidebar(tester);
    final g = await mouseDragOnto(tester, spaceRow(1), channelRow(4));
    await g.up();
    await tester.pumpAndSettle();
    expect(moves.spaces, [(spaceId(1), 1)]);
    await moves.host.dispose(tester);
  });

  testWidgets('touch: a swipe moves nothing; a long press lifts the row, then '
      'it is dragged', (tester) async {
    final moves = await pumpSidebar(tester, holdMode: SidebarHoldMode.touch);
    // A swipe without the long press.
    final swipe = await tester.startGesture(tester.getCenter(channelRow(1)));
    await swipe.moveBy(const Offset(0, 120));
    await tester.pump();
    await swipe.up();
    await tester.pumpAndSettle();
    expect(moves.channels, isEmpty);

    final g = await tester.startGesture(tester.getCenter(channelRow(1)));
    await tester.pump(kLongPressTimeout + const Duration(milliseconds: 100));
    final rect = tester.getRect(channelRow(3));
    final start = tester.getCenter(channelRow(1));
    final target = Offset(rect.center.dx, rect.bottom - rect.height / 4);
    for (var i = 1; i <= 10; i++) {
      await g.moveTo(Offset.lerp(start, target, i / 10)!);
      await tester.pump(const Duration(milliseconds: 16));
    }
    await g.up();
    await tester.pumpAndSettle();
    expect(moves.channels, [(spaceId(1), channelId(1), 2)]);
    await moves.host.dispose(tester);
  });

  testWidgets('touch: a long press lifts a space too', (tester) async {
    final moves = await pumpSidebar(tester, holdMode: SidebarHoldMode.touch);
    final g = await tester.startGesture(tester.getCenter(spaceRow(2)));
    await tester.pump(kLongPressTimeout + const Duration(milliseconds: 100));
    final start = tester.getCenter(spaceRow(2));
    final target = tester.getTopLeft(spaceRow(1)) - const Offset(0, 4);
    for (var i = 1; i <= 10; i++) {
      await g.moveTo(Offset.lerp(start, Offset(start.dx, target.dy), i / 10)!);
      await tester.pump(const Duration(milliseconds: 16));
    }
    await g.up();
    await tester.pumpAndSettle();
    expect(moves.spaces, [(spaceId(2), 0)]);
    await moves.host.dispose(tester);
  });

  for (final (level, row, hint) in [
    (SidebarLevel.spaces, spaceRow(1), 'Spaces are arranged by activity'),
    (
      SidebarLevel.channels,
      channelRow(1),
      'Channels and projects are arranged by activity',
    ),
    (SidebarLevel.chats, postRow(1), 'Chats are arranged by activity'),
  ]) {
    testWidgets('automatic ${level.name}: a drag moves nothing and shows the '
        'note', (tester) async {
      final moves = await pumpSidebar(
        tester,
        arrangement: SidebarArrangement(
          autoSpaces: level == SidebarLevel.spaces,
          autoChannels: level == SidebarLevel.channels,
          autoChats: level == SidebarLevel.chats,
        ),
      );
      final g = await mouseDragOnto(tester, row, channelRow(3));
      await g.up();
      await tester.pumpAndSettle();
      expect(moves.spaces, isEmpty);
      expect(moves.channels, isEmpty);
      expect(moves.threads, isEmpty);
      expect(find.textContaining(hint), findsOneWidget);
      await moves.host.dispose(tester);
    });
  }

  testWidgets('desktop, default settings: a small mouse drag on a post never '
      'moves its forum and shows the note (review of M4-51)', (tester) async {
    final moves = await pumpSidebar(
      tester,
      arrangement: const SidebarArrangement(),
    );
    final g = await tester.startGesture(
      tester.getCenter(postRow(2)),
      kind: PointerDeviceKind.mouse,
    );
    for (var i = 0; i < 3; i++) {
      await g.moveBy(const Offset(0, 1));
      await tester.pump(const Duration(milliseconds: 16));
    }
    final target = tester.getCenter(channelRow(3));
    final start = tester.getCenter(postRow(2));
    for (var i = 1; i <= 10; i++) {
      await g.moveTo(Offset.lerp(start, target, i / 10)!);
      await tester.pump(const Duration(milliseconds: 16));
    }
    await g.up();
    await tester.pumpAndSettle();
    expect(moves.channels, isEmpty);
    expect(moves.threads, isEmpty);
    expect(
      find.textContaining('Chats are arranged by activity'),
      findsOneWidget,
    );
    await moves.host.dispose(tester);
  });

  testWidgets('automatic chats on touch: a long press shows the note', (
    tester,
  ) async {
    final moves = await pumpSidebar(
      tester,
      arrangement: const SidebarArrangement(),
      holdMode: SidebarHoldMode.touch,
      locale: const Locale('de'),
    );
    await tester.longPress(postRow(2));
    await tester.pumpAndSettle();
    expect(
      // language-check: off
      find.textContaining('Chats werden nach Aktivität angeordnet'),
      // language-check: on
      findsOneWidget,
    );
    await moves.host.dispose(tester);
  });

  Future<void> openMenu(WidgetTester tester, Finder row) async {
    await tester.tapAt(
      tester.getCenter(row),
      buttons: kSecondaryMouseButton,
      kind: PointerDeviceKind.mouse,
    );
    await tester.pumpAndSettle();
  }

  testWidgets('keyboard: the channel menu moves the channel one place down '
      'and up; the ends are disabled', (tester) async {
    final moves = await pumpSidebar(tester);
    await openMenu(tester, channelRow(2));
    await tester.tap(find.byKey(const ValueKey('menu.moveDown')));
    await tester.pumpAndSettle();
    await openMenu(tester, channelRow(2));
    await tester.tap(find.byKey(const ValueKey('menu.moveUp')));
    await tester.pumpAndSettle();
    expect(moves.channels, [
      (spaceId(1), channelId(2), 2),
      (spaceId(1), channelId(2), 0),
    ]);
    await openMenu(tester, channelRow(1));
    final up = tester.widget<MenuItemButton>(
      find.ancestor(
        of: find.text('Move up'),
        matching: find.byType(MenuItemButton),
      ),
    );
    expect(up.onPressed, isNull);
    await moves.host.dispose(tester);
  });

  testWidgets('keyboard: the post menu moves a post; the space menu a space', (
    tester,
  ) async {
    final moves = await pumpSidebar(tester);
    await openMenu(tester, postRow(1));
    await tester.tap(find.byKey(const ValueKey('menu.moveDown')));
    await tester.pumpAndSettle();
    expect(moves.threads, [(channelId(2), threadId(1), 1)]);
    await openMenu(tester, spaceRow(1));
    await tester.tap(find.byKey(const ValueKey('menu.moveDown')));
    await tester.pumpAndSettle();
    expect(moves.spaces, [(spaceId(1), 1)]);
    await moves.host.dispose(tester);
  });

  testWidgets('keyboard: at an automatic level the menu says why and both '
      'moves are disabled', (tester) async {
    final moves = await pumpSidebar(
      tester,
      arrangement: const SidebarArrangement(autoChannels: true),
    );
    await openMenu(tester, channelRow(2));
    expect(find.text('Arranged by activity'), findsOneWidget);
    for (final label in ['Move up', 'Move down']) {
      final item = tester.widget<MenuItemButton>(
        find.ancestor(
          of: find.text(label),
          matching: find.byType(MenuItemButton),
        ),
      );
      expect(item.onPressed, isNull);
    }
    expect(moves.channels, isEmpty);
    await moves.host.dispose(tester);
  });

  testWidgets('screen readers: a channel row and a post row have "Move up" '
      'and "Move down"', (tester) async {
    final semantics = tester.ensureSemantics();
    final moves = await pumpSidebar(tester);
    void perform(Finder row, String label) {
      final node = tester.getSemantics(row);
      final ids = node.getSemanticsData().customSemanticsActionIds ?? const [];
      final action = ids
          .map(CustomSemanticsAction.getAction)
          .singleWhere((a) => a?.label == label)!;
      node.owner!.performAction(
        node.id,
        SemanticsAction.customAction,
        CustomSemanticsAction.getIdentifier(action),
      );
    }

    perform(channelRow(2), 'Move down');
    perform(postRow(2), 'Move up');
    await tester.pump();
    expect(moves.channels, [(spaceId(1), channelId(2), 2)]);
    expect(moves.threads, [(channelId(2), threadId(2), 0)]);
    semantics.dispose();
    await moves.host.dispose(tester);
  });
}
