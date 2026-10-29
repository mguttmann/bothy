// SPDX-License-Identifier: Apache-2.0

/// The view model of the sidebar and its pure rules (brief M4-19 Design notes 1 and 5;
/// components.md 7; 04 14): the counters of a channel, the muted state, the mapping to the
/// row data of M4-07, the thread group of a channel (01 6.8, M4-40) and the navigation between
/// channels and thread rows for M4-24 and M4-40.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';

import 'sidebar_order.dart';
import 'sidebar_threads.dart';

/// A row at the top of a forum's group that is not a thread yet (01 6.8 item 8, brief M4-41):
/// the draft row "New chat" of the person's draft of a new post, or a new post this device sent
/// and the server has not created yet ("sending", or "not sent" after a refusal, 01 6.6). It
/// counts toward neither the bound of the group nor the counters.
final class SidebarNewChatRow {
  /// The draft row of the forum.
  const SidebarNewChatRow.draft() : entryId = null, title = '', failed = false;

  /// The row of the sent post of outbox entry [entryId] with its [title] ('' without one);
  /// [failed] after the server refused it.
  const SidebarNewChatRow.sent({
    required String this.entryId,
    required this.title,
    this.failed = false,
  });

  /// The outbox entry of a sent post, or null for the draft row.
  final String? entryId;

  /// The title of a sent post; empty reads "New chat".
  final String title;

  /// Whether the server refused the sent post.
  final bool failed;

  /// Whether it is the draft row.
  bool get isDraft => entryId == null;

  /// The row data state of `DsSidebarDraftRow`.
  DsDraftRowState get state => isDraft
      ? DsDraftRowState.draft
      : (failed ? DsDraftRowState.failed : DsDraftRowState.sending);
}

/// A channel row of the sidebar with what it shows.
final class SidebarChannel {
  /// Creates the entry.
  const SidebarChannel({
    required this.channel,
    required this.counts,
    required this.runActive,
    required this.muted,
    this.group,
    this.expanded = true,
    this.newChatRows = const [],
  });

  /// The rows of a forum above its thread rows that are not threads yet: the draft row and the
  /// sent posts (01 6.8 item 8, M4-41); they show while the group is expanded and count
  /// toward neither the bound nor the counters.
  final List<SidebarNewChatRow> newChatRows;

  /// The channel.
  final Channel channel;

  /// Unread messages and unread mentions of the person in the channel (04 14): every thread,
  /// whether shown below the channel or not.
  final ThreadCounts counts;

  /// The thread group of the channel (01 6.8), or null where no group is built (tests).
  final SidebarThreadGroup? group;

  /// Whether the thread group is expanded on this device (01 6.8 item 7; default expanded).
  final bool expanded;

  /// Whether the channel shows thread rows or new chat rows (and so its own arrow, 01 6.8
  /// item 7).
  bool get hasGroup => _hasThreadRows || newChatRows.isNotEmpty;

  bool get _hasThreadRows => group?.hasRows ?? false;

  /// The thread rows on screen: those of an expanded group, else none.
  List<SidebarThread> get visibleThreads =>
      _hasThreadRows && expanded ? group!.rows : const [];

  /// The new chat rows on screen: those of an expanded group, else none.
  List<SidebarNewChatRow> get visibleNewChatRows =>
      expanded ? newChatRows : const [];

  /// The counters the channel row shows: with thread rows those of D-SB-5 (an expanded group
  /// counts only what no row below it shows), else [counts]; new chat rows count nothing.
  ThreadCounts get rowCounts => _hasThreadRows ? group!.channelCounts : counts;

  /// The "Show more" row of an expanded forum group with more open posts than it shows (01 6.8
  /// item 3, M4-46), or null.
  SidebarMorePosts? get morePosts =>
      _hasThreadRows && expanded ? group!.more : null;

  /// The counters of "All posts" of an expanded forum group, or null when it has none.
  ThreadCounts? get allPostsCounts =>
      _hasThreadRows && expanded ? group!.allPostsCounts : null;

  /// Whether a run of the channel has not ended (`runs`).
  final bool runActive;

  /// Whether the person muted the channel itself ([isMuted]).
  final bool muted;

  /// Whether the channel has something unread (the unread state, or a mention).
  bool get hasUnread => counts.unread > 0 || counts.mentions > 0;

  /// The row data of `DsChannelRow` (M4-07) with every thread counted (no group shown); the
  /// row builds the label `<Name>, <kind>, <n> unread, <m> mentions, run active` from it.
  DsChannelRowData get rowData => _row(counts);

