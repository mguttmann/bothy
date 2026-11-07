// SPDX-License-Identifier: Apache-2.0

// Editing a built-in agent from the app against the Dart fake server (brief A1-09 Tests and
// requirements 12 to 16; 26 3.1, 4.1, 8.3, 8.4; R-BA-06; BA-T16 edit part): an edit that adds a
// stdio server and raises the class to host runs agent.tool_class.set before
// putAgentDefinition and is stored; one that removes it and lowers the class runs
// putAgentDefinition first and is refused nothing; a lowered class with the server kept is
// definition_needs_host until the server is removed; a stale version is 412 and Reload starts
// again; Restore saves an older version again; Change runner shows each refusal; Disable and
// Continue setup run with their own step-up purpose. The fake refuses like the real server;
// every step-up is confirmed with the device key.

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/new_agent/builtin_agents_entry.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter/widgets.dart' show Locale;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/testing.dart';

import '../admin/admin_step_up_test.dart' show RecordingPrompt;
import '../support/fake_instance.dart';

const Map<String, Object?> _plain = {
  'instructions': 'Keep the notes of Sam.',
  'skills': <Object?>[],
  'mcp_servers': <Object?>[],
  'settings': {'tool_class_wish': 'chat'},
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
    ..._withStdio,
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
  late FakeAgent smallRunner;
  final l = lookupAppLocalizations(const Locale('en'));

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
    smallRunner = world.addAgent(
      handle: 'small-runner',
      displayName: 'Small runner',
      adapterKind: 'code_runner',
      hostsAgents: 'scoped',
    );
    world.agentTemplates = [_filesTemplate];
  });

  tearDown(() async {
    expect(fake.server.violations, isEmpty);
    c.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  Future<void> signIn() async {
    final control = await connected(c, fake);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    expect(c.read(onboardingControllerProvider).step, OnboardingStep.signedIn);
  }

  NewAgentPort port() =>
      localizedAgentPort(c.read(signedInServicesProvider)!.auth.api, l);

  FakeAgent addNotes({
    String toolClass = 'chat',
    List<Map<String, Object?>> definitions = const [_plain],
  }) {
    final a = fake.server.world.addAgent(
      handle: 'notes',
      displayName: 'Notes',
      adapterKind: 'claude_agent',
      hostedBy: runner.id,
      toolClass: toolClass,
    );
    for (final d in definitions) {
      a.definitions.add(Map.of(d));
    }
    a.version += definitions.length;
    return a;
  }

  Future<NewAgentFlow> open(
    FakeAgent a, {
    Map<String, Object?>? content,
  }) async {
    final p = port();
    final offer = (await p.offer())!;
    return NewAgentFlow.forAgent(
      port: p,
      offer: offer,
      newKey: () => fake.server.world.nextId(),
      agent: await p.agent(a.id),
      runner: offer.runners.firstWhere((r) => r.id == a.hostedBy),
      stored: await p.definition(a.id),
      content: content,
    );
  }

  List<String> ops() => [
    for (final r in fake.server.requests)
      if (const {
        'setAgentToolClass',
        'putAgentDefinition',
        'setAgentHost',
        'disableAgent',
      }.contains(r.operationId))
        // A 401 is the step-up round trip of the same request (07 SR5), not a step.
        if (r.status != 401) '${r.operationId} ${r.status}',
  ];

  group('the phone entry Built-in agents', () {
    test('is there for an owner whose server offers New agent', () async {
      await signIn();
      expect(await c.read(builtinAgentsAvailableProvider.future), isTrue);
    });

    test(
      'stays without a hosting runner while a built-in agent exists',
      () async {
        runner.hostsAgents = null;
        smallRunner.hostsAgents = null;
        addNotes();
        await signIn();
        expect(await c.read(builtinAgentsAvailableProvider.future), isTrue);
      },
    );

    test(
      'is hidden without a hosting runner and without a built-in agent',
      () async {
        runner.hostsAgents = null;
        smallRunner.hostsAgents = null;
        await signIn();
        expect(await c.read(builtinAgentsAvailableProvider.future), isFalse);
      },
    );

    test('is hidden for a person who is no owner', () async {
      addNotes();
      final control = await connected(c, fake);
      await control.loginWithPassword(
        WorldSeed.samLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
      expect(await c.read(builtinAgentsAvailableProvider.future), isFalse);
    });
  });

  test(
    'an added stdio server with host: the class first, then the definition',
    () async {
      await signIn();
      final a = addNotes();
      final f = await open(a);
      f.edit((d) {
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
      fake.server.requests.clear();
      await f.run();
      expect(f.failure, isNull);
      expect(f.finished, isTrue);
      expect(ops(), ['setAgentToolClass 200', 'putAgentDefinition 200']);
      expect(a.toolClass, 'host');
      expect(a.definitions, hasLength(2));
      expect(a.definitions.last['mcp_servers'], hasLength(1));
      expect(prompt.asked, everyElement('Edit the agent Notes'));
    },
  );

  test('a removed stdio server with chat: the definition first, no definition_needs_host', () async {
    await signIn();
    final a = addNotes(toolClass: 'host', definitions: [_withStdio]);
    final f = await open(a);
    f.edit((d) {
      d.servers.clear();
      d.pickClass(AgentToolClass.chat);
    });
    fake.server.requests.clear();
    await f.run();
    expect(f.failure, isNull);
    expect(ops(), ['putAgentDefinition 200', 'setAgentToolClass 200']);
    expect(a.toolClass, 'chat');
  });

  test(
    'a lowered class with the server kept: definition_needs_host, then removed',
    () async {
      await signIn();
      final a = addNotes(toolClass: 'host', definitions: [_withStdio]);
      final f = await open(a);
      f.edit((d) => d.pickClass(AgentToolClass.chat));
      fake.server.requests.clear();
      await f.run();
      expect(f.failure?.reason, 'definition_needs_host');
      // The picked class is also the definition's wish, so the definition is stored first.
      expect(ops(), ['putAgentDefinition 200', 'setAgentToolClass 409']);
      expect(a.toolClass, 'host');
      f.removeLocalServers();
      fake.server.requests.clear();
      await f.run();
      expect(f.finished, isTrue);
      expect(ops(), ['putAgentDefinition 200', 'setAgentToolClass 200']);
      expect(a.toolClass, 'chat');
    },
  );

  test('another device saved meanwhile: 412, Reload, then saved', () async {
    await signIn();
    final a = addNotes();
    final f = await open(a);
    f.edit((d) => d.instructions = 'Mine.');
    a
      ..definitions.add({..._plain, 'instructions': 'Theirs.'})
      ..version += 1;
    fake.server.requests.clear();
    await f.run();
    expect(f.failure?.code, 'version_conflict');
    expect(ops(), ['putAgentDefinition 412']);
    await f.reload();
    expect(f.draft!.instructions, 'Theirs.');
    f.edit((d) => d.instructions = 'Mine after all.');
    await f.run();
    expect(f.finished, isTrue);
    expect(a.definitions.last['instructions'], 'Mine after all.');
  });

  test('Restore saves the content of an older version as a new one', () async {
    await signIn();
    final a = addNotes(
      definitions: [
        _plain,
        {..._plain, 'instructions': 'Second.'},
      ],
    );
    final p = port();
    final versions = await p.versions(a.id);
    expect([for (final v in versions) v.version], [2, 1]);
    final old = await p.definitionVersion(a.id, 1);
    final f = await open(a, content: old.json);
    expect(f.hasChanges, isTrue);
    fake.server.requests.clear();
    await f.run();
    expect(ops(), ['putAgentDefinition 200']);
    expect(a.definitions, hasLength(3));
    expect(a.definitions.last['instructions'], 'Keep the notes of Sam.');
  });

  test('Change runner: each refusal, then the move', () async {
    await signIn();
    final a = addNotes(toolClass: 'host', definitions: [_withStdio]);
    final p = port();
    Future<NewAgentFailure> refused(String to) async {
      try {
        await p.setHost(await p.agent(a.id), to, name: 'Notes');
      } on NewAgentFailure catch (f) {
        return f;
      }
      fail('moved');
    }

    expect((await refused(smallRunner.id)).reason, 'host_tool_class_ceiling');
    a
      ..billingMode = 'personal'
      ..credentialHolderId = fake.seed.alex.id;
    final other = fake.server.world.addAgent(
      handle: 'build-runner',
      displayName: 'Build runner',
      adapterKind: 'code_runner',
      hostsAgents: 'host',
    );
    expect((await refused(other.id)).reason, 'hosted_holder_mismatch');
    final plain = fake.server.world.addAgent(
      handle: 'plain-runner',
      displayName: 'Plain runner',
      adapterKind: 'code_runner',
    );
    expect((await refused(plain.id)).errors.single.code, 'host_not_capable');
    a.billingMode = 'organization';
    prompt.asked.clear();
    final moved = await p.setHost(await p.agent(a.id), other.id, name: 'Notes');
    expect(moved.hostedBy, other.id);
    expect(a.hostedBy, other.id);
    expect(
      prompt.asked,
      everyElement('Move the agent Notes to another runner'),
    );
  });

  test('Disable sends the reason with its step-up', () async {
    await signIn();
    final a = addNotes();
    prompt.asked.clear();
    final state = await port().disable(
      a.id,
      reason: 'Not needed any more',
      name: 'Notes',
    );
    expect(state.active, isFalse);
    expect(prompt.asked, ['Disable the agent Notes']);
    final row = fake.server.audit.rows.lastWhere(
      (r) => r['action'] == 'agent.disable',
    );
    expect((row['details'] as Map?)?['reason'], 'Not needed any more');
  });

  test(
    'Continue setup: the class, then the definition, no second agent',
    () async {
      await signIn();
      final a = addNotes(definitions: const []);
      final p = port();
      final offer = (await p.offer())!;
      final f = NewAgentFlow.forAgent(
        port: p,
        offer: offer,
        newKey: () => fake.server.world.nextId(),
        agent: await p.agent(a.id),
        runner: offer.runners.firstWhere((r) => r.id == runner.id),
      )..useTemplate(offer.templates.single);
      fake.server.requests.clear();
      await f.run();
      expect(f.finished, isTrue);
      expect(ops(), ['setAgentToolClass 200', 'putAgentDefinition 200']);
      expect(
        fake.server.requests.where((r) => r.operationId == 'createAgent'),
        isEmpty,
      );
      expect(a.definitions, hasLength(1));
      expect(prompt.asked, everyElement('Set up the agent Notes'));
    },
  );
}
