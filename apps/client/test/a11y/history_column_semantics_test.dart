// SPDX-License-Identifier: Apache-2.0

// The history column of the app as screen readers receive it (brief M4-32, a gap found by the
// platform tree test; 01 8, WCAG 4.1.2): the composer's field is a node of its own. Before, the
// field's semantics went into the node of the whole column, which became one text field with
// the header, the messages and the composer's buttons below it; macOS presents a text field as
// a leaf, so VoiceOver reached none of them.

import 'package:design_system/testing.dart';
import 'package:flutter_test/flutter_test.dart';

import '../composer/composer_test.dart' as composer;
import '../support/thread_host.dart' show composerField;

void main() {
  testWidgets('the history column: the composer field is a node of its own', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    final host = await composer.open(tester);
    final problems = dsSemanticsProblems(tester);
    expect(problems, isEmpty, reason: problems.join('\n'));
    final field = tester.getSemantics(composerField);
    expect(field.getSemanticsData().flagsCollection.isTextField, isTrue);
    expect(field.label, startsWith('Message to'));
    await host.dispose(tester);
    handle.dispose();
  });
}
