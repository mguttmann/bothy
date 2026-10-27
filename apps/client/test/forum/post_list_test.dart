// SPDX-License-Identifier: Apache-2.0

// The post list over the local database (brief M4-19 Tests; components.md 6; 01 6.1): sort
// by activity and creation, the filters Open, Waiting for me and Tags, the Done section with
// its count and its collapse state that survives a restart on this device, the row labels.

import 'dart:convert';

import 'package:client/forum/forum_model.dart';
import 'package:client/forum/post_list_column.dart';
import 'package:client/forum/providers.dart';
import 'package:client/sidebar/providers.dart';
import 'package:client/sync/thread_ops.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

/// No paging in these tests: every post is stored already.
class NoPages extends ForumPager {
  @override
  PagerState build(String channelId, String sort) =>
      const PagerState(hasMore: false);

  @override
  Future<void> loadMore() async {}
}

final List<Override> overrides = [
  forumPagerProvider.overrideWith2((_) => NoPages()),
  ownPrincipalProvider.overrideWith(
    (ref) async => Principal(
      id: alex,
      kind: PrincipalKind.user,
      handle: 'alex',
      displayName: 'Alex',
    ),
  ),
];

Future<void> fill(LocalDatabase db) async {
  await db.structureDao.upsertSpaceIfNewer(space(1));
  await db.structureDao.upsertChannelIfNewer(
    channel(1, kind: ChannelKind.forum, name: 'ideas'),
  );
  await db.structureDao.putForumTags(channelId(1), [
    tag(1, 'bug', color: 'clay'),
    tag(2, 'idea'),
  ]);
  for (final t in [
    thread(1, activeMinutes: 50, createdMinutes: 1, tags: [tagId(1)]),
    thread(2, activeMinutes: 10, createdMinutes: 2, lastSeq: 4),
    thread(3, activeMinutes: 40, createdMinutes: 3, state: ThreadState.closed),
    thread(4, activeMinutes: 30, createdMinutes: 4, tags: [tagId(2)]),
  ]) {
    await db.threadsDao.upsertIfNewer(t);
  }
  // Post 2: my run waits for me; post 4: an unread mention of me.
  await db.runsDao.upsertIfNewer(
    run(1, RunState.waitingInput, threadN: 2, triggeredBy: alex),
  );
  await db.personalDao.applyReadState(
    readState(4, lastRead: 0, unread: 2, mentions: 1),
  );
}

List<int> order(WidgetTester tester) {
  final rows = <(double, int)>[];
  for (var n = 1; n <= 4; n++) {
    final f = find.byKey(ValueKey('postList.post.${threadId(n).value}'));
    if (f.evaluate().isNotEmpty) rows.add((tester.getTopLeft(f).dy, n));
  }
  rows.sort((a, b) => a.$1.compareTo(b.$1));
  return [for (final r in rows) r.$2];
}

Future<ColumnHost> pumpList(
  WidgetTester tester, {
  LocalDatabase? database,
  bool filled = false,
}) async {
  final host = await ColumnHost.open(tester, database: database);
  if (!filled) await tester.runAsync(() => fill(host.db));
  await host.pump(
    tester,
    PostListColumn(channelId: channelId(1)),
    overrides: overrides,
  );
  return host;
}

/// The row whose label starts with [label] (the row's badges and tags follow it).
Finder labelled(String label) =>
    find.bySemanticsLabel(RegExp('^${RegExp.escape(label)}(\n|\$)'));

