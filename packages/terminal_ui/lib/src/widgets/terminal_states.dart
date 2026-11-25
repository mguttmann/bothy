// SPDX-License-Identifier: Apache-2.0

/// The banners of the states of a tab (docs/design/components.md 12 "States"): connected
/// shows none, detached an `info` banner with "Reattach", ended its reason with the screen
/// still readable, blocked a `danger` banner without a new attempt.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart' show AppLocalizations, problemText;
import 'package:terminal_ui/src/controller/session_state.dart';

/// The text of a detached tab by its cause (15 23.2 item 8).
String detachedText(AppLocalizations l, DetachCause? cause) => switch (cause) {
  DetachCause.person || null => l.terminalStateDetached,
  DetachCause.connectionLost => l.terminalStateDetachedLost,
  DetachCause.hostUnavailable => l.terminalStateDetachedHostUnavailable,
  DetachCause.tokenStale => l.terminalStateDetachedTokenStale,
  DetachCause.replaced => l.terminalStateDetachedReplaced,
  DetachCause.serverRestart => l.terminalStateDetachedServer,
  DetachCause.slowConsumer => l.terminalStateDetachedSlow,
  DetachCause.authenticationFailed => l.terminalStateDetachedAuth,
  DetachCause.rateLimited => l.terminalStateDetachedRateLimited,
};

/// The text of the problem [code] of a tab: the hint of a device without a step-up key for
/// [stepUpUnavailableCode], the problem text of `l10n` otherwise.
String terminalProblemText(AppLocalizations l, String code, {String? reason}) =>
    code == stepUpUnavailableCode
    ? l.terminalStepUpUnavailable
    : problemText(l, code, reason: reason);

/// The banner of [state], or nothing while connected or idle.
///
/// ```dart
/// TerminalStateBanner(state: controller.state, onReattach: controller.attach)
/// ```
class TerminalStateBanner extends StatelessWidget {
  /// Creates the banner.
  const TerminalStateBanner({super.key, required this.state, this.onReattach});

  /// The state of the tab.
  final TerminalTabState state;

  /// Attaches again; offered only for a detached tab.
  final VoidCallback? onReattach;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final banner = switch (state.phase) {
      TerminalPhase.idle || TerminalPhase.connected => null,
      TerminalPhase.preparing || TerminalPhase.connecting => DsBanner(
        tone: DsTone.info,
        text: l.terminalStateConnecting,
      ),
      TerminalPhase.detached => DsBanner(
        tone: DsTone.info,
        text: [
          detachedText(l, state.detachCause),
          if (state.problemCode case final code?)
            terminalProblemText(l, code, reason: state.problemReason),
        ].join(' '),
        actionLabel: onReattach == null ? null : l.terminalReattach,
        onAction: onReattach,
      ),
      TerminalPhase.exited => DsBanner(
        tone: DsTone.info,
        text: l.terminalStateExited(state.exitCode ?? 0),
      ),
      TerminalPhase.ended => DsBanner(
        tone: DsTone.info,
        text: l.terminalStateEnded(
          l.terminalCloseReason(state.closeReason ?? 'other'),
        ),
      ),
      TerminalPhase.blocked => DsBanner(
        tone: DsTone.danger,
        text: switch (state.blockCause) {
          BlockCause.revoked => l.terminalStateBlockedRevoked(
            l.terminalCloseReason(state.closeReason ?? 'other'),
          ),
          BlockCause.keyMismatch => l.terminalStateBlockedKeyMismatch,
          BlockCause.protocolError => l.terminalStateBlockedProtocol,
          BlockCause.handshakeFailed || null => l.terminalStateBlockedHandshake,
        },
      ),
      TerminalPhase.failed when state.problemCode == stepUpUnavailableCode =>
        DsBanner(tone: DsTone.warning, text: l.terminalStepUpUnavailable),
      TerminalPhase.failed => DsBanner(
        tone: DsTone.danger,
        text: l.terminalStateFailed(
          problemText(
            l,
            state.problemCode ?? 'open_failed',
            reason: state.problemReason,
          ),
        ),
      ),
    };
    return banner ?? const SizedBox.shrink();
  }
}
