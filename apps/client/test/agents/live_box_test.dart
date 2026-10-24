// SPDX-License-Identifier: Apache-2.0

// The live box of the open thread (brief M4-21 Tests; 01 6.2, D3; 09 5.4, 5.5; 04 11.3; 15 7.4):
// running, waiting for input, failed with reason, stall notice, interrupted with "Send again";
// at most one rebuild per frame under ten patches; Stop hidden without the right and without a
// confirmation dialog when shown; the collapse at the end into the line built from the summary.

import 'package:client/agents/providers.dart';
import 'package:client/agents/run_mapping.dart';
import 'package:client/sync/message_ops_app.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/agent_fakes.dart';
import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

/// The triggering message of Alex and the live box anchor of run 1.
List<Message> runMessages({TriggerState state = TriggerState.working}) => [
  msg(
    1,
    author: alex,
    text: '@robin run the tests',
  ).copyWith(triggerState: state),
  msg(
    2,
    author: robin,
    text: '',
    kind: MessageKind.run,
    origin: MessageOrigin.agent,
    minutes: 1,
  ).copyWith(runId: runId(1)),
];

final class OpenRun {
  OpenRun(this.host, this.source, this.reads);

  final ColumnHost host;
  final StoreRunSource source;
  final FakeAgentReads reads;
}

Future<OpenRun> openRun(
  WidgetTester tester, {
  RunState state = RunState.running,
  String? reason,
  AccessLevel level = AccessLevel.trigger,
  Map<String, Object?>? doc,
}) async {
  final host = await ColumnHost.open(tester);
  final source = StoreRunSource();
  final reads = FakeAgentReads();
  if (doc != null) source.fetcher.docs[runId(1)] = doc;
  await host.run(tester, () async {
    await seedThread(host.db, messages: runMessages(), level: level);
    await host.db.runsDao.upsertIfNewer(run(1, state, reason: reason));
  });
  await host.pump(
    tester,
    HistoryColumn(threadId: threadId(1)),
    location: '/t/${threadId(1).value}',
    overrides: [
      ...threadOverrides(FakeThreadReads(host.db)),
      runSourceProvider.overrideWithValue(source),
      agentReadsProvider.overrideWithValue(reads),
    ],
  );
  return OpenRun(host, source, reads);
}

Future<List<OutboxEntry>> outbox(WidgetTester tester, ColumnHost host) =>
    host.run(tester, () => host.db.outboxDao.allEntries());

