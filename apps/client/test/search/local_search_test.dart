// SPDX-License-Identifier: Apache-2.0

// The offline search keeps the best hits of the filtered set (14 6; red team M1 of M4-23): the
// place and type filters run before the limit, and the filters that need the stored objects
// (author, time, attachment, thread state) read further pages until the limit is reached. Many
// better ranked rows of another channel or author never hide the stored hits.

import 'package:client/search/local_search.dart';
import 'package:client/search/search_port.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/structure.dart';
import '../support/thread_fixtures.dart';

void main() {
  late LocalDatabase db;
  const limit = 5;

  setUp(() async {
    db = openInMemoryForTests(clock: () => t0);
    await db.structureDao.upsertSpaceIfNewer(space(1));
    for (final n in [1, 2]) {
      await db.structureDao.upsertChannelIfNewer(channel(n));
      await db.threadsDao.upsertIfNewer(thread(n, channelN: n));
    }
    // More than [limit] short, better ranked rows by Sam in channel 1 (bm25 favours short
    // texts), then two long rows by Alex in channel 2.
    for (var seq = 1; seq <= 4 * limit; seq++) {
      final m = msg(seq, text: 'deploy');
      await db.messagesDao.upsertIfNewer(m);
      await db.searchDao.indexMessage(m, spaceId: spaceId(1));
    }
    for (var seq = 1; seq <= 2; seq++) {
      final m = msg(
        seq,
        threadN: 2,
        channelN: 2,
        author: alex,
        text: 'the deploy of the release train waits for the last review today',
      );
      await db.messagesDao.upsertIfNewer(m);
      await db.searchDao.indexMessage(m, spaceId: spaceId(1));
    }
  });
  tearDown(() => db.close());

  test(
    'a channel filter finds the hits of a channel ranked below the limit',
    () async {
      final hits = await LocalSearch(db).search(
        SearchRequest(q: 'deploy', channelIds: [channelId(2)]),
        limit: limit,
      );
      expect(hits.map((h) => h.channelId).toSet(), {channelId(2)});
      expect(hits, hasLength(2));
    },
  );

  test(
    'an author filter reads further pages until it finds the hits',
    () async {
      final hits = await LocalSearch(db).search(
        SearchRequest(q: 'deploy', authorId: alex),
        limit: limit,
      );
      expect(hits, hasLength(2));
      expect(hits.map((h) => h.authorId).toSet(), {alex});
    },
  );

  test('without a filter the limit holds', () async {
    final hits = await LocalSearch(db)
        .search(const SearchRequest(q: 'deploy'), limit: limit);
    expect(hits, hasLength(limit));
  });

  test(
    'the DAO filters kind and place in SQL and pages with the offset',
    () async {
      final first = await db.searchDao.search(
        'deploy',
        limit: 3,
        channelIds: {channelId(1)},
        kinds: {SearchKind.message},
      );
      final second = await db.searchDao.search(
        'deploy',
        limit: 3,
        offset: 3,
        channelIds: {channelId(1)},
        kinds: {SearchKind.message},
      );
      expect(first, hasLength(3));
      expect(second, hasLength(3));
      expect(
        {...first.map((h) => h.targetId)}
            .intersection({...second.map((h) => h.targetId)}),
        isEmpty,
      );
      expect(
        await db.searchDao.search(
          'deploy',
          limit: 3,
          kinds: {SearchKind.threadTitle},
        ),
        isEmpty,
      );
    },
  );
}
