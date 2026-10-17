// SPDX-License-Identifier: Apache-2.0

/// The bar mark: rows of bars over a time range (components.md 11).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import '../model/bar_model.dart';
import 'viz_colors.dart';
import 'viz_texts.dart';

/// One row per label with its bars between [BarModel.from] and [BarModel.to].
///
/// Run bars take the run state roles and start with the icon of their state, so the state is
/// never color alone; the semantics label of a row names the row, the number of runs and every
/// state as text ("dev-runner, #backend, 2 runs, Done, Failed").
class BarMark extends StatelessWidget {
  /// Creates the mark of [model].
  const BarMark({super.key, required this.model});

  /// The rows.
  final BarModel model;

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final row in model.rows)
          Semantics(
            container: true,
            label: [
              texts.l10n.vizBarRowLabel(row.label, row.segments.length),
              for (final s in row.segments)
                if (s.runState != null) texts.timelineState(s.runState!),
            ].join(', '),
            child: ExcludeSemantics(
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: DsSpace.s2),
                child: Row(
                  children: [
                    SizedBox(
                      width: DsLayout.columnMin,
                      child: Text(
                        row.label,
                        overflow: TextOverflow.ellipsis,
                        style: dsTextStyle(DsType.bodySm)
                            .copyWith(color: colors.textSecondary),
                      ),
                    ),
                    const SizedBox(width: DsSpace.s8),
                    Expanded(
                      child: SizedBox(
                        height: DsIconSize.sizeSm,
                        child: _BarTrack(model: model, row: row),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
      ],
    );
  }
}

/// The bars of one row with the state icon at the start of every run bar.
class _BarTrack extends StatelessWidget {
  const _BarTrack({required this.model, required this.row});

  final BarModel model;
  final BarRow row;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    return Stack(
      children: [
        Positioned.fill(
          child: CustomPaint(
            painter: BarPainter(
              from: model.from,
              to: model.to,
              segments: row.segments,
              colors: [
                for (final s in row.segments)
                  s.runState != null
                      ? runStateColor(colors, s.runState!)
                      : vizSeriesColor(colors, s.colorIndex!),
              ],
              grid: colors.vizGrid,
            ),
          ),
        ),
        for (final s in row.segments)
          if (s.runState != null)
            Positioned.fill(
              // Align places the icon at the start of its bar without a LayoutBuilder, which
              // would break the intrinsic sizing of tables and goldens.
              child: Align(
                alignment: Alignment(
                  2 * BarPainter.fractionOf(model.from, model.to, s.start) - 1,
                  0,
                ),
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    color: colors.surfaceRaised,
                    shape: BoxShape.circle,
                  ),
                  child: DsIcon(
                    runStateGlyph(s.runState!),
                    size: DsIconSizeRole.sm,
                    color: colors.textPrimary,
                    decorative: true,
                  ),
                ),
              ),
            ),
      ],
    );
  }
}

/// Paints the bars of one row on its track.
class BarPainter extends CustomPainter {
  /// Creates the painter; [colors] has one entry per segment.
  const BarPainter({
    required this.from,
    required this.to,
    required this.segments,
    required this.colors,
    required this.grid,
  });

  /// Start of the range.
  final DateTime from;

  /// End of the range.
  final DateTime to;

  /// The bars.
  final List<BarSegment> segments;

  /// Color per bar.
  final List<Color> colors;

  /// `color.viz.grid`, the track.
  final Color grid;

  /// The position of [t] between [from] (0) and [to] (1), clamped to that range.
  static double fractionOf(DateTime from, DateTime to, DateTime t) {
    final span = to.difference(from).inMilliseconds;
    if (span <= 0) return 0;
    return (t.difference(from).inMilliseconds / span).clamp(0, 1).toDouble();
  }

  @override
  void paint(Canvas canvas, Size size) {
    canvas.drawLine(
      Offset(0, size.height / 2),
      Offset(size.width, size.height / 2),
      Paint()
        ..color = grid
        ..strokeWidth = DsBorderWidth.hairline,
    );
    if (!to.isAfter(from)) return;
    double x(DateTime t) => size.width * fractionOf(from, to, t);
    for (var i = 0; i < segments.length; i++) {
      final s = segments[i];
      final left = x(s.start);
      final right = x(s.end);
      canvas.drawRRect(
        RRect.fromLTRBR(
          left,
          0,
          right <= left ? left + DsBorderWidth.spine : right,
          size.height,
          const Radius.circular(DsRadius.xs),
        ),
        Paint()..color = colors[i],
      );
    }
  }

  @override
  bool shouldRepaint(BarPainter oldDelegate) =>
      oldDelegate.segments != segments ||
      oldDelegate.from != from ||
      oldDelegate.to != to ||
      oldDelegate.grid != grid;
}
