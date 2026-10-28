// SPDX-License-Identifier: Apache-2.0

/// The tree of spaces, channels and thread rows (components.md 7; 01 6.8): spaces in the
/// person's order, dragged into a new order with the mouse by their row (brief M4-19 Design
/// note 2; channel and thread rows cannot be dragged, 01 6.8 item 10).
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
/// without it (the input of `moveSpace`). The list scrolls and builds its spaces lazily. With
/// [onOpenThread] every channel shows its thread group (01 6.8).
class SpaceTree extends StatelessWidget {
  /// Creates the tree.
  const SpaceTree({
    super.key,
    required this.nodes,
    required this.selectedChannel,
    required this.onToggle,
    required this.onMove,
    required this.onOpenChannel,
    this.selectedThread,
    this.onOpenThread,
    this.onToggleGroup,
    this.onAdd,
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

  /// The thread the route shows, or null.
  final ThreadId? selectedThread;

  /// Opens a thread row; null shows no thread groups.
  final ValueChanged<ThreadId>? onOpenThread;

  /// Collapses or expands the thread group of a channel.
  final void Function(ChannelId channel, bool expanded)? onToggleGroup;

  /// The "+" action of a forum row, or null for none.
  final ValueChanged<ChannelId>? onAdd;

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
        return KeyedSubtree(
          key: ValueKey('sidebar.node.${id.value}'),
          // The list gives every item the actions to move it on a node of its own; the space's
          // name makes that node say which space it moves (brief M5-10).
          child: Semantics(
            label: AppLocalizations.of(context)
                .sidebarSpaceGroup(node.entry.space.name),
            child: SpaceNode(
              node: node,
              selectedChannel: selectedChannel,
              selectedThread: selectedThread,
              onToggle: (expanded) => onToggle(node, expanded),
              onOpenChannel: onOpenChannel,
              onOpenThread: onOpenThread,
              onToggleGroup: onToggleGroup,
              onAdd: onAdd,
              onMoveUp: i > 0 ? () => onMove(id, i - 1) : null,
              onMoveDown: i < nodes.length - 1 ? () => onMove(id, i + 1) : null,
              // Only the space row starts a drag: a channel or thread row must never move
              // anything (01 6.8 item 10, D-SB-7).
              dragHandle: (row) =>
                  ReorderableDragStartListener(index: i, child: row),
            ),
          ),
        );
      },
    );
  }
}
