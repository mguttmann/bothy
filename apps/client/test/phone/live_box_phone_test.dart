// SPDX-License-Identifier: Apache-2.0

// The live box and the cards on the phone (brief M5-02 fixed requirement 15, Design note 5,
// Tests; 01 5.3 M3 and M4): the compact row with Stop at its top right in every active state,
// the full box as a sheet on a tap, stacked approval buttons, and no swipe, drag or fling on an
// approval card sends an interaction.

import 'package:client/agents/providers.dart';
import 'package:client/sync/agent_ops.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/agent_fakes.dart';
import '../support/card_host.dart';
import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

Future<ColumnHost> openRun(
  WidgetTester tester,
  RunState state,
  Map<String, Object?> doc,
) async {
  final host = await ColumnHost.open(tester);
  final source = StoreRunSource();
  source.fetcher.docs[runId(1)] = doc;
  await host.run(tester, () async {
    await seedThread(
      host.db,
      messages: [
        msg(1, author: alex, text: '@robin run the tests'),
        msg(
          2,
          author: robin,
          text: '',
          kind: MessageKind.run,
          origin: MessageOrigin.agent,
          minutes: 1,
        ).copyWith(runId: runId(1)),
      ],
      level: AccessLevel.trigger,
    );
    await host.db.runsDao.upsertIfNewer(run(1, state));
  });
  await host.pump(
    tester,
    HistoryColumn(threadId: threadId(1)),
    location: '/t/${threadId(1).value}',
    platform: 'ios',
    overrides: [
      ...threadOverrides(FakeThreadReads(host.db)),
      runSourceProvider.overrideWithValue(source),
      agentReadsProvider.overrideWithValue(FakeAgentReads()),
    ],
  );
  return host;
}

void main() {
  for (final (state, doc) in [
    (RunState.running, runDoc(1, statusLine: 'go test ./internal/sync/...')),
    (
      RunState.waitingInput,
      runDoc(1, state: 'waiting_input', statusLine: 'Which database?'),
    ),
    (RunState.queued, runDoc(1, state: 'queued')),
  ]) {
    testWidgets('${state.name}: the compact row with Stop at its top right', (
      tester,
    ) async {
      final host = await openRun(tester, state, doc);
      final line = find.byKey(ValueKey('livebox.line.${runId(1).value}'));
      expect(line, findsOneWidget);
      // On the phone there is no expanded box in the history.
      expect(find.byKey(ValueKey('livebox.${runId(1).value}')), findsNothing);
      final stop = find.bySemanticsLabel(en.liveBoxStopRun('Robin'));
      expect(stop, findsOneWidget);
      final row = tester.getRect(line);
      final button = tester.getRect(stop);
      expect(button.right, closeTo(row.right, DsSpace.s16));
      expect(button.top, closeTo(row.top, DsSpace.s8));
      await host.dispose(tester);
    });
  }

  testWidgets('a tap on the row opens the full box as a sheet', (tester) async {
    final host = await openRun(
      tester,
      RunState.running,
      runDoc(
        1,
        statusLine: 'go test ./internal/sync/...',
        steps: {'s1': step('s1', 1, status: 'completed', name: 'read')},
      ),
    );
    await tester.tap(find.byKey(ValueKey('livebox.line.${runId(1).value}')));
    await host.settle(tester);
    final sheet = find.byKey(ValueKey('livebox.sheet.${runId(1).value}'));
    expect(sheet, findsOneWidget);
    expect(
      find.descendant(of: sheet, matching: find.byType(DsLiveBox)),
      findsOneWidget,
    );
    // Stop is in the sheet too.
    expect(
      find.descendant(
        of: sheet,
        matching: find.bySemanticsLabel(en.liveBoxStopRun('Robin')),
      ),
      findsOneWidget,
    );
    await host.dispose(tester);
  });

  testWidgets('approval buttons stack at full width; no gesture approves', (
    tester,
  ) async {
    final host = await openCard(
      tester,
      text: 'go test wants to use the network.',
      platform: 'ios',
      components: [
        button(1, cardId, customId: 'once', label: 'Allow once'),
        button(2, cardId, customId: 'deny', label: 'Deny', style: 'danger'),
      ],
    );
    final once = find.byKey(const ValueKey('ds-component-once'));
    final deny = find.byKey(const ValueKey('ds-component-deny'));
    final card = tester.getRect(cardFinder);
    expect(tester.getRect(once).width, card.width);
    expect(tester.getRect(deny).top, greaterThan(tester.getRect(once).top));
    for (final gesture in [
      () => tester.drag(cardFinder, const Offset(300, 0)),
      () => tester.drag(cardFinder, const Offset(-300, 0)),
      () => tester.drag(once, const Offset(250, 0)),
      () => tester.fling(once, const Offset(-250, 0), 1000),
      () => tester.fling(deny, const Offset(0, -200), 1000),
    ]) {
      await gesture();
      await host.settle(tester);
    }
    expect(await entriesOf(tester, host, AgentOps.createInteraction), isEmpty);
    await tester.tap(once);
    await host.settle(tester);
    expect(
      await entriesOf(tester, host, AgentOps.createInteraction),
      hasLength(1),
    );
    await host.dispose(tester);
  });
}
