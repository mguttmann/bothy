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
import 'sidebar_arrange.dart';
import 'sidebar_drag.dart';
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
/// expanded group, "Show more" or "Show less" ([onShowAllPosts], M4-46) and "All posts" at the
/// end of a forum's group, its own arrow ([onToggleGroup])
/// and on a forum the "+" action ([onAdd]). The arrow keys walk the tree (D-SB-8): arrow right
/// on the channel expands its group or enters its first row, arrow left collapses it; arrow left
/// on a thread row goes back to the channel. The open posts of a forum are dragged among each
/// other with [onMoveThread] unless chats are arranged by activity ([chatsArranged]); their
/// menus and screen reader actions have "Move up" and "Move down" (01 6.16, M4-51). The
/// channel's own [moves] go to its menu and its row's screen reader actions; the channel row is
/// dragged through [channelDrag] ([SidebarDragRow] of its space). Text channel threads keep their order (D-M4-51-1).
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
    this.selectedNewChat,
    this.onOpenNewChat,
    this.onShowAllPosts,
    this.moves,
    this.onMoveThread,
    this.chatsArranged = true,
    this.touch = false,
    this.channelDrag,
  });

  /// Wraps the channel row (only the row, not its thread group) so it is dragged among the
  /// channels of its space (01 6.16, M4-51), or null.
  final Widget Function(Widget row)? channelDrag;

  /// "Move up" and "Move down" of the channel (01 6.16, M4-51), or null for none.
  final SidebarMoves? moves;

  /// Moves an open post of the forum to an index among the post rows on screen (in the list
  /// without it), or null for no drag (01 6.16, M4-51).
  final void Function(ThreadId thread, int to)? onMoveThread;

  /// Whether the posts of the forum are arranged by activity ("Arrange chats automatically").
  final bool chatsArranged;

  /// Whether the screen is a touch screen: a long press starts a drag (D-M4-51-6).
  final bool touch;

  /// Lists every open post of a forum group (true, "Show more") or the newest ones (false,
  /// "Show less"); null shows no such row (01 6.8 item 3, M4-46).
  final ValueChanged<bool>? onShowAllPosts;

  /// The new chat row the route selects, or null (brief M4-41).
  final SidebarNewChatSelection? selectedNewChat;

  /// Opens a new chat row of this forum: its draft (null) or the sent post of an outbox entry;
  /// null shows none.
  final ValueChanged<String?>? onOpenNewChat;

  /// The channel with its counters, states and thread group.
  final SidebarChannel entry;

  /// Whether the route shows this channel or one of its threads.
  final bool selected;

  /// The thread the route shows, or null.
  final ThreadId? selectedThread;

  /// Opens the channel.
  final VoidCallback onOpen;

  /// Opens a thread of the group; null shows no group (tests of the tree without groups).
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
  final FocusNode _more = FocusNode(debugLabel: 'sidebar.morePosts');
  final Map<ThreadId, FocusNode> _threads = {};
  final Map<String, FocusNode> _newChats = {};

  @override
  void dispose() {
    _channel.dispose();
    _allPosts.dispose();
    _more.dispose();
    for (final n in [..._threads.values, ..._newChats.values]) {
      n.dispose();
    }
    super.dispose();
  }

  FocusNode _threadNode(ThreadId id) =>
      _threads.putIfAbsent(id, () => FocusNode(debugLabel: 'sidebar.thread'));

  /// The key of a new chat row: its outbox entry, or `draft`.
  static String _newChatKey(SidebarNewChatRow row) => row.entryId ?? 'draft';

  FocusNode _newChatNode(SidebarNewChatRow row) => _newChats.putIfAbsent(
    _newChatKey(row),
    () => FocusNode(debugLabel: 'sidebar.newChat'),
  );

  /// Drops the nodes of rows that left, so the maps do not grow with every row ever shown.
  void _prune(Iterable<ThreadId> shown, Iterable<String> newChats) {
    _dropAbsent(_threads, shown.toSet());
    _dropAbsent(_newChats, newChats.toSet());
  }

  static void _dropAbsent<K>(Map<K, FocusNode> nodes, Set<K> keep) {
    final gone = [
      for (final id in nodes.keys)
        if (!keep.contains(id)) id,
    ];
    for (final id in gone) {
      // The row that held it is gone from the tree in this build; dispose after the frame.
      final node = nodes.remove(id)!;
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
      final newChats = widget.onOpenNewChat == null
          ? const <SidebarNewChatRow>[]
          : e.visibleNewChatRows;
      if (!e.expanded) {
        toggle(true);
      } else if (newChats.isNotEmpty) {
        _newChatNode(newChats.first).requestFocus();
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

  /// The drag of the open forum post [r] at index [i] of [count] post rows (01 6.16, M4-51); a
  /// text channel thread, or a forum without [ChannelEntry.onMoveThread], is not wrapped.
  Widget _post(int i, int count, Widget row, SidebarThread r) {
    final move = widget.onMoveThread;
    if (move == null || widget.entry.channel.kind != ChannelKind.forum) {
      return row;
    }
    return SidebarDragRow(
      key: ValueKey('sidebar.dragPost.${r.thread.id.value}'),
      data: SidebarDragData(
        level: SidebarLevel.chats,
        parent: widget.entry.channel.id.value,
        id: r.thread.id.value,
        index: i,
      ),
      label: r.thread.title,
      enabled: !widget.chatsArranged,
      arranged: widget.chatsArranged,
      touch: widget.touch,
      onDrop: (id, to) => move(ThreadId.parse(id), to),
      child: row,
    );
  }

  SidebarMoves? _postMoves(ThreadId id, int i, int count) {
    final move = widget.onMoveThread;
    if (move == null || widget.entry.channel.kind != ChannelKind.forum) {
      return null;
    }
    if (widget.chatsArranged) return const SidebarMoves(arranged: true);
    return SidebarMoves(
      up: i > 0 ? () => move(id, i - 1) : null,
      down: i < count - 1 ? () => move(id, i + 1) : null,
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final w = widget;
    final e = w.entry;
    final grouped = w.onOpenThread != null;
    final rows = grouped ? e.visibleThreads : const <SidebarThread>[];
    final newChats = grouped && w.onOpenNewChat != null
        ? e.visibleNewChatRows
        : const <SidebarNewChatRow>[];
    _prune(
      [for (final r in rows) r.thread.id],
      [for (final r in newChats) _newChatKey(r)],
    );
    final newChat = w.selectedNewChat;
    final threadSelected =
        rows.any((r) => r.thread.id == w.selectedThread) ||
        (newChat != null && newChat.forum == e.channel.id);
    final forum = e.channel.kind == ChannelKind.forum;
    final rest = grouped ? e.allPostsCounts : null;
    final more = grouped && w.onShowAllPosts != null ? e.morePosts : null;
    final addLabel = e.channel.projectBound
        ? l.sidebarNewChatIn(e.channel.name)
        : l.sidebarNewPostIn(e.channel.name);
    final channelRowOnly = Padding(
      padding: const EdgeInsetsDirectional.only(
        start: DsSpace.s16,
        end: DsSpace.s4,
      ),
      // The keys are handled below the menu region: its menu anchor binds the arrow keys to
      // directional focus for every row inside it.
      child: ChannelMenu(
        channel: e.channel,
        // 01 6.8 item 8: the context menu of a forum has the same entry as "+".
        newChatLabel: forum && w.onAdd != null ? addLabel : null,
        onNewChat: forum ? w.onAdd : null,
        moves: w.moves,
        child: Focus(
          canRequestFocus: false,
          skipTraversal: true,
          includeSemantics: false,
          onKeyEvent: _onChannelKey,
          child: Semantics(
            customSemanticsActions: w.moves?.semanticsActions(l),
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
              addLabel: forum && w.onAdd != null ? addLabel : null,
              onAdd: forum ? w.onAdd : null,
            ),
          ),
        ),
      ),
    );
    // Only the channel row lifts the channel: a drag on one of its posts must never move the
    // forum (review of M4-51).
    final channelRow = (w.channelDrag ?? (row) => row)(channelRowOnly);
    if (rows.isEmpty && newChats.isEmpty) return channelRow;
    // The channel with its rows is one node inside the space's group, and the rows one node
    // below the channel row, so VoiceOver and TalkBack get the levels space, channel, thread as
    // nesting of the semantics tree (components.md 7; brief M5-15 requirement 15): Flutter has
    // no tree role or level to set.
    return Semantics(
      container: true,
      explicitChildNodes: true,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          channelRow,
          Semantics(
            container: true,
            explicitChildNodes: true,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                // The draft row and the sent posts at the top of the group (01 6.8 item 8,
                // M4-41).
                for (final (i, r) in newChats.indexed)
                  NewChatEntry(
                    channel: e.channel.id,
                    row: r,
                    selected: newChat?.selects(e.channel.id, r) ?? false,
                    last:
                        rest == null &&
                        rows.isEmpty &&
                        i == newChats.length - 1,
                    onOpen: () => w.onOpenNewChat!(r.entryId),
                    focusNode: _newChatNode(r),
                    onKeyEvent: _onRowKey,
                  ),
                for (final (i, r) in rows.indexed)
                  _post(
                    i,
                    rows.length,
                    ThreadEntry(
                      entry: r,
                      channel: e.channel,
                      selected: r.thread.id == w.selectedThread,
                      last:
                          rest == null && more == null && i == rows.length - 1,
                      onOpen: () => w.onOpenThread!(r.thread.id),
                      focusNode: _threadNode(r.thread.id),
                      onKeyEvent: _onRowKey,
                      moves: _postMoves(r.thread.id, i, rows.length),
                    ),
                    r,
                  ),
                if (more != null)
                  MorePostsEntry(
                    channel: e.channel.id,
                    more: more,
                    onToggle: w.onShowAllPosts!,
                    focusNode: _more,
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
            ),
          ),
        ],
      ),
    );
  }
}