  /// The row data of `DsChannelRow` above its thread group: the counters of [rowCounts]
  /// (01 6.8 item 6); the run dot stays an aggregate of every thread.
  DsChannelRowData get treeRowData => _row(rowCounts);

  DsChannelRowData _row(ThreadCounts shown) => DsChannelRowData(
    id: channel.id.value,
    name: channel.name,
    kind: DsChannelKind.fromCode(channel.kind.code),
    projectBound: channel.projectBound,
    unreadCount: shown.unread,
    mentionCount: shown.mentions,
    runActive: runActive,
    muted: muted,
  );
}

/// A space node of the tree: the space in the person's order with its channels.
final class SidebarSpaceNode {
  /// Creates the node.
  const SidebarSpaceNode({required this.entry, required this.channels});

  /// The space with its personal state.
  final SidebarSpace entry;

  /// The channels of the space in their order.
  final List<SidebarChannel> channels;

  /// Whether the space shows its channels.
  bool get expanded => !entry.collapsed;

  /// A copy with [channels].
  SidebarSpaceNode copyWithChannels(List<SidebarChannel> channels) =>
      SidebarSpaceNode(entry: entry, channels: channels);

  /// The row data of `DsSpaceRow` (M4-07): icon and category of the space, the expanded state
  /// and whether a channel has unread items (the dot of a collapsed space, components.md 7).
  DsSpaceRowData get rowData => DsSpaceRowData(
    id: entry.space.id.value,
    name: entry.space.name,
    icon: entry.space.icon ?? '',
    category: DsCategory.fromCode(entry.space.color),
    expanded: expanded,
    hasUnread: channels.any((c) => c.hasUnread),
  );
}

/// The counters of [channel] (04 14): for every thread the engine counts ([engine], from
/// the snapshot, `read_state.updated` and its own counting of `message.*`) its value; for
/// every other thread of the channel with a stored read state ([readStates]) the server's
/// counters stored there (`unread_count` where delivered, `mention_count`). Never derived
/// from grants.
///
/// Example: `channelCounts(engine: engine.catchUp.countsOfChannel(id), readStates: rows,
/// channel: id)`.
ThreadCounts channelCounts({
  required Map<ThreadId, ThreadCounts> engine,
  required Iterable<ReadState> readStates,
  required ChannelId channel,
}) {
  var unread = 0;
  var mentions = 0;
  for (final c in engine.values) {
    unread += c.unread;
    mentions += c.mentions;
  }
  for (final r in readStates) {
    if (r.channelId != channel || engine.containsKey(r.threadId)) continue;
    unread += r.unreadCount ?? 0;
    mentions += r.mentionCount;
  }
  return ThreadCounts(unread: unread, mentions: mentions);
}

/// Whether the person muted [channel]: the channel's own row of `notification_preferences`
/// gives `mute` after 13 3.1, that is its level is `mute` or its `muted_until` lies after
/// [now] (a passed `muted_until` has no effect, D-M3-18-3; "Mute until" keeps the level that
/// applies afterwards, docs/DECISIONS.md D-M4-33-2). Levels inherited from the space are shown
/// by the channel menu of M4-33, not here (brief M4-19 Design note 1).
bool isMuted(
  ChannelId channel,
  Iterable<NotificationPreference> preferences, {
  required DateTime now,
}) => preferences.any(
  (p) =>
      p.targetKind == NotificationTargetKind.channel &&
      p.targetId == channel.value &&
      isMutedTarget(p, now: now),
);

/// Whether the row [preference] mutes its own target at [now]: its level is `mute`, or its
/// `muted_until` lies after [now] (13 3.1; D-M3-18-3, D-M4-33-2). The thread rows of the sidebar
/// use it for the thread's own row (01 6.8 item 6).
bool isMutedTarget(
  NotificationPreference preference, {
  required DateTime now,
}) =>
    preference.level == NotificationLevel.mute ||
    (preference.mutedUntil != null && preference.mutedUntil!.isAfter(now));

/// A place the sidebar navigation can open: a channel, or a thread row below it (01 7, 6.8).
final class SidebarTarget {
  /// Creates the target; [thread] is null for the channel row itself.
  const SidebarTarget(this.channel, [this.thread]);

  /// The channel of the row.
  final ChannelId channel;

  /// The thread of a thread row, or null for a channel row.
  final ThreadId? thread;

  /// The router path: `/t/<id>` for a thread row, `/c/<id>` for a channel row.
  String get path =>
      thread == null ? '/c/${channel.value}' : '/t/${thread!.value}';

  @override
  bool operator ==(Object other) =>
      other is SidebarTarget &&
      other.channel == channel &&
      other.thread == thread;

