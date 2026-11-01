// SPDX-License-Identifier: Apache-2.0

/// The wire form of the API (JSON as `docs/api/openapi.yaml` defines it, 04 2.2) mapped to the
/// domain types of `package:domain`, which the local store takes. Event payloads, the snapshot
/// and REST responses go through the same functions, so an object looks the same whichever
/// way it arrived (04 9 item 3).
///
/// Every function throws a [FormatException] when a required member is missing or of the wrong
/// type; unknown members are ignored and unknown enum values decode to `unknown` (04 6.1).
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

/// A JSON object of the wire.
typedef Json = Map<String, Object?>;

/// The member [key] of [j] as an object.
Json obj(Json j, String key) {
  final v = j[key];
  if (v is Map) return v.cast<String, Object?>();
  throw FormatException('member $key is not an object');
}

/// The member [key] of [j] as an object, or null when absent.
Json? optObj(Json j, String key) {
  final v = j[key];
  if (v == null) return null;
  if (v is Map) return v.cast<String, Object?>();
  throw FormatException('member $key is not an object');
}

/// The member [key] of [j] as a list of objects; absent is empty.
List<Json> objList(Json j, String key) {
  final v = j[key];
  if (v == null) return const [];
  if (v is! List) throw FormatException('member $key is not a list');
  return [
    for (final e in v)
      if (e is Map)
        e.cast<String, Object?>()
      else
        throw FormatException('an element of $key is not an object'),
  ];
}

/// The member [key] of [j] as a string.
String str(Json j, String key) {
  final v = j[key];
  if (v is String) return v;
  throw FormatException('member $key is not a string');
}

/// The member [key] of [j] as a string, or null when absent.
String? optStr(Json j, String key) {
  final v = j[key];
  if (v == null) return null;
  if (v is String) return v;
  throw FormatException('member $key is not a string');
}

/// The member [key] of [j] as an integer.
int integer(Json j, String key) {
  final v = j[key];
  if (v is int) return v;
  throw FormatException('member $key is not an integer');
}

/// The member [key] of [j] as an integer, or null when absent.
int? optInt(Json j, String key) {
  final v = j[key];
  if (v == null) return null;
  if (v is int) return v;
  throw FormatException('member $key is not an integer');
}

/// The member [key] of [j] as a boolean; absent is [orElse].
bool flag(Json j, String key, {bool orElse = false}) {
  final v = j[key];
  if (v == null) return orElse;
  if (v is bool) return v;
  throw FormatException('member $key is not a boolean');
}

/// The member [key] of [j] as a time (RFC 3339).
DateTime time(Json j, String key) => DateTime.parse(str(j, key)).toUtc();

/// The member [key] of [j] as a time, or null when absent.
DateTime? optTime(Json j, String key) {
  final v = optStr(j, key);
  return v == null ? null : DateTime.parse(v).toUtc();
}

/// The member [key] of [j] as a list of strings; absent is empty.
List<String> strList(Json j, String key) {
  final v = j[key];
  if (v == null) return const [];
  if (v is! List) throw FormatException('member $key is not a list');
  return [
    for (final e in v)
      if (e is String)
        e
      else
        throw FormatException('an element of $key is not a string'),
  ];
}

T? _opt<T>(String? text, T Function(String) parse) =>
    text == null ? null : parse(text);

// ---------------------------------------------------------------------------------------------
// Messages

/// A message of the wire with what hangs on it (API `Message`): the files of its attachments,
/// the reaction summaries with the caller's own, and the components.
final class MessageBundle {
  /// Creates a bundle.
  const MessageBundle({
    required this.message,
    required this.author,
    required this.files,
    required this.attachments,
    required this.reactions,
    required this.components,
  });

  /// The message row.
  final Message message;

  /// The author (API `PrincipalRef`), stored as a principal row for display.
  final Principal author;

  /// The files of the attachments.
  final List<FileMeta> files;

  /// The attachments in their order.
  final List<MessageAttachment> attachments;

