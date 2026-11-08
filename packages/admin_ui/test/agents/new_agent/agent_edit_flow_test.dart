// SPDX-License-Identifier: Apache-2.0

// Editing a built-in agent with the flow of "New agent" (brief A1-09 requirements 12, 13, 15
// and 16; 26 4.1, 8.3, 8.4; R-BA-06, R-BA-11; BA-T16 edit part): an edit saves with
// putDefinition and If-Match of the version it started from; a raised class is set before the
// definition and a lowered one after it; a lowered class while a stdio server stays is
// refused with definition_needs_host until the server is removed; a stale version is
// version_conflict and "Reload" starts again from the current version; the changed parts
// are named; "Restore" saves an older content as a new version; "Show" changes nothing;
// "Continue setup" resumes at the first missing step, class before definition.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter_test/flutter_test.dart';

import 'fake_new_agent_port.dart';

const Map<String, Object?> _plain = {
  'instructions': 'Keep the notes of Sam.',
  'skills': [
    {
      'name': 'summarize',
      'description': 'Summarizes a thread',
      'body': 'Write three lines.',
    },
  ],
  'mcp_servers': <Object?>[],
  'settings': {'tool_class_wish': 'scoped'},
  'origin': {'kind': 'manual'},
};

const Map<String, Object?> _withStdio = {
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
};

