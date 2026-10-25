// SPDX-License-Identifier: Apache-2.0

/// The generated `Run` and `RunSummary` of `api_client` for the run graph and the control room
/// cards of `bothy_viz` (docs/spec/25-visualization.md 7.1 row `lib/src/run_graph/`: input is
/// the generated type, so the package keeps its dependencies; brief M4-39 Boundaries).
library;

import 'package:api_client/api_client.dart' as rest;
import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';

/// The run document [doc] of the run store as the generated `Run`, or null when it does not
/// decode (a document the generated serializers reject is not drawn).
rest.Run? apiRunOf(RunDocument doc) {
  try {
    return rest.standardSerializers.deserializeWith(
      rest.Run.serializer,
      doc.raw,
    );
  } on Object {
    return null;
  }
}

/// The stored summary [run] as the generated `RunSummary`, with [stallWarnedAt] from the last
/// `listActiveRuns` load (09 5.6: the local table keeps no stall mark); null when a value of a
/// newer server does not decode.
rest.RunSummary? apiSummaryOf(RunSummary run, {DateTime? stallWarnedAt}) {
  String stamp(DateTime t) => t.toUtc().toIso8601String();
  final json = <String, Object?>{
    'id': run.id.value,
    'thread_id': run.threadId.value,
    'channel_id': run.channelId.value,
    'agent_id': run.agentId.value,
    'triggered_by': run.triggeredBy.value,
    'state': run.state.code,
    'state_reason': ?run.stateReason,
    'version': run.version.value,
    'billing_mode_expected': run.billingModeExpected.code,
    'billing_mode_used': ?run.billingModeUsed?.code,
    'status_line': ?run.statusLine,
    'summary': ?run.summary,
    'cost': {
      'amount_micros': run.cost.amountMicros,
      'currency': run.cost.currency,
      'is_estimate': run.cost.isEstimate,
    },
    'step_count': run.stepCount,
    'duration_ms': ?run.durationMs,
    'queued_at': stamp(run.queuedAt),
    'started_at': ?(run.startedAt == null ? null : stamp(run.startedAt!)),
    'finished_at': ?(run.finishedAt == null ? null : stamp(run.finishedAt!)),
    'stall_warned_at': ?(stallWarnedAt == null ? null : stamp(stallWarnedAt)),
  };
  try {
    return rest.standardSerializers.deserializeWith(
      rest.RunSummary.serializer,
      json,
    );
  } on Object {
    return null;
  }
}
