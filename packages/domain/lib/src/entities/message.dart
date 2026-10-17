// SPDX-License-Identifier: Apache-2.0

/// Messages (03 `messages`; 15 6.1): without `client_payload_sha256`, with `trigger_state`.
library;

import '../coded.dart';
import '../ids.dart';
import '../seq.dart';

/// Kind of a message (03 `messages.kind`, API `MessageKind`).
enum MessageKind implements Coded {
  /// Text.
  text('text'),

  /// Voice message.
  voice('voice'),

  /// Anchor of a live box.
  run('run'),

  /// Message with components.
  card('card'),

  /// System line.
  system('system'),

  /// A value of a newer server.
  unknown('');

  const MessageKind(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static MessageKind fromCode(String code) => decodeCode(values, code, unknown);
}

/// Origin of a message (03 `messages.origin`, API `MessageOrigin`).
enum MessageOrigin implements Coded {
  /// A person; only this triggers agents.
  human('human'),

  /// An agent.
  agent('agent'),

  /// An app.
  app('app'),

  /// An import.
  import('import'),

  /// The server.
  system('system'),

  /// A value of a newer server.
  unknown('');

  const MessageOrigin(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static MessageOrigin fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// State of a triggering message (03 `messages.trigger_state`, API `TriggerState`, 09 6).
enum TriggerState implements Coded {
  /// Received.
  received('received'),

  /// Queued.
  queued('queued'),

  /// Working.
  working('working'),

  /// Done.
  done('done'),

  /// Failed.
  failed('failed'),

  /// Cancelled.
  cancelled('cancelled'),

  /// Delivered as a follow-up.
  followupDelivered('followup_delivered'),

  /// A value of a newer server.
  unknown('');

  const TriggerState(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static TriggerState fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// Code of a system line (03 `messages.system_code`); the client builds the text in its language.
enum SystemCode implements Coded {
  /// A thread was reopened.
  threadReopened('thread.reopened'),

  /// An agent session could not be resumed.
  agentSessionResumeFailed('agent_session.resume_failed'),

  /// A value of a newer server.
  unknown('');

  const SystemCode(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static SystemCode fromCode(String code) => decodeCode(values, code, unknown);
}

/// A message (03 `messages`, API `Message`). Attachments, reactions and components are separate entities.
final class Message {
  /// Creates a [Message].
  const Message({
    required this.id,
    required this.threadId,
    required this.channelId,
    required this.threadSeq,
    required this.authorId,
    required this.clientMsgId,
    required this.origin,
    required this.kind,
    required this.contentMd,
    required this.contentFormat,
    required this.contentTruncated,
    required this.silent,
    required this.mentionIds,
    required this.pinned,
    required this.version,
    required this.createdAt,
    this.systemCode,
    this.replyToMessageId,
    this.runId,
    this.triggerState,
    this.triggerStateAt,
    this.triggerStateReason,
    this.editedAt,
    this.deletedAt,
  });

  /// ID.
  final MessageId id;

  /// Thread.
  final ThreadId threadId;

  /// Channel (copy of the thread's channel).
  final ChannelId channelId;

  /// Gapless position in the thread (04 4).
  final ThreadSeq threadSeq;

  /// Author.
  final PrincipalId authorId;

  /// UUIDv7 the author's client created (04 5.1).
  final MessageId clientMsgId;

  /// Origin.
  final MessageOrigin origin;

  /// Kind.
  final MessageKind kind;

  /// Code of a system line.
  final SystemCode? systemCode;

  /// Markdown; empty after deletion and for system lines.
  final String contentMd;

  /// Version of the Markdown dialect.
  final int contentFormat;

  /// Whether the content was truncated in the event; the client reloads it (04 2.2).
  final bool contentTruncated;

  /// Quoted message.
  final MessageId? replyToMessageId;

  /// Run of a live box anchor.
  final RunId? runId;

  /// Trigger state; absent for a message that triggers nothing.
  final TriggerState? triggerState;

  /// Time of the trigger state.
  final DateTime? triggerStateAt;

  /// Reason code for `failed` and `cancelled`.
  final String? triggerStateReason;

  /// No notification.
  final bool silent;

  /// Mentioned principals; the client counts mentions from them (04 14).
  final List<PrincipalId> mentionIds;

  /// Whether the message is pinned.
  final bool pinned;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Last edit.
  final DateTime? editedAt;

  /// Deletion time of a tombstone.
  final DateTime? deletedAt;

  /// Creation time.
  final DateTime createdAt;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  Message copyWith({
    ThreadId? threadId,
    ChannelId? channelId,
    ThreadSeq? threadSeq,
    PrincipalId? authorId,
    MessageId? clientMsgId,
    MessageOrigin? origin,
    MessageKind? kind,
    SystemCode? systemCode,
    String? contentMd,
    int? contentFormat,
    bool? contentTruncated,
    MessageId? replyToMessageId,
    RunId? runId,
    TriggerState? triggerState,
    DateTime? triggerStateAt,
    String? triggerStateReason,
    bool? silent,
    List<PrincipalId>? mentionIds,
    bool? pinned,
    ResourceVersion? version,
    DateTime? editedAt,
    DateTime? deletedAt,
    DateTime? createdAt,
  }) => Message(
    id: id,
    threadId: threadId ?? this.threadId,
    channelId: channelId ?? this.channelId,
    threadSeq: threadSeq ?? this.threadSeq,
    authorId: authorId ?? this.authorId,
    clientMsgId: clientMsgId ?? this.clientMsgId,
    origin: origin ?? this.origin,
    kind: kind ?? this.kind,
    systemCode: systemCode ?? this.systemCode,
    contentMd: contentMd ?? this.contentMd,
    contentFormat: contentFormat ?? this.contentFormat,
    contentTruncated: contentTruncated ?? this.contentTruncated,
    replyToMessageId: replyToMessageId ?? this.replyToMessageId,
    runId: runId ?? this.runId,
    triggerState: triggerState ?? this.triggerState,
    triggerStateAt: triggerStateAt ?? this.triggerStateAt,
    triggerStateReason: triggerStateReason ?? this.triggerStateReason,
    silent: silent ?? this.silent,
    mentionIds: mentionIds ?? this.mentionIds,
    pinned: pinned ?? this.pinned,
    version: version ?? this.version,
    editedAt: editedAt ?? this.editedAt,
    deletedAt: deletedAt ?? this.deletedAt,
    createdAt: createdAt ?? this.createdAt,
  );

  @override
  bool operator ==(Object other) =>
      other is Message &&
      other.id == id &&
      other.threadId == threadId &&
      other.channelId == channelId &&
      other.threadSeq == threadSeq &&
      other.authorId == authorId &&
      other.clientMsgId == clientMsgId &&
      other.origin == origin &&
      other.kind == kind &&
      other.systemCode == systemCode &&
      other.contentMd == contentMd &&
      other.contentFormat == contentFormat &&
      other.contentTruncated == contentTruncated &&
      other.replyToMessageId == replyToMessageId &&
      other.runId == runId &&
      other.triggerState == triggerState &&
      other.triggerStateAt == triggerStateAt &&
      other.triggerStateReason == triggerStateReason &&
      other.silent == silent &&
      listEquals(other.mentionIds, mentionIds) &&
      other.pinned == pinned &&
      other.version == version &&
      other.editedAt == editedAt &&
      other.deletedAt == deletedAt &&
      other.createdAt == createdAt;

  @override
  int get hashCode => Object.hashAll([
    id,
    threadId,
    channelId,
    threadSeq,
    authorId,
    clientMsgId,
    origin,
    kind,
    systemCode,
    contentMd,
    contentFormat,
    contentTruncated,
    replyToMessageId,
    runId,
    triggerState,
    triggerStateAt,
    triggerStateReason,
    silent,
    Object.hashAll(mentionIds),
    pinned,
    version,
    editedAt,
    deletedAt,
    createdAt,
  ]);
}
