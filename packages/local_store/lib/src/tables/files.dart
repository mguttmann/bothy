// SPDX-License-Identifier: Apache-2.0

/// Mirror tables of file metadata (15 6.1: `files`, `message_attachments`); the bytes live in
/// the file cache on disk (M4-22), `file_cache` keeps their paths.
library;

import 'package:drift/drift.dart';

/// 03 `files` without `object_key`, the scan engine and the transcript text; the thumbnails of
/// the API `File` as JSON. The index serves the removal of a channel (04 8.3), which looks up
/// the files of its scope (version 16).
@DataClassName('FileRow')
@TableIndex(name: 'files_scope_channel', columns: {#scopeChannelId})
class Files extends Table {
  @override
  String get tableName => 'files';

  /// 03 `files.id`.
  TextColumn get id => text()();

  /// 03 `files.sha256`.
  TextColumn get sha256 => text().nullable()();

  /// 03 `files.size_bytes`.
  IntColumn get sizeBytes => integer()();

  /// 03 `files.mime_type`.
  TextColumn get mimeType => text()();

  /// 03 `files.original_name`.
  TextColumn get originalName => text()();

  /// 03 `files.uploaded_by`.
  TextColumn get uploadedBy => text()();

  /// 03 `files.purpose`.
  TextColumn get purpose => text()();

  /// 03 `files.scope_channel_id`.
  TextColumn get scopeChannelId => text().nullable()();

  /// 03 `files.upload_state`.
  TextColumn get uploadState => text()();

  /// 03 `files.scan_status`.
  TextColumn get scanStatus => text()();

  /// 03 `files.width`.
  IntColumn get width => integer().nullable()();

  /// 03 `files.height`.
  IntColumn get height => integer().nullable()();

  /// 03 `files.duration_ms`.
  IntColumn get durationMs => integer().nullable()();

  /// 03 `files.transcript_status`.
  TextColumn get transcriptStatus => text()();

  /// 03 `files.transcript_language`.
  TextColumn get transcriptLanguage => text().nullable()();

  /// 03 `files.transcript_source`.
  TextColumn get transcriptSource => text().nullable()();

  /// 03 `file_thumbnails` as a JSON array of the API `Thumbnail`.
  TextColumn get thumbnails => text()();

  /// 03 `files.created_at`.
  DateTimeColumn get createdAt => dateTime()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

/// 03 `message_attachments`. The indexes on the thread and the channel serve the deletion of
/// a thread (cleanup, removal), the files of a thread and the removal of a channel (version
/// 16).
@DataClassName('MessageAttachmentRow')
@TableIndex(name: 'message_attachments_file', columns: {#fileId})
@TableIndex(name: 'message_attachments_thread', columns: {#threadId})
@TableIndex(name: 'message_attachments_channel', columns: {#channelId})
class MessageAttachments extends Table {
  @override
  String get tableName => 'message_attachments';

  /// 03 `message_attachments.message_id`.
  TextColumn get messageId => text()();

  /// 03 `message_attachments.file_id`.
  TextColumn get fileId => text()();

  /// 03 `message_attachments.position`.
  IntColumn get position => integer()();

  /// 03 `message_attachments.thread_id`.
  TextColumn get threadId => text()();

  /// 03 `message_attachments.channel_id`.
  TextColumn get channelId => text()();

  @override
  Set<Column<Object>> get primaryKey => {messageId, fileId};
}
