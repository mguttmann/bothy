// SPDX-License-Identifier: Apache-2.0

// The thread groups of M1 on the phone over the local database (brief M5-15 Tests and fixed
// requirements 11 to 14; 01 6.8; D-SB-13): the groups with "All posts" and the "+" action at
// `density.target-min`; a tap on a thread row opens M3, on the forum name or "All posts" M2, on
// the draft row the new chat page; a row that stops qualifying stays while a finger is on the
// list or the screen reader is in it (Design note 1) and leaves after; the collapse state of a
// group is kept on the device.

import 'dart:ui' show AccessibilityFeatures;

import 'package:client/phone/phone_navigation.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/new_chat_fixtures.dart';
import '../support/structure.dart';
import '../support/widgets.dart';
import 'support/phone_threads.dart';

/// A screen reader runs (`accessibleNavigation`), as VoiceOver or TalkBack report it.
final class _ScreenReader implements AccessibilityFeatures {
  @override
  bool get accessibleNavigation => true;

  @override
  dynamic noSuchMethod(Invocation invocation) => false;
}

Future<ColumnHost> open(WidgetTester tester, {String platform = 'ios'}) async {
  final host = await ColumnHost.open(tester);
  await tester.runAsync(() => fillThreads(host.db));
  await pumpPhone(tester, host, platform: platform);
  return host;
}

List<PhonePageKind> kinds(WidgetTester tester) => [
  for (final p in phoneShell(tester).pages) p.kind,
];

