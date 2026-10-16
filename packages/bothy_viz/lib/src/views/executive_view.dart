// SPDX-License-Identifier: Apache-2.0

/// The executive page (docs/spec/25-visualisierung.md 6.4).
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

/// The answers of the five queries of the executive page; a part the person may not see is
/// `null` with its flag.
final class ExecutiveData {
  /// Creates the data.
  const ExecutiveData({
    this.usage,
    this.runs,
    this.errors,
    this.costs,
    this.slo,
    this.sloOwnerOnly = false,
  });

  /// `getUsageStats`.
  final api.StatsSeries? usage;

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
}

/// The tiles of the executive page by tile (25 6.4), derived from [ExecutiveData].
final class ExecutiveTiles {
  /// Derives the tiles; [costsVisible] is `false` in TV mode without `viz.tv_show_costs`.
  factory ExecutiveTiles.of(ExecutiveData data, {required bool costsVisible}) {
    final usage = ExecutiveModel.from(usage: data.usage);
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

  /// Usage: messages, new threads, active people (minimum group applied by the server).
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
/// Reads `getUsageStats`, `getRunStats`, `getErrorStats`, `getCostStats` and `getStatsSlo`
/// once for the host's range. `forbidden` on `getStatsSlo` shows "Only for the owner" in the
/// traffic light tile; `forbidden` on `getCostStats` leaves the costs out (no
/// `cost.read.aggregate`); any other problem shows its text. Costs are never shown as zero.
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

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  static bool _forbidden(Object error) =>
      error is api.ApiProblem && error.code == 'forbidden';

  Future<void> _load() async {
    final q = widget.queries;
    final range = widget.options.range;
    final space = widget.options.scope.spaceId;
    var ownerOnly = false;
    try {
      // Future.wait listens to every query at once, so a failing one is never unhandled.
      final answers = await Future.wait<Object?>([
        q.usage(from: range.from, to: range.to, spaceId: space),
        q.runs(from: range.from, to: range.to, spaceId: space),
        q.errors(from: range.from, to: range.to, spaceId: space),
        q
            .costs(from: range.from, to: range.to, spaceId: space)
            .then<api.CostSeries?>(
              (c) => c,
              onError: (Object e) => _forbidden(e) ? null : throw e,
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
        runs: answers[1] as api.StatsSeries?,
        errors: answers[2] as api.StatsSeries?,
        costs: answers[3] as api.CostSeries?,
        slo: answers[4] as api.StatsSloReport?,
        sloOwnerOnly: ownerOnly,
      );
      if (mounted) setState(() => _data = data);
    } on Object catch (error) {
      if (mounted) setState(() => _error = vizProblemCode(error));
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = VizTexts.of(context).l10n;
    final data = _data;
    final tiles = data == null
        ? null
        : ExecutiveTiles.of(
            data,
            costsVisible: VizTvScope.costsVisible(context),
          );
    return VizViewScaffold(
      title: l10n.vizViewExecutive,
      options: widget.options,
      status: _error != null
          ? VizFailed(_error!)
          : tiles == null
          ? const VizLoading()
          : const VizReady(),
      tables: (texts) => [
        tileTable(texts, l10n.vizTileUsage, tiles!.usage),
        tileTable(texts, l10n.vizTileRuns, tiles.runs),
        if (tiles.costs case final costs?)
          tileTable(texts, l10n.vizTileCosts, costs),
        if (!tiles.sloOwnerOnly) trafficLightTable(texts, tiles.lights),
      ],
      chart: (context) => ExecutiveTileGroups(tiles: tiles!),
    );
  }
}

/// The tile groups of the executive page; the statistics screen of the administration (A7)
/// reuses them.
class ExecutiveTileGroups extends StatelessWidget {
  /// Creates the groups of [tiles].
  const ExecutiveTileGroups({super.key, required this.tiles});

  /// The tiles.
  final ExecutiveTiles tiles;

  @override
  Widget build(BuildContext context) {
    final l10n = VizTexts.of(context).l10n;
    final colors = context.dsColors;
    final costs = tiles.costs;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        _Group(
          title: l10n.vizTileUsage,
          children: [for (final t in tiles.usage) MetricTileMark(model: t)],
        ),
        _Group(
          title: l10n.vizTileRuns,
          children: [for (final t in tiles.runs) MetricTileMark(model: t)],
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
