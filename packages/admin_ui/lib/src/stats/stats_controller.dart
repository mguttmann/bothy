// SPDX-License-Identifier: Apache-2.0

/// The state of the statistics screen A7: the chosen range and scope, the answers of the
/// statistics operations and the entries of the scope selector the server refused
/// (docs/observability/admin-statistics.md 2 to 4; brief M6-06 Design notes 1 and 3).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as api;
import 'package:flutter/foundation.dart';

import '../data/self_repository.dart' show SelfActionFailed;
import 'stats_range.dart';
import 'stats_sections.dart' show runTotal;
import 'stats_source.dart';

/// The answer of one query: its [value], or the [error] the source threw.
final class StatsPart<T> {
  /// An answer.
  const StatsPart.value(T this.value) : error = null;

  /// A failure.
  const StatsPart.failed(Object this.error) : value = null;

  /// The answer, or null after a failure.
  final T? value;

  /// The error, or null.
  final Object? error;

  /// The server's problem of [error], or null (no problem or no answer).
  api.ApiProblem? get problem {
    final e = error;
    return e is SelfActionFailed ? e.problem : null;
  }
}

/// The answers of one load for one range and scope.
final class StatsData {
  /// Creates the data.
  const StatsData({
    required this.query,
    required this.runs,
    required this.runsPrevious,
    required this.tools,
    required this.errors,
    required this.usage,
    required this.costsByAgent,
    required this.costsBySpace,
  });

  /// The query of this load.
  final StatsQuery query;

  /// `getRunStats` of the range.
  final StatsPart<api.StatsSeries> runs;

  /// `getRunStats` of the previous range of the same length (the comparison of the run tile).
  final StatsPart<api.StatsSeries> runsPrevious;

  /// `getToolCallStats`.
  final StatsPart<api.StatsSeries> tools;

  /// `getErrorStats`.
  final StatsPart<api.StatsSeries> errors;

  /// `getUsageStats` in weeks (`active_people` exists only per week or month, D-M6-05-3).
  final StatsPart<api.StatsSeries> usage;

  /// `getCostStats` grouped by agent.
  final StatsPart<api.CostSeries> costsByAgent;

  /// `getCostStats` grouped by space.
  final StatsPart<api.CostSeries> costsBySpace;

  /// The runs of the range, or null without an answer.
  double? get runsTotal {
    final r = runs.value;
    return r == null ? null : runTotal(r);
  }

  /// The runs of the previous range, or null without an answer. The server widens the previous
  /// range to the bucket that holds the start of this one ([StatsRange]); a bucket of the same
  /// start in both answers is the same bucket of the same scope, so it counts for this range
  /// only. A bucket without runs is absent from both answers alike.
  double? get previousRunsTotal {
    final p = runsPrevious.value;
    if (p == null) return null;
    final shared = {...?runs.value?.buckets.map((b) => b.start)};
    return runTotal(
      p.rebuild((b) => b.buckets.removeWhere((x) => shared.contains(x.start))),
    );
  }
}

/// An entry of the scope selector the server refused, with its problem; [spaceId] null is
/// "All spaces".
final class StatsRemovedScope {
  /// Creates the entry.
  const StatsRemovedScope({
    required this.spaceId,
    required this.name,
    required this.problem,
  });

  /// The space, or null for the instance.
  final String? spaceId;

  /// The name of the space, empty for the instance.
  final String name;

  /// The server's problem, or null without an answer.
  final api.ApiProblem? problem;
}

/// Loads the statistics of A7 and keeps the choice of range, scope and currency.
///
/// [start] reads the spaces, the agent names and the telemetry link once and then [load]s.
/// Every load runs the queries of Design note 1 at once: the runs of the range and of the
/// previous range, the tool calls, the error rates, the usage in weeks and the costs by agent
/// and by space. When the runs of the chosen scope are refused (`403`, `404`), the entry leaves
/// the selector with a note and the next entry loads ("All spaces" first, then the spaces in
/// the server's order), so the server alone decides what a person sees (15 1 principle 1). A
/// cost query with `currency_required` offers the currencies it names. Answers of an older
/// load are dropped.
///
/// Example:
/// ```dart
/// final c = StatsController(source: repository, clock: DateTime.now);
/// await c.start();
/// c.data?.runs.value; // the runs of the last 7 days
/// ```
class StatsController extends ChangeNotifier {
  /// Creates the controller over [source]; [clock] gives the end of the preset ranges.
  StatsController({required this.source, DateTime Function()? clock})
    : clock = clock ?? DateTime.now {
    range = StatsRange.of(StatsPreset.week, now: this.clock(), days: days);
  }

  /// The calendar days of the server's `stats.time_zone` (UTC until [start] read it).
  StatsDays days = StatsDays(null);

  /// The statistics calls.
  final AdminStatsSource source;

  /// The current time.
  final DateTime Function() clock;

  /// The chosen range.
  late StatsRange range;

  /// The chosen space, or null for "All spaces".
  String? spaceId;

  /// Whether the selector still offers "All spaces".
  bool offersInstance = true;

  /// The spaces of the selector (`listSpaces` without the refused ones), or null while loading.
  List<StatsSpace>? spaces;

  /// The entries the server refused, in the order of the refusals.
  final List<StatsRemovedScope> removed = [];

