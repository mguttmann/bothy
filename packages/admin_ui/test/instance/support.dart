// SPDX-License-Identifier: Apache-2.0

// Shared setup of the instance tests (brief M6-09 Tests): the fake repository with the step-up
// dialog as prompt, so every save path shows the dialog first (the fake answers like the server
// behind the interceptor: `401 step_up_required` until a step-up is fresh), and helpers that
// confirm the change and the step-up.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

export '../admin/support.dart';

/// A fake repository whose step-up prompt is the dialog over [navigator].
FakeAdminRepository instanceRepository(GlobalKey<NavigatorState> navigator) =>
    FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );

/// Pumps [child] in the app frame with [navigator] as the dialog host.
Future<void> pumpInstance(
  WidgetTester tester,
  Widget child,
  GlobalKey<NavigatorState> navigator, {
  Locale locale = const Locale('en'),
}) => pumpInApp(tester, child, navigatorKey: navigator, locale: locale);

/// Confirms the open confirmation ([ConfirmChange]).
Future<void> confirmChange(WidgetTester tester) async {
  expect(find.byKey(const ValueKey('confirm.dialog')), findsOneWidget);
  await tester.tap(find.byKey(const ValueKey('confirm.submit')));
  await tester.pumpAndSettle();
}

/// Confirms the step-up dialog that must be open with [purpose].
Future<void> confirmStepUp(WidgetTester tester, String purpose) async {
  await tester.pumpAndSettle();
  expect(find.byType(StepUpDialog), findsOneWidget);
  expect(find.text(purpose), findsOneWidget);
  await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
  await tester.pumpAndSettle();
}

/// The text of the widget with [key] (a Text).
String textOf(WidgetTester tester, String key) =>
    tester.widget<Text>(find.byKey(ValueKey(key))).data!;

/// Opens the dropdown with [key] and chooses the item showing [text] in its menu.
Future<void> pickOption(WidgetTester tester, String key, String text) async {
  await tester.tap(find.byKey(ValueKey(key)));
  await tester.pumpAndSettle();
  await tester.tap(find.text(text).last);
  await tester.pumpAndSettle();
}
