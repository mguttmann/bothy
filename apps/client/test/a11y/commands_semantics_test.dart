// SPDX-License-Identifier: Apache-2.0

// Accessibility of the commands (brief M4-32 Design note 1; 01 6.4 and 8; components.md 9 and
// 10): the slash suggestions are labelled options with their description, the follow-up form
// of the composer names the agent, and the agent part of the session column says presence,
// billing mode and costs in words.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/agents/agents_golden_test.dart' as golden;

void main() {
  dsAccessibilityTests(
    'the commands and the session agent part',
    build: golden.commandsAndSession,
    window: const Size(1280, 1600),
    semantics: (tester, l) {
      // Options of a list: the description and the owner are read with the command.
      expect(
        tester.getSemantics(
          find.bySemanticsLabel('/stats, About stats, Robin'),
        ),
        isSemantics(hasSelectedState: true),
      );
      expect(
        find.bySemanticsLabel(l.composerFollowupTo('Robin')),
        findsOneWidget,
      );
    },
    reachable: (l) => [find.byType(TextField)],
  );
}
