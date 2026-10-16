// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';

import 'package:bothy_viz/bothy_viz.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/views.dart';

void main() {
  Future<ViewWorld> pumpBackend(
    WidgetTester tester, {
    String state = 'rate_limits',
    bool withLive = false,
    bool asTable = false,
  }) async {
    final world = ViewWorld(state: state, withLive: withLive);
    await tester.runAsync(world.load);
    await pumpWorld(tester, world, world.view(VizViewKind.backend));
    if (asTable) {
      await tester.tap(find.byKey(const ValueKey('viz-as-table')));
      await tester.pumpAndSettle();
    }
    return world;
  }

  String all(WidgetTester tester) => renderedTexts(tester).join('\n');

  for (final asTable in [false, true]) {
    final mode = asTable ? 'table' : 'view';
    testWidgets('configured next to reported mode with the mismatch marker '
        '($mode)', (tester) async {
      await pumpBackend(tester, asTable: asTable);
      final text = all(tester);
      expect(text, contains('Configured mode'));
      expect(text, contains('Reported mode'));
      // dev-runner is configured organization and reports personal.
      expect(text, contains('Reported: Personal'));
      // Texter reports the configured mode: no marker for it.
      expect('Reported: Organization'.allMatches(text), isEmpty);
      expect(text, contains('per run'));
    });

    testWidgets('I2 and I3 denials and switches per agent ($mode)', (
      tester,
    ) async {
      await pumpBackend(tester, asTable: asTable);
      final text = all(tester);
      expect(text, contains('Denied before delivery (I2)'));
      expect(text, contains('Failed on mode mismatch (I3)'));
      if (!asTable) {
        expect(text, contains('Denied before delivery (I2): 4'));
        expect(text, contains('Failed on mode mismatch (I3): 3'));
      }
    });

    testWidgets('every rate-limit state with its text ($mode)', (tester) async {
      await pumpBackend(tester, asTable: asTable);
      final text = all(tester);
      for (final state in [
        'within the limit',
        'near the limit',
        'limited',
        'unknown',
      ]) {
        expect(text, contains(state), reason: state);
      }
      expect(text, contains('resets at 10:20'));
    });
  }

  testWidgets('presence and reported mode come from the live frame', (
    tester,
  ) async {
    await pumpBackend(tester, state: 'normal', withLive: true);
    final text = all(tester);
    expect(text, contains('Online'));
    expect(text, contains('Degraded'));
    // The frame reports organization for dev-runner, newer than the report's personal.
    expect(text, isNot(contains('Reported: Personal')));
  });

  testWidgets('personal costs carry "information only"', (tester) async {
    await pumpBackend(tester);
    expect(all(tester), contains('information only'));
  });

  test(
    'the backend schema has no runtime, subscription or credential member',
    () {
      // The view draws only the members of the generated types; the fixture names every
      // member the schema allows, so none of them can carry a runtime name (I5, T8).
      final items =
          (jsonDecode(fixtureFiles['StatsBackendReport.rate_limits.json']!)
                  as Map<String, Object?>)['items']!
              as List<Object?>;
      final keys = {
        for (final item in items.cast<Map<String, Object?>>()) ...item.keys,
      };
      expect(
        keys,
        everyElement(isNot(matches(r'runtime|subscription|credential'))),
      );
    },
  );

  testWidgets('no runtime, subscription or credential text is shown', (
    tester,
  ) async {
    await pumpBackend(tester, state: 'normal', withLive: true);
    final text = all(tester).toLowerCase();
    for (final word in ['runtime', 'subscription', 'credential']) {
      expect(text, isNot(contains(word)), reason: word);
    }
  });
}
