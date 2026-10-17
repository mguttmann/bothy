// SPDX-License-Identifier: Apache-2.0

/// Spaces and the personal sidebar state (03 `spaces`, `user_space_states`; 15 6.1).
library;

import '../ids.dart';
import '../seq.dart';

/// A space, the top-level group of the sidebar (03 `spaces`).
final class Space {
  /// Creates a [Space].
  const Space({
    required this.id,
    required this.name,
    required this.sortKey,
    required this.createdAt,
    required this.version,
    this.icon,
    this.color,
    this.defaultOperatorAgentId,
    this.archivedAt,
  });

  /// ID.
  final SpaceId id;

  /// Name.
  final String name;

  /// Glyph name from the design tokens, if any.
  final String? icon;

  /// Key of a category color, if any.
  final String? color;

  /// Global order (compared as bytes).
  final String sortKey;

  /// Default operator agent, if any.
  final PrincipalId? defaultOperatorAgentId;

  /// Creation time.
  final DateTime createdAt;

  /// Archive time, if archived.
  final DateTime? archivedAt;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  Space copyWith({
    String? name,
    String? icon,
    String? color,
    String? sortKey,
    PrincipalId? defaultOperatorAgentId,
    DateTime? createdAt,
    DateTime? archivedAt,
    ResourceVersion? version,
  }) => Space(
    id: id,
    name: name ?? this.name,
    icon: icon ?? this.icon,
    color: color ?? this.color,
    sortKey: sortKey ?? this.sortKey,
    defaultOperatorAgentId:
        defaultOperatorAgentId ?? this.defaultOperatorAgentId,
    createdAt: createdAt ?? this.createdAt,
    archivedAt: archivedAt ?? this.archivedAt,
    version: version ?? this.version,
  );

  @override
  bool operator ==(Object other) =>
      other is Space &&
      other.id == id &&
      other.name == name &&
      other.icon == icon &&
      other.color == color &&
      other.sortKey == sortKey &&
      other.defaultOperatorAgentId == defaultOperatorAgentId &&
      other.createdAt == createdAt &&
      other.archivedAt == archivedAt &&
      other.version == version;

  @override
  int get hashCode => Object.hash(
    id,
    name,
    icon,
    color,
    sortKey,
    defaultOperatorAgentId,
    createdAt,
    archivedAt,
    version,
  );
}

/// Personal sidebar state of one space, synchronized across devices (03 `user_space_states`).
final class UserSpaceState {
  /// Creates an [UserSpaceState].
  const UserSpaceState({
    required this.spaceId,
    required this.collapsed,
    required this.version,
    this.sortKey,
    this.updatedByDeviceId,
  });

  /// Space.
  final SpaceId spaceId;

  /// Whether the space is collapsed.
  final bool collapsed;

  /// Own order; absent means the global order.
  final String? sortKey;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Device of the last write; the own echo is ignored (04 14).
  final DeviceId? updatedByDeviceId;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  UserSpaceState copyWith({
    bool? collapsed,
    String? sortKey,
    ResourceVersion? version,
    DeviceId? updatedByDeviceId,
  }) => UserSpaceState(
    spaceId: spaceId,
    collapsed: collapsed ?? this.collapsed,
    sortKey: sortKey ?? this.sortKey,
    version: version ?? this.version,
    updatedByDeviceId: updatedByDeviceId ?? this.updatedByDeviceId,
  );

  @override
  bool operator ==(Object other) =>
      other is UserSpaceState &&
      other.spaceId == spaceId &&
      other.collapsed == collapsed &&
      other.sortKey == sortKey &&
      other.version == version &&
      other.updatedByDeviceId == updatedByDeviceId;

  @override
  int get hashCode =>
      Object.hash(spaceId, collapsed, sortKey, version, updatedByDeviceId);
}
