// SPDX-License-Identifier: Apache-2.0

/// Channels and forum tags as the client sees them (03 `channels`, `forum_tags`; 15 6.1): without prompt, skills and project binding, which need `manage`.
library;

import '../coded.dart';
import '../ids.dart';
import '../seq.dart';
import 'personal.dart';

/// Kind of a channel (03 `channels.kind`, API `ChannelKind`).
enum ChannelKind implements Coded {
  /// Text channel with a main thread.
  text('text'),

  /// Forum of threads.
  forum('forum'),

  /// Direct message.
  dm('dm'),

  /// A value of a newer server.
  unknown('');

  const ChannelKind(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static ChannelKind fromCode(String code) => decodeCode(values, code, unknown);
}

/// Default sort of a forum (03 `channels.forum_default_sort`, API `ForumSort`).
enum ForumSort implements Coded {
  /// By last activity.
  lastActivity('last_activity'),

  /// By creation.
  created('created'),

  /// A value of a newer server.
  unknown('');

  const ForumSort(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static ForumSort fromCode(String code) => decodeCode(values, code, unknown);
}

/// How a channel finds its operator (API `OperatorMode`).
enum OperatorMode implements Coded {
  /// From the space default.
  inherit('inherit'),

  /// No operator.
  none('none'),

  /// A set agent.
  agent('agent'),

  /// A value of a newer server.
  unknown('');

