// SPDX-License-Identifier: Apache-2.0

// The message actions of the thread view (brief M4-20 Tests; 01 3.3, 6.7; 04 13.3): each
// action writes the right outbox entry; an edit with `412` shows both versions; `410` keeps the
// text; "View versions" lists the revisions; delete asks once; the move dialog lists only
// channels with the same operator; actions without the capability are hidden.

import 'dart:convert';

import 'package:client/sync/message_ops_app.dart';
import 'package:client/sync/thread_ops.dart';
import 'package:client/thread/thread_reads.dart';
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

Future<void> openMenu(
  WidgetTester tester,
  ColumnHost host,
  MessageId id,
) async {
  await tester.tap(messageKey(id), buttons: kSecondaryButton);
  await tester.pump(const Duration(milliseconds: 500));
  await host.settle(tester);
}

Future<void> choose(WidgetTester tester, ColumnHost host, String action) async {
  await tester.ensureVisible(find.byKey(ValueKey('action.$action')));
  await tester.pump(const Duration(milliseconds: 300));
  await tester.tap(find.byKey(ValueKey('action.$action')));
  // The menu's closing and the dialog's opening animations.
  await tester.pump(const Duration(milliseconds: 500));
  await host.settle(tester);
}

/// The writes of the actions: without the read position the view writes on its own and
/// without the draft the composer saves (both have their own tests).
Future<List<OutboxEntry>> entries(
  WidgetTester tester,
  ColumnHost host,
) async => [
  for (final e in await host.run(tester, () => host.db.outboxDao.allEntries()))
    if (e.kind != PersonalOps.putReadState &&
        e.kind != PersonalOps.putThreadDraft)
      e,
];

Map<String, Object?> body(OutboxEntry e) => decodePayload(e.payload);

Future<ColumnHost> open(
  WidgetTester tester, {
  List<Capability> capabilities = const [],
  List<Message>? messages,
  FakeThreadReads Function(ColumnHost)? reads,
  Channel? ch,
}) async {
  final host = await ColumnHost.open(tester);
  await host.run(
    tester,
    () => seedThread(
      host.db,
      ch: ch,
      capabilities: capabilities,
      messages:
          messages ??
          [
            msg(1, author: sam, text: 'Printer is jammed'),
            msg(2, author: alex, text: 'I will look', minutes: 10),
          ],
    ),
  );
  await pumpThread(tester, host, reads?.call(host) ?? FakeThreadReads(host.db));
  return host;
}

