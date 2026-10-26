// SPDX-License-Identifier: Apache-2.0

// `DsMenuItem` (brief M4-32, a gap found by the desktop checklist; 01 8, WCAG 4.1.2): an entry
// of a menu is a button for screen readers, says when it is the chosen one, says when it is
// disabled, and can be pressed through its semantics.

import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

Widget menu(List<Widget> entries) => MenuAnchor(
  menuChildren: entries,
  builder: (context, controller, _) =>
      TextButton(onPressed: controller.open, child: const Text('Open menu')),
);

void main() {
  testWidgets('entries are buttons, the chosen one is selected', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    var chosen = '';
    await pumpDs(
      tester,
      menu([
        DsMenuItem(
          selected: true,
          onPressed: () => chosen = 'all',
          child: const Text('All'),
        ),
        DsMenuItem(
          selected: false,
          onPressed: () => chosen = 'mute',
          child: const Text('Mute'),
        ),
        const DsMenuItem(onPressed: null, child: Text('Export history')),
      ]),
    );
    await tester.tap(find.text('Open menu'));
    await tester.pumpAndSettle();
    expect(dsSemanticsProblems(tester), isEmpty);
    expect(
      tester.getSemantics(find.text('All')),
      isSemantics(
        label: 'All',
        isButton: true,
        hasSelectedState: true,
        isSelected: true,
        hasEnabledState: true,
        isEnabled: true,
        isFocusable: true,
        hasTapAction: true,
        hasFocusAction: true,
      ),
    );
    expect(
      tester.getSemantics(find.text('Export history')),
      isSemantics(
        label: 'Export history',
        isButton: true,
        hasEnabledState: true,
        isEnabled: false,
      ),
    );
    tester.semantics.tap(find.semantics.byLabel('Mute'));
    await tester.pumpAndSettle();
    expect(chosen, 'mute');
    handle.dispose();
  });
}
