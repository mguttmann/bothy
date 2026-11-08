// SPDX-License-Identifier: Apache-2.0

// The personal archive of a channel or project forum in the sidebar (01 6.22, R-PR-38;
// PROJ-ARCHIVE) over the local database: the menu entry ("Archive project" for a project forum,
// "Archive" else, on the desktop by right click and Shift+F10, on the phone by a long press) and
// its screen reader action write `hidden` through the outbox and move the row at once into the
// collapsed section "Archived (n)" at the bottom of its space; the section keeps its collapse on
// this device; "Restore" brings the row back to its earlier place, also after a move of the
// other channels; an archive of another device applies, and its unread items still show.

import 'dart:convert';

import 'package:client/sidebar/sidebar_column.dart';
import 'package:design_system/design_system.dart' show DsChannelRow, DsSpaceRow;
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/structure.dart';

/// The sidebar at its usual width at the left of the window.
const Widget sidebar = Align(
  alignment: Alignment.topLeft,
  child: SizedBox(width: 280, child: SidebarColumn()),
);

Finder channelRow(int n) =>
    find.byKey(ValueKey('sidebar.channel.${channelId(n).value}'));
final Finder section = find.byKey(
  ValueKey('sidebar.archived.${spaceId(1).value}'),
);
final Finder archiveEntry = find.byKey(const ValueKey('menu.archiveChannel'));
final Finder restoreEntry = find.byKey(const ValueKey('menu.restoreChannel'));

double top(WidgetTester tester, Finder f) => tester.getTopLeft(f).dy;

/// The channel rows of space 1 on screen from top to bottom, as their numbers.
List<int> onScreen(WidgetTester tester) {
  final present = {
    for (final n in [1, 2, 3, 4])
      if (channelRow(n).evaluate().isNotEmpty) n: top(tester, channelRow(n)),
  };
  return present.keys.toList()
    ..sort((a, b) => present[a]!.compareTo(present[b]!));
}

