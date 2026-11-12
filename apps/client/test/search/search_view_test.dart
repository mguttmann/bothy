// SPDX-License-Identifier: Apache-2.0

// The search view D7 (docs/spec/01-product.md 5.1 D7, brief M4-23 Design note 1): the query of
// the route runs on open, local hits show with the note while the server is pending, the count
// line counts the loaded hits, the reasons and the capped note show, "Load more" pages, a
// filter searches again (the agent and the tag of a forum too), selecting a hit navigates to
// the thread and the message (a file to the message it is attached to), a pending search says
// so, and a timeout and a rate limit show their own text above the local hits.

import 'dart:async';
import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/assist/assist_port.dart';
import 'package:client/assist/providers.dart';
import 'package:client/search/providers.dart';
import 'package:client/search/search_port.dart';
import 'package:client/search/search_view.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:sync_engine/sync_engine.dart' as sync;

import '../support/fakes.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

final DateTime now = DateTime.utc(2026, 10, 24, 12);

class FakeServer implements SearchPort {
  final List<({SearchRequest request, String? cursor})> calls = [];
  final List<Completer<SearchResultPage>> answers = [];

  @override
  Future<SearchResultPage> search(
    SearchRequest request, {
    String? cursor,
    SearchCancel? cancel,
  }) {
    calls.add((request: request, cursor: cursor));
    final c = Completer<SearchResultPage>();
    answers.add(c);
    return c.future;
  }
}

class FakeLocal implements LocalSearchPort {
  FakeLocal(this.hits);

  final List<SearchResultItem> hits;

  @override
  Future<List<SearchResultItem>> search(
    SearchRequest request, {
    required int limit,
  }) async => hits;
}

final MessageId message1 = MessageId.parse(uuid(0x701));

SearchResultItem messageHit(String id, {bool local = false, String? text}) =>
    SearchResultItem(
      id: id,
      type: SearchHitType.message,
      path: const ['Engineering', 'server', 'Gateway: resume'],
      segments: [
        const SearchSegment(text: 'window ', match: false),
        SearchSegment(text: text ?? 'resume_window', match: true),
      ],
      threadId: threadId(1),
      messageId: message1,
      channelId: channelId(1),
      authorId: sam,
      authorName: 'Sam',
      at: now.subtract(const Duration(hours: 2)),
      local: local,
    );

final SearchResultItem threadHit = SearchResultItem(
  id: threadId(2).value,
  type: SearchHitType.thread,
  path: const ['Engineering', 'releases', 'Version 0.4'],
  segments: const [SearchSegment(text: 'Version 0.4', match: true)],
  threadId: threadId(2),
  channelId: channelId(2),
  authorId: robin,
  at: now.subtract(const Duration(days: 1)),
);

/// A file hit as the server delivers it: no channel of the file, the thread and the message it
/// is attached to.
final SearchResultItem fileHit = SearchResultItem(
  id: uuid(0x901),
  type: SearchHitType.file,
  path: const ['Engineering', 'server', 'Gateway: resume'],
  segments: const [SearchSegment(text: 'notes-sync.md', match: true)],
  threadId: threadId(1),
  messageId: MessageId.parse(uuid(0x703)),
  authorId: sam,
  title: 'notes-sync.md',
  at: now.subtract(const Duration(days: 2)),
);

class Host {
  Host(this.tester, {String initialQuery = 'resume_window'}) {
    router = GoRouter(
      initialLocation: '/search',
      routes: [
        GoRoute(
          path: '/search',
          builder: (context, state) => SearchView(initialQuery: initialQuery),
        ),
        GoRoute(
          path: '/t/:id',
          builder: (context, state) => const Text('thread'),
        ),
        GoRoute(
          path: '/c/:id',
          builder: (context, state) => const Text('channel'),
        ),
      ],
    );
    addTearDown(router.dispose);
    addTearDown(phases.close);
  }

  final WidgetTester tester;
  late final GoRouter router;
  final FakeServer server = FakeServer();
  final RecordingAssistant assistant = RecordingAssistant();
  final FakeLocal local = FakeLocal([messageHit('local-1', local: true)]);
  final StreamController<sync.ConnectionState> phases =
      StreamController<sync.ConnectionState>.broadcast();

