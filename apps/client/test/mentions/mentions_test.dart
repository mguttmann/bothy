// SPDX-License-Identifier: Apache-2.0

// The mentions view (brief M4-23 Goal and requirement 16; 01 3.5): the person's own mentions
// from `listMyMentions` with path and author from the local database, paging with the cursor,
// the jump to the message, and a failure with "Try again".

import 'package:client/mentions/mentions_view.dart';
import 'package:client/mentions/providers.dart';
import 'package:design_system/design_system.dart' show dsIsolate;
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/until.dart';
import '../support/widgets.dart';

Message mention(int seq, String text) =>
    msg(seq, author: sam, text: text, mentions: [alex]);

Future<void> fill(LocalDatabase db) async {
  await db.structureDao.upsertSpaceIfNewer(space(1, name: 'Engineering'));
  await db.structureDao.upsertChannelIfNewer(channel(1, name: 'server'));
  await db.threadsDao.upsertIfNewer(thread(1, title: 'Gateway resume'));
  for (final p in people) {
    await db.principalsDao.put(p);
  }
}

Finder entry(int seq) =>
    find.byKey(ValueKey('mentions.entry.${messageId(seq).value}'));

void main() {
  testWidgets(
    'lists mentions with path and author, pages and jumps to the message',
    (tester) async {
      final cursors = <String?>[];
      Future<({List<Message> messages, String? nextCursor})> source(
        String? cursor,
      ) async {
        cursors.add(cursor);
        return cursor == null
            ? (messages: [mention(2, '@Alex please look')], nextCursor: 'c2')
            : (messages: [mention(1, '@Alex older one')], nextCursor: null);
      }

      final host = await ColumnHost.open(tester);
      await tester.runAsync(() => fill(host.db));
      await host.pump(
        tester,
        const MentionsView(),
        overrides: [mentionsSourceProvider.overrideWithValue(source)],
      );
      await pumpUntil(
        tester,
        () => entry(2).evaluate().isNotEmpty,
        what: 'page 1',
      );
      expect(
        find.descendant(
          of: entry(2),
          matching: find.text(
            [
              'Engineering',
              'server',
              'Gateway resume',
            ].map(dsIsolate).join(' > '),
          ),
        ),
        findsOneWidget,
      );
      expect(
        find.descendant(of: entry(2), matching: find.text('@Alex please look')),
        findsOneWidget,
      );
      expect(entry(1), findsNothing);
      await tester.tap(find.byKey(const ValueKey('mentions.loadMore')));
      await pumpUntil(
        tester,
        () => entry(1).evaluate().isNotEmpty,
        what: 'page 2',
      );
      expect(cursors, [null, 'c2']);
      expect(find.byKey(const ValueKey('mentions.loadMore')), findsNothing);

      await tester.tap(entry(2));
      await host.settle(tester);
      expect(
        host.visited.last,
        '/t/${threadId(1).value}?m=${messageId(2).value}',
      );
      await host.dispose(tester);
    },
  );

  testWidgets('shows the empty text and a failure with Try again', (
    tester,
  ) async {
    var fail = true;
    Future<({List<Message> messages, String? nextCursor})> source(
      String? cursor,
    ) async {
      if (fail) throw StateError('offline');
      return (messages: const <Message>[], nextCursor: null);
    }

    final host = await ColumnHost.open(tester);
    await host.pump(
      tester,
      const MentionsView(),
      overrides: [mentionsSourceProvider.overrideWithValue(source)],
    );
    await pumpUntil(
      tester,
      () => find.text(en.mentionsFailed).evaluate().isNotEmpty,
      what: 'the failure',
    );
    fail = false;
    await tester.tap(find.byKey(const ValueKey('mentions.retry')));
    await pumpUntil(
      tester,
      () => find.text(en.mentionsEmpty).evaluate().isNotEmpty,
      what: 'the empty text',
    );
    await host.dispose(tester);
  });
}
