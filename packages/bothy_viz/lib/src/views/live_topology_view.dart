// SPDX-License-Identifier: Apache-2.0

/// The live topology (docs/spec/25-visualization.md 6.1; screen D12).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import '../marks/lane_mark.dart';
import '../marks/task_tree_mark.dart';
import '../marks/viz_texts.dart';
import '../model/live_topology_model.dart';
import '../model/run_card_model.dart';
import '../model/series_model.dart';
import '../sources/live_frame_source.dart';
import '../sources/stats_query_source.dart';
import 'table/live_topology_table.dart';
import 'view_scaffold.dart';
import 'view_texts.dart';
import 'viz_host_options.dart';

/// Lanes per space and channel with a card per active run, the rates of the last minute,
/// the time of the frame, the stale marker and "More runs than shown".
///
/// Reads `stats.live` through [live]; without a live source (a host without stream) it reads
/// the frame once through `getStatsLive` of [queries]. A tap on a card reports its thread to
/// [onOpenThread]. Costs appear only where the frame carries them and, in TV mode, only with
/// `viz.tv_show_costs`. An unchanged lane keeps its widget between frames and paints in its
/// own layer, so a frame rebuilds only the changed lanes (25 7.3).
class LiveTopologyView extends StatefulWidget {
  /// Creates the view.
  const LiveTopologyView({
    super.key,
    required this.live,
    required this.queries,
    required this.options,
    required this.onOpenThread,
  });

  /// The live stream; `null` reads one frame through [queries].
  final LiveFrameSource? live;

  /// The REST queries.
  final StatsQuerySource queries;

  /// What the host knows.
  final VizHostOptions options;

  /// Opens the thread of a tapped run card.
  final void Function(String threadId) onOpenThread;

  @override
  State<LiveTopologyView> createState() => _LiveTopologyViewState();
}

class _LiveTopologyViewState extends State<LiveTopologyView>
    with VizLiveSubscription {
  LiveTopologyModel? _model;
  String? _queryError;
  final Map<(String, String), (LaneModel, Widget)> _lanes = {};
  final Map<String, (RunCardModel, Widget)> _cards = {};

  @override
  void initState() {
    super.initState();
    final live = widget.live;
    if (live != null) {
      startLive(live, widget.options.scope);
    } else {
      unawaited(_loadOnce());
    }
  }

  Future<void> _loadOnce() async {
    try {
      final frame = await widget.queries.live(
        spaceId: widget.options.scope.spaceId,
      );
      if (!mounted) return;
      setState(
        () => _model = LiveTopologyModel.fromFrame(
          frame,
          names: widget.options.names,
        ),
      );
    } on Object catch (error) {
      if (mounted) setState(() => _queryError = vizProblemCode(error));
    }
  }

  @override
  void onLiveChanged() {
    final frame = liveFrame;
    if (frame != null) {
      _model = LiveTopologyModel.fromFrame(
        frame.frame,
        names: widget.options.names,
      );
    }
  }

  @override
  void didUpdateWidget(LiveTopologyView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.onOpenThread != widget.onOpenThread) {
      _lanes.clear();
      _cards.clear();
    }
  }

  void _open(String threadId) => widget.onOpenThread(threadId);

  /// The widget of [card]: the one of the previous frame while the card is unchanged, so
  /// the element skips its rebuild.
  Widget _card(RunCardModel card) {
    final cached = _cards[card.runId];
    if (cached != null && cached.$1 == card) return cached.$2;
    final built = RunCardTreeMark(
      key: ValueKey(card.runId),
      model: card,
      onTap: _open,
    );
    _cards[card.runId] = (card, built);
    return built;
  }

  /// The widget of [lane]: the cached one while the lane is unchanged; a changed lane keeps
  /// the widgets of its unchanged cards.
  Widget _lane(LaneModel lane) {
    final key = (lane.spaceId, lane.channelId);
    final cached = _lanes[key];
    if (cached != null && cached.$1 == lane) return cached.$2;
    final built = RepaintBoundary(
      key: ValueKey(key),
      child: LaneMark(model: lane, onTapRun: _open, cardBuilder: _card),
    );
    _lanes[key] = (lane, built);
    return built;
  }

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final l10n = texts.l10n;
    final model = _model;
    final error = liveError ?? _queryError;
    final costs = VizTvScope.costsVisible(context);
    final lanes = model == null
        ? const <LaneModel>[]
        : [for (final l in model.lanes) costs ? l : l.withoutCosts()];
    final rates = model == null
        ? const <RateModel>[]
        : [
            for (final r in model.rates)
              if (vizRateShown(r, costsVisible: costs)) r,
          ];
    final keys = {for (final l in lanes) (l.spaceId, l.channelId)};
    _lanes.removeWhere((k, _) => !keys.contains(k));
    final runs = {for (final l in lanes) ...l.cards.map((c) => c.runId)};
    _cards.removeWhere((k, _) => !runs.contains(k));
    return VizViewScaffold(
      title: l10n.vizViewLiveTopology,
      options: widget.options,
      status: error != null
          ? VizFailed(error)
          : model == null
          ? const VizLoading()
          : const VizReady(),
      subtitle: model == null
          ? null
          : l10n.vizFrameTime(
              vizClockTime(texts, model.at, widget.options.timeZone),
            ),
      notes: vizLiveNotes(
        l10n,
        stale: liveStale,
        truncated: model?.runsTruncated ?? false,
      ),
      tables: (texts) => liveTopologyTables(texts, lanes: lanes, rates: rates),
      chart: (context) => Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          if (lanes.isEmpty)
            Text(
              l10n.vizNoActiveRuns,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: context.dsColors.textSecondary),
            ),
          for (final lane in lanes)
            Padding(
              padding: const EdgeInsets.only(bottom: DsSpace.s8),
              child: _lane(lane),
            ),
          const SizedBox(height: DsSpace.s8),
          _Rates(rates: rates),
        ],
      ),
    );
  }
}

/// The rates of the last minute as one line of labelled values.
class _Rates extends StatelessWidget {
  const _Rates({required this.rates});

  final List<RateModel> rates;

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    final items = [
      for (final r in rates)
        '${vizRateLabel(texts, r)}: ${vizRateValue(texts, r)}',
    ];
    return Semantics(
      container: true,
      label: [texts.l10n.vizRatesLastMinute, ...items].join(', '),
      child: ExcludeSemantics(
        child: Wrap(
          spacing: DsSpace.s16,
          runSpacing: DsSpace.s4,
          children: [
            Text(
              texts.l10n.vizRatesLastMinute,
              style: dsTextStyle(DsType.label)
                  .copyWith(color: colors.textPrimary),
            ),
            for (final item in items)
              Text(
                item,
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: colors.textSecondary),
              ),
          ],
        ),
      ),
    );
  }
}
