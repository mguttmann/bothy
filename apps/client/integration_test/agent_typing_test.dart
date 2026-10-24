// SPDX-License-Identifier: Apache-2.0

// The agent's typing line follows the run state (brief M4-21 Tests; D-L9; 04 12 item 1):
// "Robin is working" while the run is `running`, not while it is queued or after its end;
// agents send no typing events.

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/testing.dart';

import 'support/agent_flow.dart';

final AppLocalizations en = lookupAppLocalizations(const Locale('en'));

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('the typing line follows the run state', (tester) async {
    final queued = Completer<void>();
    final finish = Completer<void>();
    late ScriptedAgent agent;
    final opened = await openAgentThread(
      tester,
      setup: (world, seed, forum, thread) {
        agent = ScriptedAgent(
          world,
          agentId: seed.robin.id,
          // The anchor names the agent to the app, as the server writes it with the run.
          anchors: true,
          script: AgentScript([
            const AgentStep.queue(),
            AgentStep.until(() => queued.future),
            const AgentStep.start(),
            AgentStep.until(() => finish.future),
            const AgentStep.finish(),
          ]),
        )..attach();
      },
    );
    final app = opened.app;
    final working = find.text(en.agentWorking(dsIsolate('Robin')));
    try {
      await sendText(tester, '@robin check the logs');
      await waitUntil(tester, 'a run', () => agent.runIds.isNotEmpty);
      final runId = agent.runIds.single;
      final world = app.fake.server.world;
      await waitUntil(
        tester,
        'queued',
        () => world.runs[runId]!['state'] == 'queued',
      );
      await tester.pump(const Duration(milliseconds: 100));
      expect(working, findsNothing, reason: 'a queued run does not work yet');

      queued.complete();
      await waitUntil(
        tester,
        'running',
        () => world.runs[runId]!['state'] == 'running',
      );
      await app.waitFor(working);
      // No typing event of the agent was ever sent (D-L9).
      expect(world.log.where((e) => e.type == 'typing.started'), isEmpty);

      finish.complete();
      await waitUntil(tester, 'the end', () => working.evaluate().isEmpty);
    } finally {
      agent.detach();
      await app.stop();
    }
  });
}
