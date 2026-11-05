// SPDX-License-Identifier: Apache-2.0

// The caller's access to a space or channel first learned through `space.created` or
// `channel.created` (brief FIX-access-on-created): the server sends no level and no capabilities
// in these events and no `access.changed` to an owner, so the engine loads the view through REST
// after the commit; a row stored earlier without access is loaded on the next connection from
// the cursor, so a device recovers without a resync. The second group runs the path with
// `SyncEngine` against the fake server, whose `createChannel` and `createSpace` emit the events
// as the server does.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart' as fake;
import 'package:test/test.dart';

import '../scenarios/support.dart' as scenario;
import 'support.dart';

/// `channel.created` of channel [n] in space [spaceN], without level or capabilities.
Map<String, Object?> channelCreated(int seq, int n, {int spaceN = 1}) =>
    envelope(
      'channel.created',
      seq: seq,
      payload: {'channel': channelJson(n, spaceN: spaceN)},
      resourceKind: 'channel',
      resourceId: channelId(n),
      version: 1,
      scope: {'space_id': spaceId(spaceN), 'channel_id': channelId(n)},
    );

/// `space.created` of space [n].
Map<String, Object?> spaceCreated(int seq, int n) => envelope(
  'space.created',
  seq: seq,
  payload: {'space': spaceJson(n)},
  resourceKind: 'space',
  resourceId: spaceId(n),
  version: 1,
  scope: {'space_id': spaceId(n)},
);

/// The `ChannelView` of channel [n] for an owner: `manage` among the capabilities.
Map<String, Object?> ownerChannelView(int n, {int spaceN = 1}) => {
  ...channelView(n, spaceN: spaceN, level: 'trigger'),
  'capabilities': ['export', 'manage'],
};

