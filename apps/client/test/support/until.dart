// SPDX-License-Identifier: Apache-2.0

// Waiting for a condition in widget tests that mix fake time and real IO.

import 'package:flutter_test/flutter_test.dart';

/// Lets real IO run and pumps frames until [done] holds; fails naming [what] once
/// [patience] of wall-clock time has passed (the patience of the repository's test kits,
/// `testkit.Patience` of the Go services). Use it instead of a fixed number of settle rounds:
/// how many rounds real IO (a temporary folder, a file write) needs depends on the load of
/// the machine.
///
/// Example:
/// ```dart
/// await pumpUntil(tester, () => voice.phase == VoicePhase.recorded, what: 'the stop');
/// ```
Future<void> pumpUntil(
  WidgetTester tester,
  bool Function() done, {
  required String what,
  Duration patience = const Duration(minutes: 3),
}) async {
  // dart:core's Stopwatch runs on the real clock, also inside the fake zone of testWidgets.
  final watch = Stopwatch()..start();
  while (!done()) {
    if (watch.elapsed >= patience) {
      fail('$what did not happen within $patience');
    }
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 5)),
    );
    await tester.pump();
  }
}
