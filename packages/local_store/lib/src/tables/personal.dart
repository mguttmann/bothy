// SPDX-License-Identifier: Apache-2.0

/// Personal sync data (15 6.1: `read_states`, `drafts`, `user_settings`,
/// `notification_preferences`, `quiet_hours`; 04 14). One person per database, so the
/// `user_id` columns of 03 are implied.
library;

import 'package:drift/drift.dart';

/// 03 `read_states` with the counters the server computed (API `ReadState`).
@DataClassName('ReadStateRow')
@TableIndex(name: 'read_states_channel', columns: {#channelId})
class ReadStates extends Table {
  @override
  String get tableName => 'read_states';

  /// 03 `read_states.thread_id`.
  TextColumn get threadId => text()();

  /// 03 `read_states.channel_id`.
  TextColumn get channelId => text()();

  /// 03 `read_states.last_read_seq`.
  IntColumn get lastReadSeq => integer()();

  /// API `ReadState.mention_count`.
  IntColumn get mentionCount => integer()();

  /// API `UnreadThread.unread_count`, where delivered.
  IntColumn get unreadCount => integer().nullable()();

  /// 03 `read_states.version`.
  IntColumn get version => integer()();

  /// 03 `read_states.updated_by_device_id`.
  TextColumn get updatedByDeviceId => text().nullable()();

  /// 03 `read_states.updated_at`.
  DateTimeColumn get updatedAt => dateTime().nullable()();

  @override
  Set<Column<Object>> get primaryKey => {threadId};
}

/// 03 `drafts`; never touched by the cleanup and kept by every migration (15 6.2, 15 6.3).
@DataClassName('DraftRow')
@TableIndex(name: 'drafts_channel', columns: {#channelId})
class Drafts extends Table {
  @override
  String get tableName => 'drafts';

  /// 03 `drafts.id`.
  TextColumn get id => text()();

  /// 03 `drafts.thread_id`; null for a new forum post.
  TextColumn get threadId => text().nullable()();

  /// 03 `drafts.channel_id`.
  TextColumn get channelId => text()();

  /// 03 `drafts.content_md`.
  TextColumn get contentMd => text()();

  /// 03 `drafts.reply_to_message_id`.
  TextColumn get replyToMessageId => text().nullable()();

  /// 03 `drafts.file_ids` as a JSON array.
  TextColumn get fileIds => text()();

  /// 03 `drafts.version`.
  IntColumn get version => integer()();

  /// 03 `drafts.updated_by_device_id`.
  TextColumn get updatedByDeviceId => text().nullable()();

  /// 03 `drafts.updated_at`.
  DateTimeColumn get updatedAt => dateTime().nullable()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

/// 03 `user_settings`.
@DataClassName('UserSettingRow')
class UserSettings extends Table {
  @override
  String get tableName => 'user_settings';

  /// 03 `user_settings.key`.
  TextColumn get key => text()();

  /// 03 `user_settings.value` as JSON text.
  TextColumn get value => text()();

  /// 03 `user_settings.version`.
  IntColumn get version => integer()();

  /// 03 `user_settings.updated_by_device_id`.
  TextColumn get updatedByDeviceId => text().nullable()();

  /// 03 `user_settings.updated_at`.
  DateTimeColumn get updatedAt => dateTime().nullable()();

  @override
  Set<Column<Object>> get primaryKey => {key};
}

/// 03 `notification_preferences`.
@DataClassName('NotificationPreferenceRow')
class NotificationPreferences extends Table {
  @override
  String get tableName => 'notification_preferences';

  /// 03 `notification_preferences.target_kind`.
  TextColumn get targetKind => text()();

  /// 03 `notification_preferences.target_id`.
  TextColumn get targetId => text()();

  /// 03 `notification_preferences.level`.
  TextColumn get level => text()();

  /// 03 `notification_preferences.muted_until`.
  DateTimeColumn get mutedUntil => dateTime().nullable()();

  /// 03 `notification_preferences.version`.
  IntColumn get version => integer()();

  @override
  Set<Column<Object>> get primaryKey => {targetKind, targetId};
}

/// 03 `quiet_hours`.
@DataClassName('QuietHoursRow')
class QuietHoursRules extends Table {
  @override
  String get tableName => 'quiet_hours';

  /// 03 `quiet_hours.id`.
  TextColumn get id => text()();

  /// 03 `quiet_hours.weekdays`.
  IntColumn get weekdays => integer()();

  /// 03 `quiet_hours.start_local`.
  TextColumn get startLocal => text()();

  /// 03 `quiet_hours.end_local`.
  TextColumn get endLocal => text()();

  /// 03 `quiet_hours.time_zone`.
  TextColumn get timeZone => text().nullable()();

  /// 03 `quiet_hours.enabled`.
  BoolColumn get enabled => boolean()();

  /// 03 `quiet_hours.version`.
  IntColumn get version => integer()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}