  /// The reaction summaries (`reactions` with `my_reactions`).
  final List<ReactionSummary> reactions;

  /// The components.
  final List<MessageComponent> components;
}

/// Maps an API `Message` (the member `message` of `message.created` and `message.updated`,
/// `getMessage`, an element of `listThreadMessages`).
MessageBundle messageFromJson(Json j) {
  final id = MessageId.parse(str(j, 'id'));
  final threadId = ThreadId.parse(str(j, 'thread_id'));
  final channelId = ChannelId.parse(str(j, 'channel_id'));
  final message = Message(
    id: id,
    threadId: threadId,
    channelId: channelId,
    threadSeq: ThreadSeq(integer(j, 'thread_seq')),
    authorId: PrincipalId.parse(str(obj(j, 'author'), 'id')),
    clientMsgId: MessageId.parse(str(j, 'client_msg_id')),
    origin: MessageOrigin.fromCode(str(j, 'origin')),
    kind: MessageKind.fromCode(str(j, 'kind')),
    systemCode: _opt(optStr(j, 'system_code'), SystemCode.fromCode),
    contentMd: str(j, 'content_md'),
    contentFormat: integer(j, 'content_format'),
    contentTruncated: flag(j, 'content_truncated'),
    replyToMessageId: _opt(optStr(j, 'reply_to_message_id'), MessageId.parse),
    runId: _opt(optStr(j, 'run_id'), RunId.parse),
    triggerState: _opt(optStr(j, 'trigger_state'), TriggerState.fromCode),
    triggerStateAt: optTime(j, 'trigger_state_at'),
    triggerStateReason: optStr(j, 'trigger_state_reason'),
    silent: flag(j, 'silent'),
    mentionIds: [
      for (final m in strList(j, 'mention_ids')) PrincipalId.parse(m),
    ],
    pinned: flag(j, 'pinned'),
    version: ResourceVersion(integer(j, 'version')),
    editedAt: optTime(j, 'edited_at'),
    deletedAt: optTime(j, 'deleted_at'),
    createdAt: time(j, 'created_at'),
  );
  final attachments = objList(j, 'attachments');
  final mine = strList(j, 'my_reactions').toSet();
  return MessageBundle(
    message: message,
    author: principalFromJson(obj(j, 'author')),
    files: [for (final a in attachments) fileFromJson(obj(a, 'file'))],
    attachments: [
      for (final a in attachments)
        MessageAttachment(
          messageId: id,
          fileId: FileId.parse(str(obj(a, 'file'), 'id')),
          position: integer(a, 'position'),
          threadId: threadId,
          channelId: channelId,
        ),
    ],
    reactions: [
      for (final r in objList(j, 'reactions'))
        ReactionSummary(
          messageId: id,
          emoji: str(r, 'emoji'),
          count: integer(r, 'count'),
          me: mine.contains(str(r, 'emoji')),
        ),
    ],
    components: [
      for (final c in objList(j, 'components')) componentFromJson(c, id),
    ],
  );
}

/// Maps an API `Component` of message [messageId].
MessageComponent componentFromJson(Json j, MessageId messageId) =>
    MessageComponent(
      id: ComponentId.parse(str(j, 'id')),
      messageId: messageId,
      customId: str(j, 'custom_id'),
      type: ComponentType.fromCode(str(j, 'type')),
      definition: obj(j, 'definition'),
      position: integer(j, 'position'),
      singleUse: flag(j, 'single_use'),
      requiredCapability: _opt(
        optStr(j, 'required_capability'),
        Capability.fromCode,
      ),
      restrictedTo: ComponentRestriction.fromCode(str(j, 'restricted_to')),
      runId: _opt(optStr(j, 'run_id'), RunId.parse),
      state: ComponentState.fromCode(str(j, 'state')),
      expiresAt: optTime(j, 'expires_at'),
      usedBy: _opt(optStr(j, 'used_by'), PrincipalId.parse),
      usedAt: optTime(j, 'used_at'),
      version: ResourceVersion(integer(j, 'version')),
    );

