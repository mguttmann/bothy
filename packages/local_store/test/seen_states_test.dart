// SPDX-License-Identifier: Apache-2.0

// The seen marks of the sidebar status dots (01 6.17, brief M4-52 Design note 3): the run and
// state of a chat the person last had open on this device (`thread_seen_states`), local only,
// replaced by every new mark and deleted with the thread.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  final runA = RunId.parse(uuid(0x701));
  final runB = RunId.parse(uuid(0x702));

  test('without a mark no thread is seen', () async {
    expect(await db.forumViewDao.watchSeenStates().first, isEmpty);
  });

  test('a mark is stored and a newer one replaces it', () async {
    final dao = db.forumViewDao;
    await dao.markSeen(threadId(1), runA, RunState.running);
    expect(await dao.watchSeenStates().first, {
      threadId(1): ThreadSeenState(
        threadId: threadId(1),
        runId: runA,
        runState: RunState.running,
      ),
    });

    await dao.markSeen(threadId(1), runB, RunState.failed);
    await dao.markSeen(threadId(2), runA, RunState.completed);
    final marks = await dao.watchSeenStates().first;
    expect(marks[threadId(1)]!.runId, runB);
    expect(marks[threadId(1)]!.runState, RunState.failed);
    expect(marks[threadId(2)]!.runState, RunState.completed);
  });

  test('the watch emits a new mark', () async {
    final dao = db.forumViewDao;
    final sizes = <int>[];
    final sub = dao.watchSeenStates().listen((m) => sizes.add(m.length));
    await pumpEventQueue();
    await dao.markSeen(threadId(1), runA, RunState.waitingInput);
    await pumpEventQueue();
    await sub.cancel();
    expect(sizes, [0, 1]);
  });

  test('the removal of a thread deletes its mark', () async {
    final dao = db.forumViewDao;
    await db.structureDao.upsertSpaceIfNewer(space(1));
    await db.structureDao.upsertChannelIfNewer(channel(1));
    await db.threadsDao.upsertIfNewer(thread(1));
    await db.threadsDao.upsertIfNewer(thread(2));
    await dao.markSeen(threadId(1), runA, RunState.completed);
    await dao.markSeen(threadId(2), runB, RunState.completed);

    await db.removeTargets([TargetRef.thread(threadId(1))]);

    expect((await dao.watchSeenStates().first).keys, [threadId(2)]);
  });

  test('the removal of a channel deletes the marks of its threads', () async {
    final dao = db.forumViewDao;
    await db.structureDao.upsertSpaceIfNewer(space(1));
    await db.structureDao.upsertChannelIfNewer(channel(1));
    await db.threadsDao.upsertIfNewer(thread(1));
    await dao.markSeen(threadId(1), runA, RunState.completed);

    await db.removeTargets([TargetRef.channel(channel(1).id)]);

    expect(await dao.watchSeenStates().first, isEmpty);
  });
}
