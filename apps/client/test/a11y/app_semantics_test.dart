// SPDX-License-Identifier: Apache-2.0

// What the desktop app gives every screen (brief M4-32 Design note 1; 01 8 "Language" and
// "Focus", 01 9): the language of the app is set in the semantics tree (the application
// locale that screen readers use for the whole window) and follows the system language before
// a person chose one; the app draws one visible focus ring for every control (DsFocusRing).

import 'dart:io';

import 'package:client/app/app.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';

/// The onboarding at E1, so the app starts without a server.
class _AtServerAddress extends OnboardingController {
  @override
  OnboardingState build() => const OnboardingState(step: OnboardingStep.server);
}

Future<void> pumpApp(WidgetTester tester, Directory root) async {
  tester.view
    ..physicalSize = const Size(1280, 900)
    ..devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        appPortsProvider.overrideWithValue(
          testPorts(dataRoot: root, platform: 'macos'),
        ),
        onboardingControllerProvider.overrideWith(_AtServerAddress.new),
      ],
      child: const ClientApp(),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  for (final language in ['de', 'en']) {
    testWidgets('the language of the app is set in the semantics tree: '
        '$language from the system', (tester) async {
      final root = Directory.systemTemp.createTempSync('language');
      addTearDown(() => root.deleteSync(recursive: true));
      tester.platformDispatcher.localesTestValue = [Locale(language)];
      addTearDown(tester.platformDispatcher.clearLocalesTestValue);
      await pumpApp(tester, root);
      expect(
        tester.platformDispatcher.applicationLocale?.languageCode,
        language,
      );
    });
  }

  testWidgets('the app draws the focus ring for every control', (tester) async {
    final root = Directory.systemTemp.createTempSync('ring');
    addTearDown(() => root.deleteSync(recursive: true));
    await pumpApp(tester, root);
    expect(find.byType(DsFocusRing), findsOneWidget);
  });
}
