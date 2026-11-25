// SPDX-License-Identifier: Apache-2.0

// A device without a step-up key (Windows, D-M4-11-2) cannot give the fresh step-up of the
// terminal: the tab says so and names the devices that can, instead of "Something went wrong
// (open_failed)" (AUD-PKG-R2-owner-server, #677).

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';
import 'support/widgets.dart';

void main() {
  test('an opening without a step-up key fails with step_up_unavailable and '
      'sends nothing', () async {
    final h = await Harness.create(stepUpUnavailable: true);
    await h.open();
    expect(h.controller.state.phase, TerminalPhase.failed);
    expect(h.controller.state.problemCode, stepUpUnavailableCode);
    expect(h.host.openStreams, 0);
  });

  testWidgets('the banner names the hint, not a generic failure', (
    tester,
  ) async {
    await pumpApp(
      tester,
      const TerminalStateBanner(
        state: TerminalTabState(
          phase: TerminalPhase.failed,
          problemCode: stepUpUnavailableCode,
        ),
      ),
    );
    final l = lookupAppLocalizations(const Locale('en'));
    expect(find.text(l.terminalStepUpUnavailable), findsOneWidget);
    expect(find.textContaining('open_failed'), findsNothing);
    expect(find.textContaining('could not be opened'), findsNothing);
  });
}
