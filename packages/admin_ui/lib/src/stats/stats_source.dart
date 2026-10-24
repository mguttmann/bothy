// SPDX-License-Identifier: Apache-2.0

/// The data of the statistics screen A7: one typed call per statistics operation of M6-05, the
/// spaces of the scope selector, the names of the agents and the telemetry link of `getInstance`
/// (docs/observability/admin-statistics.md 2 to 6; brief M6-06 Scope and Design note 1).
///
/// No permission logic and no arithmetic of statistics here: every number comes from the
/// server's answer, and a refused query reaches the screen as [SelfActionFailed] with the
/// server's problem (15 1 principle 1).
library;

import 'package:api_client/api_client.dart' as api;

import '../data/access_repository.dart' show adminCall;
import '../data/self_repository.dart' show SelfActionFailed;

/// The bucket size of a statistics query (`StatsGranularity`).
enum StatsBucketSize {
  /// One hour, in UTC.
  hour,

  /// One day from local midnight of `stats.time_zone`.
  day,

  /// One ISO week, from Monday.
  week,

  /// One calendar month.
  month,
}

/// The grouping of `getCostStats` (`group_by`).
enum StatsCostGrouping {
  /// One group per space (`group_key` is the space ID).
  space,

  /// One group per agent (`group_key` is the agent ID).
  agent,

  /// No grouping.
  none,
}

/// The parameters every statistics query shares: the range from [from] (inclusive) to [to]
/// (exclusive), the [bucket] size and the space, or null for the whole instance.
final class StatsQuery {
  /// Creates the query.
  const StatsQuery({
    required this.from,
    required this.to,
    required this.bucket,
    this.spaceId,
  });

  /// Start of the range.
  final DateTime from;

  /// End of the range.
  final DateTime to;

  /// The bucket size.
  final StatsBucketSize bucket;

  /// The space, or null for the instance (owners only; the server answers `403` otherwise).
  final String? spaceId;

  /// The same query with [bucket] instead.
  StatsQuery withBucket(StatsBucketSize bucket) =>
      StatsQuery(from: from, to: to, bucket: bucket, spaceId: spaceId);

  @override
  bool operator ==(Object other) =>
      other is StatsQuery &&
      other.from == from &&
      other.to == to &&
      other.bucket == bucket &&
      other.spaceId == spaceId;

  @override
  int get hashCode => Object.hash(from, to, bucket, spaceId);
}

/// One space of the scope selector (`listSpaces`): its ID and name.
final class StatsSpace {
  /// Creates the space.
  const StatsSpace({required this.id, required this.name});

  /// The space ID.
  final String id;

  /// The name.
  final String name;
}

/// The port of A7 to the server (brief M6-06 Scope): one call per statistics operation, plus
/// what the selectors and the labels need.
///
/// Errors: [SelfActionFailed] with the server's problem (`403`, `404`, `400 validation_failed`
/// with `currency_required`) or without an answer.
///
/// Example:
/// ```dart
/// final runs = await source.runStats(
///   StatsQuery(from: from, to: to, bucket: StatsBucketSize.day, spaceId: space),
/// );
/// ```
abstract interface class AdminStatsSource {
  /// `getUsageStats`: messages, threads, runs, active people (withheld below
  /// `privacy.stats_min_group_size`, named in `below_min_group`) and storage per space.
  Future<api.StatsSeries> usageStats(StatsQuery query);

  /// `getRunStats`: ended runs by result, the duration percentiles and the failure reasons per
  /// agent.
  Future<api.StatsSeries> runStats(StatsQuery query);

  /// `getToolCallStats`: calls, failed calls and the median duration per tool name.
  Future<api.StatsSeries> toolCallStats(StatsQuery query);

  /// `getErrorStats`: the error rates of the scope.
  Future<api.StatsSeries> errorStats(StatsQuery query);

  /// `getCostStats` grouped by [groupBy] in [currency] (null: the server's choice, which fails
  /// with `currency_required` when the range has several currencies).
  Future<api.CostSeries> costStats(
    StatsQuery query, {
    required StatsCostGrouping groupBy,
    String? currency,
  });

  /// The spaces the caller can list (`listSpaces`), for the scope selector; the first query of a
  /// space decides whether the server answers for it.
  Future<List<StatsSpace>> statsSpaces();

  /// The display name per agent ID (`listAgents`), for the labels of the agent groups.
  Future<Map<String, String>> statsAgentNames();

