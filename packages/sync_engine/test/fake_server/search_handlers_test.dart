// SPDX-License-Identifier: Apache-2.0

// The search handlers M4-23 added to the fake (brief M4-23 Scope): `search` with the filters of
// 14 4.4, the sorts of 4.5, segments (4.6) and cursor paging with `cursor_invalid` (4.7), the
// candidate limit, the prefix search of 4.9, `listMyMentions` and `listActiveRuns`. Only what
// the caller sees is returned, and every response passes the contract check of the fake.

import 'package:api_client/api_client.dart';
import 'package:built_collection/built_collection.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late FakeWorld world;
  late WorldSeed w;
  late SearchApi alexSearch;
  late SearchApi samSearch;
  late ApiClient alexApi;
  late ApiClient samApi;

  setUp(() async {
    h = await FakeWorldHarness.start();
    world = h.server.world;
    w = h.seed;
    final alex = h.device();
    await signedIn(h, alex);
    final sam = h.device(login: WorldSeed.samLogin, keySeed: 2);
    await signedIn(h, sam);
    alexApi = alex.api;
    samApi = sam.api;
    alexSearch = alexApi.getSearchApi();
    samSearch = samApi.getSearchApi();
  });
  tearDown(() async {
    expect(h.server.violations, isEmpty);
    await h.stop();
  });

  /// The problem body of a failed [call] (status, code, errors).
  Future<Map<String, Object?>> problem(Future<Object?> call) => call.then(
    (_) => throw StateError('expected a problem'),
    onError: (Object e) {
      final d = e as DioException;
      final body = Map<String, Object?>.from(d.response!.data! as Map);
      return body..['status'] = d.response!.statusCode;
    },
  );

  JsonMap post(FakePerson author, String thread, String text) =>
      w.post(author, thread, text);

  /// Sets the `created_at` of [message] so time filters and order are deterministic.
  void at(JsonMap message, String stamp) => message['created_at'] = stamp;

  List<String> ids(SearchHitPage page) => [
    for (final hit in page.items)
      hit.message?.id ?? hit.thread?.id ?? hit.file!.id,
  ];

  group('search', () {
    test('finds visible messages with segments, names and score', () async {
      final hit = post(w.sam, w.releaseNotes, 'The invoice export fails');
      post(w.sam, w.budget, 'Invoice of the private budget');
      post(w.alex, w.printer, 'The printer is empty');

      final page = (await alexSearch.search(q: 'INVOICE')).data!;
      expect(ids(page), [hit['id']]);
      final first = page.items.single;
      expect(first.kind, SearchHitKind.message);
      expect(first.threadTitle, 'Release notes');
      expect(first.channelName, 'general');
      expect(first.spaceName, 'Team');
      expect(first.score, 1);
      expect(
        [for (final s in first.segments) (s.text, s.match)],
        [('The ', false), ('invoice', true), (' export fails', false)],
      );
      expect(page.candidatesCapped, isFalse);
      expect(page.nextCursor, isNull);

      // Sam sees the private channel, Alex does not.
      final sam = (await samSearch.search(q: 'invoice')).data!;
      expect(sam.items, hasLength(2));
    });

    test('every word must occur; deleted messages are not found', () async {
      final both = post(w.alex, w.releaseNotes, 'Invoice export broken');
      post(w.alex, w.releaseNotes, 'Invoice only');
      final gone = post(w.alex, w.releaseNotes, 'Invoice export again');
      world.deleteMessage(w.alex.id, gone['id']! as String);

      final page = (await alexSearch.search(q: 'invoice export')).data!;
      expect(ids(page), [both['id']]);
    });

    test('an empty query gives empty, a long one too_long', () async {
      final empty = await problem(alexSearch.search(q: '  ?! -- '));
      expect(empty['status'], 400);
      expect(empty['code'], 'validation_failed');
      expect(empty['errors'], [
        {'pointer': '#query/q', 'code': 'empty'},
      ]);

      world.searchSettings.queryMaxChars = 5;
      final long = await problem(alexSearch.search(q: 'invoices'));
      expect(long['code'], 'validation_failed');
      expect(long['errors'], [
        {'pointer': '#query/q', 'code': 'too_long'},
      ]);
    });

    test('a filter outside its types gives not_applicable', () async {
      Future<void> expectNotApplicable(
        Future<Object?> call,
        String name,
      ) async {
        final p = await problem(call);
        expect(p['code'], 'validation_failed');
        expect(p['errors'], [
          {'pointer': '#query/$name', 'code': 'not_applicable'},
        ]);
      }

      await expectNotApplicable(
        alexSearch.search(q: 'x', type: 'threads', authorId: w.alex.id),
        'author_id',
      );
      await expectNotApplicable(
        alexSearch.search(q: 'x', type: 'files', hasFile: true),
        'has_file',
      );
      await expectNotApplicable(
        alexSearch.search(q: 'x', type: 'files', threadState: ThreadState.open),
        'thread_state',
      );
      await expectNotApplicable(
        alexSearch.search(
          q: 'x',
          type: 'files',
          tagId: BuiltList([world.nextId()]),
        ),
        'tag_id',
      );
    });

    test('the filters of 14 4.4 narrow the hits', () async {
      final a = post(w.alex, w.releaseNotes, 'Report one');
      final b = post(w.sam, w.printer, 'Report two');
      final c = post(w.sam, w.releaseNotes, 'Report three');
      at(a, '2026-10-01T10:00:00.000Z');
      at(b, '2026-10-02T10:00:00.000Z');
      at(c, '2026-10-03T10:00:00.000Z');

      Future<List<String>> run({
        BuiltList<String>? spaceId,
        BuiltList<String>? channelId,
        String? threadId,
        String? authorId,
        String? agentId,
        DateTime? after,
        DateTime? before,
        bool? hasFile,
        ThreadState? threadState,
      }) async => ids(
        (await alexSearch.search(
          q: 'report',
          sort: 'newest',
          spaceId: spaceId,
          channelId: channelId,
          threadId: threadId,
          authorId: authorId,
          agentId: agentId,
          after: after,
          before: before,
          hasFile: hasFile,
          threadState: threadState,
        )).data!,
      );

      expect(await run(), [c['id'], b['id'], a['id']]);
      expect(await run(spaceId: BuiltList([w.space])), hasLength(3));
      expect(await run(spaceId: BuiltList([world.nextId()])), isEmpty);
      expect(await run(channelId: BuiltList([w.support])), [b['id']]);
      expect(await run(channelId: BuiltList([w.support, w.general])), [
        c['id'],
        b['id'],
        a['id'],
      ]);
      // An invisible channel in the filter yields no hits, not an error.
      expect(await run(channelId: BuiltList([w.private])), isEmpty);
      expect(await run(threadId: w.releaseNotes), [c['id'], a['id']]);
      expect(await run(authorId: w.sam.id), [c['id'], b['id']]);
      expect(
        await run(
          after: DateTime.utc(2026, 10, 2, 10),
          before: DateTime.utc(2026, 10, 3, 10),
        ),
        [b['id']],
      );
      expect(await run(hasFile: true), isEmpty);
      expect(await run(hasFile: false), hasLength(3));
      world.setThreadState(w.sam.id, w.printer, closed: true);
      expect(await run(threadState: ThreadState.closed), [b['id']]);
      world.setChannelOperator(w.general, w.robin.id);
      expect(await run(agentId: w.robin.id), [c['id'], a['id']]);
    });

    test('relevance ranks by matched words, newest breaks a tie', () async {
      final twice = post(w.alex, w.releaseNotes, 'printer printer jam');
      final older = post(w.alex, w.releaseNotes, 'printer once');
      final newer = post(w.alex, w.releaseNotes, 'printer again');
      at(twice, '2026-10-01T10:00:00.000Z');
      at(older, '2026-10-02T10:00:00.000Z');
      at(newer, '2026-10-03T10:00:00.000Z');

      final relevance = (await alexSearch.search(q: 'printer')).data!;
      expect(ids(relevance), [twice['id'], newer['id'], older['id']]);
      expect(relevance.items.first.score, 2);

      final newest = (await alexSearch.search(
        q: 'printer',
        sort: 'newest',
      )).data!;
      expect(ids(newest), [newer['id'], older['id'], twice['id']]);
      expect(newest.items.first.score, isNull);
    });

    test('pages with a cursor bound to the query', () async {
      final posted = [
        for (var i = 0; i < 5; i++) post(w.alex, w.releaseNotes, 'Paging $i'),
      ];
      final seen = <String>[];
      String? cursor;
      do {
        final page = (await alexSearch.search(
          q: 'paging',
          limit: 2,
          cursor: cursor,
        )).data!;
        expect(page.items.length, lessThanOrEqualTo(2));
        seen.addAll(ids(page));
        cursor = page.nextCursor;
      } while (cursor != null);
      expect(seen.toSet(), {for (final m in posted) m['id']});
      expect(seen, hasLength(5));

      final first = (await alexSearch.search(q: 'paging', limit: 2)).data!;
      final changed = await problem(
        alexSearch.search(
          q: 'paging',
          sort: 'newest',
          cursor: first.nextCursor,
        ),
      );
      expect(changed['status'], 400);
      expect(changed['code'], 'cursor_invalid');
      final other = await problem(
        alexSearch.search(q: 'other', cursor: first.nextCursor),
      );
      expect(other['code'], 'cursor_invalid');
      final garbage = await problem(
        alexSearch.search(q: 'paging', cursor: 'not-a-cursor'),
      );
      expect(garbage['code'], 'cursor_invalid');
      // Another person cannot reuse Alex's cursor (14 4.7).
      final sam = await problem(
        samSearch.search(q: 'paging', limit: 2, cursor: first.nextCursor),
      );
      expect(sam['code'], 'cursor_invalid');
      // The page size is not part of the hash.
      final larger = (await alexSearch.search(
        q: 'paging',
        limit: 10,
        cursor: first.nextCursor,
      )).data!;
      expect(larger.items, hasLength(3));
    });

    test('reports candidates_capped above the candidate limit', () async {
      for (var i = 0; i < 3; i++) {
        post(w.alex, w.releaseNotes, 'Capped $i');
      }
      world.searchSettings.maxCandidates = 2;
      final page = (await alexSearch.search(q: 'capped')).data!;
      expect(page.candidatesCapped, isTrue);
      expect(page.items, hasLength(2));
      world.searchSettings.maxCandidates = 3;
      expect(
        (await alexSearch.search(q: 'capped')).data!.candidatesCapped,
        isFalse,
      );
    });

    test('type threads matches visible titles', () async {
      final page = (await alexSearch.search(
        q: 'printer',
        type: 'threads',
      )).data!;
      expect(ids(page), [w.printer]);
      final hit = page.items.single;
      expect(hit.kind, SearchHitKind.thread);
      expect(hit.threadTitle, 'Printer');
      expect(hit.channelName, 'support');
      expect(
        [for (final s in hit.segments) (s.text, s.match)],
        [('Printer', true)],
      );
      expect(
        (await alexSearch.search(q: 'budget', type: 'threads')).data!.items,
        isEmpty,
      );
      expect(
        ids(
          (await samSearch.search(
            q: 'budget',
            type: 'threads',
            threadState: ThreadState.open,
          )).data!,
        ),
        [w.budget],
      );
    });

    test('type files matches the name and the transcript', () async {
      final report = world.addReadyFile(
        uploaderId: w.alex.id,
        channelId: w.general,
        name: 'quarterly-report.pdf',
        mimeType: 'application/pdf',
        bytes: [1, 2, 3],
      );
      final voice = world.addReadyFile(
        uploaderId: w.alex.id,
        channelId: w.general,
        name: 'voice.m4a',
        mimeType: 'audio/mp4',
        bytes: [4, 5, 6],
      );
      final hidden = world.addReadyFile(
        uploaderId: w.sam.id,
        channelId: w.private,
        name: 'private-report.pdf',
        mimeType: 'application/pdf',
        bytes: [7],
      );
      // Not attached: not found by anyone but never through search (14 2).
      world.addReadyFile(
        uploaderId: w.alex.id,
        channelId: w.general,
        name: 'loose-report.pdf',
        mimeType: 'application/pdf',
        bytes: [8],
      );
      world
        ..createMessage(w.alex.id, w.releaseNotes, {
          'client_msg_id': world.nextId(),
          'content_md': 'See the attachment',
          'attachment_file_ids': [report],
        })
        ..createMessage(w.alex.id, w.releaseNotes, {
          'client_msg_id': world.nextId(),
          'kind': 'voice',
          'attachment_file_ids': [voice],
        })
        ..createMessage(w.sam.id, w.budget, {
          'client_msg_id': world.nextId(),
          'content_md': 'Private',
          'attachment_file_ids': [hidden],
        });

      final byName = (await alexSearch.search(
        q: 'report',
        type: 'files',
      )).data!;
      expect(ids(byName), [report]);
      expect(byName.items.single.kind, SearchHitKind.file);
      expect(byName.items.single.threadTitle, 'Release notes');

      final seq = world.headSeq;
      world.setTranscript(voice, 'Call the customer tomorrow', language: 'en');
      expect(world.log.last.type, 'file.updated');
      expect(world.headSeq, greaterThan(seq));
      final transcript = (await alexApi.getFilesApi().getTranscript(
        fileId: voice,
      )).data!;
      expect(transcript.status, TranscriptStatus.done);
      expect(transcript.text, 'Call the customer tomorrow');
      expect(transcript.language, 'en');

      final byTranscript = (await alexSearch.search(
        q: 'customer',
        type: 'files',
        authorId: w.alex.id,
      )).data!;
      expect(ids(byTranscript), [voice]);
      expect(
        byTranscript.items.single.segments
            .where((s) => s.match)
            .map((s) => s.text),
        ['customer'],
      );
      expect(
        (await alexSearch.search(q: 'customer')).data!.items,
        isEmpty,
        reason: 'a transcript is a file hit, not a message hit (14 2)',
      );
    });

    test('has_file keeps messages with attachments', () async {
      final file = world.addReadyFile(
        uploaderId: w.alex.id,
        channelId: w.general,
        name: 'notes.txt',
        mimeType: 'text/plain',
        bytes: [1],
      );
      final withFile = world.createMessage(w.alex.id, w.releaseNotes, {
        'client_msg_id': world.nextId(),
        'content_md': 'Minutes attached',
        'attachment_file_ids': [file],
      }).message;
      final without = post(w.alex, w.releaseNotes, 'Minutes follow');
      expect(
        ids((await alexSearch.search(q: 'minutes', hasFile: true)).data!),
        [withFile['id']],
      );
      expect(
        ids((await alexSearch.search(q: 'minutes', hasFile: false)).data!),
        [without['id']],
      );
    });
  });

  group('searchThreadTitles', () {
    test('matches visible titles by prefix, newest activity first', () async {
      final notes = world.addThread(
        w.support,
        'Release planning',
        createdBy: w.sam.id,
      );
      world.threads[w.releaseNotes]!['last_activity_at'] =
          '2026-10-01T10:00:00.000Z';
      world.threads[notes]!['last_activity_at'] = '2026-10-02T10:00:00.000Z';
      world.addThread(w.private, 'Release secrets', createdBy: w.sam.id);

      final hits = (await alexSearch.searchThreadTitles(q: 'rel')).data!;
      expect([for (final t in hits.items) t.threadId], [notes, w.releaseNotes]);
      expect(hits.items.first.title, 'Release planning');
      expect(hits.items.first.channelId, w.support);

      expect(
        [
          for (final t in (await alexSearch.searchThreadTitles(
            q: 'rel no',
          )).data!.items)
            t.threadId,
        ],
        [w.releaseNotes],
      );
      expect(
        (await alexSearch.searchThreadTitles(q: 'rel', limit: 1)).data!.items,
        hasLength(1),
      );
      // Words shorter than search.prefix_min_chars are dropped; nothing left, nothing found.
      expect(
        (await alexSearch.searchThreadTitles(q: 'r')).data!.items,
        isEmpty,
      );
      world.searchSettings.prefixMinChars = 1;
      expect(
        (await alexSearch.searchThreadTitles(q: 'r')).data!.items,
        hasLength(2),
      );
    });
  });

  group('listMyMentions', () {
    JsonMap mention(FakePerson author, String thread, FakePerson whom) =>
        world.createMessage(author.id, thread, {
          'client_msg_id': world.nextId(),
          'content_md': '@${whom.handle} please look',
          'mention_ids': [whom.id],
        }).message;

    test(
      'lists visible, undeleted mentions newest first with paging',
      () async {
        final a = mention(w.sam, w.releaseNotes, w.alex);
        final b = mention(w.sam, w.printer, w.alex);
        final c = mention(w.alex, w.printer, w.alex);
        final gone = mention(w.sam, w.printer, w.alex);
        mention(w.sam, w.releaseNotes, w.sam);
        at(a, '2026-10-01T10:00:00.000Z');
        at(b, '2026-10-02T10:00:00.000Z');
        at(c, '2026-10-03T10:00:00.000Z');
        world.deleteMessage(w.sam.id, gone['id']! as String);
        // A mention in a channel Alex does not see stays out of the list.
        final hidden = mention(w.sam, w.budget, w.alex);

        final me = alexApi.getMeApi();
        final seen = <String>[];
        String? cursor;
        do {
          final page = (await me.listMyMentions(
            limit: 2,
            cursor: cursor,
          )).data!;
          for (final item in page.items) {
            expect(item.createdAt, item.message.createdAt);
            seen.add(item.message.id);
          }
          cursor = page.nextCursor;
        } while (cursor != null);
        expect(seen, [c['id'], b['id'], a['id']]);
        expect(seen, isNot(contains(hidden['id'])));

        final bad = await problem(me.listMyMentions(cursor: 'nope'));
        expect(bad['code'], 'cursor_invalid');
      },
    );
  });

  group('listActiveRuns', () {
    test(
      'lists active visible runs as RunSummary with filter and paging',
      () async {
        final m1 = post(w.alex, w.releaseNotes, 'Run one');
        final m2 = post(w.alex, w.printer, 'Run two');
        final m3 = post(w.sam, w.budget, 'Run hidden');
        final m4 = post(w.alex, w.printer, 'Run done');
        final queued = world.createRun(
          agentId: w.robin.id,
          triggeringMessageId: m1['id']! as String,
          triggeredBy: w.alex.id,
        );
        final other = world.addPerson(
          handle: 'helper',
          displayName: 'Helper',
          kind: 'agent',
        );
        final running = world.createRun(
          agentId: other.id,
          triggeringMessageId: m2['id']! as String,
          triggeredBy: w.alex.id,
        );
        world.setRunState(running['id']! as String, 'running');
        final waiting = world.createRun(
          agentId: w.robin.id,
          triggeringMessageId: m4['id']! as String,
          triggeredBy: w.alex.id,
        );
        world.setRunState(waiting['id']! as String, 'waiting_input');
        final hidden = world.createRun(
          agentId: w.robin.id,
          triggeringMessageId: m3['id']! as String,
          triggeredBy: w.sam.id,
        );
        final ended = world.createRun(
          agentId: w.robin.id,
          triggeringMessageId: m1['id']! as String,
          triggeredBy: w.alex.id,
        );
        world.setRunState(ended['id']! as String, 'completed');

        final runs = alexApi.getRunsApi();
        final seen = <String>[];
        String? cursor;
        do {
          final page = (await runs.listActiveRuns(
            limit: 2,
            cursor: cursor,
          )).data!;
          seen.addAll([for (final r in page.items) r.id]);
          cursor = page.nextCursor;
        } while (cursor != null);
        expect(seen.toSet(), {queued['id'], running['id'], waiting['id']});
        expect(seen, hasLength(3));
        expect(seen, isNot(contains(hidden['id'])));

        final robin = (await runs.listActiveRuns(agentId: w.robin.id)).data!;
        expect(
          {for (final r in robin.items) r.id},
          {queued['id'], waiting['id']},
        );
        expect(robin.items.first.agentId, w.robin.id);

        final samRuns = (await samApi.getRunsApi().listActiveRuns()).data!;
        expect({for (final r in samRuns.items) r.id}, contains(hidden['id']));

        final first = (await runs.listActiveRuns(limit: 1)).data!;
        final bad = await problem(
          runs.listActiveRuns(agentId: w.robin.id, cursor: first.nextCursor),
        );
        expect(bad['code'], 'cursor_invalid');
      },
    );
  });
}
