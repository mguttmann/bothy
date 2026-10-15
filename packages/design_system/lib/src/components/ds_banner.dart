// SPDX-License-Identifier: Apache-2.0

/// The banner for info, warning and danger notes (live box stall, offline, card errors).
library;

import 'package:flutter/material.dart';

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_common.dart';

/// A note on a tinted surface with the icon of its [tone] and a text, optionally with one
/// action.
///
/// Info, warning and danger differ in icon shape and text, not only in color. The banner is a
/// semantics container with [text] as its label; [liveRegion] makes screen readers announce it
/// when it appears.
///
/// ```dart
/// DsBanner(tone: DsTone.warning, text: l.liveBoxStall(4))
/// ```
class DsBanner extends StatelessWidget {
  /// Creates the banner.
  const DsBanner({
    super.key,
    required this.tone,
    required this.text,
    this.actionLabel,
    this.onAction,
    this.liveRegion = false,
  });

  /// Info, warning or danger.
  final DsTone tone;

  /// The note.
  final String text;

  /// The label of the action, or null for none.
  final String? actionLabel;

  /// The action, or null.
  final VoidCallback? onAction;

  /// Whether the banner is a polite live region.
  final bool liveRegion;

  @override
  Widget build(BuildContext context) {
    final t = dsToneColors(context, tone);
    final style = dsTextStyle(DsType.bodySm).copyWith(color: t.fg);
    return Semantics(
      container: true,
      liveRegion: liveRegion,
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s12,
          vertical: DsSpace.s8,
        ),
        decoration: BoxDecoration(
          color: t.subtle,
          border: Border.all(color: t.border, width: DsBorderWidth.hairline),
          borderRadius: BorderRadius.circular(DsRadius.md),
        ),
        child: Row(
          children: [
            DsIcon(dsToneGlyph(tone), size: DsIconSizeRole.sm, color: t.fg, decorative: true),
            const SizedBox(width: DsSpace.s8),
            Expanded(child: Text(text, style: style)),
            if (actionLabel != null && onAction != null)
              DsTappable(
                onTap: onAction,
                label: actionLabel,
                excludeChildSemantics: true,
                child: Center(
                  widthFactor: 1,
                  heightFactor: 1,
                  child: Padding(
                    padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
                    child: Text(
                      actionLabel!,
                      style: dsTextStyle(DsType.label).copyWith(color: t.fg),
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
