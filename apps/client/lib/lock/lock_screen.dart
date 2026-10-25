// SPDX-License-Identifier: Apache-2.0

/// The screens of the closed app lock: E6 wired to the [AppLockController], and the notice of
/// a device without a device credential (docs/spec/01-product.md 5.6 E6; brief M5-08 Design
/// notes 3 and 4).
library;

import 'dart:async';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../app/router.dart' show OnboardingStep;
import '../onboarding/e6_app_lock.dart';
import '../onboarding/onboarding_card.dart';
import 'app_lock_controller.dart';

/// E6 while the lock is [AppLockStatus.locked], the notice while it is
/// [AppLockStatus.unavailable]; rebuilds with [lock].
class AppLockGate extends StatelessWidget {
  /// Creates the gate.
  const AppLockGate({
    super.key,
    required this.lock,
    required this.productName,
    required this.onLogout,
    this.logo,
  });

  /// The lock.
  final AppLockController lock;

  /// `branding.product_name`.
  final String productName;

  /// The logo bytes, or null.
  final Uint8List? logo;

  /// "Log out": ends the session, deletes the local data and shows E7.
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: lock,
    builder: (context, _) {
      final l = AppLocalizations.of(context);
      if (lock.status == AppLockStatus.unavailable) {
        return AppLockUnavailableScreen(
          key: const ValueKey('screen.e6.unavailable'),
          productName: productName,
          logo: logo,
          onCheckAgain: () => unawaited(lock.check()),
          onLogout: onLogout,
        );
      }
      return AppLockScreen(
        key: const ValueKey('screen.e6'),
        productName: productName,
        logo: logo,
        failed: lock.failed,
        onUnlock: () => unawaited(lock.unlock(l.appLockReason(productName))),
        onLogout: onLogout,
      );
    },
  );
}

/// The notice when the lock applies and the device has no device credential: the way to the
/// system settings, "Check again" and "Log out", never content (fixed requirement 14).
class AppLockUnavailableScreen extends StatelessWidget {
  /// Creates the screen.
  const AppLockUnavailableScreen({
    super.key,
    required this.productName,
    required this.onCheckAgain,
    required this.onLogout,
    this.logo,
  });

  /// `branding.product_name`.
  final String productName;

  /// The logo bytes, or null.
  final Uint8List? logo;

  /// Checks the device again (after a credential was set in the system settings).
  final VoidCallback onCheckAgain;

  /// Logs out and deletes the local data.
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return OnboardingCard(
      onEnter: onCheckAgain,
      children: [
        BrandMark(logo: logo),
        cardGap,
        CardTitle(l.appLockNoCredentialTitle(productName)),
        cardGap,
        CardText(l.appLockNoCredentialBody, center: true),
        cardGap,
        FilledButton(
          key: const ValueKey('e6.checkAgain'),
          onPressed: onCheckAgain,
          child: Text(l.appLockCheckAgain),
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

/// Whether the content stays hidden behind the privacy cover: a lock that is still deciding at
/// a cold start ([AppLockStatus.checking]) or on a return from the background
/// ([AppLockController.deciding]) shows no content yet (Design note 2; D-M5-08-8).
bool lockHidesContent(AppLockController? lock) =>
    lock != null && (lock.status == AppLockStatus.checking || lock.deciding);

/// The screen the router shows for the onboarding [step] while [lock] holds: signed in with a
/// closed lock, every route resolves to E6 (`redirectFor` keeps the requested path, a deep
/// link or a tapped notification among them, and opens it after the unlock).
///
/// Example: `routeStep(OnboardingStep.signedIn, lockedController)` is
/// [OnboardingStep.locked].
OnboardingStep routeStep(OnboardingStep step, AppLockController? lock) =>
    step == OnboardingStep.signedIn && lock != null && lock.closed
    ? OnboardingStep.locked
    : step;
