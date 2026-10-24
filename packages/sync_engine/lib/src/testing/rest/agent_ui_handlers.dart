// SPDX-License-Identifier: Apache-2.0

/// Handlers of the agent interface (brief M4-21 Scope "Fake server handlers"):
/// `createInteraction` (09 11.2, 11.3), `listChannelCommands`, `autocompleteCommand`,
/// `invokeCommand` (09 10), `listThreadRuns`, `getThreadCosts`, `listAgentSessions`,
/// `resetAgentSession` and `bindAgentSession` (09 9).
library;

import '../rest_fake.dart';
import '../world.dart';
import '../world_agents.dart';

/// Registers the handlers on [rest].
///
/// What the fake leaves out of the server's rules: the guard of `agent.invoke` beyond the
/// level `trigger` (reachability, I2, project binding), the delivery of the interaction and
/// the invocation to the agent (`interaction.created`, `command.invoked`), and the expiry
/// job (an `expires_at` in the past is refused like an inactive component).
void registerAgentUiHandlers(RestFake rest) {
  final world = rest.world;
  final ui = FakeAgentInterface.of(world);

  void needsTrigger(FakeRequest r, String channelId) {
    if (world.levelOf(r.person.id, channelId) != 'trigger') {
      throw const FakeProblem(
        403,
        'agent_invocation_denied',
        reason: 'level_insufficient',
      );
    }
  }

  JsonMap visibleCommand(FakeRequest r) {
    final channelId = r.param('channel_id');
    if (!world.seesChannel(r.person.id, channelId)) {
      throw const FakeProblem(404, 'not_found');
    }
    final id = r.param('command_id');
    for (final c in ui.commandsOf(channelId)) {
      if (c['id'] == id) return c;
    }
    throw const FakeProblem(404, 'not_found');
  }

  rest
    ..handle('createInteraction', (r) {
      final found = ui.componentById(r.param('component_id'));
      final message = found?.message;
      final component = found?.component;
      if (message == null ||
          component == null ||
          !world.seesChannel(r.person.id, message['channel_id']! as String)) {
        throw const FakeProblem(404, 'not_found');
      }
      final clientId = r.json['client_interaction_id']! as String;
      final key = '${r.person.id}|$clientId';
      final known = ui.interactions[key];
      // Idempotent by (principal, client_interaction_id) (09 11.4).
      if (known != null) return FakeResponse(200, known);
      final channelId = message['channel_id']! as String;
      needsTrigger(r, channelId);
      final expires = component['expires_at'] as String?;
      if (component['state'] != 'active' ||
          (expires != null && !DateTime.parse(expires).isAfter(world.now()))) {
        throw const FakeProblem(409, 'component_used');
      }
      final capability = component['required_capability'] as String?;
      final owner = r.person.isOwner;
      if (!owner &&
          capability != null &&
          !(world.channels[channelId]!.capabilities[r.person.id]?.contains(
                capability,
              ) ??
              false)) {
        throw const FakeProblem(403, 'forbidden', reason: 'capability_missing');
      }
      final runId = component['run_id'] as String?;
      final restricted = component['restricted_to'];
      final starter = runId == null ? null : world.runs[runId]?['triggered_by'];
      if (!owner &&
          (restricted == 'owner' ||
              (restricted == 'run_starter' && starter != r.person.id))) {
        throw const FakeProblem(
          403,
          'forbidden',
          reason: 'component_restricted',
        );
      }
      final now = world.stamp();
      // 09 11.3: a used single-use component disables the other active ones of the card.
      if (component['single_use'] == true) {
        for (final c
            in (message['components']! as List<Object?>).cast<JsonMap>()) {
          if (c['single_use'] == true && c['state'] == 'active') {
            final mine = identical(c, component);
            c
              ..['state'] = mine ? 'used' : 'disabled'
              ..['version'] = (c['version']! as int) + 1;
            if (mine) {
              c
                ..['used_by'] = r.person.id
                ..['used_at'] = now;
            }
          }
        }
        world.updateMessageFields(message['id']! as String, const {});
      }
      final interaction = <String, Object?>{
        'id': world.nextId(),
        'component_id': component['id'],
        'custom_id': component['custom_id'],
        'message_id': message['id'],
        'thread_id': message['thread_id'],
        'run_id': ?runId,
        'principal_id': r.person.id,
        'client_interaction_id': clientId,
        'values': r.json['values'] ?? const <String, Object?>{},
        'state': 'received',
        'created_at': now,
      };
      ui.interactions[key] = interaction;
      return FakeResponse(201, interaction);
    })
    ..handle('listChannelCommands', (r) {
      final channelId = r.param('channel_id');
      if (!world.seesChannel(r.person.id, channelId)) {
        throw const FakeProblem(404, 'not_found');
      }
      return FakeResponse(200, {'items': ui.commandsOf(channelId)});
    })
    ..handle('autocompleteCommand', (r) {
      final command = visibleCommand(r);
      needsTrigger(r, r.param('channel_id'));
      final option = r.json['option']! as String;
      final value = r.json['value']! as String;
      ui.autocompleteRequests.add((
        commandId: command['id']! as String,
        option: option,
        value: value,
      ));
      final others = <String, Object?>{
        for (final o in (r.json['options'] as List<Object?>? ?? const []))
          if (o is JsonMap) o['name']! as String: o['value'],
      };
      final choices =
          ui.suggestions[command['id']]?.call(option, value, others) ??
          const <JsonMap>[];
      return FakeResponse(200, {'choices': choices});
    })
    ..handle('invokeCommand', (r) {
      final command = visibleCommand(r);
      final channelId = r.param('channel_id');
      needsTrigger(r, channelId);
      final invocation = <String, Object?>{
        'invocation_id': world.nextId(),
        'command_id': command['id'],
        'channel_id': channelId,
        'thread_id': ?r.json['thread_id'],
        'accepted': true,
      };
      ui.invocations.add({
        ...invocation,
        'principal_id': r.person.id,
        'options': r.json['options'] ?? const <Object?>[],
      });
      return FakeResponse(202, invocation);
    })
    ..handle('listThreadRuns', (r) {
      final thread = world.visibleThread(r.person.id, r.param('thread_id'));
      final items = [
        for (final run in world.runs.values)
          if (run['thread_id'] == thread['id']) world.runSummary(run),
      ];
      return FakeResponse(200, {'items': items});
    })
    ..handle('getThreadCosts', (r) {
      final thread = world.visibleThread(r.person.id, r.param('thread_id'));
      final totals = <String, JsonMap>{};
      final runs = <JsonMap>[];
      for (final run in world.runs.values) {
        if (run['thread_id'] != thread['id']) continue;
        final mode =
            (run['billing_mode_used'] ?? run['billing_mode_expected'])!
                as String;
        final cost = run['cost']! as JsonMap;
        final total = totals.putIfAbsent(
          '$mode|${cost['currency']}',
          () => {
            'billing_mode': mode,
            'amount_micros': 0,
            'currency': cost['currency'],
            'is_estimate': false,
          },
        );
        total
          ..['amount_micros'] =
              (total['amount_micros']! as int) + (cost['amount_micros']! as int)
          ..['is_estimate'] =
              total['is_estimate'] == true || cost['is_estimate'] == true;
        runs.add({
          'run_id': run['id'],
          'billing_mode': mode,
          'tokens': {
            'tokens_in': 0,
            'tokens_out': 0,
            'tokens_cache_read': 0,
            'tokens_cache_write': 0,
            'tokens_cache_write_1h': 0,
          },
          'costs': [cost],
          'unpriced': false,
        });
      }
      return FakeResponse(200, {
        'thread_id': thread['id'],
        'totals': totals.values.toList(),
        'runs': runs,
      });
    })
    ..handle('listAgentSessions', (r) {
      final thread = world.visibleThread(r.person.id, r.param('thread_id'));
      final items = [...?ui.sessions[thread['id']]]
        ..sort(
          (a, b) => (b['created_at']! as String).compareTo(
            a['created_at']! as String,
          ),
        );
      return FakeResponse(200, {'items': items});
    })
    ..handle('resetAgentSession', (r) {
      final thread = world.visibleThread(r.person.id, r.param('thread_id'));
      needsTrigger(r, thread['channel_id']! as String);
      final agent = _agentOf(world, thread);
      return FakeResponse(
        201,
        ui.replaceSession(thread['id']! as String, agent, reason: 'reset'),
      );
    })
    ..handle('bindAgentSession', (r) {
      final thread = world.visibleThread(r.person.id, r.param('thread_id'));
      needsTrigger(r, thread['channel_id']! as String);
      final agent = _agentOf(world, thread);
      return FakeResponse(
        201,
        ui.replaceSession(
          thread['id']! as String,
          agent,
          reason: 'rebind',
          externalRef: r.json['external_session_ref']! as String,
        ),
      );
    });
}

/// The operator agent of [thread] (the thread's, else the channel's effective operator);
/// `409 conflict` without one.
String _agentOf(FakeWorld world, JsonMap thread) {
  final own = thread['operator_agent_id'] as String?;
  final channel = world.channels[thread['channel_id']]!;
  final effective =
      own ??
      (channel.json['operator'] as JsonMap?)?['effective_agent_id'] as String?;
  if (effective == null) throw const FakeProblem(409, 'conflict');
  return effective;
}
