// SPDX-License-Identifier: Apache-2.0

/// E4 Device waiting for confirmation (docs/spec/01-produkt.md 5.6; 08 5.2).
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'code_text.dart';
import 'onboarding_card.dart';

/// The verification code large in the center, the instructions, the remaining time as text
/// and "Cancel", which logs out and deletes the keys of the device. The app polls meanwhile
/// (the controller); the remaining time is `created_at` plus `auth.device_pending_ttl`, for
/// display only (D-W-116).
class PendingDeviceScreen extends StatelessWidget {
  /// Creates the screen.
  const PendingDeviceScreen({
    super.key,
    required this.code,
    required this.deadline,
    required this.clock,
    required this.announceInterval,
    required this.onCancel,
    this.announce,
  });

  /// The verification code of this device.
  final String code;

  /// When the waiting ends.
  final DateTime deadline;

  /// The current time.
  final DateTime Function() clock;

  /// `a11y.live_announce_min_interval`.
  final Duration announceInterval;

  /// "Cancel".
  final VoidCallback onCancel;

  /// Receives the announcements of the remaining time (tests).
  final void Function(String message)? announce;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return OnboardingCard(
      children: [
        CardTitle(l.onboardingPendingTitle),
        cardGap,
        CodeText(code, groupSize: 3),
        cardGap,
        CardText(l.onboardingPendingExplain),
        cardGap,
        RemainingTimeText(
          deadline: deadline,
          clock: clock,
          announceInterval: announceInterval,
          announce: announce,
        ),
        cardGap,
        OutlinedButton(
          key: const ValueKey('e4.cancel'),
          onPressed: onCancel,
          child: Text(l.onboardingCancel),
        ),
      ],
    );
  }
}
