// SPDX-License-Identifier: Apache-2.0

// The cache cleanup of 15 6.3 with a fake clock: threads without access since
// `client.cache_max_age` go, the byte limit removes the least recently used first, and outbox,
// drafts, share inbox and the structure rows are never touched.

import 'package:domain/domain.dart';
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

  /// Thread [n] in channel 1 with one message and a search row; reaches the device now.
  Future<void> addThread(int n) async {
    await db.threadsDao.upsertIfNewer(thread(n, lastSeq: 1));
    final m = message(n, 1, text: 'Note $n from Sam');
    await db.messagesDao.upsertIfNewer(m);
    await db.searchDao.indexMessage(m);
  }

  /// A cached original of file [n] with [size] bytes, accessed now.
  Future<void> addCachedFile(int n, int size) => db.fileCacheDao.put(
    FileCacheEntry(
      fileId: file(n).id,
      variant: 'original',
      path: 'cache/file-$n',
      sizeBytes: size,
      lastAccessedAt: clock(),
    ),
  );

  CacheCleaner cleaner({
    Duration maxAge = const Duration(days: 30),
    int maxBytes = 1 << 40,
  }) => CacheCleaner(db, maxAge: maxAge, maxBytes: maxBytes, now: clock.call);

  test('old threads go, accessed ones stay', () async {
    await db.structureDao.upsertSpaceIfNewer(space(1));
    await db.structureDao.upsertChannelIfNewer(channel(1));
    await addThread(1);
    await addThread(2);
    await addCachedFile(1, 10);
    clock.advance(const Duration(days: 6));
    await db.threadsDao.touch(threadId(2));
    clock.advance(const Duration(days: 3));

    final paths = await cleaner(maxAge: const Duration(days: 7)).run();

    expect(await db.threadsDao.byId(threadId(1)), isNull);
    expect(await db.threadsDao.byId(threadId(2)), isNotNull);
    expect(await db.messagesDao.pageBefore(threadId(1), null, 10), isEmpty);
    expect(
      await db.messagesDao.pageBefore(threadId(2), null, 10),
      hasLength(1),
    );
    final hits = await db.searchDao.search('note', limit: 10);
    expect(hits.map((h) => h.threadId), [threadId(2)]);
    expect(paths, [
      'cache/file-1',
    ], reason: 'the file was not opened for 9 days');
    expect(await db.structureDao.spaceById(space(1).id), isNotNull);
    expect(await db.structureDao.channelById(channel(1).id), isNotNull);
  });

  test('the byte limit removes the least recently used first', () async {
    const gigabyte = 1 << 30;
    await addThread(1); // accessed at 0
    clock.advance(const Duration(minutes: 1));
    await addCachedFile(2, gigabyte); // accessed at 1
    clock.advance(const Duration(minutes: 1));
    await addCachedFile(3, gigabyte); // accessed at 2

    final paths = await cleaner(maxBytes: gigabyte + gigabyte ~/ 2).run();

    expect(await db.threadsDao.byId(threadId(1)), isNull);
    expect(paths, ['cache/file-2']);
    expect(await db.fileCacheDao.entry(file(3).id, 'original'), isNotNull);
  });

  test('outbox, drafts and share inbox are never touched', () async {
    await db.structureDao.upsertSpaceIfNewer(space(1));
    await db.structureDao.upsertChannelIfNewer(channel(1));
    await addThread(1);
    await db.personalDao.upsertDraftIfNewer(draft(1));
    await db.outboxDao.enqueue(
      id: uuid(1),
      kind: 'postMessage',
      threadId: threadId(1),
      payload: '{"content_md":"Hello from Robin"}',
    );
    await db.shareInboxDao.add(
      ShareInboxEntry(
        id: uuid(2),
        payload: '{}',
        filePaths: const ['share/photo.png'],
        createdAt: at(0),
      ),
    );
    final drafts = await db.personalDao.allDrafts();
    final outbox = await db.outboxDao.entriesFor(threadId(1));
    final shares = await db.shareInboxDao.all();
    clock.advance(const Duration(days: 365));

    await cleaner(maxAge: const Duration(days: 1), maxBytes: 0).run();

    expect(await db.threadsDao.byId(threadId(1)), isNull);
    expect(await db.personalDao.allDrafts(), drafts);
    expect(await db.outboxDao.entriesFor(threadId(1)), outbox);
    expect(await db.shareInboxDao.all(), shares);
    expect(await db.structureDao.spaceById(space(1).id), isNotNull);
    expect(await db.structureDao.channelById(channel(1).id), isNotNull);
  });

  test('the limits come from AppConfig', () {
    final config = AppConfig.defaults();
    final c = CacheCleaner.fromConfig(db, config, now: clock.call);
    expect(c.maxAge, config.cacheMaxAge);
    expect(c.maxBytes, config.cacheMaxBytes);
  });
}
