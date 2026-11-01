// SPDX-License-Identifier: Apache-2.0

/// The timeline of runs and minute counts (docs/spec/25-visualization.md 6.5).
library;

import 'package:api_client/api_client.dart' as api;

import 'bar_model.dart';
import 'names.dart';
import 'run_label.dart';
import 'series_model.dart';

/// Runs as bars per chat and agent and the minute counts as series, over one range.
final class TimelineModel {
  /// Creates the model; [TimelineModel.fromTimeline] is the normal way.
  const TimelineModel({
    required this.from,
    required this.to,
    required this.bars,
    required this.series,
    required this.runsTruncated,
  });

  /// Derives the model from [timeline]; [names] gives the agent names. Never calls the network.
  ///
  /// Bars: one row per chat and agent in the order of the first run, labelled by the chat the
  /// server named ([VizRunLabel]); runs without a chat (the person may not read the thread) form
  /// one row per agent and channel with the agent as the main text and no link. A run without
  /// `started_at` starts at `queued_at`, a run without `finished_at` ends at [to]. Series: one
  /// per metric and dimension in the order of the first bucket, the counts summed over channel
  /// and agent per minute.
  factory TimelineModel.fromTimeline(
    api.StatsTimeline timeline, {
    VizNames names = const VizNames(),
  }) {
    final rows = <(String, String), List<BarSegment>>{};
    final labels = <(String, String), VizRunLabel>{};
    for (final run in timeline.runs) {
      final chat = run.chat;
      final key = (chat?.threadId ?? 'channel:${run.channelId}', run.agentId);
      labels.putIfAbsent(
        key,
        () => VizRunLabel.of(
          chat,
          agent: names.agent(run.agentId),
          id: chat?.threadId ?? run.channelId,
        ),
      );
      rows
          .putIfAbsent(key, () => <BarSegment>[])
          .add(
            BarSegment(
              start: run.startedAt ?? run.queuedAt,
              end: run.finishedAt ?? timeline.to,
              runState: run.state,
            ),
          );
    }
    final sums = <(api.StatsMetric, String), Map<DateTime, double>>{};
    for (final bucket in timeline.buckets) {
      final perMinute = sums.putIfAbsent((
        bucket.metric,
        bucket.dimension,
      ), () => {});
      perMinute[bucket.start] = (perMinute[bucket.start] ?? 0) + bucket.count;
    }
    var index = 0;
    return TimelineModel(
      from: timeline.from,
      to: timeline.to,
      runsTruncated: timeline.runsTruncated,
      bars: BarModel(
        from: timeline.from,
        to: timeline.to,
        rows: List.unmodifiable([
          for (final MapEntry(:key, value: segments) in rows.entries)
            BarRow(
              run: labels[key],
              segments: List.unmodifiable(
                segments..sort((a, b) => a.start.compareTo(b.start)),
              ),
            ),
        ]),
      ),
      series: List.unmodifiable([
        for (final MapEntry(key: (metric, dimension), value: points)
            in sums.entries)
          SeriesModel(
            metric: metric,
            dimension: dimension.isEmpty ? null : dimension,
            colorIndex: index++,
            points: List.unmodifiable(
              (points.entries.toList()..sort((a, b) => a.key.compareTo(b.key)))
                  .map((e) => SeriesPoint(e.key, e.value)),
            ),
          ),
      ]),
    );
  }

  /// Start of the range.
  final DateTime from;

  /// End of the range.
  final DateTime to;

  /// The runs as bars.
  final BarModel bars;

  /// The minute counts per metric and dimension.
  final List<SeriesModel> series;

  /// The server cut the runs (`runs_truncated`): the view shows "More runs than shown".
  final bool runsTruncated;
}
