// SPDX-License-Identifier: Apache-2.0

/// Rows of the structure tables to domain entities and back (15 6.1).
library;

import 'dart:convert';

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../models.dart';
import 'codec.dart';

/// The insert companion of [space]; `level` and `capabilities` stay as stored.
SpacesCompanion spaceToCompanion(Space space) => SpacesCompanion.insert(
  id: space.id.value,
  name: space.name,
  icon: Value(space.icon),
  color: Value(space.color),
  sortKey: space.sortKey,
  defaultOperatorAgentId: Value(space.defaultOperatorAgentId?.value),
  createdAt: space.createdAt,
  archivedAt: Value(space.archivedAt),
  version: space.version.value,
);

/// The space of [row].
Space spaceFromRow(SpaceRow row) => Space(
  id: SpaceId.parse(row.id),
  name: row.name,
  icon: row.icon,
  color: row.color,
  sortKey: row.sortKey,
  defaultOperatorAgentId: optional(
    row.defaultOperatorAgentId,
    PrincipalId.parse,
  ),
  createdAt: row.createdAt,
  archivedAt: row.archivedAt,
  version: ResourceVersion(row.version),
);

/// The insert companion of [state].
UserSpaceStatesCompanion userSpaceStateToCompanion(UserSpaceState state) =>
    UserSpaceStatesCompanion.insert(
      spaceId: state.spaceId.value,
      collapsed: state.collapsed,
      sortKey: Value(state.sortKey),
      version: state.version.value,
      updatedByDeviceId: Value(state.updatedByDeviceId?.value),
    );

/// The sidebar state of [row].
UserSpaceState userSpaceStateFromRow(UserSpaceStateRow row) => UserSpaceState(
  spaceId: SpaceId.parse(row.spaceId),
  collapsed: row.collapsed,
  sortKey: row.sortKey,
  version: ResourceVersion(row.version),
  updatedByDeviceId: optional(row.updatedByDeviceId, DeviceId.parse),
);

/// The insert companion of [channel]; `level` and `capabilities` stay as stored.
ChannelsCompanion channelToCompanion(Channel channel) =>
    ChannelsCompanion.insert(
      id: channel.id.value,
      spaceId: Value(channel.spaceId?.value),
      kind: channel.kind.code,
      name: channel.name,
      topic: channel.topic,
      sortKey: channel.sortKey,
      effectiveOperatorAgentId: Value(channel.operator.effectiveAgentId?.value),
      forumDefaultSort: Value(channel.forumDefaultSort?.code),
      defaultNotificationLevel: Value(channel.defaultNotificationLevel?.code),
      readOnly: channel.readOnly,
      mainThreadId: Value(channel.mainThreadId?.value),
      dmParticipantIds: encodeTexts(channel.dmParticipantIds),
      createdAt: channel.createdAt,
      archivedAt: Value(channel.archivedAt),
      version: channel.version.value,
      settingsVersion: channel.settingsVersion.value,
      projectBound: Value(channel.projectBound),
    );

/// The operator setting of [channel] for `channel_operators`.
ChannelOperatorsCompanion channelOperatorToCompanion(Channel channel) =>
    ChannelOperatorsCompanion.insert(
      channelId: channel.id.value,
      operatorMode: channel.operator.mode.code,
      operatorAgentId: Value(channel.operator.agentId?.value),
    );

/// The channel of [row] with its operator setting [operator] (absent: mode `unknown`).
Channel channelFromRow(ChannelRow row, ChannelOperatorRow? operator) => Channel(
  id: ChannelId.parse(row.id),
  spaceId: optional(row.spaceId, SpaceId.parse),
  kind: ChannelKind.fromCode(row.kind),
  name: row.name,
  topic: row.topic,
  sortKey: row.sortKey,
  operator: ChannelOperator(
    mode: OperatorMode.fromCode(operator?.operatorMode ?? ''),
    agentId: optional(operator?.operatorAgentId, PrincipalId.parse),
    effectiveAgentId: optional(row.effectiveOperatorAgentId, PrincipalId.parse),
  ),
  forumDefaultSort: optional(row.forumDefaultSort, ForumSort.fromCode),
  defaultNotificationLevel: optional(
    row.defaultNotificationLevel,
    NotificationLevel.fromCode,
  ),
  readOnly: row.readOnly,
  mainThreadId: optional(row.mainThreadId, ThreadId.parse),
  dmParticipantIds: decodeTexts(row.dmParticipantIds, PrincipalId.parse),
  createdAt: row.createdAt,
  archivedAt: row.archivedAt,
  version: ResourceVersion(row.version),
  settingsVersion: ResourceVersion(row.settingsVersion),
  projectBound: row.projectBound,
);

/// The insert companion of [tag].
ForumTagsCompanion forumTagToCompanion(ForumTag tag) =>
    ForumTagsCompanion.insert(
      id: tag.id.value,
      channelId: tag.channelId.value,
      name: tag.name,
      color: Value(tag.color),
      sortKey: tag.sortKey,
      restricted: tag.restricted,
    );

/// The forum tag of [row].
ForumTag forumTagFromRow(ForumTagRow row) => ForumTag(
  id: ForumTagId.parse(row.id),
  channelId: ChannelId.parse(row.channelId),
  name: row.name,
  color: row.color,
  sortKey: row.sortKey,
  restricted: row.restricted,
);

/// The stored access columns of [view]: level code and capabilities as a JSON array.
({String level, String capabilities}) accessToColumns(AccessView view) => (
  level: view.level.code,
  capabilities: jsonEncode([for (final c in view.capabilities) c.code]),
);

/// The access view of the stored columns, or null when no level was stored yet.
AccessView? accessFromColumns(String? level, String capabilities) {
  if (level == null) return null;
  return AccessView(
    level: AccessLevel.fromCode(level),
    capabilities: decodeTexts(capabilities, Capability.fromCode),
  );
}
