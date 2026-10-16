// SPDX-License-Identifier: Apache-2.0

import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/views.dart';

void main() {
  for (final MapEntry(key: kind, value: states) in viewStates.entries) {
    for (final state in states) {
      testWidgets(
        '${kind.name} $state runs no animation in the reduced context',
        (tester) async {
          final world = ViewWorld(state: state);
          await tester.runAsync(world.load);
          await pumpWorld(tester, world, world.view(kind));
          expect(tester.hasRunningAnimations, isFalse);
          final toggle = find.byKey(const ValueKey('viz-as-table'));
          if (toggle.evaluate().isNotEmpty) {
            await tester.tap(toggle);
            await tester.pump();
            expect(tester.hasRunningAnimations, isFalse);
          }
        },
      );
    }
  }

  testWidgets('the timeline playhead jumps in the reduced context', (
    tester,
  ) async {
    final world = ViewWorld();
    await pumpWorld(tester, world, world.view(VizViewKind.timeline));
    await tester.tap(find.byKey(const ValueKey('viz-timeline-play')));
    await tester.pump();
    final align = find.byType(AnimatedAlign);
    expect(tester.widget<AnimatedAlign>(align).duration, Duration.zero);
    final before = tester.getTopLeft(
      find.descendant(of: align, matching: find.byType(ColoredBox)),
    );
    world.clock.advance(const Duration(minutes: 1));
    await tester.pump();
    expect(tester.hasRunningAnimations, isFalse);
    final after = tester.getTopLeft(
      find.descendant(of: align, matching: find.byType(ColoredBox)),
    );
    expect(after.dx, greaterThan(before.dx));
  });

  testWidgets('the timeline playhead moves with motion.* in the full context', (
    tester,
  ) async {
    final world = ViewWorld();
    await pumpWorld(
      tester,
      world,
      world.view(VizViewKind.timeline),
      motion: DsMotionMode.full,
      settleIcons: false,
    );
    expect(
      tester.widget<AnimatedAlign>(find.byType(AnimatedAlign)).duration,
      DsMotion.full.durationMove,
    );
  });
}
