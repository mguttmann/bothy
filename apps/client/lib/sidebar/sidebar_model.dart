// SPDX-License-Identifier: Apache-2.0

/// The view model of the sidebar and its pure rules (brief M4-19 Design notes 1 and 5;
/// components.md 7; 04 14): the counters of a channel, the muted state, the mapping to the
/// row data of M4-07 and the navigation between channels for M4-24.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';

import 'sidebar_order.dart';

/// A channel row of the sidebar with what it shows.
final class SidebarChannel {
  /// Creates the entry.
  const SidebarChannel({
    required this.channel,
    required this.counts,
    required this.runActive,
    required this.muted,
  });

  /// The channel.
  final Channel channel;

  /// Unread messages and unread mentions of the person in the channel (04 14).
  final ThreadCounts counts;

  /// Whether a run of the channel has not ended (`runs`).
  final bool runActive;

  /// Whether the person muted the channel itself ([isMuted]).
  final bool muted;

  /// Whether the channel has something unread (the unread state, or a mention).
  bool get hasUnread => counts.unread > 0 || counts.mentions > 0;

  /// The row data of `DsChannelRow` (M4-07); the row builds the label
  /// `<Name>, <n> unread, <m> mentions, run active` from it.
  DsChannelRowData get rowData => DsChannelRowData(
    id: channel.id.value,
    name: channel.name,
    kind: DsChannelKind.fromCode(channel.kind.code),
    unreadCount: counts.unread,
    mentionCount: counts.mentions,
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
/// with level `mute` whose `muted_until` is absent or after [now]. Levels inherited from the
/// space are shown by the channel menu of M4-33, not here (brief M4-19 Design note 1).
bool isMuted(
  ChannelId channel,
  Iterable<NotificationPreference> preferences, {
  required DateTime now,
}) => preferences.any(
  (p) =>
      p.targetKind == NotificationTargetKind.channel &&
      p.targetId == channel.value &&
      p.level == NotificationLevel.mute &&
      (p.mutedUntil == null || p.mutedUntil!.isAfter(now)),
);

/// The channel after (or, with [forward] false, before) [current] in [tree], wrapping at the
/// ends; null when there is none.
///
/// Without [unreadOnly] only channels of expanded spaces count (the ones on screen); with it
/// every channel with unread items, also in collapsed spaces (01 7: next or previous unread
/// channel). A [current] that is not in the list starts from the beginning (or the end).
ChannelId? neighbourChannel(
  List<SidebarSpaceNode> tree,
  ChannelId? current, {
  required bool forward,
  bool unreadOnly = false,
}) {
  final all = [
    for (final node in tree)
      if (unreadOnly || node.expanded)
        for (final c in node.channels) c,
  ];
  final candidates = [
    for (final c in all)
      if (!unreadOnly || c.hasUnread || c.channel.id == current) c,
  ];
  final index = candidates.indexWhere((c) => c.channel.id == current);
  final pool = [
    for (final c in candidates)
      if (c.channel.id != current) c,
  ];
  if (pool.isEmpty) return null;
  if (index < 0) {
    return (forward ? pool.first : pool.last).channel.id;
  }
  final n = candidates.length;
  final next = (index + (forward ? 1 : -1) + n) % n;
  return candidates[next].channel.id;
}

/// The navigation between channels the sidebar offers M4-24 (01 7: next or previous channel,
/// next or previous unread channel): it reads the current tree and selection and calls
/// [open] with the channel to show.
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

  /// The selected channel, or null.
  final ChannelId? Function() selected;

  /// Opens a channel (the router path `/c/<id>`).
  final void Function(ChannelId channel) open;

  /// Opens the next channel; returns whether one was found.
  bool next() => _go(forward: true, unreadOnly: false);

  /// Opens the previous channel.
  bool previous() => _go(forward: false, unreadOnly: false);

  /// Opens the next channel with unread items.
  bool nextUnread() => _go(forward: true, unreadOnly: true);

  /// Opens the previous channel with unread items.
  bool previousUnread() => _go(forward: false, unreadOnly: true);

  bool _go({required bool forward, required bool unreadOnly}) {
    final target = neighbourChannel(
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
