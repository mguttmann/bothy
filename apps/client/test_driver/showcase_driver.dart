// SPDX-License-Identifier: Apache-2.0

/// The driver of `flutter drive` for the showcase (brief M7-28 Design note 1): runs
/// e2e_test/showcase/showcase_test.dart on the device and writes every image of its report data
/// as `<name>.png` into the folder `SHOWCASE_OUT` names (tools/showcase/capture.py strips and
/// files them). The exit code is the test's; a run without `SHOWCASE_OUT` fails before it
/// writes anything.
library;

import 'dart:convert';
import 'dart:io';

import 'package:integration_test/integration_test_driver.dart';

/// Writes the images of [data] (`images`: name to base64 PNG) into [out]; returns the paths.
///
/// Throws a [FormatException] for a name that is not lower case letters, digits and hyphens,
/// so a name can never leave [out].
List<String> writeImages(Map<String, dynamic>? data, Directory out) {
  final images = (data?['images'] as Map<String, dynamic>?) ?? const {};
  final written = <String>[];
  for (final MapEntry(:key, :value) in images.entries) {
    if (!RegExp(r'^[a-z0-9]+(-[a-z0-9]+)*$').hasMatch(key)) {
      throw FormatException('image name $key');
    }
    final file = File('${out.path}/$key.png')
      ..writeAsBytesSync(base64.decode(value as String));
    written.add(file.path);
  }
  return written;
}

/// Runs the showcase on the device and writes its images.
Future<void> main() {
  final out = Platform.environment['SHOWCASE_OUT'];
  if (out == null || !Directory(out).existsSync()) {
    stderr.writeln('SHOWCASE_OUT must name an existing folder');
    exit(2);
  }
  return integrationDriver(
    responseDataCallback: (data) async {
      for (final path in writeImages(data, Directory(out))) {
        stdout.writeln('showcase: wrote $path');
      }
    },
  );
}