void main() {
  late FakeNewAgentPort port;
  var keys = 0;

  setUp(() {
    keys = 0;
    port = FakeNewAgentPort();
  });

  Future<NewAgentFlow> open(
    String id, {
    Map<String, Object?>? content,
    bool readOnly = false,
  }) async {
    final agent = await port.agent(id);
    final stored = await port.definition(id);
    port.calls.clear();
    return NewAgentFlow.forAgent(
      port: port,
      offer: standardOffer,
      newKey: () => 'key-${++keys}',
      agent: agent,
      runner: devRunner,
      stored: stored,
      content: content,
      readOnly: readOnly,
    );
  }

  group('edit', () {
    test('starts from the current version with the agent identity', () async {
      port.addAgent(
        'a',
        toolClass: AgentToolClass.scoped,
        definitions: [_plain],
      );
      final flow = await open('a');
      expect(flow.mode, NewAgentMode.edit);
      expect(flow.draft!.name, 'Notes');
      expect(flow.draft!.handle, 'notes');
      expect(flow.draft!.instructions, 'Keep the notes of Sam.');
      expect(flow.draft!.toolClass, AgentToolClass.scoped);
      expect(flow.draft!.definitionJson(), _plain);
      expect(flow.hasChanges, isFalse);
      expect(flow.changes, isEmpty);
      expect(flow.canEditIdentity, isFalse);
      expect(flow.canEditHost, isFalse);
    });

    test('saves with putDefinition, If-Match of the edited version and the edit kind', () async {
      port.addAgent(
        'a',
        toolClass: AgentToolClass.scoped,
        definitions: [_plain],
      );
      final flow = await open('a');
      flow.edit((d) => d.instructions = 'Keep the notes of Robin.');
      expect(flow.hasChanges, isTrue);
      expect(flow.changes.map((c) => c.kind), [
        DefinitionChangeKind.instructions,
      ]);
      expect(flow.steps, [NewAgentStep.storeDefinition]);
      await flow.run();
      expect(flow.finished, isTrue);
      expect(port.writes, ['putDefinition']);
      expect(port.putIfMatches, [1]);
      expect(port.changes, ['putDefinition edit']);
      expect(
        port.definitionsOf('a').last['instructions'],
        'Keep the notes of Robin.',
      );
      expect(port.audit, ['agent.update']);
    });

    test(
      'an added stdio server with class host: the class before the definition',
      () async {
        port.addAgent('a', definitions: [_plain]);
        final flow = await open('a');
        flow.edit((d) {
          d.servers.add(
            DraftMcpServer(
              name: 'files',
              transport: 'stdio',
              command: 'npx',
              package: '@example/mcp-files@1.4.2',
            ),
          );
          d.pickClass(AgentToolClass.host);
        });
        expect(flow.steps, [
          NewAgentStep.setClass,
          NewAgentStep.storeDefinition,
        ]);
        expect(flow.changes.map((c) => (c.kind, c.name)), [
          (DefinitionChangeKind.serverAdded, 'files'),
          (DefinitionChangeKind.setting, 'tool_class_wish'),
          (DefinitionChangeKind.toolClass, 'host'),
        ]);
        await flow.run();
        expect(flow.failure, isNull);
        expect(flow.finished, isTrue);
        expect(port.writes, ['setToolClass host', 'putDefinition']);
        expect(port.changes, ['setToolClass edit', 'putDefinition edit']);
        expect(port.classOf('a'), AgentToolClass.host);
        expect(port.definitionsOf('a').length, 2);
      },
    );

    test(
      'a removed stdio server with class chat: the definition before the class',
      () async {
        port.addAgent(
          'a',
          toolClass: AgentToolClass.host,
          definitions: [_withStdio],
        );
        final flow = await open('a');
        flow.edit((d) {
          d.servers.clear();
          d.pickClass(AgentToolClass.chat);
        });
        expect(flow.steps, [
          NewAgentStep.storeDefinition,
          NewAgentStep.setClass,
        ]);
        await flow.run();
        expect(flow.failure, isNull);
        expect(port.writes, ['putDefinition', 'setToolClass chat']);
        expect(port.classOf('a'), AgentToolClass.chat);
      },
    );

    test(
      'a lowered class with the stdio server kept: definition_needs_host, then '
      'the server removed and Try again',
      () async {
        port.addAgent(
          'a',
          toolClass: AgentToolClass.host,
          definitions: [_withStdio],
        );
        final flow = await open('a');
        flow.edit((d) => d.pickClass(AgentToolClass.chat));
        await flow.run();
        expect(flow.failedStep, NewAgentStep.setClass);
        expect(flow.failure?.reason, 'definition_needs_host');
        expect(port.classOf('a'), AgentToolClass.host);
        expect(flow.hasLocalServers, isTrue);
        flow.removeLocalServers();
        expect(flow.draft!.hasStdio, isFalse);
        port.calls.clear();
        await flow.run();
        expect(flow.failure, isNull);
        expect(flow.finished, isTrue);
        expect(port.writes, ['putDefinition', 'setToolClass chat']);
        expect(port.classOf('a'), AgentToolClass.chat);
      },
    );

    test(
      'another device saved meanwhile: version_conflict, Reload starts again',
      () async {
        port.addAgent(
          'a',
          toolClass: AgentToolClass.scoped,
          definitions: [_plain],
        );
        final flow = await open('a');
        flow.edit((d) => d.instructions = 'Mine.');
        port.changeBehind('a', {..._plain, 'instructions': 'Theirs.'});
        await flow.run();
        expect(flow.failedStep, NewAgentStep.storeDefinition);
        expect(flow.failure?.code, 'version_conflict');
        expect(port.putIfMatches, [1]);
        expect(port.definitionsOf('a').last['instructions'], 'Theirs.');
        await flow.reload();
        expect(flow.failure, isNull);
        expect(flow.draft!.instructions, 'Theirs.');
        expect(flow.hasChanges, isFalse);
        flow.edit((d) => d.instructions = 'Mine again.');
        await flow.run();
        expect(flow.finished, isTrue);
        expect(port.putIfMatches, [1, 2]);
      },
    );

    test('another device changed only the class: an edit that keeps the class leaves it as it is', () async {
      port.addAgent(
        'a',
        toolClass: AgentToolClass.scoped,
        definitions: [_plain],
      );
      final flow = await open('a');
      port.changeClassBehind('a', AgentToolClass.chat);
      flow.edit((d) => d.instructions = 'Keep the notes of Robin.');
      expect(flow.changes.map((c) => c.kind), [
        DefinitionChangeKind.instructions,
      ]);
      await flow.run();
      expect(flow.failure, isNull);
      expect(flow.finished, isTrue);
      expect(flow.steps, [NewAgentStep.storeDefinition]);
      expect(port.writes, ['putDefinition']);
      expect(port.classOf('a'), AgentToolClass.chat);
    });

    test('another device changed the class an edit changes too: version_conflict, Reload '
        'starts from its class', () async {
      port.addAgent(
        'a',
        toolClass: AgentToolClass.scoped,
        definitions: [_plain],
      );
      final flow = await open('a');
      port.changeClassBehind('a', AgentToolClass.chat);
      flow.edit((d) => d.pickClass(AgentToolClass.host));
      expect(flow.changes.map((c) => (c.kind, c.name)).last, (
        DefinitionChangeKind.toolClass,
        'host',
      ));
      await flow.run();
      expect(flow.failedStep, NewAgentStep.setClass);
      expect(flow.failure?.code, 'version_conflict');
      expect(port.writes, isEmpty);
      expect(port.classOf('a'), AgentToolClass.chat);
      await flow.reload();
      expect(flow.failure, isNull);
      expect(flow.draft!.toolClass, AgentToolClass.chat);
      expect(flow.hasChanges, isFalse);
    });

    test('names every changed part', () async {
      port.addAgent(
        'a',
        toolClass: AgentToolClass.scoped,
        definitions: [_plain],
      );
      final flow = await open('a');
      flow.edit((d) {
        d.skills.first.body = 'Write two lines.';
        d.skills.add(DraftSkill(name: 'translate', description: 'Translates'));
        d.servers.add(
          DraftMcpServer(name: 'docs', url: 'https://mcp.example.com'),
        );
        d.model = 'claude-sonnet-4-5';
        d.costCap = '2';
      });
      expect(flow.changes.map((c) => (c.kind, c.name)), [
        (DefinitionChangeKind.skillChanged, 'summarize'),
        (DefinitionChangeKind.skillAdded, 'translate'),
        (DefinitionChangeKind.serverAdded, 'docs'),
        (DefinitionChangeKind.setting, 'model'),
        (DefinitionChangeKind.setting, 'max_cost_usd_per_run'),
      ]);
      flow.edit((d) {
        d.skills.removeAt(0);
      });
      expect(
        flow.changes.map((c) => (c.kind, c.name)),
        contains((DefinitionChangeKind.skillRemoved, 'summarize')),
      );
    });

    test('billing and the runner stay as they are', () async {
      port.addAgent('a', definitions: [_plain]);
      final flow = await open('a');
      flow
        ..runner = alexRunner
        ..billing = 'personal';
      expect(flow.runner, devRunner);
      expect(flow.billing, 'organization');
    });
  });

  group('versions', () {
    test('Restore saves the older content as a new version', () async {
      port.addAgent(
        'a',
        toolClass: AgentToolClass.host,
        definitions: [_plain, _withStdio],
      );
      final old = await port.definitionVersion('a', 1);
      final flow = await open('a', content: old.json);
      expect(flow.mode, NewAgentMode.edit);
      expect(flow.hasChanges, isTrue);
      expect(flow.draft!.instructions, 'Keep the notes of Sam.');
      await flow.run();
      expect(flow.finished, isTrue);
      expect(port.putIfMatches, [2]);
      expect(port.definitionsOf('a').length, 3);
      expect(port.definitionsOf('a').last, _plain);
    });

    test(
      'Restore of a version with a stdio server under chat preselects host',
      () async {
        port.addAgent('a', definitions: [_withStdio, _plain]);
        final old = await port.definitionVersion('a', 1);
        final flow = await open('a', content: old.json);
        expect(flow.draft!.toolClass, AgentToolClass.host);
        expect(flow.steps.first, NewAgentStep.setClass);
        // The class is raised before the definition, as for an edit (26 8.4).
        await flow.run();
        expect(flow.failure, isNull);
        expect(flow.finished, isTrue);
        expect(port.writes, ['setToolClass host', 'putDefinition']);
        expect(port.definitionsOf('a').last, _withStdio);
      },
    );

    test('Show is read-only and saves nothing', () async {
      port.addAgent('a', definitions: [_plain, _withStdio]);
      final old = await port.definitionVersion('a', 1);
      final flow = await open('a', content: old.json, readOnly: true);
      expect(flow.mode, NewAgentMode.view);
      expect(flow.canEditContent, isFalse);
      flow.edit((d) => d.instructions = 'Changed.');
      expect(flow.draft!.instructions, 'Keep the notes of Sam.');
      await flow.run();
      expect(port.writes, isEmpty);
    });
  });

  group('Continue setup', () {
    test(
      'resumes at the first missing step, class before definition',
      () async {
        port.addAgent('a');
        final flow = await open('a');
        expect(flow.mode, NewAgentMode.setup);
        expect(flow.draft, isNull);
        flow.useTemplate(filesTemplate);
        // The agent keeps its identity; the template gives the content.
        expect(flow.draft!.name, 'Notes');
        expect(flow.draft!.handle, 'notes');
        expect(flow.steps, [
          NewAgentStep.createAgent,
          NewAgentStep.setClass,
          NewAgentStep.storeDefinition,
        ]);
        expect(flow.stateOf(NewAgentStep.createAgent), NewAgentStepState.done);
        await flow.run();
        expect(flow.failure, isNull);
        expect(flow.finished, isTrue);
        expect(port.writes, ['setToolClass host', 'putDefinition']);
        expect(port.changes, ['setToolClass setup', 'putDefinition setup']);
        expect(port.agentCount, 1);
      },
    );

    test('a class already set is not set again', () async {
      port.addAgent('a', toolClass: AgentToolClass.host);
      final flow = await open('a');
      flow.useTemplate(filesTemplate);
      expect(flow.stateOf(NewAgentStep.setClass), NewAgentStepState.done);
      await flow.run();
      expect(port.writes, ['putDefinition']);
    });
  });
}
