// SPDX-License-Identifier: Apache-2.0

// A banner with an action keeps its text readable at text scale 2 on a phone (brief M5-10;
// 01 8 "Font size", WCAG 1.4.4 and 1.4.10): the action moves below the text instead of
// squeezing it into a column of a few characters.

import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

const String _note =
    'Microphone access is off. Allow it in the Settings app: Apps, this app, Microphone.';

Future<void> _pump(WidgetTester tester, double scale) async {
  tester.view
    ..physicalSize = const Size(390, 844) * 3
    ..devicePixelRatio = 3;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    dsPhoneA11yHost(
      Padding(
        padding: const EdgeInsets.all(DsSpace.s16),
        child: DsBanner(
          tone: DsTone.warning,
          text: _note,
          actionLabel: 'Abbrechen',
          onAction: () {},
        ),
      ),
      textScale: scale,
    ),
  );
  await dsPumpIcons(tester);
}

void main() {
  for (final scale in [1.0, 2.0]) {
    testWidgets(
      'the text of a banner with an action keeps at least half its width at '
      'text scale $scale',
      (tester) async {
        await _pump(tester, scale);
        final banner = tester.getSize(find.byType(DsBanner)).width;
        final text = tester.getSize(find.text(_note)).width;
        expect(text, greaterThanOrEqualTo(banner / 2));
        expect(tester.takeException(), isNull);
        expect(find.text('Abbrechen').hitTestable(), findsOneWidget);
      },
    );
  }

  testWidgets('at text scale 1 the action stays beside a short text', (
    tester,
  ) async {
    tester.view
      ..physicalSize = const Size(390, 844) * 3
      ..devicePixelRatio = 3;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      dsPhoneA11yHost(
        DsBanner(
          tone: DsTone.info,
          text: 'Offline.',
          actionLabel: 'Retry',
          onAction: () {},
        ),
      ),
    );
    await dsPumpIcons(tester);
    expect(
      tester.getCenter(find.text('Retry')).dy,
      moreOrLessEquals(tester.getCenter(find.text('Offline.')).dy, epsilon: 2),
    );
  });

  testWidgets('a banner reports its intrinsic height inside IntrinsicHeight', (
    tester,
  ) async {
    await tester.pumpWidget(
      dsPhoneA11yHost(
        IntrinsicHeight(
          child: DsBanner(
            tone: DsTone.warning,
            text: _note,
            actionLabel: 'Abbrechen',
            onAction: () {},
          ),
        ),
        textScale: 2,
      ),
    );
    await dsPumpIcons(tester);
    expect(tester.takeException(), isNull);
    expect(
      tester.getSize(find.byType(IntrinsicHeight)).height,
      tester.getSize(find.byType(DsBanner)).height,
    );
  });
}
