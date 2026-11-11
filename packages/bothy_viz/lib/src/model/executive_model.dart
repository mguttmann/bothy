// SPDX-License-Identifier: Apache-2.0

/// The executive page (docs/spec/25-visualization.md 6.4).
library;

import 'package:api_client/api_client.dart' as api;

import 'costs.dart';
import 'metric_tile_model.dart';
import 'names.dart';
import 'series_model.dart';
import 'traffic_light_model.dart';

/// Whether the query metric [name] is a rate: `<what>_rate` (`run_failure_rate`,
/// `webhook_failure_rate`) or the push rate of a provider (`push_failure_rate_apns`). The
/// server sends a rate as a share from 0 to 1 (failed per finished), never as percent; the
/// statistics page of the administration tells rates from counts with this rule too.
///
/// A count is never a rate, also when its code reads like one: `failed_reason_rate_limited`
/// counts the runs that failed for the reason `rate_limited`.
bool vizIsRate(String name) =>
    !ExecutiveModel.isCount(name) &&
    (name.endsWith('_rate') || name.startsWith('push_failure_rate_'));

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
  /// ([usage], and [people] for the answer in weeks), `getRunStats` ([runs]), `getErrorStats`
  /// ([errors]) and `getCostStats` ([costs]); [names] names the groups of the series. Never
  /// calls the network.
  ///
  /// Tiles: one per metric name of [usage], [runs] and [errors] in the order of the first
  /// bucket. A count ([isCount]) is the sum of every bucket, so it covers the whole range of
  /// the query. A level is the value of the latest bucket of a single group and carries the
  /// start of that bucket ([MetricTileModel.since]); a rate ([vizIsRate]) is a level that the
  /// server sends as a share and the tile holds in percent. [people] adds only its
  /// `active_people` after the tiles of [usage]: the server counts people per week or month
  /// only, and withholds a number below the minimum group
  /// ([MetricTileModel.belowMinGroup]). Cost tiles: one per billing mode of [costs] with the
  /// summed amount; `personal` carries "information only", `organization` "Estimate" while
  /// any of its buckets is an estimate; `null` without [costs].
  factory ExecutiveModel.from({
    api.StatsSloReport? slo,
    api.StatsSeries? usage,
    api.StatsSeries? people,
    api.StatsSeries? runs,
    api.StatsSeries? errors,
    api.CostSeries? costs,
    VizNames names = const VizNames(),
  }) {
    final series = <SeriesModel>[];
    final tiles = <MetricTileModel>[];
    final named = <String>{};
    void add(api.StatsSeries? response) {
      if (response == null) return;
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

    add(usage);
    if (people != null && named.add(peopleMetric)) {
      final tile = _peopleTile(people);
      if (tile != null) tiles.add(tile);
    }
    add(runs);
    add(errors);
    return ExecutiveModel(
      trafficLights: slo == null ? const [] : TrafficLightModel.fromReport(slo),
      tiles: List.unmodifiable(tiles),
      series: List.unmodifiable(series),
      costTiles: costs == null ? null : List.unmodifiable(_costTiles(costs)),
    );
  }

  /// Counts of the query responses that add up over buckets and groups, by their full name;
  /// [isCount] adds the counts named by a prefix.
  static const Set<String> additiveMetrics = {
    'messages',
    'threads_created',
    'threads_done',
    'runs',
    'completed',
    'failed',
    'cancelled',
    'invocation_denials',
    'scan_errors',
    'scan_infected',
  };

  /// The distinct people of a week or month (`getUsageStats`): a level, never a sum.
  static const String peopleMetric = 'active_people';

  /// The current storage of a space, which the server puts into the last bucket only: a level
  /// that belongs to no bucket, so its tile names none.
  static const String storageMetric = 'storage_bytes';

  /// Whether [metric] is a count that adds up over buckets and groups: [additiveMetrics], the
  /// messages by origin (`messages_human`) and the failed runs by reason
  /// (`failed_reason_stalled`). Every other metric (percentiles, rates, distinct counts such
  /// as `active_people`, a name of a newer server) is a level that does not.
  static bool isCount(String metric) =>
      additiveMetrics.contains(metric) ||
      metric.startsWith('messages_') ||
      metric.startsWith('failed_reason_');

  /// The tile of [metric] over its [groups]: a count is the sum of every bucket; a level is
  /// the value of the latest bucket, and only with one group, because levels of several
  /// groups do not combine (a distinct count of two spaces is no sum). `null` when no value
  /// can be shown, never zero.
  static MetricTileModel? _tile(String metric, List<SeriesModel> groups) {
    final rate = vizIsRate(metric);
    final unit = rate
        ? TileUnit.percent
        : metric.contains('_ms')
        ? TileUnit.milliseconds
        : TileUnit.count;
    if (isCount(metric)) {
      final sum = groups.fold<double>(
        0,
        (a, s) => a + s.points.fold<double>(0, (b, p) => b + p.value),
      );
      return MetricTileModel(queryMetric: metric, value: sum, unit: unit);
    }
    if (groups.length != 1 || groups.single.points.isEmpty) return null;
    final latest = groups.single.points.last;
    return MetricTileModel(
      queryMetric: metric,
      // The server sends a rate as a share from 0 to 1; a percent tile holds 0 to 100.
      value: rate ? latest.value * 100 : latest.value,
      unit: unit,
      since: metric == storageMetric ? null : latest.at,
    );
  }

  /// The tile of [peopleMetric] from the answer in weeks: the latest bucket that carries the
  /// number or withholds it, and only with one group (the server groups by space, and the
  /// people of two spaces are no sum). `null` without such a bucket.
  static MetricTileModel? _peopleTile(api.StatsSeries people) {
    bool withheld(api.StatsBucket b) =>
        b.belowMinGroup?.contains(peopleMetric) ?? false;
    final groups = <String?>{};
    api.StatsBucket? latest;
    for (final bucket in people.buckets) {
      if (!withheld(bucket) && !bucket.metrics.containsKey(peopleMetric)) {
        continue;
      }
      groups.add(bucket.groupKey);
      if (latest == null || bucket.start.isAfter(latest.start)) latest = bucket;
    }
    if (latest == null || groups.length != 1) return null;
    return MetricTileModel(
      queryMetric: peopleMetric,
      value: latest.metrics[peopleMetric],
      unit: TileUnit.count,
      belowMinGroup: withheld(latest),
      since: latest.start,
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
