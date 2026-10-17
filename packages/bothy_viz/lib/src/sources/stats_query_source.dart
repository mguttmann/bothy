// SPDX-License-Identifier: Apache-2.0

/// The port of the statistics queries (docs/spec/25-visualization.md 5, 7.1).
library;

import 'package:api_client/api_client.dart' as api;

/// One typed call per query operation of `docs/api/openapi.yaml`.
///
/// Every call returns the generated type of the operation or throws the [api.ApiProblem] of
/// the failed request (M4-02); the views map `forbidden`, `not_found`, `feature_disabled` and
/// `rate_limited` to their messages (25 11). The apps implement it with the generated client,
/// the tests and the harness with [FixtureStatsQuerySource].
///
/// ```dart
/// final report = await source.slo();
/// ```
abstract interface class StatsQuerySource {
  /// `getStatsLive`: the current frame of [spaceId] or of the instance.
  Future<api.StatsLiveFrame> live({String? spaceId});

  /// `getStatsTimeline`: runs and minute counts from [from] to [to].
  Future<api.StatsTimeline> timeline({
    required DateTime from,
    required DateTime to,
    String? spaceId,
    List<api.StatsMetric>? metrics,
  });

  /// `getStatsSlo`: the SLO report of the own measurement.
  Future<api.StatsSloReport> slo();

  /// `getStatsBackends`: the backend report from [from] to [to].
  Future<api.StatsBackendReport> backends({
    required DateTime from,
    required DateTime to,
    String? spaceId,
  });

  /// `getUsageStats`.
  Future<api.StatsSeries> usage({
    required DateTime from,
    required DateTime to,
    api.StatsGranularity? granularity,
    String? spaceId,
  });

  /// `getRunStats`.
  Future<api.StatsSeries> runs({
    required DateTime from,
    required DateTime to,
    api.StatsGranularity? granularity,
    String? spaceId,
    String? agentId,
  });

  /// `getErrorStats`.
  Future<api.StatsSeries> errors({
    required DateTime from,
    required DateTime to,
    api.StatsGranularity? granularity,
    String? spaceId,
  });

  /// `getCostStats`.
  Future<api.CostSeries> costs({
    required DateTime from,
    required DateTime to,
    api.StatsGranularity? granularity,
    String? groupBy,
    String? spaceId,
    String? agentId,
  });
}
