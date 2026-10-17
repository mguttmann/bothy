// SPDX-License-Identifier: Apache-2.0

/// File metadata and attachments (03 `files`, `message_attachments`; 15 6.1): metadata only, bytes live in the file cache.
library;

import '../coded.dart';
import '../ids.dart';

/// Purpose of a file (03 `files.purpose`).
enum FilePurpose implements Coded {
  /// Attachment.
  attachment('attachment'),

  /// Avatar.
  avatar('avatar'),

  /// Branding.
  branding('branding'),

  /// Privacy export.
  gdprExport('gdpr_export'),

  /// A value of a newer server.
  unknown('');

  const FilePurpose(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static FilePurpose fromCode(String code) => decodeCode(values, code, unknown);
}

/// Upload state of a file (03 `files.upload_state`).
enum UploadState implements Coded {
  /// Pending.
  pending('pending'),

  /// Complete.
  complete('complete'),

  /// Failed.
  failed('failed'),

  /// A value of a newer server.
  unknown('');

  const UploadState(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static UploadState fromCode(String code) => decodeCode(values, code, unknown);
}

/// Malware scan state (03 `files.scan_status`).
enum ScanStatus implements Coded {
  /// Pending.
  pending('pending'),

  /// Clean.
  clean('clean'),

  /// Infected.
  infected('infected'),

  /// Error.
  error('error'),

  /// Skipped.
  skipped('skipped'),

  /// A value of a newer server.
  unknown('');

  const ScanStatus(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static ScanStatus fromCode(String code) => decodeCode(values, code, unknown);
}

/// Transcript state (03 `files.transcript_status`, API `TranscriptStatus`).
enum TranscriptStatus implements Coded {
  /// None.
  none('none'),

  /// Pending.
  pending('pending'),

  /// Done.
  done('done'),

  /// Failed.
  failed('failed'),

  /// A value of a newer server.
  unknown('');

