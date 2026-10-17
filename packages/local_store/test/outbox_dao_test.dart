// SPDX-License-Identifier: Apache-2.0

// The outbox (15 7.5, 04 13.2): FIFO per thread, dependencies, edits, failures with a 412
// revision, and entries that survive close and reopen.

import 'dart:io';

import 'package:domain/domain.dart';
import 'package:drift/drift.dart' show driftRuntimeOptions;
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late FakeClock clock;
  late LocalDatabase db;
  setUp(() {
    clock = FakeClock();
    db = openInMemoryForTests(clock: clock.call);
  });
  tearDown(() => db.close());

  Future<void> add(String id, ThreadId? thread, {String? dependsOn}) async {
    await db.outboxDao.enqueue(
      id: id,
      kind: 'postMessage',
      payload: '{"text":"$id"}',
      threadId: thread,
      dependsOn: dependsOn,
    );
    clock.advance(const Duration(seconds: 1));
  }

  test('FIFO per thread across interleaved threads', () async {
    final dao = db.outboxDao;
    await add(uuid(1), threadId(1));
    await add(uuid(2), threadId(2));
    await add(uuid(3), threadId(1));
    await add(uuid(4), threadId(2));
    expect((await dao.nextForThread(threadId(1)))!.id, uuid(1));
    expect((await dao.nextForThread(threadId(2)))!.id, uuid(2));
    expect(await dao.pendingThreads(), [threadId(1), threadId(2)]);
    await dao.markState(uuid(1), OutboxState.sending);
    expect((await dao.nextForThread(threadId(1)))!.attempts, 1);
    await dao.markState(uuid(1), OutboxState.confirmed);
    expect((await dao.nextForThread(threadId(1)))!.id, uuid(3));
    await dao.replaceWithServer(uuid(1));
    expect(
      [for (final e in await dao.entriesFor(threadId(1))) e.id],
      [uuid(3)],
    );
  });

  test('equal creation times fall back to the ID', () async {
    await db.outboxDao.enqueue(
      id: uuid(9),
      kind: 'k',
      payload: '{}',
      threadId: threadId(1),
    );
    await db.outboxDao.enqueue(
      id: uuid(8),
      kind: 'k',
      payload: '{}',
      threadId: threadId(1),
    );
    expect((await db.outboxDao.nextForThread(threadId(1)))!.id, uuid(8));
  });

  test('a message waits for the upload it depends on', () async {
    final dao = db.outboxDao;
    await add(uuid(1), null);
    await add(uuid(2), threadId(1), dependsOn: uuid(1));
    expect(await dao.nextForThread(threadId(1)), isNull);
    expect((await dao.nextForThread(null))!.id, uuid(1));
    await dao.markState(uuid(1), OutboxState.confirmed);
    expect((await dao.nextForThread(threadId(1)))!.id, uuid(2));
    expect(await dao.pendingThreads(), [threadId(1)]);
  });

  test('updatePayload, waiting with a backoff, recordFailure with a 412 and delete', () async {
    final dao = db.outboxDao;
    await add(uuid(1), threadId(1));
    await dao.updatePayload(uuid(1), '{"text":"edited"}');
    await dao.markState(uuid(1), OutboxState.waiting, nextAttemptAt: at(30));
    var entry = (await dao.byId(uuid(1)))!;
    expect(entry.payload, '{"text":"edited"}');
    expect(entry.state, OutboxState.waiting);
    expect(entry.nextAttemptAt, at(30));
    await dao.recordFailure(
      uuid(1),
      code: 'precondition_failed',
      reason: 'version',
      conflictCurrent: '{"version":4}',
    );
    entry = (await dao.watchEntriesFor(threadId(1)).first).single;
    expect(entry.state, OutboxState.failed);
    expect(entry.state.isFinal, isTrue);
    expect(entry.lastErrorCode, 'precondition_failed');
    expect(entry.lastErrorReason, 'version');
    expect(entry.conflictCurrent, '{"version":4}');
    expect(await dao.nextForThread(threadId(1)), isNull);
    await dao.delete(uuid(1));
    expect(await dao.byId(uuid(1)), isNull);
  });

  test('updateDependsOn re-points an entry to another dependency', () async {
    final dao = db.outboxDao;
    await add(uuid(1), null);
    await add(uuid(2), threadId(1), dependsOn: uuid(1));
    await dao.updateDependsOn(uuid(2), uuid(3));
    expect((await dao.byId(uuid(2)))!.dependsOn, uuid(3));
    await dao.updateDependsOn(uuid(2), null);
    expect((await dao.byId(uuid(2)))!.dependsOn, isNull);
  });

  test('allEntries reads every entry of every thread, final ones included', () async {
    final dao = db.outboxDao;
    await add(uuid(1), threadId(1));
    await add(uuid(2), null);
    await add(uuid(3), threadId(2));
    await dao.recordFailure(uuid(1), code: 'validation_failed');
    // Thread 1 holds only a failed entry, so pendingThreads no longer names it.
    expect(await dao.pendingThreads(), [null, threadId(2)]);
    expect(
      [for (final e in await dao.allEntries()) e.id],
      [uuid(1), uuid(2), uuid(3)],
    );
  });

  test(
    'a new ID is a new row; an existing ID is never updated by enqueue',
    () async {
      await add(uuid(1), threadId(1));
      await expectLater(add(uuid(1), threadId(1)), throwsA(anything));
      expect(await db.outboxDao.entriesFor(threadId(1)), hasLength(1));
      expect(() => OutboxState.fromCode('other'), throwsFormatException);
    },
  );

  test('entries survive close and reopen', () async {
    // The in-memory database of setUp stays open next to the file; both have own executors.
    driftRuntimeOptions.dontWarnAboutMultipleDatabases = true;
    addTearDown(
      () => driftRuntimeOptions.dontWarnAboutMultipleDatabases = false,
    );
    final dir = await tempDir();
    addTearDown(() => dir.delete(recursive: true));
    final file = File('${dir.path}/store.db');
    var store = await openLocalStore(
      file: file,
      keys: FixedKeySource(testKeyA),
      inBackground: false,
      clock: clock.call,
    );
    await store.outboxDao.enqueue(
      id: uuid(1),
      kind: 'editMessage',
      payload: '{}',
      threadId: threadId(1),
      expectedVersion: const ResourceVersion(3),
    );
    final before = await store.outboxDao.entriesFor(threadId(1));
    await store.close();
    store = await openLocalStore(file: file, keys: FixedKeySource(testKeyA));
    expect(await store.outboxDao.entriesFor(threadId(1)), before);
    expect(before.single.expectedVersion, const ResourceVersion(3));
    await store.close();
  });
}
