// SPDX-License-Identifier: Apache-2.0

/// Access to messages and what hangs on them: reactions, pins, mentions, components, file
/// metadata, attachments and the loaded ranges of a thread (15 6.1, 04 4, 04 9).
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../mappers/files.dart';
import '../mappers/threads_messages.dart';
import '../models.dart';
import 'upsert.dart';

/// Data access object of messages; hands out domain types only.
final class MessagesDao {
  /// Creates the DAO on a database; use `LocalDatabase.messagesDao`.
  MessagesDao(this._db);

  final LocalDatabase _db;

  /// Stores [message] if its version is newer (04 9 item 3); returns whether it applied.
  Future<bool> upsertIfNewer(Message message) {
    final row = messageToCompanion(message);
    return upsertIfNewerVersion(
      _db,
      _db.messages,
      row,
      update: row,
      version: (t) => t.version,
    );
  }

  /// The message [id], or null.
  Future<Message?> byId(MessageId id) async {
    final row = await (_db.select(
      _db.messages,
    )..where((t) => t.id.equals(id.value))).getSingleOrNull();
    return row == null ? null : messageFromRow(row);
  }

  /// Up to [limit] messages of [threadId] before position [before] (all positions when null),
  /// in ascending `thread_seq`; the page that scrolling up loads.
  Future<List<Message>> pageBefore(
    ThreadId threadId,
    ThreadSeq? before,
    int limit,
  ) async {
    final rows =
        await (_db.select(_db.messages)
              ..where(
                (t) =>
                    t.threadId.equals(threadId.value) &
                    (before == null
                        ? const Constant(true)
                        : t.threadSeq.isSmallerThanValue(before.value)),
              )
              ..orderBy([
                (t) => OrderingTerm(
                  expression: t.threadSeq,
                  mode: OrderingMode.desc,
                ),
              ])
              ..limit(limit))
            .get();
    return [for (final row in rows.reversed) messageFromRow(row)];
  }

  /// The newest [limit] messages of [threadId] in ascending `thread_seq`.
  Stream<List<Message>> watchThread(ThreadId threadId, {required int limit}) =>
      (_db.select(_db.messages)
            ..where((t) => t.threadId.equals(threadId.value))
            ..orderBy([
              (t) => OrderingTerm(
                expression: t.threadSeq,
                mode: OrderingMode.desc,
              ),
            ])
            ..limit(limit))
          .map(messageFromRow)
          .watch()
          .map((page) => List.unmodifiable(page.reversed));

  /// The contiguous window of [threadId] (15 7.3 item 4): the newest [limit] messages of the
  /// loaded range that ends at the newest loaded message, the gap before that range and the
  /// gap up to the thread's `last_message_seq`. Messages stored after a gap stay hidden until
  /// their range is loaded.
  Stream<ContiguousWindow> watchContiguous(
    ThreadId threadId, {
    required int limit,
  }) => _db
      .customSelect(
        'SELECT 1',
        readsFrom: {_db.messages, _db.loadedRangesTable, _db.threads},
      )
      .watch()
      .asyncMap((_) => contiguous(threadId, limit: limit));

  /// One value of [watchContiguous].
  Future<ContiguousWindow> contiguous(
    ThreadId threadId, {
    required int limit,
  }) async {
    final ranges = (await loadedRanges(threadId)).ranges;
    if (ranges.isEmpty) return const ContiguousWindow(messages: []);
    final window = ranges.last;
    final rows =
        await (_db.select(_db.messages)
              ..where(
                (t) =>
                    t.threadId.equals(threadId.value) &
                    t.threadSeq.isBetweenValues(
                      window.from.value,
                      window.to.value,
                    ),
              )
              ..orderBy([
                (t) => OrderingTerm(
                  expression: t.threadSeq,
                  mode: OrderingMode.desc,
                ),
              ])
              ..limit(limit))
            .get();
    final before = ranges.length > 1 ? ranges[ranges.length - 2].to.value : 0;
    final thread = await (_db.select(
      _db.threads,
    )..where((t) => t.id.equals(threadId.value))).getSingleOrNull();
    final last = thread?.lastMessageSeq ?? window.to.value;
    return ContiguousWindow(
      messages: List.unmodifiable([
        for (final row in rows.reversed) messageFromRow(row),
      ]),
      gapBefore: window.from.value > before + 1
          ? SeqRange(ThreadSeq(before + 1), ThreadSeq(window.from.value - 1))
          : null,
      gapAfter: last > window.to.value
          ? SeqRange(window.to.next, ThreadSeq(last))
          : null,
    );
  }