/// Space 1 with the text channel 1, the project forum 2 and the forums 3 and 4 (global order
/// 1 to 4); space 2 with the text channel 5.
Future<void> fill(LocalDatabase db) async {
  await db.structureDao.upsertSpaceIfNewer(space(1));
  await db.structureDao.upsertSpaceIfNewer(space(2));
  await db.structureDao.upsertChannelIfNewer(channel(1, name: 'general'));
  await db.structureDao.upsertChannelIfNewer(
    channel(
      2,
      kind: ChannelKind.forum,
      name: 'website',
    ).copyWith(projectBound: true),
  );
  await db.structureDao.upsertChannelIfNewer(
    channel(3, kind: ChannelKind.forum, name: 'ideas'),
  );
  await db.structureDao.upsertChannelIfNewer(
    channel(4, kind: ChannelKind.forum, name: 'plans'),
  );
  await db.structureDao.upsertChannelIfNewer(
    channel(5, spaceN: 2, name: 'ops'),
  );
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

void main() {
  late AppLocalizations l;
  setUpAll(() async {
    l = await AppLocalizations.delegate.load(const Locale('en'));
  });

  testWidgets('the menu names the entry for a project forum and for every '
      'other channel, and archives through the outbox', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(tester, sidebar);
    expect(section, findsNothing, reason: 'no section without an archive');

    for (final (n, label) in [
      (1, l.channelMenuArchive),
      (2, l.channelMenuArchiveProject),
      (3, l.channelMenuArchive),
    ]) {
      await rightClick(tester, host, channelRow(n));
      expect(archiveEntry, findsOneWidget, reason: 'channel $n');
      expect(
        find.descendant(of: archiveEntry, matching: find.text(label)),
        findsOneWidget,
        reason: 'channel $n',
      );
      // A click outside closes the menu.
      await tester.tapAt(const Offset(700, 500));
      await tester.pumpAndSettle();
      expect(archiveEntry, findsNothing, reason: 'menu of $n closed');
    }

    await rightClick(tester, host, channelRow(2));
    await tester.tap(archiveEntry);
    await host.settle(tester);
    expect(await outbox(tester, host.db), [
      {
        'kind': 'putMySidebarChannel',
        'channel_id': channelId(2).value,
        'hidden': true,
      },
    ]);
    expect((await stateOf(tester, host, 2))!.hidden, isTrue);
    // The row leaves the list at once; the section shows its count, collapsed.
    expect(onScreen(tester), [1, 3, 4]);
    expect(section, findsOneWidget);
    expect(
      find.descendant(of: section, matching: find.text(l.sidebarArchived(1))),
      findsOneWidget,
    );
    expect(channelRow(2), findsNothing, reason: 'collapsed by default');
    await host.dispose(tester);
  });

  testWidgets('the section sits at the very bottom of its space, counts its '
      'channels and keeps its collapse on this device', (tester) async {
    final db = await tester.runAsync(
      () async => openInMemoryForTests(clock: () => t0),
    );
    var host = await ColumnHost.open(tester, database: db);
    await tester.runAsync(() async {
      await fill(db!);
      await db.structureDao.putUserChannelHiddenLocally(
        channelId(1),
        hiddenAt: t0,
      );
      await db.structureDao.putUserChannelHiddenLocally(
        channelId(3),
        hiddenAt: t0,
      );
    });
    await host.pump(tester, sidebar);
    expect(
      find.descendant(of: section, matching: find.text(l.sidebarArchived(2))),
      findsOneWidget,
    );
    expect(onScreen(tester), [2, 4]);
    expect(top(tester, section), greaterThan(top(tester, channelRow(4))));
    final space2 = find.byKey(ValueKey('sidebar.space.${spaceId(2).value}'));
    expect(top(tester, section), lessThan(top(tester, space2)));

    await tester.tap(section);
    await host.settle(tester);
    // Expanded: the archived rows below the section, in the manual order.
    expect(onScreen(tester), [2, 4, 1, 3]);
    expect(top(tester, channelRow(1)), greaterThan(top(tester, section)));
    expect(
      await tester.runAsync(
        () => db!.forumViewDao.watchArchivedExpandedSpaces().first,
      ),
      {spaceId(1)},
    );
    expect(
      await outbox(tester, db!),
      isEmpty,
      reason: 'the collapse of the section is not synchronized',
    );
    await host.dispose(tester);

    host = await ColumnHost.open(tester, database: db);
    await host.pump(tester, sidebar);
    expect(onScreen(tester), [2, 4, 1, 3], reason: 'kept after a restart');
    await tester.tap(section);
    await host.settle(tester);
    expect(onScreen(tester), [2, 4]);
    await host.dispose(tester);
    await tester.runAsync(db.close);
  });

  testWidgets('"Restore" brings the channel back to its earlier place, also '
      'after the other channels moved', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      await fill(host.db);
      await host.db.forumViewDao.setArchivedExpanded(
        spaceId(1),
        expanded: true,
      );
    });
    await host.pump(tester, sidebar);
    expect(onScreen(tester), [1, 2, 3, 4]);

    await rightClick(tester, host, channelRow(2));
    await tester.tap(archiveEntry);
    await host.settle(tester);
    expect(onScreen(tester), [1, 3, 4, 2]);

    // A move of channel 4 above channel 3 keeps the place of the archived channel 2.
    await rightClick(tester, host, channelRow(4));
    await tester.tap(find.byKey(const ValueKey('menu.moveUp')));
    await host.settle(tester);
    expect(onScreen(tester), [1, 4, 3, 2]);
    final order = (await outbox(
      tester,
      host.db,
    )).where((w) => w['kind'] == 'putMySidebarSpace').single['channel_order'];
    expect(order, [
      channelId(1).value,
      channelId(2).value,
      channelId(4).value,
      channelId(3).value,
    ]);

    await rightClick(tester, host, channelRow(2));
    expect(archiveEntry, findsNothing);
    expect(
      find.descendant(
        of: restoreEntry,
        matching: find.text(l.channelMenuRestore),
      ),
      findsOneWidget,
    );
    await tester.tap(restoreEntry);
    await host.settle(tester);
    expect(onScreen(tester), [1, 2, 4, 3]);
    expect(section, findsNothing, reason: 'no archived channel left');
    expect((await outbox(tester, host.db)).last, {
      'kind': 'putMySidebarChannel',
      'channel_id': channelId(2).value,
      'hidden': false,
    });
    await host.dispose(tester);
  });

  testWidgets('Shift+F10 opens the menu with the entry on Windows, and the '
      'row has the screen reader action', (tester) async {
    final handle = tester.ensureSemantics();
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(tester, sidebar, platform: 'windows');
    final general = tester.getSemantics(channelRow(1));
    expect(
      general.getSemanticsData().customSemanticsActionIds,
      contains(
        CustomSemanticsAction.getIdentifier(
          CustomSemanticsAction(label: l.channelMenuArchive),
        ),
      ),
    );
    Focus.of(
      tester.element(
        find.descendant(of: channelRow(1), matching: find.byType(Text)).first,
      ),
    ).requestFocus();
    await host.settle(tester);
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
    await tester.sendKeyEvent(LogicalKeyboardKey.f10);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
    await tester.pumpAndSettle();
    expect(archiveEntry, findsOneWidget);
    await tester.tap(archiveEntry);
    await host.settle(tester);
    expect((await stateOf(tester, host, 1))!.hidden, isTrue);
    await host.dispose(tester);
    handle.dispose();
  });

  for (final platform in ['ios', 'android']) {
    testWidgets('a long press opens the menu with the entry on $platform', (
      tester,
    ) async {
      final host = await ColumnHost.open(tester);
      await tester.runAsync(() => fill(host.db));
      await host.pump(
        tester,
        const SidebarColumn(phone: true),
        platform: platform,
      );
      await tester.longPress(channelRow(3));
      await host.settle(tester);
      expect(archiveEntry, findsOneWidget);
      await tester.tap(archiveEntry);
      await host.settle(tester);
      expect((await stateOf(tester, host, 3))!.hidden, isTrue);
      expect(channelRow(3), findsNothing);
      expect(section, findsOneWidget);
      await host.dispose(tester);
    });
  }

  testWidgets('an archive of another device applies, and the unread items of '
      'an archived channel still show', (tester) async {
    final handle = tester.ensureSemantics();
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      await fill(host.db);
      await host.db.personalDao.applyReadState(
        readState(1, lastRead: 2, unread: 3, mentions: 1),
      );
    });
    await host.pump(tester, sidebar);
    expect(onScreen(tester), [1, 2, 3, 4]);
    await host.run(
      tester,
      () => host.db.structureDao.upsertUserChannelStateIfNewer(
        UserChannelState(
          channelId: channelId(1),
          threadOrder: const [],
          version: const ResourceVersion(1),
          hiddenAt: t0,
        ),
      ),
    );
    await host.settle(tester);
    expect(onScreen(tester), [2, 3, 4]);
    expect(section, findsOneWidget);
    // Collapsing the space keeps its unread dot from the archived channel.
    await tester.tap(find.byKey(ValueKey('sidebar.space.${spaceId(1).value}')));
    await host.settle(tester);
    expect(
      tester
          .widget<DsSpaceRow>(
            find.byKey(ValueKey('sidebar.space.${spaceId(1).value}')),
          )
          .data
          .hasUnread,
      isTrue,
      reason: 'the collapsed space still shows its unread dot',
    );
    await tester.tap(find.byKey(ValueKey('sidebar.space.${spaceId(1).value}')));
    await host.settle(tester);
    await tester.tap(section);
    await host.settle(tester);
    final row = tester.widget<DsChannelRow>(channelRow(1)).data;
    expect(
      (row.unreadCount, row.mentionCount),
      (3, 1),
      reason: 'the archived row shows its counters',
    );

    // Restored on the other device: back in its place.
    await host.run(
      tester,
      () => host.db.structureDao.upsertUserChannelStateIfNewer(
        UserChannelState(
          channelId: channelId(1),
          threadOrder: const [],
          version: const ResourceVersion(2),
        ),
      ),
    );
    await host.settle(tester);
    expect(onScreen(tester), [1, 2, 3, 4]);
    expect(section, findsNothing);
    await host.dispose(tester);
    handle.dispose();
  });
}
