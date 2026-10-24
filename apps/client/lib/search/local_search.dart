// SPDX-License-Identifier: Apache-2.0

/// The offline search of the search view (docs/spec/14-search.md 6, brief M4-23 Design note 1):
/// `SearchDao.search` of `local_store` over the FTS5 index of the cached messages, thread
/// titles and transcripts, with the path, author and time read from the local database. The
/// index holds only what this device was allowed to see and loses the rows of removed targets
/// (04 8.3), so the hits need no permission check here (15 1 principle 1).
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart' as store;

import 'search_port.dart';

/// The [LocalSearchPort] over [db].
///
/// Example: `LocalSearch(db).search(SearchRequest(q: 'deploy'), limit: 20)`.
final class LocalSearch implements LocalSearchPort {
  /// Creates the search over [db].
  const LocalSearch(this.db);

  /// The local database of the signed-in instance.
  final store.LocalDatabase db;

  /// Up to [limit] index rows for the words of `request.q` (see [localQueryOf]), most relevant
  /// first, kept where they match the filters of [request] that the local database can check:
  /// the type (messages, thread titles, transcripts for files), spaces, channels, thread,
  /// author, time, attachment and thread state. A row whose object is no longer stored is
  /// left out.
  @override
  Future<List<SearchResultItem>> search(
    SearchRequest request, {
    required int limit,
  }) async {
    final query = localQueryOf(request.q);
    if (query.isEmpty) return const [];
    final rows = await db.searchDao.search(query, limit: limit);
    final items = <SearchResultItem>[];
    for (final row in rows) {
      if (!_typeMatches(row.kind, request.type)) continue;
      if (!_placeMatches(row, request)) continue;
      final item = await _item(row, request);
      if (item != null) items.add(item);
    }
    return items;
  }

  static bool _typeMatches(store.SearchKind kind, SearchType type) =>
      switch (type) {
        SearchType.messages => kind == store.SearchKind.message,
        SearchType.threads => kind == store.SearchKind.threadTitle,
        SearchType.files => kind == store.SearchKind.transcript,
      };

  static bool _placeMatches(store.SearchHit row, SearchRequest request) {
    if (request.spaceIds.isNotEmpty &&
        !request.spaceIds.contains(row.spaceId)) {
      return false;
    }
    if (request.channelIds.isNotEmpty &&
        !request.channelIds.contains(row.channelId)) {
      return false;
    }
    final thread = request.threadId;
    return thread == null || row.threadId == thread;
  }

  Future<SearchResultItem?> _item(
    store.SearchHit row,
    SearchRequest request,
  ) async {
    final threadId = row.threadId;
    final thread = threadId == null ? null : await db.threadsDao.byId(threadId);
    final state = request.threadState;
    if (state != null && thread?.state != state) return null;
    final channelId = row.channelId;
    final channel = channelId == null
        ? null
        : await db.structureDao.channelById(channelId);
    final spaceId = row.spaceId ?? channel?.spaceId;
    final space = spaceId == null
        ? null
        : await db.structureDao.spaceById(spaceId);
    final path = [
      for (final part in [space?.name, channel?.name, thread?.title])
        if (part != null && part.isNotEmpty) part,
    ];
    final segments = segmentsOfSnippet(row.snippet);
    switch (row.kind) {
      case store.SearchKind.message:
        final id = MessageId.parse(row.targetId);
        final message = await db.messagesDao.byId(id);
        if (message == null) return null;
        if (!_authorAndTimeMatch(
          request,
          message.authorId,
          message.createdAt,
        )) {
          return null;
        }
        if (request.hasFile != null &&
            (await db.messagesDao.attachmentsOf(id)).isNotEmpty !=
                request.hasFile) {
          return null;
        }
        return SearchResultItem(
          id: row.targetId,
          type: SearchHitType.message,
          path: path,
          segments: segments,
          threadId: message.threadId,
          messageId: id,
          channelId: message.channelId,
          authorId: message.authorId,
          at: message.createdAt,
          local: true,
        );
      case store.SearchKind.threadTitle:
        if (thread == null || !_timeMatches(request, thread.createdAt)) {
          return null;
        }
        return SearchResultItem(
          id: row.targetId,
          type: SearchHitType.thread,
          path: path,
          segments: segments,
          threadId: thread.id,
          channelId: thread.channelId,
          authorId: thread.createdBy,
          at: thread.createdAt,
          local: true,
        );
      case store.SearchKind.transcript:
        final file = await db.messagesDao.fileById(FileId.parse(row.targetId));
        if (file != null &&
            !_authorAndTimeMatch(request, file.uploadedBy, file.createdAt)) {
          return null;
        }
        return SearchResultItem(
          id: row.targetId,
          type: SearchHitType.transcript,
          path: path,
          segments: segments,
          threadId: threadId,
          channelId: channelId,
          authorId: file?.uploadedBy,
          title: file?.originalName,
          at: file?.createdAt,
          local: true,
        );
    }
  }

  static bool _authorAndTimeMatch(
    SearchRequest request,
    PrincipalId author,
    DateTime at,
  ) {
    final wanted = request.authorId;
    return (wanted == null || wanted == author) && _timeMatches(request, at);
  }

  /// The half-open interval of 14 4.4: `after` inclusive, `before` exclusive.
  static bool _timeMatches(SearchRequest request, DateTime at) {
    final after = request.after;
    final before = request.before;
    return (after == null || !at.isBefore(after)) &&
        (before == null || at.isBefore(before));
  }
}

/// The words of the server query [q] that the local index can look for: `SearchDao.search`
/// requires every word (as a word or a prefix) and takes operators as text, so a negated word
/// (`-draft`) and the operator `or` are left out instead of turning into required words
/// (docs/decisions.d/M4-23.md). Quotes are dropped by the DAO.
///
/// Example: `localQueryOf('"resume window" -draft or heartbeat')` is
/// `"resume window" heartbeat`.
String localQueryOf(String q) => q
    .split(RegExp(r'\s+'))
    .where((w) => w.isNotEmpty && !w.startsWith('-') && w.toLowerCase() != 'or')
    .join(' ');

/// The segments of a local [snippet]: the parts between `SearchDaoMarks.open` and
/// `SearchDaoMarks.close` are matches, the rest is text; empty parts are left out. The markers
/// are control characters that never occur in Markdown, so nothing needs unescaping, and the
/// text is never interpreted (14 4.6).
///
/// Example: `segmentsOfSnippet('a \u0002b\u0003 c')` is `a `, `b` (match), ` c`.
List<SearchSegment> segmentsOfSnippet(String snippet) {
  final segments = <SearchSegment>[];
  final buffer = StringBuffer();
  var match = false;
  void flush() {
    if (buffer.isNotEmpty) {
      segments.add(SearchSegment(text: buffer.toString(), match: match));
      buffer.clear();
    }
  }

  for (final rune in snippet.runes) {
    final char = String.fromCharCode(rune);
    if (char == store.SearchDaoMarks.open ||
        char == store.SearchDaoMarks.close) {
      flush();
      match = char == store.SearchDaoMarks.open;
    } else {
      buffer.write(char);
    }
  }
  flush();
  return segments;
}
