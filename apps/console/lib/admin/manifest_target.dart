// SPDX-License-Identifier: Apache-2.0

/// The file pickers of the console: the browser's file input for the manifest of an app (brief
/// M6-08 Design note 1; 11 4.1: a file, never a URL) and for the logo of the instance (M6-09).
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

/// The file types of a logo (12 3.7: JPEG, PNG or WebP; the server's `415` decides).
const String logoAccept =
    '.png,.jpg,.jpeg,.webp,image/png,image/jpeg,image/webp';

/// The name of the logo input in the page (`data-bothy-file-input`).
const String logoInputTag = 'logo';

/// The [LogoPicker] of `admin_ui` over [files] (brief M6-09 Design note 2).
///
/// Example: `AdminBody(pickLogo: logoPickerOf(environment.files!), ...)`.
LogoPicker logoPickerOf(FileChooser files) => () async {
  final file = await files.choose(accept: logoAccept, tag: logoInputTag);
  return file == null ? null : PickedFile(file.name, file.bytes);
};
