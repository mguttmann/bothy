// SPDX-License-Identifier: Apache-2.0

/// The trigger state of a triggering message as an icon with a label (01 6.3; components.md
/// 5; 09 6; brief M4-21 Design note 2). The icon, its color, the tooltip and the screen
/// reader label come from `DsTriggerStateIcon`; this file only maps the stored message.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';

/// The trigger state of [message] with its reason code, or null for a message that triggered
/// nothing.
///
/// `cancelled` stays its own state: a deliberate stop is neutral, never an error (09 6). A
/// state of a newer server maps to `DsTriggerState.unknown` and keeps its code for the label.
///
/// Example: `triggerOf(message)?.state == DsTriggerState.followupDelivered`.
({DsTriggerState state, String? reason, String code})? triggerOf(
  Message message,
) {
  final state = message.triggerState;
  if (state == null) return null;
  return (
    state: DsTriggerState.fromCode(state.code),
    reason: message.triggerStateReason,
    code: state.code,
  );
}

/// Whether [state] is shown in the error style: only `failed` (09 6: `cancelled` is a
/// deliberate stop, `done` and `followup_delivered` are success states).
bool triggerIsError(DsTriggerState state) => state == DsTriggerState.failed;
