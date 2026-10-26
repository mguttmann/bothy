// SPDX-License-Identifier: Apache-2.0

/// The writes of the thread view, every one through the outbox (15 1 principle 3; 04 13.2,
/// 13.3; brief M4-20 Design notes 2 to 5).
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../files/upload_executor.dart';
import '../sync/message_ops_app.dart';
import '../sync/thread_ops.dart';

/// The actions of messages, the header and the composer on the outbox of [engine].
///
/// Example: `await ThreadActions(engine).send(thread, 'Hello Sam');`
final class ThreadActions {
  /// Creates the actions on [engine]; [outboxDao] (the engine's database) lets an edit join
  /// the edit of the same message that is not confirmed yet.
  const ThreadActions(this.engine, {this.outboxDao});

  /// The engine whose outbox takes the writes.
  final SyncEngine engine;

  /// The outbox table of the engine's database, or null.
  final OutboxDao? outboxDao;

  Outbox get _outbox => engine.outbox;

  /// Sends [text] into [thread] (with [replyTo] and [attachments]); the server deletes the
  /// draft in the same transaction (`clear_draft`, 04 14).
  Future<OutboxEntry> send(
    ThreadId thread,
    String text, {
    MessageId? replyTo,
    MessageAttachments? attachments,
  }) => _outbox.enqueue(
    messageRequest(thread, text, replyTo: replyTo, attachments: attachments),
  );

  /// The outbox request of a message with [text] in [thread]: [kind] `voice` for a voice
  /// message (12 8.1), [attachments] for files, which make it wait for their upload entries
  /// (15 7.5, 12 3.4; brief M4-22 Design note 2). [id] is the entry ID, and so the
  /// `client_msg_id`, that a share brings (brief M5-06); null lets the outbox make one.
  /// [clearDraft] deletes the person's draft of the thread with the send; by default a text
  /// message does, because it is the typed draft.
  static OutboxRequest messageRequest(
    ThreadId thread,
    String text, {
    MessageId? replyTo,
    MessageAttachments? attachments,
    MessageKind kind = MessageKind.text,
    String? id,
    bool? clearDraft,
  }) => OutboxRequest(
    kind: MessageOps.createMessage,
    id: id,
    threadId: thread,
    dependsOn: attachments?.uploads ?? const [],
    payload: {
      if (kind != MessageKind.text) 'kind': kind.code,
      'content_md': text,
      // A voice message or a share is not the typed draft: the draft stays (04 14).
      if (clearDraft ?? kind == MessageKind.text) 'clear_draft': true,
      'reply_to_message_id': ?replyTo?.value,
      ...?attachments?.payload,
    },
  );

  /// Edits the confirmed [message] to [text] with `If-Match` of its version (04 13.3). When an
  /// own edit of the message is not confirmed yet, the new text joins it (`Outbox.edit`: in
  /// place, or a follow-up that takes the version the first edit returns), so the person's
  /// second edit never meets a `412` against the first.
  Future<void> edit(Message message, String text) async {
    final pending = await _pendingEdit(message);
    if (pending != null) {
      await _outbox.edit(pending, {'content_md': text});
      return;
    }
    await _enqueueEdit(message, text);
  }

  Future<String?> _pendingEdit(Message message) async {
    final dao = outboxDao;
    if (dao == null) return null;
    String? found;
    for (final e in await dao.entriesFor(message.threadId)) {
      if (e.kind != MessageOps.updateMessage ||
          e.state.isFinal ||
          e.state == OutboxState.failed) {
        continue;
      }
      if (decodePayload(e.payload)['message_id'] == message.id.value) {
        found = e.id;
      }
    }
    return found;
  }

  Future<OutboxEntry> _enqueueEdit(Message message, String text) =>
      _outbox.enqueue(
        OutboxRequest(
          kind: MessageOps.updateMessage,
          threadId: message.threadId,
          payload: {
            'message_id': message.id.value,
            'version': message.version.value,
            'content_md': text,
          },
          expectedVersion: message.version,
        ),
      );

  /// Edits the waiting or failed outbox entry [id] to [text] (04 13.3).
  Future<String> editEntry(String id, String text) =>
      _outbox.edit(id, {'content_md': text});

