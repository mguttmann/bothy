// SPDX-License-Identifier: Apache-2.0

// The run graph M12 and the control room M11 on the phone against the fake server (brief M5-14
// Tests; 01 5.3 M3, M11, M12, 5.4): a scripted run with a subagent grows live in M12 opened
// from the button "Graph" of the live box sheet; back goes to the thread (the edge swipe on
// iOS, system back on Android); the tab Running shows the run as a card, whose tap opens M12
// again. The agent waits at gates the test opens, so each state is on screen before the next.

import 'dart:async';

import 'package:client/phone/phone_navigation.dart';
import 'package:client/phone/phone_shell.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/testing.dart';

import 'support/phone_flow.dart';

/// Taps [finder] once the page or sheet that holds it has finished its transition: a tap on a
/// sheet or page that still slides in can land beside the target on a loaded machine.
Future<void> tapSettled(WidgetTester tester, Finder finder) async {
  await until(tester, 'shown: $finder', () => finder.evaluate().isNotEmpty);
  final route = ModalRoute.of(tester.element(finder));
  await until(
    tester,
    'the transition below $finder ended',
    () =>
        route?.animation == null ||
        route!.animation!.status == AnimationStatus.completed,
  );
  await tester.tap(finder);
  await tester.pump();
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('a run with a subagent grows live in M12 from the sheet and '
      'from a card', (tester) async {
    await asPhone(() async {
      final app = await startPhone(tester);
      final world = app.fake.server.world;
      final seed = app.fake.seed;
      final forum = world.addForum(seed.space, 'deploys', {
        seed.alex.id: 'trigger',
        seed.robin.id: 'write',
      });
      final thread = world.addThread(
        forum,
        'Release checklist',
        createdBy: seed.alex.id,
      );
      world
        ..resolveMentions = true
        ..setChannelOperator(forum, seed.robin.id);
      final started = Completer<void>();
      final grow = Completer<void>();
      final finish = Completer<void>();
      final subagent = world.nextId();
      final inner = world.nextId();
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
      final agent = ScriptedAgent(
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
              'path': '/steps/$subagent',
              'value': step(subagent, 1, 'subagent', 'tests'),
            },
          ]),
          AgentStep.until(() => grow.future),
          AgentStep.patch([
            {
              'op': 'add',
              'path': '/steps/$inner',
              'value': step(inner, 2, 'tool_call', 'Bash', parent: subagent),
            },
          ]),
          AgentStep.until(() => finish.future),
          const AgentStep.finish(),
        ]),
      )..attach();
      PhoneShellState shell() =>
          tester.state<PhoneShellState>(find.byType(PhoneShell));
      try {
        await signIn(app);
        await app.waitFor(find.byKey(ValueKey('sidebar.channel.$forum')));
        await tapSettled(
          tester,
          find.byKey(ValueKey('sidebar.channel.$forum')),
        );
        await app.waitFor(find.byKey(ValueKey('postList.post.$thread')));
        await tapSettled(tester, find.byKey(ValueKey('postList.post.$thread')));
        await app.waitFor(find.byKey(ValueKey('history.$thread')));
        await sendText(tester, '@robin run the release checks');
        await until(tester, 'a run', () => agent.runIds.isNotEmpty);
        started.complete();
        final runId = agent.runIds.single;

        // M3: the compact row opens the full box as a sheet, whose "Graph" opens M12 on top
        // of the thread.
        await app.waitFor(find.byKey(ValueKey('livebox.line.$runId')));
        await tapSettled(tester, find.byKey(ValueKey('livebox.line.$runId')));
        await app.waitFor(find.byKey(const ValueKey('livebox.graph')));
        await tapSettled(tester, find.byKey(const ValueKey('livebox.graph')));
        await app.waitFor(find.byKey(ValueKey('runGraph.row.$subagent')));
        expect(shell().pages.map((p) => p.kind), [
          PhonePageKind.spaces,
          PhonePageKind.channel,
          PhonePageKind.thread,
          PhonePageKind.runGraph,
        ]);

        // A patch grows the graph live.
        grow.complete();
        await app.waitFor(find.byKey(ValueKey('runGraph.row.$inner')));

        // Back goes to the thread.
        await goBack(tester);
        await until(
          tester,
          'the thread on top',
          () => shell().pages.last.kind == PhonePageKind.thread,
        );

        // The tab bar shows on a tab's first page: back to Spaces, then the tab Running.
        while (shell().pages.length > 1) {
          final depth = shell().pages.length;
          await goBack(tester);
          await until(
            tester,
            'one page less',
            () => shell().pages.length < depth,
          );
        }

        // M11: the tab Running shows the run as a card; a tap opens M12 on Running.
        await app.waitFor(find.byKey(const ValueKey('phone.tab.running')));
        await tapSettled(
          tester,
          find.byKey(const ValueKey('phone.tab.running')),
        );
        await app.waitFor(find.byKey(ValueKey('running.card.$runId')));
        await tapSettled(tester, find.byKey(ValueKey('runCard.body.$runId')));
        await app.waitFor(find.byKey(ValueKey('runGraph.row.$inner')));
        expect(shell().tab, PhoneTab.running);
        expect(shell().pages.map((p) => p.kind), [
          PhonePageKind.running,
          PhonePageKind.runGraph,
        ]);

        finish.complete();
        await until(
          tester,
          'the run completed',
          () => world.runs[runId]!['state'] == 'completed',
        );
        expect(app.fake.server.violations, isEmpty);
      } finally {
        agent.detach();
        await app.stop();
      }
    });
  });
}
