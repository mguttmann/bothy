// SPDX-License-Identifier: Apache-2.0

// Accessibility of the status page (brief M7-21; 01 section 8): labelled controls, contrast of
// text in the default theme, the sections as headings, the overall state in words, and "Server
// unreachable" as a live region.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

Widget _screen(FakeAdminRepository r, {bool unreachable = false}) =>
    StatusScreen(
      source: LiveStatusSource(
        loader: r.status,
        changes: const Stream.empty(),
        unreachable: unreachable ? Stream.value(true) : const Stream.empty(),
      ),
      manage: r,
      timeZone: 'Europe/Berlin',
      clock: testClock,
    );

void main() {
  testWidgets('labelled controls, contrast, headings and the state in words', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    final r = FakeAdminRepository(prompt: StepUpDialogPrompt(() => null));
    await pumpInApp(tester, _screen(r));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    await expectLater(tester, meetsGuideline(textContrastGuideline));
    for (final heading in ['Components', 'Open incidents']) {
      expect(
        tester.getSemantics(find.text(heading)),
        matchesSemantics(label: heading, isHeader: true),
        reason: heading,
      );
    }
    expect(find.bySemanticsLabel('Overall state: Degraded'), findsOneWidget);
    semantics.dispose();
  });

  testWidgets('Server unreachable is announced', (tester) async {
    final semantics = tester.ensureSemantics();
    final r = FakeAdminRepository(prompt: StepUpDialogPrompt(() => null));
    await pumpInApp(tester, _screen(r, unreachable: true));
    final node = tester.getSemantics(find.text('Server unreachable'));
    expect(node, isSemantics(isLiveRegion: true));
    semantics.dispose();
  });
}
