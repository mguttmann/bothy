// SPDX-License-Identifier: Apache-2.0

/// The header of the terminal view (screen D11 of docs/spec/01-product.md; docs/spec/24-terminal.md
/// 12; docs/design/components.md 12 "Header"): host, connection, LLM login badge, the start
/// of the fingerprint and the remaining time.
library;

import 'package:api_client/api_client.dart' show Host;
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show fingerprintDisplay, fingerprintShort;
import 'package:flutter/material.dart';
import 'package:terminal_ui/src/controller/session_state.dart';
import 'package:terminal_ui/src/widgets/open_terminal_dialog.dart';

/// The icon of the header, the `terminal-window` of the icon set.
const DsGlyph terminalGlyph = DsGlyph('terminal-window', DsIconWeight.regular);

/// Returns the remaining time until [openedAt] plus [maxDuration] at [now], never negative;
/// null when either is unknown (brief M4-35 rule 9: without `terminal.max_session_duration`
/// the header shows no remaining time).
Duration? remainingTime(
  DateTime? openedAt,
  Duration? maxDuration,
  DateTime now,
) {
  if (openedAt == null || maxDuration == null) return null;
  final left = openedAt.add(maxDuration).difference(now);
  return left.isNegative ? Duration.zero : left;
}

/// Formats [d] as hours and minutes, `11:42`.
String formatRemaining(Duration d) {
  final minutes = d.inMinutes;
  return '${minutes ~/ 60}:${(minutes % 60).toString().padLeft(2, '0')}';
}

/// One row: icon, host name, connection, login badge, fingerprint start and remaining time.
///
/// ```dart
/// TerminalHeader(host: host, state: controller.state, now: clock.now(),
///     maxSessionDuration: const Duration(hours: 12))
/// ```
class TerminalHeader extends StatelessWidget {
  /// Creates the header.
  const TerminalHeader({
    super.key,
    required this.host,
    required this.state,
    required this.now,
    this.maxSessionDuration,
  });

  /// The host of the tab.
  final Host host;

  /// The state of the tab.
  final TerminalTabState state;

  /// The current time, for the remaining time.
  final DateTime now;

  /// `terminal.max_session_duration` as the app passes it; null hides the remaining time.
  final Duration? maxSessionDuration;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final (dot, text) = switch (state.phase) {
      TerminalPhase.connected => (c.successFg, l.terminalHeaderConnected),
      TerminalPhase.preparing ||
      TerminalPhase.connecting => (c.warningFg, l.terminalHeaderConnecting),
      _ => (c.textMuted, l.terminalHeaderOffline),
    };
    final caption = dsTextStyle(DsType.caption)
        .copyWith(color: c.textSecondary);
    final fingerprint = host.keyFingerprint;
    final left = remainingTime(state.openedAt, maxSessionDuration, now);
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s12,
        vertical: DsSpace.s8,
      ),
      child: Wrap(
        spacing: DsSpace.s12,
        runSpacing: DsSpace.s4,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              DsIcon(terminalGlyph, decorative: true),
              const SizedBox(width: DsSpace.s8),
              // Flexible: a long name wraps on a phone instead of running off the screen.
              Flexible(
                child: Text(
                  host.name,
                  style: dsTextStyle(DsType.heading)
                      .copyWith(color: c.textPrimary),
                ),
              ),
            ],
          ),
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              ExcludeSemantics(
                child: Container(
                  width: DsSize.statusDot,
                  height: DsSize.statusDot,
                  decoration: BoxDecoration(color: dot, shape: BoxShape.circle),
                ),
              ),
              const SizedBox(width: DsSpace.s4),
              Flexible(child: Text(text, style: caption)),
            ],
          ),
          llmLoginBadge(context, host),
          if (fingerprint != null)
            Semantics(
              label: l.terminalHeaderFingerprintLabel(
                fingerprintDisplay(fingerprint),
              ),
              excludeSemantics: true,
              child: Text(
                l.terminalHeaderFingerprint(fingerprintShort(fingerprint)),
                style: dsTextStyle(DsType.codeSm)
                    .copyWith(color: c.textSecondary),
              ),
            ),
          if (left != null)
            Text(
              l.terminalHeaderRemaining(formatRemaining(left)),
              style: caption,
            ),
        ],
      ),
    );
  }
}
