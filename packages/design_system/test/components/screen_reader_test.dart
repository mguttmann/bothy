// SPDX-License-Identifier: Apache-2.0

// What a screen reader receives from the components (brief M4-32, gaps found by the desktop
// checklist; 01 8, WCAG 4.1.2): every name is read once. The composer field used to read its
// name and then its placeholder with the same words, and a post waiting for the person read
// "Waiting for you" in its label and once more from its badge.

import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  for (final offline in [false, true]) {
    testWidgets('the composer field reads its name once (offline $offline)', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      final l = await en();
      await pumpDs(
        tester,
        DsComposer(
          targetName: 'ideas',
          maxLines: 6,
          enterSends: true,
          offline: offline,
          onSend: (_) {},
        ),
      );
      final problems = dsSemanticsProblems(tester);
      expect(problems, isEmpty, reason: problems.join('\n'));
      expect(
        find.bySemanticsLabel(l.composerMessageTo('ideas')),
        findsOneWidget,
      );
      handle.dispose();
    });
  }

  testWidgets('a post waiting for the person says so once', (tester) async {
    final handle = tester.ensureSemantics();
    await pumpDs(
      tester,
      DsThreadRow(
        const DsThreadRowData(
          id: 't',
          title: 'Deploy question',
          timeLabel: '10:00',
          runState: DsRunState.waitingInput,
          waitingForMe: true,
        ),
        onOpen: () {},
      ),
    );
    final problems = dsSemanticsProblems(tester);
    expect(problems, isEmpty, reason: problems.join('\n'));
    handle.dispose();
  });
}
