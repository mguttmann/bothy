// SPDX-License-Identifier: Apache-2.0

// An approval card through the interface (brief M4-21 Tests; 01 6.5, D4; 09 11.2, 11.3;
// components.md 8): the agent posts an approval card, Alex approves once and the card shows
// the choice; a card of a run another person started is disabled for Alex with the reason.

import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/testing.dart';

import 'support/agent_flow.dart';

final AppLocalizations en = lookupAppLocalizations(const Locale('en'));

List<JsonMap> approval() => [
  {
    'custom_id': 'once',
    'type': 'button',
    'definition': {'label': 'Allow once', 'value': 'once', 'style': 'primary'},
    'restricted_to': 'run_starter',
  },
  {
    'custom_id': 'deny',
    'type': 'button',
    'definition': {'label': 'Deny', 'value': 'deny', 'style': 'danger'},
    'restricted_to': 'run_starter',
  },
];

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('Alex approves once; a card of Sam\'s run is disabled with the '
      'reason', (tester) async {
    late ScriptedAgent agent;
    final opened = await openAgentThread(
      tester,
      setup: (world, seed, forum, thread) {
        // An owner meets every restriction (09 11.2); here Alex is a member.
        seed.alex.isOwner = false;
        agent = ScriptedAgent(
          world,
          agentId: seed.robin.id,
          script: AgentScript([
            const AgentStep.queue(),
            const AgentStep.start(),
            AgentStep.card('go test wants to use the network.', approval()),
            const AgentStep.awaitInteraction(),
            const AgentStep.reply('Tests ran.'),
            const AgentStep.finish(),
          ]),
        )..attach();
      },
    );
    final app = opened.app;
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    try {
      await sendText(tester, '@robin run the tests');
      await waitUntil(tester, 'a run', () => agent.runIds.isNotEmpty);
      final runId = agent.runIds.single;
      await waitUntil(tester, 'the card', () => agent.cards[runId] != null);
      final card = agent.cards[runId]!.single;
      final allow = find.descendant(
        of: find.byKey(ValueKey('card.$card')),
        matching: find.byKey(const ValueKey('ds-component-once')),
      );
      await app.waitFor(allow);
      await waitUntil(
        tester,
        'the button enabled',
        () => tester.widget<DsButton>(allow).onPressed != null,
      );
      await tester.tap(allow);
      await tester.tap(allow);
      await app.waitFor(
        find.textContaining("chose 'Allow once'", findRichText: true),
      );
      await app.waitFor(find.text('Tests ran.'));
      final interactions = FakeAgentInterface.of(world).interactions.values;
      expect(interactions, hasLength(1), reason: 'a double click is one');
      expect(interactions.single['principal_id'], seed.alex.id);

      // Sam starts a run; its card is restricted to Sam (or an owner).
      final trigger = seed.post(seed.sam, opened.thread, 'Deploy please');
      final samRun = world.createRun(
        agentId: seed.robin.id,
        triggeringMessageId: trigger['id']! as String,
        triggeredBy: seed.sam.id,
      );
      final samCard = FakeAgentInterface.of(world).postCard(
        seed.robin.id,
        opened.thread,
        'Deploy to production?',
        approval(),
        runId: samRun['id']! as String,
      );
      final locked = find.descendant(
        of: find.byKey(ValueKey('card.$samCard')),
        matching: find.text(en.componentsApproverRunStarter(dsIsolate('Sam'))),
      );
      await app.waitFor(locked);
      final samAllow = find.descendant(
        of: find.byKey(ValueKey('card.$samCard')),
        matching: find.byKey(const ValueKey('ds-component-once')),
      );
      expect(tester.widget<DsButton>(samAllow).onPressed, isNull);
    } finally {
      agent.detach();
      await app.stop();
    }
  });
}