/// Maps an API `File` (file metadata; `file.updated`, attachments).
FileMeta fileFromJson(Json j) => FileMeta(
  id: FileId.parse(str(j, 'id')),
  sha256: optStr(j, 'sha256'),
  sizeBytes: integer(j, 'size_bytes'),
  mimeType: str(j, 'mime_type'),
  originalName: str(j, 'original_name'),
  uploadedBy: PrincipalId.parse(str(j, 'uploaded_by')),
  purpose: FilePurpose.fromCode(str(j, 'purpose')),
  scopeChannelId: _opt(optStr(j, 'scope_channel_id'), ChannelId.parse),
  uploadState: UploadState.fromCode(str(j, 'upload_state')),
  scanStatus: ScanStatus.fromCode(str(j, 'scan_status')),
  width: optInt(j, 'width'),
  height: optInt(j, 'height'),
  durationMs: optInt(j, 'duration_ms'),
  transcriptStatus: TranscriptStatus.fromCode(str(j, 'transcript_status')),
  transcriptLanguage: optStr(j, 'transcript_language'),
  transcriptSource: _opt(
    optStr(j, 'transcript_source'),
    TranscriptSource.fromCode,
  ),
  thumbnails: [
    for (final t in objList(j, 'thumbnails'))
      Thumbnail(
        variant: ThumbnailVariant.fromCode(str(t, 'variant')),
        mimeType: str(t, 'mime_type'),
        width: integer(t, 'width'),
        height: integer(t, 'height'),
      ),
  ],
  createdAt: time(j, 'created_at'),
);

// ---------------------------------------------------------------------------------------------
// Threads and structure

/// The `deleted_at` of an API `Space`, `Channel` or `Thread` (01 6.18, D-DEL-5): set while the
/// item is deleted, pending or final; null for an item that is not deleted. The domain models
/// do not carry it, because a deleted item is never stored (04 8.3 item 3).
DateTime? deletedAtOf(Json j) => optTime(j, 'deleted_at');

/// Whether the API object [j] carries `deleted_at` (D-DEL-5).
bool isDeleted(Json j) => deletedAtOf(j) != null;

/// Maps an API `Thread`.
Thread threadFromJson(Json j) => Thread(
  id: ThreadId.parse(str(j, 'id')),
  channelId: ChannelId.parse(str(j, 'channel_id')),
  kind: ThreadKind.fromCode(str(j, 'kind')),
  parentMessageId: _opt(optStr(j, 'parent_message_id'), MessageId.parse),
  title: str(j, 'title'),
  titleSource: TitleSource.fromCode(str(j, 'title_source')),
  state: ThreadState.fromCode(str(j, 'state')),
  closedAt: optTime(j, 'closed_at'),
  closedBy: _opt(optStr(j, 'closed_by'), PrincipalId.parse),
  operatorAgentId: _opt(optStr(j, 'operator_agent_id'), PrincipalId.parse),
  billingMode: _opt(optStr(j, 'billing_mode'), BillingMode.fromCode),
  lastMessageSeq: ThreadSeq(integer(j, 'last_message_seq')),
  unreadFloorSeq: ThreadSeq(optInt(j, 'unread_floor_seq') ?? 0),
  lastActivityAt: time(j, 'last_activity_at'),
  messageCount: integer(j, 'message_count'),
  createdBy: PrincipalId.parse(str(j, 'created_by')),
  createdAt: time(j, 'created_at'),
  movedAt: optTime(j, 'moved_at'),
  version: ResourceVersion(integer(j, 'version')),
  tagIds: [for (final t in strList(j, 'tag_ids')) ForumTagId.parse(t)],
);

