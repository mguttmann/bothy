// SPDX-License-Identifier: Apache-2.0

/// The REST queries of the six views over the generated client (docs/spec/25-visualization.md
/// 5, 7.1; brief M6-16 Scope).
library;

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:built_collection/built_collection.dart';
import 'package:dio/dio.dart';

/// [StatsQuerySource] over `api`: one call per query operation, each error as the
/// [api.ApiProblem] of the answer (the views map `forbidden`, `not_found`, `feature_disabled`
/// and `rate_limited` to their texts, 25 11).
///
/// The desktop and mobile app pass the client of the signed-in session (DPoP, refresh and the
/// step-up interceptor apply), the console its cookie client (CSRF header); the source itself
/// knows neither (D-M6-16-6). An error without a problem body (no answer at all) passes as it
/// is, so the views show the general text.
///
/// Example:
/// ```dart
/// final queries = ApiStatsQuerySource(session.api);
/// final report = await queries.slo();
/// ```
final class ApiStatsQuerySource implements StatsQuerySource {
  /// Creates the source over [_api].
  ApiStatsQuerySource(this._api);

  final api.ApiClient _api;

  api.StatsApi get _stats => _api.getStatsApi();

  static Future<T> _call<T>(Future<Response<T>> Function() call) async {
    try {
      final data = (await call()).data;
      if (data == null) throw const api.ApiProblem(status: 0, code: 'internal');
      return data;
    } on DioException catch (e) {
      final problem =
          api.ApiProblem.of(e) ??
          (e.response == null
              ? null
              : api.ApiProblem.fromResponse(e.response!));
      if (problem == null) rethrow;
      throw problem;
    }
  }

  @override
  Future<api.StatsLiveFrame> live({String? spaceId}) =>
      _call(() => _stats.getStatsLive(spaceId: spaceId));

  @override
  Future<api.StatsTimeline> timeline({
    required DateTime from,
    required DateTime to,
    String? spaceId,
    List<api.StatsMetric>? metrics,
  }) => _call(
    () => _stats.getStatsTimeline(
      from: from.toUtc(),
      to: to.toUtc(),
      spaceId: spaceId,
      metric: metrics == null ? null : BuiltList<api.StatsMetric>(metrics),
    ),
  );

  @override
  Future<api.StatsSloReport> slo() => _call(_stats.getStatsSlo);

  @override
  Future<api.StatsBackendReport> backends({
    required DateTime from,
    required DateTime to,
    String? spaceId,
  }) => _call(
    () => _stats.getStatsBackends(
      from: from.toUtc(),
      to: to.toUtc(),
      spaceId: spaceId,
    ),
  );

  @override
  Future<api.StatsSeries> usage({
    required DateTime from,
    required DateTime to,
    api.StatsGranularity? granularity,
    String? spaceId,
  }) => _call(
    () => _stats.getUsageStats(
      from: from.toUtc(),
      to: to.toUtc(),
      granularity: granularity,
      spaceId: spaceId,
    ),
  );

  @override
  Future<api.StatsSeries> runs({
    required DateTime from,
    required DateTime to,
    api.StatsGranularity? granularity,
    String? spaceId,
    String? agentId,
  }) => _call(
    () => _stats.getRunStats(
      from: from.toUtc(),
      to: to.toUtc(),
      granularity: granularity,
      spaceId: spaceId,
      agentId: agentId,
    ),
  );

  @override
  Future<api.StatsSeries> errors({
    required DateTime from,
    required DateTime to,
    api.StatsGranularity? granularity,
    String? spaceId,
  }) => _call(
    () => _stats.getErrorStats(
      from: from.toUtc(),
      to: to.toUtc(),
      granularity: granularity,
      spaceId: spaceId,
    ),
  );

  @override
  Future<api.CostSeries> costs({
    required DateTime from,
    required DateTime to,
    api.StatsGranularity? granularity,
    String? groupBy,
    String? spaceId,
    String? agentId,
  }) => _call(
    () => _stats.getCostStats(
      from: from.toUtc(),
      to: to.toUtc(),
      granularity: granularity,
      groupBy: groupBy,
      spaceId: spaceId,
      agentId: agentId,
    ),
  );
}
