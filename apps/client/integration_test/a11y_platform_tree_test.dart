// SPDX-License-Identifier: Apache-2.0

// What VoiceOver and Narrator receive (brief M4-32 Design note 2; 01 8): the app runs against
// the fake server with a running agent, and the platform accessibility tree of the app window
// (macOS Accessibility, Windows UI Automation) holds every button and text field of the
// semantics tree with its name and a matching role, and the live box as a named container.
// The views: the thread view with sidebar, post list, history, live box and composer, and the
// settings. Where the system has no probe (Linux) the test reports itself as skipped.

import 'dart:async';
import 'dart:ui' as ui;

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
  // The live box animates while the run works, so the frames never settle.
  await tester.pump(const Duration(milliseconds: 100));
  await tester.pump(const Duration(milliseconds: 100));
  final watch = Stopwatch()..start();
  var expected = <ExpectedElement>[];
  var missing = expected;
  var native = <NativeElement>[];
  while (true) {
    native = await probe.read();
    expected = expectedElements(tester, containers: containers);
    expect(expected, isNotEmpty, reason: '$screen: no semantics');
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
    // The platform accessibility first, then the framework's semantics, so the engine gets the
    // whole tree from its first update. flutter_test's TestPlatformDispatcher does not pass
    // the framework's setSemanticsTreeEnabled to the engine, which would drop every update; the
    // test asks the engine itself, as the framework does in the app.
    await probe.enable();
    ui.PlatformDispatcher.instance.setSemanticsTreeEnabled(true);
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
      await sendText(tester, '@robin summarize the release notes');
      await app.waitFor(find.text('Reading the thread'), timeout: patience);
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
      // The engine's accessibility made the framework hold a semantics handle of its own; the
      // test releases it, as the platform does when the screen reader stops.
      tester.platformDispatcher.semanticsEnabledTestValue = false;
      semantics.dispose();
      await app.stop();
    }
  });
}
