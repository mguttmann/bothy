// SPDX-License-Identifier: Apache-2.0

// Offline search (14 6, Design note 5): FTS5 with `unicode61 remove_diacritics 2`, `bm25`
// ordering, `snippet` marks, no hits after the removal of a target, and `search_rows` as the
// twin of the index through every kind of write.

import 'package:domain/domain.dart';
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
      // A German sample is the point of this test (search folds umlauts and accents).
      // language-check: off
      await db.searchDao.indexMessage(
        message(1, 1, text: 'Größere Änderungen im Café übernommen'),
        spaceId: space(1).id,
      );
      for (final query in ['anderungen', 'Änderungen', 'cafe', 'ubernommen']) {
        // language-check: on
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
    await db.removeTargets([TargetRef.thread(threadId(2))]);
    expect(await db.searchDao.search('shared', limit: 10), isEmpty);
  });

  test('search_rows names exactly the rows of the index after every kind of '
      'write', () async {
    /// Both tables hold the same rows under the same rowid; returns how many.
    Future<int> twins() async {
      const columns = 'kind, target_id, thread_id, channel_id, space_id';
      final index = await db
          .customSelect(
            'SELECT rowid AS id, $columns FROM search_index ORDER BY rowid',
          )
          .get();
      final rows = await db
          .customSelect('SELECT id, $columns FROM search_rows ORDER BY id')
          .get();
      expect([for (final r in rows) r.data], [for (final r in index) r.data]);
      return rows.length;
    }

    await db.structureDao.upsertSpaceIfNewer(space(1));
    await db.structureDao.upsertChannelIfNewer(channel(1));
    await db.structureDao.upsertChannelIfNewer(channel(2));
    for (final n in [1, 2, 3]) {
      final t = thread(
        n,
        channelN: n == 3 ? 2 : 1,
        lastSeq: 2,
        title: 'Plan $n',
      );
      await db.threadsDao.upsertIfNewer(t);
      await db.searchDao.indexThreadTitle(t, spaceId: space(1).id);
      for (final seq in [1, 2]) {
        final m = message(n, seq, channelN: n == 3 ? 2 : 1, text: 'Note $seq');
        await db.messagesDao.upsertIfNewer(m);
        await db.searchDao.indexMessage(m, spaceId: space(1).id);
      }
    }
    await db.searchDao.indexTranscript(
      fileId: file(1).id,
      threadId: threadId(1),
      channelId: channel(1).id,
      text: 'Sam on the phone',
    );
    expect(await twins(), 10);

    // An edit replaces its row, an empty text and a tombstone remove theirs.
    await db.searchDao.indexMessage(message(1, 1, version: 2, text: 'Edited'));
    expect(await twins(), 10);
    await db.searchDao.indexMessage(message(1, 2, version: 2, text: ''));
    expect(await twins(), 9);
    await db.messagesDao.tombstoneWithFiles(
      messageId(2, 1),
      deletedAt: at(9),
      version: const ResourceVersion(2),
    );
    expect(await twins(), 8);
    // No stored message attaches the file of the transcript.
    expect(await db.searchDao.pruneTranscripts(), 1);
    expect(await twins(), 7);
    // The removal of a thread, then of a channel, then the resync.
    await db.removeTargets([TargetRef.thread(threadId(2))]);
    expect(await twins(), 5);
    await db.removeTargets([TargetRef.channel(channel(2).id)]);
    expect(await twins(), 2);
    expect(
      (await db.searchDao.search('edited plan', limit: 10)),
      isEmpty,
      reason: 'the title and the message of thread 1 are two rows',
    );
    expect(await db.searchDao.search('plan', limit: 10), hasLength(1));
    await db.syncStateDao.commitPage(
      (txn) => db.syncStateDao.clearMirror(txn, visibleChannels: const {}),
      cursor: const GlobalSeq(1),
    );
    expect(await twins(), 0);
  });
}
