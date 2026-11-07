// SPDX-License-Identifier: Apache-2.0

// The built-in agent operations the fake learned for "New agent" (brief A1-08 Scope; 26 3.1,
// 4.3 item 7, 4.4 item 5, 8.1, 8.2): `hosts_agents` for owners only, `createAgent` with
// `hosted_by`, the class and definition rules that fix the step order (`tool_class_too_low`,
// `definition_needs_host`, `host_tool_class_ceiling`), templates, the interview and the channel
// steps. Every request and answer passes the contract check.

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

const Map<String, Object?> _stdioDefinition = {
  'instructions': 'Keep notes for Alex.',
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
  'origin': {'kind': 'manual'},
};

void main() {
  late FakeWorldHarness h;
  late FakeWorld world;
  late WorldSeed w;
  late TestDevice alex;
  late FakeAgent runner;

  setUp(() async {
    h = await FakeWorldHarness.start();
    world = h.server.world;
    w = h.seed;
    alex = h.device();
    await signedIn(h, alex);
    runner = world.addAgent(
      handle: 'dev-runner',
      displayName: 'Dev runner',
      adapterKind: 'code_runner',
      hostsAgents: 'host',
    );
  });
  tearDown(() async {
    expect(h.server.violations, isEmpty);
    await h.stop();
  });

  Future<Map<String, Object?>?> problem(Future<Object?> call) async {
    try {
      await call;
      return null;
    } on DioException catch (e) {
      return Map<String, Object?>.from(e.response!.data as Map);
    }
  }

  AgentsApi agents([TestDevice? d]) => (d ?? alex).api.getAgentsApi();

  Future<Agent> create(String handle, {String? hostedBy}) async =>
      (await agents().createAgent(
        idempotencyKey: world.nextId(),
        agentCreate: AgentCreate(
          (b) => b
            ..handle = handle
            ..displayName = 'Notes'
            ..adapterKind = 'claude_agent'
            ..hostedBy = hostedBy ?? runner.id,
        ),
      )).data!;

  Future<Agent> setClass(String id, int version, ToolClass toolClass) async =>
      (await agents().setAgentToolClass(
        agentId: id,
        ifMatch: '"$version"',
        toolClassSet: ToolClassSet(
          (b) => b
            ..toolClass = toolClass
            ..reason = 'New agent',
        ),
      )).data!;

  Future<AgentDefinitionVersion> put(
    String id,
    Map<String, Object?> definition, {
    int? ifMatch,
  }) async => (await agents().putAgentDefinition(
    agentId: id,
    ifMatch: ifMatch == null ? null : '"$ifMatch"',
    agentDefinition: standardSerializers.deserializeWith(
      AgentDefinition.serializer,
      definition,
    )!,
  )).data!;

  Future<int> versionOf(String id) async =>
      (await agents().getAgent(agentId: id)).data!.version;

  test('hosts_agents and hosted_by show to owners only', () async {
    final created = await create('notes');
    final list = (await agents().listAgents()).data!.items;
    final card = list.firstWhere((a) => a.id == runner.id);
    expect(card.presence.hostsAgents, ToolClass.host);
    expect(created.hostedBy, runner.id);

    final sam = h.device(login: WorldSeed.samLogin, keySeed: 2);
    await signedIn(h, sam);
    final seen = (await agents(sam).getAgent(agentId: runner.id)).data!;
    expect(seen.presence.hostsAgents, isNull);
  });

  test(
    'a stdio definition needs host first, and host stays while it is stored',
    () async {
      final a = await create('notes');
      final refused = await problem(put(a.id, _stdioDefinition));
      expect(refused!['code'], 'validation_failed');
      expect((refused['errors']! as List).single, {
        'pointer': '/mcp_servers/0/transport',
        'code': 'tool_class_too_low',
      });
      expect(world.agents[a.id]!.definitions, isEmpty);

      final raised = await setClass(a.id, a.version, ToolClass.host);
      expect(raised.toolClass, ToolClass.host);
      expect((await put(a.id, _stdioDefinition)).version, 1);

      final down = await problem(
        setClass(a.id, await versionOf(a.id), ToolClass.chat),
      );
      expect(down!['reason'], 'definition_needs_host');

      // Without the server the class may go down after the definition (26 8.4).
      final plain = {..._stdioDefinition, 'mcp_servers': <Object?>[]};
      expect((await put(a.id, plain, ifMatch: 1)).version, 2);
      final lowered = await setClass(
        a.id,
        await versionOf(a.id),
        ToolClass.chat,
      );
      expect(lowered.toolClass, ToolClass.chat);
      expect(
        [
          for (final r in h.server.audit.rows)
            if (r['resource_id'] == a.id) r['action'],
        ],
        [
          'agent.create',
          'agent.tool_class.set',
          'agent.update',
          'agent.update',
          'agent.tool_class.set',
        ],
      );
    },
  );

  test('a class above the runner ceiling is host_tool_class_ceiling', () async {
    runner.hostsAgents = 'scoped';
    final a = await create('notes');
    final refused = await problem(setClass(a.id, a.version, ToolClass.host));
    expect(refused!['reason'], 'host_tool_class_ceiling');
  });

  test('hosted_by must name a runner that hosts agents', () async {
    final plain = world.addAgent(handle: 'helper', displayName: 'Helper');
    final refused = await problem(create('notes', hostedBy: plain.id));
    expect((refused!['errors']! as List).single, {
      'pointer': '/hosted_by',
      'code': 'host_not_capable',
    });
  });

  test(
    'templates answer 404 on an older server; the interview falls back',
    () async {
      world.agentTemplates = null;
      final missing = await problem(agents().listAgentTemplates());
      expect(missing!['code'], 'not_found');

      final answer = (await agents().interviewAgent(
        agentInterviewRequest: AgentInterviewRequest(
          (b) => b.turns.add(
            AgentInterviewTurn(
              (t) => t
                ..role = AgentInterviewTurnRoleEnum.owner
                ..text = 'Notes of our meetings',
            ),
          ),
        ),
      )).data!;
      expect(answer.outcome, AgentInterviewAnswerOutcomeEnum.fallback);
      expect(
        answer.fallbackReason,
        AgentInterviewAnswerFallbackReasonEnum.builderNotConfigured,
      );
      expect(world.interviewRequests.single['turns'], hasLength(1));
    },
  );

  test('a channel is created and gets the new agent as its operator', () async {
    final a = await create('notes');
    final channels = alex.api.getChannelsApi();
    final channel = (await channels.createChannel(
      spaceId: w.space,
      idempotencyKey: world.nextId(),
      channelCreate: ChannelCreate(
        (b) => b
          ..kind = ChannelCreateKindEnum.text
          ..name = 'notes',
      ),
    )).data!;
    await channels.setChannelOperator(
      channelId: channel.id,
      ifMatch: '"${channel.version}"',
      operatorSet: OperatorSet(
        (b) => b
          ..mode = OperatorMode.agent
          ..agentId = a.id
          ..reason = 'New agent',
      ),
    );
    expect(
      (world.channels[channel.id]!.json['operator']! as Map)['agent_id'],
      a.id,
    );
  });
}
