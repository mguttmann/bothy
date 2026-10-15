// SPDX-License-Identifier: Apache-2.0

/// Rows of threads, messages, reactions, mentions and components to domain entities and back.
library;

import 'dart:convert';

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import 'codec.dart';

/// The insert companion of [thread]; [accessedAt] fills `last_accessed_at` of a new row.
ThreadsCompanion threadToCompanion(Thread thread, DateTime accessedAt) =>
    ThreadsCompanion.insert(
      id: thread.id.value,
      channelId: thread.channelId.value,
      kind: thread.kind.code,
      parentMessageId: Value(thread.parentMessageId?.value),
      title: thread.title,
      titleSource: thread.titleSource.code,
      state: thread.state.code,
      closedAt: Value(thread.closedAt),
      closedBy: Value(thread.closedBy?.value),
      operatorAgentId: Value(thread.operatorAgentId?.value),
      billingMode: Value(thread.billingMode?.code),
      lastMessageSeq: thread.lastMessageSeq.value,
      unreadFloorSeq: thread.unreadFloorSeq.value,
      lastActivityAt: thread.lastActivityAt,
      messageCount: thread.messageCount,
      createdBy: thread.createdBy.value,
      createdAt: thread.createdAt,
      movedAt: Value(thread.movedAt),
      version: thread.version.value,
      lastAccessedAt: toMillis(accessedAt),
    );

/// The thread of [row] with the tags [tagIds].
Thread threadFromRow(ThreadRow row, List<ForumTagId> tagIds) => Thread(
  id: ThreadId.parse(row.id),
  channelId: ChannelId.parse(row.channelId),
  kind: ThreadKind.fromCode(row.kind),
  parentMessageId: optional(row.parentMessageId, MessageId.parse),
  title: row.title,
  titleSource: TitleSource.fromCode(row.titleSource),
  state: ThreadState.fromCode(row.state),
  closedAt: row.closedAt,
  closedBy: optional(row.closedBy, PrincipalId.parse),
  operatorAgentId: optional(row.operatorAgentId, PrincipalId.parse),
  billingMode: optional(row.billingMode, BillingMode.fromCode),
  lastMessageSeq: ThreadSeq(row.lastMessageSeq),
  unreadFloorSeq: ThreadSeq(row.unreadFloorSeq),
  lastActivityAt: row.lastActivityAt,
  messageCount: row.messageCount,
  createdBy: PrincipalId.parse(row.createdBy),
  createdAt: row.createdAt,
  movedAt: row.movedAt,
  version: ResourceVersion(row.version),
  tagIds: List.unmodifiable(tagIds),
);

/// The insert companion of [message].
MessagesCompanion messageToCompanion(Message message) =>
    MessagesCompanion.insert(
      id: message.id.value,
      threadId: message.threadId.value,
      channelId: message.channelId.value,
      threadSeq: message.threadSeq.value,
      authorPrincipalId: message.authorId.value,
      clientMsgId: message.clientMsgId.value,
      origin: message.origin.code,
      kind: message.kind.code,
      systemCode: Value(message.systemCode?.code),
      contentMd: message.contentMd,
      contentFormat: message.contentFormat,
      contentTruncated: message.contentTruncated,
      replyToMessageId: Value(message.replyToMessageId?.value),
      runId: Value(message.runId?.value),
      triggerState: Value(message.triggerState?.code),
      triggerStateReason: Value(message.triggerStateReason),
      triggerStateAt: Value(message.triggerStateAt),
      silent: message.silent,
      mentionIds: encodeTexts(message.mentionIds),
      pinned: message.pinned,
      version: message.version.value,
      editedAt: Value(message.editedAt),
      deletedAt: Value(message.deletedAt),
      createdAt: message.createdAt,
    );

/// The message of [row].
Message messageFromRow(MessageRow row) => Message(
  id: MessageId.parse(row.id),
  threadId: ThreadId.parse(row.threadId),
  channelId: ChannelId.parse(row.channelId),
  threadSeq: ThreadSeq(row.threadSeq),
  authorId: PrincipalId.parse(row.authorPrincipalId),
  clientMsgId: MessageId.parse(row.clientMsgId),
  origin: MessageOrigin.fromCode(row.origin),
  kind: MessageKind.fromCode(row.kind),
  systemCode: optional(row.systemCode, SystemCode.fromCode),
  contentMd: row.contentMd,
  contentFormat: row.contentFormat,
  contentTruncated: row.contentTruncated,
  replyToMessageId: optional(row.replyToMessageId, MessageId.parse),
  runId: optional(row.runId, RunId.parse),
  triggerState: optional(row.triggerState, TriggerState.fromCode),
  triggerStateAt: row.triggerStateAt,
  triggerStateReason: row.triggerStateReason,
  silent: row.silent,
  mentionIds: decodeTexts(row.mentionIds, PrincipalId.parse),
  pinned: row.pinned,
  version: ResourceVersion(row.version),
  editedAt: row.editedAt,
  deletedAt: row.deletedAt,
  createdAt: row.createdAt,
);

/// The reaction summary of [row].
ReactionSummary reactionFromRow(ReactionSummaryRow row) => ReactionSummary(
  messageId: MessageId.parse(row.messageId),
  emoji: row.emoji,
  count: row.count,
  me: row.me,
);

/// The mention of [row].
Mention mentionFromRow(MentionRow row) => Mention(
  messageId: MessageId.parse(row.messageId),
  principalId: PrincipalId.parse(row.principalId),
  threadId: ThreadId.parse(row.threadId),
);

/// The insert companion of [component].
MessageComponentsCompanion componentToCompanion(MessageComponent component) =>
    MessageComponentsCompanion.insert(
      id: component.id.value,
      messageId: component.messageId.value,
      customId: component.customId,
      type: component.type.code,
      definition: jsonEncode(component.definition),
      position: component.position,
      singleUse: component.singleUse,
      requiredCapability: Value(component.requiredCapability?.code),
      restrictedTo: component.restrictedTo.code,
      runId: Value(component.runId?.value),
      state: component.state.code,
      expiresAt: Value(component.expiresAt),
      usedBy: Value(component.usedBy?.value),
      usedAt: Value(component.usedAt),
      version: component.version.value,
    );

/// The component of [row].
MessageComponent componentFromRow(MessageComponentRow row) => MessageComponent(
  id: ComponentId.parse(row.id),
  messageId: MessageId.parse(row.messageId),
  customId: row.customId,
  type: ComponentType.fromCode(row.type),
  definition: (jsonDecode(row.definition) as Map).cast<String, Object?>(),
  position: row.position,
  singleUse: row.singleUse,
  requiredCapability: optional(row.requiredCapability, Capability.fromCode),
  restrictedTo: ComponentRestriction.fromCode(row.restrictedTo),
  runId: optional(row.runId, RunId.parse),
  state: ComponentState.fromCode(row.state),
  expiresAt: row.expiresAt,
  usedBy: optional(row.usedBy, PrincipalId.parse),
  usedAt: row.usedAt,
  version: ResourceVersion(row.version),
);
