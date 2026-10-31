// SPDX-License-Identifier: Apache-2.0

// The thread groups of the sidebar over the local database (brief M4-40 Tests; 01 6.8 items 4
// and 7; Design note 2; brief M4-46): a row that stops qualifying stays while the pointer is in
// the sidebar and leaves when it leaves; a new thread appears at the top at once; the collapse
// state of a group survives a restart on this device; a thread row of an automatic forum does not
// move (01 6.16); a forum
// lists every open post by last activity with "Show more" and "Show less" in place, kept on this
// device, and no row moves under the pointer.

import 'package:client/sidebar/sidebar_column.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

/// Now in these tests: the fake clock of the column host stands at [t0].
Thread post(
  int n, {
  required int created,
  int ageHours = 500,
  ThreadState state = ThreadState.open,
  int version = 1,
}) => Thread(
  id: threadId(n),
  channelId: channelId(1),
  kind: ThreadKind.thread,
  title: 'Post $n',
  titleSource: TitleSource.manual,
  state: state,
  lastMessageSeq: const ThreadSeq(4),
  unreadFloorSeq: const ThreadSeq(0),
  lastActivityAt: t0.subtract(Duration(hours: ageHours)),
  messageCount: 4,
  createdBy: sam,
  createdAt: t0.subtract(Duration(days: 30, minutes: -created)),
  version: ResourceVersion(version),
  tagIds: const [],
);

Future<void> fill(LocalDatabase db) async {
  await db.structureDao.upsertSpaceIfNewer(space(1));
  await db.structureDao.upsertChannelIfNewer(
    channel(1, kind: ChannelKind.forum, name: 'server'),
  );
  await db.structureDao.upsertChannelIfNewer(channel(2, name: 'general'));
  // Post 1 is old and unread; post 2 is recent. Both are open, so the forum lists both.
  await db.threadsDao.upsertIfNewer(post(1, created: 1));
  await db.threadsDao.upsertIfNewer(post(2, created: 2, ageHours: 1));
  await db.personalDao.applyReadState(readState(1, lastRead: 1, unread: 3));
}

/// The sidebar at its usual width at the left of the window, so the pointer can leave it.
const Widget sidebar = Align(
  alignment: Alignment.topLeft,
  child: SizedBox(width: 280, child: SidebarColumn()),
);

Finder row(int n) =>
    find.byKey(ValueKey('sidebar.thread.${threadId(n).value}'));

Finder channelRow(int n) =>
    find.byKey(ValueKey('sidebar.channel.${channelId(n).value}'));

double top(WidgetTester tester, Finder f) => tester.getTopLeft(f).dy;

/// The "Show more" row of the forum (channel 1).
Finder moreRow() =>
    find.byKey(ValueKey('sidebar.morePosts.${channelId(1).value}'));

/// The thread rows of [ns] that are on screen.
List<int> shown(Iterable<int> ns) => [
  for (final n in ns)
    if (row(n).evaluate().isNotEmpty) n,
];

/// The forum (channel 1) with 24 open posts that are read and idle for days, as after an
/// import, newest activity first by number; the text channel (channel 2) with an old open
/// thread.
Future<void> fillImported(LocalDatabase db) async {
  await db.structureDao.upsertSpaceIfNewer(space(1));
  await db.structureDao.upsertChannelIfNewer(
    channel(1, kind: ChannelKind.forum, name: 'server'),
  );
  await db.structureDao.upsertChannelIfNewer(channel(2, name: 'general'));
  for (var n = 1; n <= 24; n++) {
    await db.threadsDao.upsertIfNewer(
      post(n, created: 100 - n, ageHours: 100 + n),
    );
  }
  await db.threadsDao.upsertIfNewer(
    Thread(
      id: threadId(50),
      channelId: channelId(2),
      kind: ThreadKind.thread,
      title: 'Old thread',
      titleSource: TitleSource.manual,
      state: ThreadState.open,
      lastMessageSeq: const ThreadSeq(4),
      unreadFloorSeq: const ThreadSeq(0),
      lastActivityAt: t0.subtract(const Duration(hours: 500)),
      messageCount: 4,
      createdBy: sam,
      createdAt: t0.subtract(const Duration(days: 30)),
      version: const ResourceVersion(1),
      tagIds: const [],
    ),
  );
}

