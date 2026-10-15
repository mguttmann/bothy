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
/// Run bars take the run state roles; the row label and its semantics name the row and the
/// number of runs ("dev-runner, #backend, 2 runs"), and every bar of a run carries its state as
/// text in the label of the row, so the state is never color alone.
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
                        height: DsSpace.s16,
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

  @override
  void paint(Canvas canvas, Size size) {
    canvas.drawLine(
      Offset(0, size.height / 2),
      Offset(size.width, size.height / 2),
      Paint()
        ..color = grid
        ..strokeWidth = DsBorderWidth.hairline,
    );
    final span = to.difference(from).inMilliseconds;
    if (span <= 0) return;
    double x(DateTime t) =>
        (size.width * t.difference(from).inMilliseconds / span).clamp(
          0,
          size.width,
        );
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
