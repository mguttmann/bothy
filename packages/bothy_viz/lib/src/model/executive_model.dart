// SPDX-License-Identifier: Apache-2.0

/// The executive page (docs/spec/25-visualisierung.md 6.4).
library;

import 'package:api_client/api_client.dart' as api;

import 'costs.dart';
import 'metric_tile_model.dart';
import 'names.dart';
import 'series_model.dart';
import 'traffic_light_model.dart';

/// Traffic lights, metric tiles, cost tiles per mode and the series of one range.
///
/// Every part is optional input: a query the user may not run, or one the server answered
/// without a member, leaves its part empty or `null` ("not shown"), never zero (25 4 rule 5).
final class ExecutiveModel {
  /// Creates the model; [ExecutiveModel.from] is the normal way.
  const ExecutiveModel({
    required this.trafficLights,
    required this.tiles,
    required this.series,
    this.costTiles,
  });

  /// Derives the model from the responses of `getStatsSlo` ([slo]), `getUsageStats`
  /// ([usage]), `getRunStats` ([runs]), `getErrorStats` ([errors]) and `getCostStats`
  /// ([costs]); [names] names the groups of the series. Never calls the network.
  ///
  /// Tiles: one per metric name of [usage], [runs] and [errors] in the order of the first
  /// bucket ([additiveMetrics] summed, levels from the latest bucket of a single group). Cost tiles: one per billing mode of [costs] with the summed
  /// amount; `personal` carries "information only", `organization` "Estimate" while any of
  /// its buckets is an estimate; `null` without [costs].
  factory ExecutiveModel.from({
    api.StatsSloReport? slo,
    api.StatsSeries? usage,
    api.StatsSeries? runs,
    api.StatsSeries? errors,
    api.CostSeries? costs,
    VizNames names = const VizNames(),
  }) {
    final series = <SeriesModel>[];
    final tiles = <MetricTileModel>[];
    final named = <String>{};
    for (final response in [usage, runs, errors]) {
      if (response == null) continue;
      final parts = SeriesModel.fromStatsSeries(
        response,
        firstColor: series.length,
        names: names,
      );
      series.addAll(parts);
      final byMetric = <String, List<SeriesModel>>{};
      for (final s in parts) {
        byMetric.putIfAbsent(s.queryMetric!, () => []).add(s);
      }
      for (final MapEntry(key: metric, value: groups) in byMetric.entries) {
        // A metric of an earlier query (`runs` of usage and of run stats) keeps its first tile.
        if (!named.add(metric)) continue;
        final tile = _tile(metric, groups);
        if (tile != null) tiles.add(tile);
      }
    }
    return ExecutiveModel(
      trafficLights: slo == null ? const [] : TrafficLightModel.fromReport(slo),
      tiles: List.unmodifiable(tiles),
      series: List.unmodifiable(series),
      costTiles: costs == null ? null : List.unmodifiable(_costTiles(costs)),
    );
  }

  /// Counters of the query responses that add up over buckets and groups; every other metric
  /// (percentiles, rates, distinct counts such as `active_people`) is a level that does not.
  static const Set<String> additiveMetrics = {
    'messages',
    'threads_created',
    'runs',
    'completed',
    'failed',
    'cancelled',
    'invocation_denials',
  };

  /// The tile of [metric] over its [groups]: a counter is the sum of every bucket; a level is
  /// the value of the latest bucket, and only with one group, because levels of several
  /// groups do not combine (a distinct count of two spaces is no sum). `null` when no value
  /// can be shown, never zero.
  static MetricTileModel? _tile(String metric, List<SeriesModel> groups) {
    final unit = metric.endsWith('_rate')
        ? TileUnit.percent
        : metric.contains('_ms')
        ? TileUnit.milliseconds
        : TileUnit.count;
    if (additiveMetrics.contains(metric)) {
      final sum = groups.fold<double>(
        0,
        (a, s) => a + s.points.fold<double>(0, (b, p) => b + p.value),
      );
      return MetricTileModel(queryMetric: metric, value: sum, unit: unit);
    }
    if (groups.length != 1 || groups.single.last == null) return null;
    return MetricTileModel(
      queryMetric: metric,
      value: groups.single.last,
      unit: unit,
    );
  }

  static List<MetricTileModel> _costTiles(api.CostSeries costs) {
    final micros = <api.BillingMode, int>{};
    final estimate = <api.BillingMode, bool>{};
    for (final bucket in costs.buckets) {
      micros[bucket.billingMode] =
          (micros[bucket.billingMode] ?? 0) + bucket.amountMicros;
      estimate[bucket.billingMode] =
          (estimate[bucket.billingMode] ?? false) || bucket.isEstimate;
    }
    return [
      for (final MapEntry(key: mode, value: amount) in micros.entries)
        MetricTileModel.cost(
          VizCost.of(
            micros: amount,
            currency: costs.currency,
            mode: mode,
            isEstimate: estimate[mode]!,
          ),
        ),
    ];
  }

  /// One traffic light per SLO in the order of the report; empty without a report.
  final List<TrafficLightModel> trafficLights;

  /// Metric tiles of the query responses.
  final List<MetricTileModel> tiles;

  /// Cost tiles per billing mode; `null` when not shown.
  final List<MetricTileModel>? costTiles;

  /// The series of the query responses, for the area mark.
  final List<SeriesModel> series;
}
