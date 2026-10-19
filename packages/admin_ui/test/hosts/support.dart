// SPDX-License-Identifier: Apache-2.0

// Helpers of the host tests (brief M4-37 Tests): a fake repository with a step-up prompt and
// taps that scroll first.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

export '../admin/support.dart';

/// Taps the widget with [key] after scrolling it into view.
Future<void> tapKey(WidgetTester tester, String key) async {
  final finder = find.byKey(ValueKey(key));
  await tester.ensureVisible(finder.first);
  await tester.tap(finder.first);
  await tester.pumpAndSettle();
}

/// Types [text] into the field with [key].
Future<void> typeKey(WidgetTester tester, String key, String text) async {
  final finder = find.byKey(ValueKey(key));
  await tester.ensureVisible(finder.first);
  await tester.enterText(finder.first, text);
  await tester.pumpAndSettle();
}

/// Opens the dropdown with [key] and picks the entry [label].
Future<void> pick(WidgetTester tester, String key, String label) async {
  await tapKey(tester, key);
  await tester.tap(find.text(label).last);
  await tester.pumpAndSettle();
}

/// The text of the [Text] with [key], or of every [Text] below it joined by spaces.
String textOf(WidgetTester tester, String key) {
  final finder = find.byKey(ValueKey(key));
  final widget = tester.widget(finder.first);
  if (widget is Text)
    return widget.data ?? widget.textSpan?.toPlainText() ?? '';
  if (widget is SelectableText) return widget.data ?? '';
  return [
    for (final t in tester.widgetList<Text>(
      find.descendant(of: finder.first, matching: find.byType(Text)),
    ))
      t.data ?? t.textSpan?.toPlainText() ?? '',
  ].join(' ');
}

/// A fake repository whose step-up prompt is the dialog of the package over [navigator].
FakeAdminRepository hostRepository(GlobalKey<NavigatorState> navigator) =>
    FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );

/// Answers the step-up dialog with "Continue".
Future<void> confirmStepUp(WidgetTester tester) async {
  expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
  await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
  await tester.pumpAndSettle();
}
