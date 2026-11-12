// SPDX-License-Identifier: Apache-2.0

// "Log out" of the Me tab asks once before it deletes the local data and the device keys
// (docs/decisions.d/AUD-client-phone-01.md; 01 5.6 E7, R-SY-14): the dialog names what goes,
// and the changes that still wait to be sent when there are some; "Cancel" and a tap outside
// keep the login, only "Log out" of the dialog logs out.

import 'dart:io';

import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/phone/providers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';

import '../../support/widgets.dart' show en;
import '../support/phone_app.dart';

/// A signed-in controller that counts its logouts.
class _Counting extends ScriptedOnboarding {
  _Counting() : super(const OnboardingState(step: OnboardingStep.signedIn));

  int logouts = 0;

  @override
  Future<void> logout() async => logouts++;
}

void main() {
  late Directory root;
  setUp(() async => root = Directory.systemTemp.createTempSync('phone'));
  tearDown(() => root.deleteSync(recursive: true));

  /// The Me tab with [waiting] outbox entries; returns the controller.
  Future<_Counting> me(WidgetTester tester, {int? waiting}) async {
    final control = _Counting();
    await PhoneApp.start(
      tester,
      root: root,
      onboarding: control,
      overrides: <Override>[
        if (waiting != null)
          outboxWaitingProvider.overrideWith((ref) => Stream.value(waiting)),
      ],
    );
    // A taller window: the Me tab lists more entries than one screen holds.
    phoneWindow(tester, size: const Size(390, 1400));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('phone.tab.me')));
    await tester.pumpAndSettle();
    return control;
  }

  final dialog = find.byKey(const ValueKey('logout.dialog'));

  testWidgets('Log out asks first and names what is deleted; nothing is logged '
      'out yet', (tester) async {
    final control = await me(tester);
    await tester.tap(find.byKey(const ValueKey('shell.logout')));
    await tester.pumpAndSettle();
    expect(dialog, findsOneWidget);
    expect(
      find.descendant(of: dialog, matching: find.text(en.phoneLogoutTitle)),
      findsOneWidget,
    );
    expect(
      find.descendant(of: dialog, matching: find.text(en.phoneLogoutBody)),
      findsOneWidget,
    );
    // Without waiting changes the dialog names none.
    expect(find.byKey(const ValueKey('logout.waiting')), findsNothing);
    expect(control.logouts, 0);
  });

  testWidgets('Cancel keeps the login', (tester) async {
    final control = await me(tester);
    await tester.tap(find.byKey(const ValueKey('shell.logout')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('logout.cancel')));
    await tester.pumpAndSettle();
    expect(dialog, findsNothing);
    expect(control.logouts, 0);
    expect(find.byKey(const ValueKey('phone.me')), findsOneWidget);
  });

  testWidgets('a tap outside the dialog keeps the login', (tester) async {
    final control = await me(tester);
    await tester.tap(find.byKey(const ValueKey('shell.logout')));
    await tester.pumpAndSettle();
    await tester.tapAt(const Offset(8, 8));
    await tester.pumpAndSettle();
    expect(dialog, findsNothing);
    expect(control.logouts, 0);
  });

  testWidgets('Log out of the dialog logs out once', (tester) async {
    final control = await me(tester);
    await tester.tap(find.byKey(const ValueKey('shell.logout')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('logout.confirm')));
    await tester.pumpAndSettle();
    expect(dialog, findsNothing);
    expect(control.logouts, 1);
  });

  for (final (count, text) in [
    (1, en.phoneLogoutWaiting(1)),
    (3, en.phoneLogoutWaiting(3)),
  ]) {
    testWidgets('with $count waiting in the outbox the dialog says that they '
        'are lost', (tester) async {
      final control = await me(tester, waiting: count);
      await tester.tap(find.byKey(const ValueKey('shell.logout')));
      await tester.pumpAndSettle();
      final waiting = find.byKey(const ValueKey('logout.waiting'));
      expect(waiting, findsOneWidget);
      expect(tester.widget<Text>(waiting).data, text);
      expect(text, contains('$count'));
      await tester.tap(find.byKey(const ValueKey('logout.cancel')));
      await tester.pumpAndSettle();
      expect(control.logouts, 0);
    });
  }
}
