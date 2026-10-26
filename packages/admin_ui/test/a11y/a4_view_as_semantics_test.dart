// SPDX-License-Identifier: Apache-2.0

// Accessibility of A4 View as person (brief M4-32 Design note 1; 01 5.6 and 8): the start of the
// view, its frame banner and the read-only structure say that the view is read-only in words,
// with contrast and targets in light and dark and every control at text scale 2.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../goldens/grants_golden_test.dart' as grants;
import '../goldens/view_as_golden_test.dart' as golden;

void main() {
  dsAccessibilityTests(
    'A4 start',
    build: grants.views['view_as']!,
    window: const Size(1280, 1400),
  );
  for (final name in const ['banner', 'structure', 'structure_later']) {
    dsAccessibilityTests(
      'A4 $name',
      build: golden.views[name]!,
      window: const Size(1280, 1400),
    );
  }
}