void main() {
  testWidgets('sorts by last activity, and by creation when chosen', (
    tester,
  ) async {
    final host = await pumpList(tester);
    expect(order(tester), [1, 4, 2]);
    await tester.tap(find.byKey(const ValueKey('postList.filter.sort')));
    await host.settle(tester);
    await tester.tap(find.byKey(const ValueKey('postList.sort.created')));
    await host.settle(tester);
    expect(order(tester), [4, 2, 1]);
    await host.dispose(tester);
  });

  testWidgets('filters Open, Waiting for me and Tags', (tester) async {
    final host = await pumpList(tester);
    await tester.tap(find.byKey(const ValueKey('postList.filter.waiting')));
    await host.settle(tester);
    expect(order(tester), [4, 2]);
    await tester.tap(find.byKey(const ValueKey('postList.filter.open')));
    await host.settle(tester);
    expect(order(tester), [1, 4, 2]);
    await tester.tap(find.byKey(const ValueKey('postList.filter.tags')));
    await host.settle(tester);
    await tester.tap(
      find.byKey(ValueKey('postList.tag.${tagId(1).value}')).first,
    );
    await host.settle(tester);
    expect(order(tester), [1]);
    expect(find.text(en.postListFilterTagsCount(1)), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets(
    'the Done section counts, collapses and keeps its state across a restart',
    (tester) async {
      final db = await tester.runAsync(
        () async => openInMemoryForTests(clock: () => t0),
      );
      var host = await pumpList(tester, database: db);
      expect(find.text(en.postListDone(1)), findsOneWidget);
      expect(order(tester), isNot(contains(3)), reason: 'collapsed by default');
      await tester.tap(find.byKey(const ValueKey('postList.done')));
      await host.settle(tester);
      expect(order(tester), [1, 4, 2, 3]);
      await host.dispose(tester);

      // A new start of the app on this device: the same database, new providers.
      host = await pumpList(tester, database: db, filled: true);
      expect(order(tester), [1, 4, 2, 3]);
      await tester.tap(find.byKey(const ValueKey('postList.done')));
      await host.settle(tester);
      expect(order(tester), [1, 4, 2]);
      await host.dispose(tester);
      await tester.runAsync(db!.close);
    },
  );

  testWidgets('a refused new post shows with Try again and Discard', (
    tester,
  ) async {
    final host = await pumpList(tester);
    // The database streams of the widgets run in fake time: write there and let both clocks
    // run until the write completes.
    final entry = await host.run(
      tester,
      () => ThreadOps.enqueue(
        host.engine,
        forum: channelId(1),
        title: 'Printer jam',
        contentMd: 'It stops after one page.',
      ),
    );
    await host.run(
      tester,
      () => host.db.outboxDao.markState(entry.id, OutboxState.failed),
    );
    await host.settle(tester);
    expect(
      find.text(en.postListCreateFailed(dsIsolate('Printer jam'))),
      findsOneWidget,
    );
    await tester.tap(find.byKey(ValueKey('postList.failed.retry.${entry.id}')));
    await host.settle(tester);
    final after = await tester.runAsync(host.db.outboxDao.allEntries);
    expect(
      after!.single.id,
      isNot(entry.id),
      reason: 'sent again with a new ID',
    );
    expect(after.single.state, isNot(OutboxState.failed));
    expect(
      find.text(en.postListCreateFailed(dsIsolate('Printer jam'))),
      findsNothing,
    );

    await host.run(
      tester,
      () => host.db.outboxDao.markState(after.single.id, OutboxState.failed),
    );
    await host.settle(tester);
    await tester.tap(
      find.byKey(ValueKey('postList.failed.discard.${after.single.id}')),
    );
    await host.settle(tester);
    expect(await host.run(tester, host.db.outboxDao.allEntries), isEmpty);
    expect(
      find.text(en.postListCreateFailed(dsIsolate('Printer jam'))),
      findsNothing,
    );
    await host.dispose(tester);
  });

  testWidgets('a refused new post shows its reason and can be edited', (
    tester,
  ) async {
    final host = await pumpList(tester);
    final entry = await host.run(
      tester,
      () => ThreadOps.enqueue(
        host.engine,
        forum: channelId(1),
        title: 'Release plan',
        contentMd: 'Version 2.',
        tagIds: [tagId(1)],
      ),
    );
    await host.run(tester, () async {
      await host.db.outboxDao.recordFailure(
        entry.id,
        code: 'forbidden',
        reason: 'restricted tag',
      );
    });
    await host.settle(tester);
    expect(
      find.text(en.postListFailureReason('restricted tag')),
      findsOneWidget,
    );
    await tester.tap(find.byKey(ValueKey('postList.failed.edit.${entry.id}')));
    await host.settle(tester);
    expect(find.text('Release plan'), findsWidgets, reason: 'prefilled');
    await tester.tap(find.byKey(ValueKey('newPost.tag.${tagId(1).value}')));
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('newPost.create')));
    await host.settle(tester);
    final after = await host.run(tester, host.db.outboxDao.allEntries);
    expect(after.single.id, isNot(entry.id));
    expect(after.single.state, isNot(OutboxState.failed));
    expect(jsonDecode(after.single.payload), containsPair('tag_ids', isEmpty));
    await host.dispose(tester);
  });

  testWidgets('after a failed page only scrolling loads again', (tester) async {
    var calls = 0;
    Future<int> nearEndCalls({required bool autoLoad}) async {
      calls = 0;
      await tester.pumpWidget(
        hostScreen(
          PostListView(
            forum: channel(1, kind: ChannelKind.forum),
            filter: const PostListFilter(),
            model: const PostListModel(open: [], done: []),
            tags: const [],
            names: const {},
            now: t0,
            doneCollapsed: true,
            morePages: true,
            autoLoad: autoLoad,
            onNearEnd: () => calls++,
            onNewPost: () {},
            onView: (_) {},
            onTags: (_) {},
            onSort: (_) {},
            onToggleDone: (_) {},
            onOpenPost: (_) {},
          ),
        ),
      );
      for (var i = 0; i < 5; i++) {
        await tester.pump();
      }
      return calls;
    }

    desktopWindow(tester);
    expect(await nearEndCalls(autoLoad: true), greaterThan(0));
    await tester.pumpWidget(const SizedBox());
    expect(await nearEndCalls(autoLoad: false), 0);
    // A list that cannot scroll still has a way: the row of the failed page tries again.
    expect(find.text(en.postListLoadFailed), findsOneWidget);
    final before = await nearEndCalls(autoLoad: false);
    await tester.tap(find.byKey(const ValueKey('postList.loadFailed.retry')));
    await tester.pump();
    expect(before, 0);
    expect(calls, 1);
  });

  // On a phone a post opened by its address (a deep link or a tapped notification) covers the
  // forum's list from the first frame: the list is built under the opaque page but never laid
  // out, so its scroll position has no extent yet and asks for no page (brief M5-11).
  testWidgets(
    'a list under an opaque page asks for no page and throws nothing',
    (tester) async {
      var calls = 0;
      final navigator = GlobalKey<NavigatorState>();
      final list = PostListView(
        forum: channel(1, kind: ChannelKind.forum),
        filter: const PostListFilter(),
        model: const PostListModel(open: [], done: []),
        tags: const [],
        names: const {},
        now: t0,
        doneCollapsed: true,
        morePages: true,
        onNearEnd: () => calls++,
        onNewPost: () {},
        onView: (_) {},
        onTags: (_) {},
        onSort: (_) {},
        onToggleDone: (_) {},
        onOpenPost: (_) {},
      );
      await tester.pumpWidget(
        hostScreen(
          Navigator(
            key: navigator,
            onGenerateInitialRoutes: (_, _) => [
              MaterialPageRoute<void>(builder: (_) => Material(child: list)),
              MaterialPageRoute<void>(builder: (_) => const SizedBox()),
            ],
          ),
        ),
      );
      for (var i = 0; i < 5; i++) {
        await tester.pump();
      }
      expect(tester.takeException(), isNull);
      expect(calls, 0);

      // Back on the list (the post closed), its first layout asks for the next page without a
      // scroll: a list too short to scroll would otherwise never ask.
      navigator.currentState!.pop();
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      expect(calls, greaterThan(0));
    },
  );

  testWidgets('the list starts at its kept offset and reports where it stops', (
    tester,
  ) async {
    final threads = [
      for (var n = 1; n <= 60; n++) thread(n, activeMinutes: -n),
    ];
    final forum = channel(1, kind: ChannelKind.forum);
    final model = buildPostList(
      threads: threads,
      channel: forum,
      filter: const PostListFilter(),
      counts: const {},
      runs: const [],
      me: alex,
    );
    double? reported;
    desktopWindow(tester);
    await tester.pumpWidget(
      hostScreen(
        PostListView(
          forum: forum,
          filter: const PostListFilter(),
          model: model,
          tags: const [],
          names: const {},
          now: t0,
          doneCollapsed: true,
          morePages: false,
          initialOffset: 600,
          onOffset: (o) => reported = o,
          onNewPost: () {},
          onView: (_) {},
          onTags: (_) {},
          onSort: (_) {},
          onToggleDone: (_) {},
          onOpenPost: (_) {},
        ),
      ),
    );
    await tester.pump();
    expect(
      find.byKey(ValueKey('postList.post.${threadId(1).value}')),
      findsNothing,
    );
    await tester.drag(
      find.byKey(const ValueKey('postList.list')),
      const Offset(0, 200),
    );
    await tester.pumpAndSettle();
    expect(reported, inInclusiveRange(380, 420), reason: '600 minus the drag');
  });

  testWidgets('row labels: title, state, time, unread', (tester) async {
    final semantics = tester.ensureSemantics();
    final host = await pumpList(tester);
    final format = L10nFormat(const Locale('en'));
    String time(int minutes) => timeLabel(
      t0.add(Duration(minutes: minutes)),
      now: t0,
      format: format,
    );
    expect(
      labelled(
        en.threadRowLabel(
          'Post 4',
          en.threadRowStateOpen,
          time(30),
          en.threadRowUnread(2),
        ),
      ),
      findsOneWidget,
    );
    expect(
      labelled(
        en.threadRowLabel(
          'Post 2',
          en.liveBoxStateWaitingForYou,
          time(10),
          en.threadRowUnread(0),
        ),
      ),
      findsOneWidget,
    );
    await tester.tap(
      find.byKey(ValueKey('postList.post.${threadId(1).value}')),
    );
    await host.settle(tester);
    expect(host.visited.last, '/t/${threadId(1).value}');
    semantics.dispose();
    await host.dispose(tester);
  });
}