void main() {
  for (final platform in const ['ios', 'android']) {
    group('on $platform', () {
      testPhone(
        'M1 shows the thread groups, "All posts" and "+" at density.target-min',
        platform,
        (tester) async {
          final host = await open(tester, platform: platform);
          expect(kinds(tester), [PhonePageKind.spaces]);
          expect(postRow(1), findsOneWidget);
          expect(postRow(2), findsOneWidget);
          // Newest first (01 6.8 item 4).
          expect(
            tester.getTopLeft(postRow(2)).dy,
            lessThan(tester.getTopLeft(postRow(1)).dy),
          );
          expect(allPostsRow(server), findsOneWidget);
          // "+" is always there on touch (01 6.8 item 8), not only on the selected forum.
          for (final forum in [server, ideas]) {
            final add = addAction(forum, en);
            expect(add, findsOneWidget, reason: forum.name);
            final size = tester.getSize(add);
            expect(size.width, greaterThanOrEqualTo(DsDensity.touch.targetMin));
            expect(
              size.height,
              greaterThanOrEqualTo(DsDensity.touch.targetMin),
            );
          }
          expect(addAction(general, en), findsNothing);
          await host.dispose(tester);
        },
      );

      testPhone('a tap on a thread row opens its history M3', platform, (
        tester,
      ) async {
        final host = await open(tester, platform: platform);
        await tester.tap(postRow(2));
        await settlePhone(tester, host);
        expect(host.visited.last, '/t/${threadId(2).value}');
        expect(kinds(tester), [
          PhonePageKind.spaces,
          PhonePageKind.channel,
          PhonePageKind.thread,
        ]);
        await host.dispose(tester);
      });

      testPhone(
        'a tap on the forum name or on "All posts" opens M2',
        platform,
        (tester) async {
          final host = await open(tester, platform: platform);
          await tester.tap(channelRow(server));
          await settlePhone(tester, host);
          expect(host.visited.last, forumPath(server));
          expect(kinds(tester), [PhonePageKind.spaces, PhonePageKind.channel]);

          await tester.tap(find.byKey(const ValueKey('phone.back')));
          await settlePhone(tester, host);
          expect(kinds(tester), [PhonePageKind.spaces]);

          await tester.tap(allPostsRow(server));
          await settlePhone(tester, host);
          expect(host.visited.last, forumPath(server));
          expect(kinds(tester), [PhonePageKind.spaces, PhonePageKind.channel]);
          await host.dispose(tester);
        },
      );

      testPhone('"+" and the draft row open the new chat page', platform, (
        tester,
      ) async {
        final host = await open(tester, platform: platform);
        await tester.tap(addAction(ideas, en));
        await settlePhone(tester, host);
        expect(host.visited.last, newChatPath(ideas));
        expect(kinds(tester), [PhonePageKind.spaces, PhonePageKind.newChat]);
        expect(find.byKey(const ValueKey('newChat.view')), findsOneWidget);
        await host.dispose(tester);

        final again = await ColumnHost.open(tester);
        await tester.runAsync(() async {
          await fillThreads(again.db);
          await again.db.personalDao.upsertDraftIfNewer(
            storedDraft(server.id, 'Half a thought'),
          );
        });
        await pumpPhone(tester, again, platform: platform);
        expect(draftRow(server), findsOneWidget);
        await tester.tap(draftRow(server));
        await settlePhone(tester, again);
        expect(again.visited.last, newChatPath(server));
        expect(kinds(tester), [PhonePageKind.spaces, PhonePageKind.newChat]);
        await again.dispose(tester);
      });
    });
  }

  testWidgets(
    'a row that stops qualifying stays while a finger is on the list and leaves after it',
    (tester) async {
      final host = await open(tester);
      final finger = await tester.startGesture(
        tester.getCenter(channelRow(general)),
      );
      await settlePhone(tester, host);
      await host.run(
        tester,
        () => host.db.threadsDao.upsertIfNewer(post1Done()),
      );
      await settlePhone(tester, host);
      expect(
        postRow(1),
        findsOneWidget,
        reason: 'no row leaves under a finger',
      );

      // The finger leaves without a tap (a cancelled touch opens nothing).
      await finger.cancel();
      await settlePhone(tester, host);
      expect(postRow(1), findsNothing);
      expect(postRow(2), findsOneWidget);
      expect(host.visited, ['/'], reason: 'the touch opened nothing');
      await host.dispose(tester);
    },
  );

  testWidgets('a row stays while the list scrolls', (tester) async {
    final host = await open(tester);
    final list = find.byKey(const ValueKey('sidebar.tree'));
    final scroll = tester.state<ScrollableState>(
      find.descendant(of: list, matching: find.byType(Scrollable)).first,
    );
    // A scroll that the app drives (a fling's tail), without a finger on the list.
    scroll.position.didStartScroll();
    await settlePhone(tester, host);
    await host.run(tester, () => host.db.threadsDao.upsertIfNewer(post1Done()));
    await settlePhone(tester, host);
    expect(postRow(1), findsOneWidget, reason: 'no row leaves while scrolling');
    scroll.position.didEndScroll();
    await settlePhone(tester, host);
    expect(postRow(1), findsNothing);
    await host.dispose(tester);
  });

  testWidgets(
    'with a screen reader the rows stay while M1 shows and update when it leaves',
    (tester) async {
      tester.platformDispatcher.accessibilityFeaturesTestValue =
          _ScreenReader();
      addTearDown(
        tester.platformDispatcher.clearAccessibilityFeaturesTestValue,
      );
      final host = await open(tester);
      await host.run(
        tester,
        () => host.db.threadsDao.upsertIfNewer(post1Done()),
      );
      await settlePhone(tester, host);
      expect(
        postRow(1),
        findsOneWidget,
        reason: 'the screen reader focus may be on the row',
      );

      // M2 on top: M1 is no longer on screen, the row leaves.
      await tester.tap(channelRow(server));
      await settlePhone(tester, host);
      await tester.tap(find.byKey(const ValueKey('phone.back')));
      await settlePhone(tester, host);
      expect(kinds(tester), [PhonePageKind.spaces]);
      expect(postRow(1), findsNothing);
      expect(postRow(2), findsOneWidget);
      await host.dispose(tester);
    },
  );

  testWidgets('the collapse state of a group is kept on the device', (
    tester,
  ) async {
    final host = await open(tester);
    await tester.tap(find.byTooltip(en.sidebarHideThreads).first);
    await settlePhone(tester, host);
    expect(postRow(2), findsNothing);
    expect(
      await host.run(
        tester,
        () => host.db.forumViewDao.watchCollapsedGroups().first,
      ),
      {server.id},
    );
    final writes = await host.run(tester, host.db.outboxDao.allEntries);
    expect(writes, isEmpty, reason: 'never synchronized (01 6.8 item 7)');
    await host.dispose(tester);
  });
}
