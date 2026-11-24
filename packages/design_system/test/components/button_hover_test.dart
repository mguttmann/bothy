// SPDX-License-Identifier: Apache-2.0

// design-system-8: the hover and pressed states of a filled primary button show, in
// color.accent.bg-hover and color.accent.bg-pressed; before, the opaque fill of the button
// covered the ink of its surface.

import 'package:design_system/design_system.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  testWidgets('a primary button paints its hover and pressed colors on top', (
    tester,
  ) async {
    await pumpDs(
      tester,
      Center(
        child: DsButton(
          label: 'Allow once',
          style: DsButtonStyle.primary,
          onPressed: () {},
        ),
      ),
    );
    final accent = DsAccent.of(dsDefaultAccent, Brightness.light);
    final material = find.descendant(
      of: find.byType(DsButton),
      matching: find.byType(Material),
    );
    final ink = tester.renderObject(material.first);
    expect(ink, paints..rrect(color: accent.accentBg));

    final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
    await mouse.addPointer(location: Offset.zero);
    addTearDown(mouse.removePointer);
    await mouse.moveTo(tester.getCenter(find.byType(DsButton)));
    await tester.pumpAndSettle();
    expect(
      ink,
      paints
        ..rrect(color: accent.accentBg)
        ..rrect(color: accent.accentBgHover),
    );

    final press = await tester.startGesture(
      tester.getCenter(find.byType(DsButton)),
    );
    await tester.pumpAndSettle();
    expect(
      ink,
      paints
        ..rrect(color: accent.accentBg)
        ..rrect(color: accent.accentBgHover)
        ..rrect(color: accent.accentBgPressed),
    );
    await press.up();
  });
}
