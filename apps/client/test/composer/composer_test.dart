// SPDX-License-Identifier: Apache-2.0

// The composer of the thread view (brief M4-20 Tests; components.md 9; 04 12, 14): Enter and
// Shift+Enter by `composer.enter_sends`, arrow up edits the last own message, mention
// suggestions only from stored principals, content over the limit is refused with the reason,
// the draft is saved through the outbox with one coalesced write, a foreign draft is not
// applied while typing, the `overwrote_newer` notice and the typing line.

import 'package:client/app/providers/settings.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

Future<List<OutboxEntry>> entries(WidgetTester tester, ColumnHost host) =>
    host.run(tester, () => host.db.outboxDao.allEntries());

Future<ColumnHost> open(
  WidgetTester tester, {
  int maxBytes = 1000,
  Map<String, Object?> settings = const {},
}) async {
  final host = await ColumnHost.open(tester);
  await host.run(
    tester,
    () => seedThread(
      host.db,
      maxBytes: maxBytes,
      messages: [
        msg(1, author: alex, text: 'My first'),
        msg(2, author: alex, text: 'My last', minutes: 30),
        msg(3, author: sam, text: 'Thanks', minutes: 40),
      ],
    ),
  );
  await pumpThreadWith(tester, host, settings);
  return host;
}

Future<void> pumpThreadWith(
  WidgetTester tester,
  ColumnHost host,
  Map<String, Object?> settings,
) async {
  final reads = FakeThreadReads(host.db);
  await host.pump(
    tester,
    HistoryColumn(threadId: threadId(1)),
    location: '/t/${threadId(1).value}',
    overrides: [
      ...threadOverrides(reads),
      userSettingsProvider.overrideWith((ref) => Stream.value(settings)),
    ],
  );
}

Future<void> type(WidgetTester tester, ColumnHost host, String text) async {
  await tester.enterText(composerField, text);
  await host.settle(tester);
}

