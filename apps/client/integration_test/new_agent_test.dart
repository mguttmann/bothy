// SPDX-License-Identifier: Apache-2.0

// "New agent" on the desktop against the fake server (brief A1-08 Tests; 01 6.20; 26 8.2, 8.3,
// BA-T16): Alex, an owner, opens the agent list with a runner that hosts agents, picks the
// template with a local MCP server, raises nothing by hand (the template wishes `host`) and
// creates the agent with a channel; the step-up is confirmed; the steps run in the order of 26
// 8.2 (class before definition) and the dialog opens the new channel. A second agent whose
// channel name is taken fails at the channel step, keeps the earlier steps, and "Try again"
// with another name finishes without a second agent.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

import 'support/test_app.dart';

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

  testWidgets(
    'a template agent with a channel, then a failed channel and Try again',
    (tester) async {
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
        world.agentTemplates = [_filesTemplate];
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

        List<String> steps() => [
          for (final r in server.requests)
            if (const {
                  'createAgent',
                  'setAgentToolClass',
                  'putAgentDefinition',
                  'createChannel',
                  'setChannelOperator',
                }.contains(r.operationId) &&
                r.status != 401)
              '${r.operationId} ${r.status}',
        ];

        go('/admin/agents');
        await tap('agents.new');
        await tap('newAgent.way.template');
        await tap('newAgent.template.file-helper');
        await tap('newAgent.channel.wanted');
        await app.waitFor(
          find.byKey(const ValueKey('newAgent.channel.name.field')),
        );
        await type('newAgent.channel.name', 'files');
        // The channel goes to the first space, which the preview preselects.
        server.requests.clear();
        await tap('newAgent.create');
        await eventually('every step done', () async {
          await confirmStepUps();
          return find
              .byKey(const ValueKey('newAgent.dialog'))
              .evaluate()
              .isEmpty;
        });
        expect(steps(), [
          'createAgent 201',
          'setAgentToolClass 200',
          'putAgentDefinition 200',
          'createChannel 201',
          'setChannelOperator 200',
        ]);
        final agent = world.agents.values.singleWhere(
          (a) => a.hostedBy == runner.id,
        );
        expect(agent.toolClass, 'host');
        expect(agent.definitions, hasLength(1));

        // The second agent: its channel name is taken, so the channel step fails.
        go('/admin/agents');
        await tap('agents.new');
        await tap('newAgent.way.template');
        await tap('newAgent.template.file-helper');
        await type('newAgent.handle', 'file-helper-2');
        await tap('newAgent.channel.wanted');
        await type('newAgent.channel.name', 'files');
        server.requests.clear();
        await tap('newAgent.create');
        await eventually('the channel step failed', () async {
          await confirmStepUps();
          return find
              .byKey(const ValueKey('newAgent.failure'))
              .evaluate()
              .isNotEmpty;
        });
        expect(
          world.agents.values.where((a) => a.hostedBy == runner.id),
          hasLength(2),
        );
        await type('newAgent.channel.name', 'files-2');
        server.requests.clear();
        await tap('newAgent.retry');
        await eventually('every step done', () async {
          await confirmStepUps();
          return find
              .byKey(const ValueKey('newAgent.dialog'))
              .evaluate()
              .isEmpty;
        });
        expect(steps(), ['createChannel 201', 'setChannelOperator 200']);
        expect(
          world.agents.values.where((a) => a.hostedBy == runner.id),
          hasLength(2),
        );
        expect(server.violations, isEmpty);
      } finally {
        await app.stop();
      }
    },
  );
}
