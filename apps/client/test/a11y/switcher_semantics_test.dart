// SPDX-License-Identifier: Apache-2.0

// Accessibility of the quick switcher (brief M4-32 Design note 1; 01 7 and 8; components.md
// 12): the field is a labelled text field, every entry reads its kind and name, the filters
// say whether they are on, and Close is a labelled button.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/search/switcher_golden_test.dart' as golden;

void main() {
  dsAccessibilityTests(
    'the quick switcher',
    build: golden.framed,
    semantics: (tester, l) {
      expect(
        find.bySemanticsLabel(RegExp('Rollout plan for the gateway')),
        findsWidgets,
      );
    },
    reachable: (l) => [
      find.byKey(const ValueKey('switcher.field')),
      find.byKey(const ValueKey('switcher.close')),
    ],
  );
}