/// Maps an API `Channel`.
Channel channelFromJson(Json j) {
  final op = obj(j, 'operator');
  return Channel(
    id: ChannelId.parse(str(j, 'id')),
    spaceId: _opt(optStr(j, 'space_id'), SpaceId.parse),
    kind: ChannelKind.fromCode(str(j, 'kind')),
    name: str(j, 'name'),
    topic: str(j, 'topic'),
    sortKey: str(j, 'sort_key'),
    operator: ChannelOperator(
      mode: OperatorMode.fromCode(str(op, 'mode')),
      agentId: _opt(optStr(op, 'agent_id'), PrincipalId.parse),
      effectiveAgentId: _opt(
        optStr(op, 'effective_agent_id'),
        PrincipalId.parse,
      ),
    ),
    forumDefaultSort: _opt(optStr(j, 'forum_default_sort'), ForumSort.fromCode),
    defaultNotificationLevel: _opt(
      optStr(j, 'default_notification_level'),
      NotificationLevel.fromCode,
    ),
    readOnly: flag(j, 'read_only'),
    // R-DM-54: derived by the server; absent in a payload of a server before it (false).
    projectBound: flag(j, 'project_bound'),
    mainThreadId: _opt(optStr(j, 'main_thread_id'), ThreadId.parse),
    dmParticipantIds: [
      for (final p in strList(j, 'dm_participant_ids')) PrincipalId.parse(p),
    ],
    createdAt: time(j, 'created_at'),
    archivedAt: optTime(j, 'archived_at'),
    version: ResourceVersion(integer(j, 'version')),
    settingsVersion: ResourceVersion(integer(j, 'settings_version')),
  );
}

/// Maps an API `Space`.
Space spaceFromJson(Json j) => Space(
  id: SpaceId.parse(str(j, 'id')),
  name: str(j, 'name'),
  icon: optStr(j, 'icon'),
  color: optStr(j, 'color'),
  sortKey: str(j, 'sort_key'),
  defaultOperatorAgentId: _opt(
    optStr(j, 'default_operator_agent_id'),
    PrincipalId.parse,
  ),
  createdAt: time(j, 'created_at'),
  archivedAt: optTime(j, 'archived_at'),
  version: ResourceVersion(integer(j, 'version')),
);

/// The level and capabilities of a `SpaceView`, `ChannelView` or `AccessTarget`.
AccessView accessFromJson(Json j) => AccessView(
  level: AccessLevel.fromCode(optStr(j, 'level') ?? ''),
  capabilities: [
    for (final c in strList(j, 'capabilities')) Capability.fromCode(c),
  ],
);

/// A space with the caller's access (API `SpaceView`).
final class SpaceWithAccess {
  /// Creates the pair.
  const SpaceWithAccess(this.space, this.access, {this.deletedAt});

  /// The space.
  final Space space;

  /// The caller's level and capabilities.
  final AccessView access;

  /// `Space.deleted_at`: set for a deleted space, which is never stored (D-DEL-5).
  final DateTime? deletedAt;
}

/// A channel with the caller's access (API `ChannelView`).
final class ChannelWithAccess {
  /// Creates the pair.
  const ChannelWithAccess(this.channel, this.access, {this.deletedAt});

  /// The channel.
  final Channel channel;

  /// The caller's level and capabilities.
  final AccessView access;

  /// `Channel.deleted_at`: set for a deleted channel, which is never stored (D-DEL-5).
  final DateTime? deletedAt;
}

/// Maps an API `SpaceView`.
SpaceWithAccess spaceViewFromJson(Json j) {
  final space = obj(j, 'space');
  return SpaceWithAccess(
    spaceFromJson(space),
    accessFromJson(j),
    deletedAt: deletedAtOf(space),
  );
}

/// Maps an API `ChannelView`.
ChannelWithAccess channelViewFromJson(Json j) {
  final channel = obj(j, 'channel');
  return ChannelWithAccess(
    channelFromJson(channel),
    accessFromJson(j),
    deletedAt: deletedAtOf(channel),
  );
}

// ---------------------------------------------------------------------------------------------
// Principals, agents, runs, commands, instance

/// Maps an API `PrincipalRef`.
Principal principalFromJson(Json j) => Principal(
  id: PrincipalId.parse(str(j, 'id')),
  kind: PrincipalKind.fromCode(str(j, 'kind')),
  handle: str(j, 'handle'),
  displayName: str(j, 'display_name'),
  avatarFileId: _opt(optStr(j, 'avatar_file_id'), FileId.parse),
);

