// SPDX-License-Identifier: Apache-2.0

// E3 Redeem enrollment code and E6 App lock (01 5.6).

import 'package:client/onboarding/e3_enrollment_code.dart';
import 'package:client/onboarding/e6_app_lock.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/instances.dart';
import '../support/widgets.dart';

void main() {
  testWidgets('E3 offers this device, a passkey with a local driver and each login service', (
    tester,
  ) async {
    desktopWindow(tester);
    final sent = <EnrollMethod>[];
    var back = 0;
    Future<void> pump(bool passkeys, {bool busy = false, LoginNotice? notice}) =>
        tester.pumpWidget(
          hostScreen(
            EnrollmentCodeScreen(
              instance: oidcOnly,
              passkeysAvailable: passkeys,
              busy: busy,
              notice: notice,
              onRedeem: (_, m) => sent.add(m),
              onBack: () => back++,
            ),
          ),
        );
    await pump(true);
    expect(find.byKey(const ValueKey('e3.method.device')), findsOneWidget);
    expect(find.byKey(const ValueKey('e3.method.passkey')), findsNothing, reason: 'no local driver');
    expect(find.text(en.onboardingEnrollLink('Company SSO')), findsOneWidget);
    expect(find.text(en.onboardingEnrollLink('Office proxy')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('e3.continue')));
    expect(sent, isEmpty, reason: 'no code yet');
    await tester.enterText(find.byKey(const ValueKey('e3.code')), 'en_ABCD');
    await tester.tap(find.text(en.onboardingEnrollLink('Office proxy')));
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect((sent.single as EnrollWithProvider).provider.name, 'Office proxy');
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.tap(find.byKey(const ValueKey('e3.back')));
    expect(back, 2);
    await pump(true, busy: true, notice: LoginNotice.failed);
    expect(find.text(en.onboardingLoginFailed), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('e3.continue')));
    expect(sent, hasLength(1));
    await tester.pumpWidget(
      hostScreen(
        EnrollmentCodeScreen(
          instance: localWithPassword,
          passkeysAvailable: true,
          onRedeem: (_, m) => sent.add(m),
          onBack: () {},
        ),
      ),
    );
    expect(find.text(en.onboardingEnrollCreatePasskey), findsOneWidget);
    await tester.enterText(find.byKey(const ValueKey('e3.code')), 'en_ABCD');
    await tester.tap(find.text(en.onboardingEnrollCreatePasskey));
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('e3.continue')));
    expect(sent.last, isA<EnrollWithPasskey>());
  });

  testWidgets('E6 shows unlock, the failure with "Try again" and "Log out"', (tester) async {
    desktopWindow(tester);
    final calls = <String>[];
    Future<void> pump({bool failed = false}) => tester.pumpWidget(
      hostScreen(
        AppLockScreen(
          productName: 'Team Chat',
          failed: failed,
          onUnlock: () => calls.add('unlock'),
          onLogout: () => calls.add('logout'),
        ),
      ),
    );
    await pump();
    expect(find.text(en.onboardingLockedTitle('Team Chat')), findsOneWidget);
    expect(find.text(en.onboardingUnlock), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await pump(failed: true);
    expect(find.text(en.onboardingUnlockFailed), findsOneWidget);
    await tester.tap(find.text(en.onboardingTryAgain));
    await tester.tap(find.byKey(const ValueKey('e6.logout')));
    expect(calls, ['unlock', 'unlock', 'logout']);
  });
}
