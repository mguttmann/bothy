// SPDX-License-Identifier: Apache-2.0

/// The executive page (docs/spec/25-visualization.md 6.4).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as api;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import '../marks/metric_tile_mark.dart';
import '../marks/traffic_light_mark.dart';
import '../marks/viz_texts.dart';
import '../model/executive_model.dart';
import '../model/metric_tile_model.dart';
import '../model/traffic_light_model.dart';
import '../sources/live_frame_source.dart';
import '../sources/stats_query_source.dart';
import 'table/executive_table.dart';
import 'view_scaffold.dart';
import 'viz_host_options.dart';

/// The range the executive page asks for, from the [range] of the host: whole UTC hours, the
/// hour that holds the end of [range] and as many hours before it as [range] is long (rounded
/// up, at least one).
///
/// The server widens a range to whole buckets, `from` down and `to` up, and counts hour
/// buckets in UTC. A range on whole UTC hours is therefore answered as asked, and the page can
/// name the period of its tiles. Asked without a bucket size, the server takes days of its own
/// time zone and answers every day the range touches: two calendar days for the last 24 hours.
///
/// ```dart
/// executiveRange(VizRange(from: DateTime.utc(2026, 10, 14, 10, 30), to: DateTime.utc(2026, 10, 15, 10, 30)));
/// // 2026-10-14 11:00 UTC to 2026-10-15 11:00 UTC
/// ```
VizRange executiveRange(VizRange range) {
  final end = range.to.toUtc();
  final hour = DateTime.utc(end.year, end.month, end.day, end.hour);
  final to = hour == end ? hour : hour.add(const Duration(hours: 1));
  final hours =
      (range.to.difference(range.from).inMicroseconds /
              Duration.microsecondsPerHour)
          .ceil();
  return VizRange(
    from: to.subtract(Duration(hours: hours < 1 ? 1 : hours)),
    to: to,
  );
}

/// The answers of the queries of the executive page; a part the person may not see is `null`
/// with its flag.
final class ExecutiveData {
  /// Creates the data.
  const ExecutiveData({
    this.usage,
    this.people,
    this.runs,
    this.errors,
    this.costs,
    this.slo,
    this.sloOwnerOnly = false,
    this.costsOneCurrency = false,
  });

  /// `getUsageStats`.
  final api.StatsSeries? usage;

  /// `getUsageStats` in weeks, for `active_people`: the server counts people per week or
  /// month only.
  final api.StatsSeries? people;

  /// `getRunStats`.
  final api.StatsSeries? runs;

  /// `getErrorStats`.
  final api.StatsSeries? errors;

  /// `getCostStats`; `null` without `cost.read.aggregate` (not shown).
  final api.CostSeries? costs;

  /// `getStatsSlo`; `null` when [sloOwnerOnly].
  final api.StatsSloReport? slo;

  /// The server answered `forbidden` for the traffic light: a space manager has none, the
  /// tile says "Only for the owner".
  final bool sloOwnerOnly;

  /// The range has costs in several currencies; [costs] holds the first one the server named.
  final bool costsOneCurrency;
}

/// The tiles of the executive page by tile (25 6.4), derived from [ExecutiveData].
final class ExecutiveTiles {
  /// Derives the tiles; [costsVisible] is `false` in TV mode without `viz.tv_show_costs`.
  factory ExecutiveTiles.of(ExecutiveData data, {required bool costsVisible}) {
    final usage = ExecutiveModel.from(usage: data.usage, people: data.people);
    final seen = {for (final t in usage.tiles) t.queryMetric};
    final runs = ExecutiveModel.from(runs: data.runs, errors: data.errors);
    final costs = costsVisible ? ExecutiveModel.from(costs: data.costs) : null;
    final slo = ExecutiveModel.from(slo: data.slo);
    return ExecutiveTiles._(
      usage: usage.tiles,
      runs: [
        for (final t in runs.tiles)
          if (!seen.contains(t.queryMetric)) t,
      ],
      costs: costs?.costTiles,
      lights: slo.trafficLights,
      sloOwnerOnly: data.sloOwnerOnly,
    );
  }

  const ExecutiveTiles._({
    required this.usage,
    required this.runs,
    required this.costs,
    required this.lights,
    required this.sloOwnerOnly,
  });

