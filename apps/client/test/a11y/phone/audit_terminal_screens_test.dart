// SPDX-License-Identifier: Apache-2.0

// The entries m8_terminal and m9_hosts of the platform audits (brief M5-13 Design note 3) show
// the text their native tests wait for (AccessibilityAuditTests.swift "testM8Terminal",
// "testM9Hosts"; AccessibilityChecksTest.kt "m8Terminal", "m9Hosts"), so a scene that never
// appears fails here and not only on a simulator or an emulator.

import 'package:client/app/form_factor.dart';
import 'package:design_system/testing.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../integration_test/a11y_audit/audit_screens.dart';
import '../../phone/support/terminal_scenes.dart';

void main() {
  for (final (screen, text) in [
    ('m8_terminal', 'Reattach'),
    ('m9_hosts', 'Build host'),
  ]) {
    testWidgets('the audit entry $screen shows "$text"', (tester) async {
      final handle = tester.ensureSemantics();
      await tester.runAsync(prepareTerminalScenes);
      await tester.pumpWidget(
        dsPhoneA11yHost(
          auditScreens[screen]!(),
          wrap: (child) =>
              FormFactorScope(formFactor: FormFactor.phone, child: child),
        ),
      );
      await dsPumpIcons(tester);
      // The scene waits for the host; M8 then loads the host's sessions once.
      for (var i = 0; i < 5; i++) {
        await tester.pump();
      }
      // The native tests look for a label that contains the text.
      expect(find.bySemanticsLabel(RegExp(RegExp.escape(text))), findsWidgets);
      handle.dispose();
    });
  }
}
