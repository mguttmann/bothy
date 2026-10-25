// SPDX-License-Identifier: Apache-2.0

// Touch targets on the phone (brief M5-02 fixed requirement 12, Tests; 01 5.3, 8): every phone
// screen (with M11 and M12 of brief M5-14) and every onboarding screen in phone form meets the
// iOS tap target guideline (44 by 44 points, below `density.target-min` of the `touch` context)
// and labels every tap target.

import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/onboarding/onboarding_golden_test.dart' as onboarding;
import '../goldens/phone/run_graph_phone_golden_test.dart' as run_graph;
import 'support/phone_app.dart';
import 'support/phone_host.dart';
import 'support/phone_scenes.dart';

Future<void> expectGuidelines(WidgetTester tester) async {
  await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
  await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
}

void main() {
  test('the touch density asks for more than the iOS guideline', () {
    expect(DsDensity.touch.targetMin, greaterThanOrEqualTo(44));
  });

  for (final MapEntry(key: name, value: scene) in phoneScenes.entries) {
    testWidgets('$name meets the tap target guidelines', (tester) async {
      final semantics = tester.ensureSemantics();
      phoneWindow(tester);
      await tester.pumpWidget(hostPhone(Center(child: scene())));
      await tester.pumpAndSettle();
      await expectGuidelines(tester);
      semantics.dispose();
    });
  }

  // M11 and M12 (brief M5-14 fixed requirement 11).
  for (final MapEntry(key: name, value: scene) in run_graph.scenes.entries) {
    testWidgets('$name meets the tap target guidelines', (tester) async {
      final semantics = tester.ensureSemantics();
      phoneWindow(
        tester,
        size: name.endsWith('landscape') ? run_graph.landscape : phoneSize,
      );
      await tester.pumpWidget(hostPhone(Center(child: scene())));
      await dsPumpIcons(tester);
      await expectGuidelines(tester);
      semantics.dispose();
    });
  }

  for (final MapEntry(key: name, value: screen)
      in onboarding.screens().entries) {
    testWidgets('onboarding $name meets the tap target guidelines', (
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

  testWidgets('the Me page and the tab bar meet the tap target guidelines', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    final root = Directory.systemTemp.createTempSync('targets');
    addTearDown(() => root.deleteSync(recursive: true));
    final app = await PhoneApp.start(tester, root: root);
    await app.go('/settings');
    await expectGuidelines(tester);
    semantics.dispose();
  });
}
