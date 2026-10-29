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
/// dragging. The channels follow when the space is expanded, each with its thread group when
/// [onOpenThread] is given (01 6.8). A right click, the menu key or Shift+F10 opens the space's
/// menu (M4-33). Only the space row can start a drag ([dragHandle]); channel and thread rows
/// cannot be dragged (01 6.8 item 10).
class SpaceNode extends StatefulWidget {
  /// Creates the node.
  const SpaceNode({
    super.key,
    required this.node,
    required this.selectedChannel,
    required this.onToggle,
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
    this.onMoveUp,
    this.onMoveDown,
    this.dragHandle,
  });

  /// The space with its channels.
  final SidebarSpaceNode node;

  /// The selected channel, or null.
  final ChannelId? selectedChannel;

  /// Called with the requested expanded state.
  final ValueChanged<bool> onToggle;

  /// Opens a channel.
  final ValueChanged<ChannelId> onOpenChannel;

  /// The thread the route shows, or null.
  final ThreadId? selectedThread;

  /// Opens a thread row; null shows no thread groups (tests of the tree without groups).
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

  /// The "+" action of a forum row (01 6.8 item 8), or null for none.
  final ValueChanged<ChannelId>? onAdd;

  /// The action "New project" of the space row and its menu (01 6.9 item 1), or null for none.
  final ValueChanged<Space>? onNewProject;

  /// The space menu entry "Add forums for existing folders" (01 6.10 item 1), or null for none.
  final ValueChanged<Space>? onAddFolderForums;

  /// Wraps the space row so it starts a drag of the space (the list's drag listener), or null.
  final Widget Function(Widget row)? dragHandle;

  /// Moves the space one place up; null at the top.
  final VoidCallback? onMoveUp;

  /// Moves the space one place down; null at the bottom.
  final VoidCallback? onMoveDown;

  @override
  State<SpaceNode> createState() => _SpaceNodeState();
}

class _SpaceNodeState extends State<SpaceNode> {
  /// The node of the space row: arrow left on one of its channels comes back here (D-SB-8).
  final FocusNode _row = FocusNode(debugLabel: 'sidebar.space');

  @override
  void dispose() {
    _row.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final SpaceNode(
      :node,
      :selectedChannel,
      :onToggle,
      :onOpenChannel,
      :selectedThread,
      :onOpenThread,
      :onToggleGroup,
      :onShowAllPosts,
      :onAdd,
      :onNewProject,
      :onAddFolderForums,
      :selectedNewChat,
      :onOpenNewChat,
      :onMoveUp,
      :onMoveDown,
      :dragHandle,
    ) = widget;
    final space = node.entry.space;
    final newProject = onNewProject == null ? null : () => onNewProject(space);
    final newProjectLabel = newProject == null
        ? null
        : l.newProjectIn(space.name);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        // One node: the row with its name, state and the move actions; without the merge the
        // actions sat on an unnamed node of their own around the row, which VoiceOver stops at
        // without a name (brief M5-10).
        (dragHandle ?? (row) => row)(
          MergeSemantics(
            child: Semantics(
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
                  newProjectLabel: newProjectLabel,
                  onNewProject: newProject,
                  onAddFolderForums: onAddFolderForums == null
                      ? null
                      : () => onAddFolderForums(space),
                  child: DsSpaceRow(
                    node.rowData,
                    key: ValueKey('sidebar.space.${node.entry.space.id.value}'),
                    onToggle: onToggle,
                    focusNode: _row,
                    addLabel: newProjectLabel,
                    onAdd: newProject,
                  ),
                ),
              ),
            ),
          ),
        ),
        if (node.expanded)
          for (final c in node.channels)
            ChannelEntry(
              key: ValueKey('sidebar.entry.${c.channel.id.value}'),
              entry: c,
              selected: c.channel.id == selectedChannel,
              selectedThread: selectedThread,
              onOpen: () => onOpenChannel(c.channel.id),
              onOpenThread: onOpenThread,
              onToggleGroup: onToggleGroup == null
                  ? null
                  : (expanded) => onToggleGroup(c.channel.id, expanded),
              onShowAllPosts: onShowAllPosts == null
                  ? null
                  : (inFull) => onShowAllPosts(c.channel.id, inFull),
              onAdd: onAdd == null ? null : () => onAdd(c.channel.id),
              selectedNewChat: selectedNewChat,
              onOpenNewChat: onOpenNewChat == null
                  ? null
                  : (entry) => onOpenNewChat(c.channel.id, entry),
              onParent: _row.requestFocus,
            ),
      ],
    );
  }
}
