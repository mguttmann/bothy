// SPDX-License-Identifier: Apache-2.0

// The audit of the semantics tree and its guidelines (brief M4-32 Design note 1): each rule of
// `dsSemanticsProblems` finds the class of gap it is named for, a clean tree has no finding,
// a dialog barrier counts as a role, read-only text is no target and the text of an inactive
// control is not measured for contrast.

import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_test/flutter_test.dart';

import '../components/support.dart';

/// The rules of the findings for [child].
Future<List<String>> rulesOf(WidgetTester tester, Widget child) async {
  await pumpDs(tester, child);
  return [for (final p in dsSemanticsProblems(tester)) p.rule];
}

const Size box = Size(40, 40);

void main() {
  testWidgets('a control without a name is a label finding', (tester) async {
    final handle = tester.ensureSemantics();
    expect(
      await rulesOf(
        tester,
        Semantics(
          button: true,
          onTap: () {},
          child: SizedBox.fromSize(size: box),
        ),
      ),
      ['label'],
    );
    handle.dispose();
  });

  testWidgets('a tappable node without a role is a role finding', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    expect(
      await rulesOf(
        tester,
        Semantics(
          label: 'Open',
          onTap: () {},
          child: SizedBox.fromSize(size: box),
        ),
      ),
      ['role'],
    );
    handle.dispose();
  });

  testWidgets('an enabled button without a tap is an action finding', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    final problems = await rulesOf(
      tester,
      Semantics(
        container: true,
        button: true,
        enabled: true,
        label: 'Allow once',
        child: SizedBox.fromSize(size: box),
      ),
    );
    expect(problems, ['action']);
    expect(
      dsSemanticsProblems(tester).single.toString(),
      contains('label "Allow once"'),
    );
    handle.dispose();
  });

  testWidgets('an unlabelled image and an empty leaf are decorative findings', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    expect(
      await rulesOf(
        tester,
        Column(
          children: [
            Semantics(
              container: true,
              image: true,
              child: SizedBox.fromSize(size: box),
            ),
            Semantics(container: true, child: SizedBox.fromSize(size: box)),
          ],
        ),
      ),
      ['decorative', 'decorative'],
    );
    handle.dispose();
  });

  testWidgets('a label that says a part twice is a repeated finding', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    expect(
      await rulesOf(
        tester,
        Semantics(
          container: true,
          label: 'report.pdf, 47 KB\n\u2068report.pdf\u2069',
          child: SizedBox.fromSize(size: box),
        ),
      ),
      ['repeated'],
    );
    handle.dispose();
  });

  testWidgets('a text field with a node below it is a field finding', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    expect(
      await rulesOf(
        tester,
        Semantics(
          container: true,
          textField: true,
          label: 'Time zone',
          child: Semantics(
            container: true,
            button: true,
            label: 'Open the list',
            onTap: () {},
            child: SizedBox.fromSize(size: box),
          ),
        ),
      ),
      ['field'],
    );
    handle.dispose();
  });

  testWidgets('a clean tree, a dismiss barrier and named controls pass', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    expect(
      await rulesOf(
        tester,
        Column(
          children: [
            Semantics(
              container: true,
              label: 'Dismiss',
              onTap: () {},
              onDismiss: () {},
              child: SizedBox.fromSize(size: box),
            ),
            DsButton(label: 'Allow once', onPressed: () {}),
            const Text('Hello'),
          ],
        ),
      ),
      isEmpty,
    );
    await pumpDs(
      tester,
      Padding(
        padding: const EdgeInsets.all(40),
        child: TextButton(onPressed: () {}, child: const Text('Hello')),
      ),
    );
    await dsExpectAccessible(tester);
    handle.dispose();
  });

  testWidgets('read-only text is no target; small buttons are', (tester) async {
    final handle = tester.ensureSemantics();
    final guideline = DsDesktopTargetGuideline();
    await pumpDs(
      tester,
      Semantics(
        container: true,
        textField: true,
        readOnly: true,
        label: 'Transcript',
        onLongPress: () {},
        child: const SizedBox(width: 200, height: 12),
      ),
    );
    expect((await guideline.evaluate(tester)).passed, isTrue);
    await pumpDs(
      tester,
      Padding(
        // Away from the edges: a node at the edge of a scroll view may be cut and is skipped.
        padding: const EdgeInsets.all(40),
        child: Semantics(
          container: true,
          button: true,
          label: 'Tiny',
          onTap: () {},
          child: const SizedBox(width: 10, height: 10),
        ),
      ),
    );
    expect((await guideline.evaluate(tester)).passed, isFalse);
    handle.dispose();
  });

  test('the contrast guideline skips the text of an inactive control', () {
    const guideline = DsTextContrastGuideline();
    SemanticsData data({required bool enabled}) {
      final owner = SemanticsOwner(onSemanticsUpdate: (_) {});
      final node = SemanticsNode.root(owner: owner)
        ..updateWith(
          config: SemanticsConfiguration()
            ..label = 'Inherit'
            ..textDirection = TextDirection.ltr
            ..isButton = true
            ..isEnabled = enabled,
        );
      final result = node.getSemanticsData();
      owner.dispose();
      return result;
    }

    expect(guideline.shouldSkipNode(data(enabled: false)), isTrue);
    expect(guideline.shouldSkipNode(data(enabled: true)), isFalse);
  });
}
