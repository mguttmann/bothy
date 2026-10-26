// SPDX-License-Identifier: Apache-2.0

/// The bottom of the sidebar: the own avatar, the status page (M7-21) and the settings entry
/// (components.md 7; 01 5.1 D1), and the temporary "Log out" of M4-18 until the settings of M4-25 carry it.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// The foot of the sidebar.
///
/// [name] is the display name of the signed-in person, or null before the first snapshot of
/// this process (then the avatar shows no initial). [onSettings] opens `/settings`,
/// [onLogout] logs out (docs/DECISIONS.md D-M4-18-5).
class SidebarFooter extends StatelessWidget {
  /// Creates the foot.
  const SidebarFooter({
    super.key,
    required this.name,
    required this.onSettings,
    required this.onLogout,
    this.onStatus,
  });

  /// The display name, or null.
  final String? name;

  /// Opens the settings.
  final VoidCallback onSettings;

  /// Logs out.
  final VoidCallback onLogout;

  /// Opens the status page (`/status`, M7-21), or null for no entry.
  final VoidCallback? onStatus;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final own = name ?? '';
    return Container(
      decoration: BoxDecoration(
        border: Border(
          top: BorderSide(
            color: c.borderOnSidebar,
            width: DsBorderWidth.hairline,
          ),
        ),
      ),
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s12,
        vertical: DsSpace.s8,
      ),
      child: Row(
        children: [
          Semantics(
            key: const ValueKey('sidebar.avatar'),
            label: own.isEmpty ? null : l.sidebarOwnAvatar(own),
            child: DsAvatar(name: own, origin: DsMessageOrigin.human),
          ),
          const SizedBox(width: DsSpace.s8),
          Expanded(
            child: ExcludeSemantics(
              child: Text(
                dsIsolate(own),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: c.textOnSidebar),
              ),
            ),
          ),
          if (onStatus case final status?)
            DsTappable(
              key: const ValueKey('sidebar.status'),
              onTap: status,
              label: l.statusNavEntry,
              onSidebar: true,
              hoverColor: c.surfaceSidebarHover,
              excludeChildSemantics: true,
              child: SizedBox.square(
                dimension: context.dsDensity.targetMin,
                child: Center(
                  child: DsIcon(
                    DsIcons.stepStatus,
                    size: DsIconSizeRole.md,
                    color: c.textOnSidebarSecondary,
                    decorative: true,
                  ),
                ),
              ),
            ),
          DsTappable(
            key: const ValueKey('sidebar.settings'),
            onTap: onSettings,
            label: l.sidebarSettings,
            onSidebar: true,
            hoverColor: c.surfaceSidebarHover,
            excludeChildSemantics: true,
            child: SizedBox.square(
              dimension: context.dsDensity.targetMin,
              child: Center(
                child: DsIcon(
                  DsIcons.actionSettings,
                  size: DsIconSizeRole.md,
                  color: c.textOnSidebarSecondary,
                  decorative: true,
                ),
              ),
            ),
          ),
          // Flexible: with large text the label shortens instead of overflowing the column.
          Flexible(
            child: DsTappable(
              key: const ValueKey('shell.logout'),
              onTap: onLogout,
              label: l.onboardingLogOut,
              onSidebar: true,
              hoverColor: c.surfaceSidebarHover,
              excludeChildSemantics: true,
              child: Container(
                constraints: BoxConstraints(
                  minHeight: context.dsDensity.targetMin,
                ),
                alignment: Alignment.center,
                padding: const EdgeInsets.symmetric(horizontal: DsSpace.s6),
                child: Text(
                  l.onboardingLogOut,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: dsTextStyle(DsType.caption)
                      .copyWith(color: c.textOnSidebarSecondary),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
