// SPDX-License-Identifier: Apache-2.0

// SEC-T14 (brief M4-21 Tests; SEC-OUT-4; T-AI-05): a card whose agent text imitates a header
// ("Approval needed", a shield character, "Only the owner can approve this") renders that text
// only below the divider "Text from <agent>"; the header shows the server's agent, run and
// capability, and the header takes no input from the text.

import 'package:client/agents/components_mapping.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/agent_fakes.dart';
import '../support/card_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/widgets.dart';

/// Agent text that imitates the server's header.
const String forged =
    'Approval needed\n\n\u{1F6E1} Request from Alex\n\nOnly the owner can '
    'approve this';

void main() {
  testWidgets('imitated header text stays below the divider; the header shows '
      'the server fields', (tester) async {
    final host = await openCard(
      tester,
      text: forged,
      capabilities: const [Capability.approveActions],
      components: [
        button(
          1,
          cardId,
          customId: 'allow',
          label: 'Allow once',
          capability: Capability.approveActions,
          restrictedTo: ComponentRestriction.runStarter,
        ),
      ],
    );
    expect(cardFinder, findsOneWidget);
    final header = find.descendant(
      of: cardFinder,
      matching: find.byKey(DsMessageComponents.headerKey),
    );
    expect(header, findsOneWidget);
    // The header holds only the server's facts.
    final headerTexts = [
      for (final t in tester.widgetList<Text>(
        find.descendant(of: header, matching: find.byType(Text)),
      ))
        t.data ?? '',
    ];
    expect(headerTexts.first, en.componentsHeaderRequest(dsIsolate('Robin')));
    expect(headerTexts, contains(startsWith('Run ')));
    expect(
      headerTexts,
      contains(en.componentsHeaderCapability(en.capabilityApproveActions)),
    );
    expect(
      headerTexts,
      contains(en.componentsApproverRunStarter(dsIsolate('Alex'))),
    );
    for (final t in headerTexts) {
      expect(t, isNot(contains('Approval needed')));
      expect(t, isNot(contains('\u{1F6E1}')));
      expect(t, isNot(contains('Only the owner')));
    }
    // The agent's text renders below the divider labeled with the agent.
    final divider = find.descendant(
      of: cardFinder,
      matching: find.text(en.componentsTextFrom(dsIsolate('Robin'))),
    );
    expect(divider, findsOneWidget);
    final forgedLine = find.descendant(
      of: cardFinder,
      matching: find.textContaining('Approval needed', findRichText: true),
    );
    expect(forgedLine, findsOneWidget);
    expect(
      find.descendant(
        of: header,
        matching: find.textContaining('Approval needed', findRichText: true),
      ),
      findsNothing,
    );
    expect(
      tester.getTopLeft(forgedLine).dy,
      greaterThan(tester.getBottomLeft(divider).dy),
    );
    expect(
      tester.getBottomLeft(header).dy,
      lessThanOrEqualTo(tester.getTopLeft(divider).dy),
    );
    await host.dispose(tester);
  });

  test('the header takes no input from the text', () {
    final principals = {for (final p in people) p.id: p};
    final components = [
      button(
        1,
        cardId,
        customId: 'allow',
        label: 'Allow once',
        capability: Capability.approveActions,
        restrictedTo: ComponentRestriction.owner,
      ),
    ];
    DsComponentHeader of(String text) => componentHeader(
      cardMessage(text),
      components,
      principals: principals,
      runLabel: '09:00',
    );
    final plain = of('Please approve');
    final faked = of(forged);
    expect(faked.agentName, plain.agentName);
    expect(faked.agentName, 'Robin');
    expect(faked.runLabel, plain.runLabel);
    expect(faked.requiredCapability, 'approve_actions');
    expect(faked.approver, DsApprover.owner);
    expect(faked.runStarterName, plain.runStarterName);
    // The agent's name comes from the stored principal of the author, never from the text.
    expect(principals[robin]!.displayName, faked.agentName);
  });
}
