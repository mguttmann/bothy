// SPDX-License-Identifier: Apache-2.0

/// A note of the thread (01 6.21 item 3, R-PR-37; brief AR-08): one line with an info icon, in
/// the secondary text colour, without author, avatar, reactions or a menu, so it never looks
/// like a message of a person. The history draws the server's notes of wake-up ends and caps
/// with it, and the run line its derived notes (waiting without a wake-up, the usage limit).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// One note line with [text] and, when given, its [time].
class ThreadNoteLine extends StatelessWidget {
  /// Creates the line.
  const ThreadNoteLine({super.key, required this.text, this.time});

  /// The translated text of the note.
  final String text;

  /// The time of the note, formatted by the caller, or null for a derived note.
  final String? time;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    final style = dsTextStyle(DsType.caption)
        .copyWith(color: colors.textSecondary);
    final at = time;
    return Semantics(
      container: true,
      label: AppLocalizations.of(context).noteSemantics(text),
      excludeSemantics: true,
      child: Padding(
        padding: EdgeInsets.symmetric(
          horizontal: DsSpace.s8,
          vertical: context.dsDensity.messagePadY,
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            DsIcon(
              DsIcons.stateInfo,
              size: DsIconSizeRole.sm,
              color: colors.textSecondary,
              decorative: true,
            ),
            const SizedBox(width: DsSpace.s8),
            Expanded(child: Text(text, style: style)),
            if (at != null) ...[
              const SizedBox(width: DsSpace.s8),
              Text(at, style: style),
            ],
          ],
        ),
      ),
    );
  }
}
