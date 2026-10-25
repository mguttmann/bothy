// SPDX-License-Identifier: Apache-2.0

// Accessibility of the notification settings D9 (brief M4-32 Design note 1; 01 5.1 and 8): the
// level choices say which one is chosen, the tree of spaces, channels and threads is labelled
// and expandable, quiet hours and their days are labelled controls.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:client/notifications_settings/quiet_hours_editor.dart';

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

  testWidgets(
    'the quiet hours editor: roles, labels and states for screen readers',
    (tester) async {
      final handle = tester.ensureSemantics();
      tester.view
        ..physicalSize = const Size(1280, 1000)
        ..devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(
        dsA11yHost(
          Builder(
            builder: (context) => TextButton(
              onPressed: () => showQuietHoursEditor(
                context,
                personZone: 'Europe/Berlin',
                timeZones: () => const ['Asia/Tokyo', 'Europe/Berlin'],
              ),
              child: const Text('open'),
            ),
          ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      final problems = dsSemanticsProblems(tester);
      expect(problems, isEmpty, reason: problems.join('\n'));
      handle.dispose();
    },
  );
}
