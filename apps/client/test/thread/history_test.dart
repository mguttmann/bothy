// SPDX-License-Identifier: Apache-2.0

// The history of a thread over the local database (brief M4-20 Tests; 01 6.4; 04 4; 15 7.3
// item 4): grouping within `ui.message_group_window`, "Show more" over
// `ui.message_collapse_lines`, tombstones, system lines, origin badges, the gap row until the
// range is loaded, paging up that keeps the anchor and the jump to a message.

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

Finder messageKey(MessageId id) => find.byKey(ValueKey('message.${id.value}'));

DsMessage dsOf(WidgetTester tester, MessageId id) => tester.widget<DsMessage>(
  find.descendant(of: messageKey(id), matching: find.byType(DsMessage)),
);

void main() {
  testWidgets('grouping within the window, collapse, tombstone, system line, '
      'origin badges', (tester) async {
    final host = await ColumnHost.open(tester);
    final long = List.generate(40, (i) => 'line $i').join('\n\n');
    await host.run(
      tester,
      () => seedThread(
        host.db,
        messages: [
          msg(1, author: sam, minutes: 0),
          msg(2, author: sam, minutes: 1),
          msg(3, author: sam, minutes: 30),
          msg(4, author: robin, origin: MessageOrigin.agent, minutes: 31),
          msg(5, author: alex, text: long, minutes: 32),
          msg(6, author: sam, deleted: true, minutes: 33),
          msg(
            7,
            author: sam,
            kind: MessageKind.system,
            origin: MessageOrigin.system,
            systemCode: SystemCode.threadReopened,
            text: '',
            minutes: 34,
          ),
        ],
      ),
    );
    final reads = FakeThreadReads(host.db);
    await pumpThread(tester, host, reads);

    expect(dsOf(tester, messageId(1)).isContinuation, isFalse);
    expect(dsOf(tester, messageId(2)).isContinuation, isTrue);
    expect(dsOf(tester, messageId(3)).isContinuation, isFalse);
    expect(find.text(en.messageOriginAgent), findsOneWidget);
    expect(find.text(en.messageShowMore), findsOneWidget);
    expect(find.text(en.messageDeleted), findsOneWidget);
    expect(
      find.textContaining(en.messageSystemThreadReopened('').trim()),
      findsOneWidget,
    );
    await host.dispose(tester);
  });

  testWidgets('a gap shows a loading row until its range is loaded', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await host.run(tester, () async {
      await seedThread(
        host.db,
        messages: [msg(1), msg(2), msg(3)],
        th: thread(1, title: 'Printer jam', lastSeq: 6),
      );
      await host.db.messagesDao.upsertIfNewer(msg(6));
    });
    final reads = FakeThreadReads(host.db);
    await pumpThread(tester, host, reads);
    expect(find.byKey(const ValueKey('history.gapAfter')), findsOneWidget);
    expect(messageKey(messageId(6)), findsNothing);

    await host.run(tester, () async {
      await host.db.messagesDao.upsertIfNewer(msg(4));
      await host.db.messagesDao.upsertIfNewer(msg(5));
      await host.db.messagesDao.addLoadedRange(
        threadId(1),
        const ThreadSeq(4),
        const ThreadSeq(6),
      );
    });
    await host.settle(tester);
    expect(find.byKey(const ValueKey('history.gapAfter')), findsNothing);
    expect(messageKey(messageId(6)), findsOneWidget);
    final y4 = tester.getTopLeft(messageKey(messageId(4))).dy;
    final y6 = tester.getTopLeft(messageKey(messageId(6))).dy;
    expect(y4, lessThan(y6), reason: 'never out of order');
    await host.dispose(tester);
  });

  testWidgets('paging up loads older messages and keeps the anchor', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    final all = [for (var s = 1; s <= 60; s++) msg(s, minutes: s * 10)];
    await host.run(
      tester,
      () => seedThread(
        host.db,
        messages: all.sublist(40),
        th: thread(1, title: 'Printer jam', lastSeq: 60),
      ),
    );
    final reads = FakeThreadReads(host.db, archive: all);
    await pumpThread(tester, host, reads);
    expect(find.byKey(const ValueKey('history.gapBefore')), findsNothing);

    // Scroll to the oldest loaded message.
    final list = find.byKey(const ValueKey('history.list'));
    for (
      var i = 0;
      i < 40 && messageKey(messageId(41)).evaluate().isEmpty;
      i++
    ) {
      await tester.drag(list, const Offset(0, 400));
      await host.settle(tester);
    }
    expect(messageKey(messageId(41)), findsOneWidget);
    final before = tester.getTopLeft(messageKey(messageId(41))).dy;
    await tester.drag(list, const Offset(0, 50));
    await host.settle(tester);
    expect(reads.calls, contains('older 41'));
    await host.settle(tester);
    expect(
      tester.getTopLeft(messageKey(messageId(41))).dy,
      closeTo(before + 50, 60),
      reason: 'the anchor stays where the person scrolled it',
    );
    for (
      var i = 0;
      i < 20 && messageKey(messageId(40)).evaluate().isEmpty;
      i++
    ) {
      await tester.drag(list, const Offset(0, 200));
      await host.settle(tester);
    }
    expect(messageKey(messageId(40)), findsOneWidget);
    expect(
      tester.getTopLeft(messageKey(messageId(40))).dy,
      lessThan(tester.getTopLeft(messageKey(messageId(41))).dy),
    );
    await host.dispose(tester);
  });

  testWidgets('a jump to an older message loads the range and highlights it', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    final all = [for (var s = 1; s <= 60; s++) msg(s, minutes: s * 10)];
    await host.run(
      tester,
      () => seedThread(
        host.db,
        messages: all.sublist(50),
        th: thread(1, title: 'Printer jam', lastSeq: 60),
      ),
    );
    final reads = FakeThreadReads(host.db, archive: all);
    await pumpThread(tester, host, reads, jumpTo: messageId(12));
    await host.settle(tester);
    expect(reads.calls.where((c) => c.startsWith('range 12-')), isNotEmpty);
    expect(messageKey(messageId(12)), findsOneWidget);
    expect(
      find.descendant(
        of: messageKey(messageId(12)),
        matching: find.byKey(const ValueKey('history.highlight')),
      ),
      findsOneWidget,
    );
    await host.dispose(tester);
  });
}
