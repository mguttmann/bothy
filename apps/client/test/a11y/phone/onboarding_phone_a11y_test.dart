// SPDX-License-Identifier: Apache-2.0

// Accessibility of the onboarding E1 to E7 in the phone form (brief M5-10 Design note 2; 01
// 5.6, 8): every screen state, the lock E6 of M5-08 among them; the codes of E4 and E5 are read
// character by character. The announcement of the remaining time at most once per
// `a11y.live_announce_min_interval` is tested in test/phone/onboarding_semantics_test.dart.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../goldens/phone/onboarding_phone_golden_test.dart' as onboarding;
import 'support.dart';

void main() {
  for (final MapEntry(key: name, value: screen)
      in onboarding.screens().entries) {
    phoneScreenTests(
      'onboarding $name',
      build: screen,
      reachable: (l) => [
        if (name.startsWith('e1_') && name != 'e1_connecting')
          find.byKey(const ValueKey('e1.address')),
        if (name.startsWith('e6')) find.byKey(const ValueKey('e6.unlock')),
      ],
      semantics: switch (name) {
        'e4' => (tester, l) => expect(
          find.bySemanticsLabel(l.onboardingCodeLabel('4, 8, 1, 2, 0, 7')),
          findsOneWidget,
        ),
        'e5' => (tester, l) => expect(
          find.bySemanticsLabel(
            l.onboardingCodeLabel('H, Q, 7, P, 3, M, X, R'),
          ),
          findsOneWidget,
        ),
        _ => null,
      },
    );
  }
}
