// SPDX-License-Identifier: Apache-2.0

/// Threads and their tags (03 `threads`, `thread_tags`; 15 6.1).
library;

import '../coded.dart';
import '../ids.dart';
import '../seq.dart';
import 'principal.dart';

/// Kind of a thread (03 `threads.kind`).
enum ThreadKind implements Coded {
  /// Continuous history of a text or DM channel.
  main('main'),

  /// A thread.
  thread('thread'),

  /// A value of a newer server.
  unknown('');

  const ThreadKind(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static ThreadKind fromCode(String code) => decodeCode(values, code, unknown);
}

/// State of a thread (03 `threads.state`, API `ThreadState`).
enum ThreadState implements Coded {
  /// Open.
  open('open'),

  /// Done (01 6.1).
  closed('closed'),

  /// A value of a newer server.
  unknown('');

  const ThreadState(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static ThreadState fromCode(String code) => decodeCode(values, code, unknown);
}

/// Source of a thread title (03 `threads.title_source`).
enum TitleSource implements Coded {
  /// Generated.
  auto('auto'),

  /// Set by a person; stays manual.
  manual('manual'),

  /// A value of a newer server.
  unknown('');

  const TitleSource(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static TitleSource fromCode(String code) => decodeCode(values, code, unknown);
}

/// A thread (03 `threads`, API `Thread`).
final class Thread {
  /// Creates a [Thread].
  const Thread({
    required this.id,
    required this.channelId,
    required this.kind,
    required this.title,
    required this.titleSource,
    required this.state,
    required this.lastMessageSeq,
    required this.unreadFloorSeq,
    required this.lastActivityAt,
    required this.messageCount,
    required this.createdBy,
    required this.createdAt,
    required this.version,
    required this.tagIds,
    this.parentMessageId,
    this.closedAt,
    this.closedBy,
    this.operatorAgentId,
    this.billingMode,
    this.movedAt,
  });

  /// ID; fixed across a move.
  final ThreadId id;

  /// Channel.
  final ChannelId channelId;

  /// Kind.
  final ThreadKind kind;

  /// Message the thread arose from.
  final MessageId? parentMessageId;

  /// Title.
  final String title;

  /// Title source.
  final TitleSource titleSource;

  /// State.
  final ThreadState state;

  /// Time it was marked done.
  final DateTime? closedAt;

  /// Who marked it done.
  final PrincipalId? closedBy;

  /// Operator fixed at creation.
  final PrincipalId? operatorAgentId;

  /// Billing mode of the operator, if delivered.
  final BillingMode? billingMode;

  /// Highest `thread_seq` of the thread.
  final ThreadSeq lastMessageSeq;

  /// Messages up to this position count as unread for nobody (imports, 04 14).
  final ThreadSeq unreadFloorSeq;

  /// Last activity; sorts the post list.
  final DateTime lastActivityAt;

  /// Messages excluding deleted ones.
  final int messageCount;

  /// Creator.
  final PrincipalId createdBy;

  /// Creation time.
  final DateTime createdAt;

  /// Last move.
  final DateTime? movedAt;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Forum tags (03 `thread_tags`).
  final List<ForumTagId> tagIds;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  Thread copyWith({
    ChannelId? channelId,
    ThreadKind? kind,
    MessageId? parentMessageId,
    String? title,
    TitleSource? titleSource,
    ThreadState? state,
    DateTime? closedAt,
    PrincipalId? closedBy,
    PrincipalId? operatorAgentId,
    BillingMode? billingMode,
    ThreadSeq? lastMessageSeq,
    ThreadSeq? unreadFloorSeq,
    DateTime? lastActivityAt,
    int? messageCount,
    PrincipalId? createdBy,
    DateTime? createdAt,
    DateTime? movedAt,
    ResourceVersion? version,
    List<ForumTagId>? tagIds,
  }) => Thread(
    id: id,
    channelId: channelId ?? this.channelId,
    kind: kind ?? this.kind,
    parentMessageId: parentMessageId ?? this.parentMessageId,
    title: title ?? this.title,
    titleSource: titleSource ?? this.titleSource,
    state: state ?? this.state,
    closedAt: closedAt ?? this.closedAt,
    closedBy: closedBy ?? this.closedBy,
    operatorAgentId: operatorAgentId ?? this.operatorAgentId,
    billingMode: billingMode ?? this.billingMode,
    lastMessageSeq: lastMessageSeq ?? this.lastMessageSeq,
    unreadFloorSeq: unreadFloorSeq ?? this.unreadFloorSeq,
    lastActivityAt: lastActivityAt ?? this.lastActivityAt,
    messageCount: messageCount ?? this.messageCount,
    createdBy: createdBy ?? this.createdBy,
    createdAt: createdAt ?? this.createdAt,
    movedAt: movedAt ?? this.movedAt,
    version: version ?? this.version,
    tagIds: tagIds ?? this.tagIds,
  );

  @override
  bool operator ==(Object other) =>
      other is Thread &&
      other.id == id &&
      other.channelId == channelId &&
      other.kind == kind &&
      other.parentMessageId == parentMessageId &&
      other.title == title &&
      other.titleSource == titleSource &&
      other.state == state &&
      other.closedAt == closedAt &&
      other.closedBy == closedBy &&
      other.operatorAgentId == operatorAgentId &&
      other.billingMode == billingMode &&
      other.lastMessageSeq == lastMessageSeq &&
      other.unreadFloorSeq == unreadFloorSeq &&
      other.lastActivityAt == lastActivityAt &&
      other.messageCount == messageCount &&
      other.createdBy == createdBy &&
      other.createdAt == createdAt &&
      other.movedAt == movedAt &&
      other.version == version &&
      listEquals(other.tagIds, tagIds);

  @override
  int get hashCode => Object.hash(
    id,
    channelId,
    kind,
    parentMessageId,
    title,
    titleSource,
    state,
    closedAt,
    closedBy,
    operatorAgentId,
    billingMode,
    lastMessageSeq,
    unreadFloorSeq,
    lastActivityAt,
    messageCount,
    createdBy,
    createdAt,
    movedAt,
    version,
    Object.hashAll(tagIds),
  );
}

/// Assignment of a forum tag to a thread (03 `thread_tags`).
final class ThreadTag {
  /// Creates a [ThreadTag].
  const ThreadTag({required this.threadId, required this.tagId});

  /// Thread.
  final ThreadId threadId;

  /// Tag.
  final ForumTagId tagId;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  ThreadTag copyWith({ThreadId? threadId, ForumTagId? tagId}) => ThreadTag(
    threadId: threadId ?? this.threadId,
    tagId: tagId ?? this.tagId,
  );

  @override
  bool operator ==(Object other) =>
      other is ThreadTag && other.threadId == threadId && other.tagId == tagId;

  @override
  int get hashCode => Object.hash(threadId, tagId);
}
