// SPDX-License-Identifier: Apache-2.0

// A surface that gives its own label keeps its tap action for screen readers (brief M4-32, a gap
// found by the desktop checklist; 01 8, WCAG 4.1.2): `DsTappable` with `excludeChildSemantics`
// used to drop the child's semantics and with them the tap action of its `InkWell`, so
// VoiceOver and Narrator read "Allow once, button" on a card or "Stop" in the live box and
// could not press it.

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

/// Presses the node labelled [label] the way a screen reader does: the semantics tap action.
void pressBySemantics(WidgetTester tester, String label) =>
    tester.semantics.tap(find.semantics.byLabel(label));

void main() {
  testWidgets('a labelled button can be pressed by a screen reader', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    var pressed = 0;
    await pumpDs(
      tester,
      DsButton(label: 'Allow once', onPressed: () => pressed++),
    );
    expect(
      tester.getSemantics(find.bySemanticsLabel('Allow once')),
      isSemantics(
        label: 'Allow once',
        isButton: true,
        hasEnabledState: true,
        isEnabled: true,
        isFocusable: true,
        hasTapAction: true,
        hasFocusAction: true,
      ),
    );
    pressBySemantics(tester, 'Allow once');
    await tester.pump();
    expect(pressed, 1);
    handle.dispose();
  });

  testWidgets('a disabled labelled button offers no action', (tester) async {
    final handle = tester.ensureSemantics();
    await pumpDs(tester, const DsButton(label: 'Allow once', onPressed: null));
    expect(
      tester.getSemantics(find.bySemanticsLabel('Allow once')),
      isSemantics(label: 'Allow once', isButton: true, hasEnabledState: true),
    );
    handle.dispose();
  });

  testWidgets('a tappable with child semantics keeps one tap action', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    var pressed = 0;
    await pumpDs(
      tester,
      DsTappable(onTap: () => pressed++, child: const Text('Open')),
    );
    pressBySemantics(tester, 'Open');
    await tester.pump();
    expect(pressed, 1);
    handle.dispose();
  });
}
