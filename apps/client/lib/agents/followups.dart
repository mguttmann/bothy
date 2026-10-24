// SPDX-License-Identifier: Apache-2.0

/// Follow-ups during a run (09 7; 01 6.2; 04 15.4; brief M4-21 Design note 2): while a run of
/// the thread is active the composer reads "Follow-up to `agent`". The send path stays the
/// normal one: the server decides that a message is a follow-up, and the message's
/// `trigger_state` shows `followup_delivered` once the adapter has it.
library;

import 'package:domain/domain.dart';

import 'run_mapping.dart';

/// The newest active run of [runs] (states `queued`, `running`, `waiting_input`), or null.
///
/// R-DM-14 allows one active run per thread; should a late event still show two, the newest
/// one by `queued_at` wins (docs/decisions.d/M4-21.md D-M4-21-1).
RunSummary? newestActiveRun(Iterable<RunSummary> runs) {
  RunSummary? newest;
  for (final r in runs) {
    if (!isActiveRun(r.state)) continue;
    if (newest == null || !r.queuedAt.isBefore(newest.queuedAt)) newest = r;
  }
  return newest;
}

/// The agent name of the composer's follow-up form: the agent of the active run of [runs],
/// or null when no run is active. A principal that is not stored shows its ID.
String? followupAgentName(
  Iterable<RunSummary> runs,
  Map<PrincipalId, Principal> principals,
) {
  final run = newestActiveRun(runs);
  if (run == null) return null;
  return principals[run.agentId]?.displayName ?? run.agentId.value;
}
