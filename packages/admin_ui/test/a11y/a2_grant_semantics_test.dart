// SPDX-License-Identifier: Apache-2.0

// Accessibility of A2 Create grant (brief M4-32 Design note 1; 01 5.6 and 8): the three steps of the wizard and the plain-language preview have labelled fields and choices, contrast and targets in light and dark, and keep every control at text scale 2.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../goldens/grants_golden_test.dart' as golden;

void main() {
  for (final name in const ['wizard_who', 'wizard_where', 'wizard_preview']) {
    dsAccessibilityTests(
      'A2 $name',
      build: golden.views[name]!,
      window: const Size(1280, 1400),
    );
  }
}