  /// The display name per agent ID.
  Map<String, String> agentNames = const {};

  /// The telemetry link of `getInstance`, or null.
  Uri? telemetryUrl;

  /// The currency of the cost queries, or null for the server's choice.
  String? currency;

  /// The currencies a `currency_required` answer named; empty otherwise.
  List<String> currencies = const [];

  /// The answers of the last load, or null while loading.
  StatsData? data;

  /// The problem when no entry of the selector is left, or null.
  StatsRemovedScope? nothingLeft;

  bool _disposed = false;
  int _generation = 0;

  /// Reads the selector, the names and the link, then loads the default range.
  Future<void> start() async {
    final parts = await Future.wait<Object?>([
      source.statsSpaces().then<Object?>((s) => s, onError: (Object _) => null),
      source.statsAgentNames().then<Object?>(
        (n) => n,
        onError: (Object _) => null,
      ),
      source.statsInstance().then<Object?>(
        (i) => i,
        onError: (Object _) => null,
      ),
    ]);
    if (_disposed) return;
    // Without the list the selector offers "All spaces" only; the names fall back to the IDs;
    // without the instance view there is no link and days count in UTC.
    spaces = (parts[0] as List<StatsSpace>?) ?? const [];
    agentNames = (parts[1] as Map<String, String>?) ?? const {};
    final instance = parts[2] as StatsInstance?;
    telemetryUrl = instance?.telemetryDashboardUrl;
    days = StatsDays(instance?.timeZone);
    if (range.preset != StatsPreset.custom) {
      range = StatsRange.of(range.preset, now: clock(), days: days);
    }
    await load();
  }

  /// Chooses [next] and loads it.
  Future<void> setRange(StatsRange next) {
    range = next;
    return load();
  }

  /// Chooses [space] (null: "All spaces") and loads it; the currency choice starts over.
  Future<void> setScope(String? space) {
    spaceId = space;
    currency = null;
    currencies = const [];
    return load();
  }

  /// Chooses the currency of the cost queries and loads again.
  Future<void> setCurrency(String code) {
    currency = code;
    return load();
  }

  /// The name of [space] in the selector's list, else its ID.
  String spaceName(String space) =>
      spaces?.where((s) => s.id == space).firstOrNull?.name ?? space;

  /// Loads every query for the chosen range and scope.
  Future<void> load() async {
    final generation = ++_generation;
    data = null;
    nothingLeft = null;
    _notify();
    final query = StatsQuery(
      from: range.from,
      to: range.to,
      bucket: range.bucket,
      spaceId: spaceId,
    );
    final previous = range.previous;
    Future<StatsPart<T>> part<T extends Object>(Future<T> call) => call.then(
      StatsPart<T>.value,
      onError: (Object e) => StatsPart<T>.failed(e),
    );
    final runs = part(source.runStats(query));
    final runsPrevious = part(
      source.runStats(
        StatsQuery(
          from: previous.from,
          to: previous.to,
          bucket: query.bucket,
          spaceId: spaceId,
        ),
      ),
    );
    final tools = part(source.toolCallStats(query));
    final errors = part(source.errorStats(query));
    final usage = part(
      source.usageStats(query.withBucket(StatsBucketSize.week)),
    );
    final costsByAgent = part(
      source.costStats(
        query,
        groupBy: StatsCostGrouping.agent,
        currency: currency,
      ),
    );
    final costsBySpace = part(
      source.costStats(
        query,
        groupBy: StatsCostGrouping.space,
        currency: currency,
      ),
    );
    final loaded = StatsData(
      query: query,
      runs: await runs,
      runsPrevious: await runsPrevious,
      tools: await tools,
      errors: await errors,
      usage: await usage,
      costsByAgent: await costsByAgent,
      costsBySpace: await costsBySpace,
    );
    if (_disposed || generation != _generation) return;
    final status = loaded.runs.problem?.status;
    if (status == 403 || status == 404) {
      _refuse(loaded.runs.problem);
      return;
    }
    final named = requiredCurrencies(
      loaded.costsByAgent.error ?? loaded.costsBySpace.error ?? Object(),
    );
    if (named.isNotEmpty && currency == null) {
      // The server answers one currency at a time; the first named one loads, the selector
      // offers the others.
      currencies = named;
      currency = named.first;
      unawaited(load());
      return;
    }
    data = loaded;
    _notify();
  }

  /// Removes the chosen entry after the server refused it and loads the next one.
  void _refuse(api.ApiProblem? problem) {
    final space = spaceId;
    removed.add(
      StatsRemovedScope(
        spaceId: space,
        name: space == null ? '' : spaceName(space),
        problem: problem,
      ),
    );
    if (space == null) {
      offersInstance = false;
    } else {
      spaces = [
        for (final s in spaces ?? const <StatsSpace>[])
          if (s.id != space) s,
      ];
    }
    final next = (spaces ?? const <StatsSpace>[]).firstOrNull;
    if (next == null) {
      nothingLeft = removed.last;
      _notify();
      return;
    }
    spaceId = next.id;
    currency = null;
    currencies = const [];
    unawaited(load());
  }

  void _notify() {
    if (!_disposed) notifyListeners();
  }

  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }
}
