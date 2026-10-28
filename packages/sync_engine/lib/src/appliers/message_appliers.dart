// SPDX-License-Identifier: Apache-2.0

/// Appliers of `message.*`, `reaction.*` and `pin.*` (04 9 item 3, 15 5): messages by version,
/// reactions and pins with absolute values in `seq` order; every message change feeds the
/// search index and the unread counter (brief M4-14 Design note 2f); a new message of a thread
/// this database does not hold loads that thread (R-SY-28, brief M4-40 Design note 5).
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import '../dispatcher.dart';
import '../wire.dart';
import 'thread_appliers.dart';

/// The appliers of this group by event type.
final Map<String, Applier> messageAppliers = {
  'message.created': (c) => _message(c, created: true),
  'message.updated': (c) => _message(c, created: false),
  'message.deleted': _deleted,
  'reaction.added': (c) => _reaction(c, added: true),
  'reaction.removed': (c) => _reaction(c, added: false),
  'pin.added': (c) => _pin(c, pinned: true),
  'pin.removed': (c) => _pin(c, pinned: false),
};

/// Stores [b] by the version rule with its author, files, attachments, reactions, components,
/// the mention of [me] and its search row; a tombstone (`deleted_at`) clears the content and
/// removes the search row (04 2.2 item 5) and the cached files of the message, whose paths go
/// to [onErased] for deleting the bytes after the commit (20 4.7). Returns whether the message
/// changed.
///
/// Runs inside the caller's transaction.
Future<bool> storeMessage(
  LocalDatabase db,
  MessageBundle b, {
  required PrincipalId? me,
  void Function(List<String> paths)? onErased,
}) async {
  final m = b.message;
  final dao = db.messagesDao;
  final deletedAt = m.deletedAt;
  if (deletedAt != null) {
    final erased = await dao.tombstoneWithFiles(
      m.id,
      deletedAt: deletedAt,
      version: m.version,
    );
    if (erased != null) {
      if (erased.isNotEmpty) onErased?.call(erased);
      return true;
    }
    if (await dao.byId(m.id) != null) return false;
    // Never stored here: the tombstone keeps its position (04 4).
    return dao.upsertIfNewer(m);
  }
  if (!await dao.upsertIfNewer(m)) return false;
  await db.principalsDao.put(b.author);
  for (final f in b.files) {
    await dao.putFile(f);
  }
  await dao.putAttachments(m.id, b.attachments);
  for (final r in b.reactions) {
    await dao.applyReactionSummary(r);
  }
  for (final component in b.components) {
    await dao.upsertComponentIfNewer(component);
  }
  if (me != null && m.mentionIds.contains(me)) {
    await dao.putMention(
      Mention(messageId: m.id, principalId: me, threadId: m.threadId),
    );
  }
  final channel = await db.structureDao.channelById(m.channelId);
  await db.searchDao.indexMessage(m, spaceId: channel?.spaceId);
  return true;
}

/// Stores a message that REST returned for a truncated event (04 2.2) in its own transaction,
/// with the gap rule, the counter, the typing end and the echo of `message.created`.
Future<void> storeFetchedMessage(
  SyncContext sync,
  MessageBundle b, {
  required bool created,
  String? clientMsgId,
  int? seq,
}) async {
  final after = AfterCommit();
  final m = b.message;
  await sync.db.transaction(() async {
    if (!await sync.holdsContent(sync.db, m.channelId, m.threadId)) return;
    await _applyMessage(
      sync,
      sync.db,
      after,
      b,
      created: created,
      clientMsgId: clientMsgId,
      seq: seq,
    );
  });
  for (final step in after.steps) {
    await step();
  }
  for (final read in after.reads) {
    await read();
  }
}

Future<void> _message(ApplyContext c, {required bool created}) => _applyMessage(
  c.sync,
  c.db,
  c.after,
  messageFromJson(obj(c.payload, 'message')),
  created: created,
  clientMsgId: c.clientMsgId,
  seq: c.event.seq,
);

Future<void> _applyMessage(
  SyncContext sync,
  LocalDatabase db,
  AfterCommit after,
  MessageBundle b, {
  required bool created,
  required String? clientMsgId,
  int? seq,
}) async {
  final m = b.message;
  final me = sync.me;
  final previous = created ? null : await db.messagesDao.byId(m.id);
  final applied = await storeMessage(
    db,
    b,
    me: me,
    onErased: (paths) => after.steps.add(() => sync.eraser.erase(paths)),
  );
  if (created) {
    // 15 7.3 item 4: a message that does not continue the loaded range opens a gap; the gap
    // is loaded through REST before the message shows (watchContiguous).
    if (await sync.gaps.place(db, m.threadId, m.threadSeq)) {
      after.reads.add(() => sync.gaps.fill(sync, m.threadId));
    }
    after.steps.add(() => sync.ephemeral.messageFrom(m.threadId, m.authorId));
    if (clientMsgId != null && (me == null || m.authorId == me)) {
      final id = MessageId.parse(clientMsgId);
      after.steps.add(() => sync.onEcho(id, m));
    }
    await _queueUnknownThread(sync, db, after, m);
  }
  if (!applied) return;
  await _count(
    sync,
    db,
    after,
    created ? MessageCreated(m) : MessageUpdated(m, previous: previous),
    seq: seq,
  );
}

