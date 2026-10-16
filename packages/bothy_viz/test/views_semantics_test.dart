// SPDX-License-Identifier: Apache-2.0

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/fixtures.dart';
import 'support/views.dart';

void main() {
  Future<ViewWorld> pumpView(
    WidgetTester tester,
    VizViewKind kind, {
    String state = 'normal',
    DsThemeMode theme = DsThemeMode.light,
  }) async {
    final world = ViewWorld(state: state);
    await tester.runAsync(world.load);
    await pumpWorld(tester, world, world.view(kind), theme: theme);
    return world;
  }

  Future<void> toTable(WidgetTester tester) async {
    await tester.tap(find.byKey(const ValueKey('viz-as-table')));
    await tester.pumpAndSettle();
  }

  /// The texts of every table cell, as their semantics labels say them.
  Set<String> cells(WidgetTester tester) => {
    for (final t in tester.widgetList<VizTable>(find.byType(VizTable)))
      ...t.data.texts,
  };

  for (final kind in viewStates.keys) {
    for (final theme in DsThemeMode.values) {
      testWidgets('${kind.name} and its table pass the guidelines in '
          '${theme.name}', (tester) async {
        final handle = tester.ensureSemantics();
        await pumpView(tester, kind, theme: theme);
        for (final table in [false, true]) {
          if (table) await toTable(tester);
          await expectLater(tester, meetsGuideline(textContrastGuideline));
          await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
          await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
          await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
        }
        handle.dispose();
      });
    }

    testWidgets('${kind.name} and its table have labels', (tester) async {
      final handle = tester.ensureSemantics();
      await pumpView(tester, kind);
      expect(find.bySemanticsLabel(RegExp('.+')), findsWidgets);
      expect(
        find.semantics.byPredicate((n) => n.label == 'As table'),
        findsOneWidget,
      );
      await toTable(tester);
      expect(
        find.bySemanticsLabel(RegExp(r'^Sort by ')),
        findsWidgets,
        reason: 'column headers',
      );
      final first = tester.widget<VizTable>(find.byType(VizTable).first).data;
      if (first.rows.isNotEmpty) {
        expect(
          find.bySemanticsLabel(
            '${first.columns.first}: ${first.rows.first.first.text}',
          ),
          findsWidgets,
          reason: 'a cell names its column',
        );
      }
      handle.dispose();
    });
  }

  testWidgets('the topology table has the texts of every run card', (
    tester,
  ) async {
    await pumpView(tester, VizViewKind.liveTopology);
    final texts = VizTexts.of(tester.element(find.byType(LiveTopologyView)));
    final cards = [
      for (final m in tester.widgetList<RunCardMark>(find.byType(RunCardMark)))
        m.model,
    ];
    expect(cards, isNotEmpty);
    await toTable(tester);
    final table = cells(tester);
    for (final card in cards) {
      expect(table, containsAll(RunCardMark.parts(texts, card)));
    }
  });

  testWidgets('the backend table has the values of every agent panel', (
    tester,
  ) async {
    await pumpView(tester, VizViewKind.backend, state: 'rate_limits');
    final labels = [
      for (final s in tester.widgetList<Semantics>(find.byType(Semantics)))
        if (s.properties.label case final l? when l.contains('Rate limit')) l,
    ];
    await toTable(tester);
    final table = cells(tester);
    for (final text in table.difference({'none'})) {
      expect(labels.join('\n'), contains(text), reason: text);
    }
  });

  testWidgets('the executive table has the value of every tile', (
    tester,
  ) async {
    await pumpView(tester, VizViewKind.executive);
    final texts = VizTexts.of(tester.element(find.byType(ExecutiveView)));
    final tiles = [
      for (final m in tester.widgetList<MetricTileMark>(
        find.byType(MetricTileMark),
      ))
        m.model,
    ];
    final lights = [
      for (final m in tester.widgetList<TrafficLightMark>(
        find.byType(TrafficLightMark),
      ))
        m.model,
    ];
    await toTable(tester);
    final table = cells(tester);
    for (final tile in tiles) {
      expect(table, contains(MetricTileMark.label(texts, tile)));
      expect(table, contains(MetricTileMark.value(texts, tile)));
    }
    for (final light in lights) {
      expect(table, contains(texts.slo(light)));
      expect(table, contains(texts.sloState(light.status)));
    }
  });

  testWidgets('the delivery table has the values of the view', (tester) async {
    await pumpView(tester, VizViewKind.delivery);
    final labels = [
      for (final s in tester.widgetList<Semantics>(find.byType(Semantics)))
        ?s.properties.label,
    ].join('\n');
    await toTable(tester);
    for (final text in cells(tester)) {
      expect(labels, contains(text), reason: text);
    }
  });

  testWidgets('the timeline table has the rows and states of the bars', (
    tester,
  ) async {
    await pumpView(tester, VizViewKind.timeline);
    final bars = tester.widget<BarMark>(find.byType(BarMark)).model;
    await toTable(tester);
    final table = cells(tester);
    for (final row in bars.rows) {
      expect(table, contains(row.label));
    }
  });

  testWidgets('a table sorts by a column and reverses on a second tap', (
    tester,
  ) async {
    await pumpView(tester, VizViewKind.backend, state: 'rate_limits');
    await toTable(tester);
    List<String> agents() => [
      for (final row
          in tester.widget<VizTable>(find.byType(VizTable)).data.rows)
        row.first.text,
    ];
    final initial = agents();
    await tester.ensureVisible(find.text('Mode switches'));
    await tester.tap(find.text('Mode switches'));
    await tester.pump();
    final state = tester.state<State<VizTable>>(find.byType(VizTable));
    expect(state, isNotNull);
    // The data keeps the order of the view; the rendered order follows the sort.
    expect(agents(), initial);
    final rendered = [
      for (final name in initial)
        (name, tester.getTopLeft(find.text(name).last).dy),
    ]..sort((a, b) => a.$2.compareTo(b.$2));
    expect(rendered.first.$1, 'dev-runner', reason: '0 switches first');
    await tester.ensureVisible(find.text('Mode switches'));
    await tester.tap(find.text('Mode switches'));
    await tester.pump();
    final reversed = [
      for (final name in initial)
        (name, tester.getTopLeft(find.text(name).last).dy),
    ]..sort((a, b) => a.$2.compareTo(b.$2));
    expect(reversed.first.$1, 'support-agent', reason: '2 switches first');
  });

  testWidgets(
    'a traffic light change in the executive tiles is announced once',
    (tester) async {
      final world = ViewWorld();
      final slo = await tester.runAsync(
        () => memoryFixtures.sloReport('normal'),
      );
      final green = ExecutiveData(slo: slo);
      final red = ExecutiveData(
        slo: slo!.rebuild(
          (b) => b.items.map(
            (e) => e.rebuild(
              (x) => x.state = x.slo == 'bothy-api/requests-availability'
                  ? api.StatsSloState.red
                  : x.state,
            ),
          ),
        ),
      );
      Widget tiles(ExecutiveData data) => ExecutiveTileGroups(
        tiles: ExecutiveTiles.of(data, costsVisible: true),
      );
      await pumpWorld(tester, world, tiles(green));
      tester.takeAnnouncements();
      await pumpWorld(tester, world, tiles(green));
      expect(tester.takeAnnouncements(), isEmpty);
      await pumpWorld(tester, world, tiles(red));
      final said = tester.takeAnnouncements();
      expect(said, hasLength(1));
      expect(said.single.message, 'API availability is now target missed');
      await pumpWorld(tester, world, tiles(red));
      expect(tester.takeAnnouncements(), isEmpty);
    },
  );

  testWidgets('a run card tap opens its thread', (tester) async {
    final world = await pumpView(tester, VizViewKind.liveTopology);
    await tester.tap(find.byType(RunCardMark).first);
    expect(world.opened, hasLength(1));
    expect(names.agents, isNotEmpty);
  });
}
