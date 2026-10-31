// SPDX-License-Identifier: Apache-2.0

/// Mirror tables of the sidebar structure (15 6.1: `spaces`, `user_space_states`,
/// `user_channel_states`, `channels`, `forum_tags`, `channel_operators`). Columns carry the 03 names; nothing readable only with
/// `manage` (channel prompt, skills, project binding) has a column (15 6.1); of the binding only
/// the derived flag `project_bound` of the API `Channel` is kept, which every reader gets
/// (R-DM-54).
library;

import 'package:drift/drift.dart';

/// 03 `spaces` plus the caller's `level` and `capabilities` of the API `SpaceView`.
@DataClassName('SpaceRow')
class Spaces extends Table {
  @override
  String get tableName => 'spaces';

  /// 03 `spaces.id`.
  TextColumn get id => text()();

  /// 03 `spaces.name`.
  TextColumn get name => text()();

  /// 03 `spaces.icon`.
  TextColumn get icon => text().nullable()();

  /// 03 `spaces.color`.
  TextColumn get color => text().nullable()();

  /// 03 `spaces.sort_key`.
  TextColumn get sortKey => text()();

  /// 03 `spaces.default_operator_agent_id`.
  TextColumn get defaultOperatorAgentId => text().nullable()();

  /// 03 `spaces.created_at`.
  DateTimeColumn get createdAt => dateTime()();

  /// 03 `spaces.archived_at`.
  DateTimeColumn get archivedAt => dateTime().nullable()();

  /// 03 `spaces.version`.
  IntColumn get version => integer()();

  /// API `SpaceView.level`; null until a snapshot or `getSpace` delivered it (15 1 principle 1).
  TextColumn get level => text().nullable()();

  /// API `SpaceView.capabilities` as a JSON array of codes.
  TextColumn get capabilities => text().withDefault(const Constant('[]'))();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

/// 03 `user_space_states` of the person (the user column is implied: one person per database).
@DataClassName('UserSpaceStateRow')
class UserSpaceStates extends Table {
  @override
  String get tableName => 'user_space_states';

  /// 03 `user_space_states.space_id`.
  TextColumn get spaceId => text()();

  /// 03 `user_space_states.collapsed`.
  BoolColumn get collapsed => boolean()();

  /// 03 `user_space_states.sort_key`.
  TextColumn get sortKey => text().nullable()();

  /// 03 `user_space_states.version`.
  IntColumn get version => integer()();

  /// 03 `user_space_states.updated_by_device_id`.
  TextColumn get updatedByDeviceId => text().nullable()();

  /// 03 `user_space_states.channel_order` as a JSON array of channel IDs; null is the default
  /// order (version 12, M4-51).
  TextColumn get channelOrder => text().nullable()();

  @override
  Set<Column<Object>> get primaryKey => {spaceId};
}

/// 03 `user_channel_states` of the person (the user column is implied: one person per
/// database; version 12, M4-51).
@DataClassName('UserChannelStateRow')
class UserChannelStates extends Table {
  @override
  String get tableName => 'user_channel_states';

  /// 03 `user_channel_states.channel_id`.
  TextColumn get channelId => text()();

  /// 03 `user_channel_states.thread_order` as a JSON array of thread IDs.
  TextColumn get threadOrder => text().withDefault(const Constant('[]'))();

  /// 03 `user_channel_states.version`.
  IntColumn get version => integer()();

  /// 03 `user_channel_states.updated_by_device_id`.
  TextColumn get updatedByDeviceId => text().nullable()();

  @override
  Set<Column<Object>> get primaryKey => {channelId};
}

/// 03 `channels` without `prompt` and the behaviour fields (readable only with `manage`), with
/// the effective operator of the API field `operator`, the derived `project_bound` of the API
/// `Channel` and the caller's `level` and `capabilities` of the API `ChannelView`.
@DataClassName('ChannelRow')
@TableIndex(name: 'channels_space', columns: {#spaceId})
class Channels extends Table {
  @override
  String get tableName => 'channels';

  /// 03 `channels.id`.
  TextColumn get id => text()();

  /// 03 `channels.space_id`; null exactly for direct messages.
  TextColumn get spaceId => text().nullable()();

  /// 03 `channels.kind`.
  TextColumn get kind => text()();

  /// 03 `channels.name`.
  TextColumn get name => text()();

  /// 03 `channels.topic`.
  TextColumn get topic => text()();

  /// 03 `channels.sort_key`.
  TextColumn get sortKey => text()();

  /// API `ChannelOperator.effective_agent_id` (15 6.1 "effective operator as a column").
  TextColumn get effectiveOperatorAgentId => text().nullable()();

  /// 03 `channels.forum_default_sort`.
  TextColumn get forumDefaultSort => text().nullable()();

  /// 03 `channels.default_notification_level`.
  TextColumn get defaultNotificationLevel => text().nullable()();

  /// 03 `channels.read_only`.
  BoolColumn get readOnly => boolean()();

  /// API `Channel.main_thread_id`.
  TextColumn get mainThreadId => text().nullable()();

  /// 03 `dm_participants` of a direct message as a JSON array of principal IDs.
  TextColumn get dmParticipantIds => text()();

  /// 03 `channels.created_at`.
  DateTimeColumn get createdAt => dateTime()();

  /// 03 `channels.archived_at`.
  DateTimeColumn get archivedAt => dateTime().nullable()();

  /// 03 `channels.version`.
  IntColumn get version => integer()();

  /// 03 `channels.settings_version`.
  IntColumn get settingsVersion => integer()();

  /// API `Channel.project_bound` (03 `project_bindings`, R-DM-54): whether a project binding
  /// exists, so the sidebar shows a project forum (01 6.8 item 11). False in a row of a version
  /// before 6 until the snapshot that the migration asks for (M4-40).
  BoolColumn get projectBound => boolean().withDefault(const Constant(false))();

  /// API `ChannelView.level`; null until delivered.
  TextColumn get level => text().nullable()();

  /// API `ChannelView.capabilities` as a JSON array of codes.
  TextColumn get capabilities => text().withDefault(const Constant('[]'))();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

/// The operator setting of a channel (API `ChannelOperator.mode` and `agent_id`; 03
/// `channels.operator_inherit`, `operator_agent_id`). The effective operator stays on
/// `channels`, so the sidebar reads it without a join.
@DataClassName('ChannelOperatorRow')
class ChannelOperators extends Table {
  @override
  String get tableName => 'channel_operators';

  /// The channel.
  TextColumn get channelId => text()();

  /// API `ChannelOperator.mode`.
  TextColumn get operatorMode => text()();

  /// 03 `channels.operator_agent_id`.
  TextColumn get operatorAgentId => text().nullable()();

  @override
  Set<Column<Object>> get primaryKey => {channelId};
}

/// 03 `forum_tags`; a tag change raises `channels.version`, so the tags have no version.
@DataClassName('ForumTagRow')
@TableIndex(name: 'forum_tags_channel', columns: {#channelId})
class ForumTags extends Table {
  @override
  String get tableName => 'forum_tags';

  /// 03 `forum_tags.id`.
  TextColumn get id => text()();

  /// 03 `forum_tags.channel_id`.
  TextColumn get channelId => text()();

  /// 03 `forum_tags.name`.
  TextColumn get name => text()();

  /// 03 `forum_tags.color`.
  TextColumn get color => text().nullable()();

  /// 03 `forum_tags.sort_key`.
  TextColumn get sortKey => text()();

  /// 03 `forum_tags.restricted`.
  BoolColumn get restricted => boolean()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}
