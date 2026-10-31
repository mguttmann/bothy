// SPDX-License-Identifier: Apache-2.0

/// The status line of a run (components.md section 4).
library;

import 'package:flutter/material.dart';

import '../generated/design_tokens.g.dart';
import '../theme/ds_theme.dart';

/// One sentence on what the agent does right now (`runs.status_line`).
///
/// `type.body-sm` in `color.text.primary`, one line, cut at the end with the full text in a
/// tooltip; text between backticks shows as inline code. An empty or null [text] renders
/// nothing. Not a live region (components.md 2.4): its changes are patches.
///
/// ```dart
/// DsStatusLine('Runs `make check` in the worktree')
/// ```
class DsStatusLine extends StatelessWidget {
  /// Creates the line of [text].
  const DsStatusLine(this.text, {super.key});

  /// `status_line`, or null.
  final String? text;

  /// The spans of [value] in the styles of the line: text between backticks as inline code;
  /// shared with the one-line mobile live box (brief M5-17), which sets the line after the
  /// state in the same text.
  static List<InlineSpan> spans(BuildContext context, String value) {
    final colors = context.dsColors;
    final body = dsTextStyle(DsType.bodySm).copyWith(color: colors.textPrimary);
    final code = dsTextStyle(DsType.codeSm)
        .copyWith(color: colors.codeFg, backgroundColor: colors.codeBg);
    final parts = value.split('`');
    return [
      for (var i = 0; i < parts.length; i++)
        // Odd parts lie between two backticks; an unpaired backtick stays text.
        if (i.isOdd && i < parts.length - 1)
          TextSpan(text: parts[i], style: code)
        else
          TextSpan(text: i.isOdd ? '`${parts[i]}' : parts[i], style: body),
    ];
  }

  @override
  Widget build(BuildContext context) {
    final value = text?.trim() ?? '';
    if (value.isEmpty) return const SizedBox.shrink();
    return Tooltip(
      message: value,
      excludeFromSemantics: true,
      child: Text.rich(
        TextSpan(children: spans(context, value)),
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
      ),
    );
  }
}
