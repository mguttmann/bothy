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
  /// Creates the DAO on a database; use `LocalDatabase.searchDao`.
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

  /// The voice files with a done transcript that a stored, non-deleted message attaches and
  /// that have no transcript row yet, on every change of the files, attachments, messages,
  /// channels or the index (14 6: the index covers the transcripts in the cache). The text is
  /// not stored locally (15 6.1 `files`), so the caller fetches it (`getTranscript`) and hands
  /// it to [indexTranscript]. A file attached by several messages comes once, with the
  /// location of the earliest message by ID.
  Stream<List<PendingTranscript>> watchTranscriptsToIndex() => _db
      .customSelect(
        'SELECT f.id AS file_id, m.thread_id, m.channel_id, c.space_id '
        'FROM files f '
        'JOIN message_attachments a ON a.file_id = f.id '
        'JOIN messages m ON m.id = a.message_id AND m.deleted_at IS NULL '
        'LEFT JOIN channels c ON c.id = m.channel_id '
        "WHERE f.transcript_status = 'done' AND NOT EXISTS ("
        "SELECT 1 FROM search_index s WHERE s.kind = 'transcript' AND s.target_id = f.id) "
        'ORDER BY f.id, m.id',
        readsFrom: {
          _db.files,
          _db.messageAttachments,
          _db.messages,
          _db.channels,
          _db.searchIndex,
        },
      )
      .watch()
      .map((rows) {
        final seen = <String>{};
        return [
          for (final r in rows)
            if (seen.add(r.read<String>('file_id')))
              PendingTranscript(
                fileId: FileId.parse(r.read<String>('file_id')),
                threadId: ThreadId.parse(r.read<String>('thread_id')),
                channelId: ChannelId.parse(r.read<String>('channel_id')),
                spaceId: optional(
                  r.readNullable<String>('space_id'),
                  SpaceId.parse,
                ),
              ),
        ];
      });

  /// Deletes the transcript rows whose file is not stored, has no done transcript or is not
  /// attached to a stored, non-deleted message any more: a deleted voice message must not stay
  /// findable offline (04 2.2 item 5, 14 6). Returns the number of deleted rows; with nothing
  /// to delete it writes nothing, so a watcher of the index is not woken.
  Future<int> pruneTranscripts() => _db.transaction(() async {
    final stale = await _db
        .customSelect(
          "SELECT DISTINCT target_id FROM search_index WHERE kind = 'transcript' "
          'AND target_id NOT IN ('
          'SELECT f.id FROM files f '
          'JOIN message_attachments a ON a.file_id = f.id '
          'JOIN messages m ON m.id = a.message_id AND m.deleted_at IS NULL '
          "WHERE f.transcript_status = 'done')",
        )
        .get();
    final ids = [for (final r in stale) r.read<String>('target_id')];
    if (ids.isEmpty) return 0;
    return _db.customUpdate(
      "DELETE FROM search_index WHERE kind = 'transcript' "
      'AND target_id IN ${placeholders(ids.length)}',
      variables: textVariables(ids),
      updates: {_db.searchIndex},
      updateKind: UpdateKind.delete,
    );
  });

  /// Up to [limit] hits for the words of [query] after skipping [offset] hits, most relevant
  /// first (`bm25`), each with a `snippet` that marks the matches with [SearchDaoMarks]. Every
  /// word must occur, as a word or a word prefix; FTS5 operators in [query] are taken as plain
  /// text. An empty query finds nothing.
  ///
  /// The place filters run in SQL before the limit, so a filtered search still returns the best
  /// [limit] rows of the filtered set (14 6): [kinds] keeps only these kinds, [spaceIds] and
  /// [channelIds] only rows of these spaces or channels, [threadId] only rows of that thread; an
  /// empty set does not filter.
  Future<List<SearchHit>> search(
    String query, {
    required int limit,
    int offset = 0,
    Set<SearchKind> kinds = const {},
    Set<SpaceId> spaceIds = const {},
    Set<ChannelId> channelIds = const {},
    ThreadId? threadId,
  }) async {
    final words = query
        .split(RegExp(r'\s+'))
        .map((w) => w.replaceAll('"', ''))
        .where((w) => w.isNotEmpty)
        .toList();
    if (words.isEmpty) return const [];
    final match = [for (final w in words) '"$w"*'].join(' ');
    final where = StringBuffer('search_index MATCH ?');
    final variables = <Variable<Object>>[
      Variable<String>(SearchDaoMarks.open),
      Variable<String>(SearchDaoMarks.close),
      Variable<String>(SearchDaoMarks.ellipsis),
      Variable<String>(match),
    ];
    void inList(String column, Iterable<String> values) {
      if (values.isEmpty) return;
      where.write(' AND $column IN ${placeholders(values.length)}');
      variables.addAll(textVariables(values));
    }

    inList('kind', [for (final k in kinds) k.code]);
    inList('space_id', [for (final id in spaceIds) id.value]);
    inList('channel_id', [for (final id in channelIds) id.value]);
    if (threadId != null) inList('thread_id', [threadId.value]);
    variables
      ..add(Variable<int>(limit))
      ..add(Variable<int>(offset));
    final rows = await _db
        .customSelect(
          'SELECT kind, target_id, thread_id, channel_id, space_id, '
          'snippet(search_index, 5, ?, ?, ?, 16) AS snippet, '
          'bm25(search_index) AS rank '
          'FROM search_index WHERE $where '
          'ORDER BY rank LIMIT ? OFFSET ?',
          variables: variables,
          readsFrom: {_db.searchIndex},
        )
        .get();
    return [
      for (final row in rows)
        SearchHit(
          kind: SearchKind.fromCode(row.read<String>('kind')),
          targetId: row.read<String>('target_id'),
          threadId: optional(
            _orNull(row.read<String>('thread_id')),
            ThreadId.parse,
          ),
          channelId: optional(
            _orNull(row.read<String>('channel_id')),
            ChannelId.parse,
          ),
          spaceId: optional(
            _orNull(row.read<String>('space_id')),
            SpaceId.parse,
          ),
          snippet: row.readNullable<String>('snippet') ?? '',
          rank: row.read<double>('rank'),
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
