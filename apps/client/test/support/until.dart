// SPDX-License-Identifier: Apache-2.0

// Waiting for a condition in widget tests that mix fake time and real IO.

import 'package:flutter_test/flutter_test.dart';

/// The bound of every wait of the tests for something that must happen: the patience of the
/// repository's test kits (`testkit.Patience` of the Go services). A wait ends as soon as its
/// condition holds; the bound only turns a hang into a failure, so a slow or loaded machine
/// never fails a correct test (`patience_test.dart` holds the helpers to it).
const Duration testPatience = Duration(minutes: 3);

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
  Duration patience = testPatience,
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

/// Like [pumpUntil] for plain `test`s without a widget tester: lets real IO run until [done]
/// holds and fails naming [what] once [patience] of wall-clock time has passed. Use it instead
/// of `pumpEventQueue`, whose fixed number of rounds real IO may outlast on a loaded machine.
Future<void> waitUntil(
  bool Function() done, {
  required String what,
  Duration patience = const Duration(minutes: 3),
}) async {
  final watch = Stopwatch()..start();
  while (!done()) {
    if (watch.elapsed >= patience) {
      fail('$what did not happen within $patience');
    }
    await Future<void>.delayed(const Duration(milliseconds: 5));
  }
}
