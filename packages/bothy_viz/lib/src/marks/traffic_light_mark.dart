// SPDX-License-Identifier: Apache-2.0

/// The SLO traffic light (components.md 11; docs/spec/25-visualization.md 6.4, 7.4).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';

import '../model/traffic_light_model.dart';
import 'viz_colors.dart';
import 'viz_texts.dart';

/// Sends a polite announcement of [message] for the view of [context].
typedef VizAnnouncer = void Function(BuildContext context, String message);

void _announce(BuildContext context, String message) {
  SemanticsService.sendAnnouncement(
    View.of(context),
    message,
    Directionality.of(context),
  );
}

/// One SLO as a light with icon and text: "API availability: on target, own measurement".
///
/// The state is color, icon and text together, never color alone. Live values are not
/// announced continuously: the mark announces politely only when the state of the same SLO
/// changes between two builds ("API availability is now target missed"), and nothing on an
/// unchanged frame (25 7.4).
class TrafficLightMark extends StatefulWidget {
  /// Creates the light of [model]; [announcer] defaults to `SemanticsService.sendAnnouncement`.
  const TrafficLightMark({super.key, required this.model, this.announcer});

  /// The light.
  final TrafficLightModel model;

  /// Sends the announcement of a state change.
  final VizAnnouncer? announcer;

  @override
  State<TrafficLightMark> createState() => _TrafficLightMarkState();
}

class _TrafficLightMarkState extends State<TrafficLightMark> {
  @override
  void didUpdateWidget(TrafficLightMark oldWidget) {
    super.didUpdateWidget(oldWidget);
    final before = oldWidget.model;
    final after = widget.model;
    if (before.slo == after.slo && before.status != after.status) {
      final texts = VizTexts.of(context);
      final message = texts.l10n.vizTrafficLightChanged(
        texts.slo(after),
        texts.sloState(after.status),
      );
      (widget.announcer ?? _announce)(context, message);
    }
  }

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    final model = widget.model;
    final role = stateRoleColors(colors, model.role);
    final name = texts.slo(model);
    final state = texts.sloState(model.status);
    return Semantics(
      container: true,
      label: texts.l10n.vizTrafficLightLabel(name, state),
      child: ExcludeSemantics(
        child: CustomPaint(
          painter: TrafficLightPainter(fill: role.subtle, border: role.border),
          child: Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: DsSpace.s12,
              vertical: DsSpace.s8,
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                DsIcon(model.glyph, color: role.fg, decorative: true),
                const SizedBox(width: DsSpace.s8),
                Flexible(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        name,
                        style: dsTextStyle(DsType.label)
                            .copyWith(color: colors.textPrimary),
                      ),
                      Text(
                        state,
                        style: dsTextStyle(DsType.bodySm)
                            .copyWith(color: role.fg),
                      ),
                      Text(
                        texts.l10n.vizOwnMeasurement,
                        style: dsTextStyle(DsType.caption)
                            .copyWith(color: colors.textSecondary),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// Paints the light's pill in the subtle state role with its border.
class TrafficLightPainter extends CustomPainter {
  /// Creates the painter.
  const TrafficLightPainter({required this.fill, required this.border});

  /// Subtle background of the state role.
  final Color fill;

  /// Border of the state role.
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
  bool shouldRepaint(TrafficLightPainter oldDelegate) =>
      oldDelegate.fill != fill || oldDelegate.border != border;
}
