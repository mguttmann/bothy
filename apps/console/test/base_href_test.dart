// SPDX-License-Identifier: Apache-2.0

import 'package:console/run.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  // index.html has no <base> element: base-uri 'none' of the CSP would block it (brief M6-03,
  // docs/DECISIONS.md D-M6-03-2), so the path URLs take the console base from the app.
  test('the console location reports /console/ as its base', () {
    expect(ConsolePlatformLocation().getBaseHref(), '/console/');
  });
}
