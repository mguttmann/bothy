// SPDX-License-Identifier: Apache-2.0

/// The tree of spaces and channels (components.md 7): spaces in the person's order, dragged
/// into a new order with the mouse (brief M4-19 Design note 2).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'sidebar_model.dart';
import 'space_node.dart';

/// The tree of [nodes].
///
/// Dragging a space row moves the space; [onMove] gets the space and its index in the list
/// without it (the input of `moveSpace`). The list scrolls and builds its spaces lazily.
class SpaceTree extends StatelessWidget {
  /// Creates the tree.
  const SpaceTree({
    super.key,
    required this.nodes,
    required this.selectedChannel,
    required this.onToggle,
    required this.onMove,
    required this.onOpenChannel,
  });

  /// The spaces in order with their channels.
  final List<SidebarSpaceNode> nodes;

  /// The selected channel, or null.
  final ChannelId? selectedChannel;

  /// Called with a space and its requested expanded state.
  final void Function(SidebarSpaceNode node, bool expanded) onToggle;

  /// Called with a space and its new index (in the list without it).
  final void Function(SpaceId space, int to) onMove;

  /// Opens a channel.
  final ValueChanged<ChannelId> onOpenChannel;

  @override
  Widget build(BuildContext context) {
    if (nodes.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(DsSpace.s12),
        child: Text(
          AppLocalizations.of(context).sidebarNoSpaces,
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: context.dsColors.textOnSidebarSecondary),
        ),
      );
    }
    return ReorderableListView.builder(
      key: const ValueKey('sidebar.tree'),
      buildDefaultDragHandles: false,
      padding: const EdgeInsets.symmetric(horizontal: DsSpace.s4),
      itemCount: nodes.length,
      // The dragged space keeps the sidebar's surface, not the canvas of a Material card.
      proxyDecorator: (child, _, _) => Material(
        color: context.dsColors.surfaceSidebarSelected,
        child: child,
      ),
      onReorderItem: (from, to) => onMove(nodes[from].entry.space.id, to),
      itemBuilder: (context, i) {
        final node = nodes[i];
        final id = node.entry.space.id;
        return ReorderableDragStartListener(
          key: ValueKey('sidebar.node.${id.value}'),
          index: i,
          child: SpaceNode(
            node: node,
            selectedChannel: selectedChannel,
            onToggle: (expanded) => onToggle(node, expanded),
            onOpenChannel: onOpenChannel,
            onMoveUp: i > 0 ? () => onMove(id, i - 1) : null,
            onMoveDown: i < nodes.length - 1 ? () => onMove(id, i + 1) : null,
          ),
        );
      },
    );
  }
}
