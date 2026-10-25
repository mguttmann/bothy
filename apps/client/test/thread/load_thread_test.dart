// SPDX-License-Identifier: Apache-2.0

// A thread this device never stored is loaded through REST when its history opens (15 11: a
// target is loaded locally or through REST). A device that signs in after a channel was created
// never receives the `thread.created` of its main thread, so without the load the text channel
// stayed empty (found by the desktop E2E suite of M4-34 against the real server).

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';

void main() {
  testWidgets('a thread missing locally is loaded once and then shown', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    final th = thread(1, title: 'Printer jam', lastSeq: 2);
    await host.run(
      tester,
      () => seedThread(
        host.db,
        th: th,
        messages: [msg(1), msg(2)],
        storeThread: false,
      ),
    );
    final reads = FakeThreadReads(host.db)..threads[th.id] = th;
    await pumpThread(tester, host, reads);
    await host.settle(tester);

    expect(reads.calls.where((c) => c == 'thread'), hasLength(1));
    expect(find.byKey(ValueKey('history.${th.id.value}')), findsOneWidget);
    expect(
      find.byKey(ValueKey('message.${messageId(2).value}')),
      findsOneWidget,
    );
    await host.dispose(tester);
  });

  testWidgets('a refused load leaves the column empty and asks once', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await host.run(
      tester,
      () => seedThread(host.db, messages: [msg(1)], storeThread: false),
    );
    final reads = FakeThreadReads(host.db);
    await pumpThread(tester, host, reads);
    await host.settle(tester);

    expect(reads.calls.where((c) => c == 'thread'), hasLength(1));
    expect(find.byKey(ValueKey('history.${threadId(1).value}')), findsNothing);
    await host.dispose(tester);
  });
}
