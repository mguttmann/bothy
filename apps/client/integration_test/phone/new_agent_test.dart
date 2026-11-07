// SPDX-License-Identifier: Apache-2.0

// "New agent" on the phone against the fake server (brief A1-08 Tests; 01 6.20; 26 8.1, 8.2):
// Alex, an owner, holds a space row on the Spaces page, opens "New agent" from its menu, fills
// the empty form in the full-screen dialog and creates the agent on the runner that hosts
// agents; the step-up is confirmed with the device key; the dialog closes and the agent exists
// with its definition.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'support/phone_flow.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'the space menu opens New agent and the empty form creates the agent',
    (tester) async {
      await asPhone(() async {
        final app = await startPhone(tester);
        final server = app.fake.server;
        final world = server.world;
        final runner = world.addAgent(
          handle: 'dev-runner',
          displayName: 'Dev runner',
          adapterKind: 'code_runner',
          hostsAgents: 'host',
        );
        world.agentTemplates = [];

        Future<void> tap(String key) async {
          await app.waitFor(find.byKey(ValueKey(key)));
          await tester.ensureVisible(find.byKey(ValueKey(key)).first);
          await tester.pumpAndSettle();
          await app.tap(key);
        }

        Future<void> type(String key, String text) async {
          await tester.ensureVisible(find.byKey(ValueKey('$key.field')));
          await tester.enterText(find.byKey(ValueKey('$key.field')), text);
          await tester.pumpAndSettle();
        }

        try {
          await signIn(app);
          final row = find.byKey(
            ValueKey('sidebar.space.${app.fake.seed.space}'),
          );
          await app.waitFor(row);
          await tester.longPress(row);
          await tester.pumpAndSettle();
          await tap('menu.newAgent');
          await tap('newAgent.way.empty');
          await type('newAgent.name', 'Notes');
          await type('newAgent.handle', 'notes');
          await type('newAgent.instructions', 'Keep the notes of Robin.');
          // A step that waits for its step-up shows a spinner, so nothing settles after
          // Create: tap and pump instead of the settling tap.
          final create = find.byKey(const ValueKey('newAgent.create'));
          await app.waitFor(create);
          await tester.ensureVisible(create);
          await tester.pumpAndSettle();
          await tester.tap(create);
          await tester.pump();
          await eventually('the dialog closed', () async {
            if (find
                .byKey(const ValueKey('stepUp.dialog'))
                .evaluate()
                .isNotEmpty) {
              await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
              await tester.pump(const Duration(milliseconds: 300));
            }
            await tester.pump();
            return find
                .byKey(const ValueKey('newAgent.dialog'))
                .evaluate()
                .isEmpty;
          });
          final agent = world.agents.values.singleWhere(
            (a) => a.hostedBy == runner.id,
          );
          expect(
            agent.definitions.single['instructions'],
            'Keep the notes of Robin.',
          );
          expect(server.violations, isEmpty);
        } finally {
          await app.stop();
        }
      });
    },
  );
}
