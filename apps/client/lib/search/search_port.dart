// SPDX-License-Identifier: Apache-2.0

/// The ports of the search view D7 (brief M4-23 Design note 1): the server search
/// (`search`, docs/spec/14-search.md 4 and 11) and the offline search over the local index
/// (14 6), with the request, the page and the hit as the view shows them. The API adapter maps
/// the generated client and its problems (`validation_failed` with `empty` and `too_long`,
/// `cursor_invalid`) to these types, so the controller and its tests never see Dio.
library;

import 'package:api_client/api_client.dart' as rest;
import 'package:built_collection/built_collection.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';

/// The result type of the server search (14 11: `type`, default `messages`).
enum SearchType {
  /// Messages.
  messages('messages'),

  /// Thread titles.
  threads('threads'),

  /// Files: names, transcripts and extracted text (14 2).
  files('files');

  const SearchType(this.code);

  /// The value of the query parameter `type`.
  final String code;
}

/// The order of the hits (14 4.5).
enum SearchSort {
  /// `relevance`, the default.
  relevance('relevance'),

  /// `newest`: by `created_at` descending.
  newest('newest');

  const SearchSort(this.code);

  /// The value of the query parameter `sort`.
  final String code;
}

/// One request of the server or the local search: the free text `q` and the filters of 14 4.4
/// that apply to [type]. The cursor is not part of it, so the same request names every page.
final class SearchRequest {
  /// Creates the request.
  const SearchRequest({
    required this.q,
    this.type = SearchType.messages,
    this.sort = SearchSort.relevance,
    this.spaceIds = const [],
    this.channelIds = const [],
    this.threadId,
    this.authorId,
    this.after,
    this.before,
    this.hasFile,
    this.threadState,
  });

  /// The free text in the syntax of `websearch_to_tsquery` (14 4.1), without the notation.
  final String q;

  /// The result type.
  final SearchType type;

  /// The order.
  final SearchSort sort;

  /// `space_id`, several allowed.
  final List<SpaceId> spaceIds;

  /// `channel_id`, several allowed.
  final List<ChannelId> channelIds;

  /// `thread_id`: the in-thread search of M4-24.
  final ThreadId? threadId;

  /// `author_id` (messages and files).
  final PrincipalId? authorId;

  /// `after`: created at or after.
  final DateTime? after;

  /// `before`: created before.
  final DateTime? before;

  /// `has_file` (messages).
  final bool? hasFile;

  /// `thread_state` (threads and messages): [ThreadState.open] or [ThreadState.closed].
  final ThreadState? threadState;
}

/// What a hit stands for.
enum SearchHitType {
  /// A message.
  message,

  /// A thread title.
  thread,

  /// A file of the server search.
  file,

  /// The transcript of a voice message in the local index (14 6).
  transcript,
}

/// One part of an excerpt: [text] exactly as delivered, [match] whether it is a match
/// (14 4.6). The text is never parsed: HTML or Markdown in it stays text.
final class SearchSegment {
  /// Creates the segment.
  const SearchSegment({required this.text, required this.match});

  /// The text.
  final String text;

  /// Whether the text is a match.
  final bool match;

  @override
  bool operator ==(Object other) =>
      other is SearchSegment && other.text == text && other.match == match;

  @override
  int get hashCode => Object.hash(text, match);
}

/// One hit as the view shows it: the place (path), author, time and excerpt, and where
/// selecting it leads.
final class SearchResultItem {
  /// Creates the hit.
  const SearchResultItem({
    required this.id,
    required this.type,
    required this.path,
    required this.segments,
    this.threadId,
    this.messageId,
    this.channelId,
    this.authorId,
    this.authorName,
    this.title,
    this.at,
    this.local = false,
  });

  /// The ID of the message, thread or file; unique within one list.
  final String id;

  /// What the hit stands for.
  final SearchHitType type;

  /// Space, channel and thread title, as far as they are known; empty parts are left out.
  final List<String> path;

  /// The excerpt with the matches.
  final List<SearchSegment> segments;

  /// The thread selecting the hit opens, or null (a server file hit names no thread).
  final ThreadId? threadId;

  /// The message selecting the hit jumps to, or null.
  final MessageId? messageId;

  /// The channel of the hit, or null.
  final ChannelId? channelId;

  /// The author or uploader, or null.
  final PrincipalId? authorId;

  /// The author's display name as the source delivered it, or null (the view then names the
  /// stored principal of [authorId]).
  final String? authorName;

  /// The file name of a file or transcript hit, or null.
  final String? title;

  /// When the object was created, or null.
  final DateTime? at;

  /// Whether the hit comes from the local index (14 6) rather than the server.
  final bool local;
}

/// One page of the server search.
final class SearchResultPage {
  /// Creates the page.
  const SearchResultPage({
    required this.items,
    required this.nextCursor,
    required this.candidatesCapped,
  });

  /// The hits in the server's order.
  final List<SearchResultItem> items;

  /// The opaque cursor of the next page, null on the last page (14 4.7).
  final String? nextCursor;

  /// Whether `search.max_candidates` was reached (14 4.5).
  final bool candidatesCapped;
}

/// Why a server search failed, as far as the view treats it differently.
enum SearchFailureKind {
  /// `400 validation_failed` on `q` with `empty`.
  queryEmpty,

  /// `400 validation_failed` on `q` with `too_long`.
  queryTooLong,

  /// `400 cursor_invalid`: the cursor does not belong to the request (14 4.7).
  cursorInvalid,

  /// No connection: the request did not reach the server.
  offline,

  /// Any other failure.
  failed,
}

/// The failure of a [SearchPort] call.
final class SearchFailure implements Exception {
  /// Creates the failure of [kind].
  const SearchFailure(this.kind);

