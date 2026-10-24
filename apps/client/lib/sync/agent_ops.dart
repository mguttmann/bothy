// SPDX-License-Identifier: Apache-2.0

/// The outbox executors of the agent interface (brief M4-21 Scope; 04 13.2; 15 7.5; 09 10, 11):
/// the use of a component, the invocation of a registered command and the session actions of
/// `/new` and `/resume`. `cancelRun`, `setThreadTitle` and `closeThread` are those of M4-20
/// (`AppMessageOps`).
library;

import 'package:api_client/api_client.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

/// The operations of the agent interface and their payloads.
///
/// | Kind | Payload | Thread | Lane |
/// |---|---|---|---|
/// | `createInteraction` | `component_id`, `message_id`, `custom_id`, `values` | yes | thread |
/// | `invokeCommand` | `channel_id`, `command_id`, `thread_id`, `options` | yes | `command:<channel_id>` |
/// | `resetAgentSession` | empty | yes | thread |
/// | `bindAgentSession` | `external_session_ref`, optional `expected_current_session_id` | yes | thread |
///
/// The entry's ID is the `client_interaction_id` of an interaction (idempotent per person and
/// ID, 09 11.4) and the `Idempotency-Key` of the other writes, so a retry never runs a command
/// or resets a session twice (04 5).
///
/// Example: `AgentOps.register(executors, api);`
abstract final class AgentOps {
  /// `POST /components/{component_id}/interactions`.
  static const String createInteraction = 'createInteraction';

  /// `POST /channels/{channel_id}/commands/{command_id}/invoke`.
  static const String invokeCommand = 'invokeCommand';

  /// `POST /threads/{thread_id}/agent-sessions/reset`.
  static const String resetAgentSession = 'resetAgentSession';

  /// `POST /threads/{thread_id}/agent-sessions/bind`.
  static const String bindAgentSession = 'bindAgentSession';

  /// Registers the executors on [registry]; they call [api].
  static void register(OutboxExecutors registry, ApiClient api) {
    final interactions = api.getInteractionsApi();
    final commands = api.getCommandsApi();
    final threads = api.getThreadsApi();
    registry
      ..register(createInteraction, (e, ctx) async {
        final body = decodePayload(e.payload);
        final response = await interactions.createInteraction(
          componentId: _required(body, 'component_id'),
          interactionCreate: api.serializers.deserializeWith(
            InteractionCreate.serializer,
            {
              'client_interaction_id': ctx.idempotencyKey,
              'values': body['values'] ?? const <String, Object?>{},
            },
          )!,
        );
        return OutboxResponse(status: response.statusCode, body: response.data);
      })
      ..register(invokeCommand, (e, ctx) async {
        final body = decodePayload(e.payload);
        final response = await commands.invokeCommand(
          channelId: _required(body, 'channel_id'),
          commandId: _required(body, 'command_id'),
          commandInvoke: api.serializers.deserializeWith(
            CommandInvoke.serializer,
            {
              'thread_id': ?body['thread_id'],
              'options': body['options'] ?? const <Object?>[],
            },
          )!,
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode, body: response.data);
      }, lane: (e) => 'command:${decodePayload(e.payload)['channel_id']}')
      ..register(resetAgentSession, (e, ctx) async {
        final response = await threads.resetAgentSession(
          threadId: _thread(e),
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode, body: response.data);
      })
      ..register(bindAgentSession, (e, ctx) async {
        final body = decodePayload(e.payload);
        final response = await threads.bindAgentSession(
          threadId: _thread(e),
          agentSessionBind: AgentSessionBind(
            (b) => b
              ..externalSessionRef = _required(body, 'external_session_ref')
              ..expectedCurrentSessionId =
                  body['expected_current_session_id'] as String?,
          ),
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode, body: response.data);
      });
  }

  /// Puts one interaction with [component] of [message] in [thread] into the outbox of
  /// [engine]; [values] is the `InteractionCreate.values` object (09 11.1).
  static Future<OutboxEntry> enqueueInteraction(
    SyncEngine engine, {
    required ThreadId thread,
    required MessageId message,
    required MessageComponent component,
    required Map<String, Object?> values,
  }) => engine.outbox.enqueue(
    OutboxRequest(
      kind: createInteraction,
      threadId: thread,
      payload: {
        'component_id': component.id.value,
        'message_id': message.value,
        'custom_id': component.customId,
        'values': values,
      },
    ),
  );

  /// Puts the invocation of [command] in [channel] from [thread] with the option values
  /// [options] (name to value, in the command's order) into the outbox of [engine].
  ///
  /// The entry belongs to [thread], so a refusal (for example `409 run_active` or
  /// `403 agent_invocation_denied`) shows in that thread's history as a failed write with
  /// "Send again" and "Discard"; its lane stays `command:<channel_id>`.
  static Future<OutboxEntry> enqueueCommand(
    SyncEngine engine, {
    required ChannelId channel,
    required CommandId command,
    required ThreadId thread,
    List<({String name, String value})> options = const [],
  }) => engine.outbox.enqueue(
    OutboxRequest(
      kind: invokeCommand,
      threadId: thread,
      payload: {
        'channel_id': channel.value,
        'command_id': command.value,
        'thread_id': thread.value,
        'options': [
          for (final o in options) {'name': o.name, 'value': o.value},
        ],
      },
    ),
  );

  /// Puts `/new` of [thread] into the outbox of [engine] (`agent_session.reset`).
  static Future<OutboxEntry> enqueueReset(SyncEngine engine, ThreadId thread) =>
      engine.outbox.enqueue(
        OutboxRequest(kind: resetAgentSession, threadId: thread, payload: {}),
      );

  /// Puts `/resume` of [thread] with [externalSessionRef] into the outbox of [engine]
  /// (`agent_session.bind`); [expectedCurrent] guards against a session changed meanwhile.
  static Future<OutboxEntry> enqueueBind(
    SyncEngine engine,
    ThreadId thread, {
    required String externalSessionRef,
    String? expectedCurrent,
  }) => engine.outbox.enqueue(
    OutboxRequest(
      kind: bindAgentSession,
      threadId: thread,
      payload: {
        'external_session_ref': externalSessionRef,
        'expected_current_session_id': ?expectedCurrent,
      },
    ),
  );

  static String _thread(OutboxEntry e) {
    final thread = e.threadId;
    if (thread == null) throw StateError('entry ${e.id} has no thread');
    return thread.value;
  }

  static String _required(Map<String, Object?> body, String key) {
    final value = body[key];
    if (value is! String) throw StateError('member $key is missing');
    return value;
  }
}
