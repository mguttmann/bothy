// SPDX-License-Identifier: Apache-2.0

import 'dart:io';

import 'package:bothy_viz/bothy_viz.dart';
import 'package:bothy_viz_harness/main.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// The fixtures of bothy_viz, read once from the file system; no server, no network. The
/// widget tests run in a fake async zone, so the reader answers from memory.
final Map<String, String> _files = {
  for (final f in Directory('../fixtures').listSync().whereType<File>())
    f.uri.pathSegments.last: f.readAsStringSync(),
};

Future<String> _read(String name) async {
  final text = _files[name];
  if (text == null) throw FileSystemException('no fixture', name);
  return text;
}

void main() {
  Future<void> settle(WidgetTester tester) async {
    for (var i = 0; i < 5; i++) {
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 20)),
      );
      await tester.pump();
    }
    await dsPumpIcons(tester);
  }

  Future<void> choose(WidgetTester tester, String key, String item) async {
    await tester.tap(find.byKey(ValueKey('$key-button')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(item).last);
    await tester.pumpAndSettle();
    await settle(tester);
  }

  HarnessAppState state(WidgetTester tester) =>
      tester.state(find.byType(HarnessApp));

  testWidgets(
    'the harness starts without a server and every toggle changes the tree',
    (tester) async {
      tester.view.physicalSize = const Size(1600, 1200);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(
        const HarnessApp(reader: _read, motion: DsMotionMode.reduced),
      );
      await settle(tester);
      expect(find.text('Visualization harness'), findsOneWidget);
      expect(find.byType(LaneMark), findsWidgets);

      await choose(tester, 'theme', 'Dark');
      expect(state(tester).theme, DsThemeMode.dark);
      expect(
        Theme.of(tester.element(find.byType(LaneMark).first)).brightness,
        Brightness.dark,
      );

      await choose(tester, 'density', 'Touch');
      expect(
        tester.element(find.byType(LaneMark).first).dsDensity,
        DsDensity.touch,
      );

      await choose(tester, 'textScale', '2.0x');
      expect(
        MediaQuery.textScalerOf(tester.element(find.byType(LaneMark).first))
            .scale(10),
        20,
      );

      await choose(tester, 'language', 'de');
      expect(find.text('Testumgebung der Visualisierungen'), findsOneWidget);

      await choose(tester, 'fixtureState', 'truncated');
      expect(find.text('Mehr Läufe als angezeigt'), findsOneWidget);

      final before = tester.widgetList<LaneMark>(find.byType(LaneMark)).length;
      final slider = find.byKey(const ValueKey('timeLapse'));
      // One frame interval: the replay shows its empty frame.
      tester.widget<Slider>(slider).onChanged!(1);
      await tester.pumpAndSettle();
      expect(state(tester).timeLapse, 1);
      expect(
        tester.widgetList<LaneMark>(find.byType(LaneMark)).length,
        isNot(before),
      );
      final rect = tester.getRect(slider);
      await tester.tapAt(Offset(rect.right - 2, rect.center.dy));
      await tester.pumpAndSettle();
      expect(
        find.text('Keine neuen Daten'),
        findsOneWidget,
        reason: 'stale after the replay',
      );

      for (var i = 1; i < harnessMarks.length; i++) {
        await tester.tap(find.byKey(ValueKey('mark-$i')));
        await tester.pumpAndSettle();
        await dsPumpIcons(tester);
        expect(state(tester).selected, i);
      }
      expect(find.byType(MetricTileMark), findsWidgets);
    },
  );

  testWidgets('a tap on a run card shows its thread', (tester) async {
    tester.view.physicalSize = const Size(1600, 1200);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      const HarnessApp(reader: _read, motion: DsMotionMode.reduced),
    );
    await settle(tester);
    await tester.tap(find.byKey(const ValueKey('mark-1')));
    await tester.pumpAndSettle();
    await dsPumpIcons(tester);
    await tester.tap(find.byType(RunCardMark).first);
    await tester.pump();
    expect(find.textContaining('0192f4a0'), findsOneWidget);
  });
}
