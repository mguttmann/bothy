// SPDX-License-Identifier: Apache-2.0

// The sidebar ordering over the local database (brief M4-51; 01 6.16; D-M4-51-2 to D-M4-51-9):
// a channel and a forum post moved by the menu write the person's order through the outbox and
// keep it after a restart; an order from another device of the person applies; the automatic
// levels order by activity, a new message moves its chat, its forum and its space to the top,
// and nothing moves while the pointer is in the sidebar.

import 'dart:convert';

import 'package:client/sidebar/sidebar_column.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
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
Finder spaceRow(int n) =>
    find.byKey(ValueKey('sidebar.space.${spaceId(n).value}'));

double top(WidgetTester tester, Finder f) => tester.getTopLeft(f).dy;

/// The order on screen of [rows] (top to bottom), as their numbers.
List<int> onScreen(WidgetTester tester, Map<int, Finder> rows) {
  final present = {
    for (final e in rows.entries)
      if (e.value.evaluate().isNotEmpty) e.key: top(tester, e.value),
  };
  return present.keys.toList()
    ..sort((a, b) => present[a]!.compareTo(present[b]!));
}

Map<int, Finder> channels(Iterable<int> ns) => {
  for (final n in ns) n: channelRow(n),
};
Map<int, Finder> posts(Iterable<int> ns) => {for (final n in ns) n: postRow(n)};
Map<int, Finder> spaces(Iterable<int> ns) => {
  for (final n in ns) n: spaceRow(n),
};

