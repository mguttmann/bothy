// SPDX-License-Identifier: Apache-2.0

/// The row of the section "Archived (n)" at the bottom of a space group in the sidebar (01 6.22
/// item 2).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// "Archived (n)" with an arrow, indented like a channel row below its space; collapsed by
/// default, like the Done section of a forum (01 6.1).
///
/// [count] is the number of channels the person archived in the space. While collapsed, the row
/// carries [counts], the unread messages and mentions of those channels (01 6.22 item 4), as
/// "All posts" does for the posts it does not show: bold text for unread items and the mention
/// counter, both in the label. A click, a tap or Enter calls [onToggle] with the requested
/// expanded state; screen readers get the expanded or collapsed state.
class ArchivedSection extends StatelessWidget {
  /// Creates the row.
  const ArchivedSection({
    super.key,
    required this.space,
    required this.count,
    required this.expanded,
    required this.onToggle,
    this.counts = ThreadCounts.zero,
  });

  /// The space of the section.
  final SpaceId space;

  /// The archived channels of the space.
  final int count;

  /// Whether the section shows its channels.
  final bool expanded;

  /// Called with the requested expanded state.
  final ValueChanged<bool> onToggle;

  /// Unread messages and mentions of the archived channels, shown while collapsed.
  final ThreadCounts counts;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final text = l.sidebarArchived(count);
    final c = context.dsColors;
    final unread = !expanded && counts.unread > 0;
    final mentions = expanded ? 0 : counts.mentions;
    final label = [
      text,
      if (unread) l.threadRowUnread(counts.unread),
      if (mentions > 0) l.threadRowMentions(mentions),
    ].join(', ');
    return DsTappable(
      key: ValueKey('sidebar.archived.${space.value}'),
      onTap: () => onToggle(!expanded),
      label: label,
      expanded: expanded,
      button: false,
      excludeChildSemantics: true,
      child: Container(
        constraints: BoxConstraints(minHeight: context.dsDensity.rowChannel),
        padding: const EdgeInsetsDirectional.only(
          start: DsSpace.s16,
          end: DsSpace.s4,
        ),
        child: Row(
          children: [
            DsIcon(
              expanded ? DsIcons.actionExpand : DsIcons.actionCollapse,
              size: DsIconSizeRole.sm,
              color: c.textOnSidebarSecondary,
              decorative: true,
            ),
            const SizedBox(width: DsSpace.s6),
            Expanded(
              child: Text(
                text,
                style: dsTextStyle(DsType.label).copyWith(
                  color: unread ? c.textOnSidebar : c.textOnSidebarSecondary,
                  fontWeight: unread ? FontWeight.w700 : null,
                ),
              ),
            ),
            if (mentions > 0) ...[
              const SizedBox(width: DsSpace.s6),
              DsBadge(
                text: dsFormat(context).count(mentions),
                background: c.attentionBg,
                foreground: c.attentionOnBg,
              ),
            ],
          ],
        ),
      ),
    );
  }
}
