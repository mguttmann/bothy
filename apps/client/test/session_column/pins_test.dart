// SPDX-License-Identifier: Apache-2.0

// The section "Pinned" of the session column (brief M4-20 Scope; 01 5.1 D1): it lists the
// pinned messages of the thread, also those outside the loaded history (`listPins`).

import 'package:client/session_column/session_column.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';

void main() {
  testWidgets('pins outside the loaded history are listed', (tester) async {
    final host = await ColumnHost.open(tester);
    await host.run(
      tester,
      () => seedThread(
        host.db,
        messages: [
          msg(90, text: 'Loaded and pinned', pinned: true),
          msg(91),
        ],
      ),
    );
    final reads = FakeThreadReads(host.db)
      ..pins.add(msg(3, text: 'Old but pinned', pinned: true));
    await host.pump(
      tester,
      SessionColumn(threadId: threadId(1)),
      overrides: threadOverrides(reads),
    );
    expect(reads.calls, contains('pins'));
    expect(
      find.byKey(ValueKey('session.pin.${messageId(3).value}')),
      findsOneWidget,
    );
    expect(
      find.byKey(ValueKey('session.pin.${messageId(90).value}')),
      findsOneWidget,
    );
    await host.dispose(tester);
  });
}