void main() {
  testWidgets('Enter sends by default, Shift+Enter makes a new line', (
    tester,
  ) async {
    final host = await open(tester);
    await type(tester, host, 'Hello Sam');
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shift);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shift);
    await host.settle(tester);
    expect(
      (await entries(
        tester,
        host,
      )).where((e) => e.kind == MessageOps.createMessage),
      isEmpty,
    );
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await host.settle(tester);
    final sent = (await entries(
      tester,
      host,
    )).singleWhere((e) => e.kind == MessageOps.createMessage);
    expect(decodePayload(sent.payload)['content_md'], startsWith('Hello Sam'));
    expect(tester.widget<TextField>(composerField).controller!.text, isEmpty);
    await host.dispose(tester);
  });

  testWidgets('with enter_sends off, Shift+Enter sends', (tester) async {
    final host = await open(
      tester,
      settings: const {'composer.enter_sends': false},
    );
    await type(tester, host, 'Hello Sam');
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await host.settle(tester);
    expect(
      (await entries(
        tester,
        host,
      )).where((e) => e.kind == MessageOps.createMessage),
      isEmpty,
    );
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shift);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shift);
    await host.settle(tester);
    expect(
      (await entries(
        tester,
        host,
      )).where((e) => e.kind == MessageOps.createMessage),
      hasLength(1),
    );
    await host.dispose(tester);
  });

  testWidgets('arrow up in the empty field edits the last own message', (
    tester,
  ) async {
    final host = await open(tester);
    await tester.tap(composerField);
    await host.settle(tester);
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowUp);
    await host.settle(tester);
    final editor = find.byKey(const ValueKey('edit.field'));
    expect(editor, findsOneWidget);
    expect(tester.widget<TextField>(editor).controller!.text, 'My last');
    await host.dispose(tester);
  });

  testWidgets('mention suggestions come only from stored people and agents', (
    tester,
  ) async {
    final host = await open(tester);
    await host.run(
      tester,
      () => host.db.principalsDao.put(
        Principal(
          id: PrincipalId.parse(uuid(0xa9)),
          kind: PrincipalKind.app,
          handle: 'release-bot',
          displayName: 'Release bot',
        ),
      ),
    );
    await type(tester, host, 'Ask @r');
    expect(find.text('@robin'), findsOneWidget);
    expect(
      find.text('@release-bot'),
      findsNothing,
      reason: 'apps are no mention',
    );
    await type(tester, host, 'Ask @');
    expect(find.text('@alex'), findsOneWidget);
    expect(find.text('@sam'), findsOneWidget);
    expect(find.text('@nobody'), findsNothing);
    await tester.tap(find.text('@sam'));
    await host.settle(tester);
    expect(
      tester.widget<TextField>(composerField).controller!.text,
      'Ask @sam ',
    );
    await host.dispose(tester);
  });

  testWidgets('content over the limit is refused with the reason', (
    tester,
  ) async {
    final host = await open(tester, maxBytes: 10);
    await type(tester, host, 'This is longer than ten bytes');
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await host.settle(tester);
    expect(find.text(en.composerTooLong(10)), findsOneWidget);
    expect(
      (await entries(
        tester,
        host,
      )).where((e) => e.kind == MessageOps.createMessage),
      isEmpty,
    );
    await host.dispose(tester);
  });

  testWidgets(
    'the draft is saved with one coalesced write per typed sentence',
    (tester) async {
      final host = await open(tester);
      const sentence = 'Hello Sam, the printer works again.';
      for (var i = 1; i <= sentence.length; i++) {
        await tester.enterText(composerField, sentence.substring(0, i));
        await tester.pump();
      }
      await host.settle(tester);
      final drafts = (await entries(
        tester,
        host,
      )).where((e) => e.kind == PersonalOps.putThreadDraft).toList();
      // Reported in the report of M4-20: draft writes for one typed sentence.
      expect(drafts, hasLength(1));
      expect(decodePayload(drafts.single.payload)['content_md'], sentence);
      expect(drafts.single.threadId, threadId(1));
      await host.dispose(tester);
    },
  );

  testWidgets(
    'a foreign draft is applied only while the person does not type',
    (tester) async {
      final host = await open(tester);
      Draft draft(String text, int version) => Draft(
        id: DraftId.parse(uuid(0xd1)),
        threadId: threadId(1),
        channelId: channelId(1),
        contentMd: text,
        fileIds: const [],
        version: ResourceVersion(version),
      );
      await host.run(
        tester,
        () =>
            host.db.personalDao.upsertDraftIfNewer(draft('From the laptop', 1)),
      );
      await host.settle(tester);
      expect(
        tester.widget<TextField>(composerField).controller!.text,
        'From the laptop',
      );

      await type(tester, host, 'Mine now');
      await host.run(
        tester,
        () =>
            host.db.personalDao.upsertDraftIfNewer(draft('From the phone', 2)),
      );
      await host.settle(tester);
      expect(
        tester.widget<TextField>(composerField).controller!.text,
        'Mine now',
      );
      await host.dispose(tester);
    },
  );

  testWidgets('overwrote_newer shows a notice', (tester) async {
    final host = await open(tester);
    await host.run(
      tester,
      () => host.engine.catchUp.applyDraftPutResult({
        'draft': {
          'id': uuid(0xd2),
          'thread_id': threadId(1).value,
          'channel_id': channelId(1).value,
          'content_md': 'Mine',
          'file_ids': <Object?>[],
          'version': 4,
        },
        'overwrote_newer': true,
      }),
    );
    await host.settle(tester);
    expect(find.text(en.composerDraftOverwritten), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('the typing line names the people typing', (tester) async {
    final host = await open(tester);
    // The expiry timer of the indicator runs on the real clock, outside the fake time.
    await tester.runAsync(
      () async => host.engine.ephemeral.typingStarted(
        threadId(1),
        sam,
        DateTime.now().add(const Duration(seconds: 1)),
      ),
    );
    await host.settle(tester);
    expect(find.text(en.composerTyping(dsIsolate('Sam'))), findsOneWidget);
    await host.dispose(tester);
  });
}
