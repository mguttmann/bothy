// SPDX-License-Identifier: Apache-2.0

/// Images from the server only (SEC-OUT-2, T-AI-06; 12 7.3; brief M4-22 Design note 3): the
/// thumbnail of an attachment from `downloadThumbnail` through the file cache, and the full
/// view from `downloadFile` on a click. Nothing is ever loaded from a URL in the content; a
/// thumbnail that answers `409 file_not_ready` shows "being checked" and loads again on
/// `file.updated`, `409 file_blocked` shows "blocked".
library;

import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import 'attachment_mapping.dart';
import 'download_service.dart';
import 'providers.dart';

/// The thumbnail sizes of the history and of lists (12 7.3: `medium` in the chat, `small` in
/// lists).
enum PreviewSize {
  /// `small`, for the files section of the session column.
  small('small', DsSize.avatarLg * 2),

  /// `medium`, for the history.
  medium('medium', DsSize.avatarLg * 6);

  const PreviewSize(this.variant, this.extent);

  /// The thumbnail variant (12 7.3).
  final String variant;

  /// The largest edge in logical pixels.
  final double extent;
}

/// The thumbnail of the image [file]; a click opens the full view.
class ImagePreview extends ConsumerWidget {
  /// Creates the preview of [file] in [size].
  const ImagePreview({
    super.key,
    required this.file,
    this.size = PreviewSize.medium,
  });

  /// The image.
  final FileMeta file;

  /// The thumbnail size.
  final PreviewSize size;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final result = ref
        .watch(thumbnailProvider(file.id.value, size.variant))
        .value;
    Widget label(String text) => Text(
      text,
      key: ValueKey('file.${file.id.value}.state'),
      style: dsTextStyle(DsType.caption).copyWith(color: colors.textSecondary),
    );
    final Widget body = switch (result) {
      Downloaded(:final file) => Image.file(
        file,
        key: ValueKey('file.${this.file.id.value}.thumbnail'),
        fit: BoxFit.contain,
        width: size.extent,
        height: size.extent,
        semanticLabel: l.fileImageOpen(this.file.originalName),
        gaplessPlayback: true,
      ),
      DownloadNotReady() => label(l.fileStateChecking),
      DownloadBlocked() => label(l.fileStateBlocked),
      DownloadDamaged() => label(l.fileDownloadDamaged),
      DownloadFailed(:final code) => label(
        l.fileDownloadFailed(uploadFailureText(l, code)),
      ),
      _ => SizedBox.square(
        dimension: size.extent / 2,
        child: Semantics(
          label: l.fileImageLoading,
          child: DecoratedBox(
            decoration: BoxDecoration(
              color: colors.surfaceSunken,
              borderRadius: BorderRadius.circular(DsRadius.sm),
            ),
          ),
        ),
      ),
    };
    if (result is! Downloaded) return body;
    return DsTappable(
      key: ValueKey('file.${file.id.value}.view'),
      onTap: () => showImageViewer(context, file),
      label: l.fileImageOpen(dsIsolate(file.originalName)),
      excludeChildSemantics: true,
      child: ClipRRect(
        borderRadius: BorderRadius.circular(DsRadius.sm),
        child: body,
      ),
    );
  }
}

/// Opens the full view of the image [file]: the original from `downloadFile` (12 9.1), with
/// Save as and Close.
Future<void> showImageViewer(BuildContext context, FileMeta file) =>
    showDialog<void>(
      context: context,
      builder: (context) => _ImageViewer(file: file),
    );

class _ImageViewer extends ConsumerStatefulWidget {
  const _ImageViewer({required this.file});

  final FileMeta file;

  @override
  ConsumerState<_ImageViewer> createState() => _ImageViewerState();
}

class _ImageViewerState extends ConsumerState<_ImageViewer> {
  late final Future<DownloadResult?> _original =
      ref
          .read(downloadServiceProvider)
          ?.fetch(widget.file.id, name: widget.file.originalName) ??
      Future.value();

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Dialog(
      key: const ValueKey('file.viewer'),
      child: Padding(
        padding: const EdgeInsets.all(DsSpace.s16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              dsIsolate(widget.file.originalName),
              style: dsTextStyle(DsType.heading)
                  .copyWith(color: colors.textPrimary),
            ),
            const SizedBox(height: DsSpace.s12),
            Flexible(
              child: FutureBuilder<DownloadResult?>(
                future: _original,
                builder: (context, snap) => switch (snap.data) {
                  Downloaded(:final file) => InteractiveViewer(
                    child: Image.file(
                      File(file.path),
                      key: const ValueKey('file.viewer.image'),
                      fit: BoxFit.contain,
                      semanticLabel: dsIsolate(widget.file.originalName),
                    ),
                  ),
                  DownloadDamaged() => Text(l.fileDownloadDamaged),
                  DownloadNotReady() => Text(l.fileStateChecking),
                  DownloadBlocked() => Text(l.fileStateBlocked),
                  DownloadFailed(:final code) => Text(
                    l.fileDownloadFailed(uploadFailureText(l, code)),
                  ),
                  _ => const Center(child: CircularProgressIndicator()),
                },
              ),
            ),
            const SizedBox(height: DsSpace.s12),
            Wrap(
              alignment: WrapAlignment.end,
              spacing: DsSpace.s6,
              children: [
                DsButton(
                  key: const ValueKey('file.viewer.save'),
                  label: l.fileSaveAs,
                  style: DsButtonStyle.ghost,
                  onPressed: () async {
                    final downloads = ref.read(downloadServiceProvider);
                    final picker = ref.read(filePortsProvider).picker;
                    await downloads?.saveAs(widget.file, picker);
                  },
                ),
                DsButton(
                  key: const ValueKey('file.viewer.close'),
                  label: l.fileImageClose,
                  style: DsButtonStyle.secondary,
                  onPressed: () => Navigator.of(context).pop(),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