  /// Why the search failed.
  final SearchFailureKind kind;

  @override
  String toString() => 'SearchFailure(${kind.name})';
}

/// The server search (operation `search`).
abstract interface class SearchPort {
  /// One page of hits for [request], the first page without [cursor].
  ///
  /// Throws a [SearchFailure].
  Future<SearchResultPage> search(SearchRequest request, {String? cursor});
}

/// The offline search over the local index (14 6).
abstract interface class LocalSearchPort {
  /// Up to [limit] local hits for [request], most relevant first, each with `local` set.
  Future<List<SearchResultItem>> search(
    SearchRequest request, {
    required int limit,
  });
}

/// The [SearchPort] over the generated client of the signed-in instance.
///
/// Example: `ApiSearchPort(services.auth.api).search(SearchRequest(q: 'deploy'))`.
final class ApiSearchPort implements SearchPort {
  /// Creates the port over [api].
  const ApiSearchPort(this.api);

  /// The REST client.
  final rest.ApiClient api;

  @override
  Future<SearchResultPage> search(
    SearchRequest request, {
    String? cursor,
  }) async {
    try {
      final response = await api.getSearchApi().search(
        q: request.q,
        cursor: cursor,
        type: request.type.code,
        sort: request.sort.code,
        spaceId: request.spaceIds.isEmpty
            ? null
            : BuiltList([for (final id in request.spaceIds) id.value]),
        channelId: request.channelIds.isEmpty
            ? null
            : BuiltList([for (final id in request.channelIds) id.value]),
        threadId: request.threadId?.value,
        authorId: request.authorId?.value,
        after: request.after,
        before: request.before,
        hasFile: request.hasFile,
        threadState: switch (request.threadState) {
          ThreadState.open => rest.ThreadState.open,
          ThreadState.closed => rest.ThreadState.closed,
          _ => null,
        },
      );
      return searchPageFromRest(response.data!);
    } on Object catch (e) {
      throw searchFailureOf(e);
    }
  }
}

/// The [SearchFailure] of [error], an error of the generated client.
///
/// `400 validation_failed` with a field error on `q` of code `empty` or `too_long` and
/// `400 cursor_invalid` keep their meaning (14 4.1, 4.7); a request that never got a response
/// (connection error or timeout) is [SearchFailureKind.offline]; anything else is
/// [SearchFailureKind.failed].
SearchFailure searchFailureOf(Object error) {
  if (error is SearchFailure) return error;
  final problem = rest.ApiProblem.of(error);
  if (problem != null) {
    if (problem.code == 'cursor_invalid') {
      return const SearchFailure(SearchFailureKind.cursorInvalid);
    }
    if (problem.code == 'validation_failed') {
      for (final e in problem.errors) {
        if (e.pointer != '#query/q') continue;
        if (e.code == 'empty') {
          return const SearchFailure(SearchFailureKind.queryEmpty);
        }
        if (e.code == 'too_long') {
          return const SearchFailure(SearchFailureKind.queryTooLong);
        }
      }
    }
    return const SearchFailure(SearchFailureKind.failed);
  }
  if (error is DioException &&
      error.response == null &&
      (error.type == DioExceptionType.connectionError ||
          error.type == DioExceptionType.connectionTimeout ||
          error.type == DioExceptionType.sendTimeout ||
          error.type == DioExceptionType.receiveTimeout)) {
    return const SearchFailure(SearchFailureKind.offline);
  }
  return const SearchFailure(SearchFailureKind.failed);
}

/// The page of the view for the server's [page]: path from `space_name`, `channel_name` and
/// `thread_title`, the object's IDs, author and time, and the `segments` as they came. A hit
/// of a kind this version does not know is left out.
SearchResultPage searchPageFromRest(rest.SearchHitPage page) =>
    SearchResultPage(
      items: [for (final hit in page.items) ?_itemOf(hit)],
      nextCursor: page.nextCursor,
      candidatesCapped: page.candidatesCapped ?? false,
    );

SearchResultItem? _itemOf(rest.SearchHit hit) {
  final path = [
    for (final part in [hit.spaceName, hit.channelName, hit.threadTitle])
      if (part != null && part.isNotEmpty) part,
  ];
  final segments = [
    for (final s in hit.segments) SearchSegment(text: s.text, match: s.match),
  ];
  final message = hit.message;
  final thread = hit.thread;
  final file = hit.file;
  switch (hit.kind) {
    case rest.SearchHitKind.message when message != null:
      return SearchResultItem(
        id: message.id,
        type: SearchHitType.message,
        path: path,
        segments: segments,
        threadId: ThreadId.parse(message.threadId),
        messageId: MessageId.parse(message.id),
        channelId: ChannelId.parse(message.channelId),
        authorId: PrincipalId.parse(message.author.id),
        authorName: message.author.displayName,
        at: message.createdAt,
      );
    case rest.SearchHitKind.thread when thread != null:
      return SearchResultItem(
        id: thread.id,
        type: SearchHitType.thread,
        path: path,
        segments: segments,
        threadId: ThreadId.parse(thread.id),
        channelId: ChannelId.parse(thread.channelId),
        authorId: PrincipalId.parse(thread.createdBy),
        at: thread.createdAt,
      );
    case rest.SearchHitKind.file when file != null:
      final channel = file.scopeChannelId;
      return SearchResultItem(
        id: file.id,
        type: SearchHitType.file,
        path: path,
        segments: segments,
        channelId: channel == null ? null : ChannelId.parse(channel),
        authorId: PrincipalId.parse(file.uploadedBy),
        title: file.originalName,
        at: file.createdAt,
      );
    default:
      return null;
  }
}
