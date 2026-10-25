// SPDX-License-Identifier: Apache-2.0

// Touch targets on Android (brief M5-03 fixed requirement 16, Tests; 01 5.4, 8): every phone
// screen, the Android frame of M1 to M3 and every onboarding screen in phone form meet the
// Android tap target guideline (48 by 48, `density.target-min` of the `touch` context) and label
// every tap target, with the target platform Android; so do the app's Me page, tab bar and top
// app bar.

import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/onboarding/onboarding_golden_test.dart' as onboarding;
import 'support/phone_app.dart';
import 'support/phone_host.dart';
import 'support/phone_scenes.dart';

Future<void> expectGuidelines(WidgetTester tester) async {
  await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
  await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
}

/// A widget test with the target platform Android, reset inside the test.
void testAndroid(String name, Future<void> Function(WidgetTester) body) =>
    testWidgets(name, (tester) async {
      debugDefaultTargetPlatformOverride = TargetPlatform.android;
      try {
        await body(tester);
      } finally {
        debugDefaultTargetPlatformOverride = null;
      }
    });

void main() {
  test('the touch density asks for the Android guideline', () {
    expect(DsDensity.touch.targetMin, greaterThanOrEqualTo(48));
  });

  final scenes = {
    ...phoneScenes,
    for (final MapEntry(key: name, value: scene) in androidPhoneScenes.entries)
      'android $name': scene,
  };
  for (final MapEntry(key: name, value: scene) in scenes.entries) {
    testAndroid('$name meets the Android tap target guidelines', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();
      phoneWindow(tester);
      await tester.pumpWidget(hostPhone(Center(child: scene())));
      await tester.pumpAndSettle();
      await expectGuidelines(tester);
      semantics.dispose();
    });
  }

  for (final MapEntry(key: name, value: screen)
      in onboarding.screens().entries) {
    testAndroid('onboarding $name meets the Android tap target guidelines', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();
      phoneWindow(tester);
      await tester.pumpWidget(hostPhone(screen()));
      await tester.pumpAndSettle();
      await expectGuidelines(tester);
      semantics.dispose();
    });
  }

  testAndroid('the Me page, the tab bar and the top app bar meet them', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    final root = Directory.systemTemp.createTempSync('targets');
    addTearDown(() => root.deleteSync(recursive: true));
    final app = await PhoneApp.start(tester, root: root, platform: 'android');
    await app.go('/settings');
    await expectGuidelines(tester);
    await app.go('/settings/appearance');
    expect(find.byKey(const ValueKey('phone.topAppBar')), findsOneWidget);
    await expectGuidelines(tester);
    semantics.dispose();
  });
}
