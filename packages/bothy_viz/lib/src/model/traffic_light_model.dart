// SPDX-License-Identifier: Apache-2.0

/// The SLO traffic light of the executive page (docs/spec/25-visualization.md 6.4).
library;

import 'package:api_client/api_client.dart' as api;
import 'package:design_system/design_system.dart';

/// What the traffic light says; the mark takes its text from `l10n`.
enum TrafficLightStatus {
  /// `green`: "on target".
  onTarget,

  /// `amber`: "budget tight".
  budgetTight,

  /// `red`: "target missed".
  targetMissed,

  /// `insufficient_data`: "not enough data".
  notEnoughData,
}

/// State role of a traffic light: `color.success.*`, `color.warning.*`, `color.danger.*` or the
/// neutral role (25 10 item 2).
enum VizStateRole {
  /// `color.success.*`.
  success,

  /// `color.warning.*`.
  warning,

  /// `color.danger.*`.
  danger,

  /// Neutral text and border roles, for "not enough data".
  neutral,
}

/// One SLO of `getStatsSlo` as a traffic light.
///
/// The server computes ratio, remaining budget and state; the model only maps the
/// `StatsSloState` value to role, icon and text, so the client holds no threshold and no
/// permission logic (25 4 rule 5).
final class TrafficLightModel {
  /// Creates a traffic light.
  const TrafficLightModel({
    required this.slo,
    required this.metric,
    required this.status,
    required this.objective,
    required this.periodDays,
    required this.total,
    required this.good,
    this.ratio,
    this.budgetRemaining,
    this.thresholdMs,
  });

  /// Maps one entry; a state of a newer server reads as "not enough data".
  factory TrafficLightModel.fromEntry(api.StatsSloEntry entry) =>
      TrafficLightModel(
        slo: entry.slo,
        metric: entry.metric,
        status: switch (entry.state) {
          api.StatsSloState.green => TrafficLightStatus.onTarget,
          api.StatsSloState.amber => TrafficLightStatus.budgetTight,
          api.StatsSloState.red => TrafficLightStatus.targetMissed,
          _ => TrafficLightStatus.notEnoughData,
        },
        objective: entry.objective,
        periodDays: entry.periodDays,
        total: entry.total,
        good: entry.good,
        ratio: entry.ratio,
        budgetRemaining: entry.budgetRemaining,
        thresholdMs: entry.thresholdMs,
      );

  /// Every entry of [report] in the order of the report.
  static List<TrafficLightModel> fromReport(api.StatsSloReport report) =>
      List.unmodifiable(report.items.map(TrafficLightModel.fromEntry));

  /// `service/name` of the SLO.
  final String slo;

  /// Its `sli.*` metric.
  final api.StatsMetric metric;

  /// What the light says.
  final TrafficLightStatus status;

  /// Target in percent.
  final double objective;

  /// SLO period in days.
  final int periodDays;

  /// Events in the period.
  final int total;

  /// Good events in the period.
  final int good;

  /// Good share; `null` without events.
  final double? ratio;

  /// Remaining error budget; `null` when the server sent none.
  final double? budgetRemaining;

  /// Threshold of a latency SLO.
  final int? thresholdMs;

  /// The state role of [status].
  VizStateRole get role => switch (status) {
    TrafficLightStatus.onTarget => VizStateRole.success,
    TrafficLightStatus.budgetTight => VizStateRole.warning,
    TrafficLightStatus.targetMissed => VizStateRole.danger,
    TrafficLightStatus.notEnoughData => VizStateRole.neutral,
  };

  /// The icon of [status]; never color alone (25 6.4).
  DsGlyph get glyph => switch (status) {
    TrafficLightStatus.onTarget => DsIcons.triggerDone,
    TrafficLightStatus.budgetTight => DsIcons.stateWarning,
    TrafficLightStatus.targetMissed => DsIcons.stateError,
    TrafficLightStatus.notEnoughData => DsIcons.stateInfo,
  };
}
