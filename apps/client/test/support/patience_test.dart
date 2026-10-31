// SPDX-License-Identifier: Apache-2.0

// The waits of the shared test helpers are bounded by one patience (the repository's rule for
// deterministic tests; red team of M5-06): a wait for something that must happen ends as soon as
// it happens, and its bound only turns a hang into a failure, so it is never a fixed number of
// seconds that a slow or loaded machine can overrun. This test holds the helpers to
// [testPatience]: a bound written as a fixed duration in one of them fails here.

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import 'until.dart';

/// The files of the wait helpers (relative to `apps/client`) and the device test helpers of
/// sharing.
const List<String> _helpers = [
  'test/support/until.dart',
  'test/support/column_host.dart',
  'test/support/ready.dart',
  'test/support/fake_instance.dart',
  'integration_test/support/test_app.dart',
  'integration_test/phone/support/phone_flow.dart',
  'integration_test/share/support/share_flow.dart',
];

/// A bound written as a fixed duration: a default `timeout`, an end computed from `now()`, or a
/// `nativeWait`-style constant, in seconds or minutes.
final RegExp _fixedBound = RegExp(
  r'(timeout\s*=\s*const\s+Duration\((seconds|minutes):)'
  r'|(DateTime\.now\(\)\.add\(\s*const\s+Duration\((seconds|minutes):)'
  r'|(Duration\s+\w*[wW]ait\w*\s*=\s*(const\s+)?Duration\((seconds|minutes):)',
);

void main() {
  test('the patience is the one of the repository test kits', () {
    expect(testPatience, const Duration(minutes: 3));
  });

  test('no wait helper bounds a wait by a fixed number of seconds', () {
    final offenders = <String>[];
    for (final path in _helpers) {
      final lines = File(path).readAsLinesSync();
      for (var i = 0; i < lines.length; i++) {
        if (_fixedBound.hasMatch(lines[i])) offenders.add('$path:${i + 1}');
      }
    }
    expect(offenders, isEmpty);
  });
}