  /// Usage: messages, new threads and, when the answer in weeks has one space, the active
  /// people of the current week (minimum group applied by the server).
  final List<MetricTileModel> usage;

  /// Runs and error rate.
  final List<MetricTileModel> runs;

  /// Costs per mode; `null` when not shown.
  final List<MetricTileModel>? costs;

  /// SLO traffic lights.
  final List<TrafficLightModel> lights;

  /// The traffic light is only for the owner.
  final bool sloOwnerOnly;
}

/// Tiles for usage, runs and error rate, costs per mode with "Estimate" or "information
/// only", and the SLO traffic light "own measurement" with its note.
///
/// Reads `getUsageStats`, `getRunStats`, `getErrorStats` and `getCostStats` once in hours for
/// [executiveRange] of the host's range, and names that period under its title: a count is
/// its sum over the period, a level (a rate, a percentile) the value of its latest hour with
/// "Since: ...". `getUsageStats` is read a second time in weeks for the active people of the
/// current week, and `getStatsSlo` once. `forbidden` on `getStatsSlo` shows "Only for the
/// owner" in the traffic light tile; `forbidden` on `getCostStats` leaves the costs out (no
/// `cost.read.aggregate`); `currency_required` on `getCostStats` shows the costs of the first
/// currency the server names with a note; any other problem shows its text. Costs are never
/// shown as zero.
class ExecutiveView extends StatefulWidget {
  /// Creates the view.
  const ExecutiveView({
    super.key,
    required this.live,
    required this.queries,
    required this.options,
    required this.onOpenThread,
  });

  /// Not used by this view; the view contract is the same for all six views.
  final LiveFrameSource? live;

  /// The REST queries.
  final StatsQuerySource queries;

  /// What the host knows.
  final VizHostOptions options;

  /// Not used by this view; the view contract is the same for all six views.
  final void Function(String threadId) onOpenThread;

  @override
  State<ExecutiveView> createState() => _ExecutiveViewState();
}

class _ExecutiveViewState extends State<ExecutiveView> {
  ExecutiveData? _data;
  String? _error;

  /// The period of the tiles: what the page asks for is what the server answers.
  late final VizRange _period = executiveRange(widget.options.range);

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  static bool _forbidden(Object error) =>
      error is api.ApiProblem && error.code == 'forbidden';

  Future<void> _load() async {
    final q = widget.queries;
    final VizRange(:from, :to) = _period;
    final space = widget.options.scope.spaceId;
    const hour = api.StatsGranularity.hour;
    var ownerOnly = false;
    var oneCurrency = false;
    Future<api.CostSeries> costs({String? currency}) => q.costs(
      from: from,
      to: to,
      granularity: hour,
      spaceId: space,
      currency: currency,
    );
    try {
      // Future.wait listens to every query at once, so a failing one is never unhandled.
      final answers = await Future.wait<Object?>([
        q.usage(from: from, to: to, granularity: hour, spaceId: space),
        q.usage(
          from: from,
          to: to,
          granularity: api.StatsGranularity.week,
          spaceId: space,
        ),
        q.runs(from: from, to: to, granularity: hour, spaceId: space),
        q.errors(from: from, to: to, granularity: hour, spaceId: space),
        costs().then<api.CostSeries?>(
          (c) => c,
          onError: (Object e) {
            if (_forbidden(e)) return null;
            // The server answers one currency at a time: the first it names is shown.
            final named = vizRequiredCurrencies(e);
            if (named.isEmpty) throw e;
            oneCurrency = true;
            return costs(currency: named.first);
          },
        ),
        q.slo().then<api.StatsSloReport?>(
          (s) => s,
          onError: (Object e) {
            if (!_forbidden(e)) throw e;
            ownerOnly = true;
            return null;
          },
        ),
      ]);
      final data = ExecutiveData(
        usage: answers[0] as api.StatsSeries?,
        people: answers[1] as api.StatsSeries?,
        runs: answers[2] as api.StatsSeries?,
        errors: answers[3] as api.StatsSeries?,
        costs: answers[4] as api.CostSeries?,
        slo: answers[5] as api.StatsSloReport?,
        sloOwnerOnly: ownerOnly,
        costsOneCurrency: oneCurrency,
      );
      if (mounted) setState(() => _data = data);
    } on Object catch (error) {
      if (mounted) setState(() => _error = vizProblemCode(error));
    }
  }

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final l10n = texts.l10n;
    final data = _data;
    final costsVisible = VizTvScope.costsVisible(context);
    final tiles = data == null
        ? null
        : ExecutiveTiles.of(data, costsVisible: costsVisible);
    final now = widget.options.clock.now();
    final zone = widget.options.timeZone;
    return VizViewScaffold(
      title: l10n.vizViewExecutive,
      options: widget.options,
      status: _error != null
          ? VizFailed(_error!)
          : tiles == null
          ? const VizLoading()
          : const VizReady(),
      subtitle: l10n.statsRangeCustomValue(
        texts.dayTime(_period.from, now, zone),
        texts.dayTime(_period.to, now, zone),
      ),
      notes: [
        if (costsVisible && (data?.costsOneCurrency ?? false))
          (tone: DsTone.info, text: l10n.statsCurrencyRequired),
      ],
      tables: (texts) => [
        tileTable(
          texts,
          l10n.vizTileUsage,
          tiles!.usage,
          now: now,
          timeZone: zone,
        ),
        tileTable(
          texts,
          l10n.vizTileRuns,
          tiles.runs,
          now: now,
          timeZone: zone,
        ),
        if (tiles.costs case final costs?)
          tileTable(texts, l10n.vizTileCosts, costs),
        if (!tiles.sloOwnerOnly) trafficLightTable(texts, tiles.lights),
      ],
      chart: (context) =>
          ExecutiveTileGroups(tiles: tiles!, now: now, timeZone: zone),
    );
  }
}