void main() {
  ChannelId c(int n) => ChannelId.parse(channelId(n));
  SpaceId s(int n) => SpaceId.parse(spaceId(n));

  group('catch-up', () {
    late World w;
    setUp(() => w = World());
    tearDown(() => w.dispose());

    void answerChannel(int n, {int spaceN = 1}) {
      w.rest.routes['GET /channels/${channelId(n)}'] = (_) =>
          FakeReply(200, ownerChannelView(n, spaceN: spaceN));
      w.rest.routes['GET /channels/${channelId(n)}/threads'] = (_) =>
          const FakeReply(200, {'items': <Object?>[]});
    }

    test('a channel created by event gets the access of its view', () async {
      await w.connect(cursor: 30);
      await w.push('catchup.complete', {'head_seq': 30});
      answerChannel(5);

      await w.dispatch([channelCreated(31, 5)]);

      expect(w.rest.count('/channels/${channelId(5)}'), 1);
      final access = await w.db.structureDao.channelAccess(c(5));
      expect(access?.level, AccessLevel.trigger);
      expect(access?.capabilities, contains(Capability.manage));
    });

    test('a space created by event gets the access of its view and its '
        'channels', () async {
      await w.connect(cursor: 30);
      await w.push('catchup.complete', {'head_seq': 30});
      w.rest.routes['GET /spaces/${spaceId(4)}'] = (_) =>
          FakeReply(200, spaceView(4, level: 'trigger'));
      w.rest.routes['GET /spaces/${spaceId(4)}/channels'] = (_) =>
          FakeReply(200, {
            'items': [ownerChannelView(6, spaceN: 4)],
          });
      answerChannel(6, spaceN: 4);

      await w.dispatch([spaceCreated(31, 4), channelCreated(32, 6, spaceN: 4)]);

      expect(
        (await w.db.structureDao.spaceAccess(s(4)))?.level,
        AccessLevel.trigger,
      );
      expect(
        (await w.db.structureDao.channelAccess(c(6)))?.capabilities,
        contains(Capability.manage),
      );
    });

    test('a channel whose access is stored is not loaded again', () async {
      await w.connect(cursor: 30);
      await w.push('catchup.complete', {'head_seq': 30});

      await w.dispatch([
        envelope(
          'channel.updated',
          seq: 31,
          payload: {'channel': channelJson(1, version: 2)},
          resourceKind: 'channel',
          resourceId: channelId(1),
          version: 2,
          scope: {'space_id': spaceId(1), 'channel_id': channelId(1)},
        ),
      ]);

      expect(w.rest.count('/channels/${channelId(1)}'), 0);
      expect((await w.db.structureDao.channelById(c(1)))?.version.value, 2);
    });

    test('two events of one new channel in a page load it once', () async {
      await w.connect(cursor: 30);
      await w.push('catchup.complete', {'head_seq': 30});
      answerChannel(5);

      await w.dispatch([
        channelCreated(31, 5),
        envelope(
          'channel.updated',
          seq: 32,
          payload: {'channel': channelJson(5, version: 2)},
          resourceKind: 'channel',
          resourceId: channelId(5),
          version: 2,
          scope: {'space_id': spaceId(1), 'channel_id': channelId(5)},
        ),
      ]);

      expect(w.rest.count('/channels/${channelId(5)}'), 1);
    });

    test('rows stored without access load after a reconnect from the '
        'cursor', () async {
      // The state of a device before the fix: rows of an earlier channel.created and
      // space.created without level.
      await w.db.structureDao.upsertSpaceIfNewer(
        spaceViewFromJson(spaceView(4)).space,
      );
      await w.db.structureDao.upsertChannelIfNewer(
        channelViewFromJson(channelView(5)).channel,
      );
      expect(await w.db.structureDao.channelAccess(c(5)), isNull);
      answerChannel(5);
      w.rest.routes['GET /spaces/${spaceId(4)}'] = (_) =>
          FakeReply(200, spaceView(4, level: 'trigger'));
      w.rest.routes['GET /spaces/${spaceId(4)}/channels'] = (_) =>
          const FakeReply(200, {'items': <Object?>[]});

      await w.connect(cursor: 30);
      await w.push('catchup.complete', {'head_seq': 30});

      expect(w.rest.count('/channels/${channelId(5)}'), 1);
      expect(
        (await w.db.structureDao.channelAccess(c(5)))?.capabilities,
        contains(Capability.manage),
      );
      expect(
        (await w.db.structureDao.spaceAccess(s(4)))?.level,
        AccessLevel.trigger,
      );
      expect(
        w.rest.count('/channels/${channelId(1)}'),
        0,
        reason: 'a channel with stored access is not loaded',
      );
      expect(w.notices.whereType<Resynced>(), isEmpty);
    });

    test('a failed load keeps the row, logs without content and the next '
        'connection tries again', () async {
      await w.db.structureDao.upsertChannelIfNewer(
        channelViewFromJson(channelView(5)).channel,
      );
      w.rest.routes['GET /channels/${channelId(5)}'] = (_) =>
          const FakeReply(503, {'code': 'unavailable'});

      await w.connect(cursor: 30);
      await w.push('catchup.complete', {'head_seq': 30});

      expect(w.rest.count('/channels/${channelId(5)}'), 1);
      expect(await w.db.structureDao.channelById(c(5)), isNotNull);
      expect(await w.db.structureDao.channelAccess(c(5)), isNull);
      expect(w.log, contains(startsWith('catch-up: background read failed: ')));
      expect(w.log.where((l) => l.contains('channel-5')), isEmpty);

      answerChannel(5);
      // The next connection: the server no longer resumes the session and answers `ready`.
      w.ch.serverClose(4000);
      await w.idle();
      await w.h.clock.elapse(const Duration(minutes: 2));
      await w.idle();
      w.ch.push('hello', helloData(nonce: 'nonce-9'));
      await w.idle();
      w.ch.push(
        'ready',
        readyData(headSeq: 30)..['snapshot'] = snapshotJson(headSeq: 30),
      );
      await w.idle();
      await w.push('catchup.complete', {'head_seq': 30});

      expect(w.rest.count('/channels/${channelId(5)}'), 2);
      expect(
        (await w.db.structureDao.channelAccess(c(5)))?.capabilities,
        contains(Capability.manage),
      );
    });
  });

  group('engine against the fake server', () {
    test('an owner gets manage for a forum and a space created while '
        'connected', () async {
      await scenario.withScenario((server, seed, client) async {
        final world = server.world;
        await client.start();
        await fake.waitFor(
          'the first snapshot',
          () async =>
              await client.db.structureDao.channelById(
                ChannelId.parse(seed.general),
              ) !=
              null,
        );

        final forum = world.createChannel(
          seed.alex.id,
          seed.space,
          'website',
          {seed.alex.id: 'trigger'},
          kind: 'forum',
          projectBound: true,
          capabilities: {
            seed.alex.id: {'manage', 'export'},
          },
        );
        final created = world.log.last.envelope['payload']! as Map;
        expect(
          (created['channel']! as Map).keys,
          isNot(anyOf(contains('level'), contains('capabilities'))),
          reason: 'the event carries the channel only, as on the server',
        );
        await fake.waitFor(
          'manage on the new forum',
          () async =>
              (await client.db.structureDao.channelAccess(
                ChannelId.parse(forum),
              ))?.capabilities.contains(Capability.manage) ??
              false,
        );
        final row = await client.db.structureDao.channelById(
          ChannelId.parse(forum),
        );
        expect(row?.kind, ChannelKind.forum);
        expect(row?.projectBound, isTrue);

        final made = world.createSpace(
          seed.alex.id,
          'Projects',
          access: {seed.alex.id: 'trigger'},
          capabilities: {
            seed.alex.id: {'manage'},
          },
        );
        await fake.waitFor(
          'access on the new space and its channel',
          () async =>
              await client.db.structureDao.spaceAccess(
                    SpaceId.parse(made.space),
                  ) !=
                  null &&
              ((await client.db.structureDao.channelAccess(
                    ChannelId.parse(made.channel),
                  ))?.capabilities.contains(Capability.manage) ??
                  false),
        );
        expect(server.violations, isEmpty);
      });
    });
  });
}
