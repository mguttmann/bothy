// SPDX-License-Identifier: Apache-2.0

/// A space node of the sidebar tree: `DsSpaceRow` of M4-07 with its channels below it when
/// expanded (components.md 7).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:l10n/l10n.dart';

import '../menus/space_menu.dart';
import 'channel_entry.dart';
import 'sidebar_model.dart';

/// The node of [node].
///
/// The space row toggles with a click, Enter, arrow left and arrow right ([onToggle] gets the
/// requested expanded state); screen readers get the expanded or collapsed state and the
/// actions "Move up" and "Move down" ([onMoveUp], [onMoveDown]; null when the space is at
/// that end), the keyboard and screen reader way to change the personal order besides
/// dragging. The channels follow when the space is expanded. A right click, the menu key or
/// Shift+F10 opens the space's menu (M4-33).
class SpaceNode extends StatelessWidget {
  /// Creates the node.
  const SpaceNode({
    super.key,
    required this.node,
    required this.selectedChannel,
    required this.onToggle,
    required this.onOpenChannel,
    this.onMoveUp,
    this.onMoveDown,
  });

  /// The space with its channels.
  final SidebarSpaceNode node;

  /// The selected channel, or null.
  final ChannelId? selectedChannel;

  /// Called with the requested expanded state.
  final ValueChanged<bool> onToggle;

  /// Opens a channel.
  final ValueChanged<ChannelId> onOpenChannel;

  /// Moves the space one place up; null at the top.
  final VoidCallback? onMoveUp;

  /// Moves the space one place down; null at the bottom.
  final VoidCallback? onMoveDown;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          customSemanticsActions: {
            CustomSemanticsAction(label: l.sidebarMoveUp): ?onMoveUp,
            CustomSemanticsAction(label: l.sidebarMoveDown): ?onMoveDown,
          },
          child: Container(
            constraints: BoxConstraints(
              minHeight: context.dsDensity.rowChannel,
            ),
            alignment: Alignment.centerLeft,
            padding: const EdgeInsets.only(top: DsSpace.s4),
            child: SpaceMenu(
              space: node.entry.space,
              child: DsSpaceRow(
                node.rowData,
                key: ValueKey('sidebar.space.${node.entry.space.id.value}'),
                onToggle: onToggle,
              ),
            ),
          ),
        ),
        if (node.expanded)
          for (final c in node.channels)
            ChannelEntry(
              entry: c,
              selected: c.channel.id == selectedChannel,
              onOpen: () => onOpenChannel(c.channel.id),
            ),
      ],
    );
  }
}
