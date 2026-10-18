// SPDX-License-Identifier: Apache-2.0

/// Mirror tables of threads and messages (15 6.1: `threads`, `thread_tags`, `messages`,
/// `reactions_summary`, `my_mentions`, `message_components`).
library;

import 'package:drift/drift.dart';

/// 03 `threads` with the local column `last_accessed_at` for the cleanup (15 6.3).
@DataClassName('ThreadRow')
@TableIndex(name: 'threads_channel', columns: {#channelId})
@TableIndex(name: 'threads_last_accessed', columns: {#lastAccessedAt})
class Threads extends Table {
  @override
  String get tableName => 'threads';

  /// 03 `threads.id`.
  TextColumn get id => text()();

  /// 03 `threads.channel_id`.
  TextColumn get channelId => text()();

  /// 03 `threads.kind`.
  TextColumn get kind => text()();

  /// 03 `threads.parent_message_id`.
  TextColumn get parentMessageId => text().nullable()();

  /// 03 `threads.title`.
  TextColumn get title => text()();

  /// 03 `threads.title_source`.
  TextColumn get titleSource => text()();

  /// 03 `threads.state`.
  TextColumn get state => text()();

  /// 03 `threads.closed_at`.
  DateTimeColumn get closedAt => dateTime().nullable()();

  /// 03 `threads.closed_by`.
  TextColumn get closedBy => text().nullable()();

  /// 03 `threads.operator_agent_id`.
  TextColumn get operatorAgentId => text().nullable()();

  /// API `Thread.billing_mode` of the operator.
  TextColumn get billingMode => text().nullable()();

  /// 03 `threads.last_message_seq`.
  IntColumn get lastMessageSeq => integer()();

  /// 03 `threads.unread_floor_seq`.
  IntColumn get unreadFloorSeq => integer()();

  /// 03 `threads.last_activity_at`.
  DateTimeColumn get lastActivityAt => dateTime()();

  /// 03 `threads.message_count`.
  IntColumn get messageCount => integer()();

  /// 03 `threads.created_by`.
  TextColumn get createdBy => text()();

  /// 03 `threads.created_at`.
  DateTimeColumn get createdAt => dateTime()();

  /// 03 `threads.moved_at`.
  DateTimeColumn get movedAt => dateTime().nullable()();

  /// 03 `threads.version`.
  IntColumn get version => integer()();

  /// Local: last time the person opened the thread, milliseconds since the epoch (15 6.3).
  IntColumn get lastAccessedAt => integer()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

/// 03 `thread_tags`.
@DataClassName('ThreadTagRow')
class ThreadTags extends Table {
  @override
  String get tableName => 'thread_tags';

  /// 03 `thread_tags.thread_id`.
  TextColumn get threadId => text()();

  /// 03 `thread_tags.tag_id`.
  TextColumn get tagId => text()();

  @override
  Set<Column<Object>> get primaryKey => {threadId, tagId};
}

/// 03 `messages` without `client_payload_sha256` (15 6.1), with `trigger_state`; a tombstone
/// keeps its `thread_seq` (03 6.5).
@DataClassName('MessageRow')
@TableIndex(name: 'messages_thread_seq', columns: {#threadId, #threadSeq})
class Messages extends Table {
  @override
  String get tableName => 'messages';

  /// 03 `messages.id`.
  TextColumn get id => text()();

  /// 03 `messages.thread_id`.
  TextColumn get threadId => text()();

  /// 03 `messages.channel_id`.
  TextColumn get channelId => text()();

  /// 03 `messages.thread_seq`.
  IntColumn get threadSeq => integer()();

  /// 03 `messages.author_principal_id`.
  TextColumn get authorPrincipalId => text()();

  /// 03 `messages.client_msg_id`.
  TextColumn get clientMsgId => text()();

  /// 03 `messages.origin`.
  TextColumn get origin => text()();

  /// 03 `messages.kind`.
  TextColumn get kind => text()();

  /// 03 `messages.system_code`.
  TextColumn get systemCode => text().nullable()();

  /// 03 `messages.content_md`; empty for a tombstone.
  TextColumn get contentMd => text()();

  /// 03 `messages.content_format`.
  IntColumn get contentFormat => integer()();

  /// API `Message.content_truncated` (04 2.2).
  BoolColumn get contentTruncated => boolean()();

  /// 03 `messages.reply_to_message_id`.
  TextColumn get replyToMessageId => text().nullable()();

  /// 03 `messages.run_id`.
  TextColumn get runId => text().nullable()();

  /// 03 `messages.trigger_state`.
  TextColumn get triggerState => text().nullable()();

  /// 03 `messages.trigger_state_reason`.
  TextColumn get triggerStateReason => text().nullable()();

  /// 03 `messages.trigger_state_at`.
  DateTimeColumn get triggerStateAt => dateTime().nullable()();

  /// 03 `messages.silent`.
  BoolColumn get silent => boolean()();

  /// API `Message.mention_ids` as a JSON array (03 `mentions` of every principal).
  TextColumn get mentionIds => text()();

  /// 03 `pins` as a flag; `pin.*` set it without changing `version` (04 9 item 3).
  BoolColumn get pinned => boolean()();

  /// 03 `messages.version`.
  IntColumn get version => integer()();

  /// 03 `messages.edited_at`.
  DateTimeColumn get editedAt => dateTime().nullable()();

  /// 03 `messages.deleted_at`; set on a tombstone.
  DateTimeColumn get deletedAt => dateTime().nullable()();

  /// 03 `messages.created_at`.
  DateTimeColumn get createdAt => dateTime()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

/// 15 6.1 `reactions_summary` of 03 `reactions`: count and "me" per message and emoji.
@DataClassName('ReactionSummaryRow')
class ReactionsSummary extends Table {
  @override
  String get tableName => 'reactions_summary';

  /// 03 `reactions.message_id`.
  TextColumn get messageId => text()();

  /// 03 `reactions.emoji`.
  TextColumn get emoji => text()();

  /// Number of reactions with this emoji.
  IntColumn get count => integer()();

  /// Whether the person reacted with this emoji.
  BoolColumn get me => boolean()();

  @override
  Set<Column<Object>> get primaryKey => {messageId, emoji};
}

/// 15 6.1 `my_mentions` of 03 `mentions`: only mentions of the person themselves.
@DataClassName('MentionRow')
@TableIndex(name: 'my_mentions_thread', columns: {#threadId})
class MyMentions extends Table {
  @override
  String get tableName => 'my_mentions';

  /// 03 `mentions.message_id`.
  TextColumn get messageId => text()();

  /// 03 `mentions.principal_id`.
  TextColumn get principalId => text()();

  /// 03 `mentions.thread_id`.
  TextColumn get threadId => text()();

  @override
  Set<Column<Object>> get primaryKey => {messageId, principalId};
}

/// 03 `message_components`.
@DataClassName('MessageComponentRow')
@TableIndex(name: 'message_components_message', columns: {#messageId})
class MessageComponents extends Table {
  @override
  String get tableName => 'message_components';

  /// 03 `message_components.id`.
  TextColumn get id => text()();

  /// 03 `message_components.message_id`.
  TextColumn get messageId => text()();

  /// 03 `message_components.custom_id`.
  TextColumn get customId => text()();

  /// 03 `message_components.type`.
  TextColumn get type => text()();

  /// 03 `message_components.definition` as JSON text.
  TextColumn get definition => text()();

  /// 03 `message_components.position`.
  IntColumn get position => integer()();

  /// 03 `message_components.single_use`.
  BoolColumn get singleUse => boolean()();

  /// 03 `message_components.required_capability`.
  TextColumn get requiredCapability => text().nullable()();

  /// 03 `message_components.restricted_to`.
  TextColumn get restrictedTo => text()();

  /// 03 `message_components.run_id`.
  TextColumn get runId => text().nullable()();

  /// 03 `message_components.state`.
  TextColumn get state => text()();

  /// 03 `message_components.expires_at`.
  DateTimeColumn get expiresAt => dateTime().nullable()();

  /// 03 `message_components.used_by`.
  TextColumn get usedBy => text().nullable()();

  /// 03 `message_components.used_at`.
  DateTimeColumn get usedAt => dateTime().nullable()();

  /// 03 `message_components.version`.
  IntColumn get version => integer()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}
