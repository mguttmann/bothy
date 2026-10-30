// SPDX-License-Identifier: Apache-2.0

/// The chips of the files attached in a composer (brief M4-22; 12 2), shared by the composer
/// of a thread and the new chat view of a forum (M4-47): name, size and the state of each
/// upload entry, "Remove", "Upload again" and, on a long paste that became a file, "Insert as
/// text".
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

import '../files/attach_controller.dart';
import '../files/attachment_mapping.dart';
import '../files/attachment_tile.dart';
import '../files/providers.dart';
import '../files/upload_executor.dart';

/// The text of [refusal] (a file over a limit, too many files, a file that cannot be read), or
/// null without one; shared by the composer of a thread and the new chat view.
String? attachRefusalText(
  AppLocalizations l,
  AttachRefusal? refusal, {
  required String locale,
}) => switch (refusal) {
  null => null,
  FileTooLarge(:final name, :final limitBytes) => l.composerFileTooLarge(
    dsIsolate(name),
    fileSizeLabel(l, limitBytes, locale: locale),
  ),
  TooManyFiles(:final limit) => l.composerTooManyFiles(limit),
  FileUnreadable(:final name) => l.composerFileUnreadable(dsIsolate(name)),
};

/// The chips of [attach]; [views] are the outbox entries of the composer's thread, or null
/// where there is no thread yet (a new post: the state then follows the upload progress).
/// [onInsertAsText] puts the text of a long paste back into the field.
class ComposerAttachmentChips extends ConsumerWidget {
  /// Creates the chips.
  const ComposerAttachmentChips({
    super.key,
    required this.attach,
    required this.views,
    required this.onInsertAsText,
  });

  /// The attachments.
  final AttachController attach;

  /// The outbox entries of the thread, or null without a thread.
  final List<OutboxEntryView>? views;

  /// "Insert as text" of the attachment with this key.
  final void Function(String key) onInsertAsText;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final progress = ref.watch(uploadProgressProvider);
    final l = AppLocalizations.of(context);
    return Semantics(
      container: true,
      label: l.composerAttachments(attach.attachments.length),
      // Up to `message_max_attachments` chips (several photos of the library at once, M5-16)
      // scroll within a quarter of the screen, so the field and the send button stay on a
      // phone.
      child: ConstrainedBox(
        constraints: BoxConstraints(
          maxHeight: MediaQuery.sizeOf(context).height / 4,
        ),
        child: SingleChildScrollView(
          child: ListenableBuilder(
            listenable: progress,
            builder: (context, _) => Wrap(
              spacing: DsSpace.s6,
              runSpacing: DsSpace.s6,
              children: [
                for (final a in attach.attachments) _chip(l, a, progress),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _chip(
    AppLocalizations l,
    PendingAttachment a,
    UploadProgress progress,
  ) {
    OutboxEntryView? view;
    for (final v in views ?? const <OutboxEntryView>[]) {
      if (v.id == a.entryId) view = v;
    }
    final code = view?.failureCode;
    final fraction = progress.fractionOf(a.key);
    return UploadChip(
      attachmentKey: a.key,
      name: a.name,
      sizeBytes: a.sizeBytes,
      // A confirmed upload left the outbox: the server checks the file now. Without a thread
      // the chip follows the progress of the upload.
      state: view != null
          ? stateOfUpload(view)
          : views == null && fraction != null && fraction < 1
          ? AttachmentState.uploading
          : AttachmentState.checking,
      percent: ((fraction ?? 0) * 100).round(),
      reason: code == null
          ? null
          : uploadFailureText(l, code, reason: view?.failureReason),
      onRemove: () => unawaited(attach.remove(a.key)),
      onRetry: () => unawaited(attach.retry(a.key)),
      onInsertAsText: a.pastedText == null ? null : () => onInsertAsText(a.key),
      // The local preview of a photo before sending (15 14, M5-07).
      previewPath: a.mimeType.startsWith('image/') ? a.path : null,
    );
  }
}
