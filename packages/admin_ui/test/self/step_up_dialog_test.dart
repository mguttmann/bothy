// SPDX-License-Identifier: Apache-2.0

// The step-up dialog (brief M4-25 Tests): it explains the action in the person's language
// before the system prompt; "Cancel" answers no, which `StepUpService` turns into
// `StepUpCancelled` (M4-09), and the action then ends with that error.

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:auth/auth.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late StepUpDialogPrompt prompt;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    prompt = StepUpDialogPrompt(() => navigator.currentContext);
  });

  testWidgets('the dialog explains the action in English', (tester) async {
    await pumpInApp(tester, const SizedBox(), navigatorKey: navigator);
    final answer = prompt.ask('Revoke the device Alex phone');
    await tester.pumpAndSettle();

    expect(find.text('Confirm it is you'), findsOneWidget);
    expect(find.text('Revoke the device Alex phone'), findsOneWidget);
    expect(
      find.textContaining('fingerprint, face or device password'),
      findsOneWidget,
    );
    await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
    await tester.pumpAndSettle();
    expect(await answer, isTrue);
  });

  testWidgets('the dialog speaks German in a German app', (tester) async {
    await pumpInApp(
      tester,
      const SizedBox(),
      navigatorKey: navigator,
      locale: const Locale('de'),
    );
    unawaited(prompt.ask(''));
    await tester.pumpAndSettle();

    expect(find.text('Bestätige, dass du es bist'), findsOneWidget);
    expect(
      find.text('Diese Aktion braucht deine Bestätigung.'),
      findsOneWidget,
    );
    expect(find.text('Weiter'), findsOneWidget);
  });

  testWidgets('cancel answers no and the action ends with StepUpCancelled', (
    tester,
  ) async {
    await pumpInApp(tester, const SizedBox(), navigatorKey: navigator);
    final repository = FakeSelfRepository(prompt: prompt);
    final revoke = repository.revokeDevice(
      'phone',
      reason: 'lost',
      purpose: 'Revoke the device Alex phone',
    );
    final outcome = expectLater(revoke, throwsA(isA<StepUpCancelled>()));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('stepUp.cancel')));
    await tester.pumpAndSettle();
    await outcome;

    final direct = prompt.ask('Add a new device to your account');
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('stepUp.cancel')));
    await tester.pumpAndSettle();
    expect(await direct, isFalse);
  });

  test('without a window the step-up is cancelled', () async {
    expect(await StepUpDialogPrompt(() => null).ask('x'), isFalse);
  });
}
