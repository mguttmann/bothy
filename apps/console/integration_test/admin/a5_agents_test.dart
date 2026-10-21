// SPDX-License-Identifier: Apache-2.0

// A5 in the browser (brief M6-04 Design note 4): switch the billing mode of support-agent
// through the guided flow of 10 11 (the grant and the switch in one request), then create a
// token of the agent whose secret is shown once with a copy action.

import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support.dart';

void main() {
  patrolTest('A5 the guided switch and a token shown once', ($) async {
    final seed = E2eSeed.read();
    final agent = seed.agent('supportAgent');
    await grantsAt($, 'space.${seed.space('support').id}');
    await wizardFor($, seed.sam);
    await tapKey($, 'level.trigger');
    await typeKey($, 'wizard.reason', 'Support work');
    await tapKey($, 'guided.offer');
    await typeKey($, 'confirm.reason', 'Costs to the organization');
    await tapKey($, 'confirm.submit');
    await waitFor($, byKey('guided.result'));
    expectStepUp();
    await tapKey($, 'guided.close');

    await openConsole($, '/agents/${agent.id}');
    await waitFor($, byKey('agentPage.badge'));
    expect(
      find.descendant(
        of: byKey('agentPage.badge'),
        matching: find.text('Organization'),
      ),
      findsOneWidget,
    );

    await tapKey($, 'tokens.create');
    await typeKey($, 'token.name', 'Deploy');
    await typeKey($, 'token.lastDay', dayFromToday(30));
    await typeKey(
      $,
      'token.dpop',
      'NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs',
    );
    await tapKey($, 'confirm.submit');
    await waitFor($, byKey('token.secret'));
    await tapKey($, 'token.copy');
    await tapKey($, 'token.secretClose');
    await waitGone($, byKey('token.secret'));
    // Shown once: nothing on the page shows the secret again.
    expect(byKey('token.secretDialog'), findsNothing);
    expectStepUp();
  });
}
