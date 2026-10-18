// SPDX-License-Identifier: Apache-2.0

/// Appliers of `message.*`, `reaction.*` and `pin.*` (04 9 item 3, 15 5): messages by version,
/// reactions and pins with absolute values in `seq` order; every message change feeds the
/// search index and the unread counter (brief M4-14 Design note 2f).
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import '../dispatcher.dart';
import '../wire.dart';

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
/// removes the search row (04 2.2 item 5). Returns whether the message changed.
///
/// Runs inside the caller's transaction.
Future<bool> storeMessage(
  LocalDatabase db,
  MessageBundle b, {
  required PrincipalId? me,
}) async {
  final m = b.message;
  final dao = db.messagesDao;
  final deletedAt = m.deletedAt;
  if (deletedAt != null) {
    if (await dao.tombstone(m.id, deletedAt: deletedAt, version: m.version)) {
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
);

Future<void> _applyMessage(
  SyncContext sync,
  LocalDatabase db,
  AfterCommit after,
  MessageBundle b, {
  required bool created,
  required String? clientMsgId,
}) async {
  final m = b.message;
  final me = sync.me;
  final previous = created ? null : await db.messagesDao.byId(m.id);
  final applied = await storeMessage(db, b, me: me);
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
  }
  if (!applied) return;
  await _count(
    sync,
    db,
    after,
    created ? MessageCreated(m) : MessageUpdated(m, previous: previous),
  );
}

Future<void> _deleted(ApplyContext c) async {
  final p = c.payload;
  final id = MessageId.parse(str(p, 'message_id'));
  final previous = await c.db.messagesDao.byId(id);
  final version = c.resourceVersion;
  if (previous == null || version == null) return;
  final applied = await c.db.messagesDao.tombstone(
    id,
    deletedAt: time(p, 'deleted_at'),
    version: version,
  );
  if (!applied) return;
  await _count(
    c.sync,
    c.db,
    c.after,
    MessageDeleted(
      messageId: id,
      threadId: ThreadId.parse(str(p, 'thread_id')),
      threadSeq: ThreadSeq(integer(p, 'thread_seq')),
      previous: previous,
    ),
  );
}

/// Feeds the [UnreadCounter] after the commit with the read position and the import floor of
/// the thread as stored now (04 14).
Future<void> _count(
  SyncContext sync,
  LocalDatabase db,
  AfterCommit after,
  MessageEvent event,
) async {
  final me = sync.me;
  if (me == null) return;
  final read = await db.personalDao.readState(event.threadId);
  final thread = await db.threadsDao.byId(event.threadId);
  final lastRead = read?.lastReadSeq ?? const ThreadSeq(0);
  final floor = thread?.unreadFloorSeq ?? const ThreadSeq(0);
  after.steps.add(() {
    sync.counter.apply(
      event,
      me: me,
      lastReadSeq: lastRead,
      unreadFloorSeq: floor,
    );
    sync.countedThreads.add(event.threadId);
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
