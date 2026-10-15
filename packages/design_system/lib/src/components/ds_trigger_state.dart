// SPDX-License-Identifier: Apache-2.0

/// The trigger state on a triggering message (components.md section 5).
library;

import 'package:flutter/material.dart';

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_common.dart';
import 'view_models.dart';

/// The icon of a `messages.trigger_state` with its caption in the tooltip and the semantics
/// label (components.md section 5; replaces the status emoji).
///
/// Size `icon.size.sm`; the icon shape differs per state, so the state never shows by color
/// alone. For `failed` and `cancelled` the translated [reason] follows the word ("Failed:
/// Project not ready"); a reason this app does not translate shows its code. [onTap] jumps to
/// the live box. An unknown state shows the info icon with [code].
///
/// ```dart
/// DsTriggerStateIcon(state: DsTriggerState.failed, reason: 'project_not_ready')
/// ```
class DsTriggerStateIcon extends StatelessWidget {
  /// Creates the icon of [state].
  const DsTriggerStateIcon({
    super.key,
    required this.state,
    this.reason,
    this.reasonName,
    this.code,
    this.onTap,
  });

  /// The trigger state.
  final DsTriggerState state;

  /// `trigger_state_reason`, a code, or null.
  final String? reason;

  /// The name for the reason `user_cancelled`.
  final String? reasonName;

  /// The raw state code of an unknown state.
  final String? code;

  /// Jumps to the live box, or null.
  final VoidCallback? onTap;

  /// The caption of the state for [context]'s locale.
  String caption(BuildContext context) {
    final l = dsL10n(context);
    final why = reason == null
        ? null
        : dsReasonText(l, reason, name: reasonName) ?? reason;
    return switch (state) {
      DsTriggerState.received => l.triggerStateReceived,
      DsTriggerState.queued => l.triggerStateQueued,
      DsTriggerState.working => l.triggerStateWorking,
      DsTriggerState.done => l.triggerStateDone,
      DsTriggerState.failed =>
        why == null ? l.triggerStateFailed : l.triggerStateFailedWithReason(why),
      DsTriggerState.cancelled =>
        why == null
            ? l.triggerStateCancelled
            : l.triggerStateCancelledWithReason(why),
      DsTriggerState.followupDelivered => l.triggerStateFollowupDelivered,
      DsTriggerState.unknown => l.triggerStateUnknown(code ?? ''),
    };
  }

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    final (glyph, color) = switch (state) {
      DsTriggerState.received => (DsIcons.triggerReceived, colors.triggerReceived),
      DsTriggerState.queued => (DsIcons.triggerQueued, colors.triggerQueued),
      DsTriggerState.working => (DsIcons.triggerWorking, colors.triggerWorking),
      DsTriggerState.done => (DsIcons.triggerDone, colors.triggerDone),
      DsTriggerState.failed => (DsIcons.triggerFailed, colors.triggerFailed),
      DsTriggerState.cancelled => (DsIcons.runCancelled, colors.runCancelled),
      DsTriggerState.followupDelivered => (
        DsIcons.triggerFollowup,
        colors.triggerFollowup,
      ),
      DsTriggerState.unknown => (DsIcons.stateInfo, colors.textSecondary),
    };
    final text = caption(context);
    return Tooltip(
      message: text,
      excludeFromSemantics: true,
      child: DsTappable(
        onTap: onTap,
        label: text,
        button: onTap != null,
        excludeChildSemantics: true,
        radius: DsRadius.pill,
        child: Center(
          widthFactor: 1,
          heightFactor: 1,
          child: DsIcon(glyph, size: DsIconSizeRole.sm, color: color, decorative: true),
        ),
      ),
    );
  }
}