/// Maps an API `User` (`user.updated`) to the principal row of a person.
Principal userFromJson(Json j) => Principal(
  id: PrincipalId.parse(str(j, 'id')),
  kind: PrincipalKind.fromCode('user'),
  handle: str(j, 'handle'),
  displayName: str(j, 'display_name'),
  avatarFileId: _opt(optStr(j, 'avatar_file_id'), FileId.parse),
);

/// Maps an API `AgentPresence`, `AgentPresencePayload` or `AgentRuntimeStatus`.
AgentPresence presenceFromJson(Json j) => AgentPresence(
  state: AgentRuntimeState.fromCode(str(j, 'state')),
  reportedBillingMode: _opt(
    optStr(j, 'reported_billing_mode'),
    BillingMode.fromCode,
  ),
);

/// Maps an API `Agent` (`agent.updated`) to the agent card.
AgentCard agentFromJson(Json j) => AgentCard(
  id: PrincipalId.parse(str(j, 'id')),
  handle: str(j, 'handle'),
  displayName: str(j, 'display_name'),
  avatarFileId: _opt(optStr(j, 'avatar_file_id'), FileId.parse),
  description: str(j, 'description'),
  billingMode: BillingMode.fromCode(str(j, 'billing_mode')),
  toolClass: ToolClass.fromCode(str(j, 'tool_class')),
  presence: presenceFromJson(obj(j, 'presence')),
  version: ResourceVersion(integer(j, 'version')),
);

/// Maps an API `RunSummary`.
RunSummary runFromJson(Json j) {
  final cost = obj(j, 'cost');
  return RunSummary(
    id: RunId.parse(str(j, 'id')),
    threadId: ThreadId.parse(str(j, 'thread_id')),
    channelId: ChannelId.parse(str(j, 'channel_id')),
    agentId: PrincipalId.parse(str(j, 'agent_id')),
    triggeredBy: PrincipalId.parse(str(j, 'triggered_by')),
    state: RunState.fromCode(str(j, 'state')),
    stateReason: optStr(j, 'state_reason'),
    version: ResourceVersion(integer(j, 'version')),
    billingModeExpected: BillingMode.fromCode(str(j, 'billing_mode_expected')),
    billingModeUsed: _opt(optStr(j, 'billing_mode_used'), BillingMode.fromCode),
    statusLine: optStr(j, 'status_line'),
    summary: optStr(j, 'summary'),
    cost: Money(
      amountMicros: integer(cost, 'amount_micros'),
      currency: str(cost, 'currency'),
      isEstimate: flag(cost, 'is_estimate'),
    ),
    stepCount: integer(j, 'step_count'),
    durationMs: optInt(j, 'duration_ms'),
    queuedAt: time(j, 'queued_at'),
    startedAt: optTime(j, 'started_at'),
    finishedAt: optTime(j, 'finished_at'),
  );
}

/// Maps an API `Command`.
Command commandFromJson(Json j) => Command(
  id: CommandId.parse(str(j, 'id')),
  ownerPrincipalId: PrincipalId.parse(str(j, 'owner_principal_id')),
  name: str(j, 'name'),
  description: str(j, 'description'),
  scopeKind: TargetKind.fromCode(str(j, 'scope_kind')),
  scopeId: optStr(j, 'scope_id'),
  options: [
    for (final o in objList(j, 'options'))
      CommandOption(
        name: str(o, 'name'),
        description: str(o, 'description'),
        type: CommandOptionType.fromCode(str(o, 'type')),
        required: flag(o, 'required'),
        choices: [
          for (final c in objList(o, 'choices'))
            CommandChoice(name: str(c, 'name'), value: str(c, 'value')),
        ],
        autocomplete: flag(o, 'autocomplete'),
      ),
  ],
  requiredLevel: CommandLevel.fromCode(str(j, 'required_level')),
  control: flag(j, 'control'),
  version: ResourceVersion(integer(j, 'version')),
);

