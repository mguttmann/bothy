// SPDX-License-Identifier: Apache-2.0

/// The sidebar group "Live and statistics" of the desktop (docs/spec/01-product.md 4 and 5.1
/// D12; brief M6-16 Scope): "Live" opens the views of `bothy_viz` with TV mode, "Statistics"
/// the statistics A7 of the administration.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// The group with its two entries, styled like the entry "Running now" of the head.
///
/// The phone has no group: Live is M10 under Me there, and the administration is desktop and
/// web console only (01 4).
class SidebarLiveGroup extends StatelessWidget {
  /// Creates the group.
  const SidebarLiveGroup({
    super.key,
    required this.onLive,
    required this.onStatistics,
  });

  /// Opens "Live" (`/live`).
  final VoidCallback onLive;

  /// Opens "Statistics" (`/admin/statistics`).
  final VoidCallback onStatistics;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    Widget entry(String key, DsGlyph glyph, String label, VoidCallback onTap) =>
        DsTappable(
          key: ValueKey(key),
          onTap: onTap,
          label: label,
          onSidebar: true,
          excludeChildSemantics: true,
          child: Container(
            constraints: BoxConstraints(
              minHeight: context.dsDensity.rowChannel,
            ),
            padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
            child: Row(
              children: [
                DsIcon(
                  glyph,
                  size: DsIconSizeRole.sm,
                  color: c.textOnSidebarSecondary,
                  decorative: true,
                ),
                const SizedBox(width: DsSpace.s8),
                Expanded(
                  child: Text(
                    label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: dsTextStyle(DsType.bodySm)
                        .copyWith(color: c.textOnSidebar),
                  ),
                ),
              ],
            ),
          ),
        );
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        DsSpace.s12,
        0,
        DsSpace.s12,
        DsSpace.s8,
      ),
      child: Semantics(
        container: true,
        label: l.sidebarLiveGroup,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: DsSpace.s8,
                vertical: DsSpace.s4,
              ),
              child: ExcludeSemantics(
                child: Text(
                  l.sidebarLiveGroup,
                  style: dsTextStyle(DsType.label)
                      .copyWith(color: c.textOnSidebarSecondary),
                ),
              ),
            ),
            entry('sidebar.live', DsIcons.agent, l.sidebarLive, onLive),
            entry(
              'sidebar.statistics',
              DsIcons.adminStats,
              l.adminNavStatistics,
              onStatistics,
            ),
          ],
        ),
      ),
    );
  }
}
