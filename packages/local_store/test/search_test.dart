// SPDX-License-Identifier: Apache-2.0

// Offline search (14 6, Design note 5): FTS5 with `unicode61 remove_diacritics 2`, `bm25`
// ordering, `snippet` marks, and no hits after the removal of a target.

import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  test('FTS5 is compiled in', () async {
    final row = await db
        .customSelect("SELECT sqlite_compileoption_used('ENABLE_FTS5') AS on_")
        .getSingle();
    expect(row.read<int>('on_'), 1);
  });

  test(
    'German words with umlauts and accents are found without them',
    () async {
      await db.searchDao.indexMessage(
        message(1, 1, text: 'Größere Änderungen im Café übernommen'),
        spaceId: space(1).id,
      );
      for (final query in ['anderungen', 'Änderungen', 'cafe', 'ubernommen']) {
        final hits = await db.searchDao.search(query, limit: 10);
        expect(hits, hasLength(1), reason: query);
        expect(hits.single.kind, SearchKind.message);
        expect(hits.single.targetId, messageId(1, 1).value);
        expect(hits.single.threadId, threadId(1));
        expect(hits.single.channelId, channel(1).id);
        expect(hits.single.spaceId, space(1).id);
      }
    },
  );

  test('snippet marks the hit; bm25 puts the better match first', () async {
    await db.searchDao.indexMessage(
      message(1, 1, text: 'Robin deployed the release to example.com'),
    );
    await db.searchDao.indexMessage(
      message(1, 2, text: 'release release release notes by Sam'),
    );
    final hits = await db.searchDao.search('release', limit: 10);
    expect(hits, hasLength(2));
    expect(hits.first.targetId, messageId(1, 2).value);
    expect(hits.first.rank, lessThanOrEqualTo(hits.last.rank));
    expect(
      hits.last.snippet,
      contains('${SearchDaoMarks.open}release${SearchDaoMarks.close}'),
    );
    expect(hits.last.spaceId, isNull, reason: 'no space was given');
  });

  test(
    'thread titles and transcripts are found; operators are plain text',
    () async {
      await db.searchDao.indexThreadTitle(
        thread(1, title: 'Quarterly planning'),
      );
      await db.searchDao.indexTranscript(
        fileId: file(1).id,
        threadId: threadId(1),
        channelId: channel(1).id,
        text: 'Alex asks Sam about the planning budget',
      );
      final hits = await db.searchDao.search('planning', limit: 10);
      expect(hits.map((h) => h.kind).toSet(), {
        SearchKind.threadTitle,
        SearchKind.transcript,
      });
      expect(await db.searchDao.search('planning OR "', limit: 10), isEmpty);
      expect(await db.searchDao.search('   ', limit: 10), isEmpty);
    },
  );

  test('a tombstone and an edit replace the row of the message', () async {
    final m = message(1, 1, text: 'first words');
    await db.searchDao.indexMessage(m);
    await db.searchDao.indexMessage(message(1, 1, text: 'second words'));
    expect(await db.searchDao.search('first', limit: 10), isEmpty);
    expect(await db.searchDao.search('second', limit: 10), hasLength(1));
    await db.searchDao.indexMessage(m.copyWith(deletedAt: at(9)));
    expect(await db.searchDao.search('words', limit: 10), isEmpty);
  });

  test('removed targets are no longer found', () async {
    await db.structureDao.upsertSpaceIfNewer(space(1));
    await db.structureDao.upsertChannelIfNewer(channel(1));
    await db.structureDao.upsertChannelIfNewer(channel(2));
    await db.threadsDao.upsertIfNewer(thread(1, lastSeq: 1));
    await db.threadsDao.upsertIfNewer(thread(2, channelN: 2, lastSeq: 1));
    await db.searchDao.indexMessage(message(1, 1, text: 'shared word'));
    await db.searchDao.indexMessage(
      message(2, 1, channelN: 2, text: 'shared word'),
    );
    await db.removeTargets([TargetRef.channel(channel(1).id)]);
    final hits = await db.searchDao.search('shared', limit: 10);
    expect(hits.map((h) => h.channelId), [channel(2).id]);
    await db.searchDao.removeForTargets([TargetRef.thread(threadId(2))]);
    expect(await db.searchDao.search('shared', limit: 10), isEmpty);
  });
}
