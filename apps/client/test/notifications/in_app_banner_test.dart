// SPDX-License-Identifier: Apache-2.0

// The in-app banner of the phone (13 4.6 item 1; docs/decisions.d/AUD-client-app-01.md
// AUD-CA1/1 and AUD-CA1/2): the presenter keeps the newest hint, the host shows it over the
// app for its time, a tap opens the target, the close button takes it away, a quiet hint gives
// no haptic tap, and in the phone app nothing of a hint is on screen behind the closed app lock
// or after the account ended.

import 'dart:io';

import 'package:client/app/app.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/notifications/in_app_banner.dart';
import 'package:client/notifications/providers.dart';
import 'package:client/notifications/toast_content.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lock/support/lock_app.dart';
import '../support/fakes.dart';
import '../support/widgets.dart';

const ToastContent _release = ToastContent(
  title: 'general · Release notes',
  body: 'Sam: Build is green',
);
const ToastContent _printer = ToastContent(
  title: 'support · Printer',
  body: 'Sam: Paper jam',
);

final Finder _banner = find.byKey(const ValueKey('notification.banner'));
final Finder _open = find.byKey(const ValueKey('notification.banner.open'));
final Finder _dismiss = find.byKey(
  const ValueKey('notification.banner.dismiss'),
);

Future<void> _show(
  InAppBanners banners,
  ToastContent content, {
  String tag = 'thread:release',
  bool silent = false,
  String payload = '/t/release',
}) => banners.show(content, tag: tag, silent: silent, payload: payload);

/// Records the haptic taps the app asks the platform for.
List<String> _haptics(WidgetTester tester) {
  final taps = <String>[];
  tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
    SystemChannels.platform,
    (call) async {
      if (call.method == 'HapticFeedback.vibrate') {
        taps.add(call.arguments as String);
      }
      return null;
    },
  );
  addTearDown(
    () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      null,
    ),
  );
  return taps;
}

