// SPDX-License-Identifier: Apache-2.0

// The rules of the status dots (01 6.17; brief M4-52 Design notes 1, 2 and 4): the status of a
// chat from its newest run, when the person has seen it (open chat, this device's mark, a read
// written after the change, nothing unread for a change that comes with a message), a failure
// without a message, a new change after a seen one, and the aggregate of a channel.

import 'package:client/sidebar/sidebar_status.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/structure.dart';

/// Run [n] of thread [threadN] in [state], queued [n] minutes after [t0], started a minute
/// later and, when ended, finished two minutes after queueing.
RunSummary at(int n, RunState state, {int threadN = 1}) {
  final r = run(n, state, threadN: threadN);
  final ended = {
    RunState.completed,
    RunState.failed,
    RunState.cancelled,
  }.contains(state);
  return r.copyWith(
    startedAt: state == RunState.queued
        ? null
        : r.queuedAt.add(const Duration(minutes: 1)),
    finishedAt: ended ? r.queuedAt.add(const Duration(minutes: 2)) : null,
  );
}

ThreadSeenState mark(RunSummary r, [RunState? state]) => ThreadSeenState(
  threadId: r.threadId,
  runId: r.id,
  runState: state ?? r.state,
);

/// The read state of thread 1 written at [written].
ReadState readAt(DateTime written) => readState(1).copyWith(updatedAt: written);

const ThreadCounts unread = ThreadCounts(unread: 1, mentions: 0);

DsChatStatus? unseen(
  RunSummary? r, {
  bool open = false,
  ThreadSeenState? seen,
  ReadState? read,
  ThreadCounts counts = unread,
}) => unseenChatStatus(
  newestRun: r,
  open: open,
  seen: seen,
  readState: read,
  counts: counts,
);

