// SPDX-License-Identifier: Apache-2.0

/// How a host hands a manifest file to the install and update flows (11 4.1: the owner uploads
/// a file, never a URL; brief M6-08 Design note 1).
library;

import 'dart:typed_data';

/// A file the person picked: its name and its bytes.
final class PickedFile {
  /// Creates the file.
  const PickedFile(this.name, this.bytes);

  /// The file name, for example `repo-bridge.yaml`; the flows read a `.yaml` or `.yml` name as
  /// YAML.
  final String name;

  /// The content.
  final Uint8List bytes;
}

/// Opens the host's file dialog for a manifest (`.json`, `.yaml`, `.yml`) and returns the
/// chosen file, or null when the person cancels. The desktop app opens the system's file
/// dialog, the console a file input of the browser.
typedef ManifestPicker = Future<PickedFile?> Function();
