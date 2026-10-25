// SPDX-License-Identifier: Apache-2.0

// The list form of the run graph and its announcements (brief M4-39 Tests; 01 8 "Live
// content", "Run graph and control room"; design note D-RV-12).

import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/fixtures.dart';
import 'support.dart';

final AppLocalizations en = lookupAppLocalizations(const Locale('en'));

/// A graph host whose graph the test replaces.
class _Host extends StatefulWidget {
  const _Host({
    required this.initial,
    required this.clock,
    required this.sink,
    this.asList = true,
  });

  final RunGraphModel initial;
  final DateTime Function() clock;
  final List<String> sink;
  final bool asList;

  @override
  State<_Host> createState() => _HostState();
}

class _HostState extends State<_Host> {
  late RunGraphModel graph = widget.initial;

  void show(RunGraphModel next) => setState(() => graph = next);

  @override
  Widget build(BuildContext context) => SizedBox(
    height: 600,
    child: RunGraphView(
      graph: graph,
      visibleSteps: 3,
      announceMinInterval: const Duration(seconds: 10),
      clock: widget.clock,
      initiallyAsList: widget.asList,
      onAnnounce: (_, message) => widget.sink.add(message),
    ),
  );
}

List<Map<String, Object?>> _steps({
  String b = 'running',
  String a = 'running',
}) => [
  step('a', 1, kind: 'subagent', name: 'tests', status: a),
  step('t', 2, name: 'Bash', status: b, parent: 'a', start: 5),
  step('u', 3, name: 'Grep', parent: 'a', start: 6, end: 7),
];

RunGraphModel _graph(List<Map<String, Object?>> steps, int seconds) =>
    RunGraphModel.fromRun(
      runOf(steps),
      t0.add(Duration(seconds: seconds)),
      agentName: 'dev-runner',
    );

Future<_HostState> _pump(
  WidgetTester tester,
  RunGraphModel graph,
  DateTime Function() clock,
  List<String> sink, {
  bool asList = true,
}) async {
  await tester.pumpWidget(
    app(_Host(initial: graph, clock: clock, sink: sink, asList: asList)),
  );
  await dsPumpIcons(tester);
  return tester.state<_HostState>(find.byType(_Host));
}

