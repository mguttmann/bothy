// SPDX-License-Identifier: Apache-2.0

// "Retry" on the line of a failed or cancelled run (09 5.7, 01 6.2, addendum of 2026-10-30): on
// the desktop apps (macOS, Windows) and the mobile apps (iOS, Android) the newest failed or
// cancelled run of a thread offers Retry, which puts one retryRun into the outbox; a double tap
// sends once; a completed run, an older run and an interrupted run (which keeps "Send again") do
// not offer it; a refusal of the server shows its reason.

import 'package:client/agents/providers.dart';
import 'package:client/sync/message_ops_app.dart';
import 'package:client/thread/history_column.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/agent_fakes.dart';
import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

/// The triggering message of Alex and the anchors of runs 1 and 2.
List<Message> retryMessages() => [
  msg(
    1,
    author: alex,
    text: '@robin run the tests',
  ).copyWith(triggerState: TriggerState.failed),
  msg(
    2,
    author: robin,
    text: '',
    kind: MessageKind.run,
    origin: MessageOrigin.agent,
    minutes: 1,
  ).copyWith(runId: runId(1)),
  msg(
    3,
    author: robin,
    text: '',
    kind: MessageKind.run,
    origin: MessageOrigin.agent,
    minutes: 2,
  ).copyWith(runId: runId(2)),
];

/// Opens the thread with [runs] stored on [platform].
Future<ColumnHost> openThread(
  WidgetTester tester,
  List<RunSummary> runs, {
  String platform = 'macos',
}) async {
  final host = await ColumnHost.open(tester);
  await host.run(tester, () async {
    await seedThread(host.db, messages: retryMessages());
    for (final r in runs) {
      await host.db.runsDao.upsertIfNewer(r);
    }
  });
  await host.pump(
    tester,
    HistoryColumn(threadId: threadId(1)),
    location: '/t/${threadId(1).value}',
    platform: platform,
    overrides: [
      ...threadOverrides(FakeThreadReads(host.db)),
      runSourceProvider.overrideWithValue(StoreRunSource()),
      agentReadsProvider.overrideWithValue(FakeAgentReads()),
    ],
  );
  return host;
}

Future<List<OutboxEntry>> retries(WidgetTester tester, ColumnHost host) async {
  final all = await host.run(tester, () => host.db.outboxDao.allEntries());
  return [
    for (final e in all)
      if (e.kind == AppMessageOps.retryRun) e,
  ];
}

void main() {
  for (final platform in ['macos', 'windows', 'ios', 'android']) {
    testWidgets('$platform: Retry on a failed run puts one retryRun into the '
        'outbox', (tester) async {
      final host = await openThread(tester, [
        run(1, RunState.failed, reason: 'adapter_error'),
      ], platform: platform);
      final button = find.text(en.liveBoxRetry);
      expect(button, findsOneWidget);
      await tester.tap(button);
      await tester.tap(button, warnIfMissed: false);
      await host.settle(tester);
      final sent = await retries(tester, host);
      expect(sent, hasLength(1));
      expect(sent.single.payload, contains(runId(1).value));
      expect(sent.single.threadId, isNull);
      await host.dispose(tester);
    });
  }

  testWidgets('a cancelled run offers Retry too', (tester) async {
    final host = await openThread(tester, [
      run(1, RunState.cancelled, reason: 'user_cancelled'),
    ]);
    expect(find.text(en.liveBoxRetry), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('no Retry on a completed run, an older run or an interrupted '
      'run', (tester) async {
    final host = await openThread(tester, [
      run(1, RunState.failed, reason: 'adapter_error'),
      run(2, RunState.completed),
    ]);
    expect(find.text(en.liveBoxRetry), findsNothing);
    await host.dispose(tester);

    final interrupted = await openThread(tester, [
      run(1, RunState.failed, reason: 'interrupted'),
    ]);
    expect(find.text(en.liveBoxRetry), findsNothing);
    expect(find.text(en.liveBoxSendAgain), findsOneWidget);
    await interrupted.dispose(tester);
  });

  testWidgets('a refused retry shows the reason and offers Retry again', (
    tester,
  ) async {
    final host = await openThread(tester, [
      run(1, RunState.failed, reason: 'adapter_error'),
    ]);
    await tester.tap(find.text(en.liveBoxRetry));
    await host.settle(tester);
    final entry = (await retries(tester, host)).single;
    await host.run(
      tester,
      () => host.db.outboxDao.recordFailure(
        entry.id,
        code: 'agent_invocation_denied',
        reason: 'operator_not_in_channel',
      ),
    );
    await host.settle(tester);
    expect(
      find.text(en.liveBoxRetryRefused(en.triggerReasonOperatorNotInChannel)),
      findsOneWidget,
    );
    expect(find.text(en.liveBoxRetry), findsOneWidget);
    expect(await retries(tester, host), isEmpty);
    await host.dispose(tester);
  });
}
