// SPDX-License-Identifier: Apache-2.0

// The operations the fake learned for editing a built-in agent (brief A1-09 Scope; 26 3.1
// items 3 and 4, 4.1, 8.4; 10 10.3): the versions newest first without their text, an older
// version with `version`, a stale definition write as 412 with the current version,
// setAgentHost with If-Match and each refusal (host_tool_class_ceiling,
// hosted_holder_mismatch, host_not_capable, not_a_builtin_agent), and disableAgent with
// step-up. Every request and answer passes the contract check.

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

Map<String, Object?> _definition(String instructions) => {
  'instructions': instructions,
  'skills': <Object?>[],
  'mcp_servers': <Object?>[],
  'settings': {'tool_class_wish': 'chat'},
  'origin': {'kind': 'manual'},
};

void main() {
  late FakeWorldHarness h;
  late FakeWorld world;
  late TestDevice alex;
  late FakeAgent runner;
  late FakeAgent scopedRunner;
  late FakeAgent notes;

  setUp(() async {
    h = await FakeWorldHarness.start();
    world = h.server.world;
    alex = h.device();
    await signedIn(h, alex);
    runner = world.addAgent(
      handle: 'dev-runner',
      displayName: 'Dev runner',
      adapterKind: 'code_runner',
      hostsAgents: 'host',
    );
    scopedRunner = world.addAgent(
      handle: 'small-runner',
      displayName: 'Small runner',
      adapterKind: 'code_runner',
      hostsAgents: 'scoped',
    );
    notes = world.addAgent(
      handle: 'notes',
      displayName: 'Notes',
      adapterKind: 'claude_agent',
      hostedBy: runner.id,
    );
    notes.definitions
      ..add(_definition('First.'))
      ..add(_definition('Second.'));
    notes.version = 3;
  });
  tearDown(() async {
    expect(h.server.violations, isEmpty);
    await h.stop();
  });

  AgentsApi agents() => alex.api.getAgentsApi();

  Future<Map<String, Object?>?> problem(Future<Object?> call) async {
    try {
      await call;
      return null;
    } on DioException catch (e) {
      return Map<String, Object?>.from(e.response!.data as Map);
    }
  }

  Future<Agent> move(String to, {int? ifMatch}) async =>
      (await agents().setAgentHost(
        agentId: notes.id,
        ifMatch: '"${ifMatch ?? notes.version}"',
        hostSet: HostSet(
          (b) => b
            ..hostedBy = to
            ..reason = 'Move agent Notes',
        ),
      )).data!;

  test('the versions come newest first without their text', () async {
    final page = (await agents().listAgentDefinitionVersions(agentId: notes.id))
        .data!;
    expect([for (final v in page.items) v.version], [2, 1]);
    expect(page.items.every((v) => v.definition == null), isTrue);
  });

  test('an agent without a definition lists no version', () async {
    final empty = world.addAgent(
      handle: 'empty',
      displayName: 'Empty',
      adapterKind: 'claude_agent',
      hostedBy: runner.id,
    );
    final page = (await agents().listAgentDefinitionVersions(agentId: empty.id))
        .data!;
    expect(page.items, isEmpty);
  });

  test(
    'an older version is read with version, an unknown one is 404',
    () async {
      final old = (await agents().getAgentDefinition(
        agentId: notes.id,
        version: 1,
      )).data!;
      expect(old.version, 1);
      expect(old.definition!.instructions, 'First.');
      final p = await problem(
        agents().getAgentDefinition(agentId: notes.id, version: 9),
      );
      expect(p?['code'], 'not_found');
    },
  );

  test('a stale definition write is 412 with the current version', () async {
    final p = await problem(
      agents().putAgentDefinition(
        agentId: notes.id,
        ifMatch: '"1"',
        agentDefinition: standardSerializers.deserializeWith(
          AgentDefinition.serializer,
          _definition('Mine.'),
        )!,
      ),
    );
    expect(p?['code'], 'version_conflict');
    expect((p?['current'] as Map?)?['version'], 2);
    expect(notes.definitions.length, 2);
  });

  test('setAgentHost moves the agent with If-Match and an audit row', () async {
    final second = world.addAgent(
      handle: 'build-runner',
      displayName: 'Build runner',
      adapterKind: 'code_runner',
      hostsAgents: 'host',
    );
    final moved = await move(second.id);
    expect(moved.hostedBy, second.id);
    expect(moved.version, 4);
    expect(
      h.server.audit.rows.map((r) => r['action']),
      contains('agent.host.set'),
    );
    final stale = await problem(move(runner.id, ifMatch: 3));
    expect(stale?['code'], 'version_conflict');
  });

  test(
    'a runner below the class of the agent is host_tool_class_ceiling',
    () async {
      notes.toolClass = 'host';
      final p = await problem(move(scopedRunner.id));
      expect(p?['reason'], 'host_tool_class_ceiling');
      expect(notes.hostedBy, runner.id);
    },
  );

  test('a personal agent needs a runner of its holder', () async {
    notes
      ..billingMode = 'personal'
      ..credentialHolderId = h.seed.alex.id;
    final p = await problem(move(scopedRunner.id));
    expect(p?['reason'], 'hosted_holder_mismatch');
  });

  test('a runner that hosts no agents is host_not_capable', () async {
    final plain = world.addAgent(
      handle: 'plain-runner',
      displayName: 'Plain runner',
      adapterKind: 'code_runner',
    );
    final p = await problem(move(plain.id));
    expect(p?['code'], 'validation_failed');
    expect((p?['errors'] as List).single, {
      'pointer': '/hosted_by',
      'code': 'host_not_capable',
    });
  });

  test('another agent cannot move', () async {
    final p = await problem(
      agents().setAgentHost(
        agentId: runner.id,
        ifMatch: '"${runner.version}"',
        hostSet: HostSet(
          (b) => b
            ..hostedBy = scopedRunner.id
            ..reason = 'Move',
        ),
      ),
    );
    expect(p?['reason'], 'not_a_builtin_agent');
  });

  test('disableAgent disables the agent with its reason', () async {
    final disabled = (await agents().disableAgent(
      agentId: notes.id,
      reasonBody: ReasonBody((b) => b..reason = 'Not needed'),
    )).data!;
    expect(disabled.status, AgentStatusEnum.disabled);
    final row = h.server.audit.rows.lastWhere(
      (r) => r['action'] == 'agent.disable',
    );
    expect((row['details'] as Map?)?['reason'], 'Not needed');
  });
}
