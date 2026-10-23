// SPDX-License-Identifier: Apache-2.0

// `pumpUntil` waits for a condition that real IO fulfils, however slow the machine is: a
// fixed number of settle rounds (10 times 5 ms) let the voice limit test flake on loaded CI
// runners while the fake recorder still wrote its file.

import 'dart:async';

import 'package:flutter_test/flutter_test.dart';

import '../support/until.dart';

void main() {
  testWidgets('pumpUntil waits for real IO that outlasts many settle rounds', (
    tester,
  ) async {
    var rounds = 0;
    var done = false;
    // Completes in the real zone only after 40 rounds of the helper, four times the rounds
    // of the old fixed settle; the condition, not a round count or a time, ends the wait.
    Future<void> slowIo() async {
      while (rounds < 40) {
        await Future<void>.delayed(Duration.zero);
      }
      done = true;
    }

    await tester.runAsync(() async => unawaited(slowIo()));
    await pumpUntil(tester, () {
      rounds++;
      return done;
    }, what: 'the slow IO');
    expect(done, isTrue);
    expect(rounds, greaterThan(40));
  });

  testWidgets('pumpUntil names what did not happen after its patience', (
    tester,
  ) async {
    await expectLater(
      pumpUntil(tester, () => false, what: 'nothing', patience: Duration.zero),
      throwsA(
        isA<TestFailure>().having(
          (e) => e.message,
          'message',
          contains('nothing did not happen'),
        ),
      ),
    );
  });
}
