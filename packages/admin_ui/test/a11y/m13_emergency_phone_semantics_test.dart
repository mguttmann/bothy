// SPDX-License-Identifier: Apache-2.0

// Accessibility of M13 Emergency on the phone (brief M5-10 Design note 2; 01 5.3 M13, 8): the
// people with their search, one person with devices, grants and "Disable person", and the
// sheet of each action that says what happens and names the step-up.

import 'package:design_system/testing.dart';

import '../goldens/emergency_golden_test.dart' as golden;

void main() {
  for (final MapEntry(key: name, value: view)
      in golden.emergencyViews.entries) {
    dsPhoneAccessibilityTests('M13 Emergency $name', build: view);
  }
}
