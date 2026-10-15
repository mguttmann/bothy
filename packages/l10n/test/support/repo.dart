// SPDX-License-Identifier: Apache-2.0

// Test helpers: the repository root and the files of this package.

import 'dart:convert';
import 'dart:io';

/// The repository root, found by walking up from the package directory to the first directory
/// that holds `docs/spec` (as packages/bothy_noise finds its vectors).
///
/// Throws a [StateError] when no directory above the current one has `docs/spec`.
Directory repoRoot() {
  var dir = Directory.current.absolute;
  while (true) {
    if (Directory('${dir.path}/docs/spec').existsSync()) {
      return dir;
    }
    final parent = dir.parent;
    if (parent.path == dir.path) {
      throw StateError(
        'no directory above ${Directory.current.path} contains docs/spec',
      );
    }
    dir = parent;
  }
}

/// The directory of this package (`packages/l10n`).
Directory packageDir() => Directory('${repoRoot().path}/packages/l10n');

/// Reads an ARB file as a JSON object.
Map<String, Object?> readArb(String path) =>
    (jsonDecode(File(path).readAsStringSync()) as Map).cast<String, Object?>();

/// The ARB file of [locale] in `lib/l10n`.
Map<String, Object?> realArb(String locale) =>
    readArb('${packageDir().path}/lib/l10n/app_$locale.arb');

/// A fixture ARB file below `test/fixtures`.
Map<String, Object?> fixtureArb(String name) =>
    readArb('${packageDir().path}/test/fixtures/$name');
