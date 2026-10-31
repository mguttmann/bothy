// SPDX-License-Identifier: Apache-2.0

// Long pastes and long drafts in the composer (brief M4-47; 01 6.13): a paste over
// `ui.composer_paste_file_chars` or `ui.composer_paste_file_lines` becomes the attachment
// `paste.txt` (or a code file for one fenced block) shown as a chip with "Insert as text"; a
// draft over `message_max_content_bytes` shows a note and is sent as one message whose rest
// is one attachment `message.txt`, in a thread and in the new chat view of a forum.

import 'dart:convert';
import 'dart:io';

import 'package:client/files/upload_executor.dart';
import 'package:client/sync/thread_ops.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/column_host.dart';
import '../support/file_fakes.dart';
import '../support/file_widgets.dart';
import '../support/new_chat_fixtures.dart' as forum;
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

void main() {
  Future<List<OutboxEntry>> entries(WidgetTester tester, ColumnHost host) =>
      host.run(tester, () async {
        final all = await host.db.outboxDao.allEntries();
        return [
          for (final e in all)
            if (e.kind == UploadOps.upload ||
                e.kind == MessageOps.createMessage ||
                e.kind == ThreadOps.createThread)
              e,
        ];
      });

  List<OutboxEntry> uploads(List<OutboxEntry> all) => [
    for (final e in all)
      if (e.kind == UploadOps.upload) e,
  ];

  Future<ColumnHost> open(
    WidgetTester tester, {
    int maxBytes = 262144,
    int maxAttachments = 10,
  }) async {
    final host = await ColumnHost.open(tester);
    final services = FileTestServices(host.db, host.root, fakeFilePorts());
    await host.run(tester, () async {
      await seedThread(host.db, maxBytes: maxBytes, messages: [msg(1)]);
      final base = instanceWith(maxBytes: maxBytes);
      await host.db.instanceDao.put(
        base.copyWith(
          limits: base.limits.copyWith(messageMaxAttachments: maxAttachments),
        ),
      );
    });
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      overrides: [
        ...threadOverrides(FakeThreadReads(host.db)),
        ...services.overrides,
      ],
    );
    return host;
  }

  String fieldText(WidgetTester tester) =>
      tester.widget<TextField>(composerField).controller!.text;

  Future<void> paste(WidgetTester tester, ColumnHost host, String text) async {
    await tester.enterText(composerField, text);
    await settleUntil(
      tester,
      host,
      find.byKey(const ValueKey('composer.attachments')),
    );
  }

  Future<void> send(WidgetTester tester, ColumnHost host) async {
    await tester.showKeyboard(composerField);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await host.settle(tester);
  }

  // The entries once the send wrote its message or post: a send of a long draft first writes
  // `message.txt` with real IO, which a loaded machine may not finish within a fixed number
  // of settle rounds (issue #238).
  Future<List<OutboxEntry>> sent(WidgetTester tester, ColumnHost host) async {
    await host.outboxUntil(
      tester,
      (all) => all.any(
        (e) =>
            e.kind == MessageOps.createMessage ||
            e.kind == ThreadOps.createThread,
      ),
      what: 'the send',
    );
    // The frame after the write shows the emptied composer.
    await host.settle(tester);
    return entries(tester, host);
  }

  Map<String, Object?> payload(OutboxEntry e) =>
      decodePayload(e.payload).cast<String, Object?>();

  Future<String> uploadedText(
    WidgetTester tester,
    ColumnHost host,
    OutboxEntry e,
  ) => host.run(
    tester,
    () => File(payload(e)['path']! as String).readAsString(),
  );

  final long = [for (var i = 0; i < 800; i++) 'word$i'].join(' ');

  group('a long paste', () {
    testWidgets('becomes paste.txt and the field keeps the text around it', (
      tester,
    ) async {
      final host = await open(tester);
      await tester.enterText(composerField, 'See ');
      await host.settle(tester);
      await paste(tester, host, 'See $long');
      expect(long.length, greaterThan(4000));
      expect(fieldText(tester), 'See ');
      final up = uploads(await entries(tester, host)).single;
      expect(payload(up)['name'], 'paste.txt');
      expect(payload(up)['declared_mime_type'], 'text/plain');
      expect(payload(up)['size_bytes'], utf8.encode(long).length);
      expect(await uploadedText(tester, host, up), long);
      expect(
        find.descendant(
          of: find.byKey(const ValueKey('composer.attachments')),
          matching: find.textContaining('paste.txt'),
        ),
        findsOneWidget,
      );
      expect(find.text(en.composerPasteAsText), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('of more than 100 lines becomes a file, of 100 lines not', (
      tester,
    ) async {
      final host = await open(tester);
      final hundred = [for (var i = 0; i < 100; i++) 'l$i'].join('\n');
      await tester.enterText(composerField, hundred);
      await host.settle(tester);
      expect(fieldText(tester), hundred);
      expect(uploads(await entries(tester, host)), isEmpty);
      await tester.enterText(composerField, '');
      await host.settle(tester);
      await paste(tester, host, '$hundred\nl100');
      expect(fieldText(tester), '');
      expect(uploads(await entries(tester, host)), hasLength(1));
      await host.dispose(tester);
    });

    testWidgets('of one fenced python block becomes paste.py with the code', (
      tester,
    ) async {
      final host = await open(tester);
      final code = [for (var i = 0; i < 150; i++) 'x$i = $i'].join('\n');
      await paste(tester, host, '```python\n$code\n```');
      final up = uploads(await entries(tester, host)).single;
      expect(payload(up)['name'], 'paste.py');
      expect(await uploadedText(tester, host, up), '$code\n');
      await host.dispose(tester);
    });

    testWidgets('"Insert as text" removes the file and puts the paste back', (
      tester,
    ) async {
      final host = await open(tester);
      await tester.enterText(composerField, 'See ');
      await host.settle(tester);
      await paste(tester, host, 'See $long');
      await tester.tap(find.text(en.composerPasteAsText));
      await host.settle(tester);
      expect(fieldText(tester), 'See $long');
      expect(find.byKey(const ValueKey('composer.attachments')), findsNothing);
      expect(uploads(await entries(tester, host)), isEmpty);
      await host.dispose(tester);
    });

    testWidgets('is sent as the attachment of the message', (tester) async {
      final host = await open(tester);
      await tester.enterText(composerField, 'See ');
      await host.settle(tester);
      await paste(tester, host, 'See $long');
      await send(tester, host);
      final all = await sent(tester, host);
      final message = all.singleWhere(
        (e) => e.kind == MessageOps.createMessage,
      );
      expect(payload(message)['content_md'], 'See');
      expect(payload(message)['attachment_keys'], hasLength(1));
      expect(message.dependsOn, uploads(all).single.id);
      await host.dispose(tester);
    });
  });

  group('a draft over the limit', () {
    final text = [for (var i = 0; i < 40; i++) 'line number $i'].join('\n');

    testWidgets('shows the note and goes out as one message with message.txt', (
      tester,
    ) async {
      final host = await open(tester, maxBytes: 100);
      await tester.enterText(composerField, text);
      await host.settle(tester);
      expect(find.text(en.composerOverflowNote('message.txt')), findsOneWidget);
      await send(tester, host);
      final all = await sent(tester, host);
      expect(find.text(en.composerTooLong(100)), findsNothing);
      final messages = [
        for (final e in all)
          if (e.kind == MessageOps.createMessage) e,
      ];
      expect(messages, hasLength(1), reason: 'never split into several');
      final head = payload(messages.single)['content_md']! as String;
      expect(utf8.encode(head).length, lessThanOrEqualTo(100));
      final up = uploads(all).single;
      expect(payload(up)['name'], 'message.txt');
      final rest = await uploadedText(tester, host, up);
      expect('$head$rest', text);
      expect(payload(messages.single)['attachment_keys'], hasLength(1));
      expect(fieldText(tester), '');
      expect(find.text(en.composerOverflowNote('message.txt')), findsNothing);
      await host.dispose(tester);
    });

    testWidgets('sent twice while its rest is written goes out once', (
      tester,
    ) async {
      final host = await open(tester, maxBytes: 100);
      await tester.enterText(composerField, text);
      await host.settle(tester);
      await tester.showKeyboard(composerField);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      final all = await sent(tester, host);
      expect(
        all.where((e) => e.kind == MessageOps.createMessage),
        hasLength(1),
      );
      expect(uploads(all), hasLength(1));
      await host.dispose(tester);
    });

    testWidgets('is refused with the reason only when no file may be added', (
      tester,
    ) async {
      final host = await open(tester, maxBytes: 100, maxAttachments: 1);
      await paste(tester, host, long);
      await tester.enterText(composerField, text);
      await host.settle(tester);
      await send(tester, host);
      expect(find.text(en.composerTooLong(100)), findsOneWidget);
      expect(fieldText(tester), text);
      final all = await entries(tester, host);
      expect(all.where((e) => e.kind == MessageOps.createMessage), isEmpty);
      await host.dispose(tester);
    });

    testWidgets('within the limit shows no note', (tester) async {
      final host = await open(tester, maxBytes: 100000);
      await tester.enterText(composerField, text);
      await host.settle(tester);
      expect(find.text(en.composerOverflowNote('message.txt')), findsNothing);
      await host.dispose(tester);
    });
  });

  group('the new chat view of a forum', () {
    Future<ColumnHost> openNewChat(
      WidgetTester tester, {
      int maxBytes = 262144,
      int maxUpload = 1000000,
    }) async {
      final host = await ColumnHost.open(tester);
      final services = FileTestServices(host.db, host.root, fakeFilePorts());
      await tester.runAsync(() async {
        await forum.fillForums(host.db);
        final base = instanceWith(maxBytes: maxBytes);
        await host.db.instanceDao.put(
          base.copyWith(
            limits: base.limits.copyWith(fileMaxUploadBytes: maxUpload),
          ),
        );
      });
      await host.pump(
        tester,
        forum.shell,
        location: forum.newChatPath(forum.server),
        overrides: [...forum.shellOverrides(), ...services.overrides],
      );
      return host;
    }

    testWidgets('a long paste becomes paste.txt and goes out with the post', (
      tester,
    ) async {
      final host = await openNewChat(tester);
      await tester.enterText(forum.composerField, long);
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('newChat.attachments')),
      );
      expect(forum.composerText(tester), '');
      expect(find.text(en.composerPasteAsText), findsOneWidget);
      await tester.enterText(forum.composerField, 'Look at this');
      await host.settle(tester);
      await tester.showKeyboard(forum.composerField);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      final all = await sent(tester, host);
      final post = forum.creates(all).single;
      expect(payload(post)['content_md'], 'Look at this');
      expect(payload(post)['attachment_keys'], hasLength(1));
      final up = uploads(all).single;
      expect(payload(up)['name'], 'paste.txt');
      expect(post.dependsOn, up.id);
      await host.dispose(tester);
    });

    testWidgets('a post over the limit goes out once with message.txt', (
      tester,
    ) async {
      final host = await openNewChat(tester, maxBytes: 50);
      final text = [for (var i = 0; i < 20; i++) 'step $i'].join('\n');
      await tester.enterText(forum.composerField, text);
      await host.settle(tester);
      expect(find.text(en.composerOverflowNote('message.txt')), findsOneWidget);
      await tester.showKeyboard(forum.composerField);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      final all = await sent(tester, host);
      final post = forum.creates(all).single;
      final head = payload(post)['content_md']! as String;
      expect(utf8.encode(head).length, lessThanOrEqualTo(50));
      final up = uploads(all).single;
      expect(payload(up)['name'], 'message.txt');
      expect('$head${await uploadedText(tester, host, up)}', text);
      await host.dispose(tester);
    });

    testWidgets('a second long paste after "Insert as text" becomes a file', (
      tester,
    ) async {
      final host = await openNewChat(tester);
      final chips = find.byKey(const ValueKey('newChat.attachments'));
      await tester.enterText(forum.composerField, long);
      await settleUntil(tester, host, chips);
      await tester.tap(find.text(en.composerPasteAsText));
      await host.settle(tester);
      expect(chips, findsNothing);
      await tester.enterText(forum.composerField, '');
      await host.settle(tester);
      await tester.enterText(forum.composerField, long);
      await settleUntil(tester, host, chips);
      expect(tester.takeException(), isNull);
      expect(find.text(en.composerPasteAsText), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('a paste refused as a file stays text and shows the reason', (
      tester,
    ) async {
      final host = await openNewChat(tester, maxUpload: 100);
      await tester.enterText(forum.composerField, long);
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('newChat.attachError')),
      );
      expect(
        find.text(en.composerFileTooLarge(dsIsolate('paste.txt'), '100 B')),
        findsOneWidget,
      );
      expect(forum.composerText(tester), long);
      expect(uploads(await entries(tester, host)), isEmpty);
      await host.dispose(tester);
    });

    testWidgets('a failed upload of the post shows "Upload failed"', (
      tester,
    ) async {
      final host = await openNewChat(tester);
      await tester.enterText(forum.composerField, long);
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('newChat.attachments')),
      );
      final up = uploads(await entries(tester, host)).single;
      await host.run(
        tester,
        () => host.db.outboxDao.recordFailure(up.id, code: 'conflict'),
      );
      await settleUntil(tester, host, find.text(en.fileStateUploadFailed));
      expect(
        find.descendant(
          of: find.byKey(const ValueKey('newChat.attachments')),
          matching: find.text(en.fileStateUploadFailed),
        ),
        findsOneWidget,
      );
      await host.dispose(tester);
    });
  });
}