void main() {
  testWidgets('running: pinned box with state, status line and steps; Stop '
      'without a confirmation', (tester) async {
    final t = await openRun(
      tester,
      doc: runDoc(
        1,
        statusLine: 'Reading the export logs',
        lastText: 'The export fails on resume',
        contextPct: 62,
        steps: {
          's1': step('s1', 1, status: 'completed'),
          's2': step('s2', 2, kind: 'subagent', name: 'tests'),
          's3': step('s3', 3, parent: 's2', name: 'bash'),
        },
      ),
    );
    final box = find.byKey(ValueKey('livebox.${runId(1).value}'));
    expect(box, findsOneWidget);
    expect(find.text(en.liveBoxStateRunning), findsOneWidget);
    expect(find.text('Reading the export logs'), findsOneWidget);
    expect(find.text('The export fails on resume'), findsOneWidget);
    final data = tester.widget<DsLiveBox>(box).run;
    expect(data.steps.map((s) => (s.id, s.depth)), [
      ('s1', 0),
      ('s2', 0),
      ('s3', 1),
    ]);
    expect(data.contextPercent, 62);
    expect(data.cost?.isEstimate, isTrue);
    // The box stands above the history, not at the anchor.
    final top = tester.getTopLeft(box).dy;
    expect(
      top,
      lessThan(
        tester.getTopLeft(find.byKey(const ValueKey('history.list'))).dy,
      ),
    );

    await tester.tap(find.text(en.liveBoxStop));
    await t.host.settle(tester);
    expect(find.byType(AlertDialog), findsNothing);
    final stops = [
      for (final e in await outbox(tester, t.host))
        if (e.kind == AppMessageOps.cancelRun) e,
    ];
    expect(stops.single.payload, contains(runId(1).value));
    await t.host.dispose(tester);
  });

  testWidgets('Stop is hidden without the right to cancel', (tester) async {
    final t = await openRun(tester, level: AccessLevel.write, doc: runDoc(1));
    expect(find.byKey(ValueKey('livebox.${runId(1).value}')), findsOneWidget);
    expect(find.text(en.liveBoxStop), findsNothing);
    await t.host.dispose(tester);
  });

  testWidgets('waiting for input reads "Waiting for you" for the starter', (
    tester,
  ) async {
    final t = await openRun(
      tester,
      state: RunState.waitingInput,
      doc: runDoc(1, state: 'waiting_input'),
    );
    expect(find.text(en.liveBoxStateWaitingForYou), findsOneWidget);
    await t.host.dispose(tester);
  });

  testWidgets('a stall notice shows while the document carries '
      'stall_warned_at', (tester) async {
    final t = await openRun(
      tester,
      doc: runDoc(
        1,
        stallWarnedAt: t0
            .subtract(const Duration(minutes: 3))
            .toIso8601String(),
      ),
    );
    expect(find.text(en.liveBoxStall(3)), findsOneWidget);
    await t.host.dispose(tester);
  });

  testWidgets('ten patches in one frame draw the box once with the newest '
      'state', (tester) async {
    final t = await openRun(tester, doc: runDoc(1, statusLine: 'step 0'));
    expect(find.text('step 0'), findsOneWidget);
    final before = t.source.published.length;
    for (var v = 2; v <= 11; v++) {
      t.source.store.onRunUpdated({
        'run_id': runId(1).value,
        'version': v,
        'base_version': v - 1,
        'ops': [
          {'op': 'replace', 'path': '/status_line', 'value': 'step ${v - 1}'},
        ],
      });
    }
    await tester.pump();
    await tester.pump();
    expect(t.source.published.length - before, 1);
    expect(find.text('step 10'), findsOneWidget);
    await t.host.dispose(tester);
  });

  testWidgets('the end collapses the box into the line of the summary', (
    tester,
  ) async {
    final t = await openRun(tester, doc: runDoc(1));
    expect(find.byKey(ValueKey('livebox.${runId(1).value}')), findsOneWidget);
    final ended = run(1, RunState.completed).copyWith(
      version: const ResourceVersion(2),
      durationMs: 28 * 60 * 1000,
      stepCount: 3,
      summary: 'Found the cause',
    );
    await t.host.run(tester, () => t.host.db.runsDao.upsertIfNewer(ended));
    await t.host.settle(tester);
    expect(find.byKey(ValueKey('livebox.${runId(1).value}')), findsNothing);
    final line = find.byKey(ValueKey('runline.${runId(1).value}'));
    expect(line, findsOneWidget);
    expect(find.textContaining(en.liveBoxRan('28\u00a0min')), findsOneWidget);
    expect(find.textContaining(en.liveBoxTools(3)), findsOneWidget);

    // Expanding loads the document once (getRun).
    t.reads.runs[runId(1)] = runDoc(1, version: 3, state: 'completed');
    await tester.tap(line);
    await t.host.settle(tester);
    expect(
      find.byKey(ValueKey('runline.open.${runId(1).value}')),
      findsOneWidget,
    );
    await t.host.dispose(tester);
  });

  testWidgets('interrupted: "Run interrupted" with "Send again" puts the '
      'triggering text into the outbox', (tester) async {
    final t = await openRun(
      tester,
      state: RunState.failed,
      reason: 'interrupted',
    );
    t.reads.runs[runId(1)] = runDoc(
      1,
      state: 'failed',
      reason: 'interrupted',
      triggeringMessageId: messageId(1).value,
    );
    expect(find.text(en.liveBoxReasonInterrupted), findsOneWidget);
    await tester.tap(find.text(en.liveBoxSendAgain));
    await t.host.settle(tester);
    final sent = [
      for (final e in await outbox(tester, t.host))
        if (e.kind == MessageOps.createMessage) e,
    ];
    expect(sent.single.payload, contains('@robin run the tests'));
    await t.host.dispose(tester);
  });

  testWidgets('failed with a reason shows the translated reason', (
    tester,
  ) async {
    await tester.pumpWidget(
      hostScreen(
        Padding(
          padding: const EdgeInsets.all(16),
          child: DsLiveBox(
            runFromDocument(
              RunDocument.fromJson(
                runDoc(1, state: 'failed', reason: 'project_not_ready'),
              ),
              agentName: 'Robin',
              now: t0,
            ),
            visibleSteps: 6,
            announceMinInterval: const Duration(seconds: 10),
          ),
        ),
      ),
    );
    expect(find.text(en.liveBoxStateFailed), findsOneWidget);
    expect(find.text(en.liveBoxReasonProjectNotReady), findsOneWidget);
  });
}