void main() {
  group('the presenter', () {
    late InAppBanners banners;
    setUp(() => banners = InAppBanners());
    tearDown(() => banners.dispose());

    test('keeps the newest hint, whatever its slot', () async {
      expect(banners.current.value, isNull);
      await _show(banners, _release);
      expect(banners.current.value!.content, same(_release));
      expect(banners.current.value!.tag, 'thread:release');
      await _show(banners, _printer, tag: 'thread:printer');
      expect(banners.current.value!.content, same(_printer));
    });

    test('a tap gives the app path and takes the banner away', () async {
      final clicks = <String>[];
      final sub = banners.clicks.listen(clicks.add);
      await _show(banners, _release, payload: '/t/release?m=1');
      banners.open(banners.current.value!);
      await pumpEventQueue();
      expect(clicks, ['/t/release?m=1']);
      expect(banners.current.value, isNull);
      await sub.cancel();
    });

    test('dismissing an older banner keeps the one that replaced it', () async {
      await _show(banners, _release);
      final first = banners.current.value!;
      await _show(banners, _printer, tag: 'thread:printer');
      banners.dismiss(first);
      expect(banners.current.value!.content, same(_printer));
      banners.dismiss(banners.current.value!);
      expect(banners.current.value, isNull);
    });

    test('a cancelled slot goes, another slot stays', () async {
      await _show(banners, _release);
      await banners.cancelThread('thread:printer');
      expect(banners.current.value, isNotNull);
      await banners.cancelThread('thread:release');
      expect(banners.current.value, isNull);
    });

    test(
      'clear takes the banner away; a banner never starts the app',
      () async {
        await _show(banners, _release);
        banners.clear();
        expect(banners.current.value, isNull);
        expect(await banners.launchPayload(), isNull);
      },
    );

    test('nothing happens after dispose, and no text names content', () async {
      await _show(banners, _release);
      final banner = banners.current.value!;
      expect(banner.toString(), isNot(contains('Release')));
      expect(banner.toString(), isNot(contains('Build')));
      await banners.dispose();
      await _show(banners, _printer);
      banners
        ..open(banner)
        ..dismiss(banner)
        ..clear();
      await banners.cancelThread('thread:release');
    });
  });

  group('the host', () {
    late InAppBanners banners;
    setUp(() => banners = InAppBanners());
    tearDown(() => banners.dispose());

    Future<void> pump(
      WidgetTester tester, {
      bool visible = true,
      bool accessibleNavigation = false,
    }) => tester.pumpWidget(
      hostScreen(
        Builder(
          builder: (context) => MediaQuery(
            data: MediaQuery.of(context)
                .copyWith(accessibleNavigation: accessibleNavigation),
            child: InAppBannerHost(
              banners: banners,
              visible: visible,
              child: const Center(child: Text('the app')),
            ),
          ),
        ),
      ),
    );

    testWidgets('shows title and text over the app and leaves after its time', (
      tester,
    ) async {
      await pump(tester);
      expect(_banner, findsNothing);
      await _show(banners, _release);
      await tester.pump();
      expect(find.text('the app'), findsOneWidget);
      expect(
        find.descendant(of: _banner, matching: find.text(_release.title)),
        findsOneWidget,
      );
      expect(
        find.descendant(of: _banner, matching: find.text(_release.body)),
        findsOneWidget,
      );
      await tester.pump(InAppBannerHost.stay - const Duration(seconds: 1));
      expect(_banner, findsOneWidget);
      await tester.pump(const Duration(seconds: 1));
      expect(_banner, findsNothing);
      expect(banners.current.value, isNull);
    });

    testWidgets('the app under the banner still takes taps beside it', (
      tester,
    ) async {
      var taps = 0;
      await tester.pumpWidget(
        hostScreen(
          InAppBannerHost(
            banners: banners,
            child: Center(
              child: TextButton(
                onPressed: () => taps++,
                child: const Text('the app'),
              ),
            ),
          ),
        ),
      );
      await _show(banners, _release);
      await tester.pump();
      expect(_banner, findsOneWidget);
      await tester.tap(find.text('the app'));
      expect(taps, 1);
      await tester.pump(InAppBannerHost.stay);
    });

    testWidgets('a banner taken away while a frame builds goes with the next '
        'frame', (tester) async {
      // A provider may clear the banner while the widgets build (the engine of an account
      // that ended): the host must not rebuild in the middle of that frame.
      Widget tree({required bool clearing}) => hostScreen(
        Column(
          children: [
            Expanded(
              child: InAppBannerHost(
                banners: banners,
                child: const Center(child: Text('the app')),
              ),
            ),
            Builder(
              builder: (context) {
                if (clearing) banners.clear();
                return const SizedBox.shrink();
              },
            ),
          ],
        ),
      );
      await tester.pumpWidget(tree(clearing: false));
      await _show(banners, _release);
      await tester.pump();
      expect(_banner, findsOneWidget);
      await tester.pumpWidget(tree(clearing: true));
      expect(tester.takeException(), isNull);
      await tester.pump();
      expect(_banner, findsNothing);
    });

    testWidgets('a newer hint replaces the banner and gets its whole time', (
      tester,
    ) async {
      await pump(tester);
      await _show(banners, _release);
      await tester.pump();
      await tester.pump(InAppBannerHost.stay - const Duration(seconds: 1));
      await _show(banners, _printer, tag: 'thread:printer');
      await tester.pump();
      expect(find.text(_release.title), findsNothing);
      expect(find.text(_printer.title), findsOneWidget);
      await tester.pump(InAppBannerHost.stay - const Duration(seconds: 1));
      expect(find.text(_printer.title), findsOneWidget);
      await tester.pump(const Duration(seconds: 1));
      expect(_banner, findsNothing);
    });

    testWidgets('with a screen reader the banner stays until it is dismissed', (
      tester,
    ) async {
      await pump(tester, accessibleNavigation: true);
      await _show(banners, _release);
      await tester.pump();
      await tester.pump(InAppBannerHost.stay * 10);
      expect(_banner, findsOneWidget);
      await tester.tap(_dismiss);
      await tester.pump();
      expect(_banner, findsNothing);
    });

    testWidgets('a tap opens the target, the close button only dismisses', (
      tester,
    ) async {
      final clicks = <String>[];
      final sub = banners.clicks.listen(clicks.add);
      addTearDown(sub.cancel);
      await pump(tester);
      await _show(banners, _release, payload: '/t/release?m=1');
      await tester.pump();
      await tester.tap(_dismiss);
      await tester.pump();
      expect(_banner, findsNothing);
      expect(clicks, isEmpty);

      await _show(banners, _release, payload: '/t/release?m=1');
      await tester.pump();
      await tester.tap(_open);
      await tester.pump();
      expect(_banner, findsNothing);
      expect(clicks, ['/t/release?m=1']);
      // The timer of the dismissed and of the opened banner is gone with it.
      await tester.pump(InAppBannerHost.stay);
    });

    testWidgets('a hint gives one haptic tap, a quiet hint none', (
      tester,
    ) async {
      final taps = _haptics(tester);
      await pump(tester);
      await _show(banners, _release, silent: true);
      await tester.pump();
      expect(_banner, findsOneWidget);
      expect(taps, isEmpty, reason: 'quiet hours: no tap');

      await _show(banners, _printer, tag: 'thread:printer');
      await tester.pump();
      expect(taps, ['HapticFeedbackType.mediumImpact']);
      await tester.pump(InAppBannerHost.stay);
      expect(taps, hasLength(1));
    });

    testWidgets('while it may not show, a hint waits and then shows once', (
      tester,
    ) async {
      final taps = _haptics(tester);
      await pump(tester, visible: false);
      await _show(banners, _release);
      await tester.pump();
      expect(_banner, findsNothing);
      expect(find.text(_release.body), findsNothing);
      expect(taps, isEmpty);
      // Its time has not started: it is still there long after.
      await tester.pump(InAppBannerHost.stay * 2);

      await pump(tester);
      expect(_banner, findsOneWidget);
      expect(taps, hasLength(1));
      // Hidden again and back: the same banner taps no second time.
      await pump(tester, visible: false);
      expect(_banner, findsNothing);
      await pump(tester);
      expect(_banner, findsOneWidget);
      expect(taps, hasLength(1));
      await tester.pump(InAppBannerHost.stay);
      expect(_banner, findsNothing);
    });
  });

  group('in the app', () {
    late Directory root;
    setUp(() => root = Directory.systemTemp.createTempSync('banner'));
    tearDown(() => root.deleteSync(recursive: true));

    InAppBanners? bannersOf(WidgetTester tester) =>
        ProviderScope.containerOf(tester.element(find.byType(ClientApp)))
            .read(inAppBannersProvider);

    testWidgets('a desktop build has no in-app banner', (tester) async {
      desktopWindow(tester);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            appPortsProvider.overrideWithValue(
              testPorts(dataRoot: root, platform: 'macos'),
            ),
            onboardingControllerProvider.overrideWith(
              () => ScriptedOnboarding(
                const OnboardingState(step: OnboardingStep.server),
              ),
            ),
          ],
          child: const ClientApp(),
        ),
      );
      await tester.pumpAndSettle();
      expect(bannersOf(tester), isNull);
      expect(find.byType(InAppBannerHost), findsNothing);
    });

    testWidgets('behind the closed app lock no hint is on screen; '
        'it shows after the unlock', (tester) async {
      final rig = LockRig();
      await startLocked(tester, rig, root: root);
      expect(e6, findsOneWidget);
      await _show(bannersOf(tester)!, _release);
      await tester.pump();
      expect(_banner, findsNothing);
      expect(find.text(_release.title), findsNothing);
      expect(find.text(_release.body), findsNothing);

      await tester.tap(find.byKey(const ValueKey('e6.unlock')));
      await tester.pumpAndSettle();
      expect(e6, findsNothing);
      expect(_banner, findsOneWidget);
      expect(find.text(_release.body), findsOneWidget);
      await tester.pump(InAppBannerHost.stay);
      expect(_banner, findsNothing);
      await rig.dispose();
    });

    testWidgets('a banner leaves the screen when the lock closes over it', (
      tester,
    ) async {
      final rig = LockRig(lockAtStart: false);
      await startLocked(tester, rig, root: root);
      await _show(bannersOf(tester)!, _release);
      await tester.pump();
      expect(_banner, findsOneWidget);
      rig.advance(const Duration(minutes: 5));
      await rig.controller.check();
      await tester.pump();
      expect(e6, findsOneWidget);
      expect(_banner, findsNothing);
      expect(find.text(_release.body), findsNothing);
      await tester.pumpAndSettle();
      await rig.dispose();
    });

    testWidgets('signed out, no banner shows', (tester) async {
      final app = await PhoneApp.start(tester, root: root);
      final banners = bannersOf(tester)!;
      await _show(banners, _release);
      await tester.pump();
      expect(_banner, findsOneWidget);
      app.control.show(const OnboardingState(step: OnboardingStep.signedOut));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('screen.e7')), findsOneWidget);
      expect(_banner, findsNothing);
      expect(find.text(_release.body), findsNothing);
      await tester.pump(InAppBannerHost.stay);
    });
  });
}
