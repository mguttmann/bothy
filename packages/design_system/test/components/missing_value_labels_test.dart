// SPDX-License-Identifier: Apache-2.0

// design-system-20: a value the screen does not show is not read either.

import 'package:design_system/design_system.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  testWidgets('the live box label leaves out an unmeasured context', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(
      tester,
      DsLiveBox(
        const DsRunData(
          id: 'r1',
          agentName: 'Robin',
          state: DsRunState.running,
          elapsed: Duration(minutes: 4, seconds: 12),
        ),
        visibleSteps: 6,
        announceMinInterval: const Duration(seconds: 10),
        onStop: () {},
      ),
    );
    expect(
      find.bySemanticsLabel(
        RegExp(
          '^${RegExp.escape(l.liveBoxSemanticsLabelNoContext('Robin', l.liveBoxStateRunning, '4:12'))}(?!, context)',
        ),
      ),
      findsOneWidget,
    );
    expect(
      find.bySemanticsLabel(RegExp(RegExp.escape(l.liveBoxContext('0%')))),
      findsNothing,
    );
  });

  testWidgets('the personal badge without a holder names no one', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(tester, const DsBillingBadge(mode: DsBillingMode.personal));
    expect(
      find.bySemanticsLabel(l.billingLabelPersonalNoHolder),
      findsOneWidget,
    );
  });
}
