// SPDX-License-Identifier: Apache-2.0

/// The executive page (docs/spec/25-visualisierung.md 6.4).
library;

import 'package:api_client/api_client.dart' as api;

import 'costs.dart';
import 'metric_tile_model.dart';
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
  /// ([costs]). Never calls the network.
  ///
  /// Tiles: the sum per metric name of [usage], [runs] and [errors] over all buckets, in the
  /// order of the first bucket. Cost tiles: one per billing mode of [costs] with the summed
  /// amount; `personal` carries "information only", `organization` "Estimate" while any of
  /// its buckets is an estimate; `null` without [costs].
  factory ExecutiveModel.from({
    api.StatsSloReport? slo,
    api.StatsSeries? usage,
    api.StatsSeries? runs,
    api.StatsSeries? errors,
    api.CostSeries? costs,
  }) {
    final series = <SeriesModel>[];
    final tiles = <MetricTileModel>[];
    for (final response in [usage, runs, errors]) {
      if (response == null) continue;
      final parts = SeriesModel.fromStatsSeries(
        response,
        firstColor: series.length,
      );
      series.addAll(parts);
      final totals = <String, double>{};
      for (final s in parts) {
        totals[s.queryMetric!] =
            (totals[s.queryMetric!] ?? 0) +
            s.points.fold<double>(0, (a, p) => a + p.value);
      }
      for (final MapEntry(:key, :value) in totals.entries) {
        tiles.add(
          MetricTileModel(queryMetric: key, value: value, unit: TileUnit.count),
        );
      }
    }
    return ExecutiveModel(
      trafficLights: slo == null ? const [] : TrafficLightModel.fromReport(slo),
      tiles: List.unmodifiable(tiles),
      series: List.unmodifiable(series),
      costTiles: costs == null ? null : List.unmodifiable(_costTiles(costs)),
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
