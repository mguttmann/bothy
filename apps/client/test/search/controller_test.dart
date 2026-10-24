// SPDX-License-Identifier: Apache-2.0

// The search controller of D7 (brief M4-23 Design note 1, docs/spec/14-search.md 4 and 6):
// local hits while the server is pending, replaced by the server hits; offline only local hits
// with the note; cursor_invalid restarts; the reasons of validation_failed; candidates_capped;
// a newer submit wins; the notation and the filters become parameters; the in-thread scope of
// M4-24; segments render as text with marked matches and never as HTML.

import 'dart:async';
import 'dart:io';

import 'package:api_client/api_client.dart' as rest;
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/search/providers.dart';
import 'package:client/search/search_controller.dart';
import 'package:client/search/search_port.dart';
import 'package:client/search/segments_text.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart' as sync;

import '../support/fakes.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

final DateTime now = DateTime.utc(2026, 10, 24, 12);

/// A server port whose answers the test releases one by one.
class FakeServer implements SearchPort {
  final List<({SearchRequest request, String? cursor})> calls = [];
  final List<Completer<SearchResultPage>> _answers = [];

  /// The answer of call [i].
  Completer<SearchResultPage> answer(int i) => _answers[i];

  @override
  Future<SearchResultPage> search(SearchRequest request, {String? cursor}) {
    calls.add((request: request, cursor: cursor));
    final c = Completer<SearchResultPage>();
    _answers.add(c);
    return c.future;
  }
}

/// A local port with fixed hits.
class FakeLocal implements LocalSearchPort {
  FakeLocal(this.hits);

  final List<SearchResultItem> hits;
  final List<({SearchRequest request, int limit})> calls = [];

  @override
  Future<List<SearchResultItem>> search(
    SearchRequest request, {
    required int limit,
  }) async {
    calls.add((request: request, limit: limit));
    return hits;
  }
}

SearchResultItem item(String id, {bool local = false}) => SearchResultItem(
  id: 'hit-$id',
  type: SearchHitType.message,
  threadId: threadId(1),
  messageId: MessageId.parse(uuid(0x700 + id.length)),
  channelId: channelId(1),
  path: const ['Engineering', 'server', 'Gateway: resume'],
  authorName: 'Sam',
  at: now,
  segments: [SearchSegment(text: id, match: true)],
  local: local,
);

SearchResultPage page(List<SearchResultItem> items, {String? next}) =>
    SearchResultPage(items: items, nextCursor: next, candidatesCapped: false);

final List<Principal> principals = [
  Principal(
    id: sam,
    kind: PrincipalKind.user,
    handle: 'sam',
    displayName: 'Sam',
  ),
];

class Harness {
  Harness({
    List<SearchResultItem>? localHits,
    LocalSearchPort? localPort,
    this.scope,
  }) {
    local = FakeLocal(localHits ?? [item('local', local: true)]);
    container = ProviderContainer(
      overrides: [
        appPortsProvider.overrideWithValue(
          testPorts(dataRoot: Directory.systemTemp, clock: TestClock(now)),
        ),
        connectionStateProvider.overrideWith((ref) => phases.stream),
        searchPortProvider.overrideWithValue(server),
        localSearchPortProvider.overrideWithValue(localPort ?? local),
        searchPrincipalsProvider.overrideWith(
          (ref) => Stream.value(principals),
        ),
        searchChannelsProvider.overrideWithValue([channel(1, name: 'general')]),
      ],
    );
    sub = container.listen(provider, (_, _) {});
  }

  final ({ChannelId channel, ThreadId thread})? scope;
  final FakeServer server = FakeServer();
  late final FakeLocal local;
  final StreamController<sync.ConnectionState> phases =
      StreamController<sync.ConnectionState>.broadcast();
  late final ProviderContainer container;
  late final ProviderSubscription<SearchState> sub;

  SearchQueryControllerProvider get provider => scope == null
      ? searchQueryControllerProvider()
      : searchQueryControllerProvider(
          channelId: scope!.channel.value,
          threadId: scope!.thread.value,
        );

  SearchQueryController get controller => container.read(provider.notifier);

  SearchState get state => container.read(provider);

  Future<void> phase(sync.ConnectionPhase p) async {
    phases.add(sync.ConnectionState(p));
    await pumpEventQueue();
  }

  Future<void> submit(String text) async {
    controller.setInput(text);
    unawaited(controller.submit());
    await pumpEventQueue();
  }

  Future<void> dispose() async {
    sub.close();
    container.dispose();
    await phases.close();
  }
}

Future<Harness> live({
  List<SearchResultItem>? localHits,
  LocalSearchPort? localPort,
  ({ChannelId channel, ThreadId thread})? scope,
}) async {
  final h = Harness(localHits: localHits, localPort: localPort, scope: scope);
  addTearDown(h.dispose);
  // Keep the principals stream alive and read once, as the view does.
  h.container.listen(searchPrincipalsProvider, (_, _) {});
  await h.phase(sync.ConnectionPhase.live);
  return h;
}

