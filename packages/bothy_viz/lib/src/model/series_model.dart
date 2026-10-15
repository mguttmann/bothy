// SPDX-License-Identifier: Apache-2.0

/// Series of the area and bar marks and the rates of a live frame.
library;

import 'package:api_client/api_client.dart' as api;

import 'names.dart';

/// Number of categorical series roles `color.viz.series.1` to `.8` (25 10).
const int vizSeriesCount = 8;

/// One point of a series.
final class SeriesPoint {
  /// Creates a point.
  const SeriesPoint(this.at, this.value);

  /// Start of the bucket.
  final DateTime at;

  /// Value of the bucket.
  final double value;
}

/// A labelled series: a metric of 25 3.2 ([metric]) or a metric name of a query
/// ([queryMetric], for example `messages` of `getUsageStats`), with its color role.
final class SeriesModel {
  /// Creates a series; exactly one of [metric] and [queryMetric] names it.
  const SeriesModel({
    required this.colorIndex,
    required this.points,
    this.metric,
    this.queryMetric,
    this.dimension,
    this.groupKey,
    this.groupLabel,
  }) : assert((metric == null) != (queryMetric == null), 'one name');

  /// The series of a query response: one per metric name and group, in the order of the first
  /// bucket, with the color roles counted from [firstColor]; [names] resolves the group keys
  /// into [groupLabel], so two groups of one metric read differently and not by color alone.
  static List<SeriesModel> fromStatsSeries(
    api.StatsSeries series, {
    int firstColor = 0,
    VizNames names = const VizNames(),
  }) {
    final points = <(String, String?), List<SeriesPoint>>{};
    for (final bucket in series.buckets) {
      for (final MapEntry(:key, :value) in bucket.metrics.entries) {
        points
            .putIfAbsent((key, bucket.groupKey), () => [])
            .add(SeriesPoint(bucket.start, value));
      }
    }
    var index = firstColor;
    return [
      for (final MapEntry(key: (metric, group), value: list) in points.entries)
        SeriesModel(
          queryMetric: metric,
          groupKey: group,
          groupLabel: group == null ? null : names.any(group),
          colorIndex: index++,
          points: List.unmodifiable(list..sort((a, b) => a.at.compareTo(b.at))),
        ),
    ];
  }

  /// Metric of the own measurement; its label comes from `l10n` (`vizMetricLabel`).
  final api.StatsMetric? metric;

  /// Metric name of a query response (`StatsBucket.metrics` key).
  final String? queryMetric;

  /// Dimension code, `null` for all dimensions.
  final String? dimension;

  /// Group of a grouped query (space or agent ID), `null` without grouping.
  final String? groupKey;

  /// Display name of [groupKey]; `null` without grouping.
  final String? groupLabel;

  /// Position in the fixed order of the series roles; the mark wraps it after
  /// [vizSeriesCount] (`color.viz.series.<colorIndex % 8 + 1>`).
  final int colorIndex;

  /// The points in time order.
  final List<SeriesPoint> points;

  /// The last value, `null` for an empty series.
  double? get last => points.isEmpty ? null : points.last.value;
}

/// One rate of a live frame: a metric of 25 3.2 over the last full and the current minute.
final class RateModel {
  /// Creates a rate.
  const RateModel({
    required this.metric,
    required this.lastMinute,
    required this.currentMinute,
    required this.colorIndex,
    this.dimension,
    this.lastMinuteSum,
    this.currency,
  });

  /// Derives the rate of [rate]; [colorIndex] is its position in the frame.
  factory RateModel.fromRate(api.StatsLiveRate rate, int colorIndex) =>
      RateModel(
        metric: rate.metric,
        dimension: rate.dimension,
        lastMinute: rate.lastMinute,
        currentMinute: rate.currentMinute,
        lastMinuteSum: rate.lastMinuteSum,
        currency: rate.currency,
        colorIndex: colorIndex,
      );

  /// Metric.
  final api.StatsMetric metric;

  /// Dimension code; `null` for all dimensions summed.
  final String? dimension;

  /// Count of the last full minute.
  final int lastMinute;

  /// Count of the current minute up to the frame time.
  final int currentMinute;

  /// Sum of values of the last full minute (milliseconds or micro units); `null` when absent.
  final int? lastMinuteSum;

  /// Currency, only for `cost`.
  final String? currency;

  /// Position in the fixed order of the series roles.
  final int colorIndex;
}
