// SPDX-License-Identifier: Apache-2.0

// The session column's agent part, the follow-up form of the composer and the agent's typing
// line (brief M4-21 Design notes 2 and 5; 01 D1, 6.2, 6.7; D-L9).

import 'package:client/agents/agent_reads.dart';
import 'package:client/agents/providers.dart';
import 'package:client/session_column/session_column.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/agent_fakes.dart';
import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

FakeAgentReads agentReads() => FakeAgentReads()
  ..agents[robin] = (
    billingMode: BillingMode.organization,
    presence: AgentRuntimeState.online,
    reportedBillingMode: BillingMode.personal,
  )
  ..sessionsOf[threadId(1)] = <AgentSessionView>[
    (
      id: uuid(0xb01),
      externalSessionRef: 'sess-7f3a',
      current: true,
      model: 'model-large',
      cost: const Money(amountMicros: 0, currency: 'USD', isEstimate: true),
      createdAt: t0,
      endedAt: null,
    ),
    (
      id: uuid(0xb02),
      externalSessionRef: 'sess-1a2b',
      current: false,
      model: null,
      cost: const Money(amountMicros: 0, currency: 'USD', isEstimate: true),
      createdAt: t0.subtract(const Duration(days: 1)),
      endedAt: t0,
    ),
  ]
  ..costs[threadId(1)] = [
    (
      mode: BillingMode.organization,
      money: const Money(
        amountMicros: 1840000,
        currency: 'USD',
        isEstimate: true,
      ),
    ),
  ]
  ..runCounts[threadId(1)] = 5
  ..projects[channelId(1)] = 'website, main';

Future<ColumnHost> open(
  WidgetTester tester,
  Widget child,
  FakeAgentReads reads, {
  RunState state = RunState.running,
}) async {
  final host = await ColumnHost.open(tester);
  final source = StoreRunSource();
  source.fetcher.docs[runId(1)] = runDoc(
    1,
    contextPct: 62,
    model: 'model-large',
  );
  await host.run(tester, () async {
    await seedThread(
      host.db,
      th: thread(1, title: 'Deploy', operator: robin, lastSeq: 1),
      messages: [msg(1, author: alex)],
      level: AccessLevel.trigger,
    );
    await host.db.runsDao.upsertIfNewer(run(1, state));
  });
  await host.pump(
    tester,
    child,
    location: '/t/${threadId(1).value}',
    overrides: [
      ...threadOverrides(FakeThreadReads(host.db)),
      runSourceProvider.overrideWithValue(source),
      agentReadsProvider.overrideWithValue(reads),
    ],
  );
  return host;
}

void main() {
  testWidgets('the agent part: presence, backend with the warning, model, '
      'session, project, context, cost, runs, earlier sessions', (
    tester,
  ) async {
    final host = await open(
      tester,
      SessionColumn(threadId: threadId(1)),
      agentReads(),
    );
    final section = find.byKey(const ValueKey('session.agent'));
    expect(section, findsOneWidget);
    expect(find.text(en.adminAgentStateOnline), findsOneWidget);
    final badge = tester.widget<DsBillingBadge>(find.byType(DsBillingBadge));
    expect(badge.mode, DsBillingMode.organization);
    expect(badge.reportedMode, DsBillingMode.personal);
    expect(find.text('model-large'), findsOneWidget);
    expect(find.text('sess-7f3a'), findsOneWidget);
    expect(find.text('website, main'), findsOneWidget);
    expect(find.textContaining('62'), findsOneWidget);
    final cost = tester.widget<DsCostText>(find.byType(DsCostText));
    expect(cost.cost.isEstimate, isTrue);
    expect(find.text('5'), findsOneWidget);
    expect(find.text(en.sessionAgentEarlier(1)), findsOneWidget);
    expect(find.text('sess-1a2b'), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('values the person may not read stay absent', (tester) async {
    final host = await open(
      tester,
      SessionColumn(threadId: threadId(1)),
      FakeAgentReads(),
      state: RunState.completed,
    );
    expect(find.byKey(const ValueKey('session.agent')), findsOneWidget);
    expect(find.byType(DsBillingBadge), findsNothing);
    expect(find.byType(DsCostText), findsNothing);
    expect(find.byKey(const ValueKey('session.agent.runs')), findsNothing);
    await host.dispose(tester);
  });

  testWidgets('during a run the composer reads "Follow-up to" the agent and '
      'the agent is working', (tester) async {
    final host = await open(
      tester,
      HistoryColumn(threadId: threadId(1)),
      agentReads(),
    );
    expect(
      tester.widget<DsComposer>(find.byType(DsComposer)).followupAgentName,
      'Robin',
    );
    expect(find.text(en.agentWorking(dsIsolate('Robin'))), findsOneWidget);
    // The run ends: the composer is a message again and the line goes.
    await host.run(
      tester,
      () => host.db.runsDao.upsertIfNewer(
        run(1, RunState.completed).copyWith(version: const ResourceVersion(2)),
      ),
    );
    await host.settle(tester);
    expect(
      tester.widget<DsComposer>(find.byType(DsComposer)).followupAgentName,
      isNull,
    );
    expect(find.text(en.agentWorking(dsIsolate('Robin'))), findsNothing);
    await host.dispose(tester);
  });
}
