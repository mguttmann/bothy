// SPDX-License-Identifier: Apache-2.0

// The import is the subject of the test: the library compiles without Flutter (15 3.1 rule 1).
// ignore: unused_import
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

void main() {
  test('the library is a package library of the workspace', () {
    expect(Uri.parse('package:sync_engine/sync_engine.dart').pathSegments, [
      'sync_engine',
      'sync_engine.dart',
    ]);
  });
}
