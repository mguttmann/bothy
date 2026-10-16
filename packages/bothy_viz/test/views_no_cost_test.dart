// SPDX-License-Identifier: Apache-2.0

import 'package:bothy_viz/bothy_viz.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/views.dart';

void main() {
  const kinds = [
    VizViewKind.liveTopology,
    VizViewKind.backend,
    VizViewKind.delivery,
    VizViewKind.executive,
    VizViewKind.timeline,
  ];
  // Every currency of the fixtures and the cost headers of the views.
  final costPattern = RegExp(r'USD|EUR|Cost|0[.,]00');

  for (final kind in kinds) {
    for (final asTable in [false, true]) {
      testWidgets(
        '${kind.name} without cost permission shows no cost, no zero and no currency '
        '(${asTable ? 'table' : 'view'})',
        (tester) async {
          final world = ViewWorld(state: 'no_cost_permission');
          await tester.runAsync(world.load);
          await pumpWorld(tester, world, world.view(kind));
          if (asTable) {
            await tester.tap(find.byKey(const ValueKey('viz-as-table')));
            await tester.pumpAndSettle();
          }
          final hits = renderedTexts(tester).where(costPattern.hasMatch);
          expect(hits, isEmpty);
        },
      );
    }
  }

  for (final kind in [VizViewKind.backend, VizViewKind.executive]) {
    testWidgets('${kind.name} labels personal costs "information only"', (
      tester,
    ) async {
      final world = ViewWorld();
      await tester.runAsync(world.load);
      await pumpWorld(tester, world, world.view(kind));
      expect(
        renderedTexts(tester).where((t) => t.contains('information only')),
        isNotEmpty,
      );
    });
  }

  testWidgets('estimates carry "Estimate"', (tester) async {
    final world = ViewWorld();
    await tester.runAsync(world.load);
    await pumpWorld(tester, world, world.view(VizViewKind.liveTopology));
    expect(
      renderedTexts(tester).where((t) => t.contains('Estimate')),
      isNotEmpty,
    );
  });
}