/// The evaluated flags of `features` (`EvaluatedFlag`) by key.
Map<String, bool> featuresFromJson(List<Json> flags) => {
  for (final f in flags) str(f, 'key'): flag(f, 'enabled'),
};

/// Maps an API `InstanceInfo`.
InstanceInfo instanceFromJson(Json j) {
  final b = obj(j, 'branding');
  final l = obj(j, 'limits');
  return InstanceInfo(
    instanceId: str(j, 'instance_id'),
    branding: Branding(
      productName: str(b, 'product_name'),
      accentColor: optStr(b, 'accent_color'),
      logoFileId: _opt(optStr(b, 'logo_file_id'), FileId.parse),
      version: ResourceVersion(integer(b, 'version')),
    ),
    localeDefault: str(j, 'locale_default'),
    specVersion: str(j, 'spec_version'),
    features: featuresFromJson(objList(j, 'features')),
    limits: InstanceLimits(
      messageMaxContentBytes: integer(l, 'message_max_content_bytes'),
      messageMaxAttachments: integer(l, 'message_max_attachments'),
      fileMaxUploadBytes: integer(l, 'file_max_upload_bytes'),
      threadTitleMaxChars: integer(l, 'thread_title_max_chars'),
      voiceMaxDurationMs: integer(l, 'voice_max_duration_ms'),
    ),
    // A server before M5-08 sends no policy; the app then has none (AppLockPolicy is null).
    appLock: switch (optObj(j, 'app_lock')) {
      null => null,
      final lock => AppLockPolicy(
        required: flag(lock, 'required'),
        afterMs: integer(lock, 'after_ms'),
      ),
    },
  );
}

// ---------------------------------------------------------------------------------------------
// Personal data (04 14)

/// Maps an API `ReadState`.
ReadState readStateFromJson(Json j) => ReadState(
  threadId: ThreadId.parse(str(j, 'thread_id')),
  channelId: ChannelId.parse(str(j, 'channel_id')),
  lastReadSeq: ThreadSeq(integer(j, 'last_read_seq')),
  mentionCount: integer(j, 'mention_count'),
  unreadCount: optInt(j, 'unread_count'),
  version: ResourceVersion(integer(j, 'version')),
  updatedByDeviceId: _opt(optStr(j, 'updated_by_device_id'), DeviceId.parse),
  updatedAt: optTime(j, 'updated_at'),
);

/// Maps an API `Draft`.
Draft draftFromJson(Json j) => Draft(
  id: DraftId.parse(str(j, 'id')),
  threadId: _opt(optStr(j, 'thread_id'), ThreadId.parse),
  channelId: ChannelId.parse(str(j, 'channel_id')),
  contentMd: str(j, 'content_md'),
  replyToMessageId: _opt(optStr(j, 'reply_to_message_id'), MessageId.parse),
  fileIds: [for (final f in strList(j, 'file_ids')) FileId.parse(f)],
  version: ResourceVersion(integer(j, 'version')),
  updatedByDeviceId: _opt(optStr(j, 'updated_by_device_id'), DeviceId.parse),
  updatedAt: optTime(j, 'updated_at'),
);

/// Maps an API `UserSetting`.
UserSetting settingFromJson(Json j) => UserSetting(
  key: str(j, 'key'),
  value: j['value'],
  version: ResourceVersion(integer(j, 'version')),
  updatedByDeviceId: _opt(optStr(j, 'updated_by_device_id'), DeviceId.parse),
  updatedAt: optTime(j, 'updated_at'),
);

/// Maps an API `SidebarState`; an absent `channel_order` is null, the default order (M4-51).
UserSpaceState sidebarFromJson(Json j) => UserSpaceState(
  spaceId: SpaceId.parse(str(j, 'space_id')),
  collapsed: flag(j, 'collapsed'),
  sortKey: optStr(j, 'sort_key'),
  channelOrder: j['channel_order'] == null
      ? null
      : [for (final c in strList(j, 'channel_order')) ChannelId.parse(c)],
  version: ResourceVersion(integer(j, 'version')),
  updatedByDeviceId: _opt(optStr(j, 'updated_by_device_id'), DeviceId.parse),
);

