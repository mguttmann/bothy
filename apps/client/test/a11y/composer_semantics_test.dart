// SPDX-License-Identifier: Apache-2.0

// Accessibility of the composer (brief M4-32 Design note 1; 01 8; components.md 9): the field
// is a labelled multi-line text field, Send, attach and voice are labelled buttons, and the
// offline, read-only and too-long states are said in words.

import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/thread/thread_golden_test.dart' as golden;

Widget composers() => Column(
  crossAxisAlignment: CrossAxisAlignment.stretch,
  mainAxisSize: MainAxisSize.min,
  children: [
    Padding(
      padding: const EdgeInsets.all(DsSpace.s12),
      child: DsComposer(
        targetName: 'Release notes',
        maxLines: 6,
        enterSends: true,
        onSend: (_) {},
        onAttach: () {},
        onVoice: () {},
        controller: TextEditingController(text: 'Ready for review'),
      ),
    ),
    golden.states(),
  ],
);

void main() {
  dsAccessibilityTests(
    'the composer',
    build: composers,
    window: const Size(900, 1300),
    semantics: (tester, l) {
      final fields = find.byType(TextField);
      expect(fields, findsWidgets);
      for (final field in fields.evaluate()) {
        final data = tester.getSemantics(find.byWidget(field.widget));
        expect(data.getSemanticsData().flagsCollection.isTextField, isTrue);
      }
      expect(
        find.bySemanticsLabel(RegExp(l.composerTooLong(1000))),
        findsWidgets,
      );
    },
    reachable: (l) => [find.byType(TextField)],
  );
}
