// SPDX-License-Identifier: Apache-2.0

// Accessibility of A1 People (brief M4-32 Design note 1; 01 5.6 and 8): the administration shell, the people list, a person, a group and the queue of waiting devices have labelled controls with their state in words, contrast and targets in light and dark, and keep every control at text scale 2.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../goldens/admin_golden_test.dart' as golden;

void main() {
  for (final name in const [
    'shell',
    'people',
    'person',
    'group',
    'waiting_devices',
  ]) {
    dsAccessibilityTests(
      'A1 $name',
      build: golden.views[name]!,
      window: const Size(1280, 1400),
    );
  }
}
