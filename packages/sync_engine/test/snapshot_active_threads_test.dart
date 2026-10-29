// SPDX-License-Identifier: Apache-2.0

// The threads of the sidebar (brief M4-40 Tests and Design note 5; 04 6.3 item 4, R-SY-28,
// R-DM-54): the snapshot's `active_threads` are stored on the first `ready` and on a resync, not
// for a channel this device does not hold; a `message.created` of a thread the database does not
// hold loads it with one `getThread`, also when two messages of it arrive back to back;
// `channel.updated` with `project_bound` updates the stored channel. The second group runs the
// same path with `SyncEngine` against the fake server, which counts the list requests.

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart' as fake;
import 'package:test/test.dart';

import 'catch_up/support.dart';
import 'scenarios/support.dart' as scenario;

void main() {
  ThreadId t(int n) => ThreadId.parse(threadId(n));
  ChannelId c(int n) => ChannelId.parse(channelId(n));

  /// The requests of the catch-up world that list or read threads.
  List<String> threadReads(World w) => [
    for (final r in w.rest.requests)
      if (r.path.endsWith('/threads') || r.path.startsWith('/threads/')) r.path,
  ];

  group('catch-up', () {
    late World w;
    setUp(() => w = World());
    tearDown(() => w.dispose());

    test('the first ready stores the active threads of the held channels '
        'without a thread request', () async {
      await w.connect(
        snapshot: snapshotJson(
          channels: [channelView(1), channelView(2, projectBound: true)],
          activeThreads: [
            threadJson(1, title: 'Login loop'),
            threadJson(2, channelN: 2, title: 'Printer jams'),
            threadJson(3, channelN: 9, title: 'Hidden budget'),
          ],
        ),
      );

      expect((await w.db.threadsDao.byId(t(1)))?.title, 'Login loop');
      expect((await w.db.threadsDao.byId(t(2)))?.title, 'Printer jams');
      expect(
        await w.db.threadsDao.byId(t(3)),
        isNull,
        reason: 'channel 9 is not in the snapshot, so not held',
      );
      expect(await w.db.searchDao.search('printer', limit: 10), isNotEmpty);
      expect(threadReads(w), isEmpty);
      // The snapshot's ChannelView carries the flag (storeChannelView).
      expect((await w.db.structureDao.channelById(c(2)))!.projectBound, isTrue);
      expect(
        (await w.db.structureDao.channelById(c(1)))!.projectBound,
        isFalse,
      );
    });

    test('a resync stores the active threads of the new snapshot', () async {
      await w.connect(cursor: 30);
      await w.push('catchup.complete', {'head_seq': 30});
      w.rest.routes['GET /sync/snapshot'] = (_) => FakeReply(
        200,
        snapshotJson(
          headSeq: 90,
          epoch: 'epoch-2',
          activeThreads: [threadJson(4, channelN: 2, title: 'Deploy window')],
        ),
      );

      await w.push('resync_required', {
        'reason': 'epoch_changed',
        'head_seq': 90,
        'sync_epoch': 'epoch-2',
      });

      expect(w.notices.whereType<Resynced>(), hasLength(1));
      expect((await w.db.threadsDao.byId(t(4)))?.title, 'Deploy window');
      expect(threadReads(w), isEmpty);
    });

    test('a connection that continues from a cursor leaves the threads to '
        'the replayed events', () async {
      await w.connect(
        cursor: 30,
        snapshot: snapshotJson(activeThreads: [threadJson(5)]),
      );
      await w.push('catchup.complete', {'head_seq': 30});

      expect(await w.db.threadsDao.byId(t(5)), isNull);
    });

    test('two messages of a thread the database does not hold load it with '
        'one getThread', () async {
      await w.connect(cursor: 30);
      await w.push('catchup.complete', {'head_seq': 30});
      final release = Completer<void>();
      w.rest.gates['GET /threads/${threadId(6)}'] = release.future;
      w.rest.routes['GET /threads/${threadId(6)}'] = (_) =>
          FakeReply(200, threadJson(6, title: 'Flaky build', lastSeq: 2));

      // Back to back: the second message arrives while the load of the first one waits.
      w.ch
        ..push('dispatch', messageCreated(31, 1, threadN: 6))
        ..push('dispatch', messageCreated(32, 2, threadN: 6));
      while (await w.cursor() != 32 ||
          w.rest.count('/threads/${threadId(6)}') == 0) {
        await settle();
      }
      expect(w.rest.count('/threads/${threadId(6)}'), 1);
      expect(await w.db.threadsDao.byId(t(6)), isNull);

      release.complete();
      await w.idle();
      expect((await w.db.threadsDao.byId(t(6)))?.title, 'Flaky build');
      expect(w.rest.count('/threads/${threadId(6)}'), 1);

      // A thread the database holds is not loaded again.
      await w.dispatch([messageCreated(33, 3, threadN: 6)]);
      expect(w.rest.count('/threads/${threadId(6)}'), 1);
    });

    test('a failed load is logged without content and the next message '
        'tries again', () async {
      await w.connect(cursor: 30);
      await w.push('catchup.complete', {'head_seq': 30});
      w.rest.routes['GET /threads/${threadId(7)}'] = (_) =>
          const FakeReply(503, {'code': 'unavailable'});

      await w.dispatch([messageCreated(31, 1, threadN: 7)]);
      expect(w.rest.count('/threads/${threadId(7)}'), 1);
      expect(await w.db.threadsDao.byId(t(7)), isNull);
      expect(
        w.log,
        contains(startsWith('catch-up: unknown thread not loaded: ')),
      );
      expect(w.log.where((l) => l.contains('deploy note')), isEmpty);

      w.rest.routes['GET /threads/${threadId(7)}'] = (_) =>
          FakeReply(200, threadJson(7, title: 'Retry works', lastSeq: 2));
      await w.dispatch([messageCreated(32, 2, threadN: 7)]);
      expect(w.rest.count('/threads/${threadId(7)}'), 2);
      expect((await w.db.threadsDao.byId(t(7)))?.title, 'Retry works');
    });

    test('a thread of a channel this device does not hold is neither asked '
        'for nor stored', () async {
      await w.connect(cursor: 30);
      await w.push('catchup.complete', {'head_seq': 30});
      w.rest.routes['GET /threads/${threadId(8)}'] = (_) =>
          FakeReply(200, threadJson(8, channelN: 9));

      await w.dispatch([messageCreated(31, 1, threadN: 8, channelN: 9)]);

      expect(await w.db.threadsDao.byId(t(8)), isNull);
      expect(w.rest.count('/threads/${threadId(8)}'), 0);
    });

    test(
      'a channel removed while the load runs keeps its thread out',
      () async {
        await w.connect(cursor: 30);
        await w.push('catchup.complete', {'head_seq': 30});
        final release = Completer<void>();
        w.rest.gates['GET /threads/${threadId(9)}'] = release.future;
        w.rest.routes['GET /threads/${threadId(9)}'] = (_) =>
            FakeReply(200, threadJson(9, lastSeq: 1));

        w.ch
          ..push('dispatch', messageCreated(31, 1, threadN: 9))
          ..push('dispatch', accessRemoved(32, 1));
        while (await w.cursor() != 32 ||
            w.rest.count('/threads/${threadId(9)}') == 0) {
          await settle();
        }
        release.complete();
        await w.idle();

        expect(await w.db.threadsDao.byId(t(9)), isNull);
      },
    );

    test(
      'channel.updated with project_bound updates the stored channel',
      () async {
        await w.connect(cursor: 30);
        await w.push('catchup.complete', {'head_seq': 30});
        expect(
          (await w.db.structureDao.channelById(c(1)))!.projectBound,
          isFalse,
        );

        await w.dispatch([
          envelope(
            'channel.updated',
            seq: 31,
            payload: {
              'channel': channelJson(1, version: 2, projectBound: true),
            },
            resourceKind: 'channel',
            resourceId: channelId(1),
            version: 2,
            scope: {'space_id': spaceId(1), 'channel_id': channelId(1)},
          ),
        ]);

        expect(
          (await w.db.structureDao.channelById(c(1)))!.projectBound,
          isTrue,
        );
      },
    );

    test('an older payload without project_bound or active_threads decodes '
        'with the defaults', () {
      expect(
        channelViewFromJson({
          ...channelView(1),
          'channel': channelJson(1)..remove('project_bound'),
        }).channel.projectBound,
        isFalse,
      );
      expect(
        snapshotFromJson(snapshotJson()..remove('active_threads'))
            .activeThreads,
        isEmpty,
      );
    });
  });

  group('open forum posts after a full snapshot (M4-46)', () {
    late World w;
    setUp(() => w = World());
    tearDown(() => w.dispose());

    /// Forum 3 with [count] open, read posts (threads 100 and up) that no snapshot carries,
    /// listed ten per page and only with `state=open`.
    void forumWithOpenPosts(int count) {
      w.rest.routes['GET /channels/${channelId(3)}/threads'] = (r) {
        expect(r.query['state'], 'open');
        final all = [
          for (var n = 0; n < count; n++)
            threadJson(100 + n, channelN: 3, title: 'Post $n'),
        ];
        final start = int.tryParse(r.query['cursor'] ?? '') ?? 0;
        final end = start + 10 < all.length ? start + 10 : all.length;
        return FakeReply(200, {
          'items': all.sublist(start, end),
          if (end < all.length) 'next_cursor': '$end',
        });
      };
    }

    Map<String, Object?> forumView(int n) => {
      ...channelView(n),
      'channel': {...channelJson(n), 'kind': 'forum'},
    };

    Future<int> heldPosts() async => [
      for (var n = 0; n < 24; n++)
        if (await w.db.threadsDao.byId(t(100 + n)) != null) n,
    ].length;

    test('the first ready loads every open post of a forum', () async {
      forumWithOpenPosts(24);
      await w.connect(
        snapshot: snapshotJson(
          channels: [channelView(1), forumView(3)],
          activeThreads: [threadJson(1)],
        ),
      );

      expect(await heldPosts(), 24);
      expect(
        threadReads(w),
        everyElement('/channels/${channelId(3)}/threads'),
        reason: 'only the forum is listed, a text channel is not',
      );
      expect(threadReads(w), hasLength(3));
    });

    test('a resync loads every open post of a forum again', () async {
      await w.connect(cursor: 30);
      await w.push('catchup.complete', {'head_seq': 30});
      forumWithOpenPosts(24);
      w.rest.routes['GET /sync/snapshot'] = (_) => FakeReply(
        200,
        snapshotJson(
          headSeq: 90,
          epoch: 'epoch-2',
          channels: [channelView(1), forumView(3)],
        ),
      );

      await w.push('resync_required', {
        'reason': 'epoch_changed',
        'head_seq': 90,
        'sync_epoch': 'epoch-2',
      });

      expect(w.notices.whereType<Resynced>(), hasLength(1));
      expect(await heldPosts(), 24);
    });

    test('a connection that continues from a cursor lists no forum', () async {
      forumWithOpenPosts(24);
      await w.connect(
        cursor: 30,
        snapshot: snapshotJson(channels: [channelView(1), forumView(3)]),
      );
      await w.push('catchup.complete', {'head_seq': 30});

      expect(threadReads(w), isEmpty);
    });
  });

  group('engine against the fake server', () {
    test('after connect the threads come from the snapshot and the open '
        'posts of a forum from its list; new ones and project_bound '
        'follow', () async {
      await scenario.withScenario((server, seed, client) async {
        final world = server.world;
        final forum = world.addForum(seed.space, 'dev', {
          seed.alex.id: 'write',
          seed.sam.id: 'write',
        });
        // M4-46: the oldest of 21 open posts is beyond the snapshot's bound of 20 per channel.
        final older = world.addThread(
          forum,
          'Login loop',
          createdBy: seed.sam.id,
        );
        world.threads[older]!['last_activity_at'] = world.stamp(
          DateTime.utc(2020),
        );
        for (var i = 0; i < fake.FakeWorld.snapshotThreadsPerChannel; i++) {
          world.addThread(forum, 'Post $i', createdBy: seed.sam.id);
        }
        expect([
          for (final t
              in world.snapshot(seed.alex.id)['active_threads']! as List)
            (t as Map)['id'],
        ], isNot(contains(older)));
        await client.start();
        Future<Thread?> stored(String id) =>
            client.db.threadsDao.byId(ThreadId.parse(id));

        await fake.waitFor('active threads and open posts', () async {
          return await stored(older) != null &&
              await stored(seed.printer) != null &&
              await stored(seed.releaseNotes) != null;
        });
        expect(await stored(seed.budget), isNull, reason: 'Alex lacks private');
        expect(
          [for (final r in server.requestsOf('listChannelThreads')) r.path],
          [contains(forum)],
          reason: 'one list of the forum, none of a text channel',
        );
        expect(server.requestsOf('getThread'), isEmpty);

        // A post Sam creates arrives with its thread.created.
        final created = world.createThread(seed.sam.id, forum, {
          'title': 'Printer jams',
          'first_message': {
            'client_msg_id': world.nextId(),
            'content_md': 'Since Monday',
          },
        });
        final post =
            (created['thread']! as Map<String, Object?>)['id']! as String;
        await fake.waitFor(
          'created post',
          () async => await stored(post) != null,
        );
        expect(server.requestsOf('getThread'), isEmpty);

        // A thread the engine never heard of comes with its first message.
        final quiet = world.addThread(
          forum,
          'Quiet one',
          createdBy: seed.sam.id,
        );
        seed
          ..post(seed.sam, quiet, 'First')
          ..post(seed.sam, quiet, 'Second');
        await fake.waitFor(
          'loaded thread',
          () async => await stored(quiet) != null,
        );
        expect(server.requestsOf('getThread'), hasLength(1));

        world.setProjectBound(forum, bound: true);
        await fake.waitFor(
          'project_bound',
          () async =>
              (await client.db.structureDao.channelById(ChannelId.parse(forum)))
                  ?.projectBound ??
              false,
        );
        expect(server.requestsOf('listChannelThreads'), hasLength(1));
        expect(server.violations, isEmpty);
      });
    });

    test('setProjectBound raises the version and appends channel.updated', () {
      final world = fake.FakeWorld();
      final seed = fake.WorldSeed.standard(world);
      final forum = world.addForum(seed.space, 'dev', {seed.alex.id: 'write'});
      final before = world.channels[forum]!.json['version']! as int;

      world.setProjectBound(forum, bound: true);

      final json = world.channels[forum]!.json;
      expect(json['project_bound'], isTrue);
      expect(json['version'], before + 1);
      final event = world.log.last;
      expect(event.type, 'channel.updated');
      expect(event.envelope['resource'], {
        'kind': 'channel',
        'id': forum,
        'version': before + 1,
      });
      expect(event.scope, {'space_id': seed.space, 'channel_id': forum});
      expect(
        (event.envelope['payload']! as Map<String, Object?>)['channel'],
        containsPair('project_bound', true),
      );
      expect(world.mayReceive(seed.alex.id, event), isTrue);
      expect(world.mayReceive(seed.sam.id, event), isFalse);
      expect(
        (world.channelView(seed.alex.id, forum)['channel']!
            as Map<String, Object?>)['project_bound'],
        isTrue,
      );
    });
  });
}
