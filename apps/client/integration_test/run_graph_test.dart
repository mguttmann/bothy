// SPDX-License-Identifier: Apache-2.0

// The run graph and the control room against the fake server (brief M4-39 Tests; 01 D10, D14;
// 09 5.6; 04 6.2): a scripted run with a subagent and a background job grows live in the graph
// opened from the live box; the stall notice appears after the scripted `add
// /stall_warned_at`; a card of Running now opens the graph. The agent waits at gates the test
// opens, so each state is on screen before the next one comes.

import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/testing.dart';

import 'support/agent_flow.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('the graph grows live, shows the stall and opens from a card', (
    tester,
  ) async {
    final started = Completer<void>();
    final grow = Completer<void>();
    final finish = Completer<void>();
    late ScriptedAgent agent;
    late String call;
    late String subagent;
    late String job;
    late String inner;
    final opened = await openAgentThread(
      tester,
      setup: (world, seed, forum, thread) {
        call = world.nextId();
        subagent = world.nextId();
        job = world.nextId();
        inner = world.nextId();
        Map<String, Object?> step(
          String id,
          int ordinal,
          String kind,
          String name, {
          String? parent,
        }) => {
          'id': id,
          'ordinal': ordinal,
          'parent_step_id': ?parent,
          'kind': kind,
          'name': name,
          'title': '',
          'status': 'running',
          'started_at': world.stamp(),
        };
        agent = ScriptedAgent(
          world,
          agentId: seed.robin.id,
          anchors: true,
          script: AgentScript([
            const AgentStep.queue(),
            AgentStep.until(() => started.future),
            const AgentStep.start(),
            AgentStep.patch([
              {
                'op': 'add',
                'path': '/steps/$call',
                'value': step(call, 1, 'tool_call', 'Task'),
              },
              {
                'op': 'add',
                'path': '/steps/$subagent',
                'value': step(subagent, 2, 'subagent', 'tests', parent: call),
              },
              {
                'op': 'add',
                'path': '/steps/$job',
                'value': step(job, 3, 'background_job', 'npm test'),
              },
            ]),
            AgentStep.until(() => grow.future),
            AgentStep.patch([
              {
                'op': 'add',
                'path': '/steps/$inner',
                'value': step(inner, 4, 'tool_call', 'Bash', parent: subagent),
              },
            ]),
            AgentStep.until(() => finish.future),
            const AgentStep.finish(),
          ]),
        )..attach();
      },
    );
    final app = opened.app;
    final world = app.fake.server.world;
    final en = lookupAppLocalizations(const Locale('en'));
    try {
      await sendText(tester, '@robin run the release checks');
      await waitUntil(tester, 'a run', () => agent.runIds.isNotEmpty);
      started.complete();
      final runId = agent.runIds.single;
      await app.waitFor(find.byKey(ValueKey('livebox.$runId')));

      // The live box opens the graph; the merge rule draws the call as its subagent.
      await app.waitFor(find.byKey(const ValueKey('livebox.graph')));
      await tester.tap(find.byKey(const ValueKey('livebox.graph')));
      await app.waitFor(find.byKey(ValueKey('runGraph.row.$subagent')));
      await app.waitFor(find.byKey(ValueKey('runGraph.row.$job')));
      expect(find.byKey(ValueKey('runGraph.row.$call')), findsNothing);

      // A patch grows the graph live.
      grow.complete();
      await app.waitFor(find.byKey(ValueKey('runGraph.row.$inner')));

      // The stall marking of the server reaches the open graph.
      world.patchRun(runId, [
        {'op': 'add', 'path': '/stall_warned_at', 'value': world.stamp()},
      ]);
      await app.waitFor(find.textContaining(en.liveBoxStall(0)));

      // Running now shows the run as a card, which opens the graph again.
      await tester.tap(find.byKey(const ValueKey('runGraph.back')));
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey('sidebar.runningNow')));
      await app.waitFor(find.byKey(ValueKey('running.card.$runId')));
      await tester.tap(find.byKey(ValueKey('runCard.graph.$runId')));
      await app.waitFor(find.byKey(ValueKey('runGraph.row.$inner')));

      finish.complete();
      await waitUntil(
        tester,
        'the run completed',
        () => world.runs[runId]!['state'] == 'completed',
      );
    } finally {
      agent.detach();
      await app.stop();
    }
  });
}
