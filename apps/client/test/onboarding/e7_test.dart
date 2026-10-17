// SPDX-License-Identifier: Apache-2.0

// E7 Logged out (01 5.6; R-SY-14): the reason text per revoke reason, the deletion sentence,
// no login button for a disabled person.

import 'package:client/app/account_wiper.dart';
import 'package:client/onboarding/e7_signed_out.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/widgets.dart';

void main() {
  testWidgets('every cause shows its reason and the deletion sentence', (
    tester,
  ) async {
    desktopWindow(tester);
    final texts = {
      SignOutCause.loggedOut: en.onboardingReasonLoggedOut,
      SignOutCause.revokedBySelf: en.onboardingReasonRevokedBySelf,
      SignOutCause.revokedByOwner: en.onboardingReasonRevokedByOwner,
      SignOutCause.userDisabled: en.onboardingReasonUserDisabled,
      SignOutCause.inactive: en.onboardingReasonInactive,
      SignOutCause.rejected: en.onboardingReasonRejected,
      SignOutCause.pendingExpired: en.onboardingReasonPendingExpired,
      SignOutCause.sessionRevoked: en.onboardingReasonSessionRevoked,
      SignOutCause.sessionExpired: en.onboardingReasonSessionExpired,
    };
    for (final MapEntry(key: cause, value: reason) in texts.entries) {
      await tester.pumpWidget(
        hostScreen(SignedOutScreen(cause: cause, onLoginAgain: () {})),
      );
      expect(find.text(en.onboardingSignedOutReason(reason)), findsOneWidget);
      expect(find.text(en.onboardingDataDeleted), findsOneWidget);
    }
    expect(texts.keys.toSet(), SignOutCause.values.toSet());
  });

  testWidgets(
    '"Log in again" by button and Enter; a disabled person gets none',
    (tester) async {
      desktopWindow(tester);
      var again = 0;
      await tester.pumpWidget(
        hostScreen(
          SignedOutScreen(
            cause: SignOutCause.revokedBySelf,
            onLoginAgain: () => again++,
          ),
        ),
      );
      await tester.tap(find.byKey(const ValueKey('e7.loginAgain')));
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      expect(again, 2);
      await tester.pumpWidget(
        hostScreen(
          SignedOutScreen(
            cause: SignOutCause.userDisabled,
            onLoginAgain: () => again++,
          ),
        ),
      );
      expect(find.byKey(const ValueKey('e7.loginAgain')), findsNothing);
      expect(find.text(en.onboardingContactOwner), findsOneWidget);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      expect(again, 2);
    },
  );
}