/// Space 1 with the forums 1, 2 and 3 (global order) and space 2 with forum 4; forum 1 holds the
/// posts 1 to 3, post n last active n minutes after [t0]; every forum's newest activity is its
/// own number of minutes after [t0] (forum 4 the newest).
Future<void> fill(
  LocalDatabase db, {
  Map<String, Object> settings = const {},
}) async {
  await db.structureDao.upsertSpaceIfNewer(space(1));
  await db.structureDao.upsertSpaceIfNewer(space(2));
  for (final n in [1, 2, 3]) {
    await db.structureDao.upsertChannelIfNewer(
      channel(n, kind: ChannelKind.forum, name: 'forum-$n'),
    );
  }
  await db.structureDao.upsertChannelIfNewer(
    channel(4, spaceN: 2, kind: ChannelKind.forum, name: 'forum-4'),
  );
  for (final n in [1, 2, 3]) {
    await db.threadsDao.upsertIfNewer(thread(n, channelN: 1, activeMinutes: n));
  }
  await db.threadsDao.upsertIfNewer(thread(12, channelN: 2, activeMinutes: 2));
  await db.threadsDao.upsertIfNewer(thread(13, channelN: 3, activeMinutes: 5));
  await db.threadsDao.upsertIfNewer(thread(14, channelN: 4, activeMinutes: 40));
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

Future<void> menuMove(
  WidgetTester tester,
  ColumnHost host,
  Finder row,
  String key,
) async {
  await tester.tapAt(
    tester.getCenter(row),
    buttons: kSecondaryMouseButton,
    kind: PointerDeviceKind.mouse,
  );
  await host.settle(tester);
  await tester.tap(find.byKey(ValueKey(key)));
  await host.settle(tester);
}

void main() {
  testWidgets('a channel moved by the menu writes the order of its space and '
      'keeps it after a restart', (tester) async {
    final db = await tester.runAsync(
      () async => openInMemoryForTests(clock: () => t0),
    );
    var host = await ColumnHost.open(tester, database: db);
    await tester.runAsync(() => fill(db!));
    await host.pump(tester, sidebar);
    expect(onScreen(tester, channels([1, 2, 3])), [1, 2, 3]);
    await menuMove(tester, host, channelRow(1), 'menu.moveDown');
    expect(onScreen(tester, channels([1, 2, 3])), [2, 1, 3]);
    final writes = await outbox(tester, db!);
    expect(writes.single['kind'], 'putMySidebarSpace');
    expect(writes.single['space_id'], spaceId(1).value);
    expect(writes.single['channel_order'], [
      channelId(2).value,
      channelId(1).value,
      channelId(3).value,
    ]);
    await host.dispose(tester);

    host = await ColumnHost.open(tester, database: db);
    await host.pump(tester, sidebar);
    expect(onScreen(tester, channels([1, 2, 3])), [2, 1, 3], reason: 'kept');
    await host.dispose(tester);
    await tester.runAsync(db.close);
  });

  testWidgets('with chats arranged by hand a post moved by the menu writes the '
      'order of its forum and keeps it after a restart', (tester) async {
    final db = await tester.runAsync(
      () async => openInMemoryForTests(clock: () => t0),
    );
    var host = await ColumnHost.open(tester, database: db);
    await tester.runAsync(
      () => fill(db!, settings: {'sidebar.auto_arrange_chats': false}),
    );
    await host.pump(tester, sidebar);
    // No order yet: by activity, newest first.
    expect(onScreen(tester, posts([1, 2, 3])), [3, 2, 1]);
    await menuMove(tester, host, postRow(3), 'menu.moveDown');
    expect(onScreen(tester, posts([1, 2, 3])), [2, 3, 1]);
    final writes = await outbox(tester, db!);
    expect(writes.single['kind'], 'putMySidebarChannel');
    expect(writes.single['channel_id'], channelId(1).value);
    expect(writes.single['thread_order'], [
      threadId(2).value,
      threadId(3).value,
      threadId(1).value,
    ]);
    // Activity does not reorder a manual forum: post 1 gets the newest message.
    await host.run(
      tester,
      () => db.threadsDao.upsertIfNewer(
        thread(
          1,
          channelN: 1,
          activeMinutes: 99,
        ).copyWith(version: const ResourceVersion(2)),
      ),
    );
    await host.settle(tester);
    expect(onScreen(tester, posts([1, 2, 3])), [2, 3, 1]);
    await host.dispose(tester);

    host = await ColumnHost.open(tester, database: db);
    await host.pump(tester, sidebar);
    expect(onScreen(tester, posts([1, 2, 3])), [2, 3, 1], reason: 'kept');
    await host.dispose(tester);
    await tester.runAsync(db.close);
  });

  testWidgets('an order written on another device of the person applies', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(
      () => fill(host.db, settings: {'sidebar.auto_arrange_chats': false}),
    );
    await host.pump(tester, sidebar);
    await host.run(tester, () async {
      await host.db.structureDao.upsertUserSpaceStateIfNewer(
        UserSpaceState(
          spaceId: spaceId(1),
          collapsed: false,
          channelOrder: [channelId(3), channelId(1)],
          version: const ResourceVersion(1),
        ),
      );
      await host.db.structureDao.upsertUserChannelStateIfNewer(
        UserChannelState(
          channelId: channelId(1),
          threadOrder: [threadId(1), threadId(3), threadId(2)],
          version: const ResourceVersion(1),
        ),
      );
    });
    await host.settle(tester);
    // Channel 2 is not in the list: it comes first.
    expect(onScreen(tester, channels([1, 2, 3])), [2, 3, 1]);
    expect(onScreen(tester, posts([1, 2, 3])), [1, 3, 2]);
    await host.dispose(tester);
  });

  testWidgets('automatic levels: a new message moves its chat, its forum and '
      'its space to the top', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(
      () => fill(
        host.db,
        settings: {
          'sidebar.auto_arrange_spaces': true,
          'sidebar.auto_arrange_channels': true,
        },
      ),
    );
    await host.pump(tester, sidebar);
    expect(onScreen(tester, spaces([1, 2])), [
      2,
      1,
    ], reason: 'forum 4 is newest');
    expect(onScreen(tester, channels([1, 2, 3])), [3, 1, 2]);
    expect(onScreen(tester, posts([1, 2, 3])), [3, 2, 1]);
    // A message in post 1 of forum 1 in space 1.
    await host.run(
      tester,
      () => host.db.threadsDao.upsertIfNewer(
        thread(
          1,
          channelN: 1,
          activeMinutes: 60,
        ).copyWith(version: const ResourceVersion(2)),
      ),
    );
    await host.settle(tester);
    expect(onScreen(tester, spaces([1, 2])), [1, 2]);
    expect(onScreen(tester, channels([1, 2, 3])), [1, 3, 2]);
    expect(onScreen(tester, posts([1, 2, 3])), [1, 3, 2]);
    // Nothing of an automatic level lifts.
    expect(await outbox(tester, host.db), isEmpty);
    await host.dispose(tester);
  });

  testWidgets('automatic channels hold their order while the pointer is in the '
      'sidebar and apply it after the pointer left', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(
      () => fill(host.db, settings: {'sidebar.auto_arrange_channels': true}),
    );
    await host.pump(tester, sidebar);
    expect(onScreen(tester, channels([1, 2, 3])), [3, 1, 2]);
    final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
    await mouse.addPointer(location: tester.getCenter(channelRow(2)));
    await host.settle(tester);
    await host.run(
      tester,
      () => host.db.threadsDao.upsertIfNewer(
        thread(
          1,
          channelN: 1,
          activeMinutes: 60,
        ).copyWith(version: const ResourceVersion(2)),
      ),
    );
    await host.settle(tester);
    expect(onScreen(tester, channels([1, 2, 3])), [3, 1, 2], reason: 'held');
    // The window is wider than the column: the far right is outside the sidebar.
    final size = tester.view.physicalSize / tester.view.devicePixelRatio;
    await mouse.moveTo(Offset(size.width - 2, size.height - 2));
    await host.settle(tester);
    expect(onScreen(tester, channels([1, 2, 3])), [1, 3, 2]);
    await mouse.removePointer();
    await host.dispose(tester);
  });
}