void main() {
  testWidgets('the outline has tree semantics with the values of the rows', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    final graph = _graph(_steps(), 30);
    await _pump(tester, graph, () => t0, []);
    final root = tester.getSemantics(
      find.byKey(ValueKey('runGraph.row.$runId')),
    );
    expect(root.label, contains('dev-runner'));
    expect(root.label, contains(en.liveBoxStateRunning));
    expect(root.label, contains('0:30'));
    expect(root.label, contains('0.62, ${en.vizCostEstimate}'));
    expect(root.label, contains(en.runGraphTreePosition(1, 1, 1)));
    final a = tester.getSemantics(find.byKey(const ValueKey('runGraph.row.a')));
    expect(
      a.label,
      startsWith(en.runGraphRowLabel(en.toolChipKindSubagent, 'tests')),
    );
    expect(a.label, contains(en.runGraphTreePosition(2, 1, 1)));
    expect(a.label, contains(en.vizStepToolCall('Bash')));
    expect(a.flagsCollection.isExpanded.toBoolOrNull(), isTrue);
    final u = tester.getSemantics(find.byKey(const ValueKey('runGraph.row.u')));
    expect(u.label, contains(en.runGraphTreePosition(3, 2, 2)));
    expect(u.label, contains(en.liveBoxStateCompleted));
    handle.dispose();
  });

  testWidgets('a stall mark ahead of the device clock reads zero minutes', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    final graph = RunGraphModel.fromRun(
      runOf(_steps(), stalledAfter: 90),
      t0.add(const Duration(seconds: 30)),
      agentName: 'dev-runner',
    );
    await _pump(tester, graph, () => t0, []);
    final root = tester.getSemantics(
      find.byKey(ValueKey('runGraph.row.$runId')),
    );
    expect(root.label, contains(en.liveBoxStall(0)));
    handle.dispose();
  });

  testWidgets('the list form has the same rows as the trace view', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    final graph = _graph(_steps(), 30);
    Map<String, String> labels() => {
      for (final id in [runId, 'a', 't', 'u'])
        id: tester.getSemantics(find.byKey(ValueKey('runGraph.row.$id'))).label,
    };
    await _pump(tester, graph, () => t0, [], asList: false);
    final trace = labels();
    await tester.tap(find.byKey(const ValueKey('runGraph.asList')));
    await tester.pump();
    expect(find.byKey(const ValueKey('runGraph.outline')), findsOneWidget);
    expect(labels(), trace);
    handle.dispose();
  });

  testWidgets('the arrow keys move, close and open rows', (tester) async {
    final graph = _graph(_steps(), 30);
    await _pump(tester, graph, () => t0, []);
    final state = tester.state<RunGraphViewState>(find.byType(RunGraphView));
    await tester.tap(find.byKey(ValueKey('runGraph.row.$runId')));
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowLeft);
    await tester.pump();
    expect(find.byKey(const ValueKey('runGraph.row.t')), findsNothing);
    expect(state.rows.length, 2);
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowRight);
    await tester.pump();
    expect(find.byKey(const ValueKey('runGraph.row.t')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('runGraph.toggle.a')));
    await tester.pump();
    expect(find.byKey(const ValueKey('runGraph.row.t')), findsNothing);
  });

  testWidgets('"n earlier steps" opens with a tap', (tester) async {
    final graph = _graph([for (var i = 1; i <= 5; i++) step('r$i', i)], 30);
    await _pump(tester, graph, () => t0, []);
    expect(find.text(en.liveBoxEarlierSteps(2)), findsOneWidget);
    expect(find.byKey(const ValueKey('runGraph.row.r1')), findsNothing);
    await tester.tap(find.text(en.liveBoxEarlierSteps(2)));
    await tester.pump();
    expect(find.byKey(const ValueKey('runGraph.row.r1')), findsOneWidget);
  });

  testWidgets('announcements only on node end and state change, rate-limited', (
    tester,
  ) async {
    var now = t0;
    final sink = <String>[];
    final host = await _pump(tester, _graph(_steps(), 30), () => now, sink);
    // A patch that changes only durations and adds a node is never announced.
    host.show(
      _graph([
        ..._steps(),
        step('v', 4, name: 'Read', parent: 'a', start: 8),
      ], 40),
    );
    await tester.pump();
    expect(sink, isEmpty);

    // The end of a node is announced at once.
    host.show(_graph(_steps(b: 'completed'), 50));
    await tester.pump();
    expect(sink, [en.runGraphAnnounce('Tool Bash', en.liveBoxStateCompleted)]);

    // A second change within the interval waits for it and goes out once.
    now = t0.add(const Duration(seconds: 3));
    host.show(_graph(_steps(b: 'completed', a: 'failed'), 53));
    await tester.pump();
    expect(sink, hasLength(1));
    now = t0.add(const Duration(seconds: 10));
    await tester.pump(const Duration(seconds: 7));
    expect(sink, [
      en.runGraphAnnounce('Tool Bash', en.liveBoxStateCompleted),
      en.runGraphAnnounce('Subagent tests', en.liveBoxStateFailed),
    ]);
  });

  test(
    'the announcer merges changes of one interval, newest state per node',
    () {
      var now = t0;
      final sink = <String>[];
      final announcer = RunGraphAnnouncer(
        minInterval: AppConfig.defaults().liveAnnounceMinInterval,
        clock: () => now,
        sink: sink.add,
        text: (n) => '${n.id} ${n.state.name}',
      );
      announcer.observe(_graph(_steps(), 1));
      expect(sink, isEmpty, reason: 'the first graph is only remembered');
      announcer.observe(_graph(_steps(b: 'failed'), 2));
      expect(sink, ['t failed']);
      announcer.observe(_graph(_steps(b: 'failed', a: 'completed'), 3));
      announcer.observe(_graph(_steps(b: 'failed', a: 'cancelled'), 4));
      announcer.flush();
      expect(sink, hasLength(1));
      now = announcer.pendingDue!;
      announcer.flush();
      expect(sink, ['t failed', 'a cancelled']);
      expect(announcer.pendingDue, isNull);
    },
  );
}