/// R-SY-28 (04 6.3 item 4): the client fills the sidebar from the snapshot's `active_threads`
/// and keeps it current from `thread.*` and `message.created`, so a message of a thread this
/// database does not hold (not in the snapshot, created before the cursor and dropped by the
/// cache cleanup, or the main thread of a text channel) queues one `getThread` and the row
/// appears when it arrives (brief M4-40 Design note 5). [SyncContext.threadLoads] keeps it to
/// one load per thread.
Future<void> _queueUnknownThread(
  SyncContext sync,
  LocalDatabase db,
  AfterCommit after,
  Message m,
) async {
  final id = m.threadId;
  if (sync.threadLoads.contains(id) || await db.threadsDao.byId(id) != null) {
    return;
  }
  // After the commit, so a page that rolls back leaves no entry in threadLoads; the reads of a
  // commit start after all its steps, so the read queued here runs with them, and a second
  // message of the thread in the same page finds the entry and queues nothing.
  after.steps.add(() {
    if (sync.threadLoads.add(id)) {
      after.reads.add(() => _loadThread(sync, id, m.channelId));
    }
  });
}

/// Loads the thread [id] of [channelId] (`getThread`) and stores it by the version rule; a
/// failure is logged without content and the next message of the thread tries again.
Future<void> _loadThread(
  SyncContext sync,
  ThreadId id,
  ChannelId channelId,
) async {
  final db = sync.db;
  try {
    // Checked when the read runs: an earlier read or event may have stored the thread, or
    // loaded or removed its channel, since the message arrived.
    if (await db.threadsDao.byId(id) != null ||
        !await sync.holdsContent(db, channelId, id)) {
      return;
    }
    final thread = await sync.fetcher.thread(id);
    await db.transaction(() async {
      // Inside the write's transaction, so a removal during the read wins (04 8.3).
      if (await sync.holdsContent(db, thread.channelId, thread.id)) {
        await storeThread(db, thread);
      }
    });
  } on Object catch (e) {
    sync.log('unknown thread not loaded: ${e.runtimeType}');
  } finally {
    sync.threadLoads.remove(id);
  }
}

Future<void> _deleted(ApplyContext c) async {
  final p = c.payload;
  final id = MessageId.parse(str(p, 'message_id'));
  final previous = await c.db.messagesDao.byId(id);
  final version = c.resourceVersion;
  if (previous == null || version == null) return;
  final erased = await c.db.messagesDao.tombstoneWithFiles(
    id,
    deletedAt: time(p, 'deleted_at'),
    version: version,
  );
  if (erased == null) return;
  // 20 4.7: the cached bytes of the message's files go after the commit.
  if (erased.isNotEmpty) {
    c.after.steps.add(() => c.sync.eraser.erase(erased));
  }
  await _count(
    c.sync,
    c.db,
    c.after,
    seq: c.event.seq,
    MessageDeleted(
      messageId: id,
      threadId: ThreadId.parse(str(p, 'thread_id')),
      threadSeq: ThreadSeq(integer(p, 'thread_seq')),
      previous: previous,
    ),
  );
}

/// Feeds the [UnreadCounter] after the commit with the read position and the import floor of
/// the thread as stored now (04 14) and the global [seq] of the event; the counter skips an
/// event its server values hold already (D-M4-19-10).
Future<void> _count(
  SyncContext sync,
  LocalDatabase db,
  AfterCommit after,
  MessageEvent event, {
  int? seq,
}) async {
  final me = sync.me;
  if (me == null) return;
  final read = await db.personalDao.readState(event.threadId);
  final thread = await db.threadsDao.byId(event.threadId);
  final lastRead = read?.lastReadSeq ?? const ThreadSeq(0);
  final floor = thread?.unreadFloorSeq ?? const ThreadSeq(0);
  final channel = switch (event) {
    MessageCreated(:final message) ||
    MessageUpdated(:final message) => message.channelId,
    MessageDeleted(:final previous) =>
      thread?.channelId ?? previous?.channelId ?? read?.channelId,
  };
  after.steps.add(() {
    sync.counter.apply(
      event,
      me: me,
      lastReadSeq: lastRead,
      unreadFloorSeq: floor,
      at: seq == null ? null : GlobalSeq(seq),
    );
    sync.countedThreads.add(event.threadId);
    if (channel != null) sync.counterChannels[event.threadId] = channel;
    sync.counterChanges.add(event.threadId);
  });
}

Future<void> _reaction(ApplyContext c, {required bool added}) async {
  final p = c.payload;
  final emoji = str(p, 'emoji');
  // A redacted reaction carries an empty emoji and no count (04 2.2 item 5): nothing to keep.
  if (emoji.isEmpty) return;
  final messageId = MessageId.parse(str(p, 'message_id'));
  final principal = PrincipalId.parse(str(p, 'principal_id'));
  final bool me;
  if (principal == c.sync.me) {
    me = added;
  } else {
    final stored = await c.db.messagesDao.reactionsOf(messageId);
    me = stored.any((r) => r.emoji == emoji && r.me);
  }
  await c.db.messagesDao.applyReactionSummary(
    ReactionSummary(
      messageId: messageId,
      emoji: emoji,
      count: integer(p, 'count'),
      me: me,
    ),
  );
}

Future<void> _pin(ApplyContext c, {required bool pinned}) =>
    c.db.messagesDao.applyPin(
      MessageId.parse(str(obj(c.payload, 'pin'), 'message_id')),
      pinned: pinned,
    );
