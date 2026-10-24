// SPDX-License-Identifier: Apache-2.0

// The phone shell (brief M5-02 Tests; 01 5.3; 15 11; docs/decisions.d/M5-02.md D-M5-02-3): the
// tab bar with four tabs, each tab keeps its stack, a deep link builds the stack Spaces, channel,
// thread at the message, the edge swipe and the back button go back.

import 'dart:io';

import 'package:client/phone/phone_navigation.dart';
import 'package:client/sidebar/providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';
import 'support/phone_app.dart';

final String forum = channelId(1).value;
final String post = threadId(1).value;
final String other = threadId(2).value;
final String message = uuid(0x401);

Finder frame(PhonePageKind kind) =>
    find.byKey(ValueKey('phone.frame.${kind.name}'), skipOffstage: false);

void main() {
  late Directory root;
  setUp(() async => root = Directory.systemTemp.createTempSync('phone'));
  tearDown(() => root.deleteSync(recursive: true));

  // The thread of the tests lies in the forum `channel-1`.
  final overrides = [
    routeSelectionProvider.overrideWith(
      (ref, path) async => path.startsWith('/t/')
          ? RouteSelection(
              channel: channelId(1),
              kind: ChannelKind.forum,
              thread: ThreadId.parse(Uri.parse(path).pathSegments.last),
            )
          : const RouteSelection(),
    ),
    storedChannelOfRouteProvider.overrideWith(
      (ref, id) => Stream.value(channel(1, kind: ChannelKind.forum)),
    ),
  ];

  testWidgets('the tab bar has four tabs; each opens its first page', (
    tester,
  ) async {
    final app = await PhoneApp.start(tester, root: root);
    expect(find.byKey(const ValueKey('phone.tabBar')), findsOneWidget);
    expect(find.byType(NavigationDestination), findsNWidgets(4));
    expect(app.shell.pages.single.kind, PhonePageKind.spaces);
    for (final (tab, kind) in [
      (PhoneTab.mentions, PhonePageKind.mentions),
      (PhoneTab.running, PhonePageKind.running),
      (PhoneTab.me, PhonePageKind.me),
      (PhoneTab.spaces, PhonePageKind.spaces),
    ]) {
      await tester.tap(find.byKey(ValueKey('phone.tab.${tab.name}')));
      await tester.pumpAndSettle();
      expect(app.shell.tab, tab);
      expect(app.shell.pages.single.kind, kind);
      expect(app.location, tab.root);
    }
  });

  testWidgets('a deeper page hides the tab bar and shows the back button', (
    tester,
  ) async {
    final app = await PhoneApp.start(tester, root: root, overrides: overrides);
    await app.go('/c/$forum');
    expect(app.shell.pages.map((p) => p.kind), [
      PhonePageKind.spaces,
      PhonePageKind.channel,
    ]);
    expect(find.byKey(const ValueKey('phone.tabBar')), findsNothing);
    expect(find.byKey(const ValueKey('phone.back')), findsOneWidget);
    // The button is named after the page below.
    expect(find.bySemanticsLabel('Back to Spaces'), findsOneWidget);
  });

  testWidgets('a deep link builds the stack Spaces, channel, thread', (
    tester,
  ) async {
    final app = await PhoneApp.start(tester, root: root, overrides: overrides);
    await tester.tap(find.byKey(const ValueKey('phone.tab.mentions')));
    await tester.pumpAndSettle();
    await app.open(Uri.parse('chat-example://t/$post?m=$message'));
    expect(app.shell.tab, PhoneTab.spaces);
    expect(app.shell.pages, [
      const PhonePage(PhonePageKind.spaces, '/'),
      PhonePage(PhonePageKind.channel, '/c/$forum'),
      PhonePage(PhonePageKind.thread, '/t/$post?m=$message'),
    ]);
    expect(frame(PhonePageKind.spaces), findsOneWidget);
    expect(frame(PhonePageKind.channel), findsOneWidget);
    // The thread opens at the message (15 11).
    expect(
      find.byKey(ValueKey('historyColumn.$post.$message')),
      findsOneWidget,
    );
  });

  testWidgets('each tab keeps its stack', (tester) async {
    final app = await PhoneApp.start(tester, root: root, overrides: overrides);
    await tester.tap(find.byKey(const ValueKey('phone.tab.mentions')));
    await tester.pumpAndSettle();
    await app.go('/t/$post');
    expect(app.shell.tab, PhoneTab.mentions);
    expect(app.shell.pages.map((p) => p.kind), [
      PhonePageKind.mentions,
      PhonePageKind.thread,
    ]);
    // A link opens another thread in Spaces; Mentions keeps its thread.
    await app.open(Uri.parse('chat-example://t/$other'));
    expect(app.shell.tab, PhoneTab.spaces);
    await app.go('/');
    await tester.tap(find.byKey(const ValueKey('phone.tab.mentions')));
    await tester.pumpAndSettle();
    expect(app.shell.tab, PhoneTab.mentions);
    expect(app.location, '/t/$post');
    expect(find.byKey(ValueKey('historyColumn.$post.null')), findsOneWidget);
  });

  testWidgets('the back button goes to the page below', (tester) async {
    final app = await PhoneApp.start(tester, root: root, overrides: overrides);
    await app.open(Uri.parse('chat-example://t/$post'));
    await tester.tap(find.byKey(const ValueKey('phone.back')));
    await tester.pumpAndSettle();
    expect(app.location, '/c/$forum');
    await tester.tap(find.byKey(const ValueKey('phone.back')));
    await tester.pumpAndSettle();
    expect(app.location, '/');
    expect(find.byKey(const ValueKey('phone.tabBar')), findsOneWidget);
  });

  testWidgets('on iOS the swipe from the left edge goes back', (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
    try {
      final app = await PhoneApp.start(
        tester,
        root: root,
        overrides: overrides,
      );
      await app.open(Uri.parse('chat-example://t/$post'));
      expect(app.shell.pages.length, 3);
      await tester.dragFrom(
        const Offset(2, 400),
        Offset(phoneSize.width * 0.8, 0),
      );
      await tester.pumpAndSettle();
      expect(app.location, '/c/$forum');
      expect(app.shell.pages.map((p) => p.kind), [
        PhonePageKind.spaces,
        PhonePageKind.channel,
      ]);
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('the administration is not offered on the phone', (tester) async {
    final app = await PhoneApp.start(tester, root: root);
    await app.go('/admin/people');
    expect(app.shell.tab, PhoneTab.me);
    expect(find.byKey(const ValueKey('target.notFound')), findsOneWidget);
  });
}
