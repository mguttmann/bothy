// SPDX-License-Identifier: Apache-2.0

/// Executors of the message writes: create, edit, delete and reactions (04 5.1, 04 13.3).
library;

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart' show Response;
import 'package:local_store/local_store.dart';

import '../executors.dart';

/// The message operations of the outbox and their payloads.
///
/// | Kind | Payload | Lane |
/// |---|---|---|
/// | `createMessage` | `MessageCreate` without `client_msg_id` (the entry ID is used) | thread |
/// | `updateMessage` | `message_id`, `version` (the `If-Match`), `MessageUpdate` members | thread |
/// | `deleteMessage` | `message_id` | thread |
/// | `addReaction`, `removeReaction` | `message_id`, `emoji` | thread, else `message:<id>` |
///
/// `message_id` and `version` may be null in a follow-up; the outbox fills them from the
/// confirmed entry it waits for (D-M4-15-6). A null that is still there when the entry is
/// sent is a programming error (`executor_error`).
///
/// Example: `MessageOps.register(executors, api);`
abstract final class MessageOps {
  /// `POST /threads/{thread_id}/messages`.
  static const String createMessage = 'createMessage';

  /// `PATCH /messages/{message_id}` with `If-Match`.
  static const String updateMessage = 'updateMessage';

  /// `DELETE /messages/{message_id}`.
  static const String deleteMessage = 'deleteMessage';

  /// `PUT /messages/{message_id}/reactions/{emoji}`.
  static const String addReaction = 'addReaction';

  /// `DELETE /messages/{message_id}/reactions/{emoji}`.
  static const String removeReaction = 'removeReaction';

  /// Registers the five executors on [registry]; they call [api].
  static void register(OutboxExecutors registry, ApiClient api) {
    final messages = api.getMessagesApi();
    registry
      ..register(
        createMessage,
        (e, ctx) async {
          final body = decodePayload(e.payload)..['client_msg_id'] = e.id;
          final response = await messages.createMessage(
            threadId: _thread(e),
            messageCreate: api.serializers.deserializeWith(
              MessageCreate.serializer,
              body,
            )!,
          );
          return _messageResponse(response);
        },
        followUpEdit: followUpEdit,
        followUpRemove: followUpRemove,
      )
      ..register(
        updateMessage,
        (e, ctx) async {
          final body = decodePayload(e.payload);
          final id = _required<String>(body, 'message_id');
          final version =
              (body['version'] as int?) ??
              e.expectedVersion?.value ??
              (throw StateError('updateMessage without a version'));
          body
            ..remove('message_id')
            ..remove('version');
          final response = await messages.updateMessage(
            messageId: id,
            ifMatch: '"$version"',
            messageUpdate: api.serializers.deserializeWith(
              MessageUpdate.serializer,
              body,
            )!,
            extra: ctx.extra,
          );
          return _messageResponse(response);
        },
        followUpEdit: followUpEdit,
        followUpRemove: followUpRemove,
      )
      ..register(deleteMessage, (e, ctx) async {
        final id = _required<String>(decodePayload(e.payload), 'message_id');
        final response = await messages.deleteMessage(
          messageId: id,
          extra: ctx.extra,
        );
        return OutboxResponse(
          status: response.statusCode,
          result: {'message_id': id},
        );
      })
      ..register(addReaction, (e, ctx) async {
        final body = decodePayload(e.payload);
        final response = await messages.addReaction(
          messageId: _required<String>(body, 'message_id'),
          emoji: _required<String>(body, 'emoji'),
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode);
      }, lane: _reactionLane)
      ..register(removeReaction, (e, ctx) async {
        final body = decodePayload(e.payload);
        final response = await messages.removeReaction(
          messageId: _required<String>(body, 'message_id'),
          emoji: _required<String>(body, 'emoji'),
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode);
      }, lane: _reactionLane);
  }

  /// The follow-up `updateMessage` that applies [edited] (its `content_md` and
  /// `mention_ids`) after the create or edit [entry] was attempted (04 13.3).
  static OutboxRequest followUpEdit(
    OutboxEntry entry,
    Map<String, Object?> edited,
  ) => OutboxRequest(
    kind: updateMessage,
    threadId: entry.threadId,
    payload: {
      'message_id': null,
      'version': null,
      for (final key in const ['content_md', 'mention_ids'])
        if (edited.containsKey(key)) key: edited[key],
    },
  );

  /// The follow-up `deleteMessage` after the create or edit [entry] was attempted (04 13.3).
  static OutboxRequest followUpRemove(OutboxEntry entry) => OutboxRequest(
    kind: deleteMessage,
    threadId: entry.threadId,
    payload: const {'message_id': null},
  );

  static String? _reactionLane(OutboxEntry e) => e.threadId != null
      ? null
      : 'message:${decodePayload(e.payload)['message_id']}';

  static OutboxResponse _messageResponse(Response<Message> response) {
    final m = response.data;
    return OutboxResponse(
      status: response.statusCode,
      body: m,
      result: m == null ? const {} : {'message_id': m.id, 'version': m.version},
    );
  }
}

String _thread(OutboxEntry e) =>
    e.threadId?.value ?? (throw StateError('${e.kind} needs a thread'));

T _required<T extends Object>(Map<String, Object?> body, String key) {
  final value = body[key];
  if (value is! T) throw StateError('member $key is missing');
  return value;
}