void main() {
  testWidgets('reply, react, mark unread, copy and edit write the outbox', (
    tester,
  ) async {
    final host = await open(tester);
    final clipboard = <String>[];
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        if (call.method == 'Clipboard.setData') {
          clipboard.add((call.arguments as Map)['text'] as String);
        }
        return null;
      },
    );

    // Reply: the quote goes into the field and the reply reference into the message.
    await openMenu(tester, host, messageId(1));
    await choose(tester, host, 'reply');
    expect(find.byKey(const ValueKey('composer.reply')), findsOneWidget);
    await tester.enterText(
      composerField,
      '${(tester.widget<TextField>(composerField).controller!.text)}On it',
    );
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await host.settle(tester);
    var all = await entries(tester, host);
    final sent = all.firstWhere((e) => e.kind == MessageOps.createMessage);
    expect(body(sent)['reply_to_message_id'], messageId(1).value);
    expect(body(sent)['content_md'], startsWith('> Printer is jammed'));
    expect(body(sent)['clear_draft'], isTrue);

    // React toggles: add, then remove the own reaction.
    await openMenu(tester, host, messageId(1));
    await choose(tester, host, 'react');
    await tester.enterText(find.byKey(const ValueKey('reaction.input')), '+1');
    await tester.tap(find.byKey(const ValueKey('reaction.add')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    all = await entries(tester, host);
    final added = all.firstWhere((e) => e.kind == MessageOps.addReaction);
    expect(body(added), {'message_id': messageId(1).value, 'emoji': '+1'});
    await host.run(
      tester,
      () => host.db.messagesDao.applyReactionSummary(
        ReactionSummary(
          messageId: messageId(1),
          emoji: '+1',
          count: 1,
          me: true,
        ),
      ),
    );
    await host.settle(tester);
    await openMenu(tester, host, messageId(1));
    await choose(tester, host, 'react');
    await tester.tap(find.byKey(const ValueKey('reaction.chip.+1')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    all = await entries(tester, host);
    expect(all.where((e) => e.kind == MessageOps.removeReaction), hasLength(1));

    // Mark as unread: back to before the message, with the read state version.
    await host.run(
      tester,
      () => host.db.personalDao.applyReadState(readState(1, lastRead: 2)),
    );
    await openMenu(tester, host, messageId(2));
    await choose(tester, host, 'markUnread');
    all = await entries(tester, host);
    final unread = all.firstWhere(
      (e) => e.kind == AppMessageOps.markThreadUnread,
    );
    expect(body(unread)['last_read_seq'], 1);
    expect(unread.expectedVersion, const ResourceVersion(1));

    // Copy text and link.
    await openMenu(tester, host, messageId(1));
    await choose(tester, host, 'copyText');
    await openMenu(tester, host, messageId(1));
    await choose(tester, host, 'copyLink');
    expect(clipboard, [
      'Printer is jammed',
      'chat-example://t/${threadId(1).value}?m=${messageId(1).value}',
    ]);

    // Edit the own message inline.
    await openMenu(tester, host, messageId(2));
    await choose(tester, host, 'edit');
    await tester.enterText(find.byKey(const ValueKey('edit.field')), 'Fixed');
    await tester.tap(find.byKey(const ValueKey('edit.save')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    all = await entries(tester, host);
    final edit = all.firstWhere((e) => e.kind == MessageOps.updateMessage);
    expect(body(edit), {
      'message_id': messageId(2).value,
      'version': 1,
      'content_md': 'Fixed',
    });
    await host.dispose(tester);
  });

  testWidgets('an edit with 412 shows both versions; keep mine sends again', (
    tester,
  ) async {
    final host = await open(tester);
    final id = await host.run(tester, () async {
      final e = await host.engine.outbox.enqueue(
        OutboxRequest(
          kind: MessageOps.updateMessage,
          threadId: threadId(1),
          payload: {
            'message_id': messageId(2).value,
            'version': 1,
            'content_md': 'My text',
          },
          expectedVersion: const ResourceVersion(1),
        ),
      );
      await host.db.outboxDao.recordFailure(
        e.id,
        code: 'version_conflict',
        conflictCurrent: jsonEncode({
          'id': messageId(2).value,
          'content_md': 'Their text',
          'version': 2,
        }),
      );
      return e.id;
    });
    await host.settle(tester);
    await tester.tap(find.byKey(const ValueKey('outbox.compare')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    expect(find.byKey(const ValueKey('conflict.dialog')), findsOneWidget);
    expect(find.text('My text'), findsWidgets);
    expect(find.text('Their text'), findsWidgets);
    await tester.tap(find.byKey(const ValueKey('conflict.keepMine')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    final all = await entries(tester, host);
    expect(all.any((e) => e.id == id), isFalse, reason: 'replaced');
    final again = all.singleWhere((e) => e.kind == MessageOps.updateMessage);
    expect(again.expectedVersion, const ResourceVersion(2));
    expect(body(again)['content_md'], 'My text');
    await host.dispose(tester);
  });

  testWidgets('an edit of a deleted message keeps the text for copying', (
    tester,
  ) async {
    final host = await open(tester);
    await host.run(tester, () async {
      final e = await host.engine.outbox.enqueue(
        OutboxRequest(
          kind: MessageOps.updateMessage,
          threadId: threadId(1),
          payload: {
            'message_id': messageId(2).value,
            'version': 1,
            'content_md': 'Lost words',
          },
        ),
      );
      await host.db.outboxDao.recordFailure(e.id, code: 'message_deleted');
    });
    await host.settle(tester);
    expect(find.text('Lost words'), findsOneWidget);
    expect(find.byKey(const ValueKey('outbox.copy')), findsOneWidget);
    expect(find.byKey(const ValueKey('outbox.discard')), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('view versions lists the revisions of an edited message', (
    tester,
  ) async {
    late FakeThreadReads reads;
    final host = await open(
      tester,
      messages: [msg(1, author: sam, text: 'Now', edited: true, version: 2)],
      reads: (h) => reads = FakeThreadReads(h.db)
        ..revisionsOf[messageId(1)] = [
          Revision(
            number: 1,
            contentMd: 'Before',
            editedBy: sam,
            createdAt: t0,
          ),
        ],
    );
    await openMenu(tester, host, messageId(1));
    await choose(tester, host, 'versions');
    expect(find.byKey(const ValueKey('versions.dialog')), findsOneWidget);
    expect(find.text('Before'), findsOneWidget);
    expect(reads.calls, isEmpty);
    await host.dispose(tester);
  });

  testWidgets('delete asks once, then writes deleteMessage', (tester) async {
    final host = await open(tester);
    await openMenu(tester, host, messageId(2));
    await choose(tester, host, 'delete');
    expect(find.byKey(const ValueKey('delete.dialog')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('delete.cancel')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    expect(await entries(tester, host), isEmpty);
    await openMenu(tester, host, messageId(2));
    await choose(tester, host, 'delete');
    await tester.tap(find.byKey(const ValueKey('delete.confirm')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    final all = await entries(tester, host);
    expect(all.single.kind, MessageOps.deleteMessage);
    expect(body(all.single), {'message_id': messageId(2).value});
    expect(find.byKey(const ValueKey('delete.dialog')), findsNothing);
    await host.dispose(tester);
  });

  testWidgets('without capabilities pin, move and foreign delete are hidden', (
    tester,
  ) async {
    final host = await open(tester);
    await openMenu(tester, host, messageId(1));
    expect(find.byKey(const ValueKey('action.pin')), findsNothing);
    expect(find.byKey(const ValueKey('action.delete')), findsNothing);
    expect(find.byKey(const ValueKey('action.edit')), findsNothing);
    expect(find.byKey(const ValueKey('action.reply')), findsOneWidget);
    await tester.tapAt(Offset.zero);
    await host.settle(tester);
    expect(find.byKey(const ValueKey('thread.move')), findsNothing);
    await host.dispose(tester);
  });

  testWidgets('pin and move with manage; the move dialog lists only channels '
      'with the same operator', (tester) async {
    final host = await open(
      tester,
      capabilities: const [Capability.manage],
      ch: channel(1, kind: ChannelKind.forum, name: 'ideas', operator: robin),
    );
    await host.run(tester, () async {
      await host.db.structureDao.upsertChannelIfNewer(
        channel(2, name: 'same-operator', operator: robin),
      );
      await host.db.structureDao.upsertChannelIfNewer(
        channel(3, name: 'other-operator', operator: sam),
      );
      await host.db.structureDao.upsertChannelIfNewer(
        channel(4, name: 'no-operator'),
      );
    });
    await host.settle(tester);
    await openMenu(tester, host, messageId(1));
    await choose(tester, host, 'pin');
    var all = await entries(tester, host);
    expect(all.single.kind, AppMessageOps.pinMessage);

    await tester.tap(find.byKey(const ValueKey('thread.move')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    expect(find.byKey(const ValueKey('move.dialog')), findsOneWidget);
    expect(
      find.byKey(ValueKey('move.channel.${channelId(2).value}')),
      findsOneWidget,
    );
    expect(
      find.byKey(ValueKey('move.channel.${channelId(3).value}')),
      findsNothing,
    );
    expect(
      find.byKey(ValueKey('move.channel.${channelId(4).value}')),
      findsNothing,
    );
    await tester.tap(
      find.byKey(ValueKey('move.channel.${channelId(2).value}')),
    );
    await tester.enterText(
      find.byKey(const ValueKey('move.reason')),
      'Belongs there',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('move.confirm')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    all = await entries(tester, host);
    final move = all.firstWhere((e) => e.kind == AppMessageOps.moveThread);
    expect(body(move), {
      'target_channel_id': channelId(2).value,
      'reason': 'Belongs there',
    });
    expect(move.threadId, threadId(1));
    await host.dispose(tester);
  });

  testWidgets('start a thread from a message of the main history', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    final text = channel(1, name: 'general');
    final main = threadId(0x81);
    await host.run(
      tester,
      () => seedThread(
        host.db,
        ch: text,
        th: Thread(
          id: main,
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
        ),
        messages: [msg(1, threadN: 0x81, text: 'Export format?')],
      ),
    );
    await pumpThread(tester, host, FakeThreadReads(host.db), n: 0x81);
    await openMenu(tester, host, messageId(1, threadN: 0x81));
    await choose(tester, host, 'startThread');
    final all = await entries(tester, host);
    expect(all.single.kind, ThreadOps.createThread);
    expect(body(all.single), {
      'channel_id': channelId(1).value,
      'parent_message_id': messageId(1, threadN: 0x81).value,
    });
    await host.dispose(tester);
  });
}