/// The tile groups of the executive page; the statistics screen of the administration (A7)
/// reuses them.
class ExecutiveTileGroups extends StatelessWidget {
  /// Creates the groups of [tiles]; [now] and [timeZone] format the "Since: ..." of a level
  /// ([MetricTileMark.now]).
  const ExecutiveTileGroups({
    super.key,
    required this.tiles,
    this.now,
    this.timeZone = 'UTC',
  });

  /// The tiles.
  final ExecutiveTiles tiles;

  /// The current time, for the relative day of "Since: ..."; `null` leaves the note out.
  final DateTime? now;

  /// IANA time zone of the clock time of "Since: ...".
  final String timeZone;

  @override
  Widget build(BuildContext context) {
    final l10n = VizTexts.of(context).l10n;
    final colors = context.dsColors;
    final costs = tiles.costs;
    Widget tile(MetricTileModel t) =>
        MetricTileMark(model: t, now: now, timeZone: timeZone);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        _Group(
          title: l10n.vizTileUsage,
          children: [for (final t in tiles.usage) tile(t)],
        ),
        _Group(
          title: l10n.vizTileRuns,
          children: [for (final t in tiles.runs) tile(t)],
        ),
        if (costs != null)
          _Group(
            title: l10n.vizTileCosts,
            children: [for (final t in costs) MetricTileMark(model: t)],
          ),
        _Group(
          title: l10n.vizTileSlo,
          footer: tiles.sloOwnerOnly
              ? null
              : Text(
                  l10n.vizOwnMeasurementNote,
                  style: dsTextStyle(DsType.caption)
                      .copyWith(color: colors.textSecondary),
                ),
          banner: tiles.sloOwnerOnly ? l10n.vizOnlyForOwner : null,
          children: [
            if (!tiles.sloOwnerOnly)
              for (final light in tiles.lights) TrafficLightMark(model: light),
          ],
        ),
      ],
    );
  }
}

class _Group extends StatelessWidget {
  const _Group({
    required this.title,
    required this.children,
    this.footer,
    this.banner,
  });

  final String title;
  final List<Widget> children;
  final Widget? footer;

  /// A note instead of the tiles, outside the wrap so it takes the full width.
  final String? banner;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: DsSpace.s16),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            title,
            style: dsTextStyle(DsType.heading)
                .copyWith(color: context.dsColors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        if (banner != null) DsBanner(tone: DsTone.info, text: banner!),
        Wrap(spacing: DsSpace.s8, runSpacing: DsSpace.s8, children: children),
        if (footer != null) ...[const SizedBox(height: DsSpace.s4), footer!],
      ],
    ),
  );
}
