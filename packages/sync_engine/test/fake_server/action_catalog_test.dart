// SPDX-License-Identifier: Apache-2.0

// The action catalogue of the fake reads docs/spec/actions.yaml, and without the repository
// (an app under test in the macOS app sandbox) its embedded copy (docs/DECISIONS.md D-M4-26-9).

import 'dart:io';

import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

void main() {
  test('the embedded copy gives every action the step-up of the file', () {
    final file = ActionCatalog.load();
    final copy = ActionCatalog.embedded();
    for (final action in ['user.disable', 'device.approve', 'group.create']) {
      expect(copy.stepUp(action), file.stepUp(action), reason: action);
    }
  });

  test('a folder without the repository falls back to the copy', () {
    final outside = Directory.systemTemp.createTempSync('no-repo');
    addTearDown(() => outside.deleteSync(recursive: true));
    expect(ActionCatalog.read(from: outside).stepUp('user.disable'), 'true');
  });
}
