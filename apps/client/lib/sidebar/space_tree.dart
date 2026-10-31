// SPDX-License-Identifier: Apache-2.0

/// The tree of spaces, channels and thread rows (components.md 7; 01 6.8): spaces in the
/// person's order, dragged into a new order by their row (brief M4-19 Design note 2) with the
/// mouse or, on a touch screen, after a long press; channels and forum posts are dragged within
/// their parent (01 6.15, brief M4-51).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'sidebar_arrange.dart';
import 'sidebar_drag.dart';
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
    this.onShowAllPosts,
    this.onAdd,
    this.onNewProject,
    this.onAddFolderForums,
    this.selectedNewChat,
    this.onOpenNewChat,
    this.arrangement = const SidebarArrangement(),
    this.touch = false,
    this.onMoveChannel,
    this.onMoveThread,
  });

  /// Which levels are arranged by activity (01 6.15): an automatic level cannot be dragged.
  final SidebarArrangement arrangement;

  /// Whether the screen is a touch screen: a long press starts a drag (D-M4-51-6).
  final bool touch;

  /// Moves a channel within its space to an index (in the list without it), or null.
  final void Function(SpaceId space, ChannelId channel, int to)? onMoveChannel;

  /// Moves an open post within its forum to an index among the post rows on screen, or null.
  final void Function(ChannelId forum, ThreadId thread, int to)? onMoveThread;

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

  /// Lists every open post of a forum's group (true, "Show more") or the newest ones (false,
  /// "Show less"); null shows no such row (01 6.8 item 3, M4-46).
  final void Function(ChannelId forum, bool inFull)? onShowAllPosts;

  /// The new chat row the route selects, or null (brief M4-41).
  final SidebarNewChatSelection? selectedNewChat;

  /// Opens a new chat row of a forum: its draft (entry null) or a sent post; null for none.
  final void Function(ChannelId forum, String? entry)? onOpenNewChat;

  /// The "+" action of a forum row, or null for none.
  final ValueChanged<ChannelId>? onAdd;

  /// The action "New project" of a space row and its menu (01 6.9 item 1), or null for none.
  final ValueChanged<Space>? onNewProject;

  /// The space menu entry "Add forums for existing folders" (01 6.10 item 1), or null for none.
  final ValueChanged<Space>? onAddFolderForums;

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
        final arranged = arrangement.isAuto(SidebarLevel.spaces);
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
              onShowAllPosts: onShowAllPosts,
              onAdd: onAdd,
              onNewProject: onNewProject,
              onAddFolderForums: onAddFolderForums,
              selectedNewChat: selectedNewChat,
              onOpenNewChat: onOpenNewChat,
              onMoveUp: !arranged && i > 0 ? () => onMove(id, i - 1) : null,
              onMoveDown: !arranged && i < nodes.length - 1
                  ? () => onMove(id, i + 1)
                  : null,
              arrangement: arrangement,
              touch: touch,
              onMoveChannel: onMoveChannel == null
                  ? null
                  : (channel, to) => onMoveChannel!(id, channel, to),
              onMoveThread: onMoveThread,
              // The space row starts the drag of the space; on a touch screen after a long
              // press, so a swipe scrolls the list (D-M4-51-6). An automatic level shows the
              // note instead.
              dragHandle: (row) => arranged
                  ? SidebarArrangedRow(
                      level: SidebarLevel.spaces,
                      touch: touch,
                      child: row,
                    )
                  : touch
                  ? ReorderableDelayedDragStartListener(index: i, child: row)
                  : ReorderableDragStartListener(index: i, child: row),
            ),
          ),
        );
      },
    );
  }
}