  String get location => router.routeInformationProvider.value.uri.toString();

  Future<void> open() async {
    desktopWindow(tester);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appPortsProvider.overrideWithValue(
            testPorts(dataRoot: Directory.systemTemp, clock: TestClock(now)),
          ),
          connectionStateProvider.overrideWith(
            (ref) => Stream.value(
              const sync.ConnectionState(sync.ConnectionPhase.live),
            ),
          ),
          personTimeZoneProvider.overrideWithValue('UTC'),
          searchPortProvider.overrideWithValue(server),
          assistPortProvider.overrideWithValue(assistant),
          localSearchPortProvider.overrideWithValue(local),
          searchPrincipalsProvider.overrideWith(
            (ref) => Stream.value([
              Principal(
                id: robin,
                kind: PrincipalKind.agent,
                handle: 'dev-runner',
                displayName: 'dev-runner',
              ),
              Principal(
                id: sam,
                kind: PrincipalKind.user,
                handle: 'sam',
                displayName: 'Sam',
              ),
            ]),
          ),
          searchSpacesProvider.overrideWith(
            (ref) => Stream.value([space(1, name: 'Engineering')]),
          ),
          searchChannelsProvider.overrideWithValue([
            channel(1, name: 'server'),
            channel(2, name: 'releases'),
            channel(3, name: 'ideas', kind: ChannelKind.forum),
          ]),
          searchChannelTagsProvider.overrideWith(
            (ref, channelId) => Stream.value([
              if (channelId == channelId3)
                for (final t in [
                  tag(1, 'bug', channelN: 3),
                  tag(2, 'idea', channelN: 3),
                ])
                  t,
            ]),
          ),
        ],
        child: hostRouter(router),
      ),
    );
    await tester.pump();
    await tester.pump();
  }

  Future<void> answer(int i, SearchResultPage page) async {
    server.answers[i].complete(page);
    await tester.pump();
    await tester.pump();
  }

  Future<void> fail(
    int i,
    SearchFailureKind kind, {
    Duration? retryAfter,
  }) async {
    server.answers[i].completeError(
      SearchFailure(kind, retryAfter: retryAfter),
    );
    await tester.pump();
    await tester.pump();
  }
}

final String channelId3 = channelId(3).value;

SearchResultPage page(
  List<SearchResultItem> items, {
  String? next,
  bool capped = false,
}) =>
    SearchResultPage(items: items, nextCursor: next, candidatesCapped: capped);

/// The search assistant of the view's button; records the questions.
class RecordingAssistant implements AssistPort {
  final List<String> questions = [];

  @override
  Future<AssistAnswer> ask(String q, List<AssistTurn> history) async {
    questions.add(q);
    return const AssistAnswer(hits: [], terms: []);
  }
}

