// SPDX-License-Identifier: Apache-2.0

/// Opens a text attachment in the in-app document viewer (ticket FEAT-markdown-viewer): the
/// checked bytes from the server through the cache (12 9; SEC-OUT-2), the [DocumentView] over
/// them, and the actions Save as and Open in another app of the attachment tile. Only a file
/// whose state is servable opens; a blocked file or one still being checked never does.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import '../../app/providers/ports.dart';
import '../attachment_mapping.dart';
import '../download_service.dart';
import '../image_preview.dart';
import '../providers.dart';
import 'document_content.dart';
import 'document_view.dart';

/// Whether the viewer opens [file]: a text kind of [documentKindOf] in the state `ready`.
bool opensInDocumentViewer(FileMeta file) =>
    documentKindOf(file) != null && stateOfFile(file) == AttachmentState.ready;

/// Opens the viewer of [file]; [siblings] are the files of the same message (the targets of
/// relative links and images). Full screen on the phone, a large dialog on the desktop.
Future<void> showDocumentViewer(
  BuildContext context,
  WidgetRef ref,
  FileMeta file, {
  List<FileMeta> siblings = const [],
}) {
  final phone = ref.read(appPortsProvider).formFactor.isPhone;
  return showDialog<void>(
    context: context,
    useSafeArea: !phone,
    builder: (dialog) {
      final viewer = DocumentViewer(
        file: file,
        siblings: siblings,
        desktop: !phone,
        onClose: () => Navigator.of(dialog).pop(),
        onOpenLink: (uri) {
          // A link leaves the viewer, so it closes first; internal paths open in the app.
          Navigator.of(dialog).pop();
          if (uri.scheme.isEmpty) {
            if (context.mounted) context.go(uri.toString());
          } else {
            unawaited(ref.read(appPortsProvider).browser.open(uri));
          }
        },
      );
      return phone
          ? Dialog.fullscreen(child: viewer)
          : Dialog(
              insetPadding: const EdgeInsets.all(DsSpace.s32),
              clipBehavior: Clip.antiAlias,
              child: SizedBox.expand(child: viewer),
            );
    },
  );
}

/// The viewer of [file]: loads its bytes, then shows the [DocumentView].
class DocumentViewer extends ConsumerStatefulWidget {
  /// Creates the viewer.
  const DocumentViewer({
    super.key,
    required this.file,
    required this.onClose,
    required this.onOpenLink,
    this.siblings = const [],
    this.desktop = true,
  });

  /// The file.
  final FileMeta file;

  /// The files of the same message.
  final List<FileMeta> siblings;

  /// Closes the viewer.
  final VoidCallback onClose;

  /// Opens a link of the document outside the viewer.
  final void Function(Uri) onOpenLink;

  /// Whether the desktop mouse selection is used.
  final bool desktop;

  @override
  ConsumerState<DocumentViewer> createState() => _DocumentViewerState();
}

/// The outcome of loading: the content, or the text of the failure.
typedef _Loaded = ({DocumentContent? content, String? failure});

class _DocumentViewerState extends ConsumerState<DocumentViewer> {
  late final Future<_Loaded> _loaded = _load();

  Future<_Loaded> _load() async {
    final kind = documentKindOf(widget.file);
    final downloads = ref.read(downloadServiceProvider);
    if (kind == null || downloads == null) {
      return (content: null, failure: null);
    }
    final result = await downloads.fetch(
      widget.file.id,
      name: widget.file.originalName,
    );
    if (!mounted) return (content: null, failure: null);
    final l = AppLocalizations.of(context);
    if (result case Downloaded(:final file)) {
      try {
        return (content: await readDocument(file, kind), failure: null);
      } on Object {
        return (
          content: null,
          failure: l.fileDownloadFailed(
            uploadFailureText(l, 'file_unreadable'),
          ),
        );
      }
    }
    return (content: null, failure: _failureText(l, result));
  }

  String? _failureText(AppLocalizations l, DownloadResult result) =>
      switch (result) {
        Downloaded() || DownloadCancelled() => null,
        DownloadDamaged() => l.fileDownloadDamaged,
        DownloadNotReady() => l.fileStateChecking,
        DownloadBlocked() => l.fileStateBlocked,
        DownloadFailed(:final code, :final reason) => l.fileDownloadFailed(
          uploadFailureText(l, code, reason: reason),
        ),
      };

  Future<String?> _save() async {
    final l = AppLocalizations.of(context);
    final downloads = ref.read(downloadServiceProvider);
    if (downloads == null) return null;
    final result = await downloads.saveAs(
      widget.file,
      ref.read(filePortsProvider).picker,
    );
    if (!mounted) return null;
    return result is Downloaded
        ? l.fileSaved(dsIsolate(widget.file.originalName))
        : _failureText(l, result);
  }

  Future<String?> _openExternally(FileMeta file) async {
    final l = AppLocalizations.of(context);
    final downloads = ref.read(downloadServiceProvider);
    if (downloads == null) return null;
    final result = await downloads.open(
      file,
      ref.read(filePortsProvider).opener,
    );
    if (!mounted) return null;
    return _failureText(l, result);
  }