void main() {
  testWidgets(
    'a row that stops qualifying stays under the pointer and leaves after it',
    (tester) async {
      final host = await ColumnHost.open(tester);
      await tester.runAsync(() => fill(host.db));
      await host.pump(tester, sidebar);
      expect(row(1), findsOneWidget);
      expect(row(2), findsOneWidget);

      final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
      addTearDown(mouse.removePointer);
      await mouse.addPointer(location: Offset.zero);
      await mouse.moveTo(tester.getCenter(channelRow(1)));
      await host.settle(tester);

      // Post 1 is done now: it no longer qualifies, but the pointer is in the sidebar.
      await host.run(
        tester,
        () => host.db.threadsDao.upsertIfNewer(
          post(1, created: 1, state: ThreadState.closed, version: 2),
        ),
      );
      await host.settle(tester);
      expect(row(1), findsOneWidget, reason: 'no row leaves under the pointer');

      // The pointer leaves the sidebar (the window is wider than the column).
      final size = tester.view.physicalSize / tester.view.devicePixelRatio;
      await mouse.moveTo(Offset(size.width - 2, size.height - 2));
      await host.settle(tester);
      expect(row(1), findsNothing);
      expect(row(2), findsOneWidget);
      await host.dispose(tester);
    },
  );

  testWidgets('a new thread appears at the top of its group at once', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(tester, sidebar);
    final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
    addTearDown(mouse.removePointer);
    await mouse.addPointer(location: tester.getCenter(channelRow(1)));
    await host.settle(tester);

    await host.run(
      tester,
      () => host.db.threadsDao.upsertIfNewer(post(3, created: 3, ageHours: 0)),
    );
    await host.settle(tester);
    expect(row(3), findsOneWidget, reason: 'additions apply at once');
    expect(top(tester, row(3)), lessThan(top(tester, row(2))));
    expect(top(tester, row(2)), lessThan(top(tester, row(1))));
    await host.dispose(tester);
  });

  testWidgets('the collapse state of a group survives a restart', (
    tester,
  ) async {
    final db = await tester.runAsync(
      () async => openInMemoryForTests(clock: () => t0),
    );
    var host = await ColumnHost.open(tester, database: db);
    await tester.runAsync(() => fill(db!));
    await host.pump(tester, sidebar);
    expect(row(2), findsOneWidget, reason: 'expanded by default');
    await tester.tap(find.byTooltip(en.sidebarHideThreads));
    await host.settle(tester);
    expect(row(2), findsNothing);
    await host.dispose(tester);

    // A new start of the app on this device: the same database, new providers.
    host = await ColumnHost.open(tester, database: db);
    await host.pump(tester, sidebar);
    expect(row(2), findsNothing, reason: 'still collapsed');
    expect(channelRow(1), findsOneWidget);
    await host.dispose(tester);
    await tester.runAsync(db!.close);
  });

  testWidgets('a thread row of a forum arranged by activity does not move', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      await fill(host.db);
      await host.db.structureDao.upsertSpaceIfNewer(space(2));
    });
    await host.pump(tester, sidebar);
    final before = top(
      tester,
      find.byKey(ValueKey('sidebar.space.${spaceId(2).value}')),
    );
    final drag = await tester.startGesture(
      tester.getCenter(row(2)),
      kind: PointerDeviceKind.mouse,
    );
    await drag.moveBy(const Offset(0, 300));
    await host.settle(tester);
    await drag.up();
    await host.settle(tester);
    final after = top(
      tester,
      find.byKey(ValueKey('sidebar.space.${spaceId(2).value}')),
    );
    expect(after, before, reason: 'no space moved');
    final writes = await tester.runAsync(host.db.outboxDao.allEntries);
    expect(writes, isEmpty, reason: 'no order was written');
    await host.dispose(tester);
  });

  testWidgets('a forum with 24 open posts lists the newest 15, Show more lists all in place '
      'and Show less goes back; the choice survives a restart', (tester) async {
    final db = await tester.runAsync(
      () async => openInMemoryForTests(clock: () => t0),
    );
    var host = await ColumnHost.open(tester, database: db);
    await tester.runAsync(() => fillImported(db!));
    await host.pump(tester, sidebar);
    final all = [for (var n = 1; n <= 24; n++) n];
    expect(shown(all), [for (var n = 1; n <= 15; n++) n]);
    // By last activity, newest first.
    for (var n = 2; n <= 15; n++) {
      expect(top(tester, row(n - 1)), lessThan(top(tester, row(n))));
    }
    // The text channel keeps its rule: an old read thread has no row.
    expect(row(50), findsNothing);
    expect(moreRow(), findsOneWidget);
    expect(find.text(en.sidebarShowMore), findsOneWidget);
    expect(
      find.bySemanticsLabel('${en.sidebarShowMore}, ${en.sidebarMorePosts(9)}'),
      findsOneWidget,
    );
    // "Show more" sits between the last post and "All posts".
    final allPosts = find.byKey(
      ValueKey('sidebar.allPosts.${channelId(1).value}'),
    );
    expect(top(tester, moreRow()), greaterThan(top(tester, row(15))));
    expect(top(tester, moreRow()), lessThan(top(tester, allPosts)));

    await tester.ensureVisible(moreRow());
    await host.settle(tester);
    await tester.tap(moreRow());
    await host.settle(tester);
    expect(shown(all), all, reason: 'every open post, in place');
    expect(find.text(en.sidebarShowLess), findsOneWidget);
    expect(host.visited, isNot(contains('/c/${channelId(1).value}')));
    await host.dispose(tester);

    // A new start of the app on this device: the same database, new providers.
    host = await ColumnHost.open(tester, database: db);
    await host.pump(tester, sidebar);
    expect(shown(all), all, reason: 'still expanded on this device');
    await tester.ensureVisible(moreRow());
    await host.settle(tester);
    await tester.tap(moreRow());
    await host.settle(tester);
    expect(shown(all), [for (var n = 1; n <= 15; n++) n]);
    expect(find.text(en.sidebarShowMore), findsOneWidget);
    await host.dispose(tester);
    await tester.runAsync(db!.close);
  });

  testWidgets(
    'Show less applies at once although the pointer is in the sidebar',
    (tester) async {
      final host = await ColumnHost.open(tester);
      await tester.runAsync(() => fillImported(host.db));
      await host.pump(tester, sidebar);
      final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
      addTearDown(mouse.removePointer);
      await mouse.addPointer(location: tester.getCenter(channelRow(1)));
      await host.settle(tester);
      await tester.ensureVisible(moreRow());
      await host.settle(tester);
      await tester.tap(moreRow(), kind: PointerDeviceKind.mouse);
      await host.settle(tester);
      expect(shown([for (var n = 1; n <= 24; n++) n]), hasLength(24));
      await tester.ensureVisible(moreRow());
      await host.settle(tester);
      await tester.tap(moreRow(), kind: PointerDeviceKind.mouse);
      await host.settle(tester);
      expect(
        shown([for (var n = 1; n <= 24; n++) n]),
        hasLength(15),
        reason: 'the person asked for it; the rows shown before do not hold',
      );
      await host.dispose(tester);
    },
  );

  testWidgets(
    'a forum row does not move under the pointer when its post gets activity',
    (tester) async {
      final host = await ColumnHost.open(tester);
      await tester.runAsync(() => fill(host.db));
      await host.pump(tester, sidebar);
      // Post 2 was active an hour ago, post 1 long ago: 2 is above 1.
      expect(top(tester, row(2)), lessThan(top(tester, row(1))));
      final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
      addTearDown(mouse.removePointer);
      await mouse.addPointer(location: tester.getCenter(row(1)));
      await host.settle(tester);

      await host.run(
        tester,
        () => host.db.threadsDao.upsertIfNewer(
          post(1, created: 1, ageHours: 0, version: 2),
        ),
      );
      await host.settle(tester);
      expect(
        top(tester, row(2)),
        lessThan(top(tester, row(1))),
        reason: 'no row moves while the pointer is in the sidebar',
      );

      final size = tester.view.physicalSize / tester.view.devicePixelRatio;
      await mouse.moveTo(Offset(size.width - 2, size.height - 2));
      await host.settle(tester);
      expect(top(tester, row(1)), lessThan(top(tester, row(2))));
      await host.dispose(tester);
    },
  );
}
