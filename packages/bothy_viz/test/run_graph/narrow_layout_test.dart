// SPDX-License-Identifier: Apache-2.0

// The touch layout of the run graph (brief M5-14 Tests, Design note 1; 01 5.3 M12): the same
// rows and values in `narrow` and `wide`; in `narrow` the bar stands below the label, in `wide`
// beside it; the width picks the layout at the design token `DsLayout.overlayWidth`; with
// `touch` a tap on a row with children closes and opens it.

import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fixtures.dart';
import 'support.dart';

RunGraphModel _graph() => RunGraphModel.fromRun(
  runOf([
    step('a', 1, kind: 'subagent', name: 'tests', status: 'running', start: 2),
    step('t', 2, name: 'Bash', status: 'running', parent: 'a', start: 5),
    step('u', 3, name: 'Grep', parent: 'a', start: 6, end: 7),
    step('j', 4, kind: 'background_job', name: 'npm test', start: 8),
  ]),
  t0.add(const Duration(seconds: 30)),
  agentName: 'dev-runner',
);

Future<void> _pump(
  WidgetTester tester, {
  required double width,
  RunGraphLayout? layout,
  bool touch = false,
}) async {
  tester.view
    ..physicalSize = Size(width, 800)
    ..devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    app(
      SizedBox(
        width: width,
        height: 800,
        child: RunGraphView(
          graph: _graph(),
          visibleSteps: 10,
          announceMinInterval: const Duration(seconds: 10),
          clock: () => t0,
          layout: layout,
          touch: touch,
        ),
      ),
    ),
  );
  await dsPumpIcons(tester);
}

/// The row IDs in order with the semantics label of each row.
List<(String, String)> _rows(WidgetTester tester) {
  final rows = find.byWidgetPredicate(
    (w) =>
        w.key is ValueKey<String> &&
        (w.key! as ValueKey<String>).value.startsWith('runGraph.row.'),
  );
  return [
    for (final e in rows.evaluate())
      (
        (e.widget.key! as ValueKey<String>).value,
        tester
            .getSemantics(
              find
                  .descendant(
                    of: find.byWidget(e.widget),
                    matching: find.byType(Semantics),
                  )
                  .first,
            )
            .label,
      ),
  ];
}

Rect _bar(WidgetTester tester, String id) =>
    tester.getRect(find.byKey(ValueKey('runGraph.bar.$id')));

Rect _label(WidgetTester tester, String id) =>
    tester.getRect(find.byKey(ValueKey('runGraph.label.$id')));

void main() {
  testWidgets('the same rows and values in narrow and wide', (tester) async {
    await _pump(tester, width: 390, layout: RunGraphLayout.narrow);
    final narrow = _rows(tester);
    await _pump(tester, width: 390, layout: RunGraphLayout.wide);
    final wide = _rows(tester);
    expect(narrow, isNotEmpty);
    expect(narrow.map((r) => r.$1), [
      'runGraph.row.$runId',
      'runGraph.row.a',
      'runGraph.row.t',
      'runGraph.row.u',
      'runGraph.row.j',
    ]);
    expect(narrow, wide);
  });

  testWidgets('narrow: the bar below the label over the width of the row', (
    tester,
  ) async {
    await _pump(tester, width: 390, layout: RunGraphLayout.narrow);
    for (final id in [runId, 'a', 't', 'u', 'j']) {
      final bar = _bar(tester, id);
      final label = _label(tester, id);
      expect(bar.top, greaterThanOrEqualTo(label.bottom), reason: id);
      // All bars share one track, so the times stay comparable between rows.
      expect(bar.left, _bar(tester, runId).left, reason: id);
      expect(bar.width, _bar(tester, runId).width, reason: id);
    }
    expect(_bar(tester, runId).width, greaterThan(390 / 2));
  });

  testWidgets('wide: the bar beside the label', (tester) async {
    await _pump(tester, width: 844, layout: RunGraphLayout.wide);
    for (final id in [runId, 'a', 't', 'u', 'j']) {
      expect(
        _bar(tester, id).left,
        greaterThanOrEqualTo(_label(tester, id).right),
        reason: id,
      );
    }
  });

  testWidgets('the width picks the layout at DsLayout.overlayWidth', (
    tester,
  ) async {
    expect(runGraphLayoutFor(DsLayout.overlayWidth - 1), RunGraphLayout.narrow);
    expect(runGraphLayoutFor(DsLayout.overlayWidth), RunGraphLayout.wide);
    await _pump(tester, width: 390);
    expect(
      _bar(tester, 'a').top,
      greaterThanOrEqualTo(_label(tester, 'a').bottom),
    );
    await _pump(tester, width: 844);
    expect(
      _bar(tester, 'a').left,
      greaterThanOrEqualTo(_label(tester, 'a').right),
    );
  });

  testWidgets('touch: a tap on a row with children closes and opens it', (
    tester,
  ) async {
    await _pump(tester, width: 390, touch: true);
    expect(find.byKey(const ValueKey('runGraph.row.t')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('runGraph.label.a')));
    await tester.pump();
    expect(find.byKey(const ValueKey('runGraph.row.t')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('runGraph.label.a')));
    await tester.pump();
    expect(find.byKey(const ValueKey('runGraph.row.t')), findsOneWidget);
    // A row without children only selects.
    await tester.tap(find.byKey(const ValueKey('runGraph.label.j')));
    await tester.pump();
    expect(find.byKey(const ValueKey('runGraph.row.t')), findsOneWidget);
  });

  testWidgets('without touch a tap on the row only selects', (tester) async {
    await _pump(tester, width: 390);
    await tester.tap(find.byKey(const ValueKey('runGraph.label.a')));
    await tester.pump();
    expect(find.byKey(const ValueKey('runGraph.row.t')), findsOneWidget);
  });

  testWidgets('the list form has no bars in either layout', (tester) async {
    for (final layout in RunGraphLayout.values) {
      await tester.pumpWidget(
        app(
          SizedBox(
            width: 390,
            height: 800,
            child: RunGraphView(
              graph: _graph(),
              visibleSteps: 10,
              announceMinInterval: const Duration(seconds: 10),
              clock: () => t0,
              layout: layout,
              initiallyAsList: true,
            ),
          ),
        ),
      );
      await dsPumpIcons(tester);
      expect(
        find.byWidgetPredicate(
          (w) => w is CustomPaint && w.painter is RunGraphBarPainter,
        ),
        findsNothing,
        reason: layout.name,
      );
    }
  });
}