  /// Deletes the confirmed [message] (`deleteMessage`, idempotent, 04 13.3).
  Future<OutboxEntry> delete(Message message) => _outbox.enqueue(
    OutboxRequest(
      kind: MessageOps.deleteMessage,
      threadId: message.threadId,
      payload: {'message_id': message.id.value},
    ),
  );

  /// Removes the waiting entry [id], or queues the delete behind it when it was sent
  /// (04 13.3).
  Future<String?> deleteEntry(String id) => _outbox.remove(id);

  /// Adds or, with [remove], removes the reaction [emoji] of the reader on [message].
  Future<OutboxEntry> react(
    Message message,
    String emoji, {
    required bool remove,
  }) => _outbox.enqueue(
    OutboxRequest(
      kind: remove ? MessageOps.removeReaction : MessageOps.addReaction,
      threadId: message.threadId,
      payload: {'message_id': message.id.value, 'emoji': emoji},
    ),
  );

  /// Pins or unpins [message].
  Future<OutboxEntry> pin(Message message, {required bool pinned}) =>
      AppMessageOps.enqueue(
        engine,
        pinned ? AppMessageOps.pinMessage : AppMessageOps.unpinMessage,
        thread: message.threadId,
        payload: {'message_id': message.id.value},
      );

  /// Moves the read position of [thread] before [message] (`markThreadUnread` with
  /// `If-Match` of [readState], 04 14). Without a stored read state there is nothing to move
  /// back and null is returned.
  Future<OutboxEntry?> markUnread(Message message, ReadState? readState) async {
    if (readState == null) return null;
    return AppMessageOps.enqueue(
      engine,
      AppMessageOps.markThreadUnread,
      thread: message.threadId,
      payload: {'last_read_seq': message.threadSeq.value - 1},
      expectedVersion: readState.version,
    );
  }

  /// Marks [thread] read up to [seq] (`putReadState`; the server keeps the maximum, 04 14).
  Future<OutboxEntry> markRead(ThreadId thread, ThreadSeq seq) =>
      _outbox.enqueue(
        OutboxRequest(
          kind: PersonalOps.putReadState,
          threadId: thread,
          payload: {'last_read_seq': seq.value},
        ),
      );

  /// Starts a thread from [message] of the main history of [channel] (01 3.1).
  Future<OutboxEntry> startThread(ChannelId channel, Message message) =>
      ThreadOps.enqueueFromMessage(
        engine,
        channel: channel,
        message: message.id,
      );

  /// Marks [thread] done, or open again with [reopen] (01 6.1).
  Future<OutboxEntry> setDone(Thread thread, {required bool reopen}) =>
      AppMessageOps.enqueue(
        engine,
        reopen ? AppMessageOps.reopenThread : AppMessageOps.closeThread,
        thread: thread.id,
      );

  /// Sets the title of [thread] with `If-Match` of its version (01 6.7: editing makes it
  /// manual).
  Future<OutboxEntry> setTitle(Thread thread, String title) =>
      AppMessageOps.enqueue(
        engine,
        AppMessageOps.setThreadTitle,
        thread: thread.id,
        payload: {'title': title},
        expectedVersion: thread.version,
      );

  /// Moves [thread] into [target] with [reason]; the thread ID stays (01 6.7).
  Future<OutboxEntry> move(Thread thread, ChannelId target, String reason) =>
      AppMessageOps.enqueue(
        engine,
        AppMessageOps.moveThread,
        thread: thread.id,
        payload: {'target_channel_id': target.value, 'reason': reason},
      );

  /// Saves [text] as the personal template [name] (01 3.3).
  Future<OutboxEntry> saveTemplate(String name, String text) =>
      AppMessageOps.enqueue(
        engine,
        AppMessageOps.createPersonalTemplate,
        payload: {'name': name, 'content_md': text},
      );

  /// Sends the failed entry [id] again with a new ID (04 13.2 item 5).
  Future<String> retry(String id) => _outbox.retry(id);

  /// Discards the failed entry [id] (04 13.2 item 5).
  Future<void> discard(String id) => _outbox.discard(id);

  /// Keeps the person's text of the conflicting edit [id]: a new entry on the version of the
  /// current revision (04 13.3, docs/DECISIONS.md D-M4-20-3).
  Future<String> keepMine(String id, String text) =>
      _outbox.edit(id, {'content_md': text});
}
