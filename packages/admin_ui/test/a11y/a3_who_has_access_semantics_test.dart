// SPDX-License-Identifier: Apache-2.0

// Accessibility of A3 Who has access? (brief M4-32 Design note 1; 01 5.6 and 8): people, groups and agents with their effective level in words, contrast and targets in light and dark, and every control at text scale 2.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../goldens/grants_golden_test.dart' as golden;

void main() {
  for (final name in const ['who_has_access']) {
    dsAccessibilityTests(
      'A3 $name',
      build: golden.views[name]!,
      window: const Size(1280, 1400),
    );
  }
}
