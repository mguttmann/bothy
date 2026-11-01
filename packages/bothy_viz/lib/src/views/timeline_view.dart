// SPDX-License-Identifier: Apache-2.0

/// The replayable daily timeline (docs/spec/25-visualization.md 6.5).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as api;
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';

import '../marks/area_mark.dart';
import '../marks/bar_mark.dart';
import '../marks/viz_texts.dart';
import '../model/timeline_model.dart';
import '../sources/live_frame_source.dart';
import '../sources/stats_query_source.dart';
import 'table/timeline_table.dart';
import 'timeline/timeline_playback.dart';
import 'view_scaffold.dart';
import 'view_texts.dart';
import 'viz_host_options.dart';

/// The rates as areas and the runs as bars per chat and agent, played back minute by minute
/// with play, pause, a jump slider and the speeds of `viz.timeline_speeds`. A tap on a row whose
/// chat the server named reports its thread to [onOpenThread].
///
/// Reads `getStatsTimeline` once for the host's range and plays locally on the clock of the
/// options. Day boundaries come from the response (the server applies `stats.time_zone`);
/// `runs_truncated` shows "More runs than shown". In the reduced motion context the
/// playhead jumps from minute to minute without a transition. In TV mode there are no
/// controls: the playhead stays at the end, so the screen shows the whole period.
class TimelineView extends StatefulWidget {
  /// Creates the view.
  const TimelineView({
    super.key,
    required this.live,
    required this.queries,
    required this.options,
    required this.onOpenThread,
  });

  /// Not used by this view; the view contract is the same for all six views.
  final LiveFrameSource? live;

  /// The REST queries (`getStatsTimeline`).
  final StatsQuerySource queries;

  /// What the host knows (range, clock, `viz.timeline_speeds`).
  final VizHostOptions options;

  /// Opens the chat of a tapped row (and of its table row).
  final void Function(String threadId) onOpenThread;

  @override
  State<TimelineView> createState() => _TimelineViewState();
}

class _TimelineViewState extends State<TimelineView> {
  TimelineModel? _model;
  TimelinePlayback? _playback;
  String? _error;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  Future<void> _load() async {
    final range = widget.options.range;
    try {
      final timeline = await widget.queries.timeline(
        from: range.from,
        to: range.to,
        spaceId: widget.options.scope.spaceId,
      );
      if (!mounted) return;
      final all = TimelineModel.fromTimeline(
        timeline,
        names: widget.options.names,
      );
      // The `cost` buckets count bookings; a count is no amount and has neither currency nor
      // label, so the rates of the timeline leave them out (25 4 rule 2; D-M6-12-8).
      final model = TimelineModel(
        from: all.from,
        to: all.to,
        bars: all.bars,
        runsTruncated: all.runsTruncated,
        series: [
          for (final s in all.series)
            if (s.metric != api.StatsMetric.cost) s,
        ],
      );
      final playback = TimelinePlayback(
        from: model.from,
        to: model.to,
        clock: widget.options.clock,
        speeds: TimelinePlayback.parseSpeeds(
          widget.options.config.timelineSpeeds,
        ),
      )..addListener(_changed);
      setState(() {
        _model = model;
        _playback = playback;
      });
    } on Object catch (error) {
      if (mounted) setState(() => _error = vizProblemCode(error));
    }
  }

  void _changed() {
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    _playback?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final l10n = texts.l10n;
    final model = _model;
    final playback = _playback;
    final now = widget.options.clock.now();
    final zone = widget.options.timeZone;
    final shown = model == null || playback == null
        ? null
        : timelineAt(model, playback.position);
    return VizViewScaffold(
      title: l10n.vizViewTimeline,
      options: widget.options,
      status: _error != null
          ? VizFailed(_error!)
          : shown == null
          ? const VizLoading()
          : const VizReady(),
      subtitle: playback == null
          ? null
          : l10n.vizPlayhead(vizDayTime(texts, playback.position, now, zone)),
      notes: [
        if (model?.runsTruncated ?? false)
          (tone: DsTone.info, text: l10n.vizMoreRunsThanShown),
      ],
      controls: playback == null ? null : _Controls(playback: playback),
      tables: (texts) => timelineTables(
        texts,
        shown!,
        now: now,
        timeZone: zone,
        onOpenThread: widget.onOpenThread,
      ),
      chart: (context) => Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          AreaMark(title: l10n.vizChartRates, series: shown!.series),
          const SizedBox(height: DsSpace.s16),
          Text(
            l10n.vizChartRuns,
            style: dsTextStyle(DsType.heading)
                .copyWith(color: context.dsColors.textPrimary),
          ),
          const SizedBox(height: DsSpace.s8),
          _Playhead(model: model!, at: playback!.position),
          BarMark(model: shown.bars, onTapRow: widget.onOpenThread),
        ],
      ),
    );
  }
}

/// The playhead as a vertical mark over the bar track; it moves with `motion.*` and jumps
/// in the reduced context.
class _Playhead extends StatelessWidget {
  const _Playhead({required this.model, required this.at});

  final TimelineModel model;
  final DateTime at;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    final motion = context.dsMotion;
    final x = BarPainter.fractionOf(model.from, model.to, at);
    return ExcludeSemantics(
      child: Row(
        children: [
          const SizedBox(width: DsLayout.columnMin + DsSpace.s8),
          Expanded(
            child: SizedBox(
              height: DsSpace.s8,
              child: AnimatedAlign(
                alignment: Alignment(2 * x - 1, 0),
                duration: motion.durationMove,
                curve: motion.easingStandard,
                child: SizedBox(
                  width: DsBorderWidth.focus,
                  child: ColoredBox(color: colors.vizAxis),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Play or pause, the jump slider and the speeds.
class _Controls extends StatelessWidget {
  const _Controls({required this.playback});

  final TimelinePlayback playback;

  @override
  Widget build(BuildContext context) {
    final l10n = VizTexts.of(context).l10n;
    final minutes = playback.minutes;
    final index = playback.position.difference(playback.start).inMinutes;
    return Wrap(
      spacing: DsSpace.s8,
      runSpacing: DsSpace.s8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        FilledButton(
          key: const ValueKey('viz-timeline-play'),
          style: vizButtonStyle(context),
          onPressed: playback.playing ? playback.pause : playback.play,
          child: Text(playback.playing ? l10n.vizPause : l10n.vizPlay),
        ),
        for (final speed in playback.speeds)
          VizToggleButton(
            key: ValueKey('viz-timeline-speed-$speed'),
            label: l10n.vizSpeed(speed),
            selected: playback.speed == speed,
            onPressed: () => playback.setSpeed(speed),
          ),
        if (minutes > 0)
          SizedBox(
            width: DsLayout.listWidth,
            child: Slider(
              key: const ValueKey('viz-timeline-jump'),
              label: l10n.vizJump,
              semanticFormatterCallback: (_) => l10n.vizJump,
              value: index.clamp(0, minutes).toDouble(),
              max: minutes.toDouble(),
              divisions: minutes,
              onChanged: (v) => playback.jumpTo(
                playback.start.add(Duration(minutes: v.round())),
              ),
            ),
          ),
      ],
    );
  }
}
