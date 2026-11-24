// SPDX-License-Identifier: Apache-2.0

// design-system-6: every input of a form card carries its field label as its accessible name.

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  testWidgets('the text field, the switch and the choices of a form are named', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    await pumpDs(
      tester,
      DsMessageComponents(
        const DsComponentData(
          messageId: 'm1',
          items: [
            DsComponentItem(
              customId: 'f',
              type: DsComponentType.form,
              fields: [
                DsFormField(name: 'reason', label: 'Reason'),
                DsFormField(
                  name: 'urgent',
                  label: 'Urgent',
                  type: DsFieldType.boolean,
                ),
                DsFormField(
                  name: 'level',
                  label: 'Level',
                  type: DsFieldType.choice,
                  choices: ['low', 'high'],
                ),
              ],
            ),
          ],
        ),
        permitted: true,
        deniedReasonText: null,
        header: const DsComponentHeader(
          agentName: 'Robin',
          runLabel: 'R-12',
          requiredCapability: 'approve_actions',
          approver: DsApprover.runStarter,
          runStarterName: 'Sam',
        ),
        agentText: '',
        onOpenLink: (_) {},
        onActivate: (_, _) {},
      ),
    );
    final text = tester.getSemantics(find.byType(TextField));
    expect(text.label, contains('Reason'));
    expect(text.flagsCollection.isTextField, isTrue);
    final toggle = tester.getSemantics(find.byType(Switch));
    expect(toggle.label, contains('Urgent'));
    expect(
      find.semantics.byPredicate(
        (n) => n.label == 'Level' && n.childrenCount > 0,
      ),
      findsOneWidget,
    );
    handle.dispose();
  });
}
