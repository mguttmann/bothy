// SPDX-License-Identifier: Apache-2.0

/// Offline search over the cached messages, thread titles and transcripts (14 6, FTS5 in
/// `search.drift`, Design note 5). The index holds only what the cache holds; removal deletes
/// its rows with the targets (04 8.3).
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../mappers/codec.dart';
import '../models.dart';
import 'upsert.dart';

/// Data access object of `search_index`.
final class SearchDao {
  /// Creates the DAO on [db]; use `LocalDatabase.searchDao`.
  SearchDao(this._db);

  final LocalDatabase _db;

  /// Indexes the text of [message] (replacing an earlier row of it); a tombstone or an empty
  /// text removes the row. [spaceId] is the space of the channel, null for direct messages.
  Future<void> indexMessage(Message message, {SpaceId? spaceId}) => _put(
    kind: SearchKind.message,
    targetId: message.id.value,
    threadId: message.threadId.value,
    channelId: message.channelId.value,
    spaceId: spaceId?.value,
    body: message.deletedAt == null ? message.contentMd : '',
  );

  /// Indexes the title of [thread]; [spaceId] as for [indexMessage].
  Future<void> indexThreadTitle(Thread thread, {SpaceId? spaceId}) => _put(
    kind: SearchKind.threadTitle,
    targetId: thread.id.value,
    threadId: thread.id.value,
    channelId: thread.channelId.value,
    spaceId: spaceId?.value,
    body: thread.title,
  );

  /// Indexes the transcript [text] of the voice file [fileId] attached in [threadId].
  Future<void> indexTranscript({
    required FileId fileId,
    required ThreadId threadId,
    required ChannelId channelId,
    required String text,
    SpaceId? spaceId,
  }) => _put(
    kind: SearchKind.transcript,
    targetId: fileId.value,
    threadId: threadId.value,
    channelId: channelId.value,
    spaceId: spaceId?.value,
    body: text,
  );

  /// Deletes every row that belongs to one of [targets]: a space, channel or thread with
  /// everything below it. Host targets have no rows.
  Future<void> removeForTargets(List<TargetRef> targets) async {
    final columns = {
      RemovedTargetKind.space: 'space_id',
      RemovedTargetKind.channel: 'channel_id',
      RemovedTargetKind.thread: 'thread_id',
    };
    for (final entry in columns.entries) {
      final ids = [
        for (final t in targets)
          if (t.kind == entry.key) t.id,
      ];
      if (ids.isEmpty) continue;
      await _db.customUpdate(
        'DELETE FROM search_index WHERE ${entry.value} IN ${placeholders(ids.length)}',
        variables: textVariables(ids),
        updates: {_db.searchIndex},
        updateKind: UpdateKind.delete,
      );
    }
  }

  /// Up to [limit] hits for the words of [query], most relevant first (`bm25`), each with a
  /// `snippet` that marks the matches with [SearchDaoMarks]. Every word must occur, as a word or
  /// a word prefix; FTS5 operators in [query] are taken as plain text. An empty query finds
  /// nothing.
  Future<List<SearchHit>> search(String query, {required int limit}) async {
    final words = query
        .split(RegExp(r'\s+'))
        .map((w) => w.replaceAll('"', ''))
        .where((w) => w.isNotEmpty)
        .toList();
    if (words.isEmpty) return const [];
    final match = [for (final w in words) '"$w"*'].join(' ');
    final rows = await _db
        .searchIndexQuery(
          SearchDaoMarks.open,
          SearchDaoMarks.close,
          SearchDaoMarks.ellipsis,
          match,
          limit,
        )
        .get();
    return [
      for (final row in rows)
        SearchHit(
          kind: SearchKind.fromCode(row.kind),
          targetId: row.targetId,
          threadId: optional(_orNull(row.threadId), ThreadId.parse),
          channelId: optional(_orNull(row.channelId), ChannelId.parse),
          spaceId: optional(_orNull(row.spaceId), SpaceId.parse),
          snippet: row.snippet ?? '',
          rank: row.rank,
        ),
    ];
  }

  Future<void> _put({
    required SearchKind kind,
    required String targetId,
    required String threadId,
    required String channelId,
    required String? spaceId,
    required String body,
  }) => _db.transaction(() async {
    await _db.customUpdate(
      'DELETE FROM search_index WHERE kind = ? AND target_id = ?',
      variables: [Variable<String>(kind.code), Variable<String>(targetId)],
      updates: {_db.searchIndex},
      updateKind: UpdateKind.delete,
    );
    if (body.isEmpty) return;
    await _db
        .into(_db.searchIndex)
        .insert(
          SearchIndexCompanion.insert(
            kind: kind.code,
            targetId: targetId,
            threadId: threadId,
            channelId: channelId,
            // FTS5 columns are text; an absent space is stored as an empty text.
            spaceId: spaceId ?? '',
            body: body,
          ),
        );
  });

  static String? _orNull(String text) => text.isEmpty ? null : text;
}
