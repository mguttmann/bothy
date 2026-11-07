// SPDX-License-Identifier: Apache-2.0

// BA-T16 of the app against the Dart fake server (brief A1-08 Tests; 26 8.1 to 8.3): the offer
// follows the server (hidden on an older server, without a hosting runner and for a person who
// is no owner); a template with a stdio server and class host is stored with
// agent.tool_class.set before putAgentDefinition; a definition refused with tool_class_too_low,
// the class raised, "Try again": class, then definition, one agent; a stdio definition stored
// under host, a later step failing, the server removed and chat: putAgentDefinition before
// agent.tool_class.set without definition_needs_host; a failure leaves exactly the earlier
// audit entries. The fake refuses like the real server; every step-up is confirmed.

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/new_agent/new_agent_entry.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/testing.dart';

import '../admin/admin_step_up_test.dart' show RecordingPrompt;
import '../support/fake_instance.dart';

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
  HttpOverrides? saved;
  late FakeInstance fake;
  late ProviderContainer c;
  late RecordingPrompt prompt;
  late FakeAgent runner;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    prompt = RecordingPrompt();
    fake = await FakeInstance.start(stepUpPrompt: prompt);
    c = appContainer(fake);
    final world = fake.server.world;
    runner = world.addAgent(
      handle: 'dev-runner',
      displayName: 'Dev runner',
      adapterKind: 'code_runner',
      hostsAgents: 'host',
    );
    world.agentTemplates = [_filesTemplate];
  });

  tearDown(() async {
    expect(fake.server.violations, isEmpty);
    c.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  Future<void> signIn({String login = WorldSeed.alexLogin}) async {
    final control = await connected(c, fake);
    await control.loginWithPassword(login, WorldSeed.password, WorldSeed.totp);
    expect(c.read(onboardingControllerProvider).step, OnboardingStep.signedIn);
  }

  NewAgentPort port() => ApiNewAgentPort(
    c.read(signedInServicesProvider)!.auth.api,
    purpose: (name) => 'Create the agent $name',
    reason: (name) => 'New agent $name',
  );

  Future<NewAgentFlow> flow() async {
    final p = port();
    final offer = (await p.offer())!;
    return NewAgentFlow(
      port: p,
      offer: offer,
      newKey: () => fake.server.world.nextId(),
      context: await p.context(),
    )..useTemplate(offer.templates.single);
  }

  List<String> ops() => [
    for (final r in fake.server.requests)
      if (const {
        'createAgent',
        'setAgentToolClass',
        'putAgentDefinition',
        'setAgentBillingMode',
        'createChannel',
        'setChannelOperator',
      }.contains(r.operationId))
        // A 401 is the step-up round trip of the same request (07 SR5), not a step.
        if (r.status != 401) '${r.operationId} ${r.status}',
  ];

  List<Object?> rows() => [
    for (final r in fake.server.audit.rows)
      if (r['actor_principal_id'] == fake.seed.alex.id) r['action'],
  ];

  group('the offer', () {
    test('is there for an owner with a hosting runner', () async {
      await signIn();
      final offer = await c.read(newAgentOfferProvider.future);
      expect(offer!.runners.single.id, runner.id);
      expect(offer.templates.single.slug, 'file-helper');
    });

    test('is null on an older server', () async {
      fake.server.world.agentTemplates = null;
      await signIn();
      expect(await c.read(newAgentOfferProvider.future), isNull);
    });

    test('is null without a runner that hosts agents', () async {
      runner.hostsAgents = null;
      await signIn();
      expect(await c.read(newAgentOfferProvider.future), isNull);
    });

    test('is null for a person who is not an owner', () async {
      await signIn(login: WorldSeed.samLogin);
      expect(await c.read(newAgentOfferProvider.future), isNull);
    });
  });

  test('a stdio template with host: the class before the definition', () async {
    await signIn();
    fake.server.requests.clear();
    final f = await flow();
    await f.run();
    expect(f.failure, isNull);
    expect(f.finished, isTrue);
    expect(ops(), [
      'createAgent 201',
      'setAgentToolClass 200',
      'putAgentDefinition 200',
    ]);
    final agent = fake.server.world.agents[f.agent!.id]!;
    expect(agent.toolClass, 'host');
    expect(agent.hostedBy, runner.id);
    expect(agent.definitions, hasLength(1));
    expect(prompt.asked, everyElement('Create the agent File helper'));
  });

  test('tool_class_too_low, the class raised, Try again: one agent, class then definition', () async {
    await signIn();
    final f = await flow();
    f.edit((d) => d.toolClass = AgentToolClass.chat);
    fake.server.requests.clear();
    await f.run();
    expect(f.failedStep, NewAgentStep.storeDefinition);
    expect(f.errorsAt('/mcp_servers/0/transport'), ['tool_class_too_low']);
    expect(rows(), ['agent.create']);

    f.edit((d) => d.toolClass = AgentToolClass.host);
    await f.run();
    expect(f.finished, isTrue);
    expect(ops(), [
      'createAgent 201',
      'putAgentDefinition 400',
      'setAgentToolClass 200',
      'putAgentDefinition 200',
    ]);
    expect(rows(), ['agent.create', 'agent.tool_class.set', 'agent.update']);
  });

  test('stored under host, a later step failing, the server removed and chat: definition first', () async {
    await signIn();
    final world = fake.server.world;
    final space = fake.seed.space;
    world.createChannel(fake.seed.alex.id, space, 'files', {
      fake.seed.alex.id: 'trigger',
    });
    final f = await flow();
    f.edit(
      (d) => d
        ..channelWanted = true
        ..channelSpaceId = space
        ..channelName = 'files',
    );
    fake.server.requests.clear();
    await f.run();
    expect(f.failedStep, NewAgentStep.createChannel);
    expect(world.agents[f.agent!.id]!.definitions, hasLength(1));

    // The channel step is not done, so its name may change; the server and the class go.
    f.edit(
      (d) => d
        ..servers.clear()
        ..toolClass = AgentToolClass.chat
        ..channelName = 'file-helper',
    );
    fake.server.requests.clear();
    await f.run();
    expect(f.failure, isNull);
    expect(f.finished, isTrue);
    expect(ops(), [
      'putAgentDefinition 200',
      'setAgentToolClass 200',
      'createChannel 201',
      'setChannelOperator 200',
    ]);
    final agent = world.agents[f.agent!.id]!;
    expect(agent.toolClass, 'chat');
    expect(
      world.agents.values.where((a) => a.hostedBy == runner.id),
      hasLength(1),
    );
  });
}
