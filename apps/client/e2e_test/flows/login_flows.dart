// SPDX-License-Identifier: Apache-2.0

// Login flows against the real server (brief M4-34 product requirement 13 and Design note 2): the
// enrollment login with device keys and DPoP, and a waiting device of Alex that the first device
// confirms in the settings with its verification code and a step-up (08 5.2).

import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/e2e_app.dart';
import '../support/e2e_context.dart';

/// Registers the login flows.
void loginFlows(E2eContext c) {
  testWidgets('enrollment login with device keys and DPoP', (tester) async {
    final app = await E2eApp.start(tester, c.world);
    try {
      await app.connect();
      await app.enroll(await c.alexCode());
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();
      await app.waitFor(
        find.byKey(ValueKey('sidebar.channel.${c.world.generalId}')),
      );
      await app.waitFor(
        find.byKey(ValueKey('sidebar.channel.${c.world.forumId}')),
      );
      expect(app.device.transport.opened, greaterThan(0));
    } finally {
      await app.stop();
    }
  });

  testWidgets('a waiting device confirmed from the first device', (
    tester,
  ) async {
    final app = await E2eApp.start(tester, c.world);
    final second = await E2eSecondApp.start(c.world);
    try {
      await app.signIn(await c.alexCode());
      await tester.runAsync(second.loginWithPassword);
      expect(second.state.step, OnboardingStep.pending);
      final waiting = second.state.services!.auth.session!.deviceId.value;
      final code = second.state.code!;

      await app.openSettings('devices');
      await app.tap('device.$waiting.confirm');
      await app.waitFor(find.byKey(const ValueKey('confirmDevice.code')));
      await tester.enterText(
        find.byKey(const ValueKey('confirmDevice.code')),
        code,
      );
      await tester.pump();
      await app.tap('confirmDevice.submit');
      await app.waitFor(find.byKey(const ValueKey('stepUp.dialog')));
      await app.tap('stepUp.continue');
      await app.until(
        'the second device signed in',
        () => second.state.step == OnboardingStep.signedIn,
      );
    } finally {
      await tester.runAsync(second.stop);
      await app.stop();
    }
  });
}
