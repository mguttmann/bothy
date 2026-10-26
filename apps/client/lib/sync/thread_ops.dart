// SPDX-License-Identifier: Apache-2.0

/// The creation of a forum post through the outbox (`createThread`; 01 6.1; components.md 6;
/// brief M4-19 Design note 3).
library;

import 'package:api_client/api_client.dart' as wire show Thread;
import 'package:api_client/api_client.dart' hide Thread;
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../files/upload_executor.dart' show MessageAttachments;

/// The outbox operation `createThread` and its payload.
///
/// | Kind | Payload | Lane |
/// |---|---|---|
/// | `createThread` | `channel_id`, `title`, `content_md`, `tag_ids`, attachments of `MessageAttachments` | `forum:<channel_id>` |
/// | `createThread` from a message | `channel_id`, `parent_message_id` | `forum:<channel_id>` |
///
/// A thread from a message of the main history (M4-20, 01 3.1) carries `parent_message_id`
/// and no first message; the server titles it from the message (`title_source = auto`).
///
/// A forum post needs its first message (the server refuses one without, `ThreadCreate`); the
/// first message carries the entry's ID as its `client_msg_id`, which never changes on retry,
/// so a repeated send creates neither a second post nor a second message (04 5.1), and the
/// echo of that message confirms the entry (04 13.2 item 6). The first message carries the
/// files of the post's upload entries (`attachment_file_ids`, filled by
/// `AttachmentMessageOps` from the slots); a share gives the entry its own `client_msg_id`
/// (brief M5-06).
///
/// Example: `ThreadOps.register(executors, api, engine.catchUp.storeConfirmedThread);`
abstract final class ThreadOps {
  /// `POST /channels/{channel_id}/threads`.
  static const String createThread = 'createThread';

  /// Registers the executor on [registry]; it calls [api] and hands the created thread (API
  /// `Thread` as JSON) to [storeThread], which stores it by the version rule and indexes its
  /// title (`CatchUpListener.storeConfirmedThread`), so the post shows at once (its
  /// `thread.created` follows).
  static void register(
    OutboxExecutors registry,
    ApiClient api,
    Future<void> Function(Map<String, Object?> thread) storeThread,
  ) {
    final threads = api.getThreadsApi();
    registry.register(createThread, (e, ctx) async {
      final body = decodePayload(e.payload);
      final tags = body['tag_ids'];
      final files = body['attachment_file_ids'];
      final parent = body['parent_message_id'];
      final response = await threads.createThread(
        channelId: _channelOf(body),
        threadCreate: ThreadCreate((b) {
          if (parent is String) {
            b.parentMessageId = parent;
            return;
          }
          b
            ..title = body['title'] as String?
            ..firstMessage.clientMsgId = ctx.idempotencyKey
            ..firstMessage.contentMd = body['content_md'] as String?;
          if (tags is List && tags.isNotEmpty) {
            b.tagIds.addAll(tags.cast<String>());
          }
          if (files is List && files.isNotEmpty) {
            b.firstMessage.attachmentFileIds.addAll(files.cast<String>());
          }
        }),
        extra: ctx.extra,
      );
      final created = response.data;
      if (created != null) {
        final json = api.serializers.serializeWith(
          wire.Thread.serializer,
          created.thread,
        );
        if (json is Map) await storeThread(json.cast<String, Object?>());
      }
      return OutboxResponse(status: response.statusCode, body: created);
    }, lane: (e) => 'forum:${_channelOf(decodePayload(e.payload))}');
  }

  /// Puts a new post of [forum] into the outbox of [engine]: [title], the first message
  /// [contentMd] and the tags [tagIds].
  static Future<OutboxEntry> enqueue(
    SyncEngine engine, {
    required ChannelId forum,
    required String title,
    required String contentMd,
    List<ForumTagId> tagIds = const [],
  }) => engine.outbox.enqueue(
    postRequest(
      forum: forum,
      title: title,
      contentMd: contentMd,
      tagIds: tagIds,
    ),
  );

  /// The outbox request of a new post of [forum]: [title], the first message [contentMd]
  /// with [attachments] (which make it wait for their upload entries, 15 7.5) and the tags
  /// [tagIds]. [id] is the entry ID, and so the first message's `client_msg_id`, a share
  /// gives (brief M5-06); null lets the outbox make one.
  static OutboxRequest postRequest({
    required ChannelId forum,
    required String title,
    required String contentMd,
    List<ForumTagId> tagIds = const [],
    MessageAttachments? attachments,
    String? id,
  }) => OutboxRequest(
    kind: createThread,
    id: id,
    dependsOn: attachments?.uploads ?? const [],
    payload: {
      'channel_id': forum.value,
      'title': title,
      'content_md': contentMd,
      'tag_ids': [for (final t in tagIds) t.value],
      ...?attachments?.payload,
    },
  );

  /// Puts a new thread from [message] of the main history of the text channel [channel]
  /// into the outbox of [engine] (01 3.1, brief M4-20 Design note 4).
  static Future<OutboxEntry> enqueueFromMessage(
    SyncEngine engine, {
    required ChannelId channel,
    required MessageId message,
  }) => engine.outbox.enqueue(
    OutboxRequest(
      kind: createThread,
      payload: {
        'channel_id': channel.value,
        'parent_message_id': message.value,
      },
    ),
  );

  static String _channelOf(Map<String, Object?> body) {
    final channel = body['channel_id'];
    if (channel is! String) throw StateError('member channel_id is missing');
    return channel;
  }
}
