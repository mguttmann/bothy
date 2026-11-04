// SPDX-License-Identifier: Apache-2.0

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/views.dart';

const _first = '0192f4a0-7b1c-7000-8a00-000000000902';

Future<void> _pumpHistory(
  WidgetTester tester,
  ViewWorld world, {
  Locale locale = const Locale('en'),
}) async {
  tester.view.physicalSize = const Size(1400, 2400);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await pumpWorld(
    tester,
    world,
    world.view(VizViewKind.runHistory),
    locale: locale,
  );
}

void main() {
  testWidgets(
    'lists the finished runs with their figures and the retention note',
    (tester) async {
      final world = ViewWorld();
      await _pumpHistory(tester, world);
      expect(find.text('Run history'), findsOneWidget);
      expect(
        find.textContaining('Runs are kept as long as their chat'),
        findsOneWidget,
      );
      expect(find.text('backend / Payment integration'), findsOneWidget);
      expect(find.text('Done'), findsWidgets);
      expect(find.text('Failed'), findsWidgets);
      expect(find.textContaining('0.88'), findsOneWidget);
      expect(find.textContaining('Estimate'), findsWidgets);
      expect(find.textContaining('information only'), findsOneWidget);
      expect(find.text('120,400'), findsOneWidget);
      expect(find.text('940,000'), findsOneWidget);
      expect(find.text('30,500'), findsOneWidget);
      expect(find.text('39\u00a0min'), findsOneWidget);
      expect(world.queries.historyQueries.single.cursor, isNull);
      expect(world.queries.historyQueries.single.spaceId, isNull);
    },
  );

  testWidgets('loads the next page with the cursor and then shows the end', (
    tester,
  ) async {
    final world = ViewWorld();
    await _pumpHistory(tester, world);
    expect(
      find.text('Start of the history: there are no older runs.'),
      findsNothing,
    );
    await tester.ensureVisible(find.text('Load more runs'));
    await tester.tap(find.text('Load more runs'));
    await settleWorld(tester, world);
    expect(world.queries.historyQueries.last.cursor, isNotNull);
    expect(find.text('Load more runs'), findsNothing);
    expect(
      find.text('Start of the history: there are no older runs.'),
      findsOneWidget,
    );
  });

  testWidgets('the end state filter asks the server again for the space', (
    tester,
  ) async {
    final world = ViewWorld(scope: const VizScope.space('space-1'));
    await _pumpHistory(tester, world);
    await tester.tap(find.byKey(const ValueKey('viz-history-state-failed')));
    await settleWorld(tester, world);
    final last = world.queries.historyQueries.last;
    expect(last.state, 'failed');
    expect(last.spaceId, 'space-1');
    expect(find.text('backend / Payment integration'), findsNothing);
    expect(find.text('tickets / Login fails after update'), findsOneWidget);
  });

  testWidgets(
    'a row opens the detail with figures, summary, steps and the jump',
    (tester) async {
      final world = ViewWorld();
      await _pumpHistory(tester, world);
      await tester.tap(find.byKey(const ValueKey('viz-history-row-$_first')));
      await settleWorld(tester, world);
      expect(world.queries.calls, contains('run'));
      expect(
        find.text('Resume window fixed; the subagent tests pass.'),
        findsOneWidget,
      );
      expect(
        find.text('The resume window now keeps the cursor of the last batch.'),
        findsOneWidget,
      );
      expect(find.text('model-a'), findsOneWidget);
      expect(find.byType(RunGraphView), findsOneWidget);
      await tester.tap(find.text('Go to the triggering message'));
      expect(world.openedMessages, [
        '0192f4a0-7b1c-7000-8a00-000000000301/0192f4a0-7b1c-7000-8a00-000000000501',
      ]);
      await tester.tap(find.text('Open the chat'));
      expect(world.opened, ['0192f4a0-7b1c-7000-8a00-000000000301']);
      await tester.tap(find.text('Back to the run history'));
      await settleWorld(tester, world);
      expect(find.byType(RunGraphView), findsNothing);
      expect(find.text('backend / Payment integration'), findsOneWidget);
    },
  );

  testWidgets('the table form has the same values', (tester) async {
    final world = ViewWorld();
    await _pumpHistory(tester, world);
    await tester.tap(find.byKey(const ValueKey('viz-as-table')));
    await tester.pump();
    for (final header in [
      'Chat',
      'Run',
      'State',
      'Start',
      'End',
      'Duration',
      'Cost',
      'Input tokens',
      'Output tokens',
      'Cache read',
      'Cache write',
      'Subagents',
    ]) {
      expect(find.text(header), findsWidgets, reason: header);
    }
    expect(find.text('120,400'), findsOneWidget);
  });

  testWidgets('an empty history says so and offers no more page', (
    tester,
  ) async {
    final world = ViewWorld(state: 'empty');
    await _pumpHistory(tester, world);
    expect(find.text('No finished runs yet.'), findsOneWidget);
    expect(find.text('Load more runs'), findsNothing);
  });

  testWidgets('a refusal shows its text', (tester) async {
    final world = ViewWorld();
    world.queries.failOn['runHistory'] = const api.ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    await _pumpHistory(tester, world);
    expect(find.byType(RunGraphView), findsNothing);
    expect(find.text('backend / Payment integration'), findsNothing);
    expect(world.queries.calls, ['runHistory']);
  });

  testWidgets('German texts', (tester) async {
    final world = ViewWorld();
    await _pumpHistory(tester, world, locale: const Locale('de'));
    expect(find.text('Laufhistorie'), findsOneWidget);
    // language-check: off
    expect(find.text('Weitere Läufe laden'), findsOneWidget);
    // language-check: on
  });
}
