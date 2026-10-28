// SPDX-License-Identifier: Apache-2.0

// The thread groups of the sidebar over the local database (brief M4-40 Tests; 01 6.8 items 4
// and 7; Design note 2): a row that stops qualifying stays while the pointer is in the sidebar
// and leaves when it leaves; a new thread appears at the top at once; the collapse state of a
// group survives a restart on this device; thread rows cannot be dragged.

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
Thread post(int n, {required int created, int ageHours = 500}) => Thread(
  id: threadId(n),
  channelId: channelId(1),
  kind: ThreadKind.thread,
  title: 'Post $n',
  titleSource: TitleSource.manual,
  state: ThreadState.open,
  lastMessageSeq: const ThreadSeq(4),
  unreadFloorSeq: const ThreadSeq(0),
  lastActivityAt: t0.subtract(Duration(hours: ageHours)),
  messageCount: 4,
  createdBy: sam,
  createdAt: t0.subtract(Duration(days: 30, minutes: -created)),
  version: const ResourceVersion(1),
  tagIds: const [],
);

Future<void> fill(LocalDatabase db) async {
  await db.structureDao.upsertSpaceIfNewer(space(1));
  await db.structureDao.upsertChannelIfNewer(
    channel(1, kind: ChannelKind.forum, name: 'server'),
  );
  await db.structureDao.upsertChannelIfNewer(channel(2, name: 'general'));
  // Post 1 is old and qualifies only while it is unread; post 2 is recent.
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

      // Post 1 is read now: it no longer qualifies, but the pointer is in the sidebar.
      await host.run(
        tester,
        () => host.db.personalDao.applyReadState(
          ReadState(
            threadId: threadId(1),
            channelId: channelId(1),
            lastReadSeq: const ThreadSeq(4),
            mentionCount: 0,
            unreadCount: 0,
            version: const ResourceVersion(2),
          ),
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

  testWidgets('a thread row cannot be dragged', (tester) async {
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
}
