// SPDX-License-Identifier: Apache-2.0

// The cursor commits with its page (15 7.3 item 1, 04 7.5), the session fields have their own
// transactions, and the resync empties the mirror but keeps unsent work (04 7.3).

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  test(
    'a page whose apply throws changes neither rows nor the cursor',
    () async {
      final sync = db.syncStateDao;
      await sync.commitPage(
        (txn) => txn.db.messagesDao.upsertIfNewer(message(1, 1)),
        cursor: const GlobalSeq(10),
        syncEpoch: 'epoch-1',
      );
      await expectLater(
        sync.commitPage((txn) async {
          await txn.db.messagesDao.upsertIfNewer(message(1, 2));
          throw StateError('page broken');
        }, cursor: const GlobalSeq(20)),
        throwsStateError,
      );
      expect(await db.messagesDao.byId(messageId(1, 2)), isNull);
      final state = await sync.read();
      expect(state.cursor, const GlobalSeq(10));
      expect(state.syncEpoch, 'epoch-1');
    },
  );

  test('a successful page stores its rows, the cursor and the epoch', () async {
    final sync = db.syncStateDao;
    expect(await sync.read(), SyncState.empty);
    await sync.commitPage(
      (txn) => txn.db.messagesDao.upsertIfNewer(message(1, 1)),
      cursor: const GlobalSeq(7),
      syncEpoch: 'epoch-2',
    );
    await sync.commitPage(
      (txn) => txn.db.messagesDao.upsertIfNewer(message(1, 2)),
      cursor: const GlobalSeq(8),
    );
    expect(await db.messagesDao.byId(messageId(1, 2)), isNotNull);
    final state = await sync.watch().first;
    expect(state.cursor, const GlobalSeq(8));
    expect(state.syncEpoch, 'epoch-2');
  });

  test('session fields and the disconnect time leave the cursor and the epoch alone', () async {
    final sync = db.syncStateDao;
    await sync.commitPage(
      (_) async {},
      cursor: const GlobalSeq(3),
      syncEpoch: 'e',
    );
    await sync.saveSession(
      gatewaySessionId: 'session-1',
      permissionVersion: const PermissionVersion(4),
      headSeq: const GlobalSeq(9),
    );
    await sync.markDisconnected(at(5));
    await sync.markSnapshot(at(6));
    expect(
      await sync.read(),
      SyncState(
        cursor: const GlobalSeq(3),
        syncEpoch: 'e',
        headSeq: const GlobalSeq(9),
        permissionVersion: const PermissionVersion(4),
        gatewaySessionId: 'session-1',
        disconnectedAt: at(5),
        lastSnapshotAt: at(6),
      ),
    );
  });

  test(
    'clearMirror empties the mirror and loaded_ranges and keeps unsent work',
    () async {
      await db.structureDao.upsertSpaceIfNewer(space(1));
      await db.structureDao.upsertChannelIfNewer(channel(1));
      await db.threadsDao.upsertIfNewer(thread(1, lastSeq: 1));
      await db.messagesDao.upsertIfNewer(message(1, 1));
      await db.messagesDao.addLoadedRange(
        threadId(1),
        const ThreadSeq(1),
        const ThreadSeq(1),
      );
      await db.searchDao.indexMessage(message(1, 1));
      await db.personalDao.upsertDraftIfNewer(draft(1));
      await db.outboxDao.enqueue(
        id: uuid(1),
        kind: 'postMessage',
        payload: '{}',
      );
      await db.shareInboxDao.add(
        ShareInboxEntry(
          id: uuid(2),
          payload: '{}',
          filePaths: const [],
          createdAt: at(0),
        ),
      );
      await db.syncStateDao.commitPage(
        (txn) =>
            db.syncStateDao.clearMirror(txn, visibleChannels: {channel(1).id}),
        cursor: const GlobalSeq(42),
      );
      for (final table in [
        ...db.mirrorTables,
        db.loadedRangesTable,
        db.searchIndex,
        db.searchRows,
      ]) {
        final count = await db
            .customSelect('SELECT count(*) AS n FROM ${table.actualTableName}')
            .getSingle();
        expect(count.read<int>('n'), 0, reason: table.actualTableName);
      }
      expect(await db.personalDao.allDrafts(), [draft(1)]);
      expect(await db.outboxDao.byId(uuid(1)), isNotNull);
      expect(await db.shareInboxDao.all(), hasLength(1));
      expect((await db.syncStateDao.read()).cursor, const GlobalSeq(42));
    },
  );

  group('clearMirror and the file cache (04 8.3 item 3)', () {
    /// Caches the original of file [n] at `cache/file-<n>`.
    Future<void> cache(int n) => db.fileCacheDao.put(
      FileCacheEntry(
        fileId: file(n).id,
        variant: 'original',
        path: 'cache/file-$n',
        sizeBytes: 100,
        lastAccessedAt: at(0),
      ),
    );

    /// Attaches file [n] to message 1 of thread [threadN] in [channelN].
    Future<void> attach(int n, {required int threadN, required int channelN}) =>
        db.messagesDao.putAttachments(messageId(threadN, 1), [
          MessageAttachment(
            messageId: messageId(threadN, 1),
            fileId: file(n).id,
            position: 1,
            threadId: threadId(threadN),
            channelId: channel(channelN).id,
          ),
        ]);

    Future<List<String>> clear(Set<ChannelId> visible) async {
      late final List<String> paths;
      await db.syncStateDao.commitPage(
        (txn) async => paths = await db.syncStateDao.clearMirror(
          txn,
          visibleChannels: visible,
        ),
        cursor: const GlobalSeq(42),
      );
      return paths;
    }

    Future<List<String>> cachedPaths() async => [
      for (final r
          in await db
              .customSelect('SELECT path FROM file_cache ORDER BY path')
              .get())
        r.read<String>('path'),
    ];

    test('the cached files of a channel the snapshot no longer lists go, the '
        'others stay', () async {
      await db.structureDao.upsertChannelIfNewer(channel(1));
      await db.structureDao.upsertChannelIfNewer(channel(2));
      // File 1 belongs to channel 1 and file 2 to channel 2 by their scope; file 3 has no
      // stored metadata, only the attachment of a message of channel 2; file 4 is an avatar
      // without a channel.
      await db.messagesDao.putFile(file(1));
      await db.messagesDao.putFile(file(2, channelN: 2));
      await attach(3, threadN: 2, channelN: 2);
      for (final n in [1, 2, 3, 4]) {
        await cache(n);
      }
      await db.personalDao.upsertDraftIfNewer(
        draft(1, threadN: 2, channelN: 2),
      );

      final paths = await clear({channel(1).id});

      expect(paths, unorderedEquals(['cache/file-2', 'cache/file-3']));
      expect(await cachedPaths(), ['cache/file-1', 'cache/file-4']);
      expect(
        await db.personalDao.allDrafts(),
        hasLength(1),
        reason: 'drafts stay on a resync (04 7.3)',
      );
    });

    test(
      'a snapshot without channels leaves only the files of no channel',
      () async {
        await db.messagesDao.putFile(file(1));
        await cache(1);
        await cache(4);

        expect(await clear(const {}), ['cache/file-1']);
        expect(await cachedPaths(), ['cache/file-4']);
      },
    );

    test('nothing goes while every stored channel is still listed', () async {
      await db.messagesDao.putFile(file(1));
      await attach(2, threadN: 1, channelN: 1);
      await cache(1);
      await cache(2);

      expect(await clear({channel(1).id, channel(2).id}), isEmpty);
      expect(await cachedPaths(), ['cache/file-1', 'cache/file-2']);
    });
  });
}
