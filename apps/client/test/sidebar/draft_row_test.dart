// SPDX-License-Identifier: Apache-2.0

// The draft row "New chat" of a forum group in the desktop shell over the local database (brief
// M4-41 Tests; 01 6.8 item 8; D-M4-41-4): it heads the forum's group while the new chat view is
// open or a draft with text is stored, is selected while the view writes, and counts toward
// neither the bound (`ui.sidebar_threads_per_channel`) nor the counters; a second "+" opens the
// same draft; a sent post shows "Sending", a refused one "Not sent"; the row survives a
// restart; a collapsed group shows none.

import 'dart:convert';

import 'package:client/app/providers/ports.dart';
import 'package:client/sync/thread_ops.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/fakes.dart';
import '../support/new_chat_fixtures.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

/// Post [n] of [server], open, with recent activity, so it qualifies for the group.
Thread serverPost(int n) => thread(
  n,
  title: 'Post $n',
  activeMinutes: -10 * n,
  createdMinutes: -10 * n,
  lastSeq: 4,
);

/// The thread row of post [n].
Finder threadRow(int n) =>
    find.byKey(ValueKey('sidebar.thread.${threadId(n).value}'));

/// "All posts" of [forum].
Finder allPosts(Channel forum) =>
    find.byKey(ValueKey('sidebar.allPosts.${forum.id.value}'));

/// The config of the tests with `ui.sidebar_threads_per_channel` [bound].
Override boundConfig(int bound) => appConfigProvider.overrideWithValue(
  AppConfig.defaults().withOverrides({
    ...fastConfig,
    'ui.sidebar_threads_per_channel': '$bound',
  }),
);

/// Opens the host (on [database] when given) with the forums stored and the shell at
/// [location].
Future<ColumnHost> pumpShell(
  WidgetTester tester, {
  required String location,
  LocalDatabase? database,
  bool filled = false,
  Future<void> Function(LocalDatabase db)? more,
  List<Override> overrides = const [],
}) async {
  final host = await ColumnHost.open(tester, database: database);
  await tester.runAsync(() async {
    if (!filled) await fillForums(host.db);
    await more?.call(host.db);
  });
  await host.pump(
    tester,
    shell,
    location: location,
    overrides: shellOverrides(more: overrides),
  );
  return host;
}

Future<List<OutboxEntry>> outbox(WidgetTester tester, ColumnHost host) =>
    host.run(tester, host.db.outboxDao.allEntries);

DsSidebarDraftRow rowWidget(WidgetTester tester, Finder f) =>
    tester.widget<DsSidebarDraftRow>(f);

/// Types [text] into the composer of the view.
Future<void> type(WidgetTester tester, ColumnHost host, String text) async {
  await tester.enterText(composerField, text);
  await host.settle(tester);
}

/// The counters the forum row and "All posts" show.
({int unread, int mentions, int allUnread, int allMentions}) counters(
  WidgetTester tester,
  Channel forum,
) {
  final row = tester.widget<DsChannelRow>(channelRow(forum)).data;
  final all = tester.widget<DsSidebarAllPostsRow>(allPosts(forum));
  return (
    unread: row.unreadCount,
    mentions: row.mentionCount,
    allUnread: all.unreadCount,
    allMentions: all.mentionCount,
  );
}

