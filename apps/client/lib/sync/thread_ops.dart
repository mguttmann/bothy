// SPDX-License-Identifier: Apache-2.0

/// The creation of a forum post through the outbox (`createThread`; 01 6.1; components.md 6;
/// brief M4-19 Design note 3).
library;

import 'dart:async';

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
/// | `putChannelDraft` | `channel_id`, `content_md`, `base_version` | `forum:<channel_id>` |
/// | `deleteChannelDraft` | `channel_id` | `forum:<channel_id>` |
///
/// The draft of a new post (03 `drafts` with `thread_id` NULL, one per person and forum; brief
/// M4-41) shares the lane of the forum's new posts, so a draft write queued before a send goes
/// out before it. A new post sent with `clear_draft` (payload member `clear_draft`) has the
/// server delete the person's draft of the forum in the transaction of the first message (04 14;
/// docs/decisions.d/M4-41.md), so no `deleteChannelDraft` follows a send.
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

  /// `PUT /channels/{channel_id}/draft`: the draft of a new post in a forum (04 14).
  static const String putChannelDraft = 'putChannelDraft';

  /// `DELETE /channels/{channel_id}/draft`.
  static const String deleteChannelDraft = 'deleteChannelDraft';

  /// The lane of the new posts and the new post draft of [forum].
  static String laneOf(String forum) => 'forum:$forum';

  /// Registers the executor on [registry]; it calls [api] and hands the created thread (API
  /// `Thread` as JSON) to [storeThread], which stores it by the version rule and indexes its
  /// title (`CatchUpListener.storeConfirmedThread`), so the post shows at once (its
  /// `thread.created` follows). [storeDraft] gets the answer of `putChannelDraft` (API
  /// `DraftPutResult` as JSON; `CatchUpListener.applyDraftPutResult`), and [onCreated] the
  /// entry ID of a new post with the ID of the thread the server created for it, so the new
  /// chat view can open it (brief M4-41).
  static void register(
    OutboxExecutors registry,
    ApiClient api,
    Future<void> Function(Map<String, Object?> thread) storeThread, {
    Future<void> Function(Map<String, Object?> result)? storeDraft,
    void Function(String entryId, ThreadId thread)? onCreated,
  }) {
    final threads = api.getThreadsApi();
    final prefs = api.getPreferencesApi();
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
          if (body['clear_draft'] == true) b.firstMessage.clearDraft = true;
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
        onCreated?.call(e.id, ThreadId.parse(created.thread.id));
      }
      return OutboxResponse(status: response.statusCode, body: created);
    }, lane: _lane);
    registry
      ..register(putChannelDraft, (e, ctx) async {
        final body = decodePayload(e.payload);
        final base = body['base_version'];
        final response = await prefs.putChannelDraft(
          channelId: _channelOf(body),
          draftPut: DraftPut((b) {
            b.contentMd = body['content_md'] as String? ?? '';
            if (base is int) b.baseVersion = base;
          }),
          extra: ctx.extra,
        );
        final data = response.data;
        if (data != null && storeDraft != null) {
          final json = api.serializers.serializeWith(
            DraftPutResult.serializer,
            data,
          );
          // 04 14: the answer reports `overwrote_newer`; the app shows a notice.
          if (json is Map) await storeDraft(json.cast<String, Object?>());
        }
        return OutboxResponse(status: response.statusCode, body: data);
      }, lane: _lane)
      ..register(deleteChannelDraft, (e, ctx) async {
        final response = await prefs.deleteChannelDraft(
          channelId: _channelOf(decodePayload(e.payload)),
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode);
      }, lane: _lane);
  }

  static String _lane(OutboxEntry e) =>
      laneOf(_channelOf(decodePayload(e.payload)));

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
  /// gives (brief M5-06); null lets the outbox make one. [clearDraft] asks the server to delete
  /// the person's draft of a new post in [forum] with the first message (04 14; the new chat
  /// view of M4-41 sends it). An empty [title] stays empty: the server keeps `title_source`
  /// `auto` and the agent names the post (09 12).
  static OutboxRequest postRequest({
    required ChannelId forum,
    required String title,
    required String contentMd,
    List<ForumTagId> tagIds = const [],
    MessageAttachments? attachments,
    String? id,
    bool clearDraft = false,
  }) => OutboxRequest(
    kind: createThread,
    id: id,
    dependsOn: attachments?.uploads ?? const [],
    payload: {
      'channel_id': forum.value,
      'title': title,
      'content_md': contentMd,
      'tag_ids': [for (final t in tagIds) t.value],
      if (clearDraft) 'clear_draft': true,
      ...?attachments?.payload,
    },
  );

  /// The outbox request that saves [contentMd] as the person's draft of a new post in
  /// [forum]; [baseVersion] is the version of the draft this device last saw (04 14). An empty
  /// text deletes the draft on the server.
  static OutboxRequest channelDraftRequest({
    required ChannelId forum,
    required String contentMd,
    ResourceVersion? baseVersion,
  }) => OutboxRequest(
    kind: putChannelDraft,
    payload: {
      'channel_id': forum.value,
      'content_md': contentMd,
      'base_version': ?baseVersion?.value,
    },
  );

  /// The outbox request that deletes the person's draft of a new post in [forum].
  static OutboxRequest deleteChannelDraftRequest(ChannelId forum) =>
      OutboxRequest(
        kind: deleteChannelDraft,
        payload: {'channel_id': forum.value},
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

/// The threads the server created for this run's new posts, by outbox entry ID (brief M4-41):
/// the executor of `createThread` records each ([record]), and the new chat view that sent the
/// entry opens the thread once it is known ([threadOf], [changes]). Kept in memory only: after
/// a restart the post simply appears in the sidebar.
///
/// Example: `ThreadOps.register(executors, api, store, onCreated: created.record);`
final class CreatedPosts {
  final Map<String, ThreadId> _threads = {};
  final StreamController<String> _changes = StreamController.broadcast();

  /// Records that the entry [entryId] created [thread].
  void record(String entryId, ThreadId thread) {
    _threads[entryId] = thread;
    _changes.add(entryId);
  }

  /// The thread the entry [entryId] created, or null while unknown.
  ThreadId? threadOf(String entryId) => _threads[entryId];

  /// The entry IDs as they are recorded.
  Stream<String> get changes => _changes.stream;
}
