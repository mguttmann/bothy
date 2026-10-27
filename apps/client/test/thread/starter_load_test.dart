// SPDX-License-Identifier: Apache-2.0

// The main history of a text channel loads the channel's newest threads once (15 11: a target
// is loaded locally or through REST). A device that signs in after the threads were created
// never receives their `thread.created`, so without the load the main history showed the
// starter messages without their thread lines and no thread could be opened from the channel
// (found on the desktop app against the demo server).

import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';

void main() {
  const mainN = 0x81;

  Thread mainThread() =>
      thread(mainN, lastSeq: 1).copyWith(kind: ThreadKind.main, title: '');

  testWidgets('the main history loads the channel threads once and shows the '
      'starter line', (tester) async {
    final host = await ColumnHost.open(tester);
    final starter = msg(1, threadN: mainN, text: 'Watch the nightly build');
    await host.run(
      tester,
      () => seedThread(
        host.db,
        ch: channel(1),
        th: mainThread(),
        messages: [starter],
      ),
    );
    final started = thread(
      2,
      title: 'Nightly build',
      lastSeq: 2,
    ).copyWith(parentMessageId: starter.id);
    final reads = FakeThreadReads(host.db)
      ..channelThreads[channelId(1)] = [started];
    await pumpThread(tester, host, reads, n: mainN);
    await host.settle(tester);

    expect(reads.calls.where((c) => c == 'threads'), hasLength(1));
    expect(
      find.byKey(ValueKey('history.starter.${started.id.value}')),
      findsOneWidget,
    );

    // A rebuild of the same channel does not read again.
    await host.run(
      tester,
      () => host.db.messagesDao.upsertIfNewer(
        msg(1, threadN: mainN, text: 'Watch the nightly build', version: 2),
      ),
    );
    await host.settle(tester);
    expect(reads.calls.where((c) => c == 'threads'), hasLength(1));
    await host.dispose(tester);
  });

  testWidgets('a thread history does not load the channel threads', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await host.run(tester, () => seedThread(host.db, messages: [msg(1)]));
    final reads = FakeThreadReads(host.db);
    await pumpThread(tester, host, reads);
    await host.settle(tester);

    expect(reads.calls, isNot(contains('threads')));
    await host.dispose(tester);
  });
}
