// SPDX-License-Identifier: Apache-2.0

// `ColumnHost.outboxUntil` waits for outbox entries that a send writes after real IO, however
// slow the machine is: the sends of a long draft, a long paste and a voice message write a file
// before their message, and a fixed number of settle rounds after the send let those tests
// flake on loaded CI runners (issue #238, FLAKES-DART).

import 'dart:async';

import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';

void main() {
  testWidgets(
    'outboxUntil waits for a write that outlasts many settle rounds',
    (tester) async {
      final host = await ColumnHost.open(tester);
      var written = false;
      // Real IO in the real zone that takes several times the 20 rounds of one `settle` (about
      // 100 ms); then the entry is written. The condition, not a round count, ends the wait.
      Future<void> slowSend() async {
        await Future<void>.delayed(const Duration(milliseconds: 500));
        written = true;
        await host.db.outboxDao.enqueue(
          id: '0192f4a0-7b1c-7000-8000-000000000001',
          kind: 'createMessage',
          payload: '{}',
        );
      }

      await tester.runAsync(() async => unawaited(slowSend()));
      final all = await host.outboxUntil(
        tester,
        (all) => all.isNotEmpty,
        what: 'the slow send',
      );
      expect(all.single.kind, 'createMessage');
      expect(written, isTrue);
      await host.dispose(tester);
    },
  );

  testWidgets('outboxUntil names what did not happen after its patience', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    // Awaited before the check: the wait pumps, and a pending pump guards the tester.
    Object? failure;
    try {
      await host.outboxUntil(
        tester,
        (all) => all.isNotEmpty,
        what: 'nothing',
        patience: Duration.zero,
      );
    } on TestFailure catch (e) {
      failure = e;
    }
    expect(
      failure,
      isA<TestFailure>().having(
        (e) => e.message,
        'message',
        contains('nothing did not happen'),
      ),
    );
    await host.dispose(tester);
  });
}