  /// `InstanceInfo.telemetry_dashboard_url` of `getInstance`: set only for owners and only
  /// when `telemetry.dashboard_url` is configured, else null.
  Future<Uri?> telemetryDashboardUrl();
}

/// [AdminStatsSource] over the generated client (the client of the signed-in session, so DPoP,
/// refresh and the step-up interceptor apply on the desktop, and the cookie and CSRF header in
/// the console).
///
/// Example:
/// ```dart
/// final stats = ApiAdminStatsSource(api);
/// final usage = await stats.usageStats(query);
/// ```
final class ApiAdminStatsSource implements AdminStatsSource {
  /// Creates the source over `api`.
  ApiAdminStatsSource(this._api);

  final api.ApiClient _api;

  api.StatsApi get _stats => _api.getStatsApi();

  static api.StatsGranularity _granularity(StatsBucketSize b) => switch (b) {
    StatsBucketSize.hour => api.StatsGranularity.hour,
    StatsBucketSize.day => api.StatsGranularity.day,
    StatsBucketSize.week => api.StatsGranularity.week,
    StatsBucketSize.month => api.StatsGranularity.month,
  };

  @override
  Future<api.StatsSeries> usageStats(StatsQuery q) => adminCall(
    () async => (await _stats.getUsageStats(
      from: q.from.toUtc(),
      to: q.to.toUtc(),
      granularity: _granularity(q.bucket),
      spaceId: q.spaceId,
    )).data!,
  );

  @override
  Future<api.StatsSeries> runStats(StatsQuery q) => adminCall(
    () async => (await _stats.getRunStats(
      from: q.from.toUtc(),
      to: q.to.toUtc(),
      granularity: _granularity(q.bucket),
      spaceId: q.spaceId,
    )).data!,
  );

  @override
  Future<api.StatsSeries> toolCallStats(StatsQuery q) => adminCall(
    () async => (await _stats.getToolCallStats(
      from: q.from.toUtc(),
      to: q.to.toUtc(),
      granularity: _granularity(q.bucket),
      spaceId: q.spaceId,
    )).data!,
  );

  @override
  Future<api.StatsSeries> errorStats(StatsQuery q) => adminCall(
    () async => (await _stats.getErrorStats(
      from: q.from.toUtc(),
      to: q.to.toUtc(),
      granularity: _granularity(q.bucket),
      spaceId: q.spaceId,
    )).data!,
  );

  @override
  Future<api.CostSeries> costStats(
    StatsQuery q, {
    required StatsCostGrouping groupBy,
    String? currency,
  }) => adminCall(
    () async => (await _stats.getCostStats(
      from: q.from.toUtc(),
      to: q.to.toUtc(),
      granularity: _granularity(q.bucket),
      groupBy: groupBy.name,
      spaceId: q.spaceId,
      currency: currency,
    )).data!,
  );

  @override
  Future<List<StatsSpace>> statsSpaces() => adminCall(() async {
    final spaces = (await _api.getSpacesApi().listSpaces()).data!.items;
    return [
      for (final s in spaces) StatsSpace(id: s.space.id, name: s.space.name),
    ];
  });

  @override
  Future<Map<String, String>> statsAgentNames() => adminCall(() async {
    final out = <String, String>{};
    String? cursor;
    do {
      final page = (await _api.getAgentsApi().listAgents(cursor: cursor)).data!;
      for (final a in page.items) {
        out[a.id] = a.displayName;
      }
      cursor = page.nextCursor;
    } while (cursor != null);
    return out;
  });

  @override
  Future<Uri?> telemetryDashboardUrl() => adminCall(() async {
    final url =
        (await _api.getAdminApi().getInstance()).data!.telemetryDashboardUrl;
    return url == null ? null : Uri.tryParse(url);
  });
}

/// The currencies a `400 validation_failed` with `errors[].code` `currency_required` names in
/// its detail (`getCostStats`: "the currencies found in `detail`"), in their order; empty for
/// any other error.
List<String> requiredCurrencies(Object error) {
  if (error is! SelfActionFailed) return const [];
  final problem = error.problem;
  if (problem == null) return const [];
  final field = problem.errors
      .where((e) => e.code == 'currency_required')
      .firstOrNull;
  if (field == null) return const [];
  final text = field.detail ?? problem.detail ?? '';
  return <String>{
    for (final m in RegExp(r'\b[A-Z]{3}\b').allMatches(text)) m.group(0)!,
  }.toList();
}

/// The metrics of [bucket] as a plain map.
Map<String, double> metricsOf(api.StatsBucket bucket) => bucket.metrics.toMap();
