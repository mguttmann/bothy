// SPDX-License-Identifier: Apache-2.0

/// Rows of file metadata and attachments to domain entities and back.
library;

import 'dart:convert';

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';

/// The insert companion of [file].
FilesCompanion fileToCompanion(FileMeta file) => FilesCompanion.insert(
  id: file.id.value,
  sha256: Value(file.sha256),
  sizeBytes: file.sizeBytes,
  mimeType: file.mimeType,
  originalName: file.originalName,
  uploadedBy: file.uploadedBy.value,
  purpose: file.purpose.code,
  scopeChannelId: Value(file.scopeChannelId?.value),
  uploadState: file.uploadState.code,
  scanStatus: file.scanStatus.code,
  width: Value(file.width),
  height: Value(file.height),
  durationMs: Value(file.durationMs),
  transcriptStatus: file.transcriptStatus.code,
  transcriptLanguage: Value(file.transcriptLanguage),
  transcriptSource: Value(file.transcriptSource?.code),
  thumbnails: jsonEncode([
    for (final t in file.thumbnails)
      {
        'variant': t.variant.code,
        'mime_type': t.mimeType,
        'width': t.width,
        'height': t.height,
      },
  ]),
  createdAt: file.createdAt,
);

/// The file metadata of [row].
FileMeta fileFromRow(FileRow row) => FileMeta(
  id: FileId.parse(row.id),
  sha256: row.sha256,
  sizeBytes: row.sizeBytes,
  mimeType: row.mimeType,
  originalName: row.originalName,
  uploadedBy: PrincipalId.parse(row.uploadedBy),
  purpose: FilePurpose.fromCode(row.purpose),
  scopeChannelId: row.scopeChannelId == null
      ? null
      : ChannelId.parse(row.scopeChannelId!),
  uploadState: UploadState.fromCode(row.uploadState),
  scanStatus: ScanStatus.fromCode(row.scanStatus),
  width: row.width,
  height: row.height,
  durationMs: row.durationMs,
  transcriptStatus: TranscriptStatus.fromCode(row.transcriptStatus),
  transcriptLanguage: row.transcriptLanguage,
  transcriptSource: row.transcriptSource == null
      ? null
      : TranscriptSource.fromCode(row.transcriptSource!),
  thumbnails: List.unmodifiable([
    for (final t in jsonDecode(row.thumbnails) as List)
      Thumbnail(
        variant: ThumbnailVariant.fromCode((t as Map)['variant'] as String),
        mimeType: t['mime_type'] as String,
        width: t['width'] as int,
        height: t['height'] as int,
      ),
  ]),
  createdAt: row.createdAt,
);

/// The insert companion of [attachment].
MessageAttachmentsCompanion attachmentToCompanion(
  MessageAttachment attachment,
) => MessageAttachmentsCompanion.insert(
  messageId: attachment.messageId.value,
  fileId: attachment.fileId.value,
  position: attachment.position,
  threadId: attachment.threadId.value,
  channelId: attachment.channelId.value,
);

/// The attachment of [row].
MessageAttachment attachmentFromRow(MessageAttachmentRow row) =>
    MessageAttachment(
      messageId: MessageId.parse(row.messageId),
      fileId: FileId.parse(row.fileId),
      position: row.position,
      threadId: ThreadId.parse(row.threadId),
      channelId: ChannelId.parse(row.channelId),
    );
