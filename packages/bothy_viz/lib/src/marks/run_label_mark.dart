// SPDX-License-Identifier: Apache-2.0

/// The shared run label of the run card, the timeline row and TV mode (owner request 2026-11-01).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import '../model/run_label.dart';
import 'viz_texts.dart';

/// A run label in two lines: project and chat ([VizTexts.runTitle]) as the main text, agent and
/// short ID ([VizTexts.runDetail]) below it as secondary text.
///
/// Each line is cut to one line with an ellipsis; the full text of both is the tooltip, which
/// shows on hover (desktop, web) and on long press (touch). The label carries no semantics of
/// its own: the card or row around it says the text once ([VizRunLabelText.parts]).
class VizRunLabelText extends StatelessWidget {
  /// Creates the label of [label]; [titleStyle] replaces the style of the main text.
  const VizRunLabelText({super.key, required this.label, this.titleStyle});

  /// The label.
  final VizRunLabel label;

  /// Style of the main text; `null` takes `DsType.label` in the primary text color.
  final TextStyle? titleStyle;

  /// The texts of [label] in reading order, for the semantics label and the tables.
  static List<String> parts(VizTexts texts, VizRunLabel label) => [
    texts.runTitle(label),
    texts.runDetail(label),
  ];

  /// The full text the tooltip shows.
  static String full(VizTexts texts, VizRunLabel label) =>
      parts(texts, label).join('\n');

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    return Tooltip(
      message: full(texts, label),
      excludeFromSemantics: true,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            texts.runTitle(label),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style:
                titleStyle ??
                dsTextStyle(DsType.label).copyWith(color: colors.textPrimary),
          ),
          Text(
            texts.runDetail(label),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: dsTextStyle(DsType.caption)
                .copyWith(color: colors.textSecondary),
          ),
        ],
      ),
    );
  }
}
