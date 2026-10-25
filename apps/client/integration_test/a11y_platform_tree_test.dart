// SPDX-License-Identifier: Apache-2.0

// What VoiceOver and Narrator receive (brief M4-32 Design note 2; 01 8): the app runs against
// the fake server with a running agent, and the platform accessibility tree of the app window
// (macOS Accessibility, Windows UI Automation) holds every button and text field of the
// semantics tree with its name and a matching role, and the live box as a named container.
// The views: the thread view with sidebar, post list, history, live box and composer, and the
// settings. Where the system has no probe (Linux) the test reports itself as skipped.

import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/testing.dart';

import 'support/agent_flow.dart';
import 'support/platform_tree.dart';

/// The patience of the repository's test kits: the only upper bound of a wait.
const Duration patience = Duration(minutes: 3);

/// Waits until the platform tree of the app holds every element the semantics tree expects
/// of [screen]; fails with the missing ones and the platform tree.
Future<void> expectPlatformTree(
  WidgetTester tester,
  PlatformTreeProbe probe,
  String screen, {
  RegExp? containers,
}) async {
  await tester.pumpAndSettle();
  final expected = expectedElements(tester, containers: containers);
  expect(expected, isNotEmpty, reason: '$screen: no semantics');
  final watch = Stopwatch()..start();
  var missing = expected;
  var native = <NativeElement>[];
  while (true) {
    native = await probe.read();
    missing = [
      for (final e in expected)
        if (!native.any((n) => matchesPlatform(probe, e, n))) e,
    ];
    if (missing.isEmpty || watch.elapsed > patience) break;
    await tester.pump(const Duration(milliseconds: 200));
  }
  expect(
    missing,
    isEmpty,
    reason:
        '$screen: missing in the platform tree:\n${missing.join('\n')}\n'
        'platform tree:\n${native.map((n) => '${n.role} ${n.names}').join('\n')}',
  );
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('VoiceOver and Narrator receive names and roles of the views', (
    tester,
  ) async {
    final probe = PlatformTreeProbe.current();
    if (probe == null) {
      markTestSkipped('no platform accessibility probe on this system');
      return;
    }
    final semantics = tester.ensureSemantics();
    final finish = Completer<void>();
    final opened = await openAgentThread(
      tester,
      setup: (world, seed, forum, thread) {
        ScriptedAgent(
          world,
          agentId: seed.robin.id,
          anchors: true,
          script: AgentScript([
            const AgentStep.queue(),
            const AgentStep.start(),
            const AgentStep.patch([
              {
                'op': 'add',
                'path': '/status_line',
                'value': 'Reading the thread',
              },
            ]),
            AgentStep.until(() => finish.future),
            const AgentStep.reply('Done.'),
            const AgentStep.finish(),
          ]),
        ).attach();
      },
    );
    final app = opened.app;
    try {
      await probe.enable();
      await sendText(tester, '@robin summarize the release notes');
      await app.waitFor(find.text('Reading the thread'));
      await expectPlatformTree(
        tester,
        probe,
        'the thread view with the live box',
        containers: RegExp('^Run of '),
      );
      finish.complete();
      await app.openSettings('appearance');
      await expectPlatformTree(tester, probe, 'the settings');
    } finally {
      if (!finish.isCompleted) finish.complete();
      semantics.dispose();
      await app.stop();
    }
  });
}
