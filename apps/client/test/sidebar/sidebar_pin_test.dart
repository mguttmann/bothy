// SPDX-License-Identifier: Apache-2.0

// The personal pins of the sidebar (01 6.24, FEAT-PIN-CHANNELS) over the local database: "Pin to
// top" in the menu of a channel (on the desktop by right click, on the phone by a long press) and
// in the More menu of its header writes `pinned` through the outbox and moves the row to the top
// of its space at once, also while "Arrange channels and projects automatically" is on and new
// activity reorders the rest; several pins keep the order of their pin times; "Unpin" returns the
// row to its place; a pin of another device applies; a pinned row neither moves nor lifts in a
// manual space; a forum post pins to the top of its forum the same way (`pinned_threads`).

import 'dart:convert';

import 'package:client/menus/thread_menu.dart';
import 'package:client/sidebar/sidebar_column.dart';
import 'package:design_system/design_system.dart'
    show DsChannelRow, DsSidebarThreadRow;
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/structure.dart';

/// The sidebar at its usual width at the left of the window, so the pointer can leave it.
const Widget sidebar = Align(
  alignment: Alignment.topLeft,
  child: SizedBox(width: 280, child: SidebarColumn()),
);

Finder channelRow(int n) =>
    find.byKey(ValueKey('sidebar.channel.${channelId(n).value}'));
Finder postRow(int n) =>
    find.byKey(ValueKey('sidebar.thread.${threadId(n).value}'));
final Finder pinEntry = find.byKey(const ValueKey('menu.pin'));
final Finder unpinEntry = find.byKey(const ValueKey('menu.unpin'));

double top(WidgetTester tester, Finder f) => tester.getTopLeft(f).dy;

/// The order on screen of the rows [rows] finds by number, top to bottom.
List<int> onScreen(
  WidgetTester tester,
  Finder Function(int) row,
  List<int> ns,
) {
  final present = {
    for (final n in ns)
      if (row(n).evaluate().isNotEmpty) n: top(tester, row(n)),
  };
  return present.keys.toList()
    ..sort((a, b) => present[a]!.compareTo(present[b]!));
}

List<int> channelsOnScreen(WidgetTester tester) =>
    onScreen(tester, channelRow, [1, 2, 3, 4]);

/// Space 1 with the forums 1, 2 and 3 and the text channel 4 "team" (global order 1 to 4);
/// forum n holds post n, last active 10 times n minutes after [t0]; forum 1 also holds post 5,
/// last active 5 minutes after [t0].
Future<void> fill(
  LocalDatabase db, {
  Map<String, Object> settings = const {},
}) async {
  await db.structureDao.upsertSpaceIfNewer(space(1));
  for (final n in [1, 2, 3]) {
    await db.structureDao.upsertChannelIfNewer(
      channel(n, kind: ChannelKind.forum, name: 'forum-$n'),
    );
    await db.threadsDao.upsertIfNewer(
      thread(n, channelN: n, activeMinutes: 10 * n),
    );
  }
  await db.threadsDao.upsertIfNewer(thread(5, channelN: 1, activeMinutes: 5));
  await db.structureDao.upsertChannelIfNewer(channel(4, name: 'team'));
  for (final e in settings.entries) {
    await db.personalDao.upsertSettingIfNewer(
      UserSetting(
        key: e.key,
        value: e.value,
        version: const ResourceVersion(1),
      ),
    );
  }
}

Future<List<Map<String, Object?>>> outbox(
  WidgetTester tester,
  LocalDatabase db,
) async => [
  for (final e in (await tester.runAsync(db.outboxDao.allEntries))!)
    {'kind': e.kind, ...jsonDecode(e.payload) as Map<String, Object?>},
];

Future<void> rightClick(
  WidgetTester tester,
  ColumnHost host,
  Finder row,
) async {
  await tester.tapAt(
    tester.getCenter(row),
    buttons: kSecondaryMouseButton,
    kind: PointerDeviceKind.mouse,
  );
  await host.settle(tester);
}

