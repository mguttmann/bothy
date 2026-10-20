// SPDX-License-Identifier: Apache-2.0

// The offline states of the thread view (brief M4-20 Tests; 01 6.6; 04 13.2, 8.3): the offline
// hint of the input, "Sending" right after the last confirmed message, "Sending delayed",
// failed with the reason and "Send again", "Edit" and "Discard", and "No access anymore" with
// the waiting messages as copyable text.

import 'dart:async';

import 'package:client/thread/history_column.dart';
import 'package:client/thread/providers.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

Future<String> queue(WidgetTester tester, ColumnHost host, String text) =>
    host.run(tester, () async {
      final e = await host.engine.outbox.enqueue(
        OutboxRequest(
          kind: MessageOps.createMessage,
          threadId: threadId(1),
          payload: {'content_md': text, 'clear_draft': true},
        ),
      );
      return e.id;
    });

DsMessage pending(WidgetTester tester, String id) => tester.widget<DsMessage>(
  find.descendant(
    of: find.byKey(ValueKey('outbox.$id')),
    matching: find.byType(DsMessage),
  ),
);

void main() {
  testWidgets('offline hint, sending after the last message, delayed, failed '
      'with three actions', (tester) async {
    var now = t0;
    final db = openInMemoryForTests(clock: () => now);
    final host = await ColumnHost.open(tester, database: db);
    await host.run(
      tester,
      () => seedThread(host.db, messages: [msg(1), msg(2, minutes: 5)]),
    );
    await pumpThread(tester, host, FakeThreadReads(host.db));
    expect(find.text(en.composerOffline), findsOneWidget);

    final first = await queue(tester, host, 'Queued while offline');
    await host.settle(tester);
    expect(pending(tester, first).data.delivery, DsDelivery.sending);
    expect(find.text(en.messageSending), findsOneWidget);
    expect(
      tester.getTopLeft(find.byKey(ValueKey('outbox.$first'))).dy,
      greaterThan(
        tester
            .getTopLeft(find.byKey(ValueKey('message.${messageId(2).value}')))
            .dy,
      ),
    );

    // Backing off for longer than client.outbox_stuck_after: "Sending delayed".
    await host.run(tester, () async {
      await host.db.outboxDao.markState(first, OutboxState.waiting);
      now = t0.add(const Duration(minutes: 10));
    });
    final second = await queue(tester, host, 'Nudge');
    await host.settle(tester);
    expect(pending(tester, first).data.delivery, DsDelivery.delayed);
    expect(find.text(en.messageSendingDelayed), findsOneWidget);

    // Refused for good: the reason with Send again, Edit and Discard.
    await host.run(
      tester,
      () => host.db.outboxDao.recordFailure(second, code: 'payload_too_large'),
    );
    await host.settle(tester);
    final failed = pending(tester, second);
    expect(failed.data.delivery, DsDelivery.failed);
    expect(failed.data.failureText, isNotEmpty);
    expect(find.text(en.messageResend), findsOneWidget);
    expect(find.text(en.messageEdit), findsOneWidget);
    expect(find.text(en.messageDiscard), findsOneWidget);

    await tester.tap(find.text(en.messageResend));
    await host.settle(tester);
    final after = await host.run(tester, () => host.db.outboxDao.allEntries());
    expect(after.any((e) => e.id == second), isFalse, reason: 'a new ID');
    expect(
      after.where((e) => decodePayload(e.payload)['content_md'] == 'Nudge'),
      hasLength(1),
    );
    await host.dispose(tester);
    await db.close();
  });

  testWidgets('a removed thread closes with No access anymore and keeps the '
      'waiting text', (tester) async {
    final host = await ColumnHost.open(tester);
    await host.run(tester, () => seedThread(host.db, messages: [msg(1)]));
    final notices = StreamController<SyncNotice>.broadcast();
    addTearDown(notices.close);
    await queue(tester, host, 'Please keep this text');
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
    notices.add(TargetsRemoved([TargetRef.channel(channelId(1))]));
    await host.settle(tester);
    expect(find.byKey(const ValueKey('thread.noAccess')), findsOneWidget);
    expect(find.text(en.threadNoAccess), findsOneWidget);
    expect(find.byKey(ValueKey('message.${messageId(1).value}')), findsNothing);
    final text = find.widgetWithText(SelectableText, 'Please keep this text');
    expect(text, findsOneWidget);
    await host.dispose(tester);
  });
}
