// SPDX-License-Identifier: Apache-2.0

// The trigger state of a triggering message (brief M4-21 Tests; 01 6.3; components.md 5; 09 6):
// every state of the API enum shows an icon with its tooltip and screen reader label,
// `cancelled` is neutral and not styled as an error, `failed` carries the translated reason.

import 'package:client/agents/trigger_state_mapping.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

void main() {
  test('every state of the API enum maps to its own icon state', () {
    for (final state in TriggerState.values) {
      final view = triggerOf(msg(1).copyWith(triggerState: state));
      expect(view!.state.code, state.code, reason: state.code);
    }
    expect(triggerOf(msg(1)), isNull);
    expect(triggerIsError(DsTriggerState.failed), isTrue);
    expect(triggerIsError(DsTriggerState.cancelled), isFalse);
  });

  testWidgets('the history shows each state with tooltip and label; '
      'cancelled is neutral; failed names the reason', (tester) async {
    final host = await ColumnHost.open(tester);
    final states = [
      TriggerState.received,
      TriggerState.queued,
      TriggerState.working,
      TriggerState.done,
      TriggerState.failed,
      TriggerState.cancelled,
      TriggerState.followupDelivered,
    ];
    await host.run(
      tester,
      () => seedThread(
        host.db,
        messages: [
          for (var i = 0; i < states.length; i++)
            msg(i + 1, author: alex, minutes: i * 10).copyWith(
              triggerState: states[i],
              triggerStateReason: states[i] == TriggerState.failed
                  ? 'project_not_ready'
                  : null,
            ),
        ],
      ),
    );
    await pumpThread(tester, host, FakeThreadReads(host.db));

    final expected = {
      DsTriggerState.received: en.triggerStateReceived,
      DsTriggerState.queued: en.triggerStateQueued,
      DsTriggerState.working: en.triggerStateWorking,
      DsTriggerState.done: en.triggerStateDone,
      DsTriggerState.failed: en.triggerStateFailedWithReason(
        en.liveBoxReasonProjectNotReady,
      ),
      DsTriggerState.cancelled: en.triggerStateCancelled,
      DsTriggerState.followupDelivered: en.triggerStateFollowupDelivered,
    };
    final icons = tester
        .widgetList<DsTriggerStateIcon>(find.byType(DsTriggerStateIcon))
        .toList();
    expect({for (final i in icons) i.state}, expected.keys.toSet());
    for (final entry in expected.entries) {
      final icon = find.byWidgetPredicate(
        (w) => w is DsTriggerStateIcon && w.state == entry.key,
      );
      expect(
        find.descendant(of: icon, matching: find.byType(Tooltip)),
        findsOneWidget,
      );
      final tooltip = tester.widget<Tooltip>(
        find.descendant(of: icon, matching: find.byType(Tooltip)),
      );
      expect(tooltip.message, entry.value, reason: entry.key.code);
      expect(find.bySemanticsLabel(entry.value), findsWidgets);
    }

    // `cancelled` is a deliberate stop: never the color of `failed` (09 6).
    final context = tester.element(find.byType(DsTriggerStateIcon).first);
    final colors = context.dsColors;
    Color colorOf(DsTriggerState state) => tester
        .widget<DsIcon>(
          find.descendant(
            of: find.byWidgetPredicate(
              (w) => w is DsTriggerStateIcon && w.state == state,
            ),
            matching: find.byType(DsIcon),
          ),
        )
        .color!;
    expect(colorOf(DsTriggerState.cancelled), isNot(colors.triggerFailed));
    expect(colorOf(DsTriggerState.cancelled), colors.runCancelled);
    expect(colorOf(DsTriggerState.failed), colors.triggerFailed);
    await host.dispose(tester);
  });

  test('a follow-up held for the end of its run shows that it waits '
      '(FIX-followup-media-steer)', () {
    // The run was queued at minute 1; the follow-up came at minute 5.
    final followup = msg(
      2,
      minutes: 5,
    ).copyWith(triggerState: TriggerState.received);
    final active = run(1, RunState.running);
    expect(
      triggerOf(followup, runs: [active])!.state,
      DsTriggerState.followupWaiting,
    );
    // Without an active run, and for the run's own trigger, `received` stays.
    expect(
      triggerOf(followup, runs: [run(1, RunState.completed)])!.state,
      DsTriggerState.received,
    );
    final trigger = msg(
      1,
      minutes: 0,
    ).copyWith(triggerState: TriggerState.received);
    expect(triggerOf(trigger, runs: [active])!.state, DsTriggerState.received);
  });

  test('a follow-up discarded by a stop shows "discarded"', () {
    final stopped = run(
      1,
      RunState.cancelled,
      reason: 'user_cancelled',
    ).copyWith(finishedAt: t0.add(const Duration(minutes: 9)));
    final followup = msg(2, minutes: 5).copyWith(
      triggerState: TriggerState.cancelled,
      triggerStateReason: 'user_cancelled',
    );
    expect(
      triggerOf(followup, runs: [stopped])!.state,
      DsTriggerState.followupDiscarded,
    );
    // The stopped run's own trigger stays "canceled".
    final trigger = msg(1, minutes: 0).copyWith(
      triggerState: TriggerState.cancelled,
      triggerStateReason: 'user_cancelled',
    );
    expect(
      triggerOf(trigger, runs: [stopped])!.state,
      DsTriggerState.cancelled,
    );
  });

  testWidgets('the history labels a waiting follow-up', (tester) async {
    final host = await ColumnHost.open(tester);
    await host.run(tester, () async {
      await seedThread(
        host.db,
        messages: [
          msg(1, author: alex).copyWith(triggerState: TriggerState.working),
          msg(
            2,
            author: alex,
            minutes: 5,
          ).copyWith(triggerState: TriggerState.received),
        ],
      );
      await host.db.runsDao.upsertIfNewer(run(1, RunState.running));
    });
    await pumpThread(tester, host, FakeThreadReads(host.db));
    expect(find.bySemanticsLabel(en.triggerStateFollowupWaiting), findsWidgets);
    await host.dispose(tester);
  });
}
