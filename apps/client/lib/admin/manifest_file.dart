// SPDX-License-Identifier: Apache-2.0

/// The file pickers of the administration on the desktop: the system file dialog for the
/// manifest of an app (brief M6-08 Design note 1; 11 4.1: the owner uploads a file, never a URL)
/// and for the logo of the instance (M6-09). The sandbox of the macOS app allows
/// reading exactly the file the person chose (`files.user-selected.read-only`).
library;

import 'package:admin_ui/admin_ui.dart';
import 'package:file_selector/file_selector.dart';

/// Opens a file dialog for [acceptedTypeGroups] and returns the chosen file, or null; the
/// signature of `openFile` of `file_selector`.
typedef OpenFile = Future<XFile?> Function({
  List<XTypeGroup> acceptedTypeGroups,
});

/// The [ManifestPicker] over the system dialog [open] (default `openFile`), offering the types of
/// 11 3.1 (JSON, YAML) under the name [label].
///
/// Example: `AdminShell(pickManifest: manifestPicker(label: l.adminManifestFileType), ...)`.
ManifestPicker manifestPicker({required String label, OpenFile? open}) {
  final dialog = open ?? _openFile;
  return () async {
    final file = await dialog(
      acceptedTypeGroups: [
        XTypeGroup(
          label: label,
          extensions: const ['json', 'yaml', 'yml'],
          uniformTypeIdentifiers: const ['public.json', 'public.yaml'],
          mimeTypes: const ['application/json', 'application/yaml'],
        ),
      ],
    );
    if (file == null) return null;
    return PickedFile(file.name, await file.readAsBytes());
  };
}

Future<XFile?> _openFile({List<XTypeGroup> acceptedTypeGroups = const []}) =>
    openFile(acceptedTypeGroups: acceptedTypeGroups);

/// The [LogoPicker] of the instance logo over the system dialog [open] (default `openFile`),
/// offering the image types the server takes (12 3.7: PNG, JPEG, WebP) under the name [label]
/// (brief M6-09 Design note 2).
///
/// Example: `AdminShell(pickLogo: logoPicker(label: l.adminLogoFileType), ...)`.
LogoPicker logoPicker({required String label, OpenFile? open}) {
  final dialog = open ?? _openFile;
  return () async {
    final file = await dialog(
      acceptedTypeGroups: [
        XTypeGroup(
          label: label,
          extensions: const ['png', 'jpg', 'jpeg', 'webp'],
          uniformTypeIdentifiers: const [
            'public.png',
            'public.jpeg',
            'org.webmproject.webp',
          ],
          mimeTypes: const ['image/png', 'image/jpeg', 'image/webp'],
        ),
      ],
    );
    if (file == null) return null;
    return PickedFile(file.name, await file.readAsBytes());
  };
}
