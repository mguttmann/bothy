// SPDX-License-Identifier: Apache-2.0

// Accessibility of the thread view (brief M4-32 Design note 1; 01 8; components.md 3 to 5):
// the header controls are labelled buttons, messages read author, time and text, an edited
// or deleted message and a failed send say so in words, and agent text keeps the language of
// the app (no own locale in the semantics tree, 01 8 "Language").

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/thread/thread_golden_test.dart' as golden;

/// Every semantics node below the root whose data carries an own locale.
List<SemanticsNode> localeNodes(WidgetTester tester) {
  final found = <SemanticsNode>[];
  void visit(SemanticsNode node) {
    if (node.getSemanticsData().locale != null) found.add(node);
    node.visitChildren((child) {
      visit(child);
      return true;
    });
  }

  for (final view in tester.binding.renderViews) {
    final root = view.owner?.semanticsOwner?.rootSemanticsNode;
    if (root != null) visit(root);
  }
  return found;
}

void main() {
  dsAccessibilityTests(
    'the thread view',
    build: golden.history,
    window: const Size(900, 1100),
    semantics: (tester, l) {
      expect(
        find.bySemanticsLabel(RegExp('Step 1 of the check')),
        findsWidgets,
      );
      // Agent text without a language tag is not switched to another language.
      expect(localeNodes(tester), isEmpty);
    },
    reachable: (l) => [
      find.byKey(const ValueKey('thread.title')),
      find.byType(TextField),
    ],
  );
}