/// Maps an API `SidebarChannelState`: the own thread order of a channel (M4-51).
UserChannelState sidebarChannelFromJson(Json j) => UserChannelState(
  channelId: ChannelId.parse(str(j, 'channel_id')),
  threadOrder: [for (final t in strList(j, 'thread_order')) ThreadId.parse(t)],
  version: ResourceVersion(integer(j, 'version')),
  updatedByDeviceId: _opt(optStr(j, 'updated_by_device_id'), DeviceId.parse),
);

/// Maps an API `NotificationPreference`.
NotificationPreference preferenceFromJson(Json j) => NotificationPreference(
  targetKind: NotificationTargetKind.fromCode(str(j, 'target_kind')),
  targetId: str(j, 'target_id'),
  level: NotificationLevel.fromCode(str(j, 'level')),
  mutedUntil: optTime(j, 'muted_until'),
  version: ResourceVersion(integer(j, 'version')),
);

/// Maps an API `QuietHoursRule`.
QuietHours quietHoursFromJson(Json j) => QuietHours(
  id: QuietHoursId.parse(str(j, 'id')),
  weekdays: integer(j, 'weekdays'),
  startLocal: str(j, 'start_local'),
  endLocal: str(j, 'end_local'),
  timeZone: optStr(j, 'time_zone'),
  enabled: flag(j, 'enabled'),
  version: ResourceVersion(integer(j, 'version')),
);

// ---------------------------------------------------------------------------------------------
// Snapshot (04 6.3 item 4)

/// One entry of `unread_threads` (API `UnreadThread`): the server's counters of a thread.
final class UnreadThread {
  /// Creates the entry.
  const UnreadThread({
    required this.threadId,
    required this.channelId,
    required this.unreadCount,
    required this.mentionCount,
    required this.lastMessageSeq,
    required this.lastReadSeq,
  });

  /// The thread.
  final ThreadId threadId;

  /// Its channel.
  final ChannelId channelId;

  /// Unread messages.
  final int unreadCount;

  /// Unread mentions.
  final int mentionCount;

  /// The newest position of the thread.
  final ThreadSeq lastMessageSeq;

  /// The caller's read position.
  final ThreadSeq lastReadSeq;
}

/// A draft reference of the snapshot (API `DraftRef`): only its version; the text comes from
/// `listMyDrafts`.
final class DraftRef {
  /// Creates the reference.
  const DraftRef({
    required this.id,
    required this.channelId,
    required this.version,
    this.threadId,
  });

  /// The draft.
  final DraftId id;

  /// Its channel.
  final ChannelId channelId;

  /// Its thread, null for a new post.
  final ThreadId? threadId;

  /// Its version on the server.
  final ResourceVersion version;
}

/// The snapshot of `ready` and `GET /api/v1/sync/snapshot` (API `Snapshot`, 04 6.3 item 4).
final class SnapshotData {
  /// Creates the snapshot.
  const SnapshotData({
    required this.headSeq,
    required this.permissionVersion,
    required this.syncEpoch,
    required this.principal,
    required this.isOwner,
    required this.spaces,
    required this.channels,
    required this.sidebar,
    this.sidebarChannels = const [],
    required this.unreadThreads,
    required this.activeRuns,
    required this.activeThreads,
    required this.agents,
    required this.settings,
    required this.drafts,
    required this.features,
  });

  /// The newest `seq` the snapshot includes; the cursor after it.
  final GlobalSeq headSeq;

  /// The permission version of the snapshot.
  final PermissionVersion permissionVersion;

  /// The sync epoch, stored with the cursor (04 7.5).
  final String syncEpoch;

  /// The person.
  final Principal principal;

  /// Whether the person is an owner of the instance.
  final bool isOwner;