void main() {
  test(
    'local hits show while the server is pending and are replaced',
    () async {
      final h = await live();
      await h.submit('deploy');

      expect(h.server.calls, hasLength(1));
      expect(h.local.calls, hasLength(1));
      expect(h.state.pending, isTrue);
      expect(h.state.localOnly, isTrue);
      expect(h.state.showLocalNote, isTrue);
      expect(h.state.items.map((i) => i.local), [true]);

      h.server.answer(0).complete(page([item('server')]));
      await pumpEventQueue();
      expect(h.state.pending, isFalse);
      expect(h.state.localOnly, isFalse);
      expect(h.state.showLocalNote, isFalse);
      expect(h.state.items.map((i) => i.local), [false]);
      expect(h.state.items.single.segments.single.text, 'server');
    },
  );

  test('a server answer before the local one is not overwritten', () async {
    final slow = Completer<void>();
    final h = await live(
      localPort: _GatedLocal(slow, [item('late', local: true)]),
    );
    await h.submit('deploy');
    h.server.answer(0).complete(page([item('server')]));
    await pumpEventQueue();
    slow.complete();
    await pumpEventQueue();
    expect(h.state.items.map((i) => i.local), [false]);
  });

  test('offline: only local hits with the note and no server call', () async {
    final h = Harness();
    addTearDown(h.dispose);
    await h.phase(sync.ConnectionPhase.offline);
    await h.submit('deploy');

    expect(h.server.calls, isEmpty);
    expect(h.state.offline, isTrue);
    expect(h.state.pending, isFalse);
    expect(h.state.items.map((i) => i.local), [true]);
    expect(h.state.showLocalNote, isTrue);
  });

  test('a connection error keeps the local hits as offline hits', () async {
    final h = await live();
    await h.submit('deploy');
    h.server
        .answer(0)
        .completeError(const SearchFailure(SearchFailureKind.offline));
    await pumpEventQueue();
    expect(h.state.offline, isTrue);
    expect(h.state.error, isNull);
    expect(h.state.items.map((i) => i.local), [true]);
    expect(h.state.showLocalNote, isTrue);
  });

  test(
    'going live again reruns the server query of an offline search',
    () async {
      final h = Harness();
      addTearDown(h.dispose);
      await h.phase(sync.ConnectionPhase.offline);
      await h.submit('deploy');
      expect(h.server.calls, isEmpty);

      await h.phase(sync.ConnectionPhase.live);
      expect(h.server.calls, hasLength(1));
      h.server.answer(0).complete(page([item('server')]));
      await pumpEventQueue();
      expect(h.state.offline, isFalse);
      expect(h.state.items.map((i) => i.local), [false]);
    },
  );

  test(
    'paging with next_cursor appends; cursor_invalid restarts at page one',
    () async {
      final h = await live();
      await h.submit('deploy');
      h.server.answer(0).complete(page([item('a')], next: 'c1'));
      await pumpEventQueue();
      expect(h.state.nextCursor, 'c1');

      unawaited(h.controller.loadMore());
      await pumpEventQueue();
      expect(h.state.loadingMore, isTrue);
      h.server.answer(1).complete(page([item('b')], next: 'c2'));
      await pumpEventQueue();
      expect(h.state.items.map((i) => i.segments.single.text), ['a', 'b']);

      unawaited(h.controller.loadMore());
      await pumpEventQueue();
      h.server
          .answer(2)
          .completeError(const SearchFailure(SearchFailureKind.cursorInvalid));
      await pumpEventQueue();
      expect(h.server.calls.map((c) => c.cursor), [null, 'c1', 'c2', null]);
      h.server.answer(3).complete(page([item('fresh')]));
      await pumpEventQueue();
      expect(h.state.items.map((i) => i.segments.single.text), ['fresh']);
      expect(h.state.nextCursor, isNull);
      expect(h.state.error, isNull);
    },
  );

  for (final (kind, error) in [
    (SearchFailureKind.queryEmpty, SearchError.queryEmpty),
    (SearchFailureKind.queryTooLong, SearchError.queryTooLong),
    (SearchFailureKind.failed, SearchError.failed),
  ]) {
    test('${kind.name} shows ${error.name} and drops the local hits', () async {
      final h = await live();
      await h.submit('deploy');
      h.server.answer(0).completeError(SearchFailure(kind));
      await pumpEventQueue();
      expect(h.state.error, error);
      expect(h.state.items, isEmpty);
      expect(h.state.showLocalNote, isFalse);
    });
  }

  test('candidates_capped is reported', () async {
    final h = await live();
    await h.submit('deploy');
    h.server
        .answer(0)
        .complete(
          SearchResultPage(
            items: [item('a')],
            nextCursor: null,
            candidatesCapped: true,
          ),
        );
    await pumpEventQueue();
    expect(h.state.candidatesCapped, isTrue);
  });

  test('a newer submit ignores the answer of an older one', () async {
    final h = await live();
    await h.submit('first');
    await h.submit('second');
    h.server.answer(1).complete(page([item('second')]));
    await pumpEventQueue();
    h.server.answer(0).complete(page([item('first')]));
    await pumpEventQueue();
    expect(h.state.items.single.segments.single.text, 'second');
  });

  test(
    'notation and filters become parameters; inapplicable ones are left out',
    () async {
      final h = await live();
      await h.controller.setFilters(
        const SearchFilters().copyWith(
          time: SearchTime.week,
          status: SearchStatus.open,
          hasFile: true,
          sort: SearchSort.newest,
        ),
      );
      await h.submit('from:sam in:general deploy');
      final r = h.server.calls.single.request;
      expect(r.q, 'deploy');
      expect(r.type, SearchType.messages);
      expect(r.authorId, sam);
      expect(r.channelIds, [channelId(1)]);
      expect(r.hasFile, isTrue);
      expect(r.threadState, ThreadState.open);
      expect(r.after, now.subtract(const Duration(days: 7)));
      expect(r.sort, SearchSort.newest);
      expect(h.local.calls.single.request.q, 'deploy');

      unawaited(
        h.controller.setFilters(
          h.state.filters.copyWith(type: SearchType.threads),
        ),
      );
      await pumpEventQueue();
      final threads = h.server.calls.last.request;
      expect(threads.type, SearchType.threads);
      expect(threads.authorId, isNull, reason: 'author_id: messages, files');
      expect(threads.hasFile, isNull, reason: 'has_file: messages');
      expect(threads.threadState, ThreadState.open);

      unawaited(
        h.controller.setFilters(
          h.state.filters.copyWith(type: SearchType.files),
        ),
      );
      await pumpEventQueue();
      final files = h.server.calls.last.request;
      expect(files.authorId, sam);
      expect(
        files.threadState,
        isNull,
        reason: 'thread_state: threads, messages',
      );
    },
  );

  test('the local limit is the named key of AppConfig', () async {
    final h = await live();
    await h.submit('deploy');
    expect(
      h.local.calls.single.limit,
      AppConfig.defaults().quickSwitchMaxResults,
    );
  });

  test('the in-thread search sets channel_id and thread_id (M4-24)', () async {
    final h = await live(scope: (channel: channelId(4), thread: threadId(9)));
    await h.submit('in:general deploy');
    final r = h.server.calls.single.request;
    expect(r.channelIds, [channelId(4)]);
    expect(r.threadId, threadId(9));
    expect(r.spaceIds, isEmpty);
  });

  test('the API port maps the problems of 14 4 and connection errors', () {
    DioException problem(int status, String code, [String? field]) =>
        DioException(
          requestOptions: RequestOptions(path: '/search'),
          type: DioExceptionType.badResponse,
          error: rest.ApiProblem(
            status: status,
            code: code,
            errors: [
              if (field != null)
                rest.ProblemFieldError(pointer: '#query/q', code: field),
            ],
          ),
        );
    expect(
      searchFailureOf(problem(400, 'validation_failed', 'empty')).kind,
      SearchFailureKind.queryEmpty,
    );
    expect(
      searchFailureOf(problem(400, 'validation_failed', 'too_long')).kind,
      SearchFailureKind.queryTooLong,
    );
    expect(
      searchFailureOf(problem(400, 'cursor_invalid')).kind,
      SearchFailureKind.cursorInvalid,
    );
    expect(
      searchFailureOf(problem(400, 'validation_failed')).kind,
      SearchFailureKind.failed,
    );
    expect(
      searchFailureOf(problem(503, 'unavailable')).kind,
      SearchFailureKind.failed,
    );
    expect(
      searchFailureOf(
        DioException(
          requestOptions: RequestOptions(path: '/search'),
          type: DioExceptionType.connectionError,
        ),
      ).kind,
      SearchFailureKind.offline,
    );
    expect(searchFailureOf(StateError('x')).kind, SearchFailureKind.failed);
  });

  test('the API port maps the hits of a page', () {
    final json = {
      'items': [
        {
          'kind': 'message',
          'thread_title': 'Gateway: resume',
          'channel_name': 'server',
          'space_name': 'Engineering',
          'segments': [
            {'text': 'window ', 'match': false},
            {'text': 'resume_window', 'match': true},
          ],
          'message': _messageJson(),
        },
        {
          'kind': 'thread',
          'channel_name': 'releases',
          'space_name': 'Engineering',
          'thread_title': 'Version 0.4',
          'segments': [
            {'text': 'Version 0.4', 'match': true},
          ],
          'thread': _threadJson(),
        },
        {
          'kind': 'file',
          'channel_name': 'server',
          'segments': [
            {'text': 'notes-sync.md', 'match': true},
          ],
          'file': _fileJson(),
        },
      ],
      'next_cursor': 'opaque',
      'candidates_capped': true,
    };
    final restPage = rest.standardSerializers.deserializeWith(
      rest.SearchHitPage.serializer,
      json,
    )!;
    final mapped = searchPageFromRest(restPage);
    expect(mapped.nextCursor, 'opaque');
    expect(mapped.candidatesCapped, isTrue);
    final [message, thread, file] = mapped.items;
    expect(message.type, SearchHitType.message);
    expect(message.messageId, MessageId.parse(uuid(0x701)));
    expect(message.threadId, threadId(1));
    expect(message.path, ['Engineering', 'server', 'Gateway: resume']);
    expect(message.authorName, 'Sam');
    expect(message.segments.map((s) => s.match), [false, true]);
    expect(thread.type, SearchHitType.thread);
    expect(thread.threadId, threadId(2));
    expect(thread.authorId, alex);
    expect(file.type, SearchHitType.file);
    expect(file.title, 'notes-sync.md');
    expect(file.channelId, channelId(1));
    expect(file.threadId, isNull);
    expect(file.path, ['server']);
  });

  testWidgets('segments render as text with marked matches, never as HTML', (
    tester,
  ) async {
    await tester.pumpWidget(
      hostScreen(
        const Material(
          child: SegmentsText(
            segments: [
              SearchSegment(text: 'see <b>x</b> and ', match: false),
              SearchSegment(text: 'resume_window', match: true),
              SearchSegment(text: ' &amp; more', match: false),
            ],
          ),
        ),
      ),
    );
    final rich = tester.widget<RichText>(
      find.descendant(
        of: find.byType(SegmentsText),
        matching: find.byType(RichText),
      ),
    );
    expect(
      rich.text.toPlainText(),
      'see <b>x</b> and resume_window &amp; more',
    );
    final spans = <TextSpan>[];
    rich.text.visitChildren((s) {
      if (s is TextSpan && s.text != null) spans.add(s);
      return true;
    });
    expect(spans.map((s) => s.text), [
      'see <b>x</b> and ',
      'resume_window',
      ' &amp; more',
    ]);
    expect(spans[1].style?.fontWeight, FontWeight.w700);
    expect(spans[0].style?.fontWeight, isNot(FontWeight.w700));
  });
}

