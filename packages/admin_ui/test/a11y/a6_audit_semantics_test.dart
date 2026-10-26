// SPDX-License-Identifier: Apache-2.0

// Accessibility of A6 Audit (brief M4-32 Design note 1; 01 5.6 and 8): the filters and entries of the audit log and the result of the chain check (intact or broken, in words) with contrast and targets in light and dark and every control at text scale 2.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../goldens/audit_golden_test.dart' as golden;

void main() {
  for (final name in const ['list', 'chain_ok', 'chain_broken']) {
    dsAccessibilityTests(
      'A6 $name',
      build: golden.views[name]!,
      window: const Size(1280, 1400),
    );
  }
}
