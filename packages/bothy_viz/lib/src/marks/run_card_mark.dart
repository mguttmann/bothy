// SPDX-License-Identifier: Apache-2.0

/// The run card and the lane (components.md 11).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import '../model/run_card_model.dart';
import 'viz_colors.dart';
import 'viz_texts.dart';

/// Reports the thread of a tapped run card; the apps open the deep link, the console shows
/// "Open in the app". The package never navigates.
typedef RunTapCallback = void Function(String threadId);

/// One active run: agent, state with icon and text, tool chip, billing mode badge, "Approval
/// waiting" and the cost when shown; never content (25 1 rule 3).
///
/// The semantics label is built from the model, for example "dev-runner, Running, Tool Bash,
/// Organization, US$0.62, Estimate"; the parts below it are excluded so a screen reader reads
/// the card once.
class RunCardMark extends StatelessWidget {
  /// Creates the card of [model]; [onTap] makes it a button.
  const RunCardMark({super.key, required this.model, this.onTap});

  /// The card.
  final RunCardModel model;

  /// Called with [RunCardModel.threadId] on a tap.
  final RunTapCallback? onTap;

  /// The texts of the card in reading order.
  static List<String> parts(VizTexts texts, RunCardModel model) => [
    model.agentName,
    texts.runState(model.state),
    if (model.stepKind != null && model.stepName != null)
      texts.step(model.stepKind!, model.stepName!),
    texts.badge(model.billingMode),
    if (model.approvalPending) texts.l10n.vizApprovalWaiting,
    if (model.cost != null) texts.cost(model.cost!),
  ];

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    final density = context.dsDensity;
    final stateColor = runCardStateColor(colors, model.state);
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final callback = onTap;
    final card = CustomPaint(
      painter: RunCardPainter(
        fill: colors.surfaceRaised,
        border: colors.borderSubtle,
        spine: stateColor,
      ),
      child: ConstrainedBox(
        constraints: BoxConstraints(
          minHeight: density.targetMin,
          minWidth: DsLayout.columnMin,
          maxWidth: DsLayout.sessionWidth,
        ),
        child: Padding(
          padding: EdgeInsets.fromLTRB(
            DsSpace.s12 + DsBorderWidth.spine,
            density.liveboxPad,
            DsSpace.s12,
            density.liveboxPad,
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                model.agentName,
                style: dsTextStyle(DsType.label)
                    .copyWith(color: colors.textPrimary),
              ),
              const SizedBox(height: DsSpace.s4),
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  DsIcon(
                    runCardStateGlyph(model.state),
                    size: DsIconSizeRole.sm,
                    color: stateColor,
                    decorative: true,
                  ),
                  const SizedBox(width: DsSpace.s4),
                  Flexible(
                    child: Text(texts.runState(model.state), style: secondary),
                  ),
                ],
              ),
              if (model.stepKind != null && model.stepName != null) ...[
                const SizedBox(height: DsSpace.s4),
                _Chip(text: texts.step(model.stepKind!, model.stepName!)),
              ],
              const SizedBox(height: DsSpace.s4),
              Wrap(
                spacing: DsSpace.s4,
                runSpacing: DsSpace.s4,
                children: [
                  _Chip(text: texts.badge(model.billingMode)),
                  if (model.approvalPending)
                    _Chip(text: texts.l10n.vizApprovalWaiting, attention: true),
                ],
              ),
              if (model.cost != null) ...[
                const SizedBox(height: DsSpace.s4),
                Text(texts.cost(model.cost!), style: secondary),
              ],
            ],
          ),
        ),
      ),
    );
    return Semantics(
      container: true,
      button: callback != null,
      label: parts(texts, model).join(', '),
      onTap: callback == null ? null : () => callback(model.threadId),
      child: ExcludeSemantics(
        child: callback == null
            ? card
            : GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTap: () => callback(model.threadId),
                child: card,
              ),
      ),
    );
  }
}

class _Chip extends StatelessWidget {
  const _Chip({required this.text, this.attention = false});

  final String text;
  final bool attention;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    return DecoratedBox(
      decoration: BoxDecoration(
        color: attention ? colors.warningSubtle : colors.surfaceSunken,
        borderRadius: BorderRadius.circular(DsRadius.sm),
        border: Border.all(
          color: attention ? colors.warningBorder : colors.borderSubtle,
          width: DsBorderWidth.hairline,
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s6,
          vertical: DsSpace.s2,
        ),
        child: Text(
          text,
          style: dsTextStyle(
            DsType.caption,
          ).copyWith(color: attention ? colors.warningFg : colors.textPrimary),
        ),
      ),
    );
  }
}

/// Paints the card surface with its border and the run state spine on the leading edge.
class RunCardPainter extends CustomPainter {
  /// Creates the painter with its three colors.
  const RunCardPainter({
    required this.fill,
    required this.border,
    required this.spine,
  });

  /// `color.surface.raised`.
  final Color fill;

  /// `color.border.subtle`.
  final Color border;

  /// The run state role.
  final Color spine;

  @override
  void paint(Canvas canvas, Size size) {
    final rect = RRect.fromRectAndRadius(
      Offset.zero & size,
      const Radius.circular(DsRadius.md),
    );
    canvas
      ..drawRRect(rect, Paint()..color = fill)
      ..drawRRect(
        rect.deflate(DsBorderWidth.hairline / 2),
        Paint()
          ..color = border
          ..style = PaintingStyle.stroke
          ..strokeWidth = DsBorderWidth.hairline,
      )
      ..save()
      ..clipRRect(rect)
      ..drawRect(
        Rect.fromLTWH(0, 0, DsBorderWidth.spine, size.height),
        Paint()..color = spine,
      )
      ..restore();
  }

  @override
  bool shouldRepaint(RunCardPainter oldDelegate) =>
      oldDelegate.fill != fill ||
      oldDelegate.border != border ||
      oldDelegate.spine != spine;
}
