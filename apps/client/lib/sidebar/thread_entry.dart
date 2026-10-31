// SPDX-License-Identifier: Apache-2.0

/// A thread row of the sidebar tree (01 6.8; components.md 7 "Thread row"): `DsSidebarThreadRow`
/// below its channel, and "All posts" at the end of an expanded forum group.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';

import 'package:l10n/l10n.dart';

import '../menus/thread_menu.dart';
import 'sidebar_drag.dart';
import 'sidebar_model.dart';
import 'sidebar_threads.dart';

/// The row of [entry] in the group of [channel].
///
/// A click or Enter calls [onOpen] (the router path `/t/<id>`); a right click, the menu key or
/// Shift+F10 opens the thread's menu (M4-33). A forum post is dragged by its parent
/// ([SidebarDragRow], 01 6.16); [moves] adds "Move up" and "Move down" to its menu and to the
/// screen reader actions of the row (D-M4-51-7).
class ThreadEntry extends StatelessWidget {
  /// Creates the entry.
  const ThreadEntry({
    super.key,
    required this.entry,
    required this.channel,
    required this.selected,
    required this.last,
    required this.onOpen,
    this.focusNode,
    this.onKeyEvent,
    this.moves,
  });

  /// The moves of a forum post in the sidebar (01 6.16, M4-51), or null for none (text channel
  /// threads keep their order, D-M4-51-1).
  final SidebarMoves? moves;

  /// The thread with what its row shows.
  final SidebarThread entry;

  /// The channel of the group.
  final Channel channel;

  /// Whether the route shows this thread.
  final bool selected;

  /// Whether it is the last row of the group (the tree line ends there).
  final bool last;

  /// Opens the thread.
  final VoidCallback onOpen;

  /// The focus node of the row, for the arrow keys of the tree (D-SB-8).
  final FocusNode? focusNode;

  /// Handles a key while the row has the focus (the arrow keys of the tree), or null.
  final FocusOnKeyEventCallback? onKeyEvent;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsetsDirectional.only(
      start: DsSpace.s16,
      end: DsSpace.s4,
    ),
    child: ThreadMenu(
      thread: entry.thread.id,
      title: entry.thread.title,
      channel: channel,
      moves: moves,
      // Below the menu region: its menu anchor binds the arrow keys to directional focus.
      child: Focus(
        canRequestFocus: false,
        skipTraversal: true,
        includeSemantics: false,
        onKeyEvent: onKeyEvent,
        child: Semantics(
          customSemanticsActions: moves?.semanticsActions(
            AppLocalizations.of(context),
          ),
          child: DsSidebarThreadRow(
            entry.rowData,
            key: ValueKey('sidebar.thread.${entry.thread.id.value}'),
            selected: selected,
            last: last,
            onOpen: onOpen,
            focusNode: focusNode,
          ),
        ),
      ),
    ),
  );
}

/// "All posts" at the end of an expanded forum group (01 6.8 item 5): opens the forum overview
/// D15 and carries the counters of the posts not shown, never a total.
class AllPostsEntry extends StatelessWidget {
  /// Creates the entry.
  const AllPostsEntry({
    super.key,
    required this.channel,
    required this.counts,
    required this.onOpen,
    this.focusNode,
    this.onKeyEvent,
  });

  /// The forum.
  final ChannelId channel;

  /// Unread and mentions of the posts not shown in the group.
  final ThreadCounts counts;

  /// Opens the forum overview.
  final VoidCallback onOpen;

  /// The focus node of the row, for the arrow keys of the tree (D-SB-8).
  final FocusNode? focusNode;

  /// Handles a key while the row has the focus (the arrow keys of the tree), or null.
  final FocusOnKeyEventCallback? onKeyEvent;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsetsDirectional.only(
      start: DsSpace.s16,
      end: DsSpace.s4,
    ),
    child: Focus(
      canRequestFocus: false,
      skipTraversal: true,
      includeSemantics: false,
      onKeyEvent: onKeyEvent,
      child: DsSidebarAllPostsRow(
        key: ValueKey('sidebar.allPosts.${channel.value}'),
        unreadCount: counts.unread,
        mentionCount: counts.mentions,
        onOpen: onOpen,
        focusNode: focusNode,
      ),
    ),
  );
}

/// "Show more" or "Show less" above "All posts" of a forum group with more open posts than the
/// limit (01 6.8 item 3, M4-46): lists every open post in place or goes back to the newest.
class MorePostsEntry extends StatelessWidget {
  /// Creates the entry.
  const MorePostsEntry({
    super.key,
    required this.channel,
    required this.more,
    required this.onToggle,
    this.focusNode,
    this.onKeyEvent,
  });

  /// The forum.
  final ChannelId channel;

  /// The row state: the posts it shows or hides and whether the list is in full.
  final SidebarMorePosts more;

  /// Called with the requested state: true lists every open post.
  final ValueChanged<bool> onToggle;

  /// The focus node of the row, for the arrow keys of the tree (D-SB-8).
  final FocusNode? focusNode;

  /// Handles a key while the row has the focus (the arrow keys of the tree), or null.
  final FocusOnKeyEventCallback? onKeyEvent;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsetsDirectional.only(
      start: DsSpace.s16,
      end: DsSpace.s4,
    ),
    child: Focus(
      canRequestFocus: false,
      skipTraversal: true,
      includeSemantics: false,
      onKeyEvent: onKeyEvent,
      child: DsSidebarMoreRow(
        key: ValueKey('sidebar.morePosts.${channel.value}'),
        hiddenCount: more.hidden,
        expanded: more.expanded,
        onToggle: () => onToggle(!more.expanded),
        focusNode: focusNode,
      ),
    ),
  );
}

/// A new chat row at the top of a forum group (01 6.8 item 8, brief M4-41): the draft row "New
/// chat" or a sent post that the server has not created yet.
///
/// A click or Enter calls [onOpen] (the new chat view of the forum, with the sent post's entry
/// for a sent one). The row has no menu and cannot be dragged (01 6.8 item 10).
class NewChatEntry extends StatelessWidget {
  /// Creates the entry.
  const NewChatEntry({
    super.key,
    required this.channel,
    required this.row,
    required this.selected,
    required this.last,
    required this.onOpen,
    this.focusNode,
    this.onKeyEvent,
  });

  /// The forum.
  final ChannelId channel;

  /// The row.
  final SidebarNewChatRow row;

  /// Whether the route shows this row.
  final bool selected;

  /// Whether it is the last row of the group (the tree line ends there).
  final bool last;

  /// Opens the row.
  final VoidCallback onOpen;

  /// The focus node of the row, for the arrow keys of the tree (D-SB-8).
  final FocusNode? focusNode;

  /// Handles a key while the row has the focus (the arrow keys of the tree), or null.
  final FocusOnKeyEventCallback? onKeyEvent;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsetsDirectional.only(
      start: DsSpace.s16,
      end: DsSpace.s4,
    ),
    child: Focus(
      canRequestFocus: false,
      skipTraversal: true,
      includeSemantics: false,
      onKeyEvent: onKeyEvent,
      child: DsSidebarDraftRow(
        key: ValueKey(
          'sidebar.newChat.${channel.value}.${row.entryId ?? 'draft'}',
        ),
        title: row.title,
        state: row.state,
        selected: selected,
        last: last,
        onOpen: onOpen,
        focusNode: focusNode,
      ),
    ),
  );
}