  /// The attachment of the message that a relative reference [href] names (by its file name,
  /// or `attachment:<id>`), or null.
  FileMeta? _sibling(String href) {
    final uri = Uri.tryParse(href.trim());
    if (uri == null) return null;
    final id = attachmentReference(uri);
    if (id != null) {
      return widget.siblings
          .where((f) => f.id == id && stateOfFile(f) == AttachmentState.ready)
          .firstOrNull;
    }
    if (uri.hasScheme || uri.hasAuthority || uri.path.isEmpty) return null;
    final segments = uri.pathSegments.where((s) => s.isNotEmpty);
    if (segments.isEmpty) return null;
    final name = segments.last.toLowerCase();
    return widget.siblings
        .where(
          (f) =>
              f.id != widget.file.id &&
              stateOfFile(f) == AttachmentState.ready &&
              f.originalName.toLowerCase() == name,
        )
        .firstOrNull;
  }

  DsLinkTarget? _resolveLink(String href) {
    final file = _sibling(href);
    if (file == null) return null;
    return DsLinkTarget(
      Uri(scheme: attachmentScheme, path: file.id.value),
      file.originalName,
    );
  }

  void _openLink(Uri uri) {
    final id = attachmentReference(uri);
    if (id == null) {
      widget.onOpenLink(uri);
      return;
    }
    final file = widget.siblings.where((f) => f.id == id).firstOrNull;
    if (file == null) return;
    if (opensInDocumentViewer(file)) {
      // Another document of the message opens over this one; Esc comes back.
      unawaited(
        showDocumentViewer(context, ref, file, siblings: widget.siblings),
      );
    } else {
      unawaited(_openExternally(file));
    }
  }

  /// The images of the document: another attachment of the message (by name or
  /// `attachment:<id>`) shows its thumbnail from the server, a web image loads only on a click,
  /// everything else stays text.
  Widget? _image(Uri uri) {
    final file = _sibling(uri.toString());
    if (file != null && isImage(file)) {
      return ImagePreview(
        key: ValueKey('document.image.${file.id.value}'),
        file: file,
      );
    }
    if ((uri.scheme == 'https' || uri.scheme == 'http') &&
        uri.host.isNotEmpty) {
      return RemoteImageOnClick(uri: uri);
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return FutureBuilder<_Loaded>(
      future: _loaded,
      builder: (context, snap) {
        final content = snap.data?.content;
        if (content != null) {
          return DocumentView(
            name: widget.file.originalName,
            content: content,
            desktop: widget.desktop,
            onClose: widget.onClose,
            onOpenLink: _openLink,
            resolveLink: _resolveLink,
            images: _image,
            onSave: _save,
            onOpenExternally: () => _openExternally(widget.file),
          );
        }
        final failure = snap.data?.failure;
        return Semantics(
          key: const ValueKey('document.viewer'),
          scopesRoute: true,
          namesRoute: true,
          explicitChildNodes: true,
          label: l.documentViewerTitle(dsIsolate(widget.file.originalName)),
          child: Material(
            color: colors.surfaceCanvas,
            child: Padding(
              padding: const EdgeInsets.all(DsSpace.s16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          dsIsolate(widget.file.originalName),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: dsTextStyle(DsType.heading)
                              .copyWith(color: colors.textPrimary),
                        ),
                      ),
                      DsButton(
                        key: const ValueKey('document.close'),
                        label: l.documentClose,
                        style: DsButtonStyle.ghost,
                        onPressed: widget.onClose,
                      ),
                    ],
                  ),
                  const SizedBox(height: DsSpace.s16),
                  if (snap.connectionState != ConnectionState.done)
                    Text(
                      l.documentLoading,
                      key: const ValueKey('document.loading'),
                      style: dsTextStyle(DsType.body)
                          .copyWith(color: colors.textSecondary),
                    )
                  else if (failure != null)
                    Text(
                      failure,
                      key: const ValueKey('document.failure'),
                      style: dsTextStyle(DsType.body)
                          .copyWith(color: colors.textPrimary),
                    ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}

/// An image of a document from the web: a button names its domain, and only a click loads it
/// (no remote content without a click).
class RemoteImageOnClick extends StatefulWidget {
  /// Creates the placeholder of [uri].
  const RemoteImageOnClick({super.key, required this.uri});

  /// The image address (`https` or `http` with a host).
  final Uri uri;

  @override
  State<RemoteImageOnClick> createState() => _RemoteImageOnClickState();
}

class _RemoteImageOnClickState extends State<RemoteImageOnClick> {
  bool _load = false;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    if (!_load) {
      return Align(
        alignment: AlignmentDirectional.centerStart,
        child: DsButton(
          key: const ValueKey('document.remoteImage'),
          label: l.documentRemoteImage(widget.uri.host),
          style: DsButtonStyle.secondary,
          onPressed: () => setState(() => _load = true),
        ),
      );
    }
    return Image.network(
      widget.uri.toString(),
      key: const ValueKey('document.remoteImage.loaded'),
      fit: BoxFit.contain,
      errorBuilder: (_, _, _) => Text(
        l.fileDownloadFailed(widget.uri.host),
        style: dsTextStyle(DsType.caption)
            .copyWith(color: context.dsColors.textSecondary),
      ),
    );
  }
}
