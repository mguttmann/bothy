// SPDX-License-Identifier: Apache-2.0

import 'package:bothy_viz/bothy_viz.dart';

import 'fixtures.dart';

/// The models of every mark from the fixtures, one entry per relevant fixture state.
class MarkModels {
  MarkModels._(
    this.normal,
    this.noCost,
    this.timeline,
    this.lights,
    this.tiles,
    this.costTiles,
    this.usage,
  );

  /// Loads every fixture the marks show.
  static Future<MarkModels> load() async {
    final normal = LiveTopologyModel.fromFrame(
      await fixtures.liveFrame('normal'),
      names: names,
    );
    final noCost = LiveTopologyModel.fromFrame(
      await fixtures.liveFrame('no_cost_permission'),
      names: names,
    );
    final timeline = TimelineModel.fromTimeline(
      await fixtures.timeline('normal'),
      names: names,
    );
    final executive = ExecutiveModel.from(
      slo: await fixtures.sloReport('normal'),
      usage: await fixtures.series('normal', query: 'usage'),
      costs: await fixtures.costSeries('normal'),
      names: names,
    );
    return MarkModels._(
      normal,
      noCost,
      timeline,
      executive.trafficLights,
      executive.tiles,
      executive.costTiles!,
      executive.series,
    );
  }

  /// Live topology of `StatsLiveFrame.normal.json`.
  final LiveTopologyModel normal;

  /// Live topology of `StatsLiveFrame.no_cost_permission.json`.
  final LiveTopologyModel noCost;

  /// Timeline of `StatsTimeline.normal.json`.
  final TimelineModel timeline;

  /// Traffic lights of `StatsSloReport.normal.json` (all four states).
  final List<TrafficLightModel> lights;

  /// Usage tiles of `StatsSeries.usage.normal.json`.
  final List<MetricTileModel> tiles;

  /// Cost tiles of `CostSeries.normal.json`.
  final List<MetricTileModel> costTiles;

  /// Series of `StatsSeries.usage.normal.json`.
  final List<SeriesModel> usage;
}