  const OperatorMode(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static OperatorMode fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// The operator setting of a channel and the effective operator (API `ChannelOperator`).
final class ChannelOperator {
  /// Creates a [ChannelOperator].
  const ChannelOperator({
    required this.mode,
    this.agentId,
    this.effectiveAgentId,
  });

  /// Mode.
  final OperatorMode mode;

  /// Set agent for mode `agent`.
  final PrincipalId? agentId;

  /// Effective operator, if any.
  final PrincipalId? effectiveAgentId;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  ChannelOperator copyWith({
    OperatorMode? mode,
    PrincipalId? agentId,
    PrincipalId? effectiveAgentId,
  }) => ChannelOperator(
    mode: mode ?? this.mode,
    agentId: agentId ?? this.agentId,
    effectiveAgentId: effectiveAgentId ?? this.effectiveAgentId,
  );

  @override
  bool operator ==(Object other) =>
      other is ChannelOperator &&
      other.mode == mode &&
      other.agentId == agentId &&
      other.effectiveAgentId == effectiveAgentId;

  @override
  int get hashCode => Object.hash(mode, agentId, effectiveAgentId);
}

/// A channel (03 `channels`, API `Channel`).
final class Channel {
  /// Creates a [Channel].
  const Channel({
    required this.id,
    required this.kind,
    required this.name,
    required this.topic,
    required this.sortKey,
    required this.operator,
    required this.readOnly,
    required this.dmParticipantIds,
    required this.createdAt,
    required this.version,
    required this.settingsVersion,
    this.spaceId,
    this.forumDefaultSort,
    this.defaultNotificationLevel,
    this.mainThreadId,
    this.archivedAt,
  });

  /// ID.
  final ChannelId id;

  /// Space; absent exactly for direct messages.
  final SpaceId? spaceId;

  /// Kind.
  final ChannelKind kind;

  /// Name; empty for direct messages.
  final String name;

  /// Topic.
  final String topic;

  /// Order within the space.
  final String sortKey;

  /// Operator setting.
  final ChannelOperator operator;

  /// Default sort of a forum.
  final ForumSort? forumDefaultSort;

  /// Channel preset for people without an own setting.
  final NotificationLevel? defaultNotificationLevel;

  /// Read-only for every principal.
  final bool readOnly;

  /// Main thread of a text or DM channel.
  final ThreadId? mainThreadId;

  /// Participants of a direct message; empty otherwise.
  final List<PrincipalId> dmParticipantIds;

  /// Creation time.
  final DateTime createdAt;

  /// Archive time, if archived.
  final DateTime? archivedAt;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Version of behavior, prompt, skills and project; events name only this number (04 2.2 rule 1).
  final ResourceVersion settingsVersion;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  Channel copyWith({
    SpaceId? spaceId,
    ChannelKind? kind,
    String? name,
    String? topic,
    String? sortKey,
    ChannelOperator? operator,
    ForumSort? forumDefaultSort,
    NotificationLevel? defaultNotificationLevel,
    bool? readOnly,
    ThreadId? mainThreadId,
    List<PrincipalId>? dmParticipantIds,
    DateTime? createdAt,
    DateTime? archivedAt,
    ResourceVersion? version,
    ResourceVersion? settingsVersion,
  }) => Channel(
    id: id,
    spaceId: spaceId ?? this.spaceId,
    kind: kind ?? this.kind,
    name: name ?? this.name,
    topic: topic ?? this.topic,
    sortKey: sortKey ?? this.sortKey,
    operator: operator ?? this.operator,
    forumDefaultSort: forumDefaultSort ?? this.forumDefaultSort,
    defaultNotificationLevel:
        defaultNotificationLevel ?? this.defaultNotificationLevel,
    readOnly: readOnly ?? this.readOnly,
    mainThreadId: mainThreadId ?? this.mainThreadId,
    dmParticipantIds: dmParticipantIds ?? this.dmParticipantIds,
    createdAt: createdAt ?? this.createdAt,
    archivedAt: archivedAt ?? this.archivedAt,
    version: version ?? this.version,
    settingsVersion: settingsVersion ?? this.settingsVersion,
  );

  @override
  bool operator ==(Object other) =>
      other is Channel &&
      other.id == id &&
      other.spaceId == spaceId &&
      other.kind == kind &&
      other.name == name &&
      other.topic == topic &&
      other.sortKey == sortKey &&
      other.operator == operator &&
      other.forumDefaultSort == forumDefaultSort &&
      other.defaultNotificationLevel == defaultNotificationLevel &&
      other.readOnly == readOnly &&
      other.mainThreadId == mainThreadId &&
      listEquals(other.dmParticipantIds, dmParticipantIds) &&
      other.createdAt == createdAt &&
      other.archivedAt == archivedAt &&
      other.version == version &&
      other.settingsVersion == settingsVersion;

  @override
  int get hashCode => Object.hash(
    id,
    spaceId,
    kind,
    name,
    topic,
    sortKey,
    operator,
    forumDefaultSort,
    defaultNotificationLevel,
    readOnly,
    mainThreadId,
    Object.hashAll(dmParticipantIds),
    createdAt,
    archivedAt,
    version,
    settingsVersion,
  );
}

/// A tag of a forum (03 `forum_tags`).
final class ForumTag {
  /// Creates a [ForumTag].
  const ForumTag({
    required this.id,
    required this.channelId,
    required this.name,
    required this.sortKey,
    required this.restricted,
    this.color,
  });

  /// ID.
  final ForumTagId id;

  /// Forum.
  final ChannelId channelId;

  /// Name.
  final String name;

  /// Key of a category color, if any.
  final String? color;

  /// Order.
  final String sortKey;

  /// Whether only `manage` may set it.
  final bool restricted;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  ForumTag copyWith({
    ChannelId? channelId,
    String? name,
    String? color,
    String? sortKey,
    bool? restricted,
  }) => ForumTag(
    id: id,
    channelId: channelId ?? this.channelId,
    name: name ?? this.name,
    color: color ?? this.color,
    sortKey: sortKey ?? this.sortKey,
    restricted: restricted ?? this.restricted,
  );

  @override
  bool operator ==(Object other) =>
      other is ForumTag &&
      other.id == id &&
      other.channelId == channelId &&
      other.name == name &&
      other.color == color &&
      other.sortKey == sortKey &&
      other.restricted == restricted;

  @override
  int get hashCode =>
      Object.hash(id, channelId, name, color, sortKey, restricted);
}