  /// Visible spaces with access.
  final List<SpaceWithAccess> spaces;

  /// Visible channels with access.
  final List<ChannelWithAccess> channels;

  /// Sidebar states.
  final List<UserSpaceState> sidebar;

  /// The own thread orders of channels (`sidebar_channels`, M4-51); empty in a snapshot of an
  /// older server.
  final List<UserChannelState> sidebarChannels;

  /// Threads with unread messages or mentions.
  final List<UnreadThread> unreadThreads;

  /// Active runs.
  final List<RunSummary> activeRuns;

  /// The active threads of the visible channels (`active_threads`, 04 6.3 item 4, R-SY-28):
  /// open threads with an active run or unread for the person, plus the newest active ones up to
  /// `sync.snapshot_threads_per_channel` per channel, as `getThread` returns them. The sidebar
  /// lists them without one request per channel (D-SB-11); empty in a snapshot of an older
  /// server.
  final List<Thread> activeThreads;

  /// Presence of the visible agents.
  final Map<PrincipalId, AgentPresence> agents;

  /// Settings.
  final List<UserSetting> settings;

  /// Draft versions.
  final List<DraftRef> drafts;

  /// Evaluated feature flags.
  final Map<String, bool> features;
}

/// Maps an API `Snapshot`. A deleted space, a deleted channel, a channel of a deleted space and
/// a deleted thread are left out, so a snapshot never stores a deleted item back (D-DEL-5).
SnapshotData snapshotFromJson(Json j) {
  final spaces = [for (final s in objList(j, 'spaces')) spaceViewFromJson(s)];
  final deletedSpaces = {
    for (final v in spaces)
      if (v.deletedAt != null) v.space.id,
  };
  return SnapshotData(
    headSeq: GlobalSeq(integer(j, 'head_seq')),
    permissionVersion: PermissionVersion(integer(j, 'permission_version')),
    syncEpoch: str(j, 'sync_epoch'),
    principal: principalFromJson(obj(j, 'principal')),
    isOwner: flag(j, 'is_owner'),
    spaces: [
      for (final v in spaces)
        if (v.deletedAt == null) v,
    ],
    channels: [
      for (final c in objList(j, 'channels'))
        if (channelViewFromJson(c) case final v
            when v.deletedAt == null &&
                !deletedSpaces.contains(v.channel.spaceId))
          v,
    ],
    sidebar: [for (final s in objList(j, 'sidebar')) sidebarFromJson(s)],
    sidebarChannels: [
      for (final s in objList(j, 'sidebar_channels')) sidebarChannelFromJson(s),
    ],
    unreadThreads: [
      for (final u in objList(j, 'unread_threads'))
        UnreadThread(
          threadId: ThreadId.parse(str(u, 'thread_id')),
          channelId: ChannelId.parse(str(u, 'channel_id')),
          unreadCount: integer(u, 'unread_count'),
          mentionCount: integer(u, 'mention_count'),
          lastMessageSeq: ThreadSeq(integer(u, 'last_message_seq')),
          lastReadSeq: ThreadSeq(integer(u, 'last_read_seq')),
        ),
    ],
    activeRuns: [for (final r in objList(j, 'active_runs')) runFromJson(r)],
    activeThreads: [
      for (final t in objList(j, 'active_threads'))
        if (!isDeleted(t)) threadFromJson(t),
    ],
    agents: {
      for (final a in objList(j, 'agents'))
        PrincipalId.parse(str(a, 'agent_id')): presenceFromJson(a),
    },
    settings: [for (final s in objList(j, 'settings')) settingFromJson(s)],
    drafts: [
      for (final d in objList(j, 'drafts'))
        DraftRef(
          id: DraftId.parse(str(d, 'id')),
          channelId: ChannelId.parse(str(d, 'channel_id')),
          threadId: _opt(optStr(d, 'thread_id'), ThreadId.parse),
          version: ResourceVersion(integer(d, 'version')),
        ),
    ],
    features: featuresFromJson(objList(j, 'features')),
  );
}
