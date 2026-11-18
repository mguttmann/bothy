// SPDX-License-Identifier: Apache-2.0

/// The trigger state of a triggering message as an icon with a label (01 6.3; components.md
/// 5; 09 6; brief M4-21 Design note 2). The icon, its color, the tooltip and the screen
/// reader label come from `DsTriggerStateIcon`; this file only maps the stored message.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';

import 'run_mapping.dart';

/// The trigger state of [message] with its reason code, or null for a message that triggered
/// nothing.
///
/// `cancelled` stays its own state: a deliberate stop is neutral, never an error (09 6). A
/// state of a newer server maps to `DsTriggerState.unknown` and keeps its code for the label.
///
/// With the thread's [runs], a follow-up shows what became of it (09 section 7,
/// FIX-followup-media-steer): a message sent while a run of the thread was active is a
/// follow-up of that run. One still `received` while a run is active waits for the end of the
/// run (`followupWaiting`); one `cancelled` with `user_cancelled` by the stop of that run was
/// discarded (`followupDiscarded`). A run the local table does not hold leaves the stored
/// state.
///
/// Example: `triggerOf(message)?.state == DsTriggerState.followupDelivered`.
({DsTriggerState state, String? reason, String code})? triggerOf(
  Message message, {
  Iterable<RunSummary> runs = const [],
}) {
  final state = message.triggerState;
  if (state == null) return null;
  var shown = DsTriggerState.fromCode(state.code);
  final at = message.createdAt;
  bool sentDuring(RunSummary r) =>
      r.queuedAt.isBefore(at) &&
      (r.finishedAt == null || !at.isAfter(r.finishedAt!));
  if (state == TriggerState.received &&
      runs.any((r) => isActiveRun(r.state) && r.queuedAt.isBefore(at))) {
    shown = DsTriggerState.followupWaiting;
  } else if (state == TriggerState.cancelled &&
      message.triggerStateReason == 'user_cancelled' &&
      runs.any((r) => r.state == RunState.cancelled && sentDuring(r))) {
    shown = DsTriggerState.followupDiscarded;
  }
  return (state: shown, reason: message.triggerStateReason, code: state.code);
}

/// Whether [state] is shown in the error style: only `failed` (09 6: `cancelled` is a
/// deliberate stop, `done` and `followup_delivered` are success states).
bool triggerIsError(DsTriggerState state) => state == DsTriggerState.failed;
