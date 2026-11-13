// SPDX-License-Identifier: Apache-2.0

/// How a composer gets files (brief M4-22 Scope, M5-07, M5-17), shared by the composer of a
/// thread and the new chat view of a forum (FEAT-NEW-CHAT-ATTACH, ruling FNCA/5), so both offer
/// the same paths on every platform: the system's file dialog behind the attachment button, the
/// drop area and Cmd+V or Ctrl+V of files and images on the desktop, and on the phone the
/// attachment menu with the camera, the photo library and a file.
library;

import 'dart:io' show Directory, Platform;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../files/attach_controller.dart';
import '../files/providers.dart';
import '../phone/compose/media_picker.dart';
import '../settings/settings_providers.dart';

/// The items of the phone's attachment menu.
enum AttachMenuItem {
  /// A new photo of the camera.
  camera,

  /// Photos of the library.
  library,

  /// A file of the system's file picker.
  file,

  /// The templates of the thread's composer (brief M5-17).
  templates,
}

/// Opens the system's file dialog and attaches the chosen files to [attach].
Future<void> pickComposerFiles(WidgetRef ref, AttachController attach) async {
  final files = await ref.read(filePortsProvider).picker.pickFiles();
  await attach.add(files);
}

/// The attachment menu of the phone (M5-07): camera, photo library or a file, and the templates
/// with [templates] (the thread's composer has no button of its own for them beside the phone's
/// field, brief M5-17); returns the chosen item, or null when the person closed the menu.
Future<AttachMenuItem?> showAttachMenu(
  BuildContext context, {
  required bool templates,
}) {
  final l = AppLocalizations.of(context);
  return showModalBottomSheet<AttachMenuItem>(
    context: context,
    useSafeArea: true,
    builder: (context) => SafeArea(
      child: Column(
        key: const ValueKey('composer.attachMenu'),
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          for (final (item, label) in [
            (AttachMenuItem.camera, l.composerAttachCamera),
            (AttachMenuItem.library, l.composerAttachLibrary),
            (AttachMenuItem.file, l.composerAttachFile),
            if (templates) (AttachMenuItem.templates, l.composerTemplates),
          ])
            DsTappable(
              key: ValueKey('composer.attach.${item.name}'),
              label: label,
              excludeChildSemantics: true,
              onTap: () => Navigator.of(context).pop(item),
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: DsSpace.s16,
                  vertical: DsSpace.s12,
                ),
                child: Text(
                  label,
                  style: dsTextStyle(DsType.body)
                      .copyWith(color: context.dsColors.textPrimary),
                ),
              ),
            ),
        ],
      ),
    ),
  );
}

/// The picker of the camera and the photo library of a composer, writing into the app's
/// temporary folder (D-M5-07-5).
MediaPicker composerMediaPicker(WidgetRef ref) {
  final ports = ref.read(filePortsProvider);
  return MediaPicker(
    port: ports.media(),
    scratch: () async => Directory(
      '${(await ports.temporary()).path}${Platform.pathSeparator}photos',
    ),
  );
}

/// The notice a photo [outcome] leaves (a denied camera or library, a format that cannot be
/// read, a failure), or null for a picked, cancelled or refused one (the refusal shows as the
/// attachments' own).
PhotoOutcome? photoNoticeOf(PhotoOutcome outcome) =>
    outcome is PhotoPicked ||
        outcome is PhotoCancelled ||
        outcome is PhotoNoRoom
    ? null
    : outcome;

/// The text of the photo notice [outcome] (01 5.3; M5-07), or null without one; [android]
/// names the way to the camera permission of that system.
String? photoNoticeText(
  AppLocalizations l,
  PhotoOutcome? outcome, {
  required bool android,
}) => switch (outcome) {
  null || PhotoPicked() || PhotoCancelled() || PhotoNoRoom() => null,
  PhotoDenied(source: MediaSource.camera) =>
    android ? l.photoCameraDeniedAndroid : l.photoCameraDeniedIos,
  PhotoDenied(source: MediaSource.library) => l.photoLibraryDenied,
  PhotoUnsupported(:final name) => l.photoUnsupported(dsIsolate(name)),
  PhotoFailed() => l.photoFailed,
};

/// [child] with the drop area and the paste key of [attach] (brief M4-22 Scope): files dropped
/// on it and files or an image pasted with Cmd+V or Ctrl+V go to [attach]; [child] itself when
/// there is no [attach].
Widget withComposerFiles(
  WidgetRef ref,
  AttachController? attach,
  Widget child,
) {
  if (attach == null) return child;
  return AttachDropTarget(
    controller: attach,
    child: AttachPasteShortcut(
      controller: attach,
      paste: ref.read(filePortsProvider).paste,
      scratch: () async => ref.read(outgoingFolderProvider),
      originals: () => ref.read(personalSettingsProvider).pasteOriginals,
      child: child,
    ),
  );
}
