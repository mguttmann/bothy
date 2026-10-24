// SPDX-License-Identifier: Apache-2.0

// The privacy cover of the phone (brief M5-02 fixed requirement 14, Design note 6; 01 5.6 E6):
// `inactive` shows the cover with no content in the tree before the app switcher takes its
// picture, `resumed` removes it; the desktop has no cover.

import 'dart:io';

import 'package:client/app/app.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/phone/privacy_cover.dart';
import 'package:client/platform/lifecycle.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';
import '../support/widgets.dart';
import 'support/phone_app.dart';

const String text = 'Please include expired threads.';

void main() {
  test('every state but resumed covers the content', () {
    expect(
      {for (final s in AppLifecycleState.values) s: coversContent(s)},
      {
        AppLifecycleState.detached: true,
        AppLifecycleState.resumed: false,
        AppLifecycleState.inactive: true,
        AppLifecycleState.hidden: true,
        AppLifecycleState.paused: true,
      },
    );
  });

  testWidgets('the cover notifies only when it changes', (tester) async {
    final cover = PrivacyCover(initial: AppLifecycleState.resumed);
    addTearDown(cover.dispose);
    var changes = 0;
    cover.addListener(() => changes++);
    cover
      ..onState(AppLifecycleState.inactive)
      ..onState(AppLifecycleState.hidden)
      ..onState(AppLifecycleState.paused);
    expect((cover.covered, changes), (true, 1));
    cover.onState(AppLifecycleState.resumed);
    expect((cover.covered, changes), (false, 2));
  });

  testWidgets(
    'inactive shows the cover and no message text; resumed removes it',
    (tester) async {
      final semantics = tester.ensureSemantics();
      final cover = PrivacyCover(initial: AppLifecycleState.resumed);
      addTearDown(cover.dispose);
      await tester.pumpWidget(
        hostScreen(
          PrivacyCoverHost(
            cover: cover,
            productName: 'Team Chat',
            child: const Center(child: Text(text)),
          ),
        ),
      );
      expect(find.text(text), findsOneWidget);
      expect(find.byKey(const ValueKey('privacy.cover')), findsNothing);

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
      await tester.pump();
      expect(cover.covered, isTrue);
      expect(find.byKey(const ValueKey('privacy.cover')), findsOneWidget);
      expect(find.text('Team Chat'), findsOneWidget);
      // Neither painted, nor in the semantics tree, nor hit-testable.
      expect(find.text(text), findsNothing);
      expect(find.bySemanticsLabel(text), findsNothing);

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await tester.pump();
      expect(find.byKey(const ValueKey('privacy.cover')), findsNothing);
      expect(find.text(text), findsOneWidget);
      semantics.dispose();
    },
  );

  testWidgets('the phone app covers itself; the desktop app does not', (
    tester,
  ) async {
    final root = Directory.systemTemp.createTempSync('cover');
    addTearDown(() => root.deleteSync(recursive: true));
    await PhoneApp.start(tester, root: root);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    await tester.pump();
    expect(find.byKey(const ValueKey('privacy.cover')), findsOneWidget);
    expect(find.byKey(const ValueKey('phone.shell')), findsNothing);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pump();
    expect(find.byKey(const ValueKey('phone.shell')), findsOneWidget);

    // A new app on desktop ports.
    await tester.pumpWidget(const SizedBox());
    desktopWindow(tester);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appPortsProvider.overrideWithValue(testPorts(dataRoot: root)),
          onboardingControllerProvider.overrideWith(
            () => ScriptedOnboarding(
              const OnboardingState(step: OnboardingStep.signedIn),
            ),
          ),
        ],
        child: const ClientApp(),
      ),
    );
    await tester.pumpAndSettle();
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    await tester.pump();
    expect(find.byKey(const ValueKey('privacy.cover')), findsNothing);
    expect(find.byKey(const ValueKey('column.sidebar')), findsOneWidget);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
  });
}
