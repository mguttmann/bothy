// SPDX-License-Identifier: Apache-2.0

// The derivation of the run graph, one test group per rule of 09 5.6 (brief M4-39 Tests).

import 'package:bothy_viz/bothy_viz.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

/// The IDs of the nodes below [node], depth first.
List<String> tree(RunGraphNode node, [String indent = '']) => [
  for (final c in node.children) ...['$indent${c.id}', ...tree(c, '$indent  ')],
];

RunGraphModel graph(
  List<Map<String, Object?>> steps, {
  int seconds = 60,
  String state = 'running',
  String? used = 'organization',
  int? stalledAfter,
  int? finishedAfter,
  bool started = true,
}) => RunGraphModel.fromRun(
  runOf(
    steps,
    state: state,
    used: used,
    stalledAfter: stalledAfter,
    finishedAfter: finishedAfter,
    started: started,
  ),
  t0.add(Duration(seconds: seconds)),
  agentName: 'dev-runner',
);

void main() {
  group('1 root', () {
    test('the run is the root: agent, state, duration, cost and stall', () {
      final m = graph([], seconds: 252, stalledAfter: 200);
      final root = m.root;
      expect(root.kind, RunGraphNodeKind.run);
      expect(root.id, runId);
      expect(root.name, 'dev-runner');
      expect(root.state, RunGraphState.running);
      expect(root.duration, const Duration(minutes: 4, seconds: 12));
      expect(root.start, t0);
      expect(root.end, isNull);
      expect(root.cost?.micros, 620000);
      expect(root.cost?.label, CostLabel.estimate);
      expect(root.stalledSince, t0.add(const Duration(seconds: 200)));
      expect(root.isRunning, isTrue);
    });

    test('the root cost shows only in organization mode', () {
      final m = graph([], used: 'personal');
      expect(m.root.cost, isNull);
    });

    test('before the start the axis and the root begin at queued_at', () {
      final m = graph([], state: 'queued', started: false, seconds: 0);
      expect(m.axisStart, t0.subtract(const Duration(seconds: 5)));
      expect(m.root.state, RunGraphState.queued);
      expect(m.root.duration, const Duration(seconds: 5));
    });

    test('a finished run ends its axis and root bar at finished_at', () {
      final m = graph(
        [step('a', 1, end: 3)],
        state: 'completed',
        finishedAfter: 30,
        seconds: 600,
      );
      expect(m.axisEnd, t0.add(const Duration(seconds: 30)));
      expect(m.root.end, t0.add(const Duration(seconds: 30)));
      expect(m.root.duration, const Duration(seconds: 30));
      expect(m.root.isRunning, isFalse);
    });
  });

  group('2 nodes and markers', () {
    test('every step except status is a node; status steps mark their '
        'parent row', () {
      final m = graph([
        step('a', 1, kind: 'subagent', name: 'tests', status: 'running'),
        step('b', 2, kind: 'status', name: 'compacting', parent: 'a'),
        step('c', 3, kind: 'status', name: 'retry', title: 'API retry'),
        step('d', 4, parent: 'a'),
      ]);
      expect(tree(m.root), ['a', '  d']);
      expect(m.root.markers.map((x) => (x.name, x.title)), [
        ('retry', 'API retry'),
      ]);
      expect(m.root.children.single.markers.single.name, 'compacting');
    });

    test('the summary step is a row "n steps summarized" under its parent', () {
      final m = graph([
        step('a', 1, kind: 'subagent', name: 'tests', status: 'running'),
        step(
          's',
          2,
          kind: 'status',
          name: 'steps_summarized',
          parent: 'a',
          detail: {
            'summarized': {
              'count': 12,
              'by_kind': {'tool_call': 12},
            },
          },
        ),
        step('d', 3, parent: 'a'),
      ]);
      expect(tree(m.root), ['a', '  s', '  d']);
      final summary = m.root.children.single.children.first;
      expect(summary.kind, RunGraphNodeKind.summary);
      expect(summary.summarized, 12);
      expect(m.root.children.single.markers, isEmpty);
    });
  });

  group('3 parent', () {
    test('a node hangs under its parent, without one under the root', () {
      final m = graph([
        step('a', 1, kind: 'subagent', status: 'running'),
        step('b', 2, parent: 'a'),
        step('c', 3),
      ]);
      expect(tree(m.root), ['a', '  b', 'c']);
    });

    test('an unknown parent puts the node under the root until it arrives', () {
      final early = graph([step('b', 2, parent: 'a')]);
      expect(tree(early.root), ['b']);
      final healed = graph([
        step('a', 1, kind: 'subagent', status: 'running'),
        step('b', 2, parent: 'a'),
      ]);
      expect(tree(healed.root), ['a', '  b']);
    });

    test('a parent cycle never loses a node', () {
      final m = graph([step('a', 1, parent: 'b'), step('b', 2, parent: 'a')]);
      expect(tree(m.root), ['a', '  b']);
    });
  });

  group('4 merge rule', () {
    test('a tool call with exactly one task child becomes that task', () {
      final m = graph([
        step('call', 1, name: 'Task', status: 'running'),
        step(
          'agent',
          2,
          kind: 'subagent',
          name: 'tests',
          title: 'Write test cases',
          status: 'running',
          parent: 'call',
          start: 1,
        ),
        step('r', 3, parent: 'agent', start: 2, end: 3),
        step('g', 4, name: 'Grep', parent: 'call', start: 4, end: 5),
      ]);
      expect(tree(m.root), ['agent', '  r', '  g']);
      final node = m.root.children.single;
      expect(node.kind, RunGraphNodeKind.subagent);
      expect(node.name, 'tests');
      expect(node.title, 'Write test cases');
      expect(node.subtitle, 'Task');
      expect(node.state, RunGraphState.running);
      expect(node.ordinal, 1);
    });

    test('with two task children the call stays a node of its own', () {
      final m = graph([
        step('call', 1, name: 'Task', status: 'running'),
        step('j1', 2, kind: 'background_job', parent: 'call'),
        step('j2', 3, kind: 'workflow', status: 'running', parent: 'call'),
      ]);
      expect(tree(m.root), ['call', '  j1', '  j2']);
      expect(m.root.children.single.kind, RunGraphNodeKind.toolCall);
      expect(m.root.children.single.subtitle, isNull);
    });
  });

  group('5 order and time', () {
    test('children follow ordinal, not the order of the map', () {
      final m = graph([step('c', 3), step('a', 1), step('b', 2)]);
      expect(tree(m.root), ['a', 'b', 'c']);
    });

    test('a node runs from started_at to finished_at, or to now', () {
      final m = graph([
        step('a', 1, start: 10, end: 25),
        step('b', 2, status: 'running', start: 30),
      ], seconds: 50);
      final [a, b] = m.root.children;
      expect(a.start, t0.add(const Duration(seconds: 10)));
      expect(a.end, t0.add(const Duration(seconds: 25)));
      expect(a.duration, const Duration(seconds: 15));
      expect(b.end, isNull);
      expect(b.duration, const Duration(seconds: 20));
      expect(m.axisStart, t0);
      expect(m.axisEnd, t0.add(const Duration(seconds: 50)));
    });
  });

  group('6 node content', () {
    test('the current tool is the newest running tool call, after the merge', () {
      final m = graph([
        step('call', 1, name: 'Task', status: 'running'),
        step('agent', 2, kind: 'subagent', status: 'running', parent: 'call'),
        step('t1', 3, name: 'Bash', status: 'running', parent: 'agent'),
        step('t2', 4, name: 'Grep', status: 'running', parent: 'agent'),
        step('t3', 5, name: 'Edit', status: 'completed', parent: 'agent'),
      ]);
      // The merged call is no tool of the root any more: its task replaced it.
      expect(m.root.currentTool, isNull);
      expect(m.root.children.single.currentTool, 'Grep');
      expect(m.currentTool, 'Grep');
      expect(m.openSubagents, 1);
      expect(m.openWorkflows, 0);
      expect(m.openBackgroundJobs, 0);
    });

    test('tokens and cost of a child come only from detail.usage', () {
      final m = graph([
        step(
          'a',
          1,
          kind: 'subagent',
          detail: {
            'usage': {'tokens': 4200, 'cost_micros': 31000, 'currency': 'USD'},
          },
        ),
        step('b', 2, kind: 'subagent', detail: {'note': 'no usage'}),
        step(
          'c',
          3,
          kind: 'background_job',
          detail: {
            'usage': {'tokens': 10},
          },
        ),
      ]);
      final [a, b, c] = m.root.children;
      expect(a.tokens, 4200);
      expect(a.cost?.micros, 31000);
      expect(a.cost?.label, CostLabel.estimate);
      expect(b.tokens, isNull);
      expect(b.cost, isNull);
      expect(c.tokens, 10);
      expect(c.cost, isNull, reason: 'no currency, no cost');
    });

    test('a child cost shows only when the run used organization mode', () {
      final usage = {
        'usage': {'cost_micros': 31000, 'currency': 'USD'},
      };
      for (final used in [null, 'personal']) {
        final m = graph([
          step('a', 1, kind: 'subagent', detail: usage),
        ], used: used);
        expect(m.root.children.single.cost, isNull, reason: '$used');
      }
    });
  });

  group('7 limits', () {
    List<Map<String, Object?>> finished(int n, {String? parent}) => [
      for (var i = 1; i <= n; i++)
        step('${parent ?? 'r'}$i', 10 + i, parent: parent),
    ];

    List<String> rowsOf(RunGraphModel m, {Set<String> expanded = const {}}) => [
      for (final r in m.rows(visibleSteps: 3, expanded: expanded))
        switch (r) {
          RunGraphNodeRow(:final node, :final depth) => '$depth ${node.id}',
          RunGraphEarlierRow(:final count, :final depth) =>
            '$depth earlier $count',
        },
    ];

    test('above ui.livebox_visible_steps finished children the oldest '
        'collapse into "n earlier steps"', () {
      final m = graph(finished(5));
      expect(rowsOf(m), ['0 $runId', '1 earlier 2', '1 r3', '1 r4', '1 r5']);
    });

    test('"n earlier steps" expands on demand', () {
      final m = graph(finished(5));
      expect(rowsOf(m, expanded: {runId}), [
        '0 $runId',
        '1 r1',
        '1 r2',
        '1 r3',
        '1 r4',
        '1 r5',
      ]);
    });

    test('running nodes and the ancestors of running nodes never collapse', () {
      final m = graph([
        step('a', 1, kind: 'subagent', status: 'completed'),
        step('t', 2, status: 'running', parent: 'a'),
        ...finished(5),
        step('b', 30, status: 'running'),
      ]);
      expect(rowsOf(m), [
        '0 $runId',
        '1 a',
        '2 t',
        '1 earlier 2',
        '1 r3',
        '1 r4',
        '1 r5',
        '1 b',
      ]);
    });

    test('a row collapsed by the person hides its subtree', () {
      final m = graph([
        step('a', 1, kind: 'subagent', status: 'running'),
        step('t', 2, parent: 'a'),
      ]);
      final rows = m.rows(visibleSteps: 3, collapsed: {'a'});
      expect(
        [for (final r in rows) (r as RunGraphNodeRow).node.id],
        [runId, 'a'],
      );
      final a = rows[1] as RunGraphNodeRow;
      expect(a.hasChildren, isTrue);
      expect(a.expanded, isFalse);
      expect(a.position, 1);
      expect(a.siblings, 1);
    });

    test('a large run derives in one pass', () {
      final steps = [
        for (var i = 0; i < 2000; i++)
          step('s$i', i + 1, parent: i == 0 ? null : 's${i ~/ 2}'),
      ];
      final run = runOf(steps);
      final m = RunGraphModel.fromRun(run, t0, agentName: 'dev-runner');
      expect(m.nodeCount, 2000);
      expect(m.rows(visibleSteps: 3).length, greaterThan(1));
    });
  });
}