void main() {
  group('status of a run (Design note 1)', () {
    test('each state maps to its status; cancelled and unknown to none', () {
      expect(chatStatusOf(RunState.queued), DsChatStatus.working);
      expect(chatStatusOf(RunState.running), DsChatStatus.working);
      expect(chatStatusOf(RunState.waitingInput), DsChatStatus.waiting);
      expect(chatStatusOf(RunState.completed), DsChatStatus.done);
      expect(chatStatusOf(RunState.failed), DsChatStatus.failed);
      expect(chatStatusOf(RunState.cancelled), isNull);
      expect(chatStatusOf(RunState.unknown), isNull);
    });

    test('the newest run of each thread by queue time', () {
      final runs = [
        at(3, RunState.completed),
        at(1, RunState.failed),
        at(2, RunState.running, threadN: 2),
        at(4, RunState.waitingInput, threadN: 2),
      ];
      final newest = newestRuns(runs);
      expect(newest[threadId(1)]!.id, runs[0].id);
      expect(newest[threadId(2)]!.id, runs[3].id);
    });

    test('the time of the change: queue for working, end for done and failed, '
        'none for waiting', () {
      final working = at(1, RunState.running);
      expect(statusChangedAt(working), working.queuedAt);
      final done = at(2, RunState.completed);
      expect(statusChangedAt(done), done.finishedAt);
      final failed = at(3, RunState.failed);
      expect(statusChangedAt(failed), failed.finishedAt);
      expect(statusChangedAt(at(4, RunState.waitingInput)), isNull);
    });
  });

  group('unseen (Design note 2)', () {
    test('each state is unseen without any sign of a look', () {
      expect(unseen(at(1, RunState.running)), DsChatStatus.working);
      expect(unseen(at(1, RunState.queued)), DsChatStatus.working);
      expect(unseen(at(1, RunState.waitingInput)), DsChatStatus.waiting);
      expect(unseen(at(1, RunState.completed)), DsChatStatus.done);
      expect(unseen(at(1, RunState.failed)), DsChatStatus.failed);
      expect(unseen(at(1, RunState.cancelled)), isNull);
      expect(unseen(null), isNull);
    });

    test('the open chat never shows a status, whatever its colour', () {
      for (final s in [
        RunState.running,
        RunState.waitingInput,
        RunState.completed,
        RunState.failed,
      ]) {
        expect(unseen(at(1, s), open: true), isNull, reason: s.code);
      }
    });

    test('the mark of this device clears the status it names', () {
      final failed = at(1, RunState.failed);
      expect(unseen(failed, seen: mark(failed)), isNull);
      // queued and running are both working: a run that starts is no new change.
      final running = at(1, RunState.running);
      expect(unseen(running, seen: mark(running, RunState.queued)), isNull);
    });

    test('a new change after the mark brings the status back', () {
      final r = at(1, RunState.running);
      // The run the person watched finishes while they are elsewhere: green.
      expect(
        unseen(r.copyWith(state: RunState.completed), seen: mark(r)),
        DsChatStatus.done,
      );
      // The agent asks something: blue.
      expect(
        unseen(r.copyWith(state: RunState.waitingInput), seen: mark(r)),
        DsChatStatus.waiting,
      );
      // An error: red.
      expect(
        unseen(r.copyWith(state: RunState.failed), seen: mark(r)),
        DsChatStatus.failed,
      );
      // A new run after a seen one.
      final done = at(1, RunState.completed);
      expect(
        unseen(at(2, RunState.running), seen: mark(done)),
        DsChatStatus.working,
      );
    });

    test('a read written at or after the change clears it on every device', () {
      final done = at(1, RunState.completed);
      expect(unseen(done, read: readAt(done.finishedAt!)), isNull);
      expect(
        unseen(
          done,
          read: readAt(done.finishedAt!.subtract(const Duration(seconds: 1))),
        ),
        DsChatStatus.done,
      );
      final failed = at(1, RunState.failed);
      expect(
        unseen(
          failed,
          read: readAt(failed.finishedAt!.add(const Duration(seconds: 5))),
        ),
        isNull,
      );
    });

    test('nothing unread clears a change that comes with a message', () {
      // The question card comes before the run waits (09 5): read means seen.
      expect(
        unseen(at(1, RunState.waitingInput), counts: ThreadCounts.zero),
        isNull,
      );
      expect(
        unseen(
          at(1, RunState.waitingInput),
          counts: ThreadCounts.zero,
          read: readAt(t0),
        ),
        isNull,
      );
      // Without a read time the live box anchor and the answer were read.
      expect(
        unseen(at(1, RunState.running), counts: ThreadCounts.zero),
        isNull,
      );
      expect(
        unseen(at(1, RunState.completed), counts: ThreadCounts.zero),
        isNull,
      );
      // An unread mention keeps it.
      expect(
        unseen(
          at(1, RunState.waitingInput),
          counts: const ThreadCounts(unread: 0, mentions: 1),
        ),
        DsChatStatus.waiting,
      );
    });

    test('a read time older than the change decides over nothing unread', () {
      // The agent finished without a message after the person's last read: green.
      final done = at(1, RunState.completed);
      expect(
        unseen(done, counts: ThreadCounts.zero, read: readAt(done.queuedAt)),
        DsChatStatus.done,
      );
    });

    test('a failure without a message stays until it is seen', () {
      final failed = at(1, RunState.failed);
      expect(unseen(failed, counts: ThreadCounts.zero), DsChatStatus.failed);
      expect(
        unseen(failed, counts: ThreadCounts.zero, read: readAt(t0)),
        DsChatStatus.failed,
      );
    });
  });

  group('aggregate (Design note 4)', () {
    test('red over blue over yellow over green', () {
      expect(
        aggregateStatus([
          DsChatStatus.done,
          DsChatStatus.working,
          DsChatStatus.waiting,
        ]),
        DsChatStatus.waiting,
      );
      expect(
        aggregateStatus([DsChatStatus.done, DsChatStatus.failed, null]),
        DsChatStatus.failed,
      );
      expect(
        aggregateStatus([DsChatStatus.done, DsChatStatus.working]),
        DsChatStatus.working,
      );
      expect(aggregateStatus([DsChatStatus.done]), DsChatStatus.done);
      expect(aggregateStatus(const []), isNull);
    });

    test('the unseen status of every chat of a channel', () {
      final runs = [
        at(1, RunState.completed),
        at(2, RunState.failed, threadN: 2),
        at(3, RunState.waitingInput, threadN: 3),
      ];
      final statuses = unseenStatuses(
        runs: runs,
        open: {threadId(2)},
        seen: {threadId(1): mark(runs[0])},
        readStates: const [],
        counts: {for (final r in runs) r.threadId: unread},
      );
      expect(statuses, {threadId(3): DsChatStatus.waiting});
    });
  });
}
