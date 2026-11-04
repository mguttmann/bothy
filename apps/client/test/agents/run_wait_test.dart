// SPDX-License-Identifier: Apache-2.0

// Why a queued run waits in the thread view and in "Running now" (F-WARTESCHLANGE-GRUND,
// 09 5.2.1): the stored reason, the presence of the agent, its slot count and the runs of the
// same agent the device knows as running.

import 'package:client/agents/run_mapping.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';

void main() {
  final queued = run(3, RunState.queued, threadN: 3, reason: 'slots_full');
  final holders = [
    (run: run(1, RunState.running, threadN: 1), label: 'backend > Payment'),
    (run: run(2, RunState.waitingInput, threadN: 2), label: 'backend > Docs'),
  ];

  test('slots_full names the count, the running holders and the rest', () {
    final opened = <ThreadId>[];
    final wait = queuedWait(
      queued,
      presence: AgentRuntimeState.online,
      slotsTotal: 3,
      holders: holders,
      open: opened.add,
    )!;
    expect(wait.reason, DsWaitReason.slotsFull);
    expect((wait.slotsUsed, wait.slotsTotal), (3, 3));
    expect([for (final b in wait.blockers) b.label], ['backend > Payment']);
    expect(wait.hiddenBlockers, 2);
    wait.blockers.single.onTap!();
    expect(opened, [threadId(1)]);
  });

  test('an offline or never reporting runner is the reason', () {
    for (final presence in [
      AgentRuntimeState.offline,
      AgentRuntimeState.unknown,
    ]) {
      final wait = queuedWait(
        queued,
        presence: presence,
        slotsTotal: 3,
        holders: holders,
      )!;
      expect(wait.reason, DsWaitReason.agentOffline, reason: presence.code);
      expect(wait.blockers, isEmpty);
    }
  });

  test('without a slot count no numbers, and a started run has no wait', () {
    final wait = queuedWait(queued, presence: null, slotsTotal: null)!;
    expect(
      (wait.reason, wait.slotsTotal, wait.hiddenBlockers),
      (DsWaitReason.slotsFull, null, 0),
    );
    expect(
      queuedWait(run(1, RunState.running), presence: null, slotsTotal: 3),
      isNull,
    );
    expect(
      queuedWait(
        run(4, RunState.queued),
        presence: AgentRuntimeState.online,
        slotsTotal: 3,
      )!.reason,
      DsWaitReason.agentStart,
    );
  });
}
