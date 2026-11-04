// SPDX-License-Identifier: Apache-2.0

/// The table view of the run history (docs/spec/25-visualization.md 6.7, 7.4).
library;

import '../../marks/viz_texts.dart';
import '../../model/run_history_model.dart';
import '../view_texts.dart';
import 'viz_table.dart';

/// The text of an optional count: the formatted number, or "none" when the run reported none.
String vizOptionalCount(VizTexts texts, int? value) =>
    value == null ? texts.l10n.vizNone : texts.format.count(value);

/// The finished runs of [model], one row each with the values the list shows: the chat (a tap
/// opens it through [onOpenThread]), the run (a tap opens the detail through [onOpenRun]), the
/// end state, start, end, duration, cost (only with [costsVisible]), the tokens, the subagents
/// and the model.
List<VizTableData> runHistoryTables(
  VizTexts texts,
  RunHistoryModel model, {
  required DateTime now,
  required String timeZone,
  required bool costsVisible,
  void Function(String threadId)? onOpenThread,
  void Function(String runId)? onOpenRun,
}) {
  final l10n = texts.l10n;
  return [
    VizTableData(
      caption: l10n.vizViewRunHistory,
      columns: [
        l10n.vizColChat,
        l10n.vizColRun,
        l10n.vizColState,
        l10n.vizColStart,
        l10n.vizColEnd,
        l10n.vizColDuration,
        if (costsVisible) l10n.vizColRunCost,
        l10n.vizColTokensIn,
        l10n.vizColTokensOut,
        l10n.vizColCacheRead,
        l10n.vizColCacheWrite,
        l10n.vizColSubagents,
        l10n.vizColModel,
      ],
      rows: [
        for (final row in model.rows)
          [
            vizChatCell(texts, row.label, onOpenThread),
            VizCell(
              texts.runDetail(row.label),
              onTap: onOpenRun == null ? null : () => onOpenRun(row.runId),
            ),
            VizCell(texts.timelineState(row.state)),
            VizCell(
              row.startedAt == null
                  ? l10n.vizNone
                  : vizDayTime(texts, row.startedAt!, now, timeZone),
              sortBy: (row.startedAt ?? row.queuedAt).millisecondsSinceEpoch,
            ),
            VizCell(
              vizDayTime(texts, row.finishedAt, now, timeZone),
              sortBy: row.finishedAt.millisecondsSinceEpoch,
            ),
            VizCell(
              row.duration == null
                  ? l10n.vizNone
                  : texts.format.durationShort(row.duration!),
              sortBy: row.duration?.inMilliseconds ?? -1,
            ),
            if (costsVisible)
              VizCell(texts.cost(row.cost), sortBy: row.cost.micros),
            for (final value in [
              row.tokens.input,
              row.tokens.output,
              row.tokens.cacheRead,
              row.tokens.cacheWrite,
              row.subagents,
            ])
              VizCell(vizOptionalCount(texts, value), sortBy: value ?? -1),
            VizCell(row.model ?? l10n.vizNone),
          ],
      ],
    ),
  ];
}
