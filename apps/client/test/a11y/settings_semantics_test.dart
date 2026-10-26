// SPDX-License-Identifier: Apache-2.0

// Accessibility of the settings (brief M4-32 Design note 1; 01 5.1 and 8): every section,
// devices and sessions included, has labelled controls with their state (a switch says on or
// off, a choice says which one is chosen), and the time zone field has a name.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/settings/settings_golden_test.dart' as golden;

void main() {
  for (final MapEntry(key: name, value: section) in golden.sections.entries) {
    dsAccessibilityTests(
      'the settings section $name',
      build: section,
      semantics: name == 'language'
          ? (tester, l) {
              final zone = tester.getSemantics(
                find.descendant(
                  of: find.byKey(const ValueKey('settings.timeZone')),
                  matching: find.byType(TextField),
                ),
              );
              expect(zone.label, isNotEmpty);
            }
          : null,
    );
  }
}
