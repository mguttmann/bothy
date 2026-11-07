// SPDX-License-Identifier: Apache-2.0

/// What the thread shows of its wake-ups (01 6.21, 09 5.8; brief AR-08): the name, the condition
/// and the times of a pending wake-up, who may cancel it (07 2.14 item 5), the wake-up a wake
/// run came from, and the usage-limit wake-up of a run. Pure functions over the local table,
/// so the desktop and the phone read them the same way.
///
/// Every view is driven by the records the server keeps: a wake-up exists only when the agent
/// reported `wakes` when it asked for it, so a thread of an agent without `wakes` shows none
/// of it, and a pending wake-up of a runner that turned `runner.wakes` off stays visible with
/// its Cancel (01 6.21; ruling D-AR-08-1).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart' show AccessView;

/// The name of [wake]: its label isolated as agent content, or "Wake-up" without one.
String wakeName(AppLocalizations l, RunWake wake) =>
    wake.label.isEmpty ? l.wakesUnnamed : dsIsolate(wake.label);

/// The condition of [wake] as 01 6.21 item 1 words it: "at 17:55", "when the checks of #123
/// finish" or "when the command succeeds"; [time] formats a time in the person's zone.
String wakeCondition(
  AppLocalizations l,
  RunWake wake,
  String Function(DateTime at) time,
) => switch (wake.kind) {
  RunWakeKind.at => l.wakesConditionAt(time(wake.notBefore)),
  RunWakeKind.prChecks => switch (wake.condition['number']) {
    final int number => l.wakesConditionPrChecks('$number'),
    _ => l.wakesConditionPrChecksAny,
  },
  RunWakeKind.command => l.wakesConditionCommand,
  RunWakeKind.unknown => l.wakesConditionUnknown,
};

/// Whether [me] may cancel [wake] (07 2.14 item 5, `run.wake.cancel`): the wake-up's person,
/// or a person with `manage` on the channel ([access]); the server checks again.
bool mayCancelWake(RunWake wake, PrincipalId? me, AccessView? access) =>
    (me != null && wake.triggeredBy == me) ||
    (access?.capabilities.contains(Capability.manage) ?? false);

/// The pending wake-ups of [wakes], in the order they fire.
List<RunWake> pendingWakes(List<RunWake> wakes) => [
  for (final w in wakes)
    if (w.isPending) w,
];

/// The wake-up that started the wake run [run] (`fired_run_id`), or null for a run of a
/// message, a command or an interaction.
RunWake? wakeOfRun(List<RunWake> wakes, RunId run) {
  for (final w in wakes) {
    if (w.firedRunId == run) return w;
  }
  return null;
}

/// The newest usage-limit wake-up whose source is [run] (09 18.19.10), or null.
RunWake? usageWakeOf(List<RunWake> wakes, RunId run) {
  RunWake? found;
  for (final w in wakes) {
    if (w.reason != RunWakeReason.usageLimit || w.sourceRunId != run) continue;
    if (found == null || w.createdAt.isAfter(found.createdAt)) found = w;
  }
  return found;
}

/// The state reason of a run that the usage window of the account ended (09 18.19.10).
const String usageLimitedReason = 'usage_limited';
