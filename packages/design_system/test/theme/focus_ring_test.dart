// SPDX-License-Identifier: Apache-2.0

// The focus ring is always visible (01 8 "Focus", WCAG 2.4.7; components.md "Common rules";
// brief M4-32): every kind of control the apps use shows `color.focus.ring` when the keyboard
// focuses it, in light and dark, with `DsFocusRing` installed as the apps install it. The test
// renders the app before and after Tab and counts the pixels in the ring color. Before the
// ring of DsFocusRing, icon buttons, drop-down buttons, list tiles, switches, check boxes,
// radios, segmented buttons and the filled button in light showed no ring at all.

import 'dart:ui' as ui;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

/// The controls of the apps, each alone and enabled.
final Map<String, Widget Function()> controls = {
  'TextButton': () => TextButton(onPressed: () {}, child: const Text('Open')),
  'OutlinedButton': () =>
      OutlinedButton(onPressed: () {}, child: const Text('Open')),
  'FilledButton': () =>
      FilledButton(onPressed: () {}, child: const Text('Open')),
  'IconButton': () => IconButton(
    onPressed: () {},
    tooltip: 'Close',
    icon: const Icon(Icons.close),
  ),
  'DropdownButton': () => DropdownButton<String>(
    value: 'a',
    onChanged: (_) {},
    items: const [
      DropdownMenuItem(value: 'a', child: Text('All')),
      DropdownMenuItem(value: 'b', child: Text('Mentions')),
    ],
  ),
  'ListTile': () => ListTile(onTap: () {}, title: const Text('Robin')),
  'ChoiceChip': () => ChoiceChip(
    label: const Text('Dark'),
    selected: false,
    onSelected: (_) {},
  ),
  'FilterChip': () => FilterChip(
    label: const Text('Open'),
    selected: false,
    onSelected: (_) {},
  ),
  'Switch': () => Switch(value: true, onChanged: (_) {}),
  'Checkbox': () => Checkbox(value: true, onChanged: (_) {}),
  'CheckboxListTile': () => CheckboxListTile(
    value: true,
    onChanged: (_) {},
    title: const Text('Robin'),
  ),
  'SwitchListTile': () => SwitchListTile(
    value: true,
    onChanged: (_) {},
    title: const Text('Robin'),
  ),
  'RadioListTile': () => RadioGroup<int>(
    groupValue: 1,
    onChanged: (_) {},
    child: const RadioListTile<int>(value: 1, title: Text('Robin')),
  ),
  'SegmentedButton': () => SegmentedButton<int>(
    segments: const [
      ButtonSegment(value: 1, label: Text('One')),
      ButtonSegment(value: 2, label: Text('Two')),
    ],
    selected: const {1},
    onSelectionChanged: (_) {},
  ),
  'TextField': () => const SizedBox(width: 240, child: TextField()),
  'DsTappable': () =>
      DsTappable(onTap: () {}, label: 'Open', child: const Text('Open')),
  'DsButton': () => DsButton(label: 'Allow once', onPressed: () {}),
};

const Key boundary = ValueKey('boundary');

Future<int> ringPixels(WidgetTester tester, Color ring) async {
  final render =
      tester.renderObject(find.byKey(boundary)) as RenderRepaintBoundary;
  final bytes = await tester.runAsync(() async {
    final image = await render.toImage();
    try {
      return (await image.toByteData(format: ui.ImageByteFormat.rawRgba))!;
    } finally {
      image.dispose();
    }
  });
  final r = (ring.r * 255).round();
  final g = (ring.g * 255).round();
  final b = (ring.b * 255).round();
  var count = 0;
  final data = bytes!.buffer.asUint8List();
  for (var i = 0; i < data.length; i += 4) {
    if ((data[i] - r).abs() <= 24 &&
        (data[i + 1] - g).abs() <= 24 &&
        (data[i + 2] - b).abs() <= 24) {
      count++;
    }
  }
  return count;
}

void main() {
  for (final theme in DsThemeMode.values) {
    for (final MapEntry(key: name, value: control) in controls.entries) {
      testWidgets('$name draws the focus ring in ${theme.name}', (
        tester,
      ) async {
        final ds = DsContext(
          theme: theme,
          density: DsDensityMode.comfortable,
          motion: DsMotionMode.reduced,
          accent: dsDefaultAccent,
        );
        late Color ring;
        await tester.pumpWidget(
          MaterialApp(
            theme: buildDsTheme(ds),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            builder: (context, app) => DsThemeScope(
              dsContext: ds,
              child: RepaintBoundary(
                key: boundary,
                child: DsFocusRing(child: app!),
              ),
            ),
            home: DsThemeScope(
              dsContext: ds,
              child: Builder(
                builder: (context) {
                  ring = context.dsAccent.focusRing;
                  return Material(
                    color: context.dsColors.surfaceCanvas,
                    child: Center(
                      child: RepaintBoundary(
                        child: Material(
                          color: context.dsColors.surfaceCanvas,
                          child: Padding(
                            padding: const EdgeInsets.all(8),
                            child: SizedBox(
                              width: 320,
                              height: 80,
                              child: Center(child: control()),
                            ),
                          ),
                        ),
                      ),
                    ),
                  );
                },
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        final before = await ringPixels(tester, ring);
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.pumpAndSettle();
        expect(
          FocusManager.instance.highlightMode,
          FocusHighlightMode.traditional,
        );
        final after = await ringPixels(tester, ring);
        expect(after - before, greaterThanOrEqualTo(40), reason: name);
      });
    }
  }

  testWidgets('a pointer press hides the ring until the next key', (
    tester,
  ) async {
    const ds = DsContext(
      theme: DsThemeMode.light,
      density: DsDensityMode.comfortable,
      motion: DsMotionMode.reduced,
      accent: dsDefaultAccent,
    );
    final node = FocusNode();
    addTearDown(node.dispose);
    await tester.pumpWidget(
      MaterialApp(
        theme: buildDsTheme(ds),
        builder: (context, app) => DsThemeScope(
          dsContext: ds,
          child: DsFocusRing(child: app!),
        ),
        home: Material(
          child: Center(
            child: IconButton(
              focusNode: node,
              onPressed: () {},
              tooltip: 'Close',
              icon: const Icon(Icons.close),
            ),
          ),
        ),
      ),
    );
    final ring = find.byKey(const ValueKey('ds.focusRing'));
    // A click focuses the button: no ring.
    await tester.tap(find.byType(IconButton));
    node.requestFocus();
    await tester.pumpAndSettle();
    expect(ring, findsNothing);
    // A key shows it.
    await tester.sendKeyEvent(LogicalKeyboardKey.shift);
    await tester.pumpAndSettle();
    expect(ring, findsOneWidget);
    // The next pointer press hides it again.
    await tester.tap(find.byType(IconButton));
    await tester.pumpAndSettle();
    expect(ring, findsNothing);
  });
}
