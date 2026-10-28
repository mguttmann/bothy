// SPDX-License-Identifier: Apache-2.0

// The new chat view of M4-41 as a page of the phone's Spaces stack (brief M5-15 fixed
// requirement 12, Design note 2, Tests; 01 6.8 item 8): "+" opens the page with the focus in the
// input; back (the back button and the edge swipe on iOS, system back on Android) leaves it to
// M1 and deletes an empty draft; a draft with text stays; sending creates the post through the
// outbox, shows "Sending" and opens the post's history once the server created it.

import 'dart:io';

import 'package:client/app/router.dart';
import 'package:client/phone/phone_navigation.dart';
import 'package:client/sync/thread_ops.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/new_chat_fixtures.dart';
import '../support/structure.dart';
import '../support/widgets.dart';
import 'support/phone_app.dart';
import 'support/phone_threads.dart';

Future<ColumnHost> open(
  WidgetTester tester,
  String platform, {
  CreatedPosts? created,
}) async {
  final host = await ColumnHost.open(tester);
  await tester.runAsync(() => fillThreads(host.db));
  await pumpPhone(tester, host, platform: platform, created: created);
  return host;
}

List<PhonePageKind> kinds(WidgetTester tester) => [
  for (final p in phoneShell(tester).pages) p.kind,
];

Future<void> typeText(WidgetTester tester, ColumnHost host, String text) async {
  await tester.enterText(composerField, text);
  await settlePhone(tester, host);
}

/// Back as a person goes back: the back button of the page (iOS) or system back (Android).
Future<void> back(WidgetTester tester, ColumnHost host, String platform) async {
  if (platform == 'android') {
    await tester.binding.handlePopRoute();
  } else {
    await tester.tap(find.byKey(const ValueKey('phone.back')));
  }
  await settlePhone(tester, host);
}

/// The send button of the view's composer.
final Finder sendButton = find.descendant(
  of: find.byKey(const ValueKey('newChat.composer')),
  matching: find.byWidgetPredicate(
    (w) => w is DsTappable && w.label == en.composerSend,
  ),
);

void main() {
  group('the stack', () {
    final forum = channelId(1);
    Uri at(String path) => Uri.parse(path);

    test('the new chat view lies on M1 in Spaces', () {
      final path = NewChatLocation.pathOf(forum);
      expect(
        tabOf(parseLocation(at(path)), PhoneTab.mentions),
        PhoneTab.spaces,
      );
      expect(phoneStack(PhoneTab.spaces, at(path)), [
        const PhonePage(PhonePageKind.spaces, '/'),
        PhonePage(PhonePageKind.newChat, path),
      ]);
    });

    test('the view of a sent post too', () {
      final path = NewChatLocation.pathOf(forum, entry: uuid(0x801));
      expect(phoneStack(PhoneTab.spaces, at(path)), [
        const PhonePage(PhonePageKind.spaces, '/'),
        PhonePage(PhonePageKind.newChat, path),
      ]);
    });

    testWidgets('the phone app routes the path of the view to the shell', (
      tester,
    ) async {
      final root = Directory.systemTemp.createTempSync('new_chat_page');
      addTearDown(() => root.deleteSync(recursive: true));
      final app = await PhoneApp.start(tester, root: root);
      await app.go(NewChatLocation.pathOf(forum));
      expect(app.location, NewChatLocation.pathOf(forum));
      expect(app.shell.pages.last.kind, PhonePageKind.newChat);
    });
  });

  for (final platform in const ['ios', 'android']) {
    group('on $platform', () {
      testPhone('"+" opens the page with the focus in the input', platform, (
        tester,
      ) async {
        final host = await open(tester, platform);
        await tester.tap(addAction(server, en));
        await settlePhone(tester, host);
        expect(host.visited.last, newChatPath(server));
        expect(kinds(tester), [PhonePageKind.spaces, PhonePageKind.newChat]);
        expect(find.byKey(const ValueKey('newChat.view')), findsOneWidget);
        expect(composerFocused(tester), isTrue);
        // No tab bar on a deeper page; the back control names M1.
        expect(find.byKey(const ValueKey('phone.tabBar')), findsNothing);
        expect(find.byKey(const ValueKey('phone.back')), findsOneWidget);
        await host.dispose(tester);
      });

      testPhone('back leaves the page and deletes an empty draft', platform, (
        tester,
      ) async {
        final host = await open(tester, platform);
        await tester.tap(addAction(server, en));
        await settlePhone(tester, host);
        await typeText(tester, host, 'Half a thought');
        await typeText(tester, host, '');
        await back(tester, host, platform);
        expect(host.visited.last, '/');
        expect(kinds(tester), [PhonePageKind.spaces]);
        final writes = draftWrites(
          await host.run(tester, host.db.outboxDao.allEntries),
        );
        expect(writes, isNotEmpty);
        expect(payloadOf(writes.last)['content_md'], '');
        expect(draftRow(server), findsNothing);
        await host.dispose(tester);
      });

      testPhone(
        'a draft with text stays after back as the draft row',
        platform,
        (tester) async {
          final host = await open(tester, platform);
          await tester.tap(addAction(server, en));
          await settlePhone(tester, host);
          await typeText(tester, host, 'Keep me');
          await back(tester, host, platform);
          expect(kinds(tester), [PhonePageKind.spaces]);
          expect(draftRow(server), findsOneWidget);
          final writes = draftWrites(
            await host.run(tester, host.db.outboxDao.allEntries),
          );
          expect(payloadOf(writes.last)['content_md'], 'Keep me');
          await host.dispose(tester);
        },
      );

      testPhone(
        'sending creates the post through the outbox and opens its history',
        platform,
        (tester) async {
          final created = CreatedPosts();
          final host = await open(tester, platform, created: created);
          await tester.tap(addAction(server, en));
          await settlePhone(tester, host);
          await typeText(tester, host, 'Restart the gateway');
          await tester.tap(sendButton);
          await settlePhone(tester, host);
          final sent = creates(
            await host.run(tester, host.db.outboxDao.allEntries),
          );
          expect(sent, hasLength(1));
          final body = payloadOf(sent.single);
          expect(body['channel_id'], server.id.value);
          expect(body['content_md'], 'Restart the gateway');
          expect(body['clear_draft'], isTrue);
          expect(host.visited.last, newChatPath(server, entry: sent.single.id));
          expect(kinds(tester), [PhonePageKind.spaces, PhonePageKind.newChat]);
          expect(find.byKey(const ValueKey('newChat.sending')), findsOneWidget);

          created.record(sent.single.id, threadId(2));
          await settlePhone(tester, host);
          expect(host.visited.last, '/t/${threadId(2).value}');
          expect(kinds(tester), [
            PhonePageKind.spaces,
            PhonePageKind.channel,
            PhonePageKind.thread,
          ]);
          await host.dispose(tester);
        },
      );
    });
  }

  testPhone('the edge swipe leaves the page', 'ios', (tester) async {
    final host = await open(tester, 'ios');
    await tester.tap(addAction(server, en));
    await settlePhone(tester, host);
    expect(kinds(tester), [PhonePageKind.spaces, PhonePageKind.newChat]);
    final swipe = await tester.startGesture(const Offset(2, 400));
    for (var i = 0; i < 10; i++) {
      await swipe.moveBy(const Offset(30, 0));
      await tester.pump(const Duration(milliseconds: 16));
    }
    await swipe.up();
    await settlePhone(tester, host);
    expect(host.visited.last, '/');
    expect(kinds(tester), [PhonePageKind.spaces]);
    await host.dispose(tester);
  });
}
