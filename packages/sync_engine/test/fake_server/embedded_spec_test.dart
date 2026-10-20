// SPDX-License-Identifier: Apache-2.0

// The example of 07 9.3 comes from docs/spec/07-permissions.md, and without the repository (an
// app under test in the macOS app sandbox) from the embedded copy (docs/DECISIONS.md D-M4-27-7).

import 'dart:io';

import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

void main() {
  test('a folder without the repository reads the example from the copy', () {
    final outside = Directory.systemTemp.createTempSync('no-repo');
    addTearDown(() => outside.deleteSync(recursive: true));
    expect(grantPreviewExample(from: outside), grantPreviewExample());
    expect(grantPreviewExample(), isNotEmpty);
  });
}
