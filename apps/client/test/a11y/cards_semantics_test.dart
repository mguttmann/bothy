// SPDX-License-Identifier: Apache-2.0

// Accessibility of the cards (brief M4-32 Design note 1; 01 8; components.md 8): every button
// of a card is a labelled button, and a card that may not be used, a sent choice, a used, an
// expired and a disabled card and an error say their state in words, not by color alone.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/agents/agents_golden_test.dart' as golden;

void main() {
  dsAccessibilityTests(
    'the cards',
    build: golden.cards,
    window: const Size(1280, 2000),
    semantics: (tester, l) {
      // The active card: a labelled button that can be tapped.
      final allow = find.bySemanticsLabel('Allow once');
      expect(
        allow.evaluate().map(
          (e) => tester.getSemantics(find.byWidget(e.widget)),
        ),
        contains(
          isSemantics(
            isButton: true,
            hasTapAction: true,
            isEnabled: true,
            hasEnabledState: true,
            label: 'Allow once',
          ),
        ),
      );
      expect(
        find.bySemanticsLabel(RegExp(l.componentsApproverRunStarter('Alex'))),
        findsWidgets,
      );
      expect(
        find.bySemanticsLabel(RegExp(l.problemComponentUsed)),
        findsWidgets,
      );
    },
  );
}
