// SPDX-License-Identifier: Apache-2.0

// The phone terminal M8 has the same tab states as the desktop (FIX-terminal-session-lifecycle):
// "End" of a live session ends it on the host and leaves no tab; an ended session shows "Close"
// instead of the dead "Detach" and "End", and "Close" removes the tab.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../../terminal/support.dart';
import 'support.dart';

void main() {
  testWidgets(
    'End on the phone ends the session on the host and closes the tab',
    (tester) async {
      final h = await PhoneTerminalHarness.pump(tester);
      final c = await h.openShell(tester);
      final id = c.state.sessionId!;
      await tester.tap(find.widgetWithText(OutlinedButton, en.terminalEnd));
      await pumpUntil(tester, () => h.tabs.tabs.isEmpty);
      expect(h.world.api.closes, [id]);
      await tester.pump();
      expect(
        find.byKey(const ValueKey('phone.terminal.empty')),
        findsOneWidget,
      );
    },
  );

  testWidgets(
    'an ended session on the phone offers Close, which removes the tab',
    (tester) async {
      final h = await PhoneTerminalHarness.pump(tester);
      final c = await h.openShell(tester);
      h.world.events.changes.add(
        TerminalSessionUpdated(
          c.state.sessionId!,
          closed: true,
          closeReason: 'detach_expired',
        ),
      );
      await pumpUntil(tester, () => c.state.phase == TerminalPhase.ended);
      await tester.pump();
      expect(
        find.widgetWithText(OutlinedButton, en.terminalDetach),
        findsNothing,
      );
      expect(find.widgetWithText(OutlinedButton, en.terminalEnd), findsNothing);
      await tester.tap(
        find.widgetWithText(OutlinedButton, en.terminalCloseTab),
      );
      await tester.pump();
      expect(h.tabs.tabs, isEmpty);
    },
  );
}
