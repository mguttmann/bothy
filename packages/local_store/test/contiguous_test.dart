// SPDX-License-Identifier: Apache-2.0

// The contiguous window of a thread (15 7.3 item 4, 04 4): messages after a gap stay hidden
// until their range is loaded.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  List<int> seqs(ContiguousWindow w) => [
    for (final m in w.messages) m.threadSeq.value,
  ];

  test(
    'watchContiguous hides messages after a gap until the range is loaded',
    () async {
      final dao = db.messagesDao;
      await db.threadsDao.upsertIfNewer(thread(1, lastSeq: 6));
      for (final seq in [1, 2, 3, 6]) {
        await dao.upsertIfNewer(message(1, seq));
      }
      await dao.addLoadedRange(
        threadId(1),
        const ThreadSeq(1),
        const ThreadSeq(3),
      );
      final stream = dao.watchContiguous(threadId(1), limit: 50);
      final first = await stream.first;
      expect(seqs(first), [1, 2, 3]);
      expect(first.gapBefore, isNull);
      expect(first.gapAfter, SeqRange(const ThreadSeq(4), const ThreadSeq(6)));

      for (final seq in [4, 5]) {
        await dao.upsertIfNewer(message(1, seq));
      }
      await dao.addLoadedRange(
        threadId(1),
        const ThreadSeq(4),
        const ThreadSeq(6),
      );
      final loaded = await dao.contiguous(threadId(1), limit: 50);
      expect(seqs(loaded), [1, 2, 3, 4, 5, 6]);
      expect(loaded.gapAfter, isNull);
      expect((await dao.loadedRanges(threadId(1))).ranges, [
        SeqRange(const ThreadSeq(1), const ThreadSeq(6)),
      ]);
    },
  );

  test('a newer range after a gap shows the gap before it and limits to the newest', () async {
    final dao = db.messagesDao;
    for (final seq in [1, 2, 7, 8, 9]) {
      await dao.upsertIfNewer(message(1, seq));
    }
    await dao.addLoadedRange(
      threadId(1),
      const ThreadSeq(1),
      const ThreadSeq(2),
    );
    await dao.addLoadedRange(
      threadId(1),
      const ThreadSeq(7),
      const ThreadSeq(9),
    );
    final window = await dao.contiguous(threadId(1), limit: 2);
    expect(seqs(window), [8, 9]);
    expect(window.gapBefore, SeqRange(const ThreadSeq(3), const ThreadSeq(6)));
    expect(window.gapAfter, isNull);
  });

  test(
    'nothing loaded gives an empty window; pages read the stored messages',
    () async {
      final dao = db.messagesDao;
      expect((await dao.contiguous(threadId(1), limit: 5)).messages, isEmpty);
      for (final seq in [1, 2, 3, 4]) {
        await dao.upsertIfNewer(message(1, seq));
      }
      expect(
        [
          for (final m in await dao.pageBefore(
            threadId(1),
            const ThreadSeq(4),
            2,
          ))
            m.threadSeq.value,
        ],
        [2, 3],
      );
      expect(
        [
          for (final m in await dao.pageBefore(threadId(1), null, 10))
            m.threadSeq.value,
        ],
        [1, 2, 3, 4],
      );
    },
  );

  test('watchContiguous emits again when a range is loaded later', () async {
    final dao = db.messagesDao;
    await db.threadsDao.upsertIfNewer(thread(1, lastSeq: 3));
    for (final seq in [1, 3]) {
      await dao.upsertIfNewer(message(1, seq));
    }
    await dao.addLoadedRange(
      threadId(1),
      const ThreadSeq(1),
      const ThreadSeq(1),
    );
    final seen = <List<int>>[];
    final sub = dao
        .watchContiguous(threadId(1), limit: 50)
        .listen((w) => seen.add(seqs(w)));
    await pumpEventQueue();
    await dao.upsertIfNewer(message(1, 2));
    await dao.addLoadedRange(
      threadId(1),
      const ThreadSeq(2),
      const ThreadSeq(3),
    );
    await pumpEventQueue();
    await sub.cancel();
    expect(seen.first, [1]);
    expect(seen.last, [1, 2, 3]);
  });

  test(
    'watchContiguous emits no new window for a write outside its thread',
    () async {
      final dao = db.messagesDao;
      await db.threadsDao.upsertIfNewer(thread(1, lastSeq: 2));
      await db.threadsDao.upsertIfNewer(thread(2, lastSeq: 1));
      for (final seq in [1, 2]) {
        await dao.upsertIfNewer(message(1, seq));
      }
      await dao.addLoadedRange(
        threadId(1),
        const ThreadSeq(1),
        const ThreadSeq(2),
      );
      final seen = <ContiguousWindow>[];
      final sub = dao.watchContiguous(threadId(1), limit: 50).listen(seen.add);
      await pumpEventQueue();
      expect(seen, hasLength(1));

      // Thread 2 stores a message, loads a range and moves its activity: each of the three
      // tables the window reads changes, nothing of thread 1 does.
      await dao.upsertIfNewer(message(2, 1));
      await dao.addLoadedRange(
        threadId(2),
        const ThreadSeq(1),
        const ThreadSeq(1),
      );
      await db.threadsDao.advanceActivity(threadId(2), at(90));
      await pumpEventQueue();
      expect(seen, hasLength(1), reason: 'the window of thread 1 is unchanged');

      // A change of its own thread still arrives.
      await dao.upsertIfNewer(
        message(1, 2, version: 2, text: 'Edited by Alex'),
      );
      await pumpEventQueue();
      expect(seen, hasLength(2));
      expect(seen.last.messages.last.contentMd, 'Edited by Alex');
      await sub.cancel();
    },
  );

  test('two windows with the same messages and gaps are equal', () async {
    final dao = db.messagesDao;
    await db.threadsDao.upsertIfNewer(thread(1, lastSeq: 5));
    for (final seq in [2, 3]) {
      await dao.upsertIfNewer(message(1, seq));
    }
    await dao.addLoadedRange(
      threadId(1),
      const ThreadSeq(2),
      const ThreadSeq(3),
    );
    final a = await dao.contiguous(threadId(1), limit: 50);
    final b = await dao.contiguous(threadId(1), limit: 50);
    expect(a, b);
    expect(a.hashCode, b.hashCode);
    expect(a, isNot(await dao.contiguous(threadId(1), limit: 1)));
    await dao.applyPin(messageId(1, 3), pinned: true);
    expect(a, isNot(await dao.contiguous(threadId(1), limit: 50)));
  });

  test('watchContiguous keeps its own tables while the thread list of the channel is watched', () async {
    final dao = db.messagesDao;
    await db.threadsDao.upsertIfNewer(thread(1, lastSeq: 3));
    for (final seq in [1, 3]) {
      await dao.upsertIfNewer(message(1, seq));
    }
    await dao.addLoadedRange(
      threadId(1),
      const ThreadSeq(1),
      const ThreadSeq(1),
    );
    // Both trigger queries were the same SQL text, which drift shares as one stream: the
    // list's tables then decided when the window was read again.
    final list = db.threadsDao
        .watchForChannel(thread(1).channelId)
        .listen((_) {});
    final seen = <List<int>>[];
    final sub = dao
        .watchContiguous(threadId(1), limit: 50)
        .listen((w) => seen.add(seqs(w)));
    await pumpEventQueue();
    await dao.upsertIfNewer(message(1, 2));
    await dao.addLoadedRange(
      threadId(1),
      const ThreadSeq(2),
      const ThreadSeq(3),
    );
    await pumpEventQueue();
    await sub.cancel();
    await list.cancel();
    expect(seen.last, [1, 2, 3]);
  });
}
