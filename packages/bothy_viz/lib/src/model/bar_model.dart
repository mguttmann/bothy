// SPDX-License-Identifier: Apache-2.0

/// Rows of bars over a time range: the runs of the timeline (docs/spec/25-visualization.md 6.5).
library;

import 'package:api_client/api_client.dart' as api;

/// One bar from [start] to [end], colored by its run state or by a series role.
final class BarSegment {
  /// Creates a segment; exactly one of [runState] and [colorIndex] colors it.
  const BarSegment({
    required this.start,
    required this.end,
    this.runState,
    this.colorIndex,
  }) : assert((runState == null) != (colorIndex == null), 'one color');

  /// Start of the bar.
  final DateTime start;

  /// End of the bar; for a run still active, the end of the range.
  final DateTime end;

  /// Final or current state of a run: the run state roles of the live box.
  final api.RunState? runState;

  /// Position in the series roles, for bars that are not runs.
  final int? colorIndex;
}

/// One row: a label and its bars.
final class BarRow {
  /// Creates a row.
  const BarRow({required this.label, required this.segments});

  /// Row label, for example `dev-runner, #backend`.
  final String label;

  /// Bars in time order.
  final List<BarSegment> segments;
}

/// Rows of bars over the range [from] to [to].
final class BarModel {
  /// Creates the model.
  const BarModel({required this.from, required this.to, required this.rows});

  /// Start of the range.
  final DateTime from;

  /// End of the range.
  final DateTime to;

  /// The rows in the order of first appearance.
  final List<BarRow> rows;
}
