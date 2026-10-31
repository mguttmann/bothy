// SPDX-License-Identifier: Apache-2.0

/// The outbox in the history (01 6.6; 04 13.2 items 3 and 5, 13.3): a new message shows at
/// once after the last confirmed one with "Sending", "Sending delayed" after
/// `client.outbox_stuck_after`, or refused with the reason and "Send again", "Edit" and
/// "Discard"; another refused write shows its reason, an edit conflict offers both versions
/// and an edit of a deleted message keeps its text for copying.
library;

import 'dart:convert';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

import '../files/attachment_mapping.dart';
import '../files/attachment_tile.dart';
import '../files/providers.dart';
import '../files/upload_executor.dart';

/// The delivery of [view] in the message component.
DsDelivery deliveryOf(OutboxEntryView view) => switch (view.stateLabel) {
  OutboxStateLabel.failed => DsDelivery.failed,
  OutboxStateLabel.delayed => DsDelivery.delayed,
  _ => DsDelivery.sending,
};

/// The translated reason of a refused [view]: its own failure or the failure of an entry it
/// waits for (an upload, 15 7.5).
String? failureTextOf(AppLocalizations l, OutboxEntryView view) {
  final code = view.failureCode ?? view.blockedByCode;
  if (code == null) return null;
  return uploadFailureText(
    l,
    code,
    reason: view.failureReason ?? view.blockedByReason,
  );
}

/// A new message of the outbox as a message of the reader [authorName].
class PendingRow extends StatelessWidget {
  /// Creates the row.
  const PendingRow({
    super.key,
    required this.view,
    required this.authorName,
    required this.timeLabel,
    required this.collapseLines,
    required this.onResend,
    required this.onEdit,
    required this.onDiscard,
    required this.onOpenLink,
    this.uploads = const {},
    this.progress,
    this.onRetryUpload,
  });

  /// The entry.
  final OutboxEntryView view;

  /// The upload entries of the thread by attachment key (M4-22).
  final Map<String, OutboxEntryView> uploads;

  /// The progress of the uploads, or null.
  final UploadProgress? progress;

  /// "Upload again" of a failed upload entry, with its ID.
  final ValueChanged<String>? onRetryUpload;

  /// The reader's name.
  final String authorName;

  /// The time of the entry, formatted by the caller.
  final String timeLabel;

  /// `ui.message_collapse_lines`.
  final int collapseLines;

  /// "Send again" with a new ID.
  final VoidCallback onResend;

  /// "Edit".
  final VoidCallback onEdit;

  /// "Discard".
  final VoidCallback onDiscard;

  /// Opens a link of the content.
  final void Function(Uri) onOpenLink;

  /// The files of the waiting message: one chip per attachment with the state of its upload
  /// entry; a confirmed upload is being checked by the server (12 2).
  Widget? _files(AppLocalizations l) {
    final preview = view.payload['attachment_preview'];
    if (preview is! List || preview.isEmpty) return null;
    return Padding(
      padding: const EdgeInsets.only(
        left: DsSize.avatarMd + DsSpace.s12 + DsSpace.s8,
        top: DsSpace.s4,
      ),
      child: Wrap(
        spacing: DsSpace.s6,
        runSpacing: DsSpace.s6,
        children: [
          for (final item in preview.cast<Map<String, Object?>>())
            _chip(l, item),
        ],
      ),
    );
  }

