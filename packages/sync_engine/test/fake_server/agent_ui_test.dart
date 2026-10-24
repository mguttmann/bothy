// SPDX-License-Identifier: Apache-2.0

// The agent interface handlers M4-21 added to the fake (brief M4-21 Scope): interactions with
// the single-use rule and the approver rules of 09 11.2 and 11.3, the commands of a channel
// with the operator first, option suggestions and invocations (09 10), the agent sessions
// (09 9), the runs and the costs of a thread. Every response passes the contract check.

import 'package:api_client/api_client.dart';
import 'package:built_collection/built_collection.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late FakeWorld world;
  late WorldSeed w;
  late FakeAgentInterface ui;
  late ApiClient alexApi;
  late ApiClient samApi;

  setUp(() async {
    h = await FakeWorldHarness.start();
    world = h.server.world;
    w = h.seed;
    ui = FakeAgentInterface.of(world);
    final alex = h.device();
    await signedIn(h, alex);
    final sam = h.device(login: WorldSeed.samLogin, keySeed: 2);
    await signedIn(h, sam);
    alexApi = alex.api;
    samApi = sam.api;
  });
  tearDown(() async {
    expect(h.server.violations, isEmpty);
    await h.stop();
  });

  Future<int> status(Future<Object?> call) => call.then(
    (_) => 200,
    onError: (Object e) => (e as DioException).response!.statusCode!,
  );

  Future<String?> reason(Future<Object?> call) => call.then(
    (_) => null,
    onError: (Object e) =>
        ((e as DioException).response!.data! as Map)['reason'] as String?,
  );

  /// A card of Robin in Release notes with "Allow" and "Deny", bound to a run Alex started.
  ({String message, String allow, String deny}) approvalCard() {
    final trigger = w.post(w.alex, w.releaseNotes, '@robin run the tests');
    final run = world.createRun(
      agentId: w.robin.id,
      triggeringMessageId: trigger['id']! as String,
      triggeredBy: w.alex.id,
    );
    final id = ui.postCard(w.robin.id, w.releaseNotes, 'Approval needed', [
      {
        'custom_id': 'allow',
        'type': 'button',
        'definition': {'label': 'Allow once', 'value': 'allow'},
      },
      {
        'custom_id': 'deny',
        'type': 'button',
        'definition': {'label': 'Deny', 'value': 'deny'},
      },
    ], runId: run['id']! as String);
    final comps = (world.messages[id]!['components']! as List<Object?>)
        .cast<JsonMap>();
    return (
      message: id,
      allow: comps[0]['id']! as String,
      deny: comps[1]['id']! as String,
    );
  }

  Future<Response<Interaction>> interact(ApiClient api, String component) =>
      api.getInteractionsApi().createInteraction(
        componentId: component,
        interactionCreate: InteractionCreate(
          (b) => b..clientInteractionId = world.nextId(),
        ),
      );

  group('createInteraction', () {
    test('uses a single-use component once and disables the others', () async {
      final card = approvalCard();
      final versionBefore = world.messages[card.message]!['version']! as int;
      final clientId = world.nextId();
      final first = await alexApi.getInteractionsApi().createInteraction(
        componentId: card.allow,
        interactionCreate: InteractionCreate(
          (b) => b..clientInteractionId = clientId,
        ),
      );
      expect(first.statusCode, 201);
      expect(first.data!.customId, 'allow');
      final comps =
          (world.messages[card.message]!['components']! as List<Object?>)
              .cast<JsonMap>();
      expect([for (final c in comps) c['state']], ['used', 'disabled']);
      expect(comps[0]['used_by'], w.alex.id);
      expect(world.messages[card.message]!['version'], versionBefore + 1);
      expect(world.log.last.type, 'message.updated');

      // The same client ID is a replay, not a second interaction (09 11.4).
      final replay = await alexApi.getInteractionsApi().createInteraction(
        componentId: card.allow,
        interactionCreate: InteractionCreate(
          (b) => b..clientInteractionId = clientId,
        ),
      );
      expect(replay.statusCode, 200);
      expect(replay.data!.id, first.data!.id);
      // A second decision loses (09 11.3).
      expect(await status(interact(alexApi, card.deny)), 409);
    });

    test('refuses a person without trigger and one who did not start the '
        'run', () async {
      final card = approvalCard();
      // Sam has `write` in general.
      expect(await reason(interact(samApi, card.allow)), 'level_insufficient');
      world.setAccess(
        w.sam.id,
        world.threads[w.releaseNotes]!['channel_id']! as String,
        'trigger',
      );
      final restricted = ui.postCard(w.robin.id, w.releaseNotes, 'Approve', [
        {
          'custom_id': 'ok',
          'type': 'button',
          'definition': {'label': 'OK', 'value': 'ok'},
          'restricted_to': 'run_starter',
        },
      ], runId: world.runs.keys.single);
      final comp =
          ((world.messages[restricted]!['components']! as List<Object?>).single
                  as JsonMap)['id']!
              as String;
      expect(await reason(interact(samApi, comp)), 'component_restricted');
      expect((await interact(alexApi, comp)).statusCode, 201);
    });

    test('an unknown component is 404', () async {
      expect(await status(interact(alexApi, world.nextId())), 404);
    });
  });

  group('commands', () {
    test('lists the commands of a channel, suggests and invokes', () async {
      final general = world.threads[w.releaseNotes]!['channel_id']! as String;
      world.setChannelOperator(general, w.robin.id);
      final model = ui.registerCommand(
        w.robin.id,
        'model',
        description: 'Choose a model',
        options: [
          {
            'name': 'name',
            'description': 'Model',
            'type': 'string',
            'required': true,
            'autocomplete': true,
          },
        ],
        autocomplete: (option, value, _) => [
          for (final m in const ['model-large', 'model-fast'])
            if (m.startsWith(value)) {'name': m, 'value': m},
        ],
      );
      expect(world.log.last.type, 'command.changed');
      final commands = alexApi.getCommandsApi();
      final list = (await commands.listChannelCommands(channelId: general))
          .data!;
      expect([for (final c in list.items) c.name], ['model']);

      final choices = (await commands.autocompleteCommand(
        channelId: general,
        commandId: model,
        autocompleteQuery: AutocompleteQuery(
          (b) => b
            ..option = 'name'
            ..value = 'model-l',
        ),
      )).data!;
      expect([for (final c in choices.choices) c.value], ['model-large']);
      expect(ui.autocompleteRequests.single.value, 'model-l');

      final invoked = await commands.invokeCommand(
        channelId: general,
        commandId: model,
        commandInvoke: CommandInvoke(
          (b) => b
            ..threadId = w.releaseNotes
            ..options = ListBuilder([
              CommandOptionValue(
                (v) => v
                  ..name = 'name'
                  ..value = 'model-large',
              ),
            ]),
        ),
        idempotencyKey: world.nextId(),
      );
      expect(invoked.statusCode, 202);
      expect(ui.invocations.single['principal_id'], w.alex.id);
      expect(ui.invocations.single['options'], [
        {'name': 'name', 'value': 'model-large'},
      ]);
      // Sam lacks `trigger`.
      expect(
        await status(
          samApi.getCommandsApi().invokeCommand(
            channelId: general,
            commandId: model,
            commandInvoke: CommandInvoke(),
            idempotencyKey: world.nextId(),
          ),
        ),
        403,
      );
    });
  });

  group('sessions, runs and costs', () {
    test('lists, resets and binds the agent session of a thread', () async {
      final general = world.threads[w.releaseNotes]!['channel_id']! as String;
      world.setChannelOperator(general, w.robin.id);
      ui
        ..currentSession(w.releaseNotes, w.robin.id)
        ..addEarlierSession(w.releaseNotes, w.robin.id, 'sess-earlier');
      final threads = alexApi.getThreadsApi();
      var list = (await threads.listAgentSessions(threadId: w.releaseNotes))
          .data!;
      expect(list.items.where((s) => s.isCurrent), hasLength(1));
      final reset = (await threads.resetAgentSession(
        threadId: w.releaseNotes,
        idempotencyKey: world.nextId(),
      )).data!;
      expect(reset.isCurrent, isTrue);
      final bound = (await threads.bindAgentSession(
        threadId: w.releaseNotes,
        agentSessionBind: AgentSessionBind(
          (b) => b..externalSessionRef = 'sess-earlier',
        ),
        idempotencyKey: world.nextId(),
      )).data!;
      expect(bound.externalSessionRef, 'sess-earlier');
      list = (await threads.listAgentSessions(threadId: w.releaseNotes)).data!;
      expect(list.items, hasLength(4));
      expect(list.items.where((s) => s.isCurrent).single.id, bound.id);
    });

    test('lists the runs and sums the costs of a thread', () async {
      final trigger = w.post(w.alex, w.releaseNotes, '@robin go');
      world.createRun(
        agentId: w.robin.id,
        triggeringMessageId: trigger['id']! as String,
        triggeredBy: w.alex.id,
      );
      final runs = (await alexApi.getRunsApi().listThreadRuns(
        threadId: w.releaseNotes,
      )).data!;
      expect(runs.items, hasLength(1));
      final costs = (await alexApi.getStatsApi().getThreadCosts(
        threadId: w.releaseNotes,
      )).data!;
      expect(costs.totals.single.billingMode, BillingMode.organization);
      expect(costs.runs.single.runId, runs.items.single.id);
    });
  });
}
