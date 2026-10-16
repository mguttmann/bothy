// SPDX-License-Identifier: Apache-2.0

// The cursor in the page transaction (04 7.1, 15 7.3 item 1, brief M4-14 Tests): an exception
// inside a page leaves the cursor at its previous value; a cursor frame without events advances
// it.

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  test(
    'an exception inside a page leaves rows and cursor as they were',
    () async {
      await w.connect(cursor: 30);
      final bad = threadCreated(32, 2);
      // A valid envelope whose payload fails the mapping of the applier (a thread ID that is
      // not a UUID passes the generated decoder, which does not check formats).
      ((bad['payload']! as Map)['thread']! as Map)['id'] = 'not-a-uuid';
      await w.dispatch([threadCreated(31, 1), bad]);
      await w.push('cursor', {'seq': 33});
      expect(await w.cursor(), 30);
      expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(1))), isNull);
      expect(w.log.any((l) => l.contains('page failed')), isTrue);
    },
  );

  test('a cursor frame without events advances the cursor', () async {
    await w.connect(cursor: 30);
    await w.push('cursor', {'seq': 35});
    expect(await w.cursor(), 35);
    expect((await w.db.syncStateDao.read()).syncEpoch, 'epoch-1');
  });

  test('the first connection stores the snapshot with cursor and epoch in one step', () async {
    await w.connect(headSeq: 50);
    final s = await w.db.syncStateDao.read();
    expect(s.cursor, const GlobalSeq(50));
    expect(s.syncEpoch, 'epoch-1');
    expect(s.lastSnapshotAt, isNotNull);
    final channels = await w.db.structureDao
        .watchChannels(SpaceId.parse(spaceId(1)))
        .first;
    expect(channels, hasLength(2));
    expect(
      await w.db.structureDao
          .watchChannelAccess(ChannelId.parse(channelId(1)))
          .first,
      isNotNull,
    );
    expect((await w.db.instanceDao.get())!.features, {'dm.enabled': true});
  });

  test(
    'the first connection is live right after ready, without catchup.complete',
    () async {
      await w.connect(headSeq: 40);
      expect(w.h.phase, ConnectionPhase.live);
      expect(w.liveCalls, 1);
      await w.dispatch([threadCreated(41, 1), accessRemoved(42, 2)]);
      expect(
        await w.db.threadsDao.byId(ThreadId.parse(threadId(1))),
        isNotNull,
      );
      expect(await w.cursor(), 42);
      expect(
        await w.db.structureDao.channelById(ChannelId.parse(channelId(2))),
        isNull,
      );
      expect(w.notices.whereType<TargetsRemoved>(), hasLength(1));
    },
  );
}
