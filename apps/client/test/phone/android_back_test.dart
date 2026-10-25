// SPDX-License-Identifier: Apache-2.0

// System back and the top app bar on Android (brief M5-03 fixed requirement 11, Tests; 01 5.4;
// docs/decisions.d/M5-03.md D-M5-03-6): back pops the stack of the tab (thread to the post list
// to Spaces), closes a sheet without leaving its page, and on the root of a tab leaves the app
// (`SystemNavigator.pop`, recorded by a fake of the platform channel). A deeper page shows the
// top app bar with the up arrow and the page's title instead of the iOS back button.

import 'dart:async';
import 'dart:io';

import 'package:client/phone/phone_navigation.dart';
import 'package:client/sidebar/providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';
import 'support/phone_app.dart';

final String forum = channelId(1).value;
final String post = threadId(1).value;

/// Records `SystemNavigator.pop`, the call that leaves the app when nothing handled back.
final class SystemNavigatorFake {
  SystemNavigatorFake(WidgetTester tester) {
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        if (call.method == 'SystemNavigator.pop') pops++;
        return null;
      },
    );
    addTearDown(
      () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        null,
      ),
    );
  }

  /// How often the app asked to leave.
  int pops = 0;
}

/// Presses the system back button once and lets the app settle.
Future<void> systemBack(WidgetTester tester) async {
  await tester.binding.handlePopRoute();
  await tester.pumpAndSettle();
}

void main() {
  late Directory root;
  setUp(() async => root = Directory.systemTemp.createTempSync('android_back'));
  tearDown(() => root.deleteSync(recursive: true));

  /// A widget test with the target platform Android (the Material routes); the override is
  /// reset inside the test, which the binding checks before the tear-down.
  void testAndroid(String name, Future<void> Function(WidgetTester) body) =>
      testWidgets(name, (tester) async {
        debugDefaultTargetPlatformOverride = TargetPlatform.android;
        try {
          await body(tester);
        } finally {
          debugDefaultTargetPlatformOverride = null;
        }
      });

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

  Future<PhoneApp> start(WidgetTester tester) => PhoneApp.start(
    tester,
    root: root,
    overrides: overrides,
    platform: 'android',
  );

  testAndroid('back goes from the thread to the post list, then to Spaces', (
    tester,
  ) async {
    final navigator = SystemNavigatorFake(tester);
    final app = await start(tester);
    await app.open(Uri.parse('chat-example://t/$post'));
    expect(app.shell.pages, hasLength(3));

    await systemBack(tester);
    expect(app.location, '/c/$forum');
    expect(app.shell.pages.map((p) => p.kind), [
      PhonePageKind.spaces,
      PhonePageKind.channel,
    ]);

    await systemBack(tester);
    expect(app.location, '/');
    expect(app.shell.pages.single.kind, PhonePageKind.spaces);
    expect(find.byKey(const ValueKey('phone.tabBar')), findsOneWidget);
    expect(navigator.pops, 0);
  });

  testAndroid('back on the root of a tab leaves the app', (tester) async {
    final navigator = SystemNavigatorFake(tester);
    final app = await start(tester);
    await systemBack(tester);
    expect(navigator.pops, 1);
    expect(app.location, '/');

    // Every tab's root, not only Spaces, leaves the app (01 5.4).
    await tester.tap(find.byKey(const ValueKey('phone.tab.mentions')));
    await tester.pumpAndSettle();
    await systemBack(tester);
    expect(navigator.pops, 2);
    expect(app.location, '/mentions');
  });

  testAndroid('back closes a sheet and stays on its page', (tester) async {
    final navigator = SystemNavigatorFake(tester);
    final app = await start(tester);
    await app.open(Uri.parse('chat-example://t/$post'));
    final page = tester.element(
      find.byKey(const ValueKey('phone.frame.thread')),
    );
    for (final root in [false, true]) {
      // A sheet of the page's navigator and one of the app's navigator.
      unawaited(
        showModalBottomSheet<void>(
          context: page,
          useRootNavigator: root,
          builder: (_) =>
              const SizedBox(key: ValueKey('test.sheet'), height: 200),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('test.sheet')), findsOneWidget);

      await systemBack(tester);
      expect(find.byKey(const ValueKey('test.sheet')), findsNothing);
      expect(app.location, '/t/$post', reason: 'root navigator: $root');
      expect(app.shell.pages, hasLength(3));
    }
    expect(navigator.pops, 0);
  });

  testAndroid('a deeper page shows the top app bar with the up arrow', (
    tester,
  ) async {
    final app = await start(tester);
    await app.go('/c/$forum');
    expect(find.byKey(const ValueKey('phone.topAppBar')), findsOneWidget);
    expect(find.byKey(const ValueKey('phone.tabBar')), findsNothing);
    // The up arrow is named after the page below, the title is the page's own.
    expect(find.bySemanticsLabel('Back to Spaces'), findsOneWidget);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('phone.topAppBar')),
        matching: find.textContaining('channel-1'),
      ),
      findsOneWidget,
    );

    await tester.tap(find.byKey(const ValueKey('phone.back')));
    await tester.pumpAndSettle();
    expect(app.location, '/');
    expect(find.byKey(const ValueKey('phone.topAppBar')), findsNothing);
  });

  testWidgets('an iOS build keeps the iOS back button', (tester) async {
    final app = await PhoneApp.start(tester, root: root, overrides: overrides);
    await app.go('/c/$forum');
    expect(find.byKey(const ValueKey('phone.topAppBar')), findsNothing);
    expect(find.byKey(const ValueKey('phone.back')), findsOneWidget);
  });
}
