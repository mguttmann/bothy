// SPDX-License-Identifier: Apache-2.0

// The thread header on the phone has one overflow button (FIX-draft-conflict-banner; 01 5.3
// M3): the actions of the thread and the menu of M4-33 (notifications, mute) sit in the same
// menu `thread.more`, not in a second "More" button beside it.

import 'package:client/thread/history_column.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';

void main() {
  testWidgets('one overflow button holds the actions and the level menu', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await host.run(
      tester,
      () => seedThread(
        host.db,
        messages: [msg(1, author: alex, text: 'Hi')],
      ),
    );
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      platform: 'ios',
      overrides: threadOverrides(FakeThreadReads(host.db)),
    );
    await host.settle(tester);
    expect(find.byKey(const ValueKey('thread.more')), findsOneWidget);
    expect(find.byKey(const ValueKey('thread.menu')), findsNothing);

    await tester.tap(find.byKey(const ValueKey('thread.more')));
    await host.settle(tester);
    expect(find.byKey(const ValueKey('thread.session')), findsOneWidget);
    expect(find.byKey(const ValueKey('menu.notifications')), findsOneWidget);
    expect(find.byKey(const ValueKey('menu.muteUntil')), findsOneWidget);
    await host.dispose(tester);
  });
}
