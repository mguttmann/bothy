// SPDX-License-Identifier: Apache-2.0

// The screen harness of the accessibility tests (brief M4-32 Design note 1):
// `dsAccessibilityTests` registers the three tests of a screen and runs them on a sample screen
// of the design system with an icon, a button, a name check and a reachable control.

import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget sample() => Column(
  mainAxisSize: MainAxisSize.min,
  children: [
    const Text('Release notes'),
    DsIcon(DsIcons.actionSend, size: DsIconSizeRole.sm, decorative: true),
    TextButton(
      key: const ValueKey('sample.send'),
      onPressed: () {},
      child: const Text('Send'),
    ),
  ],
);

void main() {
  dsAccessibilityTests(
    'a sample screen',
    build: sample,
    open: (tester) async => tester.pump(),
    semantics: (tester, l) {
      expect(find.bySemanticsLabel('Send'), findsOneWidget);
    },
    reachable: (l) => [find.byKey(const ValueKey('sample.send'))],
  );

  dsAccessibilityTests(
    'a sample screen that scrolls itself',
    build: () => ListView(children: [sample()]),
    scroll: false,
    window: const Size(800, 600),
  );

  testWidgets('the host sets the language, the text scale and the theme', (
    tester,
  ) async {
    await tester.pumpWidget(
      dsA11yHost(
        Builder(
          builder: (context) => Text(
            '${Localizations.localeOf(context).languageCode} '
            '${MediaQuery.textScalerOf(context).scale(10)} '
            '${context.dsColors == DsColors.dark}',
          ),
        ),
        locale: const Locale('de'),
        textScale: 2,
        theme: DsThemeMode.dark,
      ),
    );
    expect(find.text('de 20.0 true'), findsOneWidget);
  });
}
