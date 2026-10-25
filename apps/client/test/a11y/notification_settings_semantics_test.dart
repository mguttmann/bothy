// SPDX-License-Identifier: Apache-2.0

// Accessibility of the notification settings D9 (brief M4-32 Design note 1; 01 5.1 and 8): the
// level choices say which one is chosen, the tree of spaces, channels and threads is labelled
// and expandable, quiet hours and their days are labelled controls.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/notifications_settings/notify_golden_test.dart' as golden;

void main() {
  dsAccessibilityTests(
    'the notification settings',
    build: golden.views['notify_d9']!,
    window: const Size(1280, 1600),
    reachable: (l) => [
      find.byKey(const ValueKey('notify.defaultLevel')),
      find.byKey(const ValueKey('quiet.add')),
    ],
  );
}
