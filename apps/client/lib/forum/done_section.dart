// SPDX-License-Identifier: Apache-2.0

/// The row of the Done section at the bottom of the post list (01 6.1; components.md 6).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// "Done (n)" with an arrow; collapsed by default (01 6.1).
///
/// [count] is the number of loaded done posts that match the filters; while further pages
/// may hold more ([more]) the label reads "Done (n+)", because `listChannelThreads` returns no
/// total (brief M4-19 Design note 3). A click or Enter calls [onToggle] with the requested
/// collapsed state; screen readers get the expanded or collapsed state.
class DoneSection extends StatelessWidget {
  /// Creates the row.
  const DoneSection({
    super.key,
    required this.count,
    required this.more,
    required this.collapsed,
    required this.onToggle,
  });

  /// The loaded done posts matching the filters.
  final int count;

  /// Whether more pages may exist.
  final bool more;

  /// Whether the section is collapsed.
  final bool collapsed;

  /// Called with the requested collapsed state.
  final ValueChanged<bool> onToggle;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final text = more ? l.postListDoneMore(count) : l.postListDone(count);
    return DsTappable(
      key: const ValueKey('postList.done'),
      onTap: () => onToggle(!collapsed),
      label: text,
      expanded: !collapsed,
      button: false,
      excludeChildSemantics: true,
      child: Container(
        constraints: BoxConstraints(minHeight: context.dsDensity.rowChannel),
        padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
        decoration: BoxDecoration(
          border: Border(
            top: BorderSide(
              color: c.borderSubtle,
              width: DsBorderWidth.hairline,
            ),
          ),
        ),
        child: Row(
          children: [
            DsIcon(
              collapsed ? DsIcons.actionCollapse : DsIcons.actionExpand,
              size: DsIconSizeRole.sm,
              color: c.textMuted,
              decorative: true,
            ),
            const SizedBox(width: DsSpace.s6),
            Expanded(
              child: Text(
                text,
                style: dsTextStyle(DsType.label).copyWith(color: c.textMuted),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
