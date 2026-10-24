// SPDX-License-Identifier: Apache-2.0

/// Handlers of the cross-cutting views of brief M4-23: `search` (14 4), `searchThreadTitles`
/// (14 4.9), `listMyMentions` (01 3.5) and `listActiveRuns` (01 D10). `getTranscript` and
/// `cancelRun` already live in `file_handlers.dart` and `run_handlers.dart`.
///
/// The fake simplifies the query language and the ranking of 14 4.2 and 4.5: `q` is split into
/// words (runs of letters and digits), every word must start a word of the searched text
/// (case-insensitive, a stand-in for the stemming of `websearch_to_tsquery`), and `or`,
/// `-word` and `"phrases"` are plain words. The relevance score is the number of matched words
/// of the text, without the recency factor.
library;

import 'dart:convert';
import 'dart:math';

import 'package:crypto/crypto.dart';

import '../rest_fake.dart';
import '../world.dart';

/// The filters of 14 4.4 and the result types each applies to.
const Map<String, Set<String>> _filterTypes = {
  'space_id': {'messages', 'threads', 'files'},
  'channel_id': {'messages', 'threads', 'files'},
  'author_id': {'messages', 'files'},
  'agent_id': {'messages', 'threads', 'files'},
  'after': {'messages', 'threads', 'files'},
  'before': {'messages', 'threads', 'files'},
  'has_file': {'messages'},
  'thread_state': {'messages', 'threads'},
  'tag_id': {'messages', 'threads'},
  'thread_id': {'messages', 'threads', 'files'},
};

/// The run states "Running now" lists (`listActiveRuns`, 01 D10).
const Set<String> _activeRunStates = {'queued', 'running', 'waiting_input'};

final RegExp _word = RegExp(r'[\p{L}\p{N}]+', unicode: true);

/// The words of [text] in lower case.
List<String> _words(String text) => [
  for (final m in _word.allMatches(text)) m[0]!.toLowerCase(),
];

/// A `400 validation_failed` on the query parameter [name] with [code] (06 5; the pointer form
/// `#query/<name>` of 14 4.4).
FakeProblem _invalid(String name, String code) => FakeProblem(
  400,
  'validation_failed',
  errors: [
    {'pointer': '#query/$name', 'code': code},
  ],
);

const FakeProblem _cursorInvalid = FakeProblem(400, 'cursor_invalid');

/// Orders two sort keys element by element, each element descending.
int _descending(List<Object?> a, List<Object?> b) {
  for (var i = 0; i < a.length; i++) {
    final c = (b[i]! as Comparable<Object>).compareTo(a[i]!);
    if (c != 0) return c;
  }
  return 0;
}

/// Orders two sort keys element by element, each element ascending.
int _ascending(List<Object?> a, List<Object?> b) => _descending(b, a);

/// The opaque cursor of 14 4.7: the sort key of the last delivered entry, the hash of the
/// request it belongs to and, for `search`, the reference time of the first page.
String _encodeCursor(String hash, List<Object?> key, {String? at}) => base64Url
    .encode(utf8.encode(jsonEncode({'h': hash, 'k': key, 't': ?at})))
    .replaceAll('=', '');

/// The key and reference time of [raw] when it belongs to [hash] and has [length] elements;
/// `400 cursor_invalid` for anything else (14 4.7, 06 5).
({List<Object?> key, String? at}) _decodeCursor(
  String raw,
  String hash,
  int length,
) {
  try {
    final decoded = jsonDecode(
      utf8.decode(base64Url.decode(base64Url.normalize(raw))),
    );
    if (decoded is Map &&
        decoded['h'] == hash &&
        decoded['k'] is List &&
        (decoded['k'] as List).length == length) {
      final key = List<Object?>.from(decoded['k'] as List);
      if (key.every((k) => k is num || k is String)) {
        return (key: key, at: decoded['t'] as String?);
      }
    }
  } on FormatException {
    // falls through to cursor_invalid
  }
  throw _cursorInvalid;
}

/// The hash a cursor carries: the principal and every parameter that shapes the result,
/// never the page size (14 4.7).
String _hashOf(String principalId, Object request) => sha256
    .convert(utf8.encode(jsonEncode([principalId, request])))
    .toString()
    .substring(0, 32);

