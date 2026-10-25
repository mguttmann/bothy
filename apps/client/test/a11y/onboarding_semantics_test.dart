// SPDX-License-Identifier: Apache-2.0

// Accessibility of the onboarding screens E1 to E7 on the desktop (brief M4-32 Design note 1;
// 01 5.6 and 8): every field and button is labelled, the codes are read character by
// character, and each screen keeps its controls at text scale 2.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/onboarding/onboarding_golden_test.dart' as golden;

void main() {
  final screens = golden.screens();
  for (final MapEntry(key: name, value: screen) in screens.entries) {
    dsAccessibilityTests(
      'onboarding $name',
      build: screen,
      window: golden.window,
      reachable: (l) => [
        if (name.startsWith('e1_') && name != 'e1_connecting')
          find.byKey(const ValueKey('e1.address')),
        if (name == 'e2_local_passkey_password')
          find.byKey(const ValueKey('e2.password.open')),
      ],
    );
  }

  dsAccessibilityTests(
    'onboarding e4 and e5 codes',
    build: () => Column(children: [screens['e4']!(), screens['e5']!()]),
    window: const Size(640, 1800),
    semantics: (tester, l) {
      expect(
        find.bySemanticsLabel(l.onboardingCodeLabel('4, 8, 1, 2, 0, 7')),
        findsOneWidget,
      );
      expect(
        find.bySemanticsLabel(l.onboardingCodeLabel('H, Q, 7, P, 3, M, X, R')),
        findsOneWidget,
      );
    },
  );
}