Future<UserChannelState?> stateOf(
  WidgetTester tester,
  ColumnHost host,
  int n,
) async => (await tester.runAsync(
  () => host.db.structureDao.watchUserChannelStates().first,
))!.where((s) => s.channelId == channelId(n)).firstOrNull;

/// A pin of channel [n] written by another device of the person at [minutes] after [t0].
Future<void> pinElsewhere(
  WidgetTester tester,
  ColumnHost host,
  int n, {
  required int minutes,
  int version = 1,
  bool pinned = true,
}) => host.run(
  tester,
  () => host.db.structureDao.upsertUserChannelStateIfNewer(
    UserChannelState(
      channelId: channelId(n),
      threadOrder: const [],
      version: ResourceVersion(version),
      pinnedAt: pinned ? t0.add(Duration(minutes: minutes)) : null,
    ),
  ),
);

void main() {
  late AppLocalizations l;
  setUpAll(() async {
    l = await AppLocalizations.delegate.load(const Locale('en'));
  });

  testWidgets('"Pin to top" pins a channel above the automatic order through '
      'the outbox; new activity reorders only the rest; "Unpin" returns it', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(
      () => fill(host.db, settings: {'sidebar.auto_arrange_channels': true}),
    );
    await host.pump(tester, sidebar);
    // One mouse for the whole test: it right-clicks rows and then leaves the sidebar.
    final mouse = await tester.createGesture(
      kind: PointerDeviceKind.mouse,
      buttons: kSecondaryMouseButton,
    );
    final size = tester.view.physicalSize / tester.view.devicePixelRatio;
    final outside = Offset(size.width - 2, size.height - 2);
    await mouse.addPointer(location: outside);
    Future<void> click(Finder row) async {
      await mouse.moveTo(tester.getCenter(row));
      await mouse.down(tester.getCenter(row));
      await mouse.up();
      await host.settle(tester);
    }

    // By activity: forum 3, 2, 1, then the team channel without posts.
    expect(channelsOnScreen(tester), [3, 2, 1, 4]);

    await click(channelRow(4));
    expect(
      find.descendant(of: pinEntry, matching: find.text(l.sidebarPin)),
      findsOneWidget,
    );
    await tester.tap(pinEntry);
    await host.settle(tester);
    expect(await outbox(tester, host.db), [
      {
        'kind': 'putMySidebarChannel',
        'channel_id': channelId(4).value,
        'pinned': true,
      },
    ]);
    expect((await stateOf(tester, host, 4))!.pinned, isTrue);
    expect(channelsOnScreen(tester), [
      4,
      3,
      2,
      1,
    ], reason: 'at once, while the pointer is in the sidebar');
    expect(tester.widget<DsChannelRow>(channelRow(4)).data.pinned, isTrue);
    expect(tester.widget<DsChannelRow>(channelRow(3)).data.pinned, isFalse);

    // New activity in forum 1 with the pointer outside the sidebar: forum 1 moves up below the
    // pinned channel.
    await mouse.moveTo(outside);
    await host.settle(tester);
    await host.run(
      tester,
      () => host.db.threadsDao.advanceActivity(
        threadId(1),
        t0.add(const Duration(minutes: 90)),
      ),
    );
    await host.settle(tester);
    expect(channelsOnScreen(tester), [4, 1, 3, 2]);

    await click(channelRow(4));
    expect(pinEntry, findsNothing);
    expect(
      find.descendant(of: unpinEntry, matching: find.text(l.sidebarUnpin)),
      findsOneWidget,
    );
    await tester.tap(unpinEntry);
    await host.settle(tester);
    expect(channelsOnScreen(tester), [
      1,
      3,
      2,
      4,
    ], reason: 'back in its place by activity at once');
    expect((await outbox(tester, host.db)).last, {
      'kind': 'putMySidebarChannel',
      'channel_id': channelId(4).value,
      'pinned': false,
    });
    await mouse.removePointer();
    await host.dispose(tester);
  });

  testWidgets('pins of another device apply, several pins keep the order of '
      'their pin times, and an unpin there returns the row', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(tester, sidebar);
    expect(channelsOnScreen(tester), [1, 2, 3, 4]);
    await pinElsewhere(tester, host, 4, minutes: 1);
    await pinElsewhere(tester, host, 2, minutes: 2);
    await host.settle(tester);
    expect(channelsOnScreen(tester), [4, 2, 1, 3]);
    await pinElsewhere(tester, host, 4, minutes: 1, version: 2, pinned: false);
    await host.settle(tester);
    expect(channelsOnScreen(tester), [2, 1, 3, 4]);
    expect(await outbox(tester, host.db), isEmpty);
    await host.dispose(tester);
  });

  testWidgets('in a manual space a pinned channel does not move or lift, and '
      'the others move only below it', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(tester, sidebar);
    await pinElsewhere(tester, host, 3, minutes: 1);
    await host.settle(tester);
    expect(channelsOnScreen(tester), [3, 1, 2, 4]);

    await rightClick(tester, host, channelRow(3));
    for (final key in ['menu.moveUp', 'menu.moveDown']) {
      final entry = tester.widget<MenuItemButton>(
        find.descendant(
          of: find.byKey(ValueKey(key)),
          matching: find.byType(MenuItemButton),
        ),
      );
      expect(entry.onPressed, isNull, reason: '$key of the pinned row');
    }
    await tester.tapAt(const Offset(700, 500));
    await tester.pumpAndSettle();

    await rightClick(tester, host, channelRow(1));
    final up = tester.widget<MenuItemButton>(
      find.descendant(
        of: find.byKey(const ValueKey('menu.moveUp')),
        matching: find.byType(MenuItemButton),
      ),
    );
    expect(up.onPressed, isNull, reason: 'the first row below the pins');
    await tester.tap(find.byKey(const ValueKey('menu.moveDown')));
    await host.settle(tester);
    expect(channelsOnScreen(tester), [3, 2, 1, 4]);
    await host.dispose(tester);
  });

  testWidgets('archiving a pinned channel keeps its pin for the restore; its '
      'row in the section shows neither pin nor entry', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      await fill(host.db);
      await host.db.forumViewDao.setArchivedExpanded(
        spaceId(1),
        expanded: true,
      );
    });
    await host.pump(tester, sidebar);
    await pinElsewhere(tester, host, 4, minutes: 1);
    await host.settle(tester);
    expect(channelsOnScreen(tester), [4, 1, 2, 3]);

    await rightClick(tester, host, channelRow(4));
    await tester.tap(find.byKey(const ValueKey('menu.archiveChannel')));
    await host.settle(tester);
    expect(channelsOnScreen(tester), [1, 2, 3, 4], reason: 'in the section');
    expect(tester.widget<DsChannelRow>(channelRow(4)).data.pinned, isFalse);
    expect((await stateOf(tester, host, 4))!.pinned, isTrue);
    await rightClick(tester, host, channelRow(4));
    expect(pinEntry, findsNothing);
    expect(unpinEntry, findsNothing);
    await tester.tap(find.byKey(const ValueKey('menu.restoreChannel')));
    await host.settle(tester);
    expect(channelsOnScreen(tester), [4, 1, 2, 3], reason: 'pinned again');
    expect(tester.widget<DsChannelRow>(channelRow(4)).data.pinned, isTrue);
    await host.dispose(tester);
  });

  for (final platform in ['ios', 'android']) {
    testWidgets('a long press opens the menu with "Pin to top" on $platform', (
      tester,
    ) async {
      final host = await ColumnHost.open(tester);
      await tester.runAsync(() => fill(host.db));
      await host.pump(
        tester,
        const SidebarColumn(phone: true),
        platform: platform,
      );
      await tester.longPress(channelRow(4));
      await host.settle(tester);
      expect(pinEntry, findsOneWidget);
      await tester.tap(pinEntry);
      await host.settle(tester);
      expect((await stateOf(tester, host, 4))!.pinned, isTrue);
      expect(channelsOnScreen(tester), [4, 1, 2, 3]);
      await host.dispose(tester);
    });
  }

  testWidgets('"Pin to top" of a forum post pins it to the top of its forum '
      'through the outbox, and "Unpin" returns it', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(tester, sidebar);
    // Chats are arranged by activity by default: post 1 before post 5.
    expect(onScreen(tester, postRow, [1, 5]), [1, 5]);

    await rightClick(tester, host, postRow(5));
    await tester.tap(pinEntry);
    await host.settle(tester);
    expect(await outbox(tester, host.db), [
      {
        'kind': 'putMySidebarChannel',
        'channel_id': channelId(1).value,
        'pinned_threads': [threadId(5).value],
      },
    ]);
    expect(onScreen(tester, postRow, [1, 5]), [5, 1]);
    expect(tester.widget<DsSidebarThreadRow>(postRow(5)).data.pinned, isTrue);
    expect((await stateOf(tester, host, 1))!.pinned, isFalse);

    await rightClick(tester, host, postRow(5));
    await tester.tap(unpinEntry);
    await host.settle(tester);
    expect((await outbox(tester, host.db)).last, {
      'kind': 'putMySidebarChannel',
      'channel_id': channelId(1).value,
      'pinned_threads': <String>[],
    });
    expect((await stateOf(tester, host, 1))!.pinnedThreads, isEmpty);
    expect(onScreen(tester, postRow, [1, 5]), [
      1,
      5,
    ], reason: 'back in its place at once, while the sidebar holds');
    await host.dispose(tester);
  });

  testWidgets('a move in a manual forum writes the order without the pins, so '
      'an unpinned post returns to its place', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      await fill(host.db, settings: {'sidebar.auto_arrange_chats': false});
      for (final n in [6, 7]) {
        await host.db.threadsDao.upsertIfNewer(
          thread(n, channelN: 1, activeMinutes: n),
        );
      }
      await host.db.structureDao.putUserChannelStateLocally(channelId(1), [
        threadId(1),
        threadId(5),
        threadId(6),
        threadId(7),
      ]);
      await host.db.structureDao.putUserChannelPinnedThreadsLocally(
        channelId(1),
        [threadId(6)],
      );
    });
    await host.pump(tester, sidebar);
    expect(onScreen(tester, postRow, [1, 5, 6, 7]), [6, 1, 5, 7]);
    await rightClick(tester, host, postRow(7));
    await tester.tap(find.byKey(const ValueKey('menu.moveUp')));
    await host.settle(tester);
    expect(onScreen(tester, postRow, [1, 5, 6, 7]), [6, 1, 7, 5]);
    expect((await outbox(tester, host.db)).last['thread_order'], [
      threadId(1).value,
      threadId(7).value,
      threadId(5).value,
      threadId(6).value,
    ], reason: 'the pinned post stays after post 5, where it was');
    await rightClick(tester, host, postRow(6));
    await tester.tap(unpinEntry);
    await host.settle(tester);
    expect(onScreen(tester, postRow, [1, 5, 6, 7]), [1, 7, 5, 6]);
    await host.dispose(tester);
  });

  testWidgets('the More menu of a main history and of a chat history has the '
      'entry', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    final team = channel(4, name: 'team');
    await host.pump(tester, TargetMenuButton(channel: team));
    await tester.tap(find.byKey(const ValueKey('thread.menu')));
    await tester.pumpAndSettle();
    await tester.tap(pinEntry);
    await host.settle(tester);
    expect((await stateOf(tester, host, 4))!.pinned, isTrue);
    await host.dispose(tester);

    final again = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(again.db));
    final forum = channel(1, kind: ChannelKind.forum, name: 'forum-1');
    await again.pump(
      tester,
      TargetMenuButton(channel: forum, thread: threadId(1)),
    );
    await tester.tap(find.byKey(const ValueKey('thread.menu')));
    await tester.pumpAndSettle();
    await tester.tap(pinEntry);
    await again.settle(tester);
    expect((await stateOf(tester, again, 1))!.pinnedThreads, [threadId(1)]);
    await again.dispose(tester);
  });
}
