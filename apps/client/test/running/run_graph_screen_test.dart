// SPDX-License-Identifier: Apache-2.0

// The run graph screen D14 (brief M4-39; 01 D14; 09 5.6; 04 6.2): the run is watched with its
// thread in the background focus, the graph shows the rows of the document, Stop only where
// allowed, and "back" goes where the graph was opened from.

import 'package:bothy_viz/bothy_viz.dart';
import 'package:client/agents/providers.dart';
import 'package:client/app/router.dart';
import 'package:client/running/run_graph_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/agent_fakes.dart';
import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/until.dart';
import 'running_now_test.dart' show fill, overrides;

Future<(ColumnHost, StoreRunSource)> pumpGraph(
  WidgetTester tester,
  int n, {
  bool fromThread = false,
  String platform = 'macos',
}) async {
  final host = await ColumnHost.open(tester);
  final source = StoreRunSource();
  source.fetcher.docs[runId(n)] = {
    ...runDoc(
      n,
      steps: {
        's1': step('s1', 1, name: 'Bash'),
        's2': step('s2', 2, kind: 'subagent', name: 'tests'),
        's3': step('s3', 3, name: 'Grep', parent: 's2'),
      },
    ),
    'thread_id': threadId(n).value,
  };
  await tester.runAsync(() => fill(host.db));
  await host.pump(
    tester,
    RunGraphScreen(
      location: RunGraphLocation(runId(n), threadId(n), fromThread: fromThread),
    ),
    platform: platform,
    overrides: [...overrides, runSourceProvider.overrideWithValue(source)],
  );
  await pumpUntil(
    tester,
    () => find.byType(RunGraphView).evaluate().isNotEmpty,
    what: 'the graph',
  );
  return (host, source);
}

void main() {
  testWidgets('the graph of a card: background focus, rows, Stop, back to '
      'Running now', (tester) async {
    final (host, source) = await pumpGraph(tester, 1);
    expect(source.background, [runId(1)]);
    for (final id in [runId(1).value, 's1', 's2', 's3']) {
      expect(find.byKey(ValueKey('runGraph.row.$id')), findsOneWidget);
    }
    expect(find.byKey(const ValueKey('runGraph.stop')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('runGraph.back')));
    await host.settle(tester);
    expect(host.visited.last, '/running');
    await host.dispose(tester);
  });

  testWidgets('without run.cancel no Stop; from the live box back to the '
      'thread', (tester) async {
    final (host, _) = await pumpGraph(tester, 2, fromThread: true);
    expect(find.byKey(const ValueKey('runGraph.stop')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('runGraph.back')));
    await host.settle(tester);
    expect(host.visited.last, '/t/${threadId(2).value}');
    await host.dispose(tester);
  });
}
