// SPDX-License-Identifier: Apache-2.0

// `DsDropdownField` (brief M4-32, a gap found four times by the desktop checklist; 01 8, WCAG
// 4.1.2): the field is a named text field without a node below it (macOS presents a text field
// as a leaf), and the keyboard still reaches the arrow and opens the list with it.

import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  testWidgets('a named text field without a node below it', (tester) async {
    final handle = tester.ensureSemantics();
    String? chosen;
    await pumpDs(
      tester,
      DsDropdownField<String>(
        label: 'Time zone',
        entries: const [
          DropdownMenuEntry(value: 'UTC', label: 'UTC'),
          DropdownMenuEntry(value: 'Europe/Berlin', label: 'Europe/Berlin'),
        ],
        initialSelection: 'UTC',
        onSelected: (value) => chosen = value,
      ),
    );
    final problems = dsSemanticsProblems(tester);
    expect(problems, isEmpty, reason: problems.join('\n'));
    final field = tester.getSemantics(find.byType(EditableText));
    expect(field.label, contains('Time zone'));
    expect(field.hasChildren, isFalse);

    // The list is closed; Tab reaches the arrow, Enter opens the list.
    expect(
      find.widgetWithText(MenuItemButton, 'Europe/Berlin').hitTestable(),
      findsNothing,
    );
    bool onArrow() =>
        FocusManager.instance.primaryFocus?.context
            ?.findAncestorWidgetOfExactType<IconButton>() !=
        null;
    for (var i = 0; i < 3 && !onArrow(); i++) {
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.pump();
    }
    expect(onArrow(), isTrue, reason: 'Tab reaches the arrow');
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    final entry = find
        .widgetWithText(MenuItemButton, 'Europe/Berlin')
        .hitTestable();
    expect(entry, findsOneWidget);
    // Escape closes the list without a choice.
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(entry, findsNothing);
    expect(chosen, isNull);
    handle.dispose();
  });
}
