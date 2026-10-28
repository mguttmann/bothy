// SPDX-License-Identifier: Apache-2.0

// The view of a sent new post (brief M4-41 requirement 14; 01 6.6): it waits while the outbox
// holds the entry, also when the list of the forum's waiting posts it watches still predates the
// send, opens the new post's history once the server created it, and leaves to the overview
// only when the entry is gone without this run seeing its answer.

import 'package:client/forum/new_chat_view.dart';
import 'package:client/forum/providers.dart';
import 'package:client/sync/thread_ops.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';

void main() {
  final forum = channel(1, kind: ChannelKind.forum, name: 'server');
  final overview = '/c/${forum.id.value}';

  Future<ColumnHost> openHost(WidgetTester tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      await host.db.structureDao.upsertSpaceIfNewer(space(1));
      await host.db.structureDao.upsertChannelIfNewer(forum);
    });
    return host;
  }

  /// Pumps the view of the sent [entry]; the forum's waiting posts read [pending], as a list
  /// the outbox's stream gave before the send.
  Future<void> pumpSent(
    WidgetTester tester,
    ColumnHost host,
    String entry, {
    required CreatedPosts created,
    List<PendingPost> pending = const [],
  }) => host.pump(
    tester,
    NewChatView(forum: forum.id, sentEntry: entry),
    location: '$overview/new?entry=$entry',
    overrides: [
      createdPostsProvider.overrideWithValue(created),
      pendingPostsProvider(forum.id.value)
          .overrideWith((ref) => Stream.value(pending)),
    ],
  );

  testWidgets(
    'a waiting entry keeps the view while the list predates the send, then the history opens',
    (tester) async {
      final host = await openHost(tester);
      final entry = await host.run(
        tester,
        () => host.engine.outbox.enqueue(
          ThreadOps.postRequest(
            forum: forum.id,
            title: '',
            contentMd: 'Fix the login page',
            clearDraft: true,
          ),
        ),
      );
      final created = CreatedPosts();
      await pumpSent(tester, host, entry.id, created: created);
      expect(find.byKey(const ValueKey('newChat.sending')), findsOneWidget);
      expect(host.visited, isNot(contains(overview)));
      created.record(entry.id, threadId(7));
      await host.settle(tester);
      expect(host.visited.last, '/t/${threadId(7).value}');
      expect(host.visited, isNot(contains(overview)));
      await host.dispose(tester);
    },
  );

  testWidgets(
    'an entry gone without its answer in this run leaves to the overview',
    (tester) async {
      final host = await openHost(tester);
      await pumpSent(
        tester,
        host,
        '01a18171-cbe5-72dc-92e0-f704ca0ea3ec',
        created: CreatedPosts(),
      );
      expect(host.visited.last, overview);
      await host.dispose(tester);
    },
  );
}