  /// The loaded ranges of [threadId].
  Future<LoadedRanges> loadedRanges(ThreadId threadId) async {
    final rows =
        await (_db.select(_db.loadedRangesTable)
              ..where((t) => t.threadId.equals(threadId.value))
              ..orderBy([(t) => OrderingTerm(expression: t.fromSeq)]))
            .get();
    var ranges = LoadedRanges.empty;
    for (final row in rows) {
      ranges = ranges.add(ThreadSeq(row.fromSeq), ThreadSeq(row.toSeq));
    }
    return ranges;
  }

  /// Adds [from] to [to] to the loaded ranges of [threadId], merged through `LoadedRanges`
  /// (M4-08), in one transaction. Throws an [ArgumentError] for an invalid range.
  Future<LoadedRanges> addLoadedRange(
    ThreadId threadId,
    ThreadSeq from,
    ThreadSeq to,
  ) => _db.transaction(() async {
    final merged = (await loadedRanges(threadId)).add(from, to);
    await (_db.delete(
      _db.loadedRangesTable,
    )..where((t) => t.threadId.equals(threadId.value))).go();
    await _db.batch(
      (b) => b.insertAll(_db.loadedRangesTable, [
        for (final r in merged.ranges)
          LoadedRangesTableCompanion.insert(
            threadId: threadId.value,
            fromSeq: r.from.value,
            toSeq: r.to.value,
          ),
      ]),
    );
    return merged;
  });

  /// Applies the absolute [summary] of `reaction.*` (04 9 item 3) without touching the
  /// message version; a count of zero removes the row.
  Future<void> applyReactionSummary(ReactionSummary summary) async {
    if (summary.count <= 0) {
      await (_db.delete(_db.reactionsSummary)..where(
            (t) =>
                t.messageId.equals(summary.messageId.value) &
                t.emoji.equals(summary.emoji),
          ))
          .go();
      return;
    }
    await _db
        .into(_db.reactionsSummary)
        .insertOnConflictUpdate(
          ReactionsSummaryCompanion.insert(
            messageId: summary.messageId.value,
            emoji: summary.emoji,
            count: summary.count,
            me: summary.me,
          ),
        );
  }

  /// The reaction summaries of [messageId], ordered by emoji.
  Stream<List<ReactionSummary>> watchReactions(MessageId messageId) =>
      (_db.select(_db.reactionsSummary)
            ..where((t) => t.messageId.equals(messageId.value))
            ..orderBy([(t) => OrderingTerm(expression: t.emoji)]))
          .map(reactionFromRow)
          .watch();

  /// Sets the absolute pin state of `pin.*` (04 9 item 3) without touching `version`.
  Future<void> applyPin(MessageId messageId, {required bool pinned}) async {
    await (_db.update(_db.messages)..where((t) => t.id.equals(messageId.value)))
        .write(MessagesCompanion(pinned: Value(pinned)));
  }

