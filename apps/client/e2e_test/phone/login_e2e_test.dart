// SPDX-License-Identifier: Apache-2.0

// Login on the phone against the real server (brief M5-11 Design note 2, PLAN row M5-11; 01 5.6;
// 08 2.5 and 5.2): E1 with the stack's address, E2 with Alex's password and a TOTP code of a
// later step than the world used, E4 waiting until Alex's other device approves it with the code
// the app shows (the approval needs its step-up), then the phone shell, live. tool/
// mobile_e2e_suite.sh runs it against infra/compose/e2e.sh; it is not part of integration_test/,
// so the platform jobs without a server never pick it up.

import 'package:api_client/api_client.dart' show DeviceStateEnum;
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support/phone_run.dart';

void main() {
  patrolTest(
    'E1, E2 with password and TOTP, E4 approved by the other device, the phone shell',
    platformAutomatorConfig: phoneAutomatorConfig,
    ($) => PhoneRun.run($, (run) async {
      final app = await run.startApp();

      await app.connect();
      // The phone form of onboarding fills the screen.
      expect(find.byKey(const ValueKey('onboarding.phone')), findsOneWidget);
      await run.loginWithPassword();

      await app.waitFor(find.byKey(const ValueKey('screen.e4')));
      expect(app.state.step, OnboardingStep.pending);
      final waiting = app.deviceId;
      run.trackDevice(waiting);
      final shown = find.descendant(
        of: find.byKey(const ValueKey('screen.e4')),
        matching: find.byKey(const ValueKey('code.groups')),
      );
      await app.waitFor(shown);
      // E4 shows the code in groups; the approval takes its digits.
      final code = ($.tester.widget<Text>(shown).data ?? '').replaceAll(
        RegExp(r'\s'),
        '',
      );
      // Compared without printing: the code is the device's one-time secret.
      expect(
        code.isNotEmpty && code == app.state.code,
        isTrue,
        reason: 'E4 shows the verification code of the waiting device',
      );

      final approved = await run.owner.approveDevice(waiting, code);
      expect(approved.state, DeviceStateEnum.active);

      await app.waitFor(find.byKey(const ValueKey('phone.shell')));
      await app.waitLive();
      expect(app.state.step, OnboardingStep.signedIn);
      expect(find.byKey(const ValueKey('phone.tabBar')), findsOneWidget);
      await app.waitFor(
        find.byKey(ValueKey('sidebar.channel.${run.world.generalId}')),
      );
    }),
  );
}