  @override
  int get hashCode => Object.hash(channel, thread);

  @override
  String toString() => path;
}

/// The row after (or, with [forward] false, before) [current] in [tree], wrapping at the
/// ends; null when there is none.
///
/// Rows are the channels and the thread rows of their expanded groups, in the order of the
/// sidebar (01 7: "Next or previous channel or thread row in the sidebar"; D-SB-8); "All posts"
/// is no target. Without [unreadOnly] only rows of expanded spaces count (the ones on screen);
/// with it every channel whose row or "All posts" shows unread items, also in collapsed spaces,
/// and every unread thread row (01 7: next or previous unread channel or thread row). A
/// [current] that is not in the list starts from the beginning (or the end); a thread that is
/// not a row stands for its channel.
SidebarTarget? neighbourTarget(
  List<SidebarSpaceNode> tree,
  SidebarTarget? current, {
  required bool forward,
  bool unreadOnly = false,
}) {
  final rows = <(SidebarTarget, bool)>[];
  for (final node in tree) {
    if (!unreadOnly && !node.expanded) continue;
    for (final c in node.channels) {
      final shown = node.expanded ? c.visibleThreads : const <SidebarThread>[];
      // In a collapsed space every count of the channel is out of view.
      final rowCounts = node.expanded ? c.rowCounts : c.counts;
      final rest = node.expanded ? c.allPostsCounts : null;
      final channelUnread =
          _unread(rowCounts) || (rest != null && _unread(rest));
      rows.add((SidebarTarget(c.channel.id), channelUnread));
      for (final t in shown) {
        rows.add((SidebarTarget(c.channel.id, t.thread.id), _unread(t.counts)));
      }
    }
  }
  var at = current;
  if (at != null && at.thread != null && !rows.any((r) => r.$1 == at)) {
    at = SidebarTarget(at.channel);
  }
  final candidates = [
    for (final r in rows)
      if (!unreadOnly || r.$2 || r.$1 == at) r.$1,
  ];
  final index = at == null ? -1 : candidates.indexOf(at);
  final pool = [
    for (final c in candidates)
      if (c != at) c,
  ];
  if (pool.isEmpty) return null;
  if (index < 0) return forward ? pool.first : pool.last;
  final n = candidates.length;
  return candidates[(index + (forward ? 1 : -1) + n) % n];
}

bool _unread(ThreadCounts c) => c.unread > 0 || c.mentions > 0;

/// The navigation between the rows of the sidebar that M4-24 binds (01 7: next or previous
/// channel or thread row, next or previous unread channel or thread row; M4-40): it reads the
/// current tree and selection and calls [open] with the row to show.
///
/// Example:
/// ```dart
/// final nav = SidebarNavigation(tree: () => tree, selected: () => current, open: go);
/// nav.nextUnread();
/// ```
final class SidebarNavigation {
  /// Creates the controller.
  const SidebarNavigation({
    required this.tree,
    required this.selected,
    required this.open,
  });

  /// The tree as the sidebar shows it.
  final List<SidebarSpaceNode> Function() tree;

  /// The selected row (the channel and the thread of the route), or null.
  final SidebarTarget? Function() selected;

  /// Opens a row (its [SidebarTarget.path]).
  final void Function(SidebarTarget target) open;

  /// Opens the next row; returns whether one was found.
  bool next() => _go(forward: true, unreadOnly: false);

  /// Opens the previous row.
  bool previous() => _go(forward: false, unreadOnly: false);

  /// Opens the next row with unread items.
  bool nextUnread() => _go(forward: true, unreadOnly: true);

  /// Opens the previous row with unread items.
  bool previousUnread() => _go(forward: false, unreadOnly: true);

  bool _go({required bool forward, required bool unreadOnly}) {
    final target = neighbourTarget(
      tree(),
      selected(),
      forward: forward,
      unreadOnly: unreadOnly,
    );
    if (target == null) return false;
    open(target);
    return true;
  }
}

/// The new chat row the route selects (brief M4-41): the draft row of [forum] while its new chat
/// view writes ([entry] null), or the row of the sent post of outbox entry [entry].
final class SidebarNewChatSelection {
  /// Creates the selection.
  const SidebarNewChatSelection(this.forum, {this.entry});

  /// The forum.
  final ChannelId forum;

  /// The outbox entry of the sent post, or null for the draft row.
  final String? entry;

  /// Whether [row] of [channel] is the selected row.
  bool selects(ChannelId channel, SidebarNewChatRow row) =>
      channel == forum && row.entryId == entry;
}
