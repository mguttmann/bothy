// SPDX-License-Identifier: Apache-2.0

// Accessibility of M4 Large components (brief M5-10 Design note 2; 01 5.3 M4, 8): buttons at
// full width, the select that opens the system picker and the form that opens as a sheet.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../phone/support/phone_scenes.dart';
import 'support.dart';

void main() {
  phoneScreenTests(
    'M4 Large components',
    build: componentsScene,
    reachable: (l) => [
      find.byKey(const ValueKey('ds-component-select-model')),
      find.byKey(const ValueKey('ds-component-form-details')),
    ],
  );
}