/// One page of [entries] (sorted by [compare] over [keyOf]) after [cursor], at most [limit]
/// long, with the cursor of the next page while more follow.
({List<T> items, String? next}) _page<T>({
  required List<T> entries,
  required List<Object?> Function(T entry) keyOf,
  required int Function(List<Object?> a, List<Object?> b) compare,
  required int limit,
  required String hash,
  List<Object?>? after,
  String? at,
}) {
  final rest = after == null
      ? entries
      : [
          for (final e in entries)
            if (compare(keyOf(e), after) > 0) e,
        ];
  final items = rest.take(limit).toList();
  final more = rest.length > items.length;
  return (
    items: items,
    next: more ? _encodeCursor(hash, keyOf(items.last), at: at) : null,
  );
}

/// The `limit` of [r] from 1 up to [max] (default [standard]); a larger value is capped
/// (06 6, `PageLimit`), a value below 1 or not a number is `400 validation_failed`.
int _limit(FakeRequest r, int standard, int max) {
  final raw = r.query('limit');
  if (raw == null) return standard;
  final asked = int.tryParse(raw);
  if (asked == null || asked < 1) throw _invalid('limit', 'invalid');
  return min(asked, max);
}

/// The text of [text] split at the words that start with a word of [query] (14 4.6): plain
/// text segments, never markup.
List<JsonMap> _segments(String text, List<String> query) {
  final out = <JsonMap>[];
  void add(String part, {required bool match}) {
    if (part.isEmpty) return;
    if (out.isNotEmpty && out.last['match'] == match) {
      out.last['text'] = '${out.last['text']}$part';
    } else {
      out.add({'text': part, 'match': match});
    }
  }

  var start = 0;
  for (final m in _word.allMatches(text)) {
    final word = m[0]!.toLowerCase();
    if (query.any(word.startsWith)) {
      add(text.substring(start, m.start), match: false);
      add(m[0]!, match: true);
      start = m.end;
    }
  }
  add(text.substring(start), match: false);
  return out;
}

