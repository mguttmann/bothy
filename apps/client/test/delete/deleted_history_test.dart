// SPDX-License-Identifier: Apache-2.0

// An open history whose chat, channel or space was deleted on another device closes (01 6.18
// item 8, D-DEL-5): a removal notice marked `deleted` replaces it with "This chat was
// deleted." instead of "No access anymore" and keeps the waiting messages as text (04 8.3
// item 4).

import 'dart:async';

import 'package:client/thread/history_column.dart';
import 'package:client/thread/providers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

void main() {
  for (final (label, target) in [
    ('chat', TargetRef.thread(threadId(1))),
    ('channel', TargetRef.channel(channelId(1))),
  ]) {
    testWidgets('a deleted $label closes the open history with the deletion '
        'sentence', (tester) async {
      final host = await ColumnHost.open(tester);
      await host.run(tester, () => seedThread(host.db, messages: [msg(1)]));
      final notices = StreamController<SyncNotice>.broadcast();
      addTearDown(notices.close);
      await host.run(
        tester,
        () => host.engine.outbox.enqueue(
          OutboxRequest(
            kind: MessageOps.createMessage,
            threadId: threadId(1),
            payload: {'content_md': 'Keep this draft text'},
          ),
        ),
      );
      await host.pump(
        tester,
        HistoryColumn(threadId: threadId(1)),
        location: '/t/${threadId(1).value}',
        overrides: [
          ...threadOverrides(FakeThreadReads(host.db)),
          syncNoticesProvider.overrideWith((ref) => notices.stream),
        ],
      );
      expect(
        find.byKey(ValueKey('message.${messageId(1).value}')),
        findsOneWidget,
      );
      notices.add(TargetsRemoved([target], deleted: true));
      await host.settle(tester);
      expect(find.byKey(const ValueKey('thread.deleted')), findsOneWidget);
      expect(find.text(en.threadDeleted), findsOneWidget);
      expect(find.text(en.threadNoAccess), findsNothing);
      expect(
        find.byKey(ValueKey('message.${messageId(1).value}')),
        findsNothing,
      );
      expect(
        find.widgetWithText(SelectableText, 'Keep this draft text'),
        findsOneWidget,
      );
      await host.dispose(tester);
    });
  }
}
