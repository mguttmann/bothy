// SPDX-License-Identifier: Apache-2.0

// Deletion and undo in the live stream (01 6.18 item 8, D-DEL-5, D-DEL-8): a `space.archived`
// or `channel.archived` whose object carries `deleted_at` and a `thread.updated` with it remove
// the item and its content as a removed target with the person's order rows kept and a
// TargetsRemoved notice; a later version without `deleted_at` brings it back, a space or channel
// with its structure through REST; an older version is stale; an archive without `deleted_at`
// stays as it was; and no snapshot or REST read stores a deleted item back.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

/// `deleted_at` of the tests.
const String deletedAt = '2026-10-15T10:05:00.000Z';

/// [object] with `deleted_at` (and the archive the deletion sets for a space or channel).
Map<String, Object?> deleted(
  Map<String, Object?> object, {
  bool archive = true,
}) => {
  ...object,
  if (archive) 'archived_at': deletedAt,
  'deleted_at': deletedAt,
};

/// `space.<verb>` of space [n] with [space].
Map<String, Object?> spaceEvent(
  String verb,
  int seq,
  int n,
  Map<String, Object?> space,
) => envelope(
  'space.$verb',
  seq: seq,
  payload: {'space': space},
  resourceKind: 'space',
  resourceId: spaceId(n),
  version: space['version']! as int,
  scope: {'space_id': spaceId(n)},
);

/// `channel.<verb>` of channel [n] with [channel].
Map<String, Object?> channelEvent(
  String verb,
  int seq,
  int n,
  Map<String, Object?> channel,
) => envelope(
  'channel.$verb',
  seq: seq,
  payload: {'channel': channel},
  resourceKind: 'channel',
  resourceId: channelId(n),
  version: channel['version']! as int,
  scope: {'space_id': spaceId(1), 'channel_id': channelId(n)},
);

