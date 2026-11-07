// SPDX-License-Identifier: Apache-2.0

// Editing a built-in agent on the desktop against the fake server (brief A1-09 Tests; 01 6.20
// item 6; 26 3.1, 4.1, 8.4; BA-T16 edit part): Alex, an owner, opens the agent card of Notes in
// the administration; Edit adds a local MCP server and raises the class to Host, and the
// fake server sees agent.tool_class.set before putAgentDefinition; Versions restores version
// 1, which removes the server again, and the class goes back to Chat after the definition;
// Change runner moves Notes to another runner; Disable ends it. Every step-up is confirmed.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

import 'support/test_app.dart';

const Map<String, Object?> _plain = {
  'instructions': 'Keep the notes of Sam.',
  'skills': <Object?>[],
  'mcp_servers': <Object?>[],
  'settings': {'tool_class_wish': 'chat'},
  'origin': {'kind': 'manual'},
};

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('edit, restore, change runner and disable a built-in agent', (
    tester,
  ) async {
    final app = await TestApp.start(tester);
    try {
      final server = app.fake.server;
      final world = server.world;
      final runner = world.addAgent(
        handle: 'dev-runner',
        displayName: 'Dev runner',
        adapterKind: 'code_runner',
        hostsAgents: 'host',
      );
      final build = world.addAgent(
        handle: 'build-runner',
        displayName: 'Build runner',
        adapterKind: 'code_runner',
        hostsAgents: 'host',
      );
      final notes = world.addAgent(
        handle: 'notes',
        displayName: 'Notes',
        adapterKind: 'claude_agent',
        hostedBy: runner.id,
      );
      notes.definitions.add(Map.of(_plain));
      notes.version = 2;
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));

      void go(String path) =>
          GoRouter.of(tester.element(find.byType(Navigator).first)).go(path);

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

      Future<void> confirmStepUps() async {
        while (find
            .byKey(const ValueKey('stepUp.dialog'))
            .evaluate()
            .isNotEmpty) {
          await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
          await tester.pumpAndSettle();
        }
      }

      Future<void> until(String what, Finder gone) =>
          eventually(what, () async {
            await confirmStepUps();
            return gone.evaluate().isEmpty;
          });

      List<String> steps() => [
        for (final r in server.requests)
          if (const {
                'setAgentToolClass',
                'putAgentDefinition',
                'setAgentHost',
                'disableAgent',
              }.contains(r.operationId) &&
              r.status != 401)
            '${r.operationId} ${r.status}',
      ];

      final dialog = find.byKey(const ValueKey('newAgent.dialog'));

      go('/admin/agents/${notes.id}');
      await tap('builtinAgent.edit');
      await tap('newAgent.server.add');
      await type('newAgent.server.0.name', 'files');
      await tap('newAgent.server.0.transport.stdio');
      await type('newAgent.server.0.command', 'npx');
      await type('newAgent.server.0.package', '@example/mcp-files@1.4.2');
      await tap('newAgent.class.host');
      server.requests.clear();
      await tap('agentEdit.save');
      await until('the edit saved', dialog);
      expect(steps(), ['setAgentToolClass 200', 'putAgentDefinition 200']);
      expect(notes.toolClass, 'host');
      expect(notes.definitions, hasLength(2));

      await tap('builtinAgent.versions');
      await tap('agentVersions.restore.1');
      await tap('newAgent.class.chat');
      server.requests.clear();
      await tap('agentEdit.save');
      await until('the restore saved', dialog);
      expect(steps(), ['putAgentDefinition 200', 'setAgentToolClass 200']);
      expect(notes.definitions.last['mcp_servers'], isEmpty);
      expect(notes.toolClass, 'chat');

      await tap('builtinAgent.changeRunner');
      await tap('agentHost.runner.${build.id}');
      server.requests.clear();
      await tap('agentHost.confirm');
      await until(
        'the agent moved',
        find.byKey(const ValueKey('agentHost.dialog')),
      );
      expect(steps(), ['setAgentHost 200']);
      expect(notes.hostedBy, build.id);

      await tap('builtinAgent.disable');
      await tester.enterText(
        find.byKey(const ValueKey('confirm.reason')),
        'Not needed any more',
      );
      server.requests.clear();
      await tap('confirm.submit');
      await eventually('the agent disabled', () async {
        await confirmStepUps();
        return world.people[notes.id]!.status == 'disabled';
      });
      expect(steps(), ['disableAgent 200']);
      expect(server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  });
}
