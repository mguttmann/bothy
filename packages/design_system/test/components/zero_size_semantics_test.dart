// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  testWidgets('the suggestion count reaches the semantics tree from the root', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    final l = await en();
    await pumpDs(
      tester,
      DsSuggestionList(
        const [
          DsSuggestion(id: '1', command: '/deploy'),
          DsSuggestion(id: '2', command: '/stop'),
        ],
        activeIndex: 0,
        onAccept: (_) {},
      ),
    );
    expect(find.semantics.byLabel(l.composerSuggestions(2)), findsOneWidget);
    handle.dispose();
  });

  testWidgets('the card title reaches the semantics tree from the root', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    await pumpDs(
      tester,
      DsMessageComponents(
        const DsComponentData(
          messageId: 'm1',
          title: 'Run the deploy?',
          items: [
            DsComponentItem(
              customId: 'allow',
              type: DsComponentType.button,
              label: 'Allow once',
              style: DsButtonStyle.primary,
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
        agentText: 'Deploy to staging?',
        onOpenLink: (_) {},
        onActivate: (_, _) {},
      ),
    );
    expect(find.semantics.byLabel('Run the deploy?'), findsOneWidget);
    handle.dispose();
  });
}
