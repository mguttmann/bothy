// SPDX-License-Identifier: Apache-2.0

// The recently visited targets of the quick switcher (brief M4-23 Design note 3): kept per
// device in the local-only table `recent_targets`, newest first, trimmed to the newest `keep`
// rows on every visit, and deleted with removed targets (04 8.3).

import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  Future<List<(RecentTargetKind, String)>> recent({int limit = 10}) async {
    final rows = await db.recentTargetsDao.watchRecent(limit: limit).first;
    return [for (final r in rows) (r.kind, r.targetId)];
  }

  test(
    'recent targets come newest first with their visit time in UTC',
    () async {
      final dao = db.recentTargetsDao;
      await dao.visit(RecentTargetKind.space, uuid(0x101), at: at(1), keep: 10);
      await dao.visit(
        RecentTargetKind.channel,
        uuid(0x201),
        at: at(3),
        keep: 10,
      );
      await dao.visit(
        RecentTargetKind.thread,
        uuid(0x301),
        at: at(2),
        keep: 10,
      );

      final rows = await dao.watchRecent(limit: 10).first;
      expect(
        [for (final r in rows) r.targetId],
        [uuid(0x201), uuid(0x301), uuid(0x101)],
      );
      expect(rows.first.kind, RecentTargetKind.channel);
      expect(rows.first.visitedAt, at(3));
      expect(rows.first.visitedAt.isUtc, isTrue);
    },
  );

  test('watchRecent returns at most limit rows', () async {
    for (var i = 0; i < 4; i++) {
      await db.recentTargetsDao.visit(
        RecentTargetKind.thread,
        uuid(0x310 + i),
        at: at(i),
        keep: 10,
      );
    }
    expect(await recent(limit: 2), [
      (RecentTargetKind.thread, uuid(0x313)),
      (RecentTargetKind.thread, uuid(0x312)),
    ]);
  });

  test('a visit keeps only the newest keep rows', () async {
    for (var i = 0; i < 5; i++) {
      await db.recentTargetsDao.visit(
        RecentTargetKind.channel,
        uuid(0x220 + i),
        at: at(i),
        keep: 3,
      );
    }
    final stored = await db
        .customSelect('SELECT count(*) AS n FROM recent_targets')
        .getSingle();
    expect(stored.read<int>('n'), 3);
    expect(await recent(), [
      (RecentTargetKind.channel, uuid(0x224)),
      (RecentTargetKind.channel, uuid(0x223)),
      (RecentTargetKind.channel, uuid(0x222)),
    ]);
  });

  test(
    'a visit of a known target moves it to the top without a second row',
    () async {
      final dao = db.recentTargetsDao;
      await dao.visit(RecentTargetKind.thread, uuid(0x331), at: at(1), keep: 2);
      await dao.visit(RecentTargetKind.thread, uuid(0x332), at: at(2), keep: 2);
      await dao.visit(RecentTargetKind.thread, uuid(0x331), at: at(3), keep: 2);
      await dao.visit(RecentTargetKind.thread, uuid(0x333), at: at(4), keep: 2);

      expect(await recent(), [
        (RecentTargetKind.thread, uuid(0x333)),
        (RecentTargetKind.thread, uuid(0x331)),
      ]);
    },
  );

  test('the same ID of two kinds are two targets', () async {
    final dao = db.recentTargetsDao;
    await dao.visit(RecentTargetKind.space, uuid(0x341), at: at(1), keep: 5);
    await dao.visit(RecentTargetKind.channel, uuid(0x341), at: at(2), keep: 5);
    expect(await recent(), [
      (RecentTargetKind.channel, uuid(0x341)),
      (RecentTargetKind.space, uuid(0x341)),
    ]);
  });

  test('watchRecent emits again after a visit', () async {
    final emissions = <List<String>>[];
    final sub = db.recentTargetsDao
        .watchRecent(limit: 5)
        .listen((rows) => emissions.add([for (final r in rows) r.targetId]));
    addTearDown(sub.cancel);
    await pumpEventQueue();
    await db.recentTargetsDao.visit(
      RecentTargetKind.space,
      uuid(0x351),
      at: at(1),
      keep: 5,
    );
    await pumpEventQueue();
    expect(emissions, [
      <String>[],
      [uuid(0x351)],
    ]);
  });

  test('the kind codes round trip and an unknown code is rejected', () {
    for (final kind in RecentTargetKind.values) {
      expect(RecentTargetKind.fromCode(kind.code), kind);
    }
    expect(RecentTargetKind.values.map((k) => k.code), [
      'space',
      'channel',
      'thread',
    ]);
    expect(() => RecentTargetKind.fromCode('host'), throwsFormatException);
  });

  test('removing targets deletes their recent rows, the channels of a removed space and the '
      'threads of a removed channel included', () async {
    // Space 1 with channel 1 (thread 1); channel 2 in space 2 with thread 2; thread 3 in
    // channel 3, space 3, removed alone; channel 4 in space 4 stays.
    for (final n in [1, 2, 3, 4]) {
      await db.structureDao.upsertSpaceIfNewer(space(n));
      await db.structureDao.upsertChannelIfNewer(channel(n, spaceN: n));
      await db.threadsDao.upsertIfNewer(thread(n, channelN: n));
    }
    final dao = db.recentTargetsDao;
    var minute = 0;
    for (final n in [1, 2, 3, 4]) {
      await dao.visit(
        RecentTargetKind.space,
        space(n).id.value,
        at: at(minute++),
        keep: 50,
      );
      await dao.visit(
        RecentTargetKind.channel,
        channel(n).id.value,
        at: at(minute++),
        keep: 50,
      );
      await dao.visit(
        RecentTargetKind.thread,
        threadId(n).value,
        at: at(minute++),
        keep: 50,
      );
    }

    await db.removeTargets([
      TargetRef.space(space(1).id),
      TargetRef.channel(channel(2).id),
      TargetRef.thread(threadId(3)),
    ]);

    expect((await recent(limit: 50)).toSet(), {
      (RecentTargetKind.space, space(2).id.value),
      (RecentTargetKind.space, space(3).id.value),
      (RecentTargetKind.channel, channel(3).id.value),
      (RecentTargetKind.space, space(4).id.value),
      (RecentTargetKind.channel, channel(4).id.value),
      (RecentTargetKind.thread, threadId(4).value),
    });
  });
}
