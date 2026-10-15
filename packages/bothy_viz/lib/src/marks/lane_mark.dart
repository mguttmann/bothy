// SPDX-License-Identifier: Apache-2.0

/// The lane of the live topology (components.md 11).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import '../model/run_card_model.dart';
import 'run_card_mark.dart';
import 'viz_texts.dart';

/// One channel of one space with its run cards in the order of the frame.
///
/// The lane is a semantics container labelled "Engineering, #backend, 2 active runs"; each
/// card inside keeps its own label and tap.
class LaneMark extends StatelessWidget {
  /// Creates the lane of [model].
  const LaneMark({super.key, required this.model, this.onTapRun});

  /// The lane.
  final LaneModel model;

  /// Passed to every [RunCardMark].
  final RunTapCallback? onTapRun;

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    final label = texts.l10n.vizLaneLabel(
      model.spaceName,
      model.channelName,
      model.cards.length,
    );
    return Semantics(
      container: true,
      label: label,
      explicitChildNodes: true,
      child: CustomPaint(
        painter: LanePainter(track: colors.vizAxis, fill: colors.surfaceSunken),
        child: Padding(
          padding: const EdgeInsets.all(DsSpace.s8),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              ExcludeSemantics(
                child: Text(
                  '${model.spaceName}, ${model.channelName}',
                  style: dsTextStyle(DsType.section)
                      .copyWith(color: colors.textSecondary),
                ),
              ),
              const SizedBox(height: DsSpace.s8),
              Wrap(
                spacing: DsSpace.s8,
                runSpacing: DsSpace.s8,
                children: [
                  for (final card in model.cards)
                    RunCardMark(model: card, onTap: onTapRun),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Paints the lane background and its track along the leading edge.
class LanePainter extends CustomPainter {
  /// Creates the painter.
  const LanePainter({required this.track, required this.fill});

  /// `color.viz.axis`.
  final Color track;

  /// `color.surface.sunken`.
  final Color fill;

  @override
  void paint(Canvas canvas, Size size) {
    canvas
      ..drawRRect(
        RRect.fromRectAndRadius(
          Offset.zero & size,
          const Radius.circular(DsRadius.lg),
        ),
        Paint()..color = fill,
      )
      ..drawLine(
        const Offset(DsBorderWidth.hairline, DsSpace.s8),
        Offset(DsBorderWidth.hairline, size.height - DsSpace.s8),
        Paint()
          ..color = track
          ..strokeWidth = DsBorderWidth.hairline,
      );
  }

  @override
  bool shouldRepaint(LanePainter oldDelegate) =>
      oldDelegate.track != track || oldDelegate.fill != fill;
}
