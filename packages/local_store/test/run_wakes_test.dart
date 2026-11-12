// SPDX-License-Identifier: Apache-2.0

// The local table of wake-ups (01 6.21, 09 5.8.1, brief AR-08 Scope): `run_wakes` mirrors the
// API `RunWake` by version, a thread's list replaces its rows, the state of `run.wake_changed`
// applies to a stored row before the reload, and the rows go with their thread. The channel
// switch `allow_autonomous_runs` round-trips, null (an older server) included.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

RunWake wake(
  int n, {
  int threadN = 1,
  int version = 1,
  RunWakeState state = RunWakeState.pending,
  RunWakeKind kind = RunWakeKind.at,
  String label = 'CI of #123',
  int minutes = 20,
}) => RunWake(
  id: WakeId.parse(uuid(0x900 + n)),
  agentId: robin,
  threadId: threadId(threadN),
  sourceRunId: RunId.parse(uuid(0x700 + n)),
  triggeredBy: alex,
  kind: kind,
  reason: RunWakeReason.agent,
  label: label,
  condition: kind == RunWakeKind.command
      ? const {'command_line': 'make test'}
      : const <String, Object?>{},
  notBefore: at(minutes),
  deadline: at(minutes + 60),
  state: state,
  resultText: '',
  createdAt: at(0),
  version: ResourceVersion(version),
);

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  test('a wake-up round-trips with every field', () async {
    final full = RunWake(
      id: WakeId.parse(uuid(0x901)),
      agentId: robin,
      threadId: threadId(1),
      sourceRunId: RunId.parse(uuid(0x701)),
      triggeredBy: alex,
      kind: RunWakeKind.prChecks,
      reason: RunWakeReason.usageLimit,
      label: 'checks',
      condition: const {
        'repository': 'example/app',
        'number': 123,
        'head_commit': 'abc',
      },
      outage: 2,
      notBefore: at(5),
      deadline: at(50),
      state: RunWakeState.fired,
      stateReason: 'condition_met',
      resultText: 'all green',
      firedRunId: RunId.parse(uuid(0x7f1)),
      firedIntoRunId: RunId.parse(uuid(0x7f2)),
      createdAt: at(1),
      endedAt: at(6),
      version: const ResourceVersion(3),
    );
    expect(await db.runsDao.upsertWakeIfNewer(full), isTrue);
    expect(await db.runsDao.watchWakesForThread(threadId(1)).first, [full]);
  });

  test('a newer version replaces a row, an older one is ignored', () async {
    final dao = db.runsDao;
    expect(await dao.upsertWakeIfNewer(wake(1)), isTrue);
    final cancelled = wake(1, version: 2, state: RunWakeState.cancelled);
    expect(await dao.upsertWakeIfNewer(cancelled), isTrue);
    expect(await dao.upsertWakeIfNewer(wake(1)), isFalse);
    expect(await dao.watchWakesForThread(threadId(1)).first, [cancelled]);
  });

  test(
    'the list of a thread replaces its rows and leaves other threads',
    () async {
      final dao = db.runsDao;
      await dao.upsertWakeIfNewer(wake(1));
      await dao.upsertWakeIfNewer(wake(2));
      await dao.upsertWakeIfNewer(wake(3, threadN: 2));
      await dao.replaceThreadWakes(threadId(1), [wake(2, version: 2)]);
      expect(await dao.watchWakesForThread(threadId(1)).first, [
        wake(2, version: 2),
      ]);
      expect(await dao.watchWakesForThread(threadId(2)).first, [
        wake(3, threadN: 2),
      ]);
    },
  );

  test('the rows of a thread come in the order they fire', () async {
    final dao = db.runsDao;
    await dao.upsertWakeIfNewer(wake(1, minutes: 30));
    await dao.upsertWakeIfNewer(wake(2, minutes: 10));
    final rows = await dao.watchWakesForThread(threadId(1)).first;
    expect([for (final w in rows) w.id], [wake(2).id, wake(1).id]);
  });

  test('the state of run.wake_changed applies to a stored row', () async {
    final dao = db.runsDao;
    await dao.upsertWakeIfNewer(wake(1));
    expect(
      await dao.applyWakeState(
        wake(1).id,
        RunWakeState.expired,
        stateReason: 'deadline_passed',
      ),
      isTrue,
    );
    final row = (await dao.watchWakesForThread(threadId(1)).first).single;
    expect(row.state, RunWakeState.expired);
    expect(row.stateReason, 'deadline_passed');
    // The reload brings the next version; the applied state did not raise it.
    expect(row.version, const ResourceVersion(1));
    expect(
      await dao.applyWakeState(WakeId.parse(uuid(0x9ff)), RunWakeState.fired),
      isFalse,
    );
  });

  test('the watch emits every change', () async {
    final sizes = <int>[];
    final sub = db.runsDao
        .watchWakesForThread(threadId(1))
        .listen((rows) => sizes.add(rows.length));
    await pumpEventQueue();
    await db.runsDao.upsertWakeIfNewer(wake(1));
    await pumpEventQueue();
    await sub.cancel();
    expect(sizes, [0, 1]);
  });

  test('the removal of a thread deletes its wake-ups', () async {
    await db.structureDao.upsertSpaceIfNewer(space(1));
    await db.structureDao.upsertChannelIfNewer(channel(1));
    await db.threadsDao.upsertIfNewer(thread(1));
    await db.threadsDao.upsertIfNewer(thread(2));
    await db.runsDao.upsertWakeIfNewer(wake(1));
    await db.runsDao.upsertWakeIfNewer(wake(2, threadN: 2));

    await db.removeTargets([TargetRef.thread(threadId(1))]);

    expect(await db.runsDao.watchWakesForThread(threadId(1)).first, isEmpty);
    expect(
      await db.runsDao.watchWakesForThread(threadId(2)).first,
      hasLength(1),
    );
  });

  test('a resync clears the wake-ups with the mirror', () async {
    await db.runsDao.upsertWakeIfNewer(wake(1));
    await db.syncStateDao.commitPage(
      (txn) => db.syncStateDao.clearMirror(txn, visibleChannels: const {}),
      cursor: const GlobalSeq(42),
    );
    expect(await db.runsDao.watchWakesForThread(threadId(1)).first, isEmpty);
  });

  test(
    'allow_autonomous_runs of a channel round-trips, null included',
    () async {
      final dao = db.structureDao;
      final older = channel(1);
      expect(await dao.upsertChannelIfNewer(older), isTrue);
      expect((await dao.channelById(older.id))!.allowAutonomousRuns, isNull);
      final on = channel(1, version: 2).copyWith(allowAutonomousRuns: true);
      expect(await dao.upsertChannelIfNewer(on), isTrue);
      expect(await dao.channelById(on.id), on);
      final off = channel(1, version: 3).copyWith(allowAutonomousRuns: false);
      expect(await dao.upsertChannelIfNewer(off), isTrue);
      expect((await dao.channelById(on.id))!.allowAutonomousRuns, isFalse);
    },
  );
}
