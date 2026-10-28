// SPDX-License-Identifier: Apache-2.0

/// A thread row of the sidebar tree (01 6.8; components.md 7 "Thread row"): `DsSidebarThreadRow`
/// below its channel, and "All posts" at the end of an expanded forum group.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';

import '../menus/thread_menu.dart';
import 'sidebar_threads.dart';

/// The row of [entry] in the group of [channel].
///
/// A click or Enter calls [onOpen] (the router path `/t/<id>`); a right click, the menu key or
/// Shift+F10 opens the thread's menu (M4-33). The row cannot be dragged (01 6.8 item 10).
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
  });

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
      // Below the menu region: its menu anchor binds the arrow keys to directional focus.
      child: Focus(
        canRequestFocus: false,
        skipTraversal: true,
        includeSemantics: false,
        onKeyEvent: onKeyEvent,
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