  Widget _chip(AppLocalizations l, Map<String, Object?> item) {
    final key = item['key']! as String;
    final upload = uploads[key];
    if (upload == null) {
      // A confirmed upload left the outbox: the chip shows the stored file, which
      // file.updated keeps current (12 2, FIX-scan-state-stale).
      final slot = view.payload[UploadOps.slotOf(key)];
      final fileId = slot is String ? slot : progress?.completedFileOf(key);
      return Consumer(
        builder: (context, ref, _) => UploadChip(
          attachmentKey: key,
          name: item['name'] as String? ?? '',
          sizeBytes: item['size_bytes'] as int? ?? 0,
          state: fileId == null
              ? AttachmentState.checking
              : stateAfterUpload(ref.watch(fileMetaProvider(fileId)).value),
        ),
      );
    }
    final state = stateOfUpload(upload);
    final code = upload.failureCode;
    return UploadChip(
      attachmentKey: key,
      name: item['name'] as String? ?? '',
      sizeBytes: item['size_bytes'] as int? ?? 0,
      state: state,
      percent: ((progress?.fractionOf(key) ?? 0) * 100).round(),
      reason: code == null
          ? null
          : uploadFailureText(l, code, reason: upload.failureReason),
      onRetry: onRetryUpload == null ? null : () => onRetryUpload!(upload.id),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final delivery = deliveryOf(view);
    final files = _files(l);
    final message = _message(l, delivery);
    if (files == null) return message;
    final live = progress;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        message,
        // The chips follow every confirmed part, as in the composer.
        if (live == null)
          files
        else
          ListenableBuilder(
            listenable: live,
            builder: (context, _) => _files(l) ?? const SizedBox.shrink(),
          ),
      ],
    );
  }

  Widget _message(AppLocalizations l, DsDelivery delivery) {
    return DsMessage(
      DsMessageData(
        id: view.id,
        authorName: authorName,
        origin: DsMessageOrigin.human,
        kind: DsMessageKind.text,
        timeLabel: timeLabel,
        content: view.payload['content_md'] as String? ?? '',
        delivery: delivery,
        failureText: delivery == DsDelivery.failed
            ? failureTextOf(l, view)
            : null,
      ),
      collapseLines: collapseLines,
      onOpenLink: onOpenLink,
      onResend: onResend,
      onEdit: onEdit,
      onDiscard: onDiscard,
    );
  }
}

/// Another refused write of the thread: the reason, and for an edit "Compare" (a conflict)
/// or the text with "Copy" (a deleted message); "Send again" and "Discard" for the rest.
class FailedWriteRow extends StatelessWidget {
  /// Creates the row.
  const FailedWriteRow({
    super.key,
    required this.view,
    required this.onCompare,
    required this.onResend,
    required this.onDiscard,
  });

  /// The failed entry.
  final OutboxEntryView view;

  /// Opens the conflict dialog with [mine] and [theirs].
  final void Function(String mine, String theirs) onCompare;

  /// "Send again" with a new ID.
  final VoidCallback onResend;

  /// "Discard".
  final VoidCallback onDiscard;

  /// The text of the server's current revision of a conflict, or null.
  String? get _theirs {
    final current = view.entry.conflictCurrent;
    if (current == null) return null;
    final json = jsonDecode(current);
    return json is Map ? json['content_md'] as String? : null;
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final mine = view.payload['content_md'] as String?;
    final theirs = _theirs;
    final deleted = view.failureCode == 'message_deleted';
    final reason = failureTextOf(l, view) ?? '';
    Widget button(String key, String label, VoidCallback onTap) => DsButton(
      key: ValueKey(key),
      label: label,
      style: DsButtonStyle.ghost,
      onPressed: onTap,
    );
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s8,
        vertical: DsSpace.s4,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          DsBanner(
            tone: DsTone.danger,
            text: l.outboxFailedAction(reason),
            liveRegion: true,
          ),
          if (mine != null && (deleted || theirs == null))
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s4),
              child: SelectableText(
                mine,
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: colors.textPrimary),
              ),
            ),
          Wrap(
            spacing: DsSpace.s6,
            children: [
              if (theirs != null && mine != null)
                button('outbox.compare', l.outboxCompare, () {
                  onCompare(mine, theirs);
                }),
              if (deleted && mine != null)
                button('outbox.copy', l.outboxCopy, () {
                  Clipboard.setData(ClipboardData(text: mine));
                }),
              if (!deleted && theirs == null)
                button('outbox.resend', l.messageResend, onResend),
              button('outbox.discard', l.messageDiscard, onDiscard),
            ],
          ),
        ],
      ),
    );
  }
}
