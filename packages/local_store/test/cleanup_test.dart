// SPDX-License-Identifier: Apache-2.0

// The cache cleanup of 15 6.3 with a fake clock: threads without access since
// `client.cache_max_age` go, an open one loses its content and keeps its row with what the
// sidebar reads of it (01 6.8), the byte limit removes the least recently used first, and
// outbox, drafts, share inbox and the structure rows are never touched.

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

  /// Thread [n] in channel 1 with one message, its search row and the search row of its
  /// title; reaches the device now. A thread that is not [open] is done (01 6.1).
  Future<void> addThread(int n, {bool open = true}) async {
    final t = thread(
      n,
      lastSeq: 1,
      title: 'Plan $n',
    ).copyWith(state: open ? ThreadState.open : ThreadState.closed);
    await db.threadsDao.upsertIfNewer(t);
    await db.searchDao.indexThreadTitle(t);
    final m = message(n, 1, text: 'Note $n from Sam');
    await db.messagesDao.upsertIfNewer(m);
    await db.messagesDao.addLoadedRange(
      threadId(n),
      const ThreadSeq(1),
      const ThreadSeq(1),
    );
    await db.searchDao.indexMessage(m);
  }

  Future<List<Message>> messagesOf(int n) =>
      db.messagesDao.pageBefore(threadId(n), null, 10);

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
    await addThread(1, open: false);
    await addThread(2, open: false);
    await addCachedFile(1, 10);
    clock.advance(const Duration(days: 6));
    await db.threadsDao.touch(threadId(2));
    clock.advance(const Duration(days: 3));

    final paths = await cleaner(maxAge: const Duration(days: 7)).run();

    expect(await db.threadsDao.byId(threadId(1)), isNull);
    expect(await db.threadsDao.byId(threadId(2)), isNotNull);
    expect(await messagesOf(1), isEmpty);
    expect(await messagesOf(2), hasLength(1));
    final hits = await db.searchDao.search('note', limit: 10);
    expect(hits.map((h) => h.threadId), [threadId(2)]);
    expect(
      (await db.searchDao.search('plan', limit: 10)).map((h) => h.threadId),
      [threadId(2)],
      reason: 'the title of a deleted thread is not found any more',
    );
    expect(paths, [
      'cache/file-1',
    ], reason: 'the file was not opened for 9 days');
    expect(await db.structureDao.spaceById(space(1).id), isNotNull);
    expect(await db.structureDao.channelById(channel(1).id), isNotNull);
  });

  test('an open thread nobody opened loses its content and keeps its row with '
      'what the sidebar reads of it', () async {
    await addThread(1);
    await addThread(2);
    await db.messagesDao.putFile(file(1));
    await db.messagesDao.putAttachments(messageId(1, 1), [
      MessageAttachment(
        messageId: messageId(1, 1),
        fileId: file(1).id,
        position: 1,
        threadId: threadId(1),
        channelId: channel(1).id,
      ),
    ]);
    await addCachedFile(1, 10);
    await db.messagesDao.applyReactionSummary(
      ReactionSummary(
        messageId: messageId(1, 1),
        emoji: '+1',
        count: 1,
        me: true,
      ),
    );
    await db.messagesDao.putMention(
      Mention(
        messageId: messageId(1, 1),
        principalId: sam,
        threadId: threadId(1),
      ),
    );
    // What the sidebar reads of the thread: its run, read state, seen mark and mute.
    final run = RunSummary(
      id: RunId.parse(uuid(0x701)),
      threadId: threadId(1),
      channelId: channel(1).id,
      agentId: robin,
      triggeredBy: alex,
      state: RunState.failed,
      version: const ResourceVersion(1),
      billingModeExpected: BillingMode.personal,
      cost: const Money(amountMicros: 0, currency: 'EUR', isEstimate: false),
      stepCount: 1,
      queuedAt: at(1),
    );
    await db.runsDao.upsertIfNewer(run);
    final read = ReadState(
      threadId: threadId(1),
      channelId: channel(1).id,
      lastReadSeq: const ThreadSeq(1),
      mentionCount: 0,
      version: const ResourceVersion(1),
    );
    await db.personalDao.applyReadState(read);
    await db.forumViewDao.markSeen(threadId(1), run.id, run.state);
    await db.personalDao.upsertNotificationPreferenceIfNewer(
      NotificationPreference(
        targetKind: NotificationTargetKind.thread,
        targetId: threadId(1).value,
        level: NotificationLevel.mute,
        version: const ResourceVersion(1),
      ),
    );
    await db.personalDao.upsertDraftIfNewer(draft(1));
    clock.advance(const Duration(days: 6));
    await db.threadsDao.touch(threadId(2));
    await db.fileCacheDao.touch(file(1).id, 'original');
    clock.advance(const Duration(days: 3));

    final paths = await cleaner(maxAge: const Duration(days: 7)).run();

    // The sidebar still lists the open thread as it did (01 6.8).
    final kept = await db.threadsDao.byId(threadId(1));
    expect(kept, isNotNull);
    expect(kept!.title, 'Plan 1');
    expect(kept.lastMessageSeq, const ThreadSeq(1));
    expect(await db.runsDao.byId(run.id), run);
    expect(await db.personalDao.readState(threadId(1)), read);
    expect((await db.forumViewDao.watchSeenStates().first).keys, [threadId(1)]);
    expect(
      (await db.personalDao.watchNotificationPreferences().first).single.level,
      NotificationLevel.mute,
    );
    expect(
      (await db.searchDao.search('plan', limit: 10)).map((h) => h.threadId),
      unorderedEquals([threadId(1), threadId(2)]),
      reason: 'a listed thread is still found by its title',
    );
    expect(await db.personalDao.allDrafts(), hasLength(1));
    // Its content left the device, the cached bytes of its file included.
    expect(await messagesOf(1), isEmpty);
    expect((await db.messagesDao.loadedRanges(threadId(1))).ranges, isEmpty);
    expect(await db.messagesDao.reactionsOf(messageId(1, 1)), isEmpty);
    expect(await db.messagesDao.mentionsIn(threadId(1)), isEmpty);
    expect(await db.messagesDao.attachmentsOf(messageId(1, 1)), isEmpty);
    expect(await db.messagesDao.fileById(file(1).id), isNull);
    expect(
      (await db.searchDao.search('note', limit: 10)).map((h) => h.threadId),
      [threadId(2)],
    );
    expect(paths, ['cache/file-1']);
    // The thread the person opened is untouched.
    expect(await messagesOf(2), hasLength(1));
  });

  test('recording an access wakes no watcher of the threads and still counts '
      'for the cleanup', () async {
    await addThread(1, open: false);
    final seen = <Thread?>[];
    final sub = db.threadsDao.watchThread(threadId(1)).listen(seen.add);
    await pumpEventQueue();
    expect(seen, hasLength(1));

    clock.advance(const Duration(days: 6));
    await db.threadsDao.touch(threadId(1));
    await pumpEventQueue();
    expect(
      seen,
      hasLength(1),
      reason: 'the access time is in no watched value',
    );
    await sub.cancel();

    clock.advance(const Duration(days: 3));
    await cleaner(maxAge: const Duration(days: 7)).run();
    expect(await db.threadsDao.byId(threadId(1)), isNotNull);
    expect(await messagesOf(1), hasLength(1));
    // An access to a thread the device does not hold writes nothing.
    await db.threadsDao.touch(threadId(9));
    expect(await db.threadsDao.byId(threadId(9)), isNull);
  });

  test('the age of an emptied open thread starts again', () async {
    await addThread(1);
    clock.advance(const Duration(days: 8));
    await cleaner(maxAge: const Duration(days: 7)).run();
    expect(await messagesOf(1), isEmpty);

    // A message arrives later; the next start one day on must not take it at once.
    await db.messagesDao.upsertIfNewer(
      message(1, 2, text: 'Late note from Sam'),
    );
    clock.advance(const Duration(days: 1));
    await cleaner(maxAge: const Duration(days: 7)).run();
    expect(await messagesOf(1), hasLength(1));

    // Unopened for another full period, it is emptied again and the row still stays.
    clock.advance(const Duration(days: 7));
    await cleaner(maxAge: const Duration(days: 7)).run();
    expect(await messagesOf(1), isEmpty);
    expect(await db.threadsDao.byId(threadId(1)), isNotNull);
  });

  test('a thread that was done meanwhile goes after its next period', () async {
    await addThread(1);
    clock.advance(const Duration(days: 8));
    await cleaner(maxAge: const Duration(days: 7)).run();
    expect(await db.threadsDao.byId(threadId(1)), isNotNull);

    await db.threadsDao.upsertIfNewer(
      thread(1, version: 2, lastSeq: 1).copyWith(state: ThreadState.closed),
    );
    clock.advance(const Duration(days: 8));
    await cleaner(maxAge: const Duration(days: 7)).run();
    expect(await db.threadsDao.byId(threadId(1)), isNull);
    expect(await db.searchDao.search('plan', limit: 10), isEmpty);
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
