// SPDX-License-Identifier: Apache-2.0

/// Attachments in the history and the composer (12 2, 12 9; 01 8; brief M4-22 Design notes 2
/// and 3): [AttachmentTile] shows a stored file with name, size and state (a thumbnail for an
/// image, a ticket player for audio and video, Open and Save as for a servable file, the
/// tombstone of a deleted one), [MessageAttachmentsView] the files of a message, [UploadChip]
/// a file whose upload entry is in the outbox, and [attachmentResolver] the internal
/// references of the Markdown content (M4-07 Design note 2).
library;

import 'dart:async';
import 'dart:io' show File;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../voice/voice_player.dart';
import 'attachment_mapping.dart';
import 'download_service.dart';
import 'image_preview.dart';
import 'providers.dart';
import 'ticket_player.dart';

/// One stored file of a message.
class AttachmentTile extends ConsumerStatefulWidget {
  /// Creates the tile of [file]; [compact] for the files section (small thumbnail).
  const AttachmentTile({super.key, required this.file, this.compact = false});

  /// The file.
  final FileMeta file;

  /// Whether the tile is the short form of a list.
  final bool compact;

  @override
  ConsumerState<AttachmentTile> createState() => _AttachmentTileState();
}

class _AttachmentTileState extends ConsumerState<AttachmentTile> {
  String? _notice;
  bool _busy = false;
  TicketPlayer? _player;
  StreamSubscription<PlayerPhase>? _phase;
  bool _playing = false;

  @override
  void dispose() {
    unawaited(_phase?.cancel());
    unawaited(_player?.dispose());
    super.dispose();
  }

  String? _noticeOf(AppLocalizations l, DownloadResult result) =>
      switch (result) {
        Downloaded() || DownloadCancelled() => null,
        DownloadDamaged() => l.fileDownloadDamaged,
        DownloadNotReady() => l.fileStateChecking,
        DownloadBlocked() => l.fileStateBlocked,
        DownloadFailed(:final code, :final reason) => l.fileDownloadFailed(
          uploadFailureText(l, code, reason: reason),
        ),
      };

  Future<void> _run(
    Future<DownloadResult> Function(DownloadService d) action, {
    String? Function(AppLocalizations l)? done,
  }) async {
    final l = AppLocalizations.of(context);
    final downloads = ref.read(downloadServiceProvider);
    if (downloads == null || _busy) return;
    setState(() {
      _busy = true;
      _notice = null;
    });
    DownloadResult result;
    try {
      result = await action(downloads);
    } on Object {
      result = const DownloadFailed('save_failed');
    }
    if (!mounted) return;
    setState(() {
      _busy = false;
      _notice = result is Downloaded ? done?.call(l) : _noticeOf(l, result);
    });
  }

  Future<void> _open() =>
      _run((d) => d.open(widget.file, ref.read(filePortsProvider).opener));

  Future<void> _save() => _run(
    (d) => d.saveAs(widget.file, ref.read(filePortsProvider).picker),
    done: (l) => l.fileSaved(dsIsolate(widget.file.originalName)),
  );

