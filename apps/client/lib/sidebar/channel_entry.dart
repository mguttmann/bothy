// SPDX-License-Identifier: Apache-2.0

/// A channel of the sidebar tree (components.md 7): `DsChannelRow` of M4-07 below its space,
/// and below it the channel's thread group (01 6.8, M4-40).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

import '../menus/channel_menu.dart';
import 'sidebar_model.dart';
import 'sidebar_threads.dart';
import 'thread_entry.dart';

/// The row of [entry], indented below its space, with its thread group.
///
/// The row shows the states of components.md 7 (read, unread, mentions, active run, muted,
/// selected), the icon of the channel's kind with the kind sentence as its tooltip (01 6.8 item
/// 11), and the label `<Name>, <kind>, <n> unread, <m> mentions, run active`; a click or Enter
/// calls [onOpen] (the router path `/c/<id>`, which shows a forum's overview D15). A right
/// click, the menu key or Shift+F10 opens the channel's menu (M4-33).
///
/// With [onOpenThread] the channel shows its thread group (01 6.8): the thread rows of an
/// expanded group, "All posts" at the end of a forum's group, its own arrow ([onToggleGroup])
/// and on a forum the "+" action ([onAdd]). The arrow keys walk the tree (D-SB-8): arrow right
/// on the channel expands its group or enters its first row, arrow left collapses it; arrow left
/// on a thread row goes back to the channel. Neither the channel nor its rows can be dragged
/// (01 6.8 item 10).
class ChannelEntry extends StatefulWidget {
  /// Creates the entry.
  const ChannelEntry({
    super.key,
    required this.entry,
    required this.selected,
    required this.onOpen,
    this.selectedThread,
    this.onOpenThread,
    this.onToggleGroup,
    this.onAdd,
    this.onParent,
  });

  /// The channel with its counters, states and thread group.
  final SidebarChannel entry;

  /// Whether the route shows this channel or one of its threads.
  final bool selected;

  /// The thread the route shows, or null.
  final ThreadId? selectedThread;

  /// Opens the channel.
  final VoidCallback onOpen;

  /// Opens a thread of the group; null shows no group (the phone until M5-15).
  final ValueChanged<ThreadId>? onOpenThread;

  /// Collapses or expands the group with the requested expanded state.
  final ValueChanged<bool>? onToggleGroup;

  /// The "+" action of a forum (01 6.8 item 8), or null for none.
  final VoidCallback? onAdd;

  /// Moves the focus to the parent row, the space (arrow left, D-SB-8), or null.
  final VoidCallback? onParent;

  @override
  State<ChannelEntry> createState() => _ChannelEntryState();
}

class _ChannelEntryState extends State<ChannelEntry> {
  final FocusNode _channel = FocusNode(debugLabel: 'sidebar.channel');
  final FocusNode _allPosts = FocusNode(debugLabel: 'sidebar.allPosts');
  final Map<ThreadId, FocusNode> _threads = {};

  @override
  void dispose() {
    _channel.dispose();
    _allPosts.dispose();
    for (final n in _threads.values) {
      n.dispose();
    }
    super.dispose();
  }

  FocusNode _threadNode(ThreadId id) =>
      _threads.putIfAbsent(id, () => FocusNode(debugLabel: 'sidebar.thread'));

  /// Drops the nodes of rows that left, so the map does not grow with every thread ever shown.
  void _prune(Iterable<ThreadId> shown) {
    final keep = shown.toSet();
    final gone = [
      for (final id in _threads.keys)
        if (!keep.contains(id)) id,
    ];
    for (final id in gone) {
      // The row that held it is gone from the tree in this build; dispose after the frame.
      final node = _threads.remove(id)!;
      WidgetsBinding.instance.addPostFrameCallback((_) => node.dispose());
    }
  }

  KeyEventResult _onChannelKey(FocusNode _, KeyEvent event) {
    if (event is! KeyDownEvent || !_channel.hasPrimaryFocus) {
      return KeyEventResult.ignored;
    }
    final e = widget.entry;
    final toggle = widget.onToggleGroup;
    final grouped = toggle != null && e.hasGroup;
    if (event.logicalKey == LogicalKeyboardKey.arrowRight) {
      if (!grouped) return KeyEventResult.ignored;
      if (!e.expanded) {
        toggle(true);
      } else if (e.visibleThreads.isNotEmpty) {
        _threadNode(e.visibleThreads.first.thread.id).requestFocus();
      }
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.arrowLeft) {
      // An expanded group collapses; otherwise the focus goes to the space (D-SB-8).
      if (grouped && e.expanded) {
        toggle(false);
        return KeyEventResult.handled;
      }
      final parent = widget.onParent;
      if (parent == null) return KeyEventResult.ignored;
      parent();
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  KeyEventResult _onRowKey(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent) return KeyEventResult.ignored;
    if (event.logicalKey == LogicalKeyboardKey.arrowLeft) {
      _channel.requestFocus();
      return KeyEventResult.handled;
    }
    // A thread row has no group of its own (01 3.1: no threads in threads).
    if (event.logicalKey == LogicalKeyboardKey.arrowRight) {
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final w = widget;
    final e = w.entry;
    final grouped = w.onOpenThread != null;
    final rows = grouped ? e.visibleThreads : const <SidebarThread>[];
    _prune([for (final r in rows) r.thread.id]);
    final threadSelected = rows.any((r) => r.thread.id == w.selectedThread);
    final forum = e.channel.kind == ChannelKind.forum;
    final rest = grouped ? e.allPostsCounts : null;
    final channelRow = Padding(
      padding: const EdgeInsetsDirectional.only(
        start: DsSpace.s16,
        end: DsSpace.s4,
      ),
      // The keys are handled below the menu region: its menu anchor binds the arrow keys to
      // directional focus for every row inside it.
      child: ChannelMenu(
        channel: e.channel,
        child: Focus(
          canRequestFocus: false,
          skipTraversal: true,
          includeSemantics: false,
          onKeyEvent: _onChannelKey,
          child: DsChannelRow(
            grouped ? e.treeRowData : e.rowData,
            key: ValueKey('sidebar.channel.${e.channel.id.value}'),
            // With a thread row of this channel selected, the row is the one marked (D1).
            selected: w.selected && !threadSelected,
            // The forum of the open post still offers "+" (D1: `{} server [+]`).
            current: w.selected,
            onOpen: w.onOpen,
            focusNode: _channel,
            groupExpanded: grouped && e.hasGroup ? e.expanded : null,
            onToggleGroup: grouped && e.hasGroup ? w.onToggleGroup : null,
            addLabel: forum && w.onAdd != null
                ? (e.channel.projectBound
                      ? l.sidebarNewChatIn(e.channel.name)
                      : l.sidebarNewPostIn(e.channel.name))
                : null,
            onAdd: forum ? w.onAdd : null,
          ),
        ),
      ),
    );
    if (rows.isEmpty) return channelRow;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        channelRow,
        for (final (i, r) in rows.indexed)
          ThreadEntry(
            entry: r,
            channel: e.channel,
            selected: r.thread.id == w.selectedThread,
            last: rest == null && i == rows.length - 1,
            onOpen: () => w.onOpenThread!(r.thread.id),
            focusNode: _threadNode(r.thread.id),
            onKeyEvent: _onRowKey,
          ),
        if (rest != null)
          AllPostsEntry(
            channel: e.channel.id,
            counts: rest,
            onOpen: w.onOpen,
            focusNode: _allPosts,
            onKeyEvent: _onRowKey,
          ),
      ],
    );
  }
}
