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

    // language-check: off
    expect(find.text('Bestätige, dass du es bist'), findsOneWidget);
    expect(
      find.text('Diese Aktion braucht deine Bestätigung.'),
      findsOneWidget,
    );
    expect(find.text('Weiter'), findsOneWidget);
    // language-check: on
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
    expect(await StepUpDialogPrompt(() => null).askPassword('x'), isNull);
  });

  group('the password variant (a device without a step-up key, 08 6.2)', () {
    Finder field(String key) => find.byKey(ValueKey(key));

    bool confirmEnabled(WidgetTester tester) =>
        tester.widget<FilledButton>(field('stepUp.confirm')).onPressed != null;

    testWidgets('asks for password and a 6-digit code and returns both', (
      tester,
    ) async {
      await pumpInApp(tester, const SizedBox(), navigatorKey: navigator);
      final answer = prompt.askPassword('Assign the runner Alex laptop');
      await tester.pumpAndSettle();

      expect(find.text('Confirm it is you'), findsOneWidget);
      expect(find.text('Assign the runner Alex laptop'), findsOneWidget);
      expect(find.textContaining('authenticator app'), findsOneWidget);
      expect(find.textContaining('fingerprint'), findsNothing);
      final password = tester.widget<TextField>(field('stepUp.password'));
      expect(password.obscureText, isTrue);
      expect(password.autofillHints, [AutofillHints.password]);
      expect(tester.widget<TextField>(field('stepUp.code')).autofillHints, [
        AutofillHints.oneTimeCode,
      ]);
      expect(
        find.text(
          'The password or the code was not accepted. Try again with a new code.',
        ),
        findsNothing,
      );

      await tester.enterText(field('stepUp.password'), 'correct horse');
      await tester.enterText(field('stepUp.code'), '12345');
      await tester.pump();
      expect(confirmEnabled(tester), isFalse);
      await tester.enterText(field('stepUp.code'), '123456');
      await tester.pump();
      expect(confirmEnabled(tester), isTrue);
      await tester.tap(field('stepUp.confirm'));
      await tester.pumpAndSettle();
      expect(await answer, (password: 'correct horse', totpCode: '123456'));
    });

    testWidgets('the code field takes digits only, at most six', (
      tester,
    ) async {
      await pumpInApp(tester, const SizedBox(), navigatorKey: navigator);
      unawaited(prompt.askPassword(''));
      await tester.pumpAndSettle();
      await tester.enterText(field('stepUp.code'), '12a 34-5678');
      await tester.pump();
      expect(
        tester.widget<TextField>(field('stepUp.code')).controller!.text,
        '123456',
      );
    });

    testWidgets('a rejected attempt shows the error line', (tester) async {
      await pumpInApp(tester, const SizedBox(), navigatorKey: navigator);
      unawaited(prompt.askPassword('x', rejected: true));
      await tester.pumpAndSettle();
      expect(
        find.text(
          'The password or the code was not accepted. Try again with a new code.',
        ),
        findsOneWidget,
      );
    });

    testWidgets('cancel answers null', (tester) async {
      await pumpInApp(tester, const SizedBox(), navigatorKey: navigator);
      final answer = prompt.askPassword('x');
      await tester.pumpAndSettle();
      await tester.enterText(field('stepUp.password'), 'typed');
      await tester.tap(field('stepUp.cancel'));
      await tester.pumpAndSettle();
      expect(await answer, isNull);
    });

    testWidgets('speaks German in a German app', (tester) async {
      await pumpInApp(
        tester,
        const SizedBox(),
        navigatorKey: navigator,
        locale: const Locale('de'),
      );
      unawaited(prompt.askPassword('', rejected: true));
      await tester.pumpAndSettle();
      // language-check: off
      expect(find.textContaining('Authenticator-App'), findsOneWidget);
      expect(find.text('Passwort'), findsOneWidget);
      expect(find.text('6-stelliger Code'), findsOneWidget);
      expect(find.textContaining('nicht angenommen'), findsOneWidget);
      expect(find.text('Bestätigen'), findsOneWidget);
      // language-check: on
    });
  });
}