  Future<void> _play() async {
    final l = AppLocalizations.of(context);
    final tickets = ref.read(fileTicketsProvider);
    if (tickets == null) return;
    if (_playing) {
      await _player?.pause();
      setState(() => _playing = false);
      return;
    }
    try {
      // 12 9.2: a new ticket per playback; it stays in memory and goes to the player only.
      final url = await tickets.url(widget.file.id);
      if (!mounted) return;
      if (isVideo(widget.file)) {
        await showVideoDialog(
          context,
          name: widget.file.originalName,
          ticket: url,
          surface: ref.read(filePortsProvider).video,
        );
        return;
      }
      final player = _player ??= ref.read(filePortsProvider).player();
      _phase ??= player.phases.listen((p) {
        if (mounted) {
          setState(() => _playing = p == PlayerPhase.playing);
        }
      });
      await player.play(url);
      if (mounted) setState(() => _playing = true);
    } on Object {
      if (mounted) setState(() => _notice = l.fileMediaFailed);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final file = widget.file;
    final state = stateOfFile(file);
    final locale = Localizations.localeOf(context).languageCode;
    final key = 'file.${file.id.value}';
    if (state == AttachmentState.deleted) {
      // 04 2.2 rule 5: a deleted file keeps only its tombstone fields.
      return KeyedSubtree(
        key: ValueKey(key),
        child: Text(
          l.fileDeleted,
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: colors.textMuted, fontStyle: FontStyle.italic),
        ),
      );
    }
    final size = fileSizeLabel(l, file.sizeBytes, locale: locale);
    final stateText = attachmentStateLabel(l, state);
    final ready = state == AttachmentState.ready;
    final image = ready && isImage(file);
    final media = ready && (isAudio(file) || isVideo(file));
    Widget button(String suffix, String label, VoidCallback onTap) => DsButton(
      key: ValueKey('$key.$suffix'),
      label: label,
      style: DsButtonStyle.ghost,
      loading: _busy,
      onPressed: _busy ? null : onTap,
    );
    final details = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          dsIsolate(file.originalName),
          overflow: TextOverflow.ellipsis,
          style: dsTextStyle(DsType.bodySm).copyWith(color: colors.textPrimary),
        ),
        Text(
          size,
          style: dsTextStyle(DsType.caption)
              .copyWith(color: colors.textSecondary),
        ),
        // The state in words, never only in color (01 8).
        if (stateText.isNotEmpty)
          Text(
            stateText,
            key: ValueKey('$key.state'),
            style: dsTextStyle(DsType.caption).copyWith(
              color: state == AttachmentState.ready
                  ? colors.textSecondary
                  : colors.textPrimary,
            ),
          ),
      ],
    );
    return Semantics(
      container: true,
      label: l.fileTileLabel(file.originalName, size, stateText),
      child: Container(
        key: ValueKey(key),
        padding: const EdgeInsets.all(DsSpace.s8),
        decoration: BoxDecoration(
          color: colors.surfaceSunken,
          borderRadius: BorderRadius.circular(DsRadius.sm),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            if (image) ...[
              ImagePreview(
                file: file,
                size: widget.compact ? PreviewSize.small : PreviewSize.medium,
              ),
              const SizedBox(height: DsSpace.s4),
            ],
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                DsIcon(
                  state == AttachmentState.blocked
                      ? DsIcons.stateWarning
                      : state == AttachmentState.failed
                      ? DsIcons.stateError
                      : DsIcons.actionAttach,
                  size: DsIconSizeRole.sm,
                  color: colors.textSecondary,
                  decorative: true,
                ),
                const SizedBox(width: DsSpace.s6),
                // The label of the tile already reads name, size and state; the texts are not
                // read a second time (brief M4-32).
                Flexible(child: ExcludeSemantics(child: details)),
              ],
            ),
            // The actions wrap below the name, so 200 percent text never overflows (01 8).
            if (media || (ready && !widget.compact))
              Wrap(
                spacing: DsSpace.s4,
                children: [
                  if (media)
                    button(
                      'play',
                      _playing
                          ? l.fileMediaPause
                          : l.fileMediaPlay(dsIsolate(file.originalName)),
                      () => unawaited(_play()),
                    ),
                  if (ready && !widget.compact) ...[
                    button('open', l.fileOpen, () => unawaited(_open())),
                    button('save', l.fileSaveAs, () => unawaited(_save())),
                  ],
                ],
              ),
            if (_notice case final notice?)
              Padding(
                padding: const EdgeInsets.only(top: DsSpace.s4),
                child: Text(
                  notice,
                  key: ValueKey('$key.notice'),
                  style: dsTextStyle(DsType.caption)
                      .copyWith(color: colors.textSecondary),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// The files of the message [messageId]: a voice message ([voice]) shows its player with the
/// transcript, every other message one [AttachmentTile] per file.
class MessageAttachmentsView extends ConsumerWidget {
  /// Creates the view.
  const MessageAttachmentsView({
    super.key,
    required this.messageId,
    this.voice = false,
  });

  /// The message.
  final MessageId messageId;

  /// Whether it is a voice message (`kind = voice`, 12 8.1).
  final bool voice;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final files =
        ref.watch(messageFilesProvider(messageId.value)).value ?? const [];
    if (files.isEmpty) return const SizedBox.shrink();
    if (voice && files.length == 1 && isAudio(files.single)) {
      return VoicePlayer(file: files.single);
    }
    return Padding(
      padding: const EdgeInsets.only(top: DsSpace.s4),
      child: Wrap(
        spacing: DsSpace.s6,
        runSpacing: DsSpace.s6,
        children: [for (final f in files) AttachmentTile(file: f)],
      ),
    );
  }
}

/// A file whose upload entry is in the outbox (12 2 "Upload running" and its failures): name,
/// size, [state] with [percent], and the actions Remove ([onRemove], the composer) and Upload
/// again ([onRetry], after a failure).
class UploadChip extends StatelessWidget {
  /// Creates the chip of the attachment [attachmentKey].
  const UploadChip({
    super.key,
    required this.attachmentKey,
    required this.name,
    required this.sizeBytes,
    required this.state,
    this.percent = 0,
    this.reason,
    this.onRemove,
    this.onRetry,
    this.previewPath,
  });

  /// The attachment key.
  final String attachmentKey;

  /// The local image file shown as the preview before sending (15 14; brief M5-07 Design
  /// note 4), or null for the attachment icon; once the upload deleted the file the icon
  /// shows again.
  final String? previewPath;

  /// The file name (a foreign name).
  final String name;

  /// The size.
  final int sizeBytes;

  /// The state.
  final AttachmentState state;

  /// The confirmed share while uploading, 0 to 100.
  final int percent;

  /// The reason of a failed upload, or null.
  final String? reason;

  /// Removes the attachment, or null.
  final VoidCallback? onRemove;

  /// Sends a failed upload again, or null.
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final locale = Localizations.localeOf(context).languageCode;
    final size = fileSizeLabel(l, sizeBytes, locale: locale);
    final stateText = attachmentStateLabel(l, state, percent: percent);
    final failed =
        state == AttachmentState.failed || state == AttachmentState.canceled;
    return Semantics(
      container: true,
      label: l.fileTileLabel(name, size, stateText),
      child: Container(
        key: ValueKey('upload.$attachmentKey'),
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s8,
          vertical: DsSpace.s4,
        ),
        decoration: BoxDecoration(
          color: colors.surfaceSunken,
          border: failed
              ? Border.all(
                  color: colors.dangerBorder,
                  width: DsBorderWidth.hairline,
                )
              : null,
          borderRadius: BorderRadius.circular(DsRadius.sm),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (previewPath case final path? when !failed)
              ClipRRect(
                key: ValueKey('upload.$attachmentKey.preview'),
                borderRadius: BorderRadius.circular(DsRadius.sm),
                child: Image.file(
                  File(path),
                  width: DsSize.avatarLg,
                  height: DsSize.avatarLg,
                  fit: BoxFit.cover,
                  // Decoded at the shown size, not the photo's full resolution.
                  cacheWidth:
                      (DsSize.avatarLg * MediaQuery.devicePixelRatioOf(context))
                          .round(),
                  excludeFromSemantics: true,
                  errorBuilder: (_, _, _) => DsIcon(
                    DsIcons.actionAttach,
                    size: DsIconSizeRole.sm,
                    color: colors.textSecondary,
                    decorative: true,
                  ),
                ),
              )
            else
              DsIcon(
                failed ? DsIcons.stateError : DsIcons.actionAttach,
                size: DsIconSizeRole.sm,
                color: failed ? colors.dangerFg : colors.textSecondary,
                decorative: true,
              ),
            const SizedBox(width: DsSpace.s4),
            Flexible(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Name, size and state are the label of the chip; read once (brief M4-32).
                  ExcludeSemantics(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          dsIsolate(name),
                          overflow: TextOverflow.ellipsis,
                          style: dsTextStyle(DsType.bodySm)
                              .copyWith(color: colors.textPrimary),
                        ),
                        Text(
                          size,
                          style: dsTextStyle(DsType.caption)
                              .copyWith(color: colors.textSecondary),
                        ),
                        Text(
                          stateText,
                          style: dsTextStyle(DsType.caption).copyWith(
                            color: failed
                                ? colors.dangerFg
                                : colors.textSecondary,
                          ),
                        ),
                      ],
                    ),
                  ),
                  if (reason case final why? when failed)
                    Text(
                      why,
                      style: dsTextStyle(DsType.caption)
                          .copyWith(color: colors.textSecondary),
                    ),
                  if (failed && onRetry != null)
                    DsButton(
                      key: ValueKey('upload.$attachmentKey.retry'),
                      label: l.fileUploadAgain,
                      style: DsButtonStyle.ghost,
                      onPressed: onRetry,
                    ),
                  if (state == AttachmentState.uploading)
                    SizedBox(
                      width: DsSize.avatarLg * 2,
                      child: LinearProgressIndicator(
                        value: percent / 100,
                        color: context.dsAccent.accentFg,
                        backgroundColor: colors.surfaceRaised,
                      ),
                    ),
                ],
              ),
            ),
            if (onRemove != null)
              DsTappable(
                key: ValueKey('upload.$attachmentKey.remove'),
                onTap: onRemove,
                label: l.fileRemove(name),
                excludeChildSemantics: true,
                child: DsIcon(
                  DsIcons.actionDismiss,
                  size: DsIconSizeRole.sm,
                  color: colors.textSecondary,
                  decorative: true,
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// The resolver of internal references (`![alt](attachment:<file_id>)`, D-M4-22-7) over the
/// image [files] of one message: a reference to one of them shows its thumbnail from the
/// server; every other URL stays text (SEC-OUT-2).
DsAttachmentResolver attachmentResolver(List<FileMeta> files) {
  final images = {
    for (final f in files)
      if (isImage(f) && stateOfFile(f) == AttachmentState.ready) f.id: f,
  };
  return (uri) {
    final id = attachmentReference(uri);
    final file = id == null ? null : images[id];
    return file == null
        ? null
        : ImagePreview(
            key: ValueKey('file.${file.id.value}.inline'),
            file: file,
          );
  };
}
