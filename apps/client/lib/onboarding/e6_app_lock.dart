// SPDX-License-Identifier: Apache-2.0

/// E6 App lock, the screen only (docs/spec/01-product.md 5.6; 08 9). The lock logic, its timer
/// and the biometric check are M5-08.
library;

import 'dart:typed_data';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'onboarding_card.dart';

/// The lock: the name and logo of the instance, "Unlock" (biometrics or the device password
/// of the system), after a failure "Unlocking failed." with "Try again", and "Log out", which
/// leads to E7 and deletes the local data. The content stays hidden.
class AppLockScreen extends StatelessWidget {
  /// Creates the screen.
  const AppLockScreen({
    super.key,
    required this.productName,
    required this.onUnlock,
    required this.onLogout,
    this.logo,
    this.failed = false,
  });

  /// `branding.product_name`.
  final String productName;

  /// The logo bytes, or null.
  final Uint8List? logo;

  /// Whether the last unlock failed.
  final bool failed;

  /// Asks the system to unlock.
  final VoidCallback onUnlock;

  /// Logs out and deletes the local data.
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return OnboardingCard(
      onEnter: onUnlock,
      children: [
        BrandMark(logo: logo),
        cardGap,
        CardTitle(l.onboardingLockedTitle(productName)),
        cardGap,
        if (failed) ...[
          DsBanner(
            key: const ValueKey('e6.failed'),
            tone: DsTone.danger,
            text: l.onboardingUnlockFailed,
            liveRegion: true,
          ),
          cardGap,
        ],
        FilledButton(
          key: const ValueKey('e6.unlock'),
          onPressed: onUnlock,
          child: Text(failed ? l.onboardingTryAgain : l.onboardingUnlock),
        ),
        controlGap,
        TextButton(
          key: const ValueKey('e6.logout'),
          onPressed: onLogout,
          child: Text(l.onboardingLogOut),
        ),
      ],
    );
  }
}