  const TranscriptStatus(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static TranscriptStatus fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// Source of a transcript (03 `files.transcript_source`, API `TranscriptSource`).
enum TranscriptSource implements Coded {
  /// An agent.
  agent('agent'),

  /// The transcription service.
  service('service'),

  /// A value of a newer server.
  unknown('');

  const TranscriptSource(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static TranscriptSource fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// Variant of a thumbnail (API `Thumbnail.variant`).
enum ThumbnailVariant implements Coded {
  /// Small.
  small('small'),

  /// Medium.
  medium('medium'),

  /// Large.
  large('large'),

  /// Video poster.
  poster('poster'),

  /// A value of a newer server.
  unknown('');

  const ThumbnailVariant(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static ThumbnailVariant fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// A thumbnail of a file (API `Thumbnail`).
final class Thumbnail {
  /// Creates a [Thumbnail].
  const Thumbnail({
    required this.variant,
    required this.mimeType,
    required this.width,
    required this.height,
  });

  /// Variant.
  final ThumbnailVariant variant;

  /// Media type.
  final String mimeType;

  /// Width in pixels.
  final int width;

  /// Height in pixels.
  final int height;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  Thumbnail copyWith({
    ThumbnailVariant? variant,
    String? mimeType,
    int? width,
    int? height,
  }) => Thumbnail(
    variant: variant ?? this.variant,
    mimeType: mimeType ?? this.mimeType,
    width: width ?? this.width,
    height: height ?? this.height,
  );

  @override
  bool operator ==(Object other) =>
      other is Thumbnail &&
      other.variant == variant &&
      other.mimeType == mimeType &&
      other.width == width &&
      other.height == height;

  @override
  int get hashCode => Object.hash(variant, mimeType, width, height);
}

/// Metadata of a file (03 `files`, API `File`). Named so it never clashes with `dart:io` in the apps.
final class FileMeta {
  /// Creates a [FileMeta].
  const FileMeta({
    required this.id,
    required this.sizeBytes,
    required this.mimeType,
    required this.originalName,
    required this.uploadedBy,
    required this.purpose,
    required this.uploadState,
    required this.scanStatus,
    required this.transcriptStatus,
    required this.thumbnails,
    required this.createdAt,
    this.sha256,
    this.scopeChannelId,
    this.width,
    this.height,
    this.durationMs,
    this.transcriptLanguage,
    this.transcriptSource,
  });

  /// ID.
  final FileId id;

  /// Hash of the stored bytes, if delivered.
  final String? sha256;

  /// Size in bytes.
  final int sizeBytes;

  /// Media type detected by the server.
  final String mimeType;

  /// Sanitized original name.
  final String originalName;

  /// Uploader.
  final PrincipalId uploadedBy;

  /// Purpose.
  final FilePurpose purpose;

  /// Channel it was uploaded for.
  final ChannelId? scopeChannelId;

  /// Upload state.
  final UploadState uploadState;

  /// Scan state.
  final ScanStatus scanStatus;

  /// Width of images and videos.
  final int? width;

  /// Height of images and videos.
  final int? height;

  /// Duration of audio and video in milliseconds.
  final int? durationMs;

  /// Transcript state.
  final TranscriptStatus transcriptStatus;

  /// BCP 47 language of the transcript.
  final String? transcriptLanguage;

  /// Source of the transcript.
  final TranscriptSource? transcriptSource;

  /// Thumbnails.
  final List<Thumbnail> thumbnails;

  /// Upload time.
  final DateTime createdAt;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  FileMeta copyWith({
    String? sha256,
    int? sizeBytes,
    String? mimeType,
    String? originalName,
    PrincipalId? uploadedBy,
    FilePurpose? purpose,
    ChannelId? scopeChannelId,
    UploadState? uploadState,
    ScanStatus? scanStatus,
    int? width,
    int? height,
    int? durationMs,
    TranscriptStatus? transcriptStatus,
    String? transcriptLanguage,
    TranscriptSource? transcriptSource,
    List<Thumbnail>? thumbnails,
    DateTime? createdAt,
  }) => FileMeta(
    id: id,
    sha256: sha256 ?? this.sha256,
    sizeBytes: sizeBytes ?? this.sizeBytes,
    mimeType: mimeType ?? this.mimeType,
    originalName: originalName ?? this.originalName,
    uploadedBy: uploadedBy ?? this.uploadedBy,
    purpose: purpose ?? this.purpose,
    scopeChannelId: scopeChannelId ?? this.scopeChannelId,
    uploadState: uploadState ?? this.uploadState,
    scanStatus: scanStatus ?? this.scanStatus,
    width: width ?? this.width,
    height: height ?? this.height,
    durationMs: durationMs ?? this.durationMs,
    transcriptStatus: transcriptStatus ?? this.transcriptStatus,
    transcriptLanguage: transcriptLanguage ?? this.transcriptLanguage,
    transcriptSource: transcriptSource ?? this.transcriptSource,
    thumbnails: thumbnails ?? this.thumbnails,
    createdAt: createdAt ?? this.createdAt,
  );

  @override
  bool operator ==(Object other) =>
      other is FileMeta &&
      other.id == id &&
      other.sha256 == sha256 &&
      other.sizeBytes == sizeBytes &&
      other.mimeType == mimeType &&
      other.originalName == originalName &&
      other.uploadedBy == uploadedBy &&
      other.purpose == purpose &&
      other.scopeChannelId == scopeChannelId &&
      other.uploadState == uploadState &&
      other.scanStatus == scanStatus &&
      other.width == width &&
      other.height == height &&
      other.durationMs == durationMs &&
      other.transcriptStatus == transcriptStatus &&
      other.transcriptLanguage == transcriptLanguage &&
      other.transcriptSource == transcriptSource &&
      listEquals(other.thumbnails, thumbnails) &&
      other.createdAt == createdAt;

  @override
  int get hashCode => Object.hash(
    id,
    sha256,
    sizeBytes,
    mimeType,
    originalName,
    uploadedBy,
    purpose,
    scopeChannelId,
    uploadState,
    scanStatus,
    width,
    height,
    durationMs,
    transcriptStatus,
    transcriptLanguage,
    transcriptSource,
    Object.hashAll(thumbnails),
    createdAt,
  );
}

/// A file attached to a message (03 `message_attachments`).
final class MessageAttachment {
  /// Creates a [MessageAttachment].
  const MessageAttachment({
    required this.messageId,
    required this.fileId,
    required this.position,
    required this.threadId,
    required this.channelId,
  });

  /// Message.
  final MessageId messageId;

  /// File.
  final FileId fileId;

  /// Order.
  final int position;

  /// Thread of the message.
  final ThreadId threadId;

  /// Channel of the message.
  final ChannelId channelId;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  MessageAttachment copyWith({
    int? position,
    ThreadId? threadId,
    ChannelId? channelId,
  }) => MessageAttachment(
    messageId: messageId,
    fileId: fileId,
    position: position ?? this.position,
    threadId: threadId ?? this.threadId,
    channelId: channelId ?? this.channelId,
  );

  @override
  bool operator ==(Object other) =>
      other is MessageAttachment &&
      other.messageId == messageId &&
      other.fileId == fileId &&
      other.position == position &&
      other.threadId == threadId &&
      other.channelId == channelId;

  @override
  int get hashCode =>
      Object.hash(messageId, fileId, position, threadId, channelId);
}
