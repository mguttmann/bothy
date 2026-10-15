// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  testWidgets('every trigger state has its icon, color token and label', (
    tester,
  ) async {
    final l = await en();
    final expected = {
      DsTriggerState.received: (DsIcons.triggerReceived, l.triggerStateReceived),
      DsTriggerState.queued: (DsIcons.triggerQueued, l.triggerStateQueued),
      DsTriggerState.working: (DsIcons.triggerWorking, l.triggerStateWorking),
      DsTriggerState.done: (DsIcons.triggerDone, l.triggerStateDone),
      DsTriggerState.failed: (DsIcons.triggerFailed, l.triggerStateFailed),
      DsTriggerState.cancelled: (DsIcons.runCancelled, l.triggerStateCancelled),
      DsTriggerState.followupDelivered: (
        DsIcons.triggerFollowup,
        l.triggerStateFollowupDelivered,
      ),
    };
    final colors = DsColors.light;
    final tokens = {
      DsTriggerState.received: colors.triggerReceived,
      DsTriggerState.queued: colors.triggerQueued,
      DsTriggerState.working: colors.triggerWorking,
      DsTriggerState.done: colors.triggerDone,
      DsTriggerState.failed: colors.triggerFailed,
      DsTriggerState.cancelled: colors.runCancelled,
      DsTriggerState.followupDelivered: colors.triggerFollowup,
    };
    for (final MapEntry(key: state, value: (glyph, label)) in expected.entries) {
      await pumpDs(tester, DsTriggerStateIcon(state: state));
      final icon = tester.widget<DsIcon>(find.byType(DsIcon));
      expect(icon.glyph, glyph, reason: state.name);
      expect(icon.color, tokens[state], reason: state.name);
      expect(find.bySemanticsLabel(label), findsOneWidget, reason: state.name);
    }
  });

  testWidgets('a failed state carries the reason in tooltip and label', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(
      tester,
      const DsTriggerStateIcon(
        state: DsTriggerState.failed,
        reason: 'project_not_ready',
      ),
    );
    final text = l.triggerStateFailedWithReason(l.liveBoxReasonProjectNotReady);
    expect(find.bySemanticsLabel(text), findsOneWidget);
    final gesture = await tester.createGesture(kind: PointerDeviceKind.mouse);
    await gesture.addPointer(location: Offset.zero);
    await gesture.moveTo(tester.getCenter(find.byType(DsIcon)));
    await tester.pumpAndSettle();
    expect(find.text(text), findsOneWidget);
    await gesture.removePointer();
  });

  testWidgets('an unknown reason shows its code; cancelled with reason', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(
      tester,
      const DsTriggerStateIcon(
        state: DsTriggerState.failed,
        reason: 'run_active_other_agent',
      ),
    );
    expect(
      find.bySemanticsLabel(l.triggerStateFailedWithReason('run_active_other_agent')),
      findsOneWidget,
    );
    await pumpDs(
      tester,
      const DsTriggerStateIcon(
        state: DsTriggerState.cancelled,
        reason: 'user_cancelled',
        reasonName: 'Sam',
      ),
    );
    expect(
      find.bySemanticsLabel(
        l.triggerStateCancelledWithReason(l.liveBoxReasonUserCancelled('Sam')),
      ),
      findsOneWidget,
    );
  });

  testWidgets('tapping jumps to the live box', (tester) async {
    var jumps = 0;
    await pumpDs(
      tester,
      DsTriggerStateIcon(state: DsTriggerState.working, onTap: () => jumps++),
    );
    await tester.tap(find.byType(DsTriggerStateIcon));
    expect(jumps, 1);
  });

  testWidgets('an unknown state shows the code', (tester) async {
    final l = await en();
    await pumpDs(
      tester,
      DsTriggerStateIcon(
        state: DsTriggerState.fromCode('future_state'),
        code: 'future_state',
      ),
    );
    expect(
      find.bySemanticsLabel(l.triggerStateUnknown('future_state')),
      findsOneWidget,
    );
  });

  test('codes map both ways and unknown codes never throw', () {
    for (final s in DsTriggerState.values.where((s) => s != DsTriggerState.unknown)) {
      expect(DsTriggerState.fromCode(s.code), s);
    }
    expect(DsTriggerState.fromCode('x'), DsTriggerState.unknown);
    expect(DsTriggerState.fromCode(null), DsTriggerState.unknown);
    expect(DsTriggerState.fromCode(''), DsTriggerState.unknown);
  });
}
