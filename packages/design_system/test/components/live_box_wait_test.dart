// SPDX-License-Identifier: Apache-2.0

// Why a queued run waits (09 5.2.1, F-WARTESCHLANGE-GRUND): the live box never says only
// "Queued"; it names the reason, the slot count and the runs that hold the slots.

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'support.dart';

const _interval = Duration(seconds: 10);

DsRunData _queued({String? reason, DsWait? wait}) => DsRunData(
  id: 'r3',
  agentName: 'Robin',
  state: DsRunState.queued,
  stateReason: reason,
  wait: wait,
);

void main() {
  test('the wait reason: held first, then offline, then the runner reason', () {
    expect(
      dsWaitReason('invocation_held', agentOffline: true),
      DsWaitReason.invocationHeld,
    );
    expect(
      dsWaitReason('slots_full', agentOffline: true),
      DsWaitReason.agentOffline,
    );
    expect(dsWaitReason(null, agentOffline: true), DsWaitReason.agentOffline);
    expect(dsWaitReason('slots_full'), DsWaitReason.slotsFull);
    expect(dsWaitReason('thread_busy'), DsWaitReason.threadBusy);
    expect(dsWaitReason(null), DsWaitReason.agentStart);
    expect(dsWaitReason('a_newer_code'), DsWaitReason.agentStart);
  });

  testWidgets('a full runner names the slot count and the runs that hold it', (
    tester,
  ) async {
    final l = await en();
    final opened = <String>[];
    await pumpDs(
      tester,
      DsLiveBox(
        _queued(
          reason: 'slots_full',
          wait: DsWait(
            reason: DsWaitReason.slotsFull,
            slotsUsed: 2,
            slotsTotal: 2,
            blockers: [
              DsWaitBlocker(
                label: 'backend / Payment',
                onTap: () => opened.add('t1'),
              ),
            ],
            hiddenBlockers: 1,
          ),
        ),
        visibleSteps: 6,
        announceMinInterval: _interval,
      ),
    );
    expect(find.text(l.liveBoxWaitSlotsFull(2, 2)), findsOneWidget);
    expect(find.text(l.liveBoxStateQueued), findsNothing);
    expect(find.text(l.liveBoxWaitHeldBy), findsOneWidget);
    expect(find.textContaining('backend / Payment'), findsOneWidget);
    expect(find.text(l.liveBoxWaitHiddenRuns(1)), findsOneWidget);
    await tester.tap(
      find.bySemanticsLabel(l.liveBoxWaitOpenRun('backend / Payment')),
    );
    expect(opened, ['t1']);
  });

  testWidgets('a run the reader may not open is no button', (tester) async {
    final l = await en();
    await pumpDs(
      tester,
      DsLiveBox(
        _queued(
          reason: 'slots_full',
          wait: const DsWait(
            reason: DsWaitReason.slotsFull,
            slotsUsed: 4,
            slotsTotal: 4,
            blockers: [DsWaitBlocker(label: 'dev-runner · 0a77f1')],
          ),
        ),
        visibleSteps: 6,
        announceMinInterval: _interval,
      ),
    );
    expect(find.text(l.liveBoxWaitSlotsFull(4, 4)), findsOneWidget);
    expect(
      find.bySemanticsLabel(l.liveBoxWaitOpenRun('dev-runner · 0a77f1')),
      findsNothing,
    );
  });

  testWidgets('an offline runner is named, also on the phone line', (
    tester,
  ) async {
    final l = await en();
    final run = _queued(
      reason: 'slots_full',
      wait: DsWait(reason: dsWaitReason('slots_full', agentOffline: true)),
    );
    await pumpDs(
      tester,
      DsLiveBox(run, visibleSteps: 6, announceMinInterval: _interval),
    );
    expect(find.text(l.liveBoxWaitAgentOffline), findsOneWidget);
    expect(find.text(l.liveBoxWaitHeldBy), findsNothing);
    await pumpDs(
      tester,
      DsLiveBox.mobile(run, announceMinInterval: _interval, onOpen: () {}),
    );
    expect(
      find.textContaining(l.liveBoxWaitAgentOffline, findRichText: true),
      findsOneWidget,
    );
  });

  testWidgets('without a wait the stored reason decides, never only Queued', (
    tester,
  ) async {
    final l = await en();
    for (final (reason, text) in [
      ('thread_busy', l.liveBoxWaitThreadBusy),
      ('invocation_held', l.liveBoxWaitInvocationHeld),
      ('slots_full', l.liveBoxWaitSlotsFullNoCount),
      (null, l.liveBoxWaitAgentStart),
    ]) {
      await pumpDs(
        tester,
        DsLiveBox(
          _queued(reason: reason),
          visibleSteps: 6,
          announceMinInterval: _interval,
        ),
      );
      expect(find.text(text), findsOneWidget, reason: '$reason');
      expect(find.text(l.liveBoxStateQueued), findsNothing, reason: '$reason');
    }
  });

  testWidgets('German texts', (tester) async {
    final g = await de();
    await pumpDs(
      tester,
      DsLiveBox(
        _queued(
          reason: 'slots_full',
          wait: const DsWait(
            reason: DsWaitReason.slotsFull,
            slotsUsed: 4,
            slotsTotal: 4,
          ),
        ),
        visibleSteps: 6,
        announceMinInterval: _interval,
      ),
      locale: const Locale('de'),
    );
    expect(find.text(g.liveBoxWaitSlotsFull(4, 4)), findsOneWidget);
    expect(
      g.liveBoxWaitSlotsFull(4, 4),
      'Wartet auf einen freien Platz (4 von 4 belegt)',
    );
  });

  test('the state word of a queued run is the wait text', () async {
    final AppLocalizations l = await en();
    expect(
      dsRunStateWord(
        l,
        _queued(
          wait: const DsWait(
            reason: DsWaitReason.slotsFull,
            slotsUsed: 3,
            slotsTotal: 3,
          ),
        ),
      ),
      l.liveBoxWaitSlotsFull(3, 3),
    );
  });
}