/// Registers the search, mention and "Running now" handlers on [rest].
///
/// Visibility follows the fake's channel rule (14 4.3): messages and threads of channels the
/// caller sees, files attached to a visible, non-deleted message (14 2). Limits come from
/// [FakeWorld.searchSettings].
void registerSearchHandlers(RestFake rest) {
  final world = rest.world;
  final settings = world.searchSettings;

  String? spaceOf(String channelId) => world.channels[channelId]?.spaceId;

  /// The agent that operates [thread]: its own operator, else the channel's effective one
  /// (the fake has no thread operators, 10 5.2).
  String? operatorOf(JsonMap thread) =>
      thread['operator_agent_id'] as String? ??
      (world.channels[thread['channel_id']]?.json['operator']
              as JsonMap?)?['effective_agent_id']
          as String?;

  rest
    ..handle('search', (r) {
      final q = r.query('q');
      if (q == null) throw _invalid('q', 'required');
      if (q.runes.length > settings.queryMaxChars) {
        throw _invalid('q', 'too_long');
      }
      final query = _words(q);
      if (query.isEmpty) throw _invalid('q', 'empty');
      final type = r.query('type') ?? 'messages';
      if (!const {'messages', 'threads', 'files'}.contains(type)) {
        throw _invalid('type', 'invalid');
      }
      final sort = r.query('sort') ?? 'relevance';
      if (!const {'relevance', 'newest'}.contains(sort)) {
        throw _invalid('sort', 'invalid');
      }
      final params = r.uri.queryParametersAll;
      for (final name in _filterTypes.keys) {
        if (params.containsKey(name) && !_filterTypes[name]!.contains(type)) {
          throw _invalid(name, 'not_applicable');
        }
      }
      List<String>? many(String name) =>
          params.containsKey(name) ? (List.of(params[name]!)..sort()) : null;
      String? time(String name) {
        final raw = r.query(name);
        if (raw == null) return null;
        final parsed = DateTime.tryParse(raw);
        if (parsed == null) throw _invalid(name, 'invalid');
        return world.stamp(parsed);
      }

      bool? flag(String name) => switch (r.query(name)) {
        null => null,
        'true' => true,
        'false' => false,
        _ => throw _invalid(name, 'invalid'),
      };
      final spaces = many('space_id');
      final channelIds = many('channel_id');
      final tags = many('tag_id');
      final threadId = r.query('thread_id');
      final authorId = r.query('author_id');
      final agentId = r.query('agent_id');
      final after = time('after');
      final before = time('before');
      final hasFile = flag('has_file');
      final threadState = r.query('thread_state');
      if (threadState != null &&
          !const {'open', 'closed'}.contains(threadState)) {
        throw _invalid('thread_state', 'invalid');
      }
      final hash = _hashOf(r.person.id, {
        'q': q,
        'type': type,
        'sort': sort,
        'space_id': spaces,
        'channel_id': channelIds,
        'tag_id': tags,
        'thread_id': threadId,
        'author_id': authorId,
        'agent_id': agentId,
        'after': after,
        'before': before,
        'has_file': hasFile,
        'thread_state': threadState,
      });
      final relevance = sort == 'relevance';
      final cursor = r.query('cursor');
      final position = cursor == null
          ? null
          : _decodeCursor(cursor, hash, relevance ? 3 : 2);
      // 14 4.7: later pages measure against the reference time of the first page.
      final reference = position?.at ?? world.stamp();

      bool threadPasses(JsonMap thread) {
        final channelId = thread['channel_id']! as String;
        if (!world.seesChannel(r.person.id, channelId)) return false;
        if (spaces != null && !spaces.contains(spaceOf(channelId))) {
          return false;
        }
        if (channelIds != null && !channelIds.contains(channelId)) {
          return false;
        }
        if (threadId != null && thread['id'] != threadId) return false;
        if (agentId != null && operatorOf(thread) != agentId) return false;
        if (threadState != null && thread['state'] != threadState) {
          return false;
        }
        if (tags != null &&
            !(thread['tag_ids'] as List<Object?>? ?? const []).any(
              tags.contains,
            )) {
          return false;
        }
        return true;
      }

      bool inTime(String createdAt) =>
          createdAt.compareTo(reference) <= 0 &&
          (after == null || createdAt.compareTo(after) >= 0) &&
          (before == null || createdAt.compareTo(before) < 0);

      /// The number of matched words of [text], or 0 when a word of the query is missing.
      int score(String text) {
        final words = _words(text);
        if (!query.every((q) => words.any((w) => w.startsWith(q)))) return 0;
        return words.where((w) => query.any(w.startsWith)).length;
      }

      final matches =
          <({JsonMap hit, String createdAt, String id, int score})>[];
      void add(
        JsonMap thread,
        JsonMap hit,
        String createdAt,
        String id,
        int s,
      ) {
        final channel = world.channels[thread['channel_id']]!;
        matches.add((
          hit: {
            ...hit,
            'thread_title': ?thread['title'],
            'channel_name': ?channel.json['name'],
            'space_name': ?world.spaces[channel.spaceId]?['name'],
          },
          createdAt: createdAt,
          id: id,
          score: s,
        ));
      }

      switch (type) {
        case 'messages':
          for (final m in world.messages.values) {
            final thread = world.threads[m['thread_id']];
            final createdAt = m['created_at']! as String;
            if (m['deleted_at'] != null ||
                thread == null ||
                !threadPasses(thread) ||
                !inTime(createdAt) ||
                (authorId != null &&
                    (m['author']! as JsonMap)['id'] != authorId)) {
              continue;
            }
            final attached = (m['attachments'] as List<Object?>?)?.isNotEmpty;
            if (hasFile != null && (attached ?? false) != hasFile) continue;
            final text = m['content_md']! as String;
            final s = score(text);
            if (s == 0) continue;
            add(
              thread,
              {
                'kind': 'message',
                'message': m,
                'segments': _segments(text, query),
              },
              createdAt,
              m['id']! as String,
              s,
            );
          }
        case 'threads':
          for (final thread in world.threads.values) {
            final createdAt = thread['created_at']! as String;
            if (!threadPasses(thread) || !inTime(createdAt)) continue;
            final title = thread['title'] as String? ?? '';
            final s = score(title);
            if (s == 0) continue;
            add(
              thread,
              {
                'kind': 'thread',
                'thread': thread,
                'segments': _segments(title, query),
              },
              createdAt,
              thread['id']! as String,
              s,
            );
          }
        case 'files':
          for (final f in world.files.values) {
            final m = world.messages[f.attachedTo];
            final thread = world.threads[m?['thread_id']];
            final createdAt = f.json['created_at']! as String;
            if (m == null ||
                m['deleted_at'] != null ||
                thread == null ||
                !f.servable ||
                !threadPasses(thread) ||
                !inTime(createdAt) ||
                (authorId != null && f.uploadedBy != authorId)) {
              continue;
            }
            final name = f.json['original_name'] as String? ?? '';
            final transcript = f.json['transcript_status'] == 'done'
                ? f.transcriptText ?? ''
                : '';
            final s = score('$name $transcript');
            if (s == 0) continue;
            final spoken = _segments(transcript, query);
            add(
              thread,
              {
                'kind': 'file',
                'file': f.json,
                // The name, then the transcript when it holds a match (14 2: both are indexed).
                'segments': [
                  ..._segments(name, query),
                  if (spoken.any((x) => x['match'] == true)) ...[
                    {'text': ' ', 'match': false},
                    ...spoken,
                  ],
                ],
              },
              createdAt,
              f.id,
              s,
            );
          }
      }

      // 14 4.5 item 1: only the newest max_candidates matches are ranked.
      matches.sort(
        (a, b) => _descending([a.createdAt, a.id], [b.createdAt, b.id]),
      );
      final capped = matches.length > settings.maxCandidates;
      final candidates = matches.take(settings.maxCandidates).toList();
      List<Object?> keyOf(
        ({JsonMap hit, String createdAt, String id, int score}) e,
      ) => relevance ? [e.score, e.createdAt, e.id] : [e.createdAt, e.id];
      candidates.sort((a, b) => _descending(keyOf(a), keyOf(b)));
      final page = _page(
        entries: candidates,
        keyOf: keyOf,
        compare: _descending,
        limit: _limit(r, settings.pageSizeDefault, settings.pageSizeMax),
        hash: hash,
        after: position?.key,
        at: reference,
      );
      return FakeResponse(200, {
        'items': [
          for (final e in page.items)
            {...e.hit, if (relevance) 'score': e.score.toDouble()},
        ],
        'next_cursor': ?page.next,
        'candidates_capped': capped,
      });
    })
    ..handle('searchThreadTitles', (r) {
      final q = r.query('q');
      if (q == null || q.isEmpty) throw _invalid('q', 'required');
      final query = [
        for (final w in _words(q))
          if (w.length >= settings.prefixMinChars) w,
      ];
      final hits = <JsonMap>[];
      if (query.isNotEmpty) {
        for (final t in world.threads.values) {
          if (!world.seesThread(r.person.id, t['id']! as String)) continue;
          final words = _words(t['title'] as String? ?? '');
          if (query.every((q) => words.any((w) => w.startsWith(q)))) {
            hits.add({
              'thread_id': t['id'],
              'channel_id': t['channel_id'],
              'title': t['title'],
              'last_activity_at': t['last_activity_at'],
            });
          }
        }
      }
      hits.sort(
        (a, b) => _descending(
          [a['last_activity_at'], a['thread_id']],
          [b['last_activity_at'], b['thread_id']],
        ),
      );
      final limit = _limit(r, settings.pageSizeDefault, settings.pageSizeMax);
      return FakeResponse(200, {'items': hits.take(limit).toList()});
    })
    ..handle('listMyMentions', (r) {
      final me = r.person.id;
      final mentions = [
        for (final m in world.messages.values)
          if (m['deleted_at'] == null &&
              (m['mention_ids'] as List<Object?>? ?? const []).contains(me) &&
              world.seesThread(me, m['thread_id']! as String))
            m,
      ];
      List<Object?> keyOf(JsonMap m) => [m['created_at'], m['id']];
      mentions.sort((a, b) => _descending(keyOf(a), keyOf(b)));
      final hash = _hashOf(me, 'mentions');
      final cursor = r.query('cursor');
      final page = _page(
        entries: mentions,
        keyOf: keyOf,
        compare: _descending,
        limit: _limit(
          r,
          settings.listPageSizeDefault,
          settings.listPageSizeMax,
        ),
        hash: hash,
        after: cursor == null ? null : _decodeCursor(cursor, hash, 2).key,
      );
      return FakeResponse(200, {
        'items': [
          for (final m in page.items)
            {'message': m, 'created_at': m['created_at']},
        ],
        'next_cursor': ?page.next,
      });
    })
    ..handle('listActiveRuns', (r) {
      final me = r.person.id;
      final agentId = r.query('agent_id');
      final runs = [
        for (final run in world.runs.values)
          if (_activeRunStates.contains(run['state']) &&
              world.seesChannel(me, run['channel_id']! as String) &&
              (agentId == null || run['agent_id'] == agentId))
            run,
      ];
      // Oldest queued first, the order of a list of work in progress.
      List<Object?> keyOf(JsonMap run) => [run['queued_at'], run['id']];
      runs.sort((a, b) => _ascending(keyOf(a), keyOf(b)));
      final hash = _hashOf(me, {'agent_id': agentId});
      final cursor = r.query('cursor');
      final page = _page(
        entries: runs,
        keyOf: keyOf,
        compare: _ascending,
        limit: _limit(
          r,
          settings.listPageSizeDefault,
          settings.listPageSizeMax,
        ),
        hash: hash,
        after: cursor == null ? null : _decodeCursor(cursor, hash, 2).key,
      );
      return FakeResponse(200, {
        'items': [for (final run in page.items) world.runSummary(run)],
        'next_cursor': ?page.next,
      });
    });
}