/// `thread.updated` of thread [n] with [thread].
Map<String, Object?> threadUpdated(
  int seq,
  int n,
  Map<String, Object?> thread,
) => envelope(
  'thread.updated',
  seq: seq,
  payload: {'thread': thread},
  resourceKind: 'thread',
  resourceId: threadId(n),
  version: thread['version']! as int,
  scope: {'channel_id': thread['channel_id'], 'thread_id': threadId(n)},
);

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  final s1 = SpaceId.parse(spaceId(1));
  final ch1 = ChannelId.parse(channelId(1));
  final ch2 = ChannelId.parse(channelId(2));
  final t1 = ThreadId.parse(threadId(1));
  final t2 = ThreadId.parse(threadId(2));

  /// Threads 1 (channel 1) and 2 (channel 2) with a message each, unread for Alex, and the
  /// person's order rows of space 1 and channel 1.
  Future<void> withContent() async {
    await w.connect(cursor: 30, headSeq: 30);
    await w.dispatch([
      threadCreated(31, 1),
      messageCreated(32, 1, content: 'deploy window agreed'),
      threadCreated(33, 2, channelN: 2),
      messageCreated(34, 1, threadN: 2, channelN: 2, content: 'deploy later'),
    ]);
    await w.push('cursor', {'seq': 34});
    await w.db.structureDao.putUserSpaceStateLocally(
      s1,
      collapsed: false,
      sortKey: 'a1',
      channelOrder: [ch2, ch1],
    );
    await w.db.structureDao.putUserChannelStateLocally(ch1, [t1]);
    expect(await w.db.searchDao.search('deploy', limit: 10), hasLength(2));
  }

  Future<List<String>> spaceOrder() async => [
    for (final s in await w.db.structureDao.watchUserSpaceStates().first)
      s.spaceId.value,
  ];

  Future<List<String>> channelOrder() async => [
    for (final s in await w.db.structureDao.watchUserChannelStates().first)
      s.channelId.value,
  ];

  group('space', () {
    test('space.archived with deleted_at removes the space and its content, '
        'keeps the order rows and notifies the removal', () async {
      await withContent();
      await w.dispatch([
        spaceEvent('archived', 35, 1, deleted(spaceJson(1, version: 2))),
      ]);
      await w.push('cursor', {'seq': 35});

      expect(await w.cursor(), 35);
      expect(await w.db.structureDao.spaceById(s1), isNull);
      expect(await w.db.structureDao.channelById(ch1), isNull);
      expect(await w.db.structureDao.channelById(ch2), isNull);
      expect(await w.db.threadsDao.byId(t1), isNull);
      expect(await w.db.threadsDao.byId(t2), isNull);
      expect(await w.storedSeqs(1), isEmpty);
      expect(await w.db.searchDao.search('deploy', limit: 10), isEmpty);
      expect(await spaceOrder(), [spaceId(1)]);
      expect(await channelOrder(), [channelId(1)]);
      expect(w.catchUp.countsOfChannel(ch1), isEmpty);
      final removed = w.notices.whereType<TargetsRemoved>().single;
      expect(removed.targets, [TargetRef.space(s1)]);
      expect(removed.deleted, isTrue);
    });

    test('a later space.updated without deleted_at restores the space with '
        'its channels and threads through REST', () async {
      await withContent();
      await w.dispatch([
        spaceEvent('archived', 35, 1, deleted(spaceJson(1, version: 2))),
      ]);
      w.rest.routes['GET /spaces/${spaceId(1)}'] = (_) => FakeReply(200, {
        'space': spaceJson(1, version: 3),
        'level': 'write',
        'capabilities': <Object?>[],
      });
      w.rest.routes['GET /spaces/${spaceId(1)}/channels'] = (_) =>
          FakeReply(200, {
            'items': [
              channelView(1),
              {
                'channel': deleted(channelJson(2, version: 2)),
                'level': 'write',
                'capabilities': <Object?>[],
              },
            ],
          });
      w.rest.routes['GET /channels/${channelId(1)}/threads'] = (_) =>
          FakeReply(200, {
            'items': [
              threadJson(1, lastSeq: 2),
              deleted(threadJson(3), archive: false),
            ],
          });
      await w.dispatch([
        spaceEvent('updated', 36, 1, spaceJson(1, version: 3)),
      ]);
      await w.push('cursor', {'seq': 36});

      final space = await w.db.structureDao.spaceById(s1);
      expect(space!.version.value, 3);
      expect(
        (await w.db.structureDao.watchSpaceAccess(s1).first)!.level,
        AccessLevel.write,
      );
      expect(await w.db.structureDao.channelById(ch1), isNotNull);
      expect(
        await w.db.structureDao.channelById(ch2),
        isNull,
        reason: 'a channel deleted on its own stays deleted',
      );
      expect(await w.db.threadsDao.byId(t1), isNotNull);
      expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(3))), isNull);
      expect(w.rest.count('/channels/${channelId(2)}/threads'), 0);
      expect(await spaceOrder(), [spaceId(1)]);
      expect(await channelOrder(), [channelId(1)]);
      expect(w.rest.requests.where((r) => r.path == '/events'), isEmpty);
    });

    test(
      'a space.updated not newer than the deletion changes nothing',
      () async {
        await withContent();
        await w.dispatch([
          spaceEvent('archived', 35, 1, deleted(spaceJson(1, version: 3))),
          spaceEvent('updated', 36, 1, spaceJson(1, version: 2)),
          spaceEvent('updated', 37, 1, spaceJson(1, version: 3)),
        ]);
        await w.push('cursor', {'seq': 37});
        expect(await w.db.structureDao.spaceById(s1), isNull);
        expect(w.rest.count('/spaces/${spaceId(1)}'), 0);
      },
    );

    test('a deletion older than the stored space changes nothing', () async {
      await withContent();
      await w.dispatch([
        spaceEvent('updated', 35, 1, spaceJson(1, version: 4)),
        spaceEvent('archived', 36, 1, deleted(spaceJson(1, version: 3))),
      ]);
      await w.push('cursor', {'seq': 36});
      expect((await w.db.structureDao.spaceById(s1))!.version.value, 4);
      expect(await w.db.threadsDao.byId(t1), isNotNull);
      expect(w.notices.whereType<TargetsRemoved>(), isEmpty);
    });

    test('space.archived without deleted_at archives as before', () async {
      await withContent();
      await w.dispatch([
        spaceEvent('archived', 35, 1, {
          ...spaceJson(1, version: 2),
          'archived_at': deletedAt,
        }),
      ]);
      await w.push('cursor', {'seq': 35});
      final space = await w.db.structureDao.spaceById(s1);
      expect(space!.archivedAt, DateTime.parse(deletedAt));
      expect(await w.db.structureDao.channelById(ch1), isNotNull);
      expect(await w.db.threadsDao.byId(t1), isNotNull);
      expect(await w.storedSeqs(1), [1]);
      expect(w.notices.whereType<TargetsRemoved>(), isEmpty);
    });
  });

  group('channel', () {
    test('channel.archived with deleted_at removes the channel, keeps its '
        'thread order, and a later channel.updated restores it', () async {
      await withContent();
      await w.dispatch([
        channelEvent('archived', 35, 1, deleted(channelJson(1, version: 2))),
      ]);
      await w.push('cursor', {'seq': 35});
      expect(await w.db.structureDao.channelById(ch1), isNull);
      expect(await w.db.threadsDao.byId(t1), isNull);
      expect(await w.storedSeqs(1), isEmpty);
      expect(await w.db.structureDao.channelById(ch2), isNotNull);
      expect(await w.db.threadsDao.byId(t2), isNotNull);
      expect(await channelOrder(), [channelId(1)]);
      expect(await spaceOrder(), [spaceId(1)]);
      final removed = w.notices.whereType<TargetsRemoved>().single;
      expect(removed.targets, [TargetRef.channel(ch1)]);
      expect(removed.deleted, isTrue);

      w.rest.routes['GET /channels/${channelId(1)}'] = (_) =>
          FakeReply(200, channelView(1, version: 3, level: 'read'));
      w.rest.routes['GET /channels/${channelId(1)}/threads'] = (_) =>
          FakeReply(200, {
            'items': [threadJson(1, lastSeq: 2)],
          });
      await w.dispatch([
        channelEvent('updated', 36, 1, channelJson(1, version: 3)),
      ]);
      await w.push('cursor', {'seq': 36});
      expect((await w.db.structureDao.channelById(ch1))!.version.value, 3);
      expect(
        (await w.db.structureDao.watchChannelAccess(ch1).first)!.level,
        AccessLevel.read,
      );
      expect(await w.db.threadsDao.byId(t1), isNotNull);
      expect(
        await w.storedSeqs(1),
        isEmpty,
        reason: 'the history loads on open',
      );
      expect(await channelOrder(), [channelId(1)]);
    });

    test('channel.archived without deleted_at archives as before', () async {
      await withContent();
      await w.dispatch([
        channelEvent('archived', 35, 1, {
          ...channelJson(1, version: 2),
          'archived_at': deletedAt,
        }),
      ]);
      await w.push('cursor', {'seq': 35});
      expect(
        (await w.db.structureDao.channelById(ch1))!.archivedAt,
        DateTime.parse(deletedAt),
      );
      expect(await w.storedSeqs(1), [1]);
      expect(w.notices.whereType<TargetsRemoved>(), isEmpty);
    });

    test(
      'an access change that reads a deleted channel does not store it',
      () async {
        await withContent();
        await w.dispatch([
          channelEvent('archived', 35, 1, deleted(channelJson(1, version: 2))),
        ]);
        w.rest.routes['GET /channels/${channelId(7)}'] = (_) => FakeReply(200, {
          'channel': deleted(channelJson(7, version: 2)),
          'level': 'write',
          'capabilities': <Object?>[],
        });
        await w.dispatch([
          envelope(
            'access.changed',
            seq: 36,
            payload: {
              'permission_version': 5,
              'added': [
                {'kind': 'channel', 'id': channelId(7), 'level': 'write'},
              ],
              'removed': <Object?>[],
              'changed': <Object?>[],
            },
            resourceKind: 'principal',
            resourceId: alexId,
            personal: true,
          ),
        ]);
        await w.push('cursor', {'seq': 36});
        expect(
          await w.db.structureDao.channelById(ChannelId.parse(channelId(7))),
          isNull,
        );
        expect(w.rest.count('/channels/${channelId(7)}/threads'), 0);
      },
    );
  });

  group('thread', () {
    test('thread.updated with deleted_at removes the thread and its messages; '
        'a later version restores it, an older one is stale', () async {
      await withContent();
      expect(w.catchUp.countsOfChannel(ch1)[t1]!.unread, 1);
      await w.dispatch([
        threadUpdated(
          35,
          1,
          deleted(threadJson(1, version: 2, lastSeq: 2), archive: false),
        ),
      ]);
      await w.push('cursor', {'seq': 35});
      expect(await w.db.threadsDao.byId(t1), isNull);
      expect(await w.storedSeqs(1), isEmpty);
      expect(await w.db.searchDao.search('agreed', limit: 10), isEmpty);
      expect(await w.db.structureDao.channelById(ch1), isNotNull);
      expect(await channelOrder(), [channelId(1)]);
      expect(w.catchUp.countsOfChannel(ch1), isEmpty);
      final removed = w.notices.whereType<TargetsRemoved>().single;
      expect(removed.targets, [TargetRef.thread(t1)]);
      expect(removed.deleted, isTrue);

      await w.dispatch([
        threadUpdated(36, 1, threadJson(1, version: 2, lastSeq: 2)),
      ]);
      await w.push('cursor', {'seq': 36});
      expect(await w.db.threadsDao.byId(t1), isNull, reason: 'stale');

      await w.dispatch([
        threadUpdated(37, 1, threadJson(1, version: 3, lastSeq: 2)),
      ]);
      await w.push('cursor', {'seq': 37});
      expect((await w.db.threadsDao.byId(t1))!.version.value, 3);
      expect(
        await w.storedSeqs(1),
        isEmpty,
        reason: 'the history loads on open',
      );
    });

    test('a thread page never stores a deleted thread', () async {
      await withContent();
      w.rest.routes['GET /channels/${channelId(1)}/threads'] = (_) =>
          FakeReply(200, {
            'items': [threadJson(4), deleted(threadJson(5), archive: false)],
          });
      await w.catchUp.loadThreadPage(ch1);
      expect(
        await w.db.threadsDao.byId(ThreadId.parse(threadId(4))),
        isNotNull,
      );
      expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(5))), isNull);
    });
  });

  test('a snapshot never stores a deleted space, channel or thread', () async {
    await w.connect(
      snapshot: snapshotJson(
        spaces: [
          spaceView(1),
          {
            'space': deleted(spaceJson(2)),
            'level': 'write',
            'capabilities': <Object?>[],
          },
        ],
        channels: [
          channelView(1),
          {
            'channel': deleted(channelJson(2)),
            'level': 'write',
            'capabilities': <Object?>[],
          },
          channelView(3, spaceN: 2),
        ],
        activeThreads: [threadJson(1), deleted(threadJson(2), archive: false)],
      ),
    );
    expect(await w.db.structureDao.spaceById(s1), isNotNull);
    expect(
      await w.db.structureDao.spaceById(SpaceId.parse(spaceId(2))),
      isNull,
    );
    expect(await w.db.structureDao.channelById(ch1), isNotNull);
    expect(await w.db.structureDao.channelById(ch2), isNull);
    expect(
      await w.db.structureDao.channelById(ChannelId.parse(channelId(3))),
      isNull,
      reason: 'a channel of a deleted space',
    );
    expect(await w.db.threadsDao.byId(t1), isNotNull);
    expect(await w.db.threadsDao.byId(t2), isNull);
  });
}
