// SPDX-License-Identifier: Apache-2.0

/// Delivery and sync live (docs/spec/25-visualization.md 6.3).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import '../marks/area_mark.dart';
import '../marks/viz_texts.dart';
import '../model/delivery_model.dart';
import '../model/series_model.dart';
import '../model/timeline_model.dart';
import '../sources/live_frame_source.dart';
import '../sources/stats_query_source.dart';
import 'table/delivery_table.dart';
import 'view_scaffold.dart';
import 'view_texts.dart';
import 'viz_host_options.dart';

/// Deliveries and their share within the threshold, reconnecting, catch-ups with their
/// duration, `resync_required` per reason, closes of slow clients, push acceptance and, with
/// one node, the open connections; below them the history of these metrics.
///
/// Instance scope only (owner): in a space scope the view says "Only for the owner" and
/// never calls a source (design note 1). Live values come from `stats.live`, the history
/// once from `getStatsTimeline` for the host's range.
class DeliveryView extends StatefulWidget {
  /// Creates the view.
  const DeliveryView({
    super.key,
    required this.live,
    required this.queries,
    required this.options,
    required this.onOpenThread,
  });

  /// The live stream; `null` reads one frame through [queries].
  final LiveFrameSource? live;

  /// The REST queries (`getStatsTimeline`, `getStatsLive`).
  final StatsQuerySource queries;

  /// What the host knows.
  final VizHostOptions options;

  /// Not used by this view; the view contract is the same for all six views.
  final void Function(String threadId) onOpenThread;

  @override
  State<DeliveryView> createState() => _DeliveryViewState();
}

class _DeliveryViewState extends State<DeliveryView> with VizLiveSubscription {
  DeliveryModel? _model;
  List<SeriesModel>? _history;
  String? _queryError;
  int _generation = 0;

  bool get _ownerOnly => !widget.options.scope.isInstance;

  @override
  void initState() {
    super.initState();
    _start();
  }

  void _start() {
    if (_ownerOnly) return;
    unawaited(_loadHistory());
    final live = widget.live;
    if (live != null) {
      startLive(live, widget.options.scope);
    } else {
      unawaited(_loadFrame());
    }
  }

  @override
  void didUpdateWidget(DeliveryView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!vizInputsChanged(
      oldLive: oldWidget.live,
      live: widget.live,
      oldQueries: oldWidget.queries,
      queries: widget.queries,
      oldOptions: oldWidget.options,
      options: widget.options,
    )) {
      return;
    }
    _generation++;
    _model = null;
    _history = null;
    _queryError = null;
    restartLive(null, widget.options.scope);
    _start();
  }

  Future<void> _loadHistory() async {
    final generation = _generation;
    final range = widget.options.range;
    try {
      final timeline = await widget.queries.timeline(
        from: range.from,
        to: range.to,
        metrics: deliveryHistoryMetrics,
      );
      if (!mounted || generation != _generation) return;
      setState(() {
        _history = [
          for (final s in TimelineModel.fromTimeline(timeline).series)
            if (deliveryHistoryMetrics.contains(s.metric)) s,
        ];
      });
    } on Object catch (error, stack) {
      _fail(error, stack, generation);
    }
  }

  Future<void> _loadFrame() async {
    final generation = _generation;
    try {
      final frame = await widget.queries.live();
      if (mounted && generation == _generation) {
        setState(() => _model = DeliveryModel.fromFrame(frame));
      }
    } on Object catch (error, stack) {
      _fail(error, stack, generation);
    }
  }

  void _fail(Object error, StackTrace stack, int generation) {
    if (!mounted || generation != _generation) return;
    setState(() => _queryError ??= vizProblemCode(error, stack));
    unawaited(stopLive());
  }

  @override
  void onLiveChanged() {
    final frame = liveFrame;
    if (frame != null) _model = DeliveryModel.fromFrame(frame.frame);
  }

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final l10n = texts.l10n;
    final model = _model;
    final history = _history;
    final error = _queryError ?? liveError;
    return VizViewScaffold(
      title: l10n.vizViewDelivery,
      options: widget.options,
      status: _ownerOnly
          ? const VizOwnerOnly()
          : error != null
          ? VizFailed(error)
          : model == null || history == null
          ? const VizLoading()
          : const VizReady(),
      subtitle: model == null || _ownerOnly
          ? null
          : l10n.vizFrameTime(
              vizClockTime(texts, model.at, widget.options.timeZone),
            ),
      notes: vizLiveNotes(l10n, stale: liveStale),
      tables: (texts) => deliveryTables(texts, model!, history!),
      chart: (context) {
        final colors = context.dsColors;
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            Wrap(
              spacing: DsSpace.s8,
              runSpacing: DsSpace.s8,
              children: [
                for (final line in deliveryLines(texts, model!))
                  Semantics(
                    container: true,
                    label: '${line.label}: ${line.value}',
                    child: ExcludeSemantics(
                      child: Container(
                        constraints: const BoxConstraints(
                          minWidth: DsLayout.columnMin,
                        ),
                        padding: const EdgeInsets.all(DsSpace.s12),
                        decoration: BoxDecoration(
                          color: colors.surfaceRaised,
                          borderRadius: BorderRadius.circular(DsRadius.lg),
                          border: Border.all(
                            color: colors.borderSubtle,
                            width: DsBorderWidth.hairline,
                          ),
                        ),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Text(
                              line.label,
                              style: dsTextStyle(DsType.label)
                                  .copyWith(color: colors.textSecondary),
                            ),
                            Text(
                              line.value,
                              style: dsTextStyle(DsType.heading)
                                  .copyWith(color: colors.textPrimary),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
              ],
            ),
            if (history!.isNotEmpty) ...[
              const SizedBox(height: DsSpace.s16),
              AreaMark(title: l10n.vizChartHistory, series: history),
            ],
          ],
        );
      },
    );
  }
}
