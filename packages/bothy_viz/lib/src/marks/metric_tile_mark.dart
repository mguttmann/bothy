// SPDX-License-Identifier: Apache-2.0

/// The metric tile (components.md 11).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';

import '../model/metric_tile_model.dart';
import 'viz_texts.dart';

/// One large number with its label, the comparison with the previous period and the cost
/// label; a value the server did not deliver reads "not shown", never zero (25 4 rule 5).
class MetricTileMark extends StatelessWidget {
  /// Creates the tile of [model].
  const MetricTileMark({super.key, required this.model});

  /// The tile.
  final MetricTileModel model;

  /// The label of the tile.
  static String label(VizTexts texts, MetricTileModel model) {
    if (model.metric != null) return texts.metric(model.metric!);
    if (model.queryMetric != null) return texts.queryMetric(model.queryMetric!);
    final mode = model.costMode;
    return mode == null
        ? texts.l10n.vizMetricCost
        : '${texts.l10n.vizMetricCost}, ${texts.badge(mode)}';
  }

  /// The formatted value, or "not shown".
  static String value(VizTexts texts, MetricTileModel model) {
    final v = model.value;
    if (v == null) return texts.l10n.vizNotShown;
    return switch (model.unit) {
      TileUnit.count => texts.format.count(v.round()),
      TileUnit.percent => texts.format.percent(v),
      TileUnit.money => texts.format.money(v / 1000000, model.currency ?? ''),
      // The unit symbol is the same in every language.
      TileUnit.milliseconds => '${texts.format.count(v.round())}\u00a0ms',
    };
  }

  /// The comparison with the previous period; `null` without one.
  static String? comparison(VizTexts texts, MetricTileModel model) {
    final v = model.value;
    final p = model.previous;
    if (v == null || p == null) return null;
    final delta = (v - p).abs();
    final text = model.unit == TileUnit.money
        ? texts.format.money(delta / 1000000, model.currency ?? '')
        : texts.format.count(delta.round());
    if (v > p) return texts.l10n.vizTileUp(text);
    if (v < p) return texts.l10n.vizTileDown(text);
    return texts.l10n.vizTileSame;
  }

  /// The cost label text; `null` for a final amount or a tile without cost.
  static String? costLabelText(VizTexts texts, MetricTileModel model) =>
      switch (model.costLabel) {
        CostLabel.estimate => texts.l10n.vizCostEstimate,
        CostLabel.informationOnly => texts.l10n.vizCostInformationOnly,
        CostLabel.finalAmount || null => null,
      };

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    final parts = [
      label(texts, model),
      value(texts, model),
      ?comparison(texts, model),
      ?costLabelText(texts, model),
    ];
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Semantics(
      container: true,
      label: parts.join(', '),
      child: ExcludeSemantics(
        child: CustomPaint(
          painter: MetricTilePainter(
            fill: colors.surfaceRaised,
            border: colors.borderSubtle,
          ),
          child: ConstrainedBox(
            constraints: const BoxConstraints(minWidth: DsLayout.columnMin),
            child: Padding(
              padding: const EdgeInsets.all(DsSpace.s16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    parts[0],
                    style: dsTextStyle(DsType.label)
                        .copyWith(color: colors.textSecondary),
                  ),
                  const SizedBox(height: DsSpace.s4),
                  Text(
                    parts[1],
                    style: dsTextStyle(DsType.display)
                        .copyWith(color: colors.textPrimary),
                  ),
                  for (final extra in parts.skip(2))
                    Text(extra, style: secondary),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// Paints the tile surface and its border.
class MetricTilePainter extends CustomPainter {
  /// Creates the painter.
  const MetricTilePainter({required this.fill, required this.border});

  /// `color.surface.raised`.
  final Color fill;

  /// `color.border.subtle`.
  final Color border;

  @override
  void paint(Canvas canvas, Size size) {
    final rect = RRect.fromRectAndRadius(
      Offset.zero & size,
      const Radius.circular(DsRadius.lg),
    );
    canvas
      ..drawRRect(rect, Paint()..color = fill)
      ..drawRRect(
        rect.deflate(DsBorderWidth.hairline / 2),
        Paint()
          ..color = border
          ..style = PaintingStyle.stroke
          ..strokeWidth = DsBorderWidth.hairline,
      );
  }

  @override
  bool shouldRepaint(MetricTilePainter oldDelegate) =>
      oldDelegate.fill != fill || oldDelegate.border != border;
}
