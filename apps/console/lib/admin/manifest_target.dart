// SPDX-License-Identifier: Apache-2.0

/// The manifest picker of the apps in the console: the browser's file input (brief M6-08 Design
/// note 1; 11 4.1: a file, never a URL).
library;

import 'package:admin_ui/admin_ui.dart';

import '../platform/browser.dart';

/// The file types of a manifest (11 3.1: JSON, YAML as an authoring format).
const String manifestAccept = '.json,.yaml,.yml,application/json';

/// The name of the input in the page (`data-bothy-file-input`).
const String manifestInputTag = 'manifest';

/// The [ManifestPicker] of `admin_ui` over [files].
///
/// Example: `AdminBody(pickManifest: manifestPickerOf(environment.files!), ...)`.
ManifestPicker manifestPickerOf(FileChooser files) => () async {
  final file = await files.choose(
    accept: manifestAccept,
    tag: manifestInputTag,
  );
  return file == null ? null : PickedFile(file.name, file.bytes);
};