  /// Turns [messageId] into a tombstone if [version] is newer: the content and the mentions
  /// go, `thread_seq` stays (03 6.5); its reactions, components, attachments and search rows
  /// are deleted in the same transaction. Returns whether it applied.
  Future<bool> tombstone(
    MessageId messageId, {
    required DateTime deletedAt,
    required ResourceVersion version,
  }) => _db.transaction(() async {
    final changed =
        await (_db.update(_db.messages)..where(
              (t) =>
                  t.id.equals(messageId.value) &
                  t.version.isSmallerThanValue(version.value),
            ))
            .write(
              MessagesCompanion(
                contentMd: const Value(''),
                contentTruncated: const Value(false),
                mentionIds: const Value('[]'),
                pinned: const Value(false),
                deletedAt: Value(deletedAt),
                version: Value(version.value),
              ),
            );
    if (changed == 0) return false;
    final id = messageId.value;
    await (_db.delete(
      _db.reactionsSummary,
    )..where((t) => t.messageId.equals(id))).go();
    await (_db.delete(
      _db.myMentions,
    )..where((t) => t.messageId.equals(id))).go();
    await (_db.delete(
      _db.messageComponents,
    )..where((t) => t.messageId.equals(id))).go();
    await (_db.delete(
      _db.messageAttachments,
    )..where((t) => t.messageId.equals(id))).go();
    await _db.customUpdate(
      "DELETE FROM search_index WHERE kind = 'message' AND target_id = ?",
      variables: [Variable<String>(id)],
      updates: {_db.searchIndex},
      updateKind: UpdateKind.delete,
    );
    return true;
  });

  /// Stores a mention of the person themselves (15 6.1 `my_mentions`).
  Future<void> putMention(Mention mention) async {
    await _db
        .into(_db.myMentions)
        .insertOnConflictUpdate(
          MyMentionsCompanion.insert(
            messageId: mention.messageId.value,
            principalId: mention.principalId.value,
            threadId: mention.threadId.value,
          ),
        );
  }

  /// The person's mentions in [threadId].
  Future<List<Mention>> mentionsIn(ThreadId threadId) async {
    final rows = await (_db.select(
      _db.myMentions,
    )..where((t) => t.threadId.equals(threadId.value))).get();
    return [for (final row in rows) mentionFromRow(row)];
  }

  /// Stores [component] if its version is newer; returns whether it applied.
  Future<bool> upsertComponentIfNewer(MessageComponent component) {
    final row = componentToCompanion(component);
    return upsertIfNewerVersion(
      _db,
      _db.messageComponents,
      row,
      update: row,
      version: (t) => t.version,
    );
  }

  /// The components of [messageId] in their order.
  Future<List<MessageComponent>> componentsOf(MessageId messageId) async {
    final rows =
        await (_db.select(_db.messageComponents)
              ..where((t) => t.messageId.equals(messageId.value))
              ..orderBy([(t) => OrderingTerm(expression: t.position)]))
            .get();
    return [for (final row in rows) componentFromRow(row)];
  }

  /// Stores the metadata of [file] (03 `files` has no version; the caller orders the writes).
  Future<void> putFile(FileMeta file) async {
    await _db.into(_db.files).insertOnConflictUpdate(fileToCompanion(file));
  }

  /// The metadata of file [id], or null.
  Future<FileMeta?> fileById(FileId id) async {
    final row = await (_db.select(
      _db.files,
    )..where((t) => t.id.equals(id.value))).getSingleOrNull();
    return row == null ? null : fileFromRow(row);
  }

  /// Replaces the attachments of [messageId] with [attachments] in one transaction.
  Future<void> putAttachments(
    MessageId messageId,
    List<MessageAttachment> attachments,
  ) => _db.transaction(() async {
    await (_db.delete(
      _db.messageAttachments,
    )..where((t) => t.messageId.equals(messageId.value))).go();
    await _db.batch(
      (b) => b.insertAll(
        _db.messageAttachments,
        attachments.map(attachmentToCompanion),
      ),
    );
  });

  /// The attachments of [messageId] in their order.
  Future<List<MessageAttachment>> attachmentsOf(MessageId messageId) async {
    final rows =
        await (_db.select(_db.messageAttachments)
              ..where((t) => t.messageId.equals(messageId.value))
              ..orderBy([(t) => OrderingTerm(expression: t.position)]))
            .get();
    return [for (final row in rows) attachmentFromRow(row)];
  }
}
