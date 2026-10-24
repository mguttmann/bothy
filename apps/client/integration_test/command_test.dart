// SPDX-License-Identifier: Apache-2.0

// A command with option suggestions through the interface (brief M4-21 Tests; 01 D5; 09 10.4,
// 10.5; components.md 9): `/model` loads with the channel, its option asks Robin for
// suggestions after the debounce, the chosen suggestion fills the option and sending invokes
// the command through `invokeCommand`, not as a message.

import 'package:design_system/design_system.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/testing.dart';

import 'support/agent_flow.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('/model with suggestions from the agent, then invoked', (
    tester,
  ) async {
    late String model;
    final opened = await openAgentThread(
      tester,
      setup: (world, seed, forum, thread) {
        model =
            ScriptedAgent(
              world,
              agentId: seed.robin.id,
              script: const AgentScript([]),
            ).registerCommand(
              'model',
              description: 'Choose a model for this session',
              options: [
                {
                  'name': 'name',
                  'description': 'Model',
                  'type': 'string',
                  'required': true,
                  'autocomplete': true,
                },
              ],
              suggest: (option, value, _) => [
                for (final m in const ['model-large', 'model-fast'])
                  if (m.startsWith(value)) {'name': m, 'value': m},
              ],
            );
      },
    );
    final app = opened.app;
    final world = app.fake.server.world;
    final ui = FakeAgentInterface.of(world);
    try {
      await tester.enterText(composerText, '/mo');
      await app.waitFor(
        find.byWidgetPredicate(
          (w) =>
              w is DsSuggestionList &&
              w.suggestions.any((s) => s.command == '/model'),
        ),
      );
      await tester.enterText(composerText, '/model model-l');
      await app.waitFor(find.text('model-large'));
      expect(ui.autocompleteRequests.last.value, 'model-l');
      expect(find.text('model-fast'), findsNothing);
      // No request repeats while the input stays the same (the answer rebuilds the composer).
      final asked = ui.autocompleteRequests.length;
      final until = DateTime.now().add(const Duration(seconds: 1));
      while (DateTime.now().isBefore(until)) {
        await tester.pump(const Duration(milliseconds: 50));
      }
      expect(ui.autocompleteRequests.length, asked);
      await tester.tap(find.text('model-large'));
      await tester.pump();
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await waitUntil(
        tester,
        'the invocation',
        () => ui.invocations.isNotEmpty,
      );
      final invocation = ui.invocations.single;
      expect(invocation['command_id'], model);
      expect(invocation['options'], [
        {'name': 'name', 'value': 'model-large'},
      ]);
      expect(
        world
            .messagesOf(opened.thread)
            .where((m) => (m['content_md']! as String).startsWith('/model')),
        isEmpty,
      );
    } finally {
      await app.stop();
    }
  });
}
