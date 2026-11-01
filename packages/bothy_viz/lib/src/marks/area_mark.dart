// SPDX-License-Identifier: Apache-2.0

/// The area mark: series over time (components.md 11).
library;

import 'dart:math' as math;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import '../model/series_model.dart';
import 'viz_colors.dart';
import 'viz_texts.dart';

/// Series as lines with a light area below, on a grid, with a legend of label and last value
/// (one line per series, wrapping at large text sizes).
///
/// The colors follow the fixed order of the series roles; the legend repeats each series as
/// text, so no series is told apart by color alone (25 7.4). The semantics label names the
/// [title] and every series with its last value.
class AreaMark extends StatelessWidget {
  /// Creates the mark for [series].
  const AreaMark({super.key, required this.title, required this.series});

  /// Heading of the chart, for example "Rates".
  final String title;

  /// The series in their order.
  final List<SeriesModel> series;

  /// The label of [s]: the metric of the own measurement or the query metric name, with its
  /// dimension and its group, so two series never differ by color alone.
  static String seriesLabel(VizTexts texts, SeriesModel s) {
    final withDimension = s.metric != null
        ? texts.metricLabel(s.metric!, s.dimension)
        : texts.queryMetric(s.queryMetric!);
    final group = s.groupLabel ?? s.groupKey;
    return group == null ? withDimension : '$withDimension, $group';
  }

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    final legend = [
      for (final s in series)
        (
          color: vizSeriesColor(colors, s.colorIndex),
          text:
              '${seriesLabel(texts, s)}: ${s.last == null ? texts.l10n.vizNotShown : texts.format.count(s.last!.round())}',
        ),
    ];
    return Semantics(
      container: true,
      label: [title, for (final item in legend) item.text].join(', '),
      child: ExcludeSemantics(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              title,
              style: dsTextStyle(DsType.heading)
                  .copyWith(color: colors.textPrimary),
            ),
            const SizedBox(height: DsSpace.s8),
            SizedBox(
              height: DsSpace.s64 * 2,
              width: double.infinity,
              child: CustomPaint(
                painter: AreaPainter(
                  series: series,
                  colors: [for (final item in legend) item.color],
                  grid: colors.vizGrid,
                  axis: colors.vizAxis,
                ),
              ),
            ),
            const SizedBox(height: DsSpace.s8),
            for (final item in legend)
              Padding(
                padding: const EdgeInsets.only(bottom: DsSpace.s4),
                child: Row(
                  children: [
                    SizedBox.square(
                      dimension: DsSize.statusDot,
                      child: DecoratedBox(
                        decoration: BoxDecoration(
                          color: item.color,
                          shape: BoxShape.circle,
                        ),
                      ),
                    ),
                    const SizedBox(width: DsSpace.s4),
                    Flexible(
                      child: Text(
                        item.text,
                        style: dsTextStyle(DsType.bodySm)
                            .copyWith(color: colors.textSecondary),
                      ),
                    ),
                  ],
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// Paints the grid, the axis and every series as line and area.
class AreaPainter extends CustomPainter {
  /// Creates the painter; [colors] has one entry per series.
  const AreaPainter({
    required this.series,
    required this.colors,
    required this.grid,
    required this.axis,
  });

  /// The series.
  final List<SeriesModel> series;

  /// Series role per series.
  final List<Color> colors;

  /// `color.viz.grid`.
  final Color grid;

  /// `color.viz.axis`.
  final Color axis;

  /// Horizontal grid lines.
  static const int gridLines = 4;

  /// Opacity of the area below a line, so overlapping areas stay readable.
  static const double areaOpacity = 0.16;

  @override
  void paint(Canvas canvas, Size size) {
    final gridPaint = Paint()
      ..color = grid
      ..strokeWidth = DsBorderWidth.hairline;
    for (var i = 0; i < gridLines; i++) {
      final y = size.height * i / gridLines;
      canvas.drawLine(Offset(0, y), Offset(size.width, y), gridPaint);
    }
    canvas.drawLine(
      Offset(0, size.height),
      Offset(size.width, size.height),
      Paint()
        ..color = axis
        ..strokeWidth = DsBorderWidth.hairline,
    );
    final points = series.expand((s) => s.points).toList();
    if (points.isEmpty) return;
    var minT = points.first.at.millisecondsSinceEpoch;
    var maxT = minT;
    var maxV = 0.0;
    for (final p in points) {
      minT = math.min(minT, p.at.millisecondsSinceEpoch);
      maxT = math.max(maxT, p.at.millisecondsSinceEpoch);
      maxV = math.max(maxV, p.value);
    }
    final spanT = math.max(1, maxT - minT);
    final spanV = maxV <= 0 ? 1.0 : maxV;
    Offset at(SeriesPoint p) => Offset(
      size.width * (p.at.millisecondsSinceEpoch - minT) / spanT,
      size.height - size.height * p.value / spanV,
    );
    for (var i = 0; i < series.length; i++) {
      final s = series[i];
      if (s.points.isEmpty) continue;
      final line = Path()..moveTo(at(s.points.first).dx, at(s.points.first).dy);
      for (final p in s.points.skip(1)) {
        line.lineTo(at(p).dx, at(p).dy);
      }
      final area = Path.from(line)
        ..lineTo(at(s.points.last).dx, size.height)
        ..lineTo(at(s.points.first).dx, size.height)
        ..close();
      canvas
        ..drawPath(
          area,
          Paint()..color = colors[i].withValues(alpha: areaOpacity),
        )
        ..drawPath(
          line,
          Paint()
            ..color = colors[i]
            ..style = PaintingStyle.stroke
            ..strokeWidth = DsBorderWidth.focus,
        );
    }
  }

  @override
  bool shouldRepaint(AreaPainter oldDelegate) =>
      oldDelegate.series != series ||
      oldDelegate.grid != grid ||
      oldDelegate.axis != axis ||
      !_sameColors(oldDelegate.colors, colors);

  static bool _sameColors(List<Color> a, List<Color> b) {
    if (a.length != b.length) return false;
    for (var i = 0; i < a.length; i++) {
      if (a[i] != b[i]) return false;
    }
    return true;
  }
}
