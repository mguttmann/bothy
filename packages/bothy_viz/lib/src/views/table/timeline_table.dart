// SPDX-License-Identifier: Apache-2.0

/// The table view of the daily timeline (docs/spec/25-visualization.md 6.5, 7.4).
library;

import '../../marks/viz_texts.dart';
import '../../model/timeline_model.dart';
import '../view_texts.dart';
import 'delivery_table.dart';
import 'viz_table.dart';

/// The timeline at the playhead: one row per run bar (row, start, end, state) and the series
/// with their value at the playhead, the same the legend of the area shows.
List<VizTableData> timelineTables(
  VizTexts texts,
  TimelineModel model, {
  required DateTime now,
  required String timeZone,
}) {
  final l10n = texts.l10n;
  return [
    VizTableData(
      caption: l10n.vizChartRuns,
      columns: [
        l10n.vizColRun,
        l10n.vizColStart,
        l10n.vizColEnd,
        l10n.vizColState,
      ],
      rows: [
        for (final row in model.bars.rows)
          for (final s in row.segments)
            [
              VizCell(row.label),
              VizCell(
                vizDayTime(texts, s.start, now, timeZone),
                sortBy: s.start.millisecondsSinceEpoch,
              ),
              VizCell(
                vizDayTime(texts, s.end, now, timeZone),
                sortBy: s.end.millisecondsSinceEpoch,
              ),
              VizCell(
                s.runState == null
                    ? l10n.vizNone
                    : texts.timelineState(s.runState!),
              ),
            ],
      ],
    ),
    seriesTable(texts, l10n.vizChartRates, model.series),
  ];
}
