// SPDX-License-Identifier: Apache-2.0

/// Reads files of the repository from the tests of the telemetry (the catalogue and the
/// contract are the sources the app's lists are held to).
library;

import 'dart:io';

/// The repository root: the first folder above the working directory with
/// `docs/observability/signals.md`.
Directory repoRoot() {
  var dir = Directory.current.absolute;
  while (!File('${dir.path}/docs/observability/signals.md').existsSync()) {
    final parent = dir.parent;
    if (parent.path == dir.path) {
      throw StateError(
        'docs/observability/signals.md not found above the tests',
      );
    }
    dir = parent;
  }
  return dir;
}

/// The text of [path] relative to the repository root.
String readRepoFile(String path) =>
    File('${repoRoot().path}/$path').readAsStringSync();
