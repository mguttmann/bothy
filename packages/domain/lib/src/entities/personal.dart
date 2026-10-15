// SPDX-License-Identifier: Apache-2.0

/// Personal sync data (03 `read_states`, `drafts`, `user_settings`, `notification_preferences`, `quiet_hours`; 04 14).
library;

import '../coded.dart';
import '../ids.dart';
import '../seq.dart';

/// Notification level (03 `notification_preferences.level`, API `NotificationLevel`).
enum NotificationLevel implements Coded {
  /// All messages.
  all('all'),

  /// Mentions only.
  mentions('mentions'),

  /// Nothing.
  mute('mute'),

  /// A value of a newer server.
  unknown('');

  const NotificationLevel(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static NotificationLevel fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// Target of a notification preference (03 `notification_preferences.target_kind`).
enum NotificationTargetKind implements Coded {
  /// A space.
  space('space'),

  /// A channel.
  channel('channel'),

  /// A thread.
  thread('thread'),

  /// A value of a newer server.
  unknown('');

  const NotificationTargetKind(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static NotificationTargetKind fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// The read state of one thread (03 `read_states`, API `ReadState`) with the counters the server computed (04 14).
final class ReadState {
  /// Creates a [ReadState].
  const ReadState({
    required this.threadId,
    required this.channelId,
    required this.lastReadSeq,
    required this.mentionCount,
    required this.version,
    this.unreadCount,
    this.updatedByDeviceId,
    this.updatedAt,
  });

  /// Thread.
  final ThreadId threadId;

  /// Channel.
  final ChannelId channelId;

  /// Highest read `thread_seq`; only increases, except through a reset (04 14).
  final ThreadSeq lastReadSeq;

  /// Unread mentions the server computed.
  final int mentionCount;

  /// Unread messages the server computed, where delivered (snapshot `UnreadThread.unread_count`).
  final int? unreadCount;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Device of the last write; the own echo is ignored (04 14).
  final DeviceId? updatedByDeviceId;

  /// Time of the last write.
  final DateTime? updatedAt;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  ReadState copyWith({
    ChannelId? channelId,
    ThreadSeq? lastReadSeq,
    int? mentionCount,
    int? unreadCount,
    ResourceVersion? version,
    DeviceId? updatedByDeviceId,
    DateTime? updatedAt,
  }) => ReadState(
    threadId: threadId,
    channelId: channelId ?? this.channelId,
    lastReadSeq: lastReadSeq ?? this.lastReadSeq,
    mentionCount: mentionCount ?? this.mentionCount,
    unreadCount: unreadCount ?? this.unreadCount,
    version: version ?? this.version,
    updatedByDeviceId: updatedByDeviceId ?? this.updatedByDeviceId,
    updatedAt: updatedAt ?? this.updatedAt,
  );

  @override
  bool operator ==(Object other) =>
      other is ReadState &&
      other.threadId == threadId &&
      other.channelId == channelId &&
      other.lastReadSeq == lastReadSeq &&
      other.mentionCount == mentionCount &&
      other.unreadCount == unreadCount &&
      other.version == version &&
      other.updatedByDeviceId == updatedByDeviceId &&
      other.updatedAt == updatedAt;

  @override
  int get hashCode => Object.hash(
    threadId,
    channelId,
    lastReadSeq,
    mentionCount,
    unreadCount,
    version,
    updatedByDeviceId,
    updatedAt,
  );
}

/// A draft, synchronized across devices (03 `drafts`, API `Draft`).
final class Draft {
  /// Creates a [Draft].
  const Draft({
    required this.id,
    required this.channelId,
    required this.contentMd,
    required this.fileIds,
    required this.version,
    this.threadId,
    this.replyToMessageId,
    this.updatedByDeviceId,
    this.updatedAt,
  });

  /// ID.
  final DraftId id;

  /// Thread; absent for a new forum post.
  final ThreadId? threadId;

  /// Channel.
  final ChannelId channelId;

  /// Markdown.
  final String contentMd;

  /// Quoted message.
  final MessageId? replyToMessageId;

  /// Uploaded files not yet sent.
  final List<FileId> fileIds;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Device of the last write.
  final DeviceId? updatedByDeviceId;

  /// Time of the last write.
  final DateTime? updatedAt;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  Draft copyWith({
    ThreadId? threadId,
    ChannelId? channelId,
    String? contentMd,
    MessageId? replyToMessageId,
    List<FileId>? fileIds,
    ResourceVersion? version,
    DeviceId? updatedByDeviceId,
    DateTime? updatedAt,
  }) => Draft(
    id: id,
    threadId: threadId ?? this.threadId,
    channelId: channelId ?? this.channelId,
    contentMd: contentMd ?? this.contentMd,
    replyToMessageId: replyToMessageId ?? this.replyToMessageId,
    fileIds: fileIds ?? this.fileIds,
    version: version ?? this.version,
    updatedByDeviceId: updatedByDeviceId ?? this.updatedByDeviceId,
    updatedAt: updatedAt ?? this.updatedAt,
  );

  @override
  bool operator ==(Object other) =>
      other is Draft &&
      other.id == id &&
      other.threadId == threadId &&
      other.channelId == channelId &&
      other.contentMd == contentMd &&
      other.replyToMessageId == replyToMessageId &&
      listEquals(other.fileIds, fileIds) &&
      other.version == version &&
      other.updatedByDeviceId == updatedByDeviceId &&
      other.updatedAt == updatedAt;

  @override
  int get hashCode => Object.hash(
    id,
    threadId,
    channelId,
    contentMd,
    replyToMessageId,
    Object.hashAll(fileIds),
    version,
    updatedByDeviceId,
    updatedAt,
  );
}

/// A personal setting (03 `user_settings`, API `UserSetting`); keys and defaults in 01 11 and 13 3.3.
final class UserSetting {
  /// Creates an [UserSetting].
  const UserSetting({
    required this.key,
    required this.version,
    this.value,
    this.updatedByDeviceId,
    this.updatedAt,
  });

  /// Key, for example `appearance.theme`.
  final String key;

  /// JSON value.
  final Object? value;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Device of the last write.
  final DeviceId? updatedByDeviceId;

  /// Time of the last write.
  final DateTime? updatedAt;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  UserSetting copyWith({
    Object? value,
    ResourceVersion? version,
    DeviceId? updatedByDeviceId,
    DateTime? updatedAt,
  }) => UserSetting(
    key: key,
    value: value ?? this.value,
    version: version ?? this.version,
    updatedByDeviceId: updatedByDeviceId ?? this.updatedByDeviceId,
    updatedAt: updatedAt ?? this.updatedAt,
  );

  @override
  bool operator ==(Object other) =>
      other is UserSetting &&
      other.key == key &&
      jsonEquals(other.value, value) &&
      other.version == version &&
      other.updatedByDeviceId == updatedByDeviceId &&
      other.updatedAt == updatedAt;

  @override
  int get hashCode =>
      Object.hash(key, jsonHash(value), version, updatedByDeviceId, updatedAt);
}

/// A notification preference of one target (03 `notification_preferences`).
final class NotificationPreference {
  /// Creates a [NotificationPreference].
  const NotificationPreference({
    required this.targetKind,
    required this.targetId,
    required this.level,
    required this.version,
    this.mutedUntil,
  });

  /// Kind of target.
  final NotificationTargetKind targetKind;

  /// ID of the space, channel or thread.
  final String targetId;

  /// Level.
  final NotificationLevel level;

  /// Temporarily muted until.
  final DateTime? mutedUntil;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  NotificationPreference copyWith({
    NotificationLevel? level,
    DateTime? mutedUntil,
    ResourceVersion? version,
  }) => NotificationPreference(
    targetKind: targetKind,
    targetId: targetId,
    level: level ?? this.level,
    mutedUntil: mutedUntil ?? this.mutedUntil,
    version: version ?? this.version,
  );

  @override
  bool operator ==(Object other) =>
      other is NotificationPreference &&
      other.targetKind == targetKind &&
      other.targetId == targetId &&
      other.level == level &&
      other.mutedUntil == mutedUntil &&
      other.version == version;

  @override
  int get hashCode =>
      Object.hash(targetKind, targetId, level, mutedUntil, version);
}

/// A quiet hours rule (03 `quiet_hours`, API `QuietHoursRule`).
final class QuietHours {
  /// Creates a [QuietHours].
  const QuietHours({
    required this.id,
    required this.weekdays,
    required this.startLocal,
    required this.endLocal,
    required this.enabled,
    required this.version,
    this.timeZone,
  });

  /// ID.
  final QuietHoursId id;

  /// Bit mask, bit 0 is Monday.
  final int weekdays;

  /// Local start time `HH:MM`.
  final String startLocal;

  /// Local end time; may be before the start (across midnight).
  final String endLocal;

  /// IANA time zone; absent means the person's zone.
  final String? timeZone;

  /// Whether the rule is on.
  final bool enabled;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  QuietHours copyWith({
    int? weekdays,
    String? startLocal,
    String? endLocal,
    String? timeZone,
    bool? enabled,
    ResourceVersion? version,
  }) => QuietHours(
    id: id,
    weekdays: weekdays ?? this.weekdays,
    startLocal: startLocal ?? this.startLocal,
    endLocal: endLocal ?? this.endLocal,
    timeZone: timeZone ?? this.timeZone,
    enabled: enabled ?? this.enabled,
    version: version ?? this.version,
  );

  @override
  bool operator ==(Object other) =>
      other is QuietHours &&
      other.id == id &&
      other.weekdays == weekdays &&
      other.startLocal == startLocal &&
      other.endLocal == endLocal &&
      other.timeZone == timeZone &&
      other.enabled == enabled &&
      other.version == version;

  @override
  int get hashCode => Object.hash(
    id,
    weekdays,
    startLocal,
    endLocal,
    timeZone,
    enabled,
    version,
  );
}
