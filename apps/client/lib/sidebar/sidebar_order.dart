// SPDX-License-Identifier: Apache-2.0

/// The personal order of the spaces in the sidebar and the keys a drag writes (components.md 7;
/// 04 14; brief M4-19 Design note 2; docs/DECISIONS.md D-M4-19-2).
library;

import 'package:domain/domain.dart';

/// A space with the person's sidebar state (03 `user_space_states`) and whether it is
/// collapsed on this device.
final class SidebarSpace {
  /// Creates the entry of [space] with its [state] (null: never changed by the person) and its
  /// [collapsed] state on this device.
  const SidebarSpace({required this.space, this.state, this.collapsed = false});

  /// The space.
  final Space space;

  /// The synchronized personal state, or null; the sidebar uses its order only.
  final UserSpaceState? state;

  /// Whether the person collapsed the space on this device (`space_view_states`; 01 6.7,
  /// R-PR-24): another device of the person never changes it.
  final bool collapsed;

  /// The key the sidebar sorts by: the personal `sort_key`, else the global one (04 14).
  String get orderKey => state?.sortKey ?? space.sortKey;

  /// The `collapsed` the server holds for the space; a write of the order sends it unchanged
  /// (the API `SidebarStatePut` needs it), false without a state.
  bool get storedCollapsed => state?.collapsed ?? false;
}

/// One write of the personal order: [sortKey] for [spaceId], keeping the [collapsed] the server
/// holds (the API `SidebarStatePut` needs both).
final class SpaceOrderWrite {
  /// Creates the write.
  const SpaceOrderWrite({
    required this.spaceId,
    required this.sortKey,
    required this.collapsed,
  });

  /// The space.
  final SpaceId spaceId;

  /// Its new personal key.
  final String sortKey;

  /// The collapse state the server holds, unchanged by the write.
  final bool collapsed;

  @override
  bool operator ==(Object other) =>
      other is SpaceOrderWrite &&
      other.spaceId == spaceId &&
      other.sortKey == sortKey &&
      other.collapsed == collapsed;

  @override
  int get hashCode => Object.hash(spaceId, sortKey, collapsed);

  @override
  String toString() =>
      'SpaceOrderWrite(${spaceId.value}, $sortKey, collapsed: $collapsed)';
}

/// The spaces the sidebar shows, in the person's order; a space in [collapsed] (the collapsed
/// spaces of this device) is collapsed.
///
/// Archived spaces are left out. The order is [SidebarSpace.orderKey] compared as bytes (the
/// server's `COLLATE "C"`), then the global key, then the ID, so every device shows the same
/// order for the same data.
///
/// Example: `orderSpaces(spaces, states).first.space.name`.
List<SidebarSpace> orderSpaces(
  Iterable<Space> spaces,
  Iterable<UserSpaceState> states, {
  Set<SpaceId> collapsed = const {},
}) {
  final byId = {for (final s in states) s.spaceId: s};
  final list = [
    for (final s in spaces)
      if (s.archivedAt == null)
        SidebarSpace(
          space: s,
          state: byId[s.id],
          collapsed: collapsed.contains(s.id),
        ),
  ];
  list.sort((a, b) {
    final byKey = a.orderKey.compareTo(b.orderKey);
    if (byKey != 0) return byKey;
    final byGlobal = a.space.sortKey.compareTo(b.space.sortKey);
    return byGlobal != 0
        ? byGlobal
        : a.space.id.value.compareTo(b.space.id.value);
  });
  return list;
}

/// The writes that move [spaceId] to index [to] of [order] (the index in the list without
/// the moved space; clamped to its bounds).
///
/// Usually one write: a key strictly between the new neighbours (`FractionalIndex.between`,
/// the rule the server validates, so the write never fails with `400`). When the neighbours
/// leave no room (equal keys, or a global key outside the fractional index), every space
/// gets a fresh key in the new order and each space whose key changes is written. Returns an
/// empty list when the space stays where it is or is not in [order].
List<SpaceOrderWrite> moveSpace(
  List<SidebarSpace> order,
  SpaceId spaceId,
  int to,
) {
  final from = order.indexWhere((s) => s.space.id == spaceId);
  if (from < 0) return const [];
  final rest = [...order]..removeAt(from);
  final at = to.clamp(0, rest.length);
  if (at == from) return const [];
  final moving = order[from];
  final before = at > 0 ? rest[at - 1].orderKey : null;
  final after = at < rest.length ? rest[at].orderKey : null;
  try {
    return [
      SpaceOrderWrite(
        spaceId: spaceId,
        sortKey: FractionalIndex.between(before, after),
        collapsed: moving.storedCollapsed,
      ),
    ];
  } on ArgumentError {
    final next = [...rest]..insert(at, moving);
    final writes = <SpaceOrderWrite>[];
    String? key;
    for (final s in next) {
      key = FractionalIndex.between(key, null);
      if (s.state?.sortKey != key) {
        writes.add(
          SpaceOrderWrite(
            spaceId: s.space.id,
            sortKey: key,
            collapsed: s.storedCollapsed,
          ),
        );
      }
    }
    return writes;
  }
}
