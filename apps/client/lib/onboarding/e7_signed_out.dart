// SPDX-License-Identifier: Apache-2.0

/// E7 Logged out (docs/spec/01-produkt.md 5.6; 04 8.5, R-SY-14).
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../app/account_wiper.dart';
import 'onboarding_card.dart';

/// The reason as text (never an internal name), "The data of this device has been deleted"
/// and "Log in again"; a disabled person gets "Contact the owner" instead of the button.
class SignedOutScreen extends StatelessWidget {
  /// Creates the screen.
  const SignedOutScreen({
    super.key,
    required this.cause,
    required this.onLoginAgain,
    this.busy = false,
  });

  /// Why the account ended.
  final SignOutCause cause;

  /// "Log in again" (E2).
  final VoidCallback onLoginAgain;

  /// Whether the services are composed again.
  final bool busy;

  /// The reason text of [cause].
  static String reasonText(AppLocalizations l, SignOutCause cause) =>
      switch (cause) {
        SignOutCause.loggedOut => l.onboardingReasonLoggedOut,
        SignOutCause.revokedBySelf => l.onboardingReasonRevokedBySelf,
        SignOutCause.revokedByOwner => l.onboardingReasonRevokedByOwner,
        SignOutCause.userDisabled => l.onboardingReasonUserDisabled,
        SignOutCause.inactive => l.onboardingReasonInactive,
        SignOutCause.rejected => l.onboardingReasonRejected,
        SignOutCause.pendingExpired => l.onboardingReasonPendingExpired,
        SignOutCause.sessionRevoked => l.onboardingReasonSessionRevoked,
        SignOutCause.sessionExpired => l.onboardingReasonSessionExpired,
      };

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final disabled = cause == SignOutCause.userDisabled;
    return OnboardingCard(
      onEnter: disabled || busy ? null : onLoginAgain,
      children: [
        CardTitle(l.onboardingSignedOutTitle),
        cardGap,
        CardText(
          l.onboardingSignedOutReason(reasonText(l, cause)),
          key: const ValueKey('e7.reason'),
        ),
        cardGap,
        CardText(l.onboardingDataDeleted, key: const ValueKey('e7.deleted')),
        cardGap,
        if (disabled)
          CardText(
            l.onboardingContactOwner,
            key: const ValueKey('e7.contactOwner'),
          )
        else
          FilledButton(
            key: const ValueKey('e7.loginAgain'),
            onPressed: busy ? null : onLoginAgain,
            child: Text(l.onboardingLogInAgain),
          ),
      ],
    );
  }
}