void main() {
  testWidgets(
    'the draft row heads the forum group while the view writes and is selected',
    (tester) async {
      final host = await pumpShell(
        tester,
        location: forumPath(server),
        more: (db) async {
          for (final n in [1, 2]) {
            await db.threadsDao.upsertIfNewer(serverPost(n));
          }
        },
      );
      expect(draftRow(server), findsNothing, reason: 'no draft, no view');
      await tester.tap(find.byTooltip(en.sidebarNewChatIn('server')));
      await host.settle(tester);
      expect(host.visited.last, newChatPath(server));
      expect(draftRow(server), findsOneWidget);
      expect(rowWidget(tester, draftRow(server)).selected, isTrue);
      expect(rowWidget(tester, draftRow(server)).state, DsDraftRowState.draft);
      expect(
        topOf(tester, draftRow(server)),
        greaterThan(topOf(tester, channelRow(server))),
      );
      expect(
        topOf(tester, draftRow(server)),
        lessThan(topOf(tester, threadRow(1))),
      );
      expect(
        topOf(tester, draftRow(server)),
        lessThan(topOf(tester, threadRow(2))),
      );
      // One screen reader node "New chat, Draft" (components.md 7).
      expect(
        find.bySemanticsLabel('${en.sidebarNewChat}, ${en.sidebarDraftState}'),
        findsOneWidget,
      );
      await host.dispose(tester);
    },
  );

  testWidgets(
    'a stored draft with text gives the row outside the view, not selected',
    (tester) async {
      final host = await pumpShell(
        tester,
        location: forumPath(ideas),
        more: (db) async {
          await db.personalDao.upsertDraftIfNewer(
            storedDraft(server.id, 'An idea'),
          );
          // A blank draft of another forum gives no row.
          await db.personalDao.upsertDraftIfNewer(
            storedDraft(ideas.id, '  ', n: 2),
          );
        },
      );
      expect(draftRow(server), findsOneWidget);
      expect(rowWidget(tester, draftRow(server)).selected, isFalse);
      expect(draftRow(ideas), findsNothing);
      // The row opens the view of its forum with the draft.
      await tester.tap(draftRow(server));
      await host.settle(tester);
      expect(host.visited.last, newChatPath(server));
      expect(composerText(tester), 'An idea');
      expect(rowWidget(tester, draftRow(server)).selected, isTrue);
      await host.dispose(tester);
    },
  );

  testWidgets('the draft row counts toward neither the bound nor the counters', (
    tester,
  ) async {
    final host = await pumpShell(
      tester,
      location: '/',
      overrides: [boundConfig(2)],
      more: (db) async {
        for (final n in [1, 2, 3, 4]) {
          await db.threadsDao.upsertIfNewer(serverPost(n));
        }
        // Unread with a mention on a shown post and on posts beyond the bound.
        await db.personalDao.applyReadState(
          readState(1, lastRead: 2, unread: 2, mentions: 1),
        );
        await db.personalDao.applyReadState(
          readState(3, lastRead: 1, unread: 3, mentions: 1),
        );
        await db.personalDao.applyReadState(
          readState(4, lastRead: 3, unread: 1),
        );
      },
    );
    final shownBefore = [
      for (final n in [1, 2, 3, 4])
        if (threadRow(n).evaluate().isNotEmpty) n,
    ];
    expect(shownBefore, hasLength(2), reason: 'the bound of 2');
    final before = counters(tester, server);
    expect(before.allUnread + before.unread, greaterThan(0));

    await host.run(
      tester,
      () => host.db.personalDao.upsertDraftIfNewer(
        storedDraft(server.id, 'An idea'),
      ),
    );
    await host.settle(tester);
    expect(draftRow(server), findsOneWidget);
    final shownAfter = [
      for (final n in [1, 2, 3, 4])
        if (threadRow(n).evaluate().isNotEmpty) n,
    ];
    expect(shownAfter, shownBefore, reason: 'still two thread rows');
    expect(counters(tester, server), before);
    await host.dispose(tester);
  });

  testWidgets('a second "+" opens the same draft', (tester) async {
    final host = await pumpShell(tester, location: forumPath(server));
    await tester.tap(find.byTooltip(en.sidebarNewChatIn('server')));
    await host.settle(tester);
    await type(tester, host, 'Draft one');
    // "+" again while the view is open: the same draft, the same route.
    await tester.tap(find.byTooltip(en.sidebarNewChatIn('server')));
    await host.settle(tester);
    expect(host.visited.last, newChatPath(server));
    expect(newChatRows(server), findsOneWidget);
    expect(composerText(tester), 'Draft one');

    // Left with text and opened again: the draft comes back, still one row and one write.
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await host.settle(tester);
    expect(host.visited.last, forumPath(server));
    expect(newChatRows(server), findsOneWidget);
    await tester.tap(find.byTooltip(en.sidebarNewChatIn('server')));
    await host.settle(tester);
    expect(host.visited.last, newChatPath(server));
    expect(newChatRows(server), findsOneWidget);
    expect(composerText(tester), 'Draft one');
    expect(draftWrites(await outbox(tester, host)), hasLength(1));
    await host.dispose(tester);
  });

  testWidgets(
    'a sent post shows "Sending" and is selected on its view, a refused one "Not sent"',
    (tester) async {
      final request = ThreadOps.postRequest(
        forum: server.id,
        title: '',
        contentMd: 'Restart the gateway',
        clearDraft: true,
      );
      final entry = uuid(0x801);
      final host = await pumpShell(
        tester,
        location: newChatPath(server, entry: entry),
        more: (db) => db.outboxDao.enqueue(
          id: entry,
          kind: request.kind,
          payload: jsonEncode(request.payload),
        ),
      );
      final row = sentRow(server, entry);
      expect(row, findsOneWidget);
      expect(rowWidget(tester, row).state, DsDraftRowState.sending);
      expect(rowWidget(tester, row).selected, isTrue);
      expect(
        find.descendant(of: row, matching: find.text(en.messageSending)),
        findsOneWidget,
      );
      expect(
        topOf(tester, row),
        greaterThan(topOf(tester, channelRow(server))),
      );

      await host.run(
        tester,
        () => host.db.outboxDao.recordFailure(
          entry,
          code: 'forbidden',
          reason: 'restricted tag',
        ),
      );
      await host.settle(tester);
      expect(rowWidget(tester, row).state, DsDraftRowState.failed);
      expect(
        find.descendant(of: row, matching: find.text(en.sidebarDraftNotSent)),
        findsOneWidget,
      );
      await host.dispose(tester);
    },
  );

  testWidgets('a stored draft in a reopened database gives the row', (
    tester,
  ) async {
    final db = await tester.runAsync(
      () async => openInMemoryForTests(clock: () => t0),
    );
    var host = await pumpShell(
      tester,
      location: '/',
      database: db,
      more: (db) => db.personalDao.upsertDraftIfNewer(
        storedDraft(server.id, 'From another device'),
      ),
    );
    expect(draftRow(server), findsOneWidget);
    await host.dispose(tester);

    // A new start of the app on this device: the same database, new providers.
    host = await pumpShell(tester, location: '/', database: db, filled: true);
    expect(draftRow(server), findsOneWidget);
    await tester.tap(draftRow(server));
    await host.settle(tester);
    expect(composerText(tester), 'From another device');
    await host.dispose(tester);
    await tester.runAsync(db!.close);
  });

  testWidgets(
    'an unsent draft write in the outbox of a reopened database gives the row',
    (tester) async {
      final db = await tester.runAsync(
        () async => openInMemoryForTests(clock: () => t0),
      );
      var host = await pumpShell(
        tester,
        location: newChatPath(server),
        database: db,
      );
      await type(tester, host, 'Typed offline');
      expect(draftWrites(await outbox(tester, host)), hasLength(1));
      await host.dispose(tester);

      // A new start: nothing is stored on the server yet, only the outbox entry waits.
      host = await pumpShell(tester, location: '/', database: db, filled: true);
      expect(await host.run(tester, host.db.personalDao.allDrafts), isEmpty);
      expect(draftRow(server), findsOneWidget);
      await tester.tap(draftRow(server));
      await host.settle(tester);
      expect(composerText(tester), 'Typed offline');
      await host.dispose(tester);
      await tester.runAsync(db!.close);
    },
  );

  testWidgets('a draft row of a collapsed group does not show', (tester) async {
    final host = await pumpShell(
      tester,
      location: '/',
      more: (db) async {
        await db.threadsDao.upsertIfNewer(serverPost(1));
        await db.personalDao.upsertDraftIfNewer(
          storedDraft(server.id, 'An idea'),
        );
        await db.forumViewDao.setGroupCollapsed(server.id, collapsed: true);
      },
    );
    expect(channelRow(server), findsOneWidget);
    expect(draftRow(server), findsNothing);
    expect(threadRow(1), findsNothing);
    // "+" expands the group first, so the draft row shows and is selected (D-M4-41-4).
    await tester.tap(channelRow(server));
    await host.settle(tester);
    await tester.tap(find.byTooltip(en.sidebarNewChatIn('server')));
    await host.settle(tester);
    expect(draftRow(server), findsOneWidget);
    expect(rowWidget(tester, draftRow(server)).selected, isTrue);
    await host.dispose(tester);
  });
}
