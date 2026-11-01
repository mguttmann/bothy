// SPDX-License-Identifier: Apache-2.0

// The agent tree below the run cards of the live topology (FEAT-topology-agent-tree;
// docs/spec/25-visualization.md 5.2 and 6.1; docs/design/topology-agent-tree.md 6).

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:bothy_viz/src/views/table/live_topology_table.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'support/fixtures.dart';
import 'support/views.dart';

/// The card of run 401 (dev-runner in #backend) of the agent tree fixture.
const _treeRun = '0192f4a0-7b1c-7000-8a00-000000000401';
const _treeThread = '0192f4a0-7b1c-7000-8a00-000000000301';

void main() {
  late api.StatsLiveFrame frame;
  late LiveTopologyModel model;
  setUpAll(() async {
    frame = await fixtures.liveFrame('agent_tree');
    model = LiveTopologyModel.fromFrame(frame, names: names);
  });

  RunCardModel card(String runId) =>
      model.cards.firstWhere((c) => c.runId == runId);

  group('model', () {
    test('the frame list becomes a tree: a parent above its children, '
        'in frame order', () {
      final roots = card(_treeRun).tasks;
      expect(
        [for (final r in roots) r.label],
        ['Build and review', 'Check the tests', null],
      );
      final [flow, check, job] = roots;
      expect(flow.kind, TaskNodeKind.workflow);
      expect([for (final c in flow.children) c.label], ['build', 'review']);
      expect(check.children.single.label, 'Run the linter');
      expect(check.children.single.state, TaskNodeState.failed);
      expect(job.kind, TaskNodeKind.backgroundJob);
      expect(job.children, isEmpty);
      expect(flow.count, 3);
    });

    test('a running node counts its runtime to the frame time, a finished '
        'one to its end; the tool and the position come along', () {
      final [flow, check, job] = card(_treeRun).tasks;
      final [build, review] = flow.children;
      // at 10:00:00; review started 09:59:31, build ran 09:58:41 to 09:59:30.
      expect(review.runtime, const Duration(seconds: 29));
      expect(review.tool, 'Grep');
      expect(review.state, TaskNodeState.running);
      expect(build.runtime, const Duration(seconds: 49));
      expect(build.state.isFinished, isTrue);
      expect([build.position, review.position, check.position], [1, 2, 3]);
      expect(check.children.single.position, 4);
      expect(job.position, 1);
    });

    test('tasks_truncated reaches the card; a run without tasks has none', () {
      expect(
        card('0192f4a0-7b1c-7000-8a00-000000000402').tasksTruncated,
        isTrue,
      );
      expect(card(_treeRun).tasksTruncated, isFalse);
      final plain = model.cards.where(
        (c) => c.runId.endsWith('403') || c.runId.endsWith('404'),
      );
      expect(plain, isNotEmpty);
      expect(plain.expand((c) => c.tasks), isEmpty);
    });

    test('a node whose parent is unknown or later hangs under the run', () {
      final at = DateTime.utc(2026, 11, 1, 9);
      api.StatsLiveTask task(String id, {String? parent}) => api.StatsLiveTask(
        (b) => b
          ..id = id
          ..parentId = parent
          ..kind = api.StatsLiveTaskKindEnum.subagent
          ..name = 'local_agent'
          ..state = api.StatsLiveTaskStateEnum.running
          ..startedAt = at.subtract(const Duration(seconds: 5)),
      );
      final roots = TaskNodeModel.tree([
        task('a', parent: 'later'),
        task('later'),
        task('b', parent: 'gone'),
        task('c', parent: 'a'),
      ], at);
      expect([for (final r in roots) r.id], ['a', 'later', 'b']);
      expect(roots.first.children.single.id, 'c');
    });

    test('equal frames give equal cards; a new runtime does not', () {
      final again = LiveTopologyModel.fromFrame(frame, names: names);
      expect(again.cards.toList(), model.cards.toList());
      final later = LiveTopologyModel.fromFrame(
        frame.rebuild((b) => b.at = frame.at.add(const Duration(seconds: 1))),
        names: names,
      );
      expect(
        later.cards.firstWhere((c) => c.runId == _treeRun),
        isNot(card(_treeRun)),
      );
    });
  });

  group('mark', () {
    Future<void> pump(WidgetTester tester, Widget child) async {
      await tester.pumpWidget(app(child));
      await dsPumpIcons(tester);
    }

    testWidgets('the card shows its tree: names, unlabelled kind and '
        'position, state, runtime and the current tool', (tester) async {
      await pump(tester, RunCardTreeMark(model: card(_treeRun)));
      expect(find.text('Build and review'), findsOneWidget);
      expect(find.text('review'), findsOneWidget);
      expect(find.text('Run the linter'), findsOneWidget);
      expect(find.text('Background job 1'), findsOneWidget);
      expect(find.text('Tool Grep'), findsOneWidget);
      expect(find.textContaining('Running'), findsWidgets);
      expect(find.textContaining('Failed'), findsOneWidget);
    });

    testWidgets('finished nodes are faded, running ones are not', (
      tester,
    ) async {
      await pump(tester, RunCardTreeMark(model: card(_treeRun)));
      Opacity? opacityOf(String text) => tester
          .widgetList<Opacity>(
            find.ancestor(of: find.text(text), matching: find.byType(Opacity)),
          )
          .firstOrNull;
      expect(opacityOf('build')?.opacity, lessThan(1));
      expect(opacityOf('Run the linter')?.opacity, lessThan(1));
      expect(opacityOf('review'), isNull);
    });

    testWidgets('a branch collapses and expands, and stays collapsed across '
        'frames', (tester) async {
      final first = card(_treeRun);
      await pump(
        tester,
        RunCardTreeMark(key: const ValueKey('r'), model: first),
      );
      const flowId = '0192f4a0-7b1c-7000-8a00-000000000501';
      await tester.tap(find.byKey(const ValueKey('taskTree.toggle.$flowId')));
      await tester.pump();
      expect(find.text('review'), findsNothing);
      expect(find.text('build'), findsNothing);
      expect(find.text('Run the linter'), findsOneWidget);

      final next = LiveTopologyModel.fromFrame(
        frame.rebuild((b) => b.at = frame.at.add(const Duration(seconds: 1))),
        names: names,
      ).cards.firstWhere((c) => c.runId == _treeRun);
      await tester.pumpWidget(
        app(RunCardTreeMark(key: const ValueKey('r'), model: next)),
      );
      expect(find.text('review'), findsNothing, reason: 'still collapsed');

      await tester.tap(find.byKey(const ValueKey('taskTree.toggle.$flowId')));
      await tester.pump();
      expect(find.text('review'), findsOneWidget);
    });

    testWidgets('a tap on a node opens the thread of the run', (tester) async {
      final opened = <String>[];
      await pump(
        tester,
        RunCardTreeMark(model: card(_treeRun), onTap: opened.add),
      );
      await tester.tap(find.text('review'));
      expect(opened, [_treeThread]);
    });

    testWidgets('screen readers hear the tree, each node with its facts and '
        'its place, and the toggles', (tester) async {
      final handle = tester.ensureSemantics();
      await pump(tester, RunCardTreeMark(model: card(_treeRun)));
      expect(
        find.bySemanticsLabel('6 agents and jobs started'),
        findsOneWidget,
      );
      expect(
        find.bySemanticsLabel(
          RegExp(
            r'^review, Subagent, Running, .+, Tool Grep, level 2, 2 of 2$',
          ),
        ),
        findsOneWidget,
      );
      expect(
        find.bySemanticsLabel(
          RegExp(r'^Background job 1, Running, .+, level 1, 3 of 3$'),
        ),
        findsOneWidget,
      );
      expect(find.bySemanticsLabel('Hide steps'), findsNWidgets(2));
      handle.dispose();
    });

    testWidgets('a cut tree says so', (tester) async {
      await pump(
        tester,
        RunCardTreeMark(model: card('0192f4a0-7b1c-7000-8a00-000000000402')),
      );
      expect(find.text('More agents and jobs than shown'), findsOneWidget);
    });

    testWidgets('German texts', (tester) async {
      await tester.pumpWidget(
        app(RunCardTreeMark(model: card(_treeRun)), locale: const Locale('de')),
      );
      await dsPumpIcons(tester);
      expect(find.text('Hintergrundjob 1'), findsOneWidget);
    });
  });

  group('table', () {
    testWidgets('the table lists each node below its run, indented by '
        'level', (tester) async {
      late List<VizTableData> tables;
      await tester.pumpWidget(
        app(
          Builder(
            builder: (context) {
              tables = liveTopologyTables(
                VizTexts.of(context),
                lanes: model.lanes,
                rates: const [],
              );
              return const SizedBox();
            },
          ),
        ),
      );
      final rows = tables.first.rows;
      final agent = tables.first.columns.indexOf('Run');
      final runRow = rows.indexWhere(
        (r) => r[agent].text.startsWith('dev-runner') && r[agent].level == 0,
      );
      final below = [
        for (final r in rows.skip(runRow + 1).take(6))
          (r[agent].text, r[agent].level),
      ];
      expect(below, [
        ('Build and review', 1),
        ('build', 2),
        ('review', 2),
        ('Check the tests', 1),
        ('Run the linter', 2),
        ('Background job 1', 1),
      ]);
      final review = rows[runRow + 3];
      expect(review[tables.first.columns.indexOf('State')].text, 'Running');
      expect(
        review[tables.first.columns.indexOf('Current step')].text,
        'Tool Grep',
      );
      expect(tables.first.texts, contains('More agents and jobs than shown'));
    });
  });

  group('view', () {
    testWidgets('the live topology draws the tree of the frame and opens '
        'the thread from a node', (tester) async {
      final world = ViewWorld(state: 'agent_tree');
      await tester.runAsync(world.load);
      await pumpWorld(tester, world, world.view(VizViewKind.liveTopology));
      expect(find.text('Build and review'), findsOneWidget);
      await tester.ensureVisible(find.text('review'));
      await tester.tap(find.text('review'));
      expect(world.opened, [_treeThread]);
    });
  });

  test('the generated texts exist in every locale', () {
    for (final locale in AppLocalizations.supportedLocales) {
      final l = lookupAppLocalizations(locale);
      expect(l.vizTaskTree(2), isNotEmpty);
      expect(l.vizMoreTasksThanShown, isNotEmpty);
    }
  });
}
