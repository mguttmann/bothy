// SPDX-License-Identifier: Apache-2.0

// The import is the subject of the test: the library compiles without Flutter (15 3.1 rule 1).
// ignore: unused_import
import 'package:api_client/api_client.dart';
import 'package:test/test.dart';

void main() {
  test('the library is a package library of the workspace', () {
    expect(Uri.parse('package:api_client/api_client.dart').pathSegments, ['api_client', 'api_client.dart']);
  });
}
