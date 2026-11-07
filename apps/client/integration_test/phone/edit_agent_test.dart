// SPDX-License-Identifier: Apache-2.0

// Editing built-in agents on the phone against the fake server (brief A1-09 Tests; 01 6.20 item
// 6; 26 8.3, 8.4; docs/decisions.d/A1-09.md): Alex, an owner, opens "Built-in agents" under
// Me; Notes is edited in the full-screen dialog and saved as a new version; Files has no
// definition, and "Continue setup" from a template runs the class step, then the definition,
// without a second agent. The step-up is confirmed with the device key.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'support/phone_flow.dart';

const Map<String, Object?> _plain = {
  'instructions': 'Keep the notes of Sam.',
  'skills': <Object?>[],
  'mcp_servers': <Object?>[],
  'settings': {'tool_class_wish': 'chat'},
  'origin': {'kind': 'manual'},
};

final Map<String, Object?> _filesTemplate = {
  'slug': 'file-helper',
  'source': 'shipped',
  'name': 'File helper',
  'description': 'Reads and sorts the files of a folder',
  'version': '1.0.0',
  'required_secrets': <String>[],
  'definition': {
    'instructions': 'Sort the files Alex names.',
    'skills': <Object?>[],
    'mcp_servers': [
      {
        'name': 'files',
        'transport': 'stdio',
        'command': 'npx',
        'package': '@example/mcp-files@1.4.2',
      },
    ],
    'settings': {'tool_class_wish': 'host'},
    'origin': {
      'kind': 'template',
      'template_slug': 'file-helper',
      'template_version': '1.0.0',
    },
  },
};

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('Me opens Built-in agents, an edit saves and a setup continues', (
    tester,
  ) async {
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
      world.agentTemplates = [_filesTemplate];
      final notes = world.addAgent(
        handle: 'notes',
        displayName: 'Notes',
        adapterKind: 'claude_agent',
        hostedBy: runner.id,
      );
      notes.definitions.add(Map.of(_plain));
      notes.version = 2;
      final files = world.addAgent(
        handle: 'files',
        displayName: 'Files',
        adapterKind: 'claude_agent',
        hostedBy: runner.id,
      );

      Future<void> tap(String key, {Finder? within}) async {
        final f = within == null
            ? find.byKey(ValueKey(key))
            : find.descendant(of: within, matching: find.byKey(ValueKey(key)));
        await app.waitFor(f);
        await tester.ensureVisible(f.first);
        await tester.pumpAndSettle();
        await tester.tap(f.first);
        await tester.pump();
      }

      Future<void> done(String what) => eventually(what, () async {
        if (find.byKey(const ValueKey('stepUp.dialog')).evaluate().isNotEmpty) {
          await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
          await tester.pump(const Duration(milliseconds: 300));
        }
        await tester.pump();
        return find.byKey(const ValueKey('newAgent.dialog')).evaluate().isEmpty;
      });

      List<String> steps() => [
        for (final r in server.requests)
          if (const {
                'createAgent',
                'setAgentToolClass',
                'putAgentDefinition',
              }.contains(r.operationId) &&
              r.status != 401)
            '${r.operationId} ${r.status}',
      ];

      try {
        await signIn(app);
        await tap('phone.tab.me');
        await tap('phone.me.builtinAgents');
        final notesCard = find.byKey(ValueKey('builtinAgents.${notes.id}'));
        await tap('builtinAgent.edit', within: notesCard);
        await tester.pumpAndSettle();
        final instructions = find.byKey(
          const ValueKey('newAgent.instructions.field'),
        );
        await tester.ensureVisible(instructions);
        await tester.enterText(instructions, 'Keep the notes of Robin.');
        await tester.pumpAndSettle();
        server.requests.clear();
        await tap('agentEdit.save');
        await done('the edit saved');
        expect(steps(), ['putAgentDefinition 200']);
        expect(
          notes.definitions.last['instructions'],
          'Keep the notes of Robin.',
        );

        final filesCard = find.byKey(ValueKey('builtinAgents.${files.id}'));
        await tap('builtinAgent.continueSetup', within: filesCard);
        await tester.pumpAndSettle();
        await tap('newAgent.way.template');
        await tester.pumpAndSettle();
        await tap('newAgent.template.file-helper');
        await tester.pumpAndSettle();
        server.requests.clear();
        await tap('agentEdit.continue');
        await done('the setup finished');
        expect(steps(), ['setAgentToolClass 200', 'putAgentDefinition 200']);
        expect(files.definitions, hasLength(1));
        expect(files.toolClass, 'host');
        expect(server.violations, isEmpty);
      } finally {
        await app.stop();
      }
    });
  });
}
