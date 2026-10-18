// SPDX-License-Identifier: Apache-2.0

// Devices and sessions (brief M4-25 Tests): the second device logs in and waits; the first
// confirms it in the settings with the code and step-up; then revokes it, and the second
// device ends in E7 with a wiped store; ending the own session logs out (08 5.2, 10; R-SY-14).

import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('confirm, revoke and end the own session', (tester) async {
    final app = await TestApp.start(tester);
    final second = await app.secondApp();
    try {
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();
      app.fake.seed.alex.newDevicesPending = true;
      await second.signIn();
      expect(second.state.step, OnboardingStep.pending);
      final waiting = second.state.services!.auth.session!.deviceId.value;
      final code = second.state.code!;

      await app.openSettings('devices');
      await app.waitFor(find.byKey(ValueKey('device.$waiting.confirm')));
      await tester.tap(find.byKey(ValueKey('device.$waiting.confirm')));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(const ValueKey('confirmDevice.code')),
        code,
      );
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey('confirmDevice.submit')));
      await app.waitFor(find.byKey(const ValueKey('stepUp.dialog')));
      await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
      await eventually(
        'the second device is signed in',
        () => second.state.step == OnboardingStep.signedIn,
      );

      await app.waitFor(find.byKey(ValueKey('device.$waiting.revoke')));
      await tester.tap(find.byKey(ValueKey('device.$waiting.revoke')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('revoke.confirm')));
      await eventually(
        'the second device ends in E7',
        () => second.state.step == OnboardingStep.signedOut,
      );
      expect(await second.instance.wiped(), isTrue);

      final current = find.byKey(const ValueKey('session.current'));
      await app.waitFor(current);
      final tile = find.ancestor(
        of: current,
        matching: find.byWidgetPredicate(
          (w) =>
              w.key is ValueKey<String> &&
              (w.key! as ValueKey<String>).value.startsWith('session.') &&
              !(w.key! as ValueKey<String>).value.endsWith('.end'),
        ),
      );
      final id = (tester.widget(tile.first).key! as ValueKey<String>).value
          .substring('session.'.length);
      await tester.tap(find.byKey(ValueKey('session.$id.end')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('revoke.confirm')));
      await app.waitFor(find.byKey(const ValueKey('screen.e7')));
      expect(await app.fake.wiped(), isTrue);
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await second.stop();
      await app.stop();
    }
  });
}
