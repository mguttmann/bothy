// SPDX-License-Identifier: Apache-2.0

// The sidebar over the local database (brief M4-19 Tests; components.md 7; 04 14): the
// personal order with the global fallback, collapse through the outbox with the row updated at
// once, the tree semantics and the channel labels.

import 'dart:convert';

import 'package:client/sidebar/sidebar_column.dart';
import 'package:client/sync/sidebar_ops.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

Future<void> fill(ColumnHost host) async {
  final db = host.db;
  for (final s in [space(1), space(2), space(3)]) {
    await db.structureDao.upsertSpaceIfNewer(s);
  }
  // Space 3 first by the person's own key; space 2 has a state without a key (global order).
  await db.structureDao.upsertUserSpaceStateIfNewer(
    UserSpaceState(
      spaceId: spaceId(3),
      collapsed: false,
      sortKey: 'a0V',
      version: const ResourceVersion(1),
    ),
  );
  await db.structureDao.upsertUserSpaceStateIfNewer(
    UserSpaceState(
      spaceId: spaceId(2),
      collapsed: false,
      version: const ResourceVersion(1),
    ),
  );
  await db.structureDao.upsertChannelIfNewer(channel(1, name: 'general'));
  await db.structureDao.upsertChannelIfNewer(
    channel(2, name: 'ideas', kind: ChannelKind.forum),
  );
  await db.structureDao.upsertChannelIfNewer(
    channel(3, spaceN: 2, name: 'ops'),
  );
  await db.structureDao.upsertChannelIfNewer(
    channel(4, spaceN: 3, name: 'quiet'),
  );
  await db.personalDao.applyReadState(
    readState(1, lastRead: 2, unread: 3, mentions: 1),
  );
  await db.runsDao.upsertIfNewer(run(1, RunState.running));
}

double top(WidgetTester tester, int n) => tester
    .getTopLeft(find.byKey(ValueKey('sidebar.space.${spaceId(n).value}')))
    .dy;

void main() {
  testWidgets('personal order from sort_key, the global order as fallback', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host));
    await host.pump(tester, const SidebarColumn());
    expect(top(tester, 3), lessThan(top(tester, 1)));
    expect(top(tester, 1), lessThan(top(tester, 2)));
    await host.dispose(tester);
  });

  testWidgets('collapse writes one outbox entry and updates the row at once', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host));
    await host.pump(tester, const SidebarColumn());
    final general = find.byKey(
      ValueKey('sidebar.channel.${channelId(1).value}'),
    );
    expect(general, findsOneWidget);
    await tester.tap(find.byKey(ValueKey('sidebar.space.${spaceId(1).value}')));
    await host.settle(tester);
    expect(general, findsNothing, reason: 'the space is collapsed at once');
    final entries = await tester.runAsync(host.db.outboxDao.allEntries);
    expect(entries!.map((e) => e.kind), [SidebarOps.putMySidebarSpace]);
    expect(jsonDecode(entries.single.payload), {
      'space_id': spaceId(1).value,
      'collapsed': true,
    });
    final states = await tester.runAsync(
      () => host.db.structureDao.watchUserSpaceStates().first,
    );
    expect(
      states!.singleWhere((s) => s.spaceId == spaceId(1)).collapsed,
      isTrue,
    );
    await host.dispose(tester);
  });

  testWidgets('a move writes the new key of the space through the outbox', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host));
    await host.pump(tester, const SidebarColumn());
    // "Move down" on space 3 (the first): it goes between space 1 and space 2.
    final semantics = tester.ensureSemantics();
    final node = tester.getSemantics(
      find
          .ancestor(
            of: find.byKey(ValueKey('sidebar.space.${spaceId(3).value}')),
            matching: find.byType(Semantics),
          )
          .first,
    );
    final action = node.getSemanticsData().customSemanticsActionIds!.map(
      CustomSemanticsAction.getAction,
    );
    final down = action.singleWhere((a) => a!.label == en.sidebarMoveDown)!;
    node.owner!.performAction(
      node.id,
      SemanticsAction.customAction,
      CustomSemanticsAction.getIdentifier(down),
    );
    await host.settle(tester);
    expect(top(tester, 1), lessThan(top(tester, 3)));
    expect(top(tester, 3), lessThan(top(tester, 2)));
    final entries = await tester.runAsync(host.db.outboxDao.allEntries);
    final payload = jsonDecode(entries!.single.payload) as Map;
    expect(payload['space_id'], spaceId(3).value);
    expect(payload['sort_key'], FractionalIndex.between('a1', 'a2'));
    semantics.dispose();
    await host.dispose(tester);
  });

  testWidgets('tree semantics: expanded and collapsed spaces, channel labels', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host));
    await host.pump(tester, SidebarColumn(selectedChannel: channelId(3)));
    expect(
      tester.getSemantics(find.bySemanticsLabel(RegExp('^Space 1'))),
      isSemantics(hasExpandedState: true, isExpanded: true),
    );
    expect(
      find.bySemanticsLabel(
        'general, ${en.channelKindNameText}, ${en.threadRowUnread(3)}, '
        '${en.threadRowMentions(1)}, ${en.channelRowRunActive}',
      ),
      findsOneWidget,
    );
    expect(
      find.bySemanticsLabel('ideas, ${en.channelKindNameForum}'),
      findsOneWidget,
    );
    expect(
      tester.getSemantics(
        find.bySemanticsLabel('ops, ${en.channelKindNameText}'),
      ),
      isSemantics(isSelected: true),
    );
    await tester.tap(find.byKey(ValueKey('sidebar.space.${spaceId(2).value}')));
    await host.settle(tester);
    expect(
      tester.getSemantics(find.bySemanticsLabel(RegExp('^Space 2'))),
      isSemantics(hasExpandedState: true, isExpanded: false),
    );
    semantics.dispose();
    await host.dispose(tester);
  });

  testWidgets('a channel opens /c/<id>; the head shows the product name', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host));
    await host.pump(tester, const SidebarColumn());
    await tester.tap(
      find.byKey(ValueKey('sidebar.channel.${channelId(2).value}')),
    );
    await host.settle(tester);
    expect(host.visited.last, '/c/${channelId(2).value}');
    expect(find.byKey(const ValueKey('sidebar.productName')), findsOneWidget);
    expect(find.byKey(const ValueKey('sidebar.mark')), findsOneWidget);
    await host.dispose(tester);
  });
}
