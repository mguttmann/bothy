// SPDX-License-Identifier: Apache-2.0

/// E5 Pair with another device (docs/spec/01-produkt.md 5.6; 08 5.3).
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'code_text.dart';
import 'notice_text.dart';
import 'onboarding_card.dart';
import 'onboarding_controller.dart';

/// The pairing code with its remaining time until `expires_at` of `beginPairing` and "New
/// code"; after the approval on an active device the new device is active (the controller
/// polls). An expired pairing shows its text instead of the code. Esc goes back to E2.
class PairingScreen extends StatelessWidget {
  /// Creates the screen.
  const PairingScreen({
    super.key,
    required this.clock,
    required this.announceInterval,
    required this.onNewCode,
    required this.onBack,
    this.code,
    this.deadline,
    this.expired = false,
    this.notice,
    this.busy = false,
    this.announce,
  });

  /// The pairing code, or null while it is requested.
  final String? code;

  /// `expires_at` of the pairing.
  final DateTime? deadline;

  /// Whether the pairing expired.
  final bool expired;

  /// The error of `beginPairing`.
  final LoginNotice? notice;

  /// Whether a request runs.
  final bool busy;

  /// The current time.
  final DateTime Function() clock;

  /// `a11y.live_announce_min_interval`.
  final Duration announceInterval;

  /// "New code".
  final VoidCallback onNewCode;

  /// Back to E2.
  final VoidCallback onBack;

  /// Receives the announcements of the remaining time (tests).
  final void Function(String message)? announce;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final shown = code;
    final until = deadline;
    return OnboardingCard(
      onEnter: busy ? null : onNewCode,
      onEscape: onBack,
      children: [
        Align(
          alignment: Alignment.centerLeft,
          child: TextButton(
            key: const ValueKey('e5.back'),
            onPressed: onBack,
            child: Text(l.onboardingBack),
          ),
        ),
        CardTitle(l.onboardingPairTitle),
        cardGap,
        if (notice != null) ...[NoticeBanner(notice!), cardGap],
        if (expired)
          CardText(
            l.onboardingPairExpired,
            key: const ValueKey('e5.expired'),
            center: true,
          )
        else if (shown != null && until != null) ...[
          CardText(l.onboardingPairExplain),
          cardGap,
          CodeText(shown),
          cardGap,
          RemainingTimeText(
            deadline: until,
            clock: clock,
            announceInterval: announceInterval,
            announce: announce,
          ),
        ],
        cardGap,
        FilledButton(
          key: const ValueKey('e5.newCode'),
          onPressed: busy ? null : onNewCode,
          child: Text(l.onboardingPairNewCode),
        ),
      ],
    );
  }
}
