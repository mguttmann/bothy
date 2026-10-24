// SPDX-License-Identifier: Apache-2.0

// A run with a follow-up through the interface (brief M4-21 Tests; 04 16.5; 01 6.2, 6.3): Alex
// mentions Robin, the trigger state goes received, queued, working; the live box shows the
// patches; a follow-up during the run reaches `followup_delivered`; the run completes and the
// box collapses into the line at its anchor. The agent waits at gates the test opens, so each
// state is on screen before the next one comes.

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/testing.dart';

import 'support/agent_flow.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('mention, live box, follow-up delivered, collapse at the end', (
    tester,
  ) async {
    final queued = Completer<void>();
    final started = Completer<void>();
    final finish = Completer<void>();
    late ScriptedAgent agent;
    final opened = await openAgentThread(
      tester,
      setup: (world, seed, forum, thread) {
        final stepId = world.nextId();
        agent = ScriptedAgent(
          world,
          agentId: seed.robin.id,
          anchors: true,
          script: AgentScript([
            AgentStep.until(() => queued.future),
            const AgentStep.queue(),
            AgentStep.until(() => started.future),
            const AgentStep.start(),
            const AgentStep.patch([
              {
                'op': 'add',
                'path': '/status_line',
                'value': 'Reading the thread',
              },
            ]),
            AgentStep.patch([
              {
                'op': 'add',
                'path': '/steps/$stepId',
                'value': {
                  'id': stepId,
                  'ordinal': 1,
                  'kind': 'tool_call',
                  'name': 'search',
                  'title': 'Search the notes',
                  'status': 'running',
                  'started_at': world.stamp(),
                },
              },
            ]),
            const AgentStep.awaitFollowup(),
            const AgentStep.deliverFollowup(),
            const AgentStep.absorbFollowup(),
            AgentStep.until(() => finish.future),
            const AgentStep.reply('Here is the summary.'),
            const AgentStep.finish(),
          ]),
        )..attach();
      },
    );
    final app = opened.app;
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final threadId = opened.thread;
    try {
      await sendText(tester, '@robin summarize the release notes');
      await app.waitFor(find.text('@robin summarize the release notes'));
      await waitUntil(tester, 'a run', () => agent.runIds.isNotEmpty);
      final trigger = newestOf(world, threadId, seed.alex.id);
      await app.waitFor(triggerOf(trigger, DsTriggerState.received));

      queued.complete();
      await app.waitFor(triggerOf(trigger, DsTriggerState.queued));
      // While the run is active the composer offers the follow-up form.
      await app.waitFor(
        find.byWidgetPredicate(
          (w) => w is DsComposer && w.followupAgentName == 'Robin',
        ),
      );

      started.complete();
      await app.waitFor(triggerOf(trigger, DsTriggerState.working));
      final runId = agent.runIds.single;
      final box = find.byKey(ValueKey('livebox.$runId'));
      await app.waitFor(box);
      await app.waitFor(
        find.descendant(of: box, matching: find.text('Reading the thread')),
      );
      await app.waitFor(
        find.descendant(of: box, matching: find.textContaining('search')),
      );

      await sendText(tester, 'Also mention the budget');
      String? followup;
      final until = DateTime.now().add(const Duration(seconds: 20));
      while (followup == null) {
        if (DateTime.now().isAfter(until)) fail('no follow-up on the server');
        final latest = newestOf(world, threadId, seed.alex.id);
        if (latest != trigger) followup = latest;
        await tester.pump(const Duration(milliseconds: 50));
      }
      await app.waitFor(triggerOf(followup, DsTriggerState.followupDelivered));

      finish.complete();
      await app.waitFor(find.text('Here is the summary.'));
      await app.waitFor(find.byKey(ValueKey('runline.$runId')));
      expect(box, findsNothing);
      await app.waitFor(triggerOf(trigger, DsTriggerState.done));
    } finally {
      agent.detach();
      await app.stop();
    }
  });
}
