// SPDX-License-Identifier: Apache-2.0

// The findings of the code review of M4-20: stored messages beyond the limit stay reachable,
// a window behind another app marks nothing read, Esc marks read after "Mark as unread", a
// second edit joins the first, and a removal in D2 closes both columns with "No access
// anymore".

import 'dart:async';

import 'package:client/thread/history_area.dart';
import 'package:client/thread/providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';

Finder messageKey(MessageId id) => find.byKey(ValueKey('message.${id.value}'));

Future<List<OutboxEntry>> entriesOf(
  WidgetTester tester,
  ColumnHost host,
  String kind,
) async => [
  for (final e in await host.run(tester, () => host.db.outboxDao.allEntries()))
    if (e.kind == kind) e,
];

void main() {
  testWidgets('stored messages beyond the limit show when scrolled up', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    final all = [for (var s = 1; s <= 70; s++) msg(s, minutes: s * 10)];
    await host.run(tester, () => seedThread(host.db, messages: all));
    final reads = FakeThreadReads(host.db);
    await pumpThread(tester, host, reads);
    final list = find.byKey(const ValueKey('history.list'));
    for (
      var i = 0;
      i < 60 && messageKey(messageId(1)).evaluate().isEmpty;
      i++
    ) {
      await tester.drag(list, const Offset(0, 600));
      await host.settle(tester);
    }
    expect(messageKey(messageId(1)), findsOneWidget);
    expect(reads.calls.where((c) => c.startsWith('older')), isEmpty);
    await host.dispose(tester);
  });

  testWidgets('a window without focus marks nothing read', (tester) async {
    final host = await ColumnHost.open(tester);
    await host.run(tester, () => seedThread(host.db, messages: [msg(1)]));
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    addTearDown(
      () => tester.binding.handleAppLifecycleStateChanged(
        AppLifecycleState.resumed,
      ),
    );
    await pumpThread(tester, host, FakeThreadReads(host.db));
    expect(await entriesOf(tester, host, PersonalOps.putReadState), isEmpty);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await host.settle(tester);
    expect(
      await entriesOf(tester, host, PersonalOps.putReadState),
      hasLength(1),
    );
    await host.dispose(tester);
  });

  testWidgets('Esc marks read again after Mark as unread', (tester) async {
    final host = await ColumnHost.open(tester);
    await host.run(tester, () async {
      await seedThread(host.db, messages: [msg(1), msg(2, minutes: 5)]);
      await host.db.personalDao.applyReadState(readState(1, lastRead: 2));
    });
    await pumpThread(tester, host, FakeThreadReads(host.db));
    await tester.tap(messageKey(messageId(1)), buttons: kSecondaryButton);
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    await tester.ensureVisible(find.byKey(const ValueKey('action.markUnread')));
    await tester.tap(find.byKey(const ValueKey('action.markUnread')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    // The server's answer moved the position back.
    await host.run(
      tester,
      () => host.db.personalDao.applyReadState(
        readState(1, lastRead: 0).copyWith(version: const ResourceVersion(2)),
        reset: true,
      ),
    );
    await host.settle(tester);
    final before = (await entriesOf(
      tester,
      host,
      PersonalOps.putReadState,
    )).length;
    await tester.tap(composerField);
    await host.settle(tester);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await host.settle(tester);
    expect(
      await entriesOf(tester, host, PersonalOps.putReadState),
      hasLength(before + 1),
    );
    await host.dispose(tester);
  });

  testWidgets('a second edit before the first is confirmed joins it', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await host.run(
      tester,
      () => seedThread(
        host.db,
        messages: [msg(1, author: alex, text: 'Old')],
      ),
    );
    await pumpThread(tester, host, FakeThreadReads(host.db));
    for (final text in ['First edit', 'Second edit']) {
      await tester.tap(messageKey(messageId(1)), buttons: kSecondaryButton);
      await tester.pump(const Duration(milliseconds: 500));
      await host.settle(tester);
      await tester.ensureVisible(find.byKey(const ValueKey('action.edit')));
      await tester.pump(const Duration(milliseconds: 300));
      await tester.tap(find.byKey(const ValueKey('action.edit')));
      await tester.pump(const Duration(milliseconds: 500));
      await host.settle(tester);
      await tester.enterText(find.byKey(const ValueKey('edit.field')), text);
      await tester.tap(find.byKey(const ValueKey('edit.save')));
      await tester.pump(const Duration(milliseconds: 500));
      await host.settle(tester);
    }
    final edits = await entriesOf(tester, host, MessageOps.updateMessage);
    expect(edits, hasLength(1));
    expect(decodePayload(edits.single.payload)['content_md'], 'Second edit');
    await host.dispose(tester);
  });

  testWidgets('a removal in D2 closes both columns with No access anymore', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    final main = Thread(
      id: threadId(0x81),
      channelId: channelId(1),
      kind: ThreadKind.main,
      title: '',
      titleSource: TitleSource.auto,
      state: ThreadState.open,
      lastMessageSeq: const ThreadSeq(1),
      unreadFloorSeq: const ThreadSeq(0),
      lastActivityAt: t0,
      messageCount: 1,
      createdBy: sam,
      createdAt: t0,
      version: const ResourceVersion(1),
      tagIds: const [],
    );
    await host.run(tester, () async {
      await seedThread(
        host.db,
        ch: channel(1, name: 'general'),
        messages: [msg(1)],
      );
      await host.db.threadsDao.upsertIfNewer(main);
      await host.db.messagesDao.upsertIfNewer(msg(1, threadN: 0x81));
      await host.db.messagesDao.addLoadedRange(
        main.id,
        const ThreadSeq(1),
        const ThreadSeq(1),
      );
    });
    final notices = StreamController<SyncNotice>.broadcast();
    addTearDown(notices.close);
    final location = Uri.parse('/t/${threadId(1).value}');
    await host.pump(
      tester,
      HistoryArea(location: location),
      location: location.toString(),
      overrides: [
        ...threadOverrides(FakeThreadReads(host.db)),
        syncNoticesProvider.overrideWith((ref) => notices.stream),
      ],
    );
    expect(
      find.byKey(ValueKey('history.${threadId(1).value}')),
      findsOneWidget,
    );
    expect(find.byKey(ValueKey('history.${main.id.value}')), findsOneWidget);
    // The engine deletes the rows, then tells the app.
    await host.run(
      tester,
      () => host.db.removeTargets([TargetRef.channel(channelId(1))]),
    );
    await host.settle(tester);
    notices.add(TargetsRemoved([TargetRef.channel(channelId(1))]));
    await host.settle(tester);
    expect(find.byKey(const ValueKey('thread.noAccess')), findsNWidgets(2));
    await host.dispose(tester);
  });
}