class _GatedLocal implements LocalSearchPort {
  _GatedLocal(this.gate, this.hits);

  final Completer<void> gate;
  final List<SearchResultItem> hits;

  @override
  Future<List<SearchResultItem>> search(
    SearchRequest request, {
    required int limit,
  }) async {
    await gate.future;
    return hits;
  }
}

Map<String, Object?> _principalJson(PrincipalId id, String name) => {
  'id': id.value,
  'kind': 'user',
  'handle': name.toLowerCase(),
  'display_name': name,
};

Map<String, Object?> _messageJson() => {
  'id': uuid(0x701),
  'thread_id': threadId(1).value,
  'channel_id': channelId(1).value,
  'thread_seq': 3,
  'author': _principalJson(sam, 'Sam'),
  'client_msg_id': uuid(0x801),
  'origin': 'human',
  'kind': 'text',
  'content_md': 'window resume_window is measured',
  'content_format': 1,
  'silent': false,
  'attachments': <Object?>[],
  'mention_ids': <Object?>[],
  'reactions': <Object?>[],
  'pinned': false,
  'components': <Object?>[],
  'version': 1,
  'created_at': '2026-10-24T10:14:00Z',
};

Map<String, Object?> _threadJson() => {
  'id': threadId(2).value,
  'channel_id': channelId(2).value,
  'kind': 'thread',
  'title': 'Version 0.4',
  'title_source': 'manual',
  'state': 'open',
  'last_message_seq': 4,
  'last_activity_at': '2026-10-23T08:00:00Z',
  'message_count': 4,
  'created_by': alex.value,
  'created_at': '2026-10-23T08:00:00Z',
  'version': 1,
  'tag_ids': <Object?>[],
};

Map<String, Object?> _fileJson() => {
  'id': uuid(0x901),
  'size_bytes': 2048,
  'mime_type': 'text/markdown',
  'original_name': 'notes-sync.md',
  'uploaded_by': robin.value,
  'purpose': 'attachment',
  'scope_channel_id': channelId(1).value,
  'upload_state': 'complete',
  'scan_status': 'clean',
  'transcript_status': 'none',
  'thumbnails': <Object?>[],
  'created_at': '2026-10-22T08:00:00Z',
};
