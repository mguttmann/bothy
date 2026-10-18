// SPDX-License-Identifier: Apache-2.0

/// Rows of the personal sync data to domain entities and back (04 14).
library;

import 'dart:convert';

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import 'codec.dart';

/// The insert companion of [state].
ReadStatesCompanion readStateToCompanion(ReadState state) =>
    ReadStatesCompanion.insert(
      threadId: state.threadId.value,
      channelId: state.channelId.value,
      lastReadSeq: state.lastReadSeq.value,
      mentionCount: state.mentionCount,
      unreadCount: Value(state.unreadCount),
      version: state.version.value,
      updatedByDeviceId: Value(state.updatedByDeviceId?.value),
      updatedAt: Value(state.updatedAt),
    );

/// The read state of [row].
ReadState readStateFromRow(ReadStateRow row) => ReadState(
  threadId: ThreadId.parse(row.threadId),
  channelId: ChannelId.parse(row.channelId),
  lastReadSeq: ThreadSeq(row.lastReadSeq),
  mentionCount: row.mentionCount,
  unreadCount: row.unreadCount,
  version: ResourceVersion(row.version),
  updatedByDeviceId: optional(row.updatedByDeviceId, DeviceId.parse),
  updatedAt: row.updatedAt,
);

/// The insert companion of [draft].
DraftsCompanion draftToCompanion(Draft draft) => DraftsCompanion.insert(
  id: draft.id.value,
  threadId: Value(draft.threadId?.value),
  channelId: draft.channelId.value,
  contentMd: draft.contentMd,
  replyToMessageId: Value(draft.replyToMessageId?.value),
  fileIds: encodeTexts(draft.fileIds),
  version: draft.version.value,
  updatedByDeviceId: Value(draft.updatedByDeviceId?.value),
  updatedAt: Value(draft.updatedAt),
);

/// The draft of [row].
Draft draftFromRow(DraftRow row) => Draft(
  id: DraftId.parse(row.id),
  threadId: optional(row.threadId, ThreadId.parse),
  channelId: ChannelId.parse(row.channelId),
  contentMd: row.contentMd,
  replyToMessageId: optional(row.replyToMessageId, MessageId.parse),
  fileIds: decodeTexts(row.fileIds, FileId.parse),
  version: ResourceVersion(row.version),
  updatedByDeviceId: optional(row.updatedByDeviceId, DeviceId.parse),
  updatedAt: row.updatedAt,
);

/// The insert companion of [setting]; the value is stored as JSON text.
UserSettingsCompanion settingToCompanion(UserSetting setting) =>
    UserSettingsCompanion.insert(
      key: setting.key,
      value: jsonEncode(setting.value),
      version: setting.version.value,
      updatedByDeviceId: Value(setting.updatedByDeviceId?.value),
      updatedAt: Value(setting.updatedAt),
    );

/// The setting of [row].
UserSetting settingFromRow(UserSettingRow row) => UserSetting(
  key: row.key,
  value: jsonDecode(row.value),
  version: ResourceVersion(row.version),
  updatedByDeviceId: optional(row.updatedByDeviceId, DeviceId.parse),
  updatedAt: row.updatedAt,
);

/// The insert companion of [preference].
NotificationPreferencesCompanion preferenceToCompanion(
  NotificationPreference preference,
) => NotificationPreferencesCompanion.insert(
  targetKind: preference.targetKind.code,
  targetId: preference.targetId,
  level: preference.level.code,
  mutedUntil: Value(preference.mutedUntil),
  version: preference.version.value,
);

/// The notification preference of [row].
NotificationPreference preferenceFromRow(NotificationPreferenceRow row) =>
    NotificationPreference(
      targetKind: NotificationTargetKind.fromCode(row.targetKind),
      targetId: row.targetId,
      level: NotificationLevel.fromCode(row.level),
      mutedUntil: row.mutedUntil,
      version: ResourceVersion(row.version),
    );

/// The insert companion of [rule].
QuietHoursRulesCompanion quietHoursToCompanion(QuietHours rule) =>
    QuietHoursRulesCompanion.insert(
      id: rule.id.value,
      weekdays: rule.weekdays,
      startLocal: rule.startLocal,
      endLocal: rule.endLocal,
      timeZone: Value(rule.timeZone),
      enabled: rule.enabled,
      version: rule.version.value,
    );

/// The quiet hours rule of [row].
QuietHours quietHoursFromRow(QuietHoursRow row) => QuietHours(
  id: QuietHoursId.parse(row.id),
  weekdays: row.weekdays,
  startLocal: row.startLocal,
  endLocal: row.endLocal,
  timeZone: row.timeZone,
  enabled: row.enabled,
  version: ResourceVersion(row.version),
);
