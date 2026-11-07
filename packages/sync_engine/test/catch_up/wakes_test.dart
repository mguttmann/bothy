// SPDX-License-Identifier: Apache-2.0

// The wake-ups of a thread in the local table (01 6.21, 09 5.8.2, brief AR-08): `run.wake_changed`
// sets the state of a stored wake-up at once and reloads the thread's list (`listThreadWakes`),
// which replaces the rows; a thread this device does not hold is not loaded; a server before
// stage AR (no route, 404) leaves the table as it is. The channel switch
// `allow_autonomous_runs` is stored, and its absence (an older server) stays null.

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

/// Wake-up [n].
String wakeId(int n) => id(0x900 + n);

/// An API `RunWake` of thread [threadN].
Map<String, Object?> wakeJson(
  int n, {
  int threadN = 1,
  String state = 'pending',
  String? stateReason,
  int version = 1,
  String label = 'CI of #123',
}) => {
  'id': wakeId(n),
  'agent_id': robinId,
  'thread_id': threadId(threadN),
  'source_run_id': id(0x700 + n),
  'triggered_by': alexId,
  'kind': 'pr_checks',
  'reason': 'agent',
  'label': label,
  'condition': {'repository': 'example/app', 'number': 123},
  'not_before': '2026-10-15T10:20:00.000Z',
  'deadline': '2026-10-22T10:00:00.000Z',
  'state': state,
  'state_reason': ?stateReason,
  'created_at': at,
  'ended_at': ?(state == 'pending' ? null : at),
  'version': version,
};

/// `run.wake_changed` of wake-up [n] in thread [threadN] (D-AR-04-5).
Map<String, Object?> wakeChanged(
  int seq,
  int n, {
  int threadN = 1,
  String state = 'pending',
  String? stateReason,
}) => envelope(
  'run.wake_changed',
  seq: seq,
  resourceKind: 'thread',
  resourceId: threadId(threadN),
  scope: {'channel_id': channelId(1), 'thread_id': threadId(threadN)},
  payload: {
    'wake_id': wakeId(n),
    'thread_id': threadId(threadN),
    'agent_id': robinId,
    'triggered_by': alexId,
    'state': state,
    'state_reason': ?stateReason,
  },
);

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  Future<void> holdThread(int n) async {
    w.rest.routes['GET /threads/${threadId(n)}'] = (_) =>
        FakeReply(200, threadJson(n));
    await w.catchUp.loadThread(ThreadId.parse(threadId(n)));
  }

  Future<List<RunWake>> stored(int threadN) =>
      w.db.runsDao.watchWakesForThread(ThreadId.parse(threadId(threadN))).first;

  test('run.wake_changed reloads the wake-ups of a held thread', () async {
    await w.connect();
    await holdThread(1);
    w.rest.routes['GET /threads/${threadId(1)}/wakes'] = (_) => FakeReply(200, {
      'items': [wakeJson(1)],
    });
    await w.dispatch([wakeChanged(41, 1)]);
    final rows = await stored(1);
    expect(rows, hasLength(1));
    expect(rows.single.id, WakeId.parse(wakeId(1)));
    expect(rows.single.kind, RunWakeKind.prChecks);
    expect(rows.single.label, 'CI of #123');
    expect(rows.single.condition['number'], 123);

    // The end comes as an event; the list then carries the ended wake-up.
    w.rest.routes['GET /threads/${threadId(1)}/wakes'] = (_) => FakeReply(200, {
      'items': [
        wakeJson(
          1,
          state: 'cancelled',
          stateReason: 'cancelled_by_person',
          version: 2,
        ),
      ],
    });
    await w.dispatch([
      wakeChanged(
        42,
        1,
        state: 'cancelled',
        stateReason: 'cancelled_by_person',
      ),
    ]);
    final ended = (await stored(1)).single;
    expect(ended.state, RunWakeState.cancelled);
    expect(ended.stateReason, 'cancelled_by_person');
    expect(ended.version, const ResourceVersion(2));
  });

  test('the state of the event applies before the reload answers', () async {
    await w.connect();
    await holdThread(1);
    await w.db.runsDao.upsertWakeIfNewer(wakeFromJson(wakeJson(1)));
    // The reload fails (for example offline): the stored row still shows the new state.
    w.rest.routes['GET /threads/${threadId(1)}/wakes'] = (_) =>
        const FakeReply(403, {'code': 'forbidden'});
    await w.dispatch([
      wakeChanged(41, 1, state: 'expired', stateReason: 'deadline_passed'),
    ]);
    final row = (await stored(1)).single;
    expect(row.state, RunWakeState.expired);
    expect(row.stateReason, 'deadline_passed');
  });

  test('a thread this device does not hold is not loaded', () async {
    await w.connect();
    await w.dispatch([wakeChanged(41, 1, threadN: 5)]);
    expect(w.rest.count('/threads/${threadId(5)}/wakes'), 0);
    expect(await stored(5), isEmpty);
  });

  test(
    'loadThreadWakes pages through the list and replaces the rows',
    () async {
      await w.connect();
      await holdThread(1);
      await w.db.runsDao.upsertWakeIfNewer(wakeFromJson(wakeJson(9)));
      w.rest.routes['GET /threads/${threadId(1)}/wakes'] = (r) =>
          r.query['cursor'] == 'next'
          ? FakeReply(200, {
              'items': [wakeJson(2)],
            })
          : FakeReply(200, {
              'items': [wakeJson(1)],
              'next_cursor': 'next',
            });
      await w.catchUp.loadThreadWakes(ThreadId.parse(threadId(1)));
      expect(
        [for (final r in await stored(1)) r.id.value],
        [wakeId(1), wakeId(2)],
      );
    },
  );

  test('a server before stage AR answers 404 and nothing changes', () async {
    await w.connect();
    await holdThread(1);
    w.rest.routes['GET /threads/${threadId(1)}/wakes'] = (_) =>
        const FakeReply(404, {'code': 'not_found'});
    await w.catchUp.loadThreadWakes(ThreadId.parse(threadId(1)));
    expect(await stored(1), isEmpty);
  });

  test('allow_autonomous_runs is stored; its absence stays null', () {
    final on = channelFromJson({
      ...channelJson(1),
      'allow_autonomous_runs': true,
    });
    expect(on.allowAutonomousRuns, isTrue);
    final off = channelFromJson({
      ...channelJson(1),
      'allow_autonomous_runs': false,
    });
    expect(off.allowAutonomousRuns, isFalse);
    expect(channelFromJson(channelJson(1)).allowAutonomousRuns, isNull);
  });

  test('wakeFromJson reads every field of the API RunWake', () {
    final wake = wakeFromJson({
      ...wakeJson(1, state: 'fired', stateReason: 'condition_met'),
      'reason': 'usage_limit',
      'outage': 2,
      'result_text': 'all green',
      'fired_run_id': id(0x7f1),
      'fired_into_run_id': id(0x7f2),
    });
    expect(wake.reason, RunWakeReason.usageLimit);
    expect(wake.outage, 2);
    expect(wake.resultText, 'all green');
    expect(wake.firedRunId, RunId.parse(id(0x7f1)));
    expect(wake.firedIntoRunId, RunId.parse(id(0x7f2)));
    expect(wake.endedAt, DateTime.parse(at));
    expect(wake.notBefore, DateTime.utc(2026, 10, 15, 10, 20));
  });
}
