// SPDX-License-Identifier: Apache-2.0

// The phone harness of the accessibility tests (brief M5-10 Design note 2):
// `dsPhoneAccessibilityTests` registers the five tests of a phone screen and runs them on a
// sample screen with a heading, a switch, a button below a long list (reached by scrolling) and
// a name check; the semantics snapshot names role, label, value, state and actions in reading
// order and fails on a changed file.

import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget sample() => ListView(
  children: [
    Semantics(header: true, child: const Text('Release notes')),
    for (var i = 0; i < 30; i++) Text('Line $i'),
    MergeSemantics(
      child: SwitchListTile(
        title: const Text('Notify me'),
        value: true,
        onChanged: (_) {},
      ),
    ),
    TextButton(
      key: const ValueKey('sample.send'),
      onPressed: () {},
      child: const Text('Send'),
    ),
  ],
);

void main() {
  dsPhoneAccessibilityTests(
    'a sample phone screen',
    build: sample,
    open: (tester) async => tester.pump(),
    semantics: (tester, l) {
      expect(find.bySemanticsLabel('Release notes'), findsOneWidget);
    },
    reachable: (l) => [find.byKey(const ValueKey('sample.send'))],
    wrap: (child) => KeyedSubtree(key: const ValueKey('wrap'), child: child),
    snapshot: 'harness_sample',
  );

  dsPhoneAccessibilityTests(
    'a sample phone screen on Android',
    build: sample,
    platform: TargetPlatform.android,
    snapshot: 'harness_sample',
  );

  testWidgets(
    'the host sets language, text scale, theme, platform and reduced motion',
    (tester) async {
      await tester.pumpWidget(
        dsPhoneA11yHost(
          Builder(
            builder: (context) => Text(
              '${Localizations.localeOf(context).languageCode} '
              '${MediaQuery.textScalerOf(context).scale(10)} '
              '${context.dsColors == DsColors.dark} '
              '${Theme.of(context).platform.name} '
              '${MediaQuery.disableAnimationsOf(context)} '
              '${context.dsDensity == DsDensity.touch}',
            ),
          ),
          locale: const Locale('de'),
          textScale: 2,
          theme: DsThemeMode.dark,
          platform: TargetPlatform.android,
        ),
      );
      expect(find.text('de 20.0 true android true true'), findsOneWidget);
    },
  );

  testWidgets(
    'the snapshot lists role, label, value, state and actions in reading order',
    (tester) async {
      final handle = tester.ensureSemantics();
      await tester.pumpWidget(
        dsPhoneA11yHost(
          Column(
            children: [
              Semantics(header: true, child: const Text('Title')),
              Semantics(
                button: true,
                enabled: false,
                label: 'Two\nlines',
                child: const SizedBox(width: 48, height: 48),
              ),
              Semantics(
                container: true,
                textField: true,
                label: 'Comment',
                value: 'Hi',
                hint: 'Optional',
                child: const SizedBox(width: 48, height: 48),
              ),
              Semantics(
                container: true,
                checked: true,
                selected: true,
                expanded: false,
                liveRegion: true,
                image: true,
                slider: true,
                link: true,
                tooltip: 'Tip',
                label: 'Mixed',
                onTap: () {},
                onLongPress: () {},
                onIncrease: () {},
                onDecrease: () {},
                onDismiss: () {},
                child: const SizedBox(width: 48, height: 48),
              ),
              Semantics(
                container: true,
                toggled: false,
                label: 'Off',
                child: const SizedBox(width: 48, height: 48),
              ),
              Semantics(
                container: true,
                toggled: true,
                expanded: true,
                label: 'On',
                child: const SizedBox(width: 48, height: 48),
              ),
            ],
          ),
        ),
      );
      final lines = dsSemanticsSnapshot(tester);
      expect(lines.first, 'header, label "Title"');
      expect(lines, contains(r'button, disabled, label "Two\nlines"'));
      expect(
        lines,
        contains('text field, label "Comment", value "Hi", hint "Optional"'),
      );
      final mixed = lines.singleWhere((l) => l.contains('"Mixed"'));
      for (final part in [
        'link',
        'image',
        'slider',
        'live region',
        'checked isTrue',
        'selected',
        'collapsed',
        'tooltip "Tip"',
        'action tap',
        'action longPress',
        'action increase',
        'action decrease',
        'action dismiss',
      ]) {
        expect(mixed, contains(part));
      }
      expect(lines, contains('switch off, label "Off"'));
      expect(lines, contains('switch on, expanded, label "On"'));
      handle.dispose();
    },
  );

  testWidgets(
    'a snapshot that differs from its file fails, a missing file fails',
    (tester) async {
      final handle = tester.ensureSemantics();
      // The comparison itself is under test, also in a run with --update-goldens.
      final update = autoUpdateGoldenFiles;
      autoUpdateGoldenFiles = false;
      try {
        await tester.pumpWidget(dsPhoneA11yHost(const Text('Changed')));
        final file = File('test/a11y/semantics/harness_changed.txt')
          ..parent.createSync(recursive: true)
          ..writeAsStringSync('label "Before"\n');
        addTearDown(() {
          if (file.existsSync()) file.deleteSync();
        });
        await expectLater(
          dsExpectSemanticsSnapshot(tester, 'harness_changed'),
          throwsA(isA<TestFailure>()),
        );
        file.deleteSync();
        await expectLater(
          dsExpectSemanticsSnapshot(tester, 'harness_changed'),
          throwsA(isA<TestFailure>()),
        );
      } finally {
        autoUpdateGoldenFiles = update;
      }
      handle.dispose();
    },
  );
}
