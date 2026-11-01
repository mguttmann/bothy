// SPDX-License-Identifier: Apache-2.0

import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/fixtures.dart';
import 'support/marks.dart';

void main() {
  late MarkModels models;
  setUpAll(() async => models = await MarkModels.load());

  Future<void> pump(WidgetTester tester, Widget child) async {
    await tester.pumpWidget(app(child));
    await dsPumpIcons(tester);
  }

  Widget all({RunTapCallback? onTap}) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      LaneMark(model: models.normal.lanes.first, onTapRun: onTap),
      RunCardMark(model: models.normal.lanes.last.cards.last, onTap: onTap),
      AreaMark(title: 'Rates', series: models.timeline.series.take(3).toList()),
      BarMark(model: models.timeline.bars),
      for (final light in models.lights) TrafficLightMark(model: light),
      MetricTileMark(model: models.tiles.first),
      for (final tile in models.costTiles) MetricTileMark(model: tile),
    ],
  );

  testWidgets('every mark has a label built from its model', (tester) async {
    final handle = tester.ensureSemantics();
    await pump(tester, all());
    expect(
      find.bySemanticsLabel(
        // The money format puts a no-break space after the currency code.
        RegExp(
          r'^backend / Payment integration, dev-runner · 000401, Running, '
          r'Tool Bash, Organization, USD\s0\.62, Estimate$',
        ),
      ),
      findsOneWidget,
    );
    expect(
      find.bySemanticsLabel('Engineering, #backend, 1 active run'),
      findsOneWidget,
    );
    expect(find.bySemanticsLabel(RegExp(r'^Rates, ')), findsOneWidget);
    expect(
      find.bySemanticsLabel(
        RegExp(r'^\w+ / [^,]+, dev-runner · \w{6}, \d+ runs?'),
      ),
      findsWidgets,
    );
    expect(
      find.bySemanticsLabel('API availability: on target, own measurement'),
      findsOneWidget,
    );
    expect(
      find.bySemanticsLabel(RegExp('Push acceptance: not enough data')),
      findsOneWidget,
    );
    expect(
      find.bySemanticsLabel(RegExp(r'^Costs, Personal, .*, information only$')),
      findsOneWidget,
    );
    handle.dispose();
  });

  testWidgets(
    'a personal run card shows its cost labelled "information only" (FIX-cost-display)',
    (tester) async {
      final handle = tester.ensureSemantics();
      final personal = models.normal.cards.firstWhere(
        (c) => c.agentName == 'support-agent',
      );
      await pump(tester, RunCardMark(model: personal));
      expect(
        find.textContaining(RegExp(r'USD.*information only')),
        findsOneWidget,
      );
      handle.dispose();
    },
  );

  testWidgets('a card without permission shows no cost', (tester) async {
    final handle = tester.ensureSemantics();
    await pump(tester, RunCardMark(model: models.noCost.cards.first));
    expect(find.textContaining('USD'), findsNothing);
    expect(find.textContaining('0.00'), findsNothing);
    handle.dispose();
  });

  testWidgets(
    'grouped series of one metric read differently, not by color alone',
    (tester) async {
      final handle = tester.ensureSemantics();
      final executive = ExecutiveModel.from(
        usage: await tester.runAsync(
          () => fixtures.series('normal', query: 'usage'),
        ),
        runs: await tester.runAsync(
          () => fixtures.series('normal', query: 'runs'),
        ),
        names: names,
      );
      await pump(tester, AreaMark(title: 'Rates', series: executive.series));
      final rows = [
        for (final s in executive.series)
          AreaMark.seriesLabel(
            VizTexts.of(tester.element(find.byType(AreaMark))),
            s,
          ),
      ];
      expect(rows.toSet(), hasLength(rows.length));
      expect(rows, contains('Messages, Engineering'));
      expect(rows, contains('Runs, dev-runner'));
      handle.dispose();
    },
  );

  testWidgets('every run bar shows the icon of its state, not color alone', (
    tester,
  ) async {
    await pump(tester, BarMark(model: models.timeline.bars));
    final segments = models.timeline.bars.rows
        .expand((r) => r.segments)
        .toList();
    final icons = tester
        .widgetList<DsIcon>(find.byType(DsIcon))
        .map((i) => i.glyph)
        .toList();
    expect(icons, [for (final s in segments) runStateGlyph(s.runState!)]);
    expect(icons, contains(DsIcons.runFailed));
  });

  testWidgets('a tap on a run card reports its thread ID', (tester) async {
    final tapped = <String>[];
    final card = models.normal.lanes.first.cards.first;
    await pump(tester, RunCardMark(model: card, onTap: tapped.add));
    await tester.tap(find.byType(RunCardMark));
    expect(tapped, [card.threadId]);
  });

  testWidgets(
    'the traffic light announces a state change once, nothing when unchanged',
    (tester) async {
      final said = <String>[];
      void announcer(BuildContext context, String message) => said.add(message);
      final green = models.lights.firstWhere(
        (l) => l.status == TrafficLightStatus.onTarget,
      );
      final red = TrafficLightModel(
        slo: green.slo,
        metric: green.metric,
        status: TrafficLightStatus.targetMissed,
        objective: green.objective,
        periodDays: green.periodDays,
        total: green.total,
        good: green.good,
      );
      await pump(tester, TrafficLightMark(model: green, announcer: announcer));
      await pump(tester, TrafficLightMark(model: green, announcer: announcer));
      expect(said, isEmpty);
      await pump(tester, TrafficLightMark(model: red, announcer: announcer));
      expect(said, ['API availability is now target missed']);
      await pump(tester, TrafficLightMark(model: red, announcer: announcer));
      expect(said, hasLength(1));
    },
  );

  testWidgets('the default announcer sends a polite announcement', (
    tester,
  ) async {
    final green = models.lights.firstWhere(
      (l) => l.status == TrafficLightStatus.onTarget,
    );
    final amber = models.lights.firstWhere(
      (l) => l.status == TrafficLightStatus.budgetTight,
    );
    final changed = TrafficLightModel(
      slo: green.slo,
      metric: green.metric,
      status: amber.status,
      objective: green.objective,
      periodDays: green.periodDays,
      total: green.total,
      good: green.good,
    );
    await pump(tester, TrafficLightMark(model: green));
    await pump(tester, TrafficLightMark(model: changed));
    expect(tester.takeException(), isNull);
  });

  for (final theme in DsThemeMode.values) {
    testWidgets('guidelines pass in ${theme.name}', (tester) async {
      final handle = tester.ensureSemantics();
      await tester.pumpWidget(app(all(onTap: (_) {}), theme: theme));
      await dsPumpIcons(tester);
      await expectLater(tester, meetsGuideline(textContrastGuideline));
      await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
      await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
      await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
      handle.dispose();
    });
  }

  testWidgets(
    'a tile without a value says not shown, with a previous value it compares',
    (tester) async {
      await pump(
        tester,
        const Column(
          children: [
            MetricTileModelView(
              MetricTileModel(
                value: null,
                unit: TileUnit.count,
                queryMetric: 'messages',
              ),
            ),
            MetricTileModelView(
              MetricTileModel(
                value: 12,
                previous: 10,
                unit: TileUnit.count,
                queryMetric: 'runs',
              ),
            ),
            MetricTileModelView(
              MetricTileModel(
                value: 8,
                previous: 10,
                unit: TileUnit.percent,
                queryMetric: 'share',
              ),
            ),
            MetricTileModelView(
              MetricTileModel(
                value: 10,
                previous: 10,
                unit: TileUnit.count,
                queryMetric: 'same',
              ),
            ),
            MetricTileModelView(
              MetricTileModel(
                value: 120,
                unit: TileUnit.milliseconds,
                queryMetric: 'duration_ms_p95',
              ),
            ),
          ],
        ),
      );
      expect(find.text('not shown'), findsOneWidget);
      expect(find.text('120\u00a0ms'), findsOneWidget);
      expect(find.text('2 more than the previous period'), findsOneWidget);
      expect(find.text('2 less than the previous period'), findsOneWidget);
      expect(find.text('same as the previous period'), findsOneWidget);
    },
  );
}

/// A const wrapper so the tile list above can be const.
class MetricTileModelView extends StatelessWidget {
  const MetricTileModelView(this.model, {super.key});

  final MetricTileModel model;

  @override
  Widget build(BuildContext context) => MetricTileMark(model: model);
}
