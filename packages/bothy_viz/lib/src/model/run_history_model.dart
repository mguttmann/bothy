// SPDX-License-Identifier: Apache-2.0

/// The view model of the run history (docs/spec/25-visualization.md 6.7).
library;

import 'package:api_client/api_client.dart' as api;

import 'costs.dart';
import 'names.dart';
import 'run_label.dart';

/// The token counts of a finished run; a count the runtime never reported is `null`, never
/// zero.
final class RunTokens {
  /// Creates the counts.
  const RunTokens({this.input, this.output, this.cacheRead, this.cacheWrite});

  /// The counts of [metrics]; [cacheWrite] adds the writes of five minutes and of one hour,
  /// `null` when neither was reported.
  factory RunTokens.of(api.RunMetrics metrics) {
    final five = metrics.tokensCacheWrite;
    final hour = metrics.tokensCacheWrite1h;
    return RunTokens(
      input: metrics.tokensIn,
      output: metrics.tokensOut,
      cacheRead: metrics.tokensCacheRead,
      cacheWrite: five == null && hour == null
          ? null
          : (five ?? 0) + (hour ?? 0),
    );
  }

  /// Input tokens.
  final int? input;

  /// Output tokens.
  final int? output;

  /// Tokens read from the prompt cache.
  final int? cacheRead;

  /// Tokens written to the prompt cache.
  final int? cacheWrite;
}

/// One finished run of the history.
final class RunHistoryRow {
  /// Creates a row; [RunHistoryRow.of] is the normal way.
  const RunHistoryRow({
    required this.runId,
    required this.threadId,
    required this.label,
    required this.state,
    required this.queuedAt,
    required this.finishedAt,
    required this.cost,
    required this.tokens,
    this.startedAt,
    this.duration,
    this.subagents,
    this.model,
    this.summary,
  });

  /// The row of [entry] with the agent name from [names].
  factory RunHistoryRow.of(api.RunHistoryEntry entry, VizNames names) {
    final started = entry.startedAt;
    final ms = entry.durationMs;
    return RunHistoryRow(
      runId: entry.id,
      threadId: entry.threadId,
      label: VizRunLabel.of(
        entry.chat,
        agent: names.agent(entry.agentId),
        id: entry.id,
      ),
      state: entry.state,
      queuedAt: entry.queuedAt,
      startedAt: started,
      finishedAt: entry.finishedAt,
      duration: ms != null
          ? Duration(milliseconds: ms)
          : started == null
          ? null
          : entry.finishedAt.difference(started),
      cost: VizCost.of(
        micros: entry.cost.amountMicros,
        currency: entry.cost.currency,
        mode: entry.billingModeUsed ?? entry.billingModeExpected,
        isEstimate: entry.cost.isEstimate,
      ),
      tokens: RunTokens.of(entry.metrics),
      subagents: entry.metrics.subagents,
      model: entry.model,
      summary: entry.summary,
    );
  }

  /// The run.
  final String runId;

  /// The thread of the run.
  final String threadId;

  /// Project, chat and agent ([VizRunLabel]).
  final VizRunLabel label;

  /// The end state: completed, failed or cancelled.
  final api.RunState state;

  /// The acceptance of the run.
  final DateTime queuedAt;

  /// The start; `null` for a run that ended before it started.
  final DateTime? startedAt;

  /// The end.
  final DateTime finishedAt;

  /// The running time; `null` for a run that never started.
  final Duration? duration;

  /// The cost with its label ("Estimate", "information only").
  final VizCost cost;

  /// The tokens.
  final RunTokens tokens;

  /// The subagents the runtime reported; `null` when it reported none.
  final int? subagents;

  /// The model; `null` when the runtime named none.
  final String? model;

  /// The summary of the agent; `null` without one.
  final String? summary;
}

/// The rows of the run history in the order of the server (newest end first).
///
/// ```dart
/// final model = RunHistoryModel.fromEntries(page.items.toList(), names: names);
/// ```
final class RunHistoryModel {
  /// Creates the model.
  const RunHistoryModel(this.rows);

  /// The model of [entries].
  factory RunHistoryModel.fromEntries(
    List<api.RunHistoryEntry> entries, {
    required VizNames names,
  }) => RunHistoryModel([for (final e in entries) RunHistoryRow.of(e, names)]);

  /// The rows.
  final List<RunHistoryRow> rows;
}
