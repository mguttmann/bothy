// SPDX-License-Identifier: Apache-2.0

/// The ordering rules of the sidebar levels (docs/spec/01-product.md 6.15; brief M4-51; design
/// note sidebar-ordering.md D-M4-51-1 to D-M4-51-9): the manual order of a level as a personal
/// list, the automatic order by activity, the held order while the person is in the sidebar and
/// the move of one row. Pure functions, shared by the desktop and the phone.
library;

import 'package:domain/domain.dart';

/// A level of the sidebar that has its own order (D-M4-51-1).
enum SidebarLevel {
  /// The spaces.
  spaces,

  /// The channels and forums of one space.
  channels,

  /// The open posts of one forum.
  chats,
}

/// Which levels are arranged by activity (the settings `sidebar.auto_arrange_*` of 01 11,
/// D-M4-51-5); a level that is not automatic shows its manual order.
final class SidebarArrangement {
  /// Creates the arrangement; the defaults are those of 01 11.
  const SidebarArrangement({
    this.autoSpaces = false,
    this.autoChannels = false,
    this.autoChats = true,
  });

  /// `sidebar.auto_arrange_spaces`.
  final bool autoSpaces;

  /// `sidebar.auto_arrange_channels`.
  final bool autoChannels;

  /// `sidebar.auto_arrange_chats`.
  final bool autoChats;

  /// Whether [level] is arranged by activity (and so cannot be dragged).
  bool isAuto(SidebarLevel level) => switch (level) {
    SidebarLevel.spaces => autoSpaces,
    SidebarLevel.channels => autoChannels,
    SidebarLevel.chats => autoChats,
  };

  @override
  bool operator ==(Object other) =>
      other is SidebarArrangement &&
      other.autoSpaces == autoSpaces &&
      other.autoChannels == autoChannels &&
      other.autoChats == autoChats;

  @override
  int get hashCode => Object.hash(autoSpaces, autoChannels, autoChats);
}

/// [byDefault] in the person's [order] (D-M4-51-3): the rows whose ID [idOf] gives is not in
/// [order] come first in their default order, then the rows of [order] in its order. IDs of
/// [order] that name no row are ignored; null or empty keeps [byDefault].
///
/// Example: `applyPersonalOrder(['a', 'b', 'c'], ['c', 'a'], (s) => s)` is `['b', 'c', 'a']`.
List<T> applyPersonalOrder<T, K>(
  List<T> byDefault,
  List<K>? order,
  K Function(T) idOf,
) {
  if (order == null || order.isEmpty) return byDefault;
  final place = <K, int>{};
  for (final (i, id) in order.indexed) {
    place.putIfAbsent(id, () => i);
  }
  final unplaced = <T>[];
  final placed = <T>[];
  for (final row in byDefault) {
    (place.containsKey(idOf(row)) ? placed : unplaced).add(row);
  }
  placed.sort((a, b) => place[idOf(a)]!.compareTo(place[idOf(b)]!));
  return [...unplaced, ...placed];
}

/// [rows] ordered by their activity [at], newest first; equal times keep the order of [rows]
/// (the manual order breaks ties, D-M4-51-4).
List<T> orderByActivity<T>(List<T> rows, DateTime Function(T) at) {
  final indexed = [...rows.indexed];
  indexed.sort((a, b) {
    final byTime = at(b.$2).compareTo(at(a.$2));
    return byTime != 0 ? byTime : a.$1.compareTo(b.$1);
  });
  return [for (final (_, row) in indexed) row];
}

/// [fresh] while the sidebar holds its rows (01 6.8 item 4, D-M4-51-4): the rows of [held] (the
/// IDs shown last, in their order) keep that order; rows new to the level come first in the
/// order of [fresh]. Without [held] it is [fresh].
List<T> holdOrder<T, K>(List<T> fresh, List<K> held, K Function(T) idOf) {
  if (held.isEmpty) return fresh;
  final place = {for (final (i, id) in held.indexed) id: i};
  final newRows = [
    for (final r in fresh)
      if (!place.containsKey(idOf(r))) r,
  ];
  final kept = [
    for (final r in fresh)
      if (place.containsKey(idOf(r))) r,
  ]..sort((a, b) => place[idOf(a)]!.compareTo(place[idOf(b)]!));
  return [...newRows, ...kept];
}

/// The order [full] after [item] moved to index [to] among the rows on screen [visible] (the
/// index in [visible] without [item], clamped to its bounds; D-M4-51-9).
///
/// Rows of [full] that are not on screen (behind "Show more") keep their place relative to the
/// others: the item lands right before the visible row that will follow it, or right after the
/// last visible row. A row on screen that is not in [full] (a selected done post, a row the
/// sidebar holds) is no place to land on: the item lands before the next visible row of [full],
/// or after the last one before it. An [item] not in [full] or [visible] changes nothing.
///
/// Example: `moveAmong(['a', 'b', 'c', 'd'], ['a', 'b'], 'a', 1)` is `['b', 'a', 'c', 'd']`.
List<K> moveAmong<K>(List<K> full, List<K> visible, K item, int to) {
  if (!full.contains(item) || !visible.contains(item)) return full;
  final others = [
    for (final v in visible)
      if (v != item) v,
  ];
  final at = to.clamp(0, others.length);
  final rest = [
    for (final f in full)
      if (f != item) f,
  ];
  // The first row of the order at or after the place, else the last one before it.
  final after = others.skip(at).where(full.contains).firstOrNull;
  if (after != null) {
    rest.insert(rest.indexOf(after), item);
    return rest;
  }
  final before = others.take(at).where(full.contains).lastOrNull;
  if (before == null) return full;
  rest.insert(rest.indexOf(before) + 1, item);
  return rest;
}

/// The activity of [channel] (D-M4-51-4): the newest `last_activity_at` of its [threads] the
/// client holds (others than the channel's are ignored; the main history counts), else its
/// creation time.
DateTime channelActivity(Channel channel, Iterable<Thread> threads) {
  var newest = channel.createdAt;
  var any = false;
  for (final t in threads) {
    if (t.channelId != channel.id) continue;
    if (!any || t.lastActivityAt.isAfter(newest)) newest = t.lastActivityAt;
    any = true;
  }
  return newest;
}

/// The activity of [space] (D-M4-51-4): the newest of the [channels] activities of its channels,
/// else its creation time.
DateTime spaceActivity(Space space, Iterable<DateTime> channels) {
  DateTime? newest;
  for (final at in channels) {
    if (newest == null || at.isAfter(newest)) newest = at;
  }
  return newest ?? space.createdAt;
}
