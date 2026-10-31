// SPDX-License-Identifier: Apache-2.0

/// The states of a file as the interface shows them (12 2; brief M4-22 Goal and Design note
/// 3): the views of the table of 12 2 from the stored metadata, the upload entry of the outbox
/// while the bytes are sent, the size and the reason texts, and the internal reference form
/// of images in the Markdown content (`attachment:<file_id>`, D-M4-22-7).
library;

import 'package:domain/domain.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

/// The view of a file (12 2).
enum AttachmentState {
  /// The upload entry waits in the outbox (offline, or its turn has not come).
  waiting,

  /// Parts are being sent ("Upload running").
  uploading,

  /// The server refused an attempt for the moment (`429`, `5xx`, no network) and the entry
  /// waits for its next one: "Paused, trying again shortly" (FIX-upload-retry-storm).
  retrying,

  /// The client is done, the server checks: "being checked".
  checking,

  /// Size or checksum wrong, or the upload was refused: "Upload failed".
  failed,

  /// The session was aborted or expired.
  canceled,

  /// Servable.
  ready,

  /// The scan found something or failed: "blocked".
  blocked,

  /// Deleted: only the tombstone fields remain (04 2.2 rule 5).
  deleted,
}

/// The view of the stored [file] (12 2): servable is `ready`; `complete` with `infected` or
/// `error` is `blocked`; `failed` is "Upload failed"; everything else waits for the check. A
/// redacted file (empty name, size 0, no thumbnails) is `deleted`.
AttachmentState stateOfFile(FileMeta file) {
  if (isTombstone(file)) return AttachmentState.deleted;
  if (file.uploadState == UploadState.failed) return AttachmentState.failed;
  if (file.uploadState == UploadState.complete) {
    return switch (file.scanStatus) {
      ScanStatus.clean || ScanStatus.skipped => AttachmentState.ready,
      ScanStatus.infected || ScanStatus.error => AttachmentState.blocked,
      _ => AttachmentState.checking,
    };
  }
  return AttachmentState.checking;
}

/// Whether [file] is the tombstone of a deleted file (04 2.2 rule 5: `original_name` `""`,
/// `size_bytes` `0`, no thumbnails).
bool isTombstone(FileMeta file) =>
    file.originalName.isEmpty && file.sizeBytes == 0 && file.thumbnails.isEmpty;

/// The view of an upload entry of the outbox: [OutboxStateLabel.failed] is "Upload failed"
/// (or canceled after `410 gone`), a request in flight is uploading, an entry backing off
/// after a refused attempt is paused, and one whose turn has not come waits.
AttachmentState stateOfUpload(OutboxEntryView view) =>
    switch (view.stateLabel) {
      OutboxStateLabel.failed =>
        view.failureCode == 'gone'
            ? AttachmentState.canceled
            : AttachmentState.failed,
      OutboxStateLabel.sending => AttachmentState.uploading,
      OutboxStateLabel.waiting ||
      OutboxStateLabel.delayed => AttachmentState.retrying,
      OutboxStateLabel.pending => AttachmentState.waiting,
    };

/// The label of [state]; [percent] is the share of confirmed bytes while uploading.
String attachmentStateLabel(
  AppLocalizations l,
  AttachmentState state, {
  int percent = 0,
}) => switch (state) {
  AttachmentState.waiting => l.fileStateWaiting,
  AttachmentState.uploading => l.fileStateUploading(percent),
  AttachmentState.retrying => l.fileStateRetrying,
  AttachmentState.checking => l.fileStateChecking,
  AttachmentState.failed => l.fileStateUploadFailed,
  AttachmentState.canceled => l.fileStateCanceled,
  AttachmentState.ready => '',
  AttachmentState.blocked => l.fileStateBlocked,
  AttachmentState.deleted => l.fileDeleted,
};

/// [bytes] in B, KB, MB or GB (powers of 1024) with one decimal above 1 KB, in the number
/// format of [locale].
String fileSizeLabel(AppLocalizations l, int bytes, {required String locale}) {
  const unit = 1024;
  String one(double v) {
    final text = v.toStringAsFixed(v >= 100 ? 0 : 1);
    return locale.startsWith('de') ? text.replaceAll('.', ',') : text;
  }

  if (bytes < unit) return l.fileSizeBytes('$bytes');
  if (bytes < unit * unit) return l.fileSizeKilobytes(one(bytes / unit));
  if (bytes < unit * unit * unit) {
    return l.fileSizeMegabytes(one(bytes / (unit * unit)));
  }
  return l.fileSizeGigabytes(one(bytes / (unit * unit * unit)));
}

/// The reason of a failed upload or a refused attachment: the reasons of 12 3.6 and 12 10,
/// else the problem text of the code.
String uploadFailureText(AppLocalizations l, String code, {String? reason}) =>
    switch ((code, reason)) {
      ('conflict', 'part_conflict') => l.fileFailPartConflict,
      ('conflict', 'quota_exceeded') => l.fileFailQuota,
      ('conflict', 'file_not_attachable') => l.fileFailNotAttachable,
      ('gone', _) => l.fileFailSessionEnded,
      ('file_unreadable', _) => l.fileFailUnreadable,
      _ => problemText(l, code, reason: reason),
    };

/// Whether [file] is an image the server serves inline with thumbnails (12 7.3).
bool isImage(FileMeta file) =>
    file.mimeType.startsWith('image/') && file.thumbnails.isNotEmpty;

/// Whether [file] is audio for a ticket player (12 9.3 inline list): MP4 audio under both
/// names the server's type detection gives (`audio/x-m4a` for an Apple recording) and MP3.
bool isAudio(FileMeta file) =>
    const {'audio/mp4', 'audio/x-m4a', 'audio/mpeg'}.contains(file.mimeType);

/// Whether [file] is a video for a ticket player (12 9.3 inline list).
bool isVideo(FileMeta file) => file.mimeType == 'video/mp4';

/// The scheme of the internal reference to an attachment of the same message in the Markdown
/// content: `![alt](attachment:<file_id>)` (D-M4-22-7; M4-07 Design note 2).
const String attachmentScheme = 'attachment';

/// The file ID of an internal reference [uri], or null for any other URL.
FileId? attachmentReference(Uri uri) {
  if (uri.scheme != attachmentScheme) return null;
  try {
    return FileId.parse(uri.path);
  } on FormatException {
    return null;
  }
}
