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
}