void main() {
  testWidgets(
    'the assistant button asks the search assistant with the field text',
    (tester) async {
      final h = Host(tester, initialQuery: '');
      await h.open();
      await tester.enterText(
        find.byKey(const ValueKey('search.field')),
        'where did the steering session move',
      );
      await tester.tap(find.byKey(const ValueKey('search.assist')));
      await tester.pump();
      await tester.pump();
      expect(find.byKey(const ValueKey('assist.dialog')), findsOneWidget);
      expect(h.assistant.questions, ['where did the steering session move']);
      expect(h.server.calls, isEmpty);
    },
  );

  testWidgets(
    'the route query runs on open; local hits with the note, then the server',
    (tester) async {
      final h = Host(tester);
      await h.open();
      expect(h.server.calls.single.request.q, 'resume_window');
      expect(find.byKey(const ValueKey('search.localNote')), findsOneWidget);
      expect(find.text(en.searchLocalOnly), findsOneWidget);
      expect(find.byKey(const ValueKey('search.hit.local-1')), findsOneWidget);

      await h.answer(0, page([messageHit(message1.value), threadHit]));
      expect(find.byKey(const ValueKey('search.localNote')), findsNothing);
      expect(find.byKey(const ValueKey('search.hit.local-1')), findsNothing);
      expect(find.text(en.searchResultCount(2)), findsOneWidget);
      expect(find.textContaining('Gateway: resume'), findsWidgets);
      expect(find.textContaining('dev-runner'), findsOneWidget);
    },
  );

  testWidgets('an empty route query does not search', (tester) async {
    final h = Host(tester, initialQuery: '');
    await h.open();
    expect(h.server.calls, isEmpty);
    expect(find.byKey(const ValueKey('search.localNote')), findsNothing);
  });

  testWidgets(
    'selecting a message hit opens the message, a thread hit the thread',
    (tester) async {
      final h = Host(tester);
      await h.open();
      await h.answer(0, page([messageHit(message1.value), threadHit]));

      await tester.tap(find.byKey(ValueKey('search.hit.${message1.value}')));
      await tester.pumpAndSettle();
      expect(h.location, '/t/${threadId(1).value}?m=${message1.value}');

      h.router.go('/search');
      await tester.pumpAndSettle();
      await tester.pump();
      await h.answer(1, page([messageHit(message1.value), threadHit]));
      await tester.tap(find.byKey(ValueKey('search.hit.${threadId(2).value}')));
      await tester.pumpAndSettle();
      expect(h.location, '/t/${threadId(2).value}');
    },
  );

  testWidgets('selecting a file hit opens the message it is attached to', (
    tester,
  ) async {
    final h = Host(tester);
    await h.open();
    await h.answer(0, page([fileHit]));
    await tester.tap(find.byKey(ValueKey('search.hit.${fileHit.id}')));
    await tester.pumpAndSettle();
    expect(h.location, '/t/${threadId(1).value}?m=${uuid(0x703)}');
  });

  testWidgets('a pending search says so until the server answered', (
    tester,
  ) async {
    final h = Host(tester);
    await h.open();
    expect(find.byKey(const ValueKey('search.pending')), findsOneWidget);
    expect(find.text(en.searchPending), findsOneWidget);
    expect(find.byKey(const ValueKey('search.count')), findsNothing);

    await h.answer(0, page([messageHit(message1.value)]));
    expect(find.byKey(const ValueKey('search.pending')), findsNothing);
    expect(find.byKey(const ValueKey('search.count')), findsOneWidget);
  });

  testWidgets(
    'a timeout asks to narrow, a rate limit names the wait; local hits stay',
    (tester) async {
      final h = Host(tester);
      await h.open();
      await h.fail(0, SearchFailureKind.timeout);
      expect(find.byKey(const ValueKey('search.pending')), findsNothing);
      expect(find.text(en.searchTimeout), findsOneWidget);
      expect(find.text(en.searchFailed), findsNothing);
      expect(find.byKey(const ValueKey('search.hit.local-1')), findsOneWidget);
      expect(find.byKey(const ValueKey('search.localNote')), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('search.submit')));
      await tester.pump();
      await h.fail(
        1,
        SearchFailureKind.rateLimited,
        retryAfter: const Duration(milliseconds: 6200),
      );
      expect(find.text(en.searchRateLimited(7)), findsOneWidget);
      expect(find.byKey(const ValueKey('search.hit.local-1')), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('search.submit')));
      await tester.pump();
      await h.fail(2, SearchFailureKind.rateLimited);
      expect(find.text(en.searchRateLimited(0)), findsOneWidget);
    },
  );

  testWidgets(
    'typing and the button search again; the reasons and the capped note show',
    (tester) async {
      final h = Host(tester);
      await h.open();
      await h.answer(0, page([messageHit(message1.value)], capped: true));
      expect(find.byKey(const ValueKey('search.capped')), findsOneWidget);
      expect(find.text(en.searchCandidatesCapped), findsOneWidget);

      await tester.enterText(find.byKey(const ValueKey('search.field')), '!!');
      await tester.tap(find.byKey(const ValueKey('search.submit')));
      await tester.pump();
      expect(h.server.calls.last.request.q, '!!');
      await h.fail(1, SearchFailureKind.queryEmpty);
      expect(find.byKey(const ValueKey('search.error')), findsOneWidget);
      expect(find.text(en.searchQueryEmpty), findsOneWidget);

      await tester.enterText(find.byKey(const ValueKey('search.field')), 'x y');
      await tester.testTextInput.receiveAction(TextInputAction.search);
      await tester.pump();
      expect(h.server.calls.last.request.q, 'x y');
      await h.fail(2, SearchFailureKind.queryTooLong);
      expect(find.text(en.searchQueryTooLong), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('search.submit')));
      await tester.pump();
      await h.fail(3, SearchFailureKind.failed);
      expect(find.text(en.searchFailed), findsOneWidget);
    },
  );

  testWidgets('Load more asks for the next page with the cursor', (
    tester,
  ) async {
    final h = Host(tester);
    await h.open();
    await h.answer(0, page([messageHit(message1.value)], next: 'c1'));
    await tester.tap(find.byKey(const ValueKey('search.loadMore')));
    await tester.pump();
    expect(h.server.calls.last.cursor, 'c1');
    await h.answer(1, page([threadHit]));
    expect(find.text(en.searchResultCount(2)), findsOneWidget);
    expect(find.byKey(const ValueKey('search.loadMore')), findsNothing);
  });

  testWidgets('a filter of the column searches again with its parameter', (
    tester,
  ) async {
    final h = Host(tester);
    await h.open();
    await h.answer(0, page(const []));
    expect(find.text(en.searchNoResults), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('search.filter.type')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(en.searchTypeThreads).last);
    await tester.pumpAndSettle();
    expect(h.server.calls.last.request.type, SearchType.threads);

    await tester.tap(find.byKey(const ValueKey('search.filter.hasFile')));
    await tester.pump();
    // has_file applies to messages only (14 4.4); the type is threads now.
    expect(h.server.calls.last.request.hasFile, isNull);

    await tester.tap(find.byKey(const ValueKey('search.filter.channel')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('releases').last);
    await tester.pumpAndSettle();
    expect(h.server.calls.last.request.channelIds, [channelId(2)]);
  });

  testWidgets('the agent and the tag of a forum search again as parameters', (
    tester,
  ) async {
    final h = Host(tester);
    await h.open();
    await h.answer(0, page(const []));

    // Only the stored agents are offered, not the people.
    await tester.tap(find.byKey(const ValueKey('search.filter.agent')));
    await tester.pumpAndSettle();
    expect(find.text('Sam'), findsNothing);
    await tester.tap(find.text('dev-runner').last);
    await tester.pumpAndSettle();
    expect(h.server.calls.last.request.agentId, robin);

    // The tags are those of the chosen forum; without one there is nothing to choose.
    DropdownButton<ForumTagId?> tags() =>
        tester.widget(find.byKey(const ValueKey('search.filter.tag')));
    expect(tags().onChanged, isNull);
    await tester.tap(find.byKey(const ValueKey('search.filter.channel')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('ideas').last);
    await tester.pumpAndSettle();
    expect(tags().onChanged, isNotNull);
    await tester.tap(find.byKey(const ValueKey('search.filter.tag')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('idea').last);
    await tester.pumpAndSettle();
    final tagged = h.server.calls.last.request;
    expect(tagged.channelIds, [channelId(3)]);
    expect(tagged.tagIds, [tagId(2)]);
    expect(tagged.agentId, robin);

    // tag_id applies to threads and messages (14 4.4): files leave it out.
    await tester.tap(find.byKey(const ValueKey('search.filter.type')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(en.searchTypeFiles).last);
    await tester.pumpAndSettle();
    expect(h.server.calls.last.request.tagIds, isEmpty);
    expect(h.server.calls.last.request.agentId, robin);
    expect(tags().onChanged, isNull);

    // Another channel has other tags: the chosen tag goes with its forum.
    await tester.tap(find.byKey(const ValueKey('search.filter.type')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(en.searchTypeMessages).last);
    await tester.pumpAndSettle();
    expect(h.server.calls.last.request.tagIds, [tagId(2)]);
    await tester.tap(find.byKey(const ValueKey('search.filter.channel')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('releases').last);
    await tester.pumpAndSettle();
    expect(h.server.calls.last.request.channelIds, [channelId(2)]);
    expect(h.server.calls.last.request.tagIds, isEmpty);
  });

  testWidgets('segments with markup stay literal text', (tester) async {
    final h = Host(tester);
    await h.open();
    await h.answer(0, page([messageHit(message1.value, text: '<b>x</b>')]));
    expect(find.text('window <b>x</b>', findRichText: true), findsOneWidget);
  });
}
